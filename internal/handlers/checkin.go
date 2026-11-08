package handlers

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/auth"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/constants"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/respond"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/services/calculator"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/services/nutrition"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/validate"
)

type CheckinHandler struct{ s store.Store }

func NewCheckinHandler(s store.Store) *CheckinHandler { return &CheckinHandler{s: s} }

// Check-in policy. Target changes are gated twice: first on enough weigh-ins to
// check in at all, then on enough weigh-in history for the energy-balance model
// to trust its weight term. Neither gate allows the UI to raise targets from
// intake alone.
const (
	checkinWarningNeedWeighIns      = "Not enough weigh-in data yet to check in — log your weight daily (need 3)."
	checkinWarningNeedWeightHistory = "Not enough weigh-in history to update targets — keep logging daily (7 needed)."

	// checkinConflictErr is the SQLite driver's exact UNIQUE text for the
	// one-check-in-per-day constraint. RecordCheckinTx returns it unwrapped so
	// this mapping cannot regress to a 500.
	checkinConflictErr = "UNIQUE constraint failed: check_in_logs.user_id, check_in_logs.check_in_date"
)

// checkinState bundles the inputs and derived gates shared by Preview and
// Create so the two endpoints can never disagree about readiness.
type checkinState struct {
	profile          models.Profile
	targets          models.Targets
	hasTargets       bool
	nutLogs          []models.NutritionLog
	bioLogs          []models.BiometricLog
	tdee             models.TDEEResult
	weighInCount     int
	weightStart      float64
	weightEnd        float64
	weightChange     float64
	hasWeighinData   bool
	daysSinceCheckIn int
	canCheckIn       bool
	warning          string
}

// loadCheckinState fetches everything the check-in decision needs and applies
// the weigh-in and interval gates. A sql.ErrNoRows from FetchProfile is
// returned unchanged so callers can render their own empty state.
func (h *CheckinHandler) loadCheckinState(ctx context.Context, userID string) (*checkinState, error) {
	profile, err := h.s.FetchProfile(ctx, userID)
	if err != nil {
		return nil, err
	}

	lookback := profile.TDEELookbackDays
	if lookback <= 0 {
		lookback = constants.DefaultTDEELookbackDays
	}
	since := time.Now().UTC().AddDate(0, 0, -lookback).Format(constants.DateFormat)

	nutLogs, err := h.s.FetchNutritionLogs(ctx, userID, since)
	if err != nil {
		return nil, err
	}
	bioLogs, err := h.s.FetchBiometricLogs(ctx, userID, since)
	if err != nil {
		return nil, err
	}

	targets, terr := h.s.FetchTargets(ctx, userID)
	hasTargets := terr == nil
	if terr != nil && !errors.Is(terr, sql.ErrNoRows) {
		return nil, terr
	}

	// Body fat feeds the static Katch-McArdle alternative, not the observed
	// energy-balance term.
	var bodyFatPct float64
	for i := len(bioLogs) - 1; i >= 0; i-- {
		if bioLogs[i].BodyFatPct > 0 {
			bodyFatPct = bioLogs[i].BodyFatPct
			break
		}
	}

	state := &checkinState{
		profile:    profile,
		targets:    targets,
		hasTargets: hasTargets,
		nutLogs:    nutLogs,
		bioLogs:    bioLogs,
		tdee:       calculator.ComputeObservedTDEE(nutLogs, bioLogs, profile, bodyFatPct),
	}

	// The weigh-in gate counts actual weight entries, not biometric rows.
	// Sleep/grip-only rows previously unlocked check-ins with zero weigh-ins.
	weightLogs := make([]models.BiometricLog, 0, len(bioLogs))
	for _, bio := range bioLogs {
		if bio.WeightKg > 0 {
			weightLogs = append(weightLogs, bio)
		}
	}
	state.weighInCount = len(weightLogs)
	state.hasWeighinData = state.weighInCount > 0
	if state.weighInCount >= 2 {
		state.weightStart = weightLogs[0].WeightKg
		state.weightEnd = weightLogs[len(weightLogs)-1].WeightKg
		state.weightChange = state.weightEnd - state.weightStart
	} else if state.weighInCount == 1 {
		state.weightStart = weightLogs[0].WeightKg
		state.weightEnd = weightLogs[0].WeightKg
	}

	// Date gate: at most one adjust-capable check-in per interval. This is a
	// cooldown, not a data requirement, so it only blocks the check-in itself.
	state.daysSinceCheckIn = int(math.MaxInt32)
	dateGateOpen := true
	var lastPtr *models.CheckInLog
	if lastDate, err := h.s.FetchLastCheckin(ctx, userID); err == nil {
		if lastDate != "" {
			lastPtr = &models.CheckInLog{CheckInDate: lastDate}
		}
	} else {
		if l, err2 := h.s.GetLastCheckin(ctx, userID); err2 == nil {
			lastPtr = &l
		}
	}
	if lastPtr != nil {
		if lastDate, err := time.Parse(constants.DateFormat, lastPtr.CheckInDate); err == nil {
			state.daysSinceCheckIn = int(time.Since(lastDate).Hours() / 24)
			dateGateOpen = state.daysSinceCheckIn >= constants.MinCheckinIntervalDays
		}
	}

	weighInGateOpen := state.weighInCount >= constants.MinWeighInsForCheckin
	state.canCheckIn = dateGateOpen && weighInGateOpen

	switch {
	case !weighInGateOpen:
		state.warning = checkinWarningNeedWeighIns
	case !dateGateOpen:
		state.warning = "Check-in is available every 5 days — keep logging until the next interval."
	case !state.tdee.WeightReliable:
		state.warning = checkinWarningNeedWeightHistory
	}

	return state, nil
}

// Preview returns check-in readiness and the recommendation that Create would
// apply. When target updates are blocked the recommendation is the CURRENT
// target with zero adjustment: the engine must never raise calories from intake
// alone without weigh-in history.
func (h *CheckinHandler) Preview(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)

	state, err := h.loadCheckinState(r.Context(), claims.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		respond.JSON(w, http.StatusOK, models.CheckinPreview{})
		return
	}
	if err != nil {
		slog.Error("load checkin state for preview", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}

	currentCals := state.targets.Calories
	recCals := int(math.Round(currentCals))
	adjustment := 0.0
	targetUpdateBlocked := !state.canCheckIn || !state.tdee.WeightReliable
	if !targetUpdateBlocked {
		exerciseCals := h.exerciseCaloriesForDate(r.Context(), claims.UserID,
			time.Now().UTC().Format(constants.DateFormat), state.targets.EatBackExercise)
		recCals, adjustment = calculator.ComputeCheckinRecommendation(state.tdee, state.profile, currentCals, exerciseCals, state.targets.EatBackExercise)
	}

	// Weight context is display-only and must not imply usable history when the
	// check-in itself is blocked.
	weightStart, weightEnd, weightChange := 0.0, 0.0, 0.0
	if state.canCheckIn && state.hasWeighinData {
		weightStart, weightEnd, weightChange = state.weightStart, state.weightEnd, state.weightChange
	}

	// Low-confidence + full-clamp warning: when the adjustment is maxed at
	// ±500 while confidence is low, the recommendation is extrapolating from
	// limited information. Only meaningful when a real update is allowed.
	lowConfidenceClamp := !targetUpdateBlocked && state.tdee.Confidence == "low" && math.Abs(adjustment) >= 490

	warning := state.warning
	if warning == "" {
		warning = state.tdee.Warning
	}

	preview := models.CheckinPreview{
		CanCheckIn:           state.canCheckIn,
		DaysSinceLastCheckIn: state.daysSinceCheckIn,

		ObservedTDEE:         state.tdee.ObservedTDEE,
		EstimatedTDEE:        state.tdee.EstimatedTDEE,
		Confidence:           state.tdee.Confidence,
		TDEEMethod:           state.tdee.Method,
		DaysOfData:           state.tdee.DaysOfData,
		LowConfidenceWarning: lowConfidenceClamp,
		TDEEConfidence:       state.tdee.Confidence,
		LookbackDays:         state.tdee.LookbackDays,

		WeightStart:    weightStart,
		WeightEnd:      weightEnd,
		WeightChange:   weightChange,
		HasWeighinData: state.hasWeighinData,

		CurrentCalories: currentCals,
		EatBackExercise: state.targets.EatBackExercise,

		RecommendedCalories: recCals,
		CalorieAdjustment:   adjustment,

		WeighInCount:        state.weighInCount,
		WeightReliable:      state.tdee.WeightReliable,
		TargetUpdateBlocked: targetUpdateBlocked,
		Warning:             warning,
	}

	respond.JSON(w, http.StatusOK, preview)
}

// POST /v1/checkin — record the check-in and apply targets only when the
// weigh-in and reliability gates are satisfied.
func (h *CheckinHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	var in models.CheckInLog
	if !respond.Decode(w, r, &in) {
		return
	}
	if in.CheckInDate != "" {
		if err := validate.Date(in.CheckInDate); err != nil {
			respond.Error(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if err := validate.NonNegativeBoundedInt("caloriesAfter", in.CaloriesAfter, 10000); err != nil {
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	in.UserID = claims.UserID
	in.ID = uuid.NewString()
	if in.CheckInDate == "" {
		in.CheckInDate = time.Now().UTC().Format("2006-01-02")
	}

	state, err := h.loadCheckinState(r.Context(), claims.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		respond.Error(w, http.StatusBadRequest, "profile not found")
		return
	}
	if err != nil {
		slog.Error("load checkin state for create", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}

	// No weigh-ins, or still in the check-in cooldown: reject without writing a
	// check-in row or touching targets.
	if !state.canCheckIn {
		respond.Error(w, http.StatusBadRequest, state.warning)
		return
	}

	// Nanosecond precision keeps target_history's
	// UNIQUE(user_id,effective_date,created_at) key collision-free when a
	// check-in snapshot and a manual targets edit land in the same second.
	// Existing rows stay second-precision; the string key only needs the
	// values to differ, so no migration is required for the mixed formats.
	now := time.Now().UTC().Format(time.RFC3339Nano)
	in.CreatedAt = now

	// Persist the engine snapshot exactly as produced for this run.
	in.ObservedTDEE = state.tdee.ObservedTDEE
	in.TDEEConfidence = state.tdee.Confidence
	in.LookbackDays = state.tdee.LookbackDays

	currentCalories := state.targets.Calories
	in.CaloriesBefore = int(math.Round(currentCalories))

	// The engine's weight-reliability gate is the policy switch. Without it the
	// check-in is still recorded, but targets are reset to their current values
	// (caloriesAfter = current) and never adjusted.
	targetUpdateBlocked := !state.tdee.WeightReliable

	var (
		newTargets *models.Targets
		snap       *models.TargetSnapshot
	)
	if targetUpdateBlocked {
		in.CaloriesAfter = int(math.Round(currentCalories))
	} else {
		exerciseCals := h.exerciseCaloriesForDate(r.Context(), claims.UserID, in.CheckInDate, state.targets.EatBackExercise)
		recommended, _ := calculator.ComputeCheckinRecommendation(state.tdee, state.profile, currentCalories, exerciseCals, state.targets.EatBackExercise)
		in.CaloriesAfter = recommended

		// Capture the superseded targets BEFORE mutating the row. These are the
		// pre-change values shown in target history.
		if state.hasTargets {
			snap = &models.TargetSnapshot{
				ID:              in.ID,
				UserID:          claims.UserID,
				EffectiveDate:   in.CheckInDate,
				Calories:        state.targets.Calories,
				ProteinG:        state.targets.ProteinG,
				CarbsG:          state.targets.CarbsG,
				FatG:            state.targets.FatG,
				FiberG:          state.targets.FiberG,
				WaterMl:         state.targets.WaterMl,
				EatBackExercise: state.targets.EatBackExercise,
				CreatedAt:       now,
			}
		}

		// Full path: recompute the COMPLETE macro set from the final calorie
		// target so macros are always consistent with the stored calories.
		targets := state.targets
		targets.UserID = claims.UserID
		targets.Calories = float64(in.CaloriesAfter)
		targets.UpdatedAt = now

		if weight := latestPlausibleWeight(state.bioLogs); weight > 0 {
			macros := nutrition.ComputeMacros(state.profile, targets.Calories, weight)
			if isFiniteMacroResult(macros) {
				targets.ProteinG = macros.ProteinG
				targets.CarbsG = macros.CarbsG
				targets.FatG = macros.FatG
				targets.FiberG = macros.FiberG
				targets.WaterMl = macros.WaterMl
			} else {
				// Never leave a torn state: keep the calorie update and log the
				// failed recompute. The previous macro row is preserved.
				slog.Warn("macro recompute produced invalid values; updating calories only",
					"user", claims.UserID, "calories", targets.Calories, "weight", weight)
			}
		} else {
			slog.Warn("macro recompute skipped: no usable weight; updating calories only",
				"user", claims.UserID, "calories", targets.Calories)
		}
		newTargets = &targets
	}

	// One transaction: the check-in row and (on the reliable path) the target
	// update + pre-change history snapshot commit together. On the blocked path
	// targets stay untouched, so a stray client-supplied caloriesAfter cannot
	// move the row and no history row is written.
	if err := h.s.RecordCheckinTx(r.Context(), &in, newTargets, snap); err != nil {
		if strings.Contains(err.Error(), checkinConflictErr) {
			respond.Error(w, http.StatusConflict, "already checked in today")
			return
		}
		slog.Error("record checkin transaction", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}

	respond.JSON(w, http.StatusCreated, in)
}

// exerciseCaloriesForDate returns the eaten-back exercise calories for a date
// when the user has enabled eat-back. Any store error is downgraded to zero
// with a warning: a missing workout sum must never fail a check-in.
func (h *CheckinHandler) exerciseCaloriesForDate(ctx context.Context, userID, date string, enabled bool) float64 {
	if !enabled {
		return 0
	}
	cals, err := h.s.SumWorkoutCaloriesForDate(ctx, userID, date)
	if err != nil {
		slog.Warn("sum workout calories for eat-back failed; treating as zero",
			"user", userID, "date", date, "err", err)
		return 0
	}
	return cals
}

// latestPlausibleWeight returns the most recent weight in the human range,
// which is what the macro formulas expect (kg).
func latestPlausibleWeight(bioLogs []models.BiometricLog) float64 {
	for i := len(bioLogs) - 1; i >= 0; i-- {
		w := bioLogs[i].WeightKg
		if w > 20 && w < 300 {
			return w
		}
	}
	return 0
}

func isFiniteMacroResult(m models.MacroResult) bool {
	values := []float64{m.ProteinG, m.CarbsG, m.FatG, m.FiberG, m.WaterMl}
	for _, v := range values {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			return false
		}
	}
	return true
}

// GET /v1/checkin/history — recent check-ins with their persisted TDEE snapshot.
func (h *CheckinHandler) History(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)

	history, err := h.s.FetchCheckinHistory(r.Context(), claims.UserID, 50)
	if err != nil {
		slog.Error("fetch checkin history", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	if history == nil {
		history = []models.CheckinHistoryEntry{}
	}
	respond.JSON(w, http.StatusOK, history)
}
