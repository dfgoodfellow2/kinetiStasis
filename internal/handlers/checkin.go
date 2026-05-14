package handlers

import (
	"database/sql"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/auth"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/constants"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/respond"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/services/calculator"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"
)

type CheckinHandler struct{ s store.Store }

func NewCheckinHandler(s store.Store) *CheckinHandler { return &CheckinHandler{s: s} }

// GET /v1/checkin — preview readiness and adjustment using adaptive TDEE
func (h *CheckinHandler) Preview(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)

	// Fetch profile
	profile, err := h.s.FetchProfile(r.Context(), claims.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		respond.JSON(w, http.StatusOK, models.CheckinPreview{})
		return
	}
	if err != nil {
		slog.Error("fetch profile for checkin preview", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}

	// Gather lookback data (use profile lookback or default)
	lookback := profile.TDEELookbackDays
	if lookback <= 0 {
		lookback = constants.DefaultTDEELookbackDays
	}
	since := time.Now().UTC().AddDate(0, 0, -lookback).Format(constants.DateFormat)
	nutLogs, err := h.s.FetchNutritionLogs(r.Context(), claims.UserID, since)
	if err != nil {
		slog.Error("fetch nutrition logs for checkin preview", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	bioLogs, err := h.s.FetchBiometricLogs(r.Context(), claims.UserID, since)
	if err != nil {
		slog.Error("fetch biometric logs for checkin preview", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}

	// Fetch current targets
	targets, err := h.s.FetchTargets(r.Context(), claims.UserID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Error("fetch targets for checkin preview", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}

	// Fetch last check-in for date gating
	var lastPtr *models.CheckInLog
	if lastDate, err := h.s.FetchLastCheckin(r.Context(), claims.UserID); err == nil {
		if lastDate != "" {
			lastPtr = &models.CheckInLog{CheckInDate: lastDate}
		}
	} else {
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

	// --- Adaptive TDEE computation (intake-driven, not weight-delta) ---
	tdeeRes := calculator.ComputeObservedTDEE(nutLogs, bioLogs, profile, bodyFatPct)

	// --- Weight trend display (cosmetic, from biometric logs) ---
	var weightStart, weightEnd, weightChange float64
	if len(bioLogs) >= 2 {
		weightStart = bioLogs[0].WeightKg
		weightEnd = bioLogs[len(bioLogs)-1].WeightKg
		weightChange = weightEnd - weightStart
	}

	// --- Date gating (MinCheckinIntervalDays rule) ---
	daysSinceCheckIn := int(math.MaxInt32)
	canCheckIn := true
	if lastPtr != nil {
		lastDate, _ := time.Parse(constants.DateFormat, lastPtr.CheckInDate)
		daysSinceCheckIn = int(time.Since(lastDate).Hours() / 24)
		canCheckIn = daysSinceCheckIn >= constants.MinCheckinIntervalDays
	}
	hasEnoughData := len(bioLogs) >= constants.MinBiometricLogsForCheckin
	canCheckIn = canCheckIn && hasEnoughData

	// --- Compute damped recommendation ---
	currentCals := targets.Calories
	recCals, adj := calculator.ComputeCheckinRecommendation(tdeeRes, profile, currentCals, 0, targets.EatBackExercise)

	// Low-confidence + full-clamp warning: when we have very few data days
	// and the adjustment is maxed at ±500, the user should know the
	// recommendation is extrapolating from limited information.
	lowConfidenceClamp := tdeeRes.Confidence == "low" && math.Abs(adj) >= 490

	preview := models.CheckinPreview{
		CanCheckIn:           canCheckIn,
		DaysSinceLastCheckIn: daysSinceCheckIn,

		ObservedTDEE:         tdeeRes.ObservedTDEE,
		EstimatedTDEE:        tdeeRes.EstimatedTDEE,
		Confidence:           tdeeRes.Confidence,
		TDEEMethod:           tdeeRes.Method,
		DaysOfData:           tdeeRes.DaysOfData,
		LowConfidenceWarning: lowConfidenceClamp,

		WeightStart:  weightStart,
		WeightEnd:    weightEnd,
		WeightChange: weightChange,

		CurrentCalories: currentCals,
		EatBackExercise: targets.EatBackExercise,

		RecommendedCalories: recCals,
		CalorieAdjustment:   adj,
	}

	respond.JSON(w, http.StatusOK, preview)
}

// POST /v1/checkin — record check-in and optionally apply target changes
func (h *CheckinHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	var in models.CheckInLog
	if !respond.Decode(w, r, &in) {
		return
	}
	in.UserID = claims.UserID
	in.ID = uuid.NewString()
	if in.CheckInDate == "" {
		in.CheckInDate = time.Now().UTC().Format("2006-01-02")
	}

	// NOTE: Previously there was a 5-day rate limit here. That gating
	// has been removed — the Create handler only enforces the unique
	// constraint (one check-in per date).

	now := time.Now().UTC().Format(time.RFC3339)
	in.CreatedAt = now

	// Insert check-in log
	if err := h.s.CreateCheckinLog(r.Context(), &in); err != nil {
		if err.Error() == "UNIQUE constraint failed: check_in_logs.user_id, check_in_logs.check_in_date" {
			respond.Error(w, http.StatusConflict, "already checked in today")
			return
		}
		slog.Error("create checkin log", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}

	// Always snapshot current targets for history
	existing, err := h.s.FetchTargets(r.Context(), claims.UserID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Error("fetch targets after checkin", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	if err == nil {
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
			slog.Error("create target snapshot", "err", err)
			respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
			return
		}
	}

	// Only update targets if caloriesAfter provided (accept) vs skip
	if in.CaloriesAfter > 0 {
		targets := existing
		targets.Calories = float64(in.CaloriesAfter)
		targets.UpdatedAt = now
		if err := h.s.UpsertTargets(r.Context(), &targets); err != nil {
			slog.Error("upsert targets after checkin", "err", err)
			respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
			return
		}
	}

	respond.JSON(w, http.StatusCreated, in)
}
