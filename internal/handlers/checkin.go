package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/auth"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/respond"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/services/calculator"
)

type CheckinHandler struct{ db *sql.DB }

func NewCheckinHandler(db *sql.DB) *CheckinHandler { return &CheckinHandler{db: db} }

// GET /v1/checkin - preview readiness and adjustment
func (h *CheckinHandler) Preview(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	// fetch profile
	profile, err := fetchProfile(r.Context(), h.db, claims.UserID)
	if err == sql.ErrNoRows {
		respond.JSON(w, http.StatusOK, models.DashboardData{})
		return
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}

	// gather lookback data (14 days)
	since := time.Now().UTC().AddDate(0, 0, -14).Format("2006-01-02")
	nutLogs, _ := fetchNutritionLogs(r.Context(), h.db, claims.UserID, since)
	bioLogs, _ := fetchBiometricLogs(r.Context(), h.db, claims.UserID, since)

	// fetch current targets
	targets, _ := fetchTargets(r.Context(), h.db, claims.UserID)

	// fetch last checkin if exists
	var last models.CheckInLog
	err = h.db.QueryRowContext(r.Context(), `SELECT id, user_id, check_in_date, weight_before, weight_after, calories_before, calories_after, reason, created_at FROM check_in_logs WHERE user_id = ? ORDER BY check_in_date DESC LIMIT 1`, claims.UserID).Scan(&last.ID, &last.UserID, &last.CheckInDate, &last.WeightBefore, &last.WeightAfter, &last.CaloriesBefore, &last.CaloriesAfter, &last.Reason, &last.CreatedAt)
	var lastPtr *models.CheckInLog
	if err == nil {
		lastPtr = &last
	}

	// Find most recent biometric log with a body fat percentage
	var bodyFatPct float64
	for i := len(bioLogs) - 1; i >= 0; i-- {
		if bioLogs[i].BodyFatPct > 0 {
			bodyFatPct = bioLogs[i].BodyFatPct
			break
		}
	}
	adj := calculator.ComputeWeightGoalAdjustment(profile, targets, nutLogs, bioLogs, lastPtr, bodyFatPct)
	respond.JSON(w, http.StatusOK, adj)
}

// POST /v1/checkin - record check-in and apply target changes
func (h *CheckinHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	var in models.CheckInLog
	if !respond.Decode(w, r, &in) {
		return
	}
	in.UserID = claims.UserID
	in.ID = uuid.NewString()
	// Use today's date if not provided
	if in.CheckInDate == "" {
		in.CheckInDate = time.Now().UTC().Format("2006-01-02")
	}

	// enforce 5-day rate limit: check last check-in date
	var lastDate string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT check_in_date FROM check_in_logs WHERE user_id = ? ORDER BY check_in_date DESC LIMIT 1`,
		claims.UserID).Scan(&lastDate)
	if err == nil {
		t, _ := time.Parse("2006-01-02", lastDate)
		if int(time.Since(t).Hours()/24) < 5 {
			respond.Error(w, http.StatusTooEarly, "check-in not available yet")
			return
		}
	}
	// naive insert
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = h.db.ExecContext(r.Context(), `INSERT INTO check_in_logs (id,user_id,check_in_date,weight_before,weight_after,calories_before,calories_after,reason,created_at) VALUES (?,?,?,?,?,?,?,?,?)`, in.ID, in.UserID, in.CheckInDate, in.WeightBefore, in.WeightAfter, in.CaloriesBefore, in.CaloriesAfter, in.Reason, now)
	if err != nil {
		// Check if it's a unique constraint violation (already checked in today)
		if err.Error() == "UNIQUE constraint failed: check_in_logs.user_id, check_in_logs.check_in_date" {
			respond.Error(w, http.StatusConflict, "already checked in today")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}

	// Optionally update targets if calories_after provided
	if in.CaloriesAfter > 0 {
		// fetch existing targets for snapshot
		var existing models.Targets
		_ = h.db.QueryRowContext(r.Context(), `SELECT user_id,calories,protein_g,carbs_g,fat_g,fiber_g,water_ml,eat_back_exercise,updated_at FROM targets WHERE user_id = ?`, claims.UserID).Scan(&existing.UserID, &existing.Calories, &existing.ProteinG, &existing.CarbsG, &existing.FatG, &existing.FiberG, &existing.WaterMl, &existing.EatBackExercise, &existing.UpdatedAt)
		// snapshot
		_, _ = h.db.ExecContext(r.Context(), `INSERT INTO target_history (id,user_id,effective_date,calories,protein_g,carbs_g,fat_g,fiber_g,created_at) VALUES (?,?,?,?,?,?,?,?,?)`, in.ID, claims.UserID, in.CheckInDate, existing.Calories, existing.ProteinG, existing.CarbsG, existing.FatG, existing.FiberG, now)
		// upsert new targets (minimal: only calories changed)
		_, _ = h.db.ExecContext(r.Context(), `INSERT INTO targets (user_id,calories,protein_g,carbs_g,fat_g,fiber_g,water_ml,eat_back_exercise,updated_at) VALUES (?,?,?,?,?,?,?,?,?) ON CONFLICT(user_id) DO UPDATE SET calories=excluded.calories,updated_at=excluded.updated_at`, claims.UserID, in.CaloriesAfter, existing.ProteinG, existing.CarbsG, existing.FatG, existing.FiberG, existing.WaterMl, existing.EatBackExercise, now)
	}

	respond.JSON(w, http.StatusCreated, in)
}
