package handlers

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"reflect"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/auth"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/constants"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/respond"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/services/calculator"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/services/nutrition"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/validate"
	"github.com/google/uuid"
)

type TargetsHandler struct{ s store.Store }

func NewTargetsHandler(s store.Store) *TargetsHandler { return &TargetsHandler{s: s} }

const (
	maxTargetsCalories = 10000
	maxTargetsMacroG   = 500
	maxTargetsFiberG   = 200
	maxTargetsWaterMl  = 20000
)

// validateTargets enforces non-negative, human-scale bounds on every target
// item, including the water and eat-back fields that were previously ignored.
func validateTargets(t *models.Targets) error {
	if err := validate.NonNegativeBounded("calories", t.Calories, maxTargetsCalories); err != nil {
		return err
	}
	if err := validate.NonNegativeBounded("proteinG", t.ProteinG, maxTargetsMacroG); err != nil {
		return err
	}
	if err := validate.NonNegativeBounded("carbsG", t.CarbsG, maxTargetsMacroG); err != nil {
		return err
	}
	if err := validate.NonNegativeBounded("fatG", t.FatG, maxTargetsMacroG); err != nil {
		return err
	}
	if err := validate.NonNegativeBounded("fiberG", t.FiberG, maxTargetsFiberG); err != nil {
		return err
	}
	if err := validate.NonNegativeBounded("waterMl", t.WaterMl, maxTargetsWaterMl); err != nil {
		return err
	}
	return nil
}

// GET /v1/targets
func (h *TargetsHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	var t models.Targets
	var err error
	t, err = h.s.FetchTargets(r.Context(), claims.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		// Compute fallback targets when no stored targets exist for the user.
		profile, err := h.s.FetchProfile(r.Context(), claims.UserID)
		if errors.Is(err, sql.ErrNoRows) {
			// No profile — cannot compute targets
			respond.Error(w, http.StatusNotFound, "profile not found")
			return
		}
		if err != nil {
			slog.Error("fetch profile for fallback targets", "err", err)
			respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
			return
		}

		// latest weight
		weight, _ := h.s.FetchLatestWeight(r.Context(), claims.UserID)

		// determine lookback days
		lookback := profile.TDEELookbackDays
		if lookback <= 0 {
			lookback = constants.DefaultTDEELookbackDays
		}
		since := time.Now().UTC().AddDate(0, 0, -lookback).Format(constants.DateFormat)

		nutLogs, err := h.s.FetchNutritionLogs(r.Context(), claims.UserID, since)
		if err != nil {
			slog.Error("fetch nutrition logs for fallback targets", "err", err)
			respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
			return
		}
		bioLogs, err := h.s.FetchBiometricLogs(r.Context(), claims.UserID, since)
		if err != nil {
			slog.Error("fetch biometric logs for fallback targets", "err", err)
			respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
			return
		}

		observed := calculator.ComputeObservedTDEE(nutLogs, bioLogs, profile, 0)
		plan := nutrition.FullDietPlan(profile, weight, observed.ObservedTDEE)

		// map MacroResult -> Targets
		computed := models.Targets{
			UserID:          claims.UserID,
			Calories:        plan.Calories,
			ProteinG:        plan.ProteinG,
			CarbsG:          plan.CarbsG,
			FatG:            plan.FatG,
			FiberG:          plan.FiberG,
			WaterMl:         plan.WaterMl,
			EatBackExercise: false,
			UpdatedAt:       time.Now().UTC().Format(time.RFC3339),
		}
		slog.Info("returning computed fallback targets", "user", claims.UserID, "observed_days", observed.DaysOfData)
		w.Header().Set("X-Computed-Targets", "true")
		respond.JSON(w, http.StatusOK, computed)
		return
	}
	if err != nil {
		slog.Error("fetch targets", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, t)
}

// PUT /v1/targets — upsert and snapshot if changed
func (h *TargetsHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	var in models.Targets
	if !respond.Decode(w, r, &in) {
		return
	}
	if err := validateTargets(&in); err != nil {
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	in.UserID = claims.UserID
	now := time.Now().UTC().Format(time.RFC3339)

	var err error

	// load existing
	var existing models.Targets
	existing, err = h.s.FetchTargets(r.Context(), claims.UserID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Error("fetch targets for update", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}

	// If changed, snapshot the old targets. Compare the full incoming struct
	// against the stored row (with volatile identity/timestamp fields
	// normalised out) so water-only and eat-back-only edits are not missed.
	changed := false
	if !errors.Is(err, sql.ErrNoRows) {
		existingCmp := existing
		inCmp := in
		existingCmp.UserID, inCmp.UserID = "", ""
		existingCmp.UpdatedAt, inCmp.UpdatedAt = "", ""
		changed = !reflect.DeepEqual(existingCmp, inCmp)
	}
	// perform snapshot + upsert via store methods (transaction handled inside store)
	if changed {
		snap := &models.TargetSnapshot{ID: uuid.New().String(), UserID: claims.UserID, EffectiveDate: time.Now().UTC().Format("2006-01-02"), Calories: existing.Calories, ProteinG: existing.ProteinG, CarbsG: existing.CarbsG, FatG: existing.FatG, FiberG: existing.FiberG, WaterMl: existing.WaterMl, EatBackExercise: existing.EatBackExercise, CreatedAt: now}
		if err := h.s.CreateTargetSnapshot(r.Context(), snap); err != nil {
			slog.Error("create target snapshot", "err", err)
			respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
			return
		}
	}
	in.UpdatedAt = now
	if err := h.s.UpsertTargets(r.Context(), &in); err != nil {
		slog.Error("upsert targets", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	in.UpdatedAt = now
	respond.JSON(w, http.StatusOK, in)
}

// GET /v1/targets/history
func (h *TargetsHandler) History(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	out, err := h.s.FetchTargetHistory(r.Context(), claims.UserID)
	if err != nil {
		slog.Error("fetch target history", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, out)
}
