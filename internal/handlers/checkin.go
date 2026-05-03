package handlers

import (
	"database/sql"
	"log/slog"
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
		slog.Error("database operation failed", "err", err, "endpoint", r.URL.Path)
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}

	// gather lookback data (14 days)
	since := time.Now().UTC().AddDate(0, 0, -14).Format("2006-01-02")
	nutLogs, err := h.s.FetchNutritionLogs(r.Context(), claims.UserID, since)
	if err != nil {
		slog.Error("database operation failed", "err", err, "endpoint", r.URL.Path)
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	bioLogs, err := h.s.FetchBiometricLogs(r.Context(), claims.UserID, since)
	if err != nil {
		slog.Error("database operation failed", "err", err, "endpoint", r.URL.Path)
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}

	// fetch current targets
	targets, err := h.s.FetchTargets(r.Context(), claims.UserID)
	if err != nil && err != sql.ErrNoRows {
		slog.Error("database operation failed", "err", err, "endpoint", r.URL.Path)
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
		// fall back to fetching full last checkin row
		if l, err2 := h.s.GetLastCheckin(r.Context(), claims.UserID); err2 == nil {
			lastPtr = &l
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
	// naive insert via store
	now := time.Now().UTC().Format(time.RFC3339)
	in.CreatedAt = now
	if err = h.s.CreateCheckinLog(r.Context(), &in); err != nil {
		// Check if it's a unique constraint violation (already checked in today)
		if err.Error() == "UNIQUE constraint failed: check_in_logs.user_id, check_in_logs.check_in_date" {
			respond.Error(w, http.StatusConflict, "already checked in today")
			return
		}
		slog.Error("database operation failed", "err", err, "endpoint", r.URL.Path)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}

	// Optionally update targets if calories_after provided
	if in.CaloriesAfter > 0 {
		// fetch existing targets for snapshot
		existing, err := h.s.FetchTargets(r.Context(), claims.UserID)
		if err != nil && err != sql.ErrNoRows {
			slog.Error("database operation failed", "err", err, "endpoint", r.URL.Path)
			respond.Error(w, http.StatusInternalServerError, "database error")
			return
		}
		// snapshot
		snap := models.TargetSnapshot{
			ID:            in.ID,
			UserID:        claims.UserID,
			EffectiveDate: in.CheckInDate,
			Calories:      existing.Calories,
			ProteinG:      existing.ProteinG,
			CarbsG:        existing.CarbsG,
			FatG:          existing.FatG,
			FiberG:        existing.FiberG,
			CreatedAt:     now,
		}
		if err := h.s.CreateTargetSnapshot(r.Context(), &snap); err != nil {
			slog.Error("database operation failed", "err", err, "endpoint", r.URL.Path)
			respond.Error(w, http.StatusInternalServerError, "database error")
			return
		}
		// upsert new targets (minimal: only calories changed)
		targets := existing
		targets.Calories = float64(in.CaloriesAfter)
		targets.UpdatedAt = now
		if err := h.s.UpsertTargets(r.Context(), &targets); err != nil {
			slog.Error("database operation failed", "err", err, "endpoint", r.URL.Path)
			respond.Error(w, http.StatusInternalServerError, "database error")
			return
		}
	}

	respond.JSON(w, http.StatusCreated, in)
}
