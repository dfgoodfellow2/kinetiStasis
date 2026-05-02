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
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"
)

type CheckinHandler struct{ s store.Store }

func NewCheckinHandler(s store.Store) *CheckinHandler { return &CheckinHandler{s: s} }

// GET /v1/checkin - preview readiness and adjustment
func (h *CheckinHandler) Preview(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	// fetch profile
	profile, err := h.s.FetchProfile(r.Context(), claims.UserID)
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
	nutLogs, err := h.s.FetchNutritionLogs(r.Context(), claims.UserID, since)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	bioLogs, err := h.s.FetchBiometricLogs(r.Context(), claims.UserID, since)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}

	// fetch current targets
	targets, err := h.s.FetchTargets(r.Context(), claims.UserID)
	if err != nil && err != sql.ErrNoRows {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}

	// fetch last checkin if exists
	var last models.CheckInLog
	var lastPtr *models.CheckInLog
	// Try to use store method to fetch last checkin date
	if lastDate, err := h.s.FetchLastCheckin(r.Context(), claims.UserID); err == nil {
		if lastDate != "" {
			last = models.CheckInLog{CheckInDate: lastDate}
			lastPtr = &last
		}
	} else {
		// fall back to direct DB read for full row if store returned an error
		db := h.s.DB()
		err = db.QueryRowContext(r.Context(), `SELECT id, user_id, check_in_date, weight_before, weight_after, calories_before, calories_after, reason, created_at FROM check_in_logs WHERE user_id = ? ORDER BY check_in_date DESC LIMIT 1`, claims.UserID).Scan(&last.ID, &last.UserID, &last.CheckInDate, &last.WeightBefore, &last.WeightAfter, &last.CaloriesBefore, &last.CaloriesAfter, &last.Reason, &last.CreatedAt)
		if err == nil {
			lastPtr = &last
		}
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
	var err error
	var lastDate string
	lastDate, err = h.s.FetchLastCheckin(r.Context(), claims.UserID)
	if err == nil && lastDate != "" {
		t, _ := time.Parse("2006-01-02", lastDate)
		if int(time.Since(t).Hours()/24) < 5 {
			respond.Error(w, http.StatusTooEarly, "check-in not available yet")
			return
		}
	}
	// naive insert
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = h.s.DB().ExecContext(r.Context(), `INSERT INTO check_in_logs (id,user_id,check_in_date,weight_before,weight_after,calories_before,calories_after,reason,created_at) VALUES (?,?,?,?,?,?,?,?,?)`, in.ID, in.UserID, in.CheckInDate, in.WeightBefore, in.WeightAfter, in.CaloriesBefore, in.CaloriesAfter, in.Reason, now)
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
		if err := h.s.DB().QueryRowContext(r.Context(), `SELECT user_id,calories,protein_g,carbs_g,fat_g,fiber_g,water_ml,eat_back_exercise,updated_at FROM targets WHERE user_id = ?`, claims.UserID).Scan(&existing.UserID, &existing.Calories, &existing.ProteinG, &existing.CarbsG, &existing.FatG, &existing.FiberG, &existing.WaterMl, &existing.EatBackExercise, &existing.UpdatedAt); err != nil && err != sql.ErrNoRows {
			respond.Error(w, http.StatusInternalServerError, "database error")
			return
		}
		// snapshot
		if _, err := h.s.DB().ExecContext(r.Context(), `INSERT INTO target_history (id,user_id,effective_date,calories,protein_g,carbs_g,fat_g,fiber_g,created_at) VALUES (?,?,?,?,?,?,?,?,?)`, in.ID, claims.UserID, in.CheckInDate, existing.Calories, existing.ProteinG, existing.CarbsG, existing.FatG, existing.FiberG, now); err != nil {
			respond.Error(w, http.StatusInternalServerError, "database error")
			return
		}
		// upsert new targets (minimal: only calories changed)
		if _, err := h.s.DB().ExecContext(r.Context(), `INSERT INTO targets (user_id,calories,protein_g,carbs_g,fat_g,fiber_g,water_ml,eat_back_exercise,updated_at) VALUES (?,?,?,?,?,?,?,?,?) ON CONFLICT(user_id) DO UPDATE SET calories=excluded.calories,updated_at=excluded.updated_at`, claims.UserID, in.CaloriesAfter, existing.ProteinG, existing.CarbsG, existing.FatG, existing.FiberG, existing.WaterMl, existing.EatBackExercise, now); err != nil {
			respond.Error(w, http.StatusInternalServerError, "database error")
			return
		}
	}

	respond.JSON(w, http.StatusCreated, in)
}
