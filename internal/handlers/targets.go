package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/auth"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/constants"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/respond"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/services/calculator"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/services/nutrition"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"
	"github.com/google/uuid"
)

type TargetsHandler struct{ s store.Store }

func NewTargetsHandler(s store.Store) *TargetsHandler { return &TargetsHandler{s: s} }

// GET /v1/targets
func (h *TargetsHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	var t models.Targets
	var err error
	t, err = h.s.FetchTargets(r.Context(), claims.UserID)
	if err == sql.ErrNoRows {
		// Compute fallback targets when no stored targets exist for the user.
		profile, err := h.s.FetchProfile(r.Context(), claims.UserID)
		if err == sql.ErrNoRows {
			// No profile — cannot compute targets
			respond.Error(w, http.StatusNotFound, "profile not found")
			return
		}
		if err != nil {
			respond.Error(w, http.StatusInternalServerError, "database error")
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
			respond.Error(w, http.StatusInternalServerError, "database error")
			return
		}
		bioLogs, err := h.s.FetchBiometricLogs(r.Context(), claims.UserID, since)
		if err != nil {
			respond.Error(w, http.StatusInternalServerError, "database error")
			return
		}

		observed := calculator.ComputeObservedTDEE(nutLogs, bioLogs, profile)
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
		log.Printf("returning computed fallback targets for user=%s (observed_days=%d)", claims.UserID, observed.DaysOfData)
		w.Header().Set("X-Computed-Targets", "true")
		respond.JSON(w, http.StatusOK, computed)
		return
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
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
	in.UserID = claims.UserID
	now := time.Now().UTC().Format(time.RFC3339)

	var err error

	// load existing
	var existing models.Targets
	existing, err = h.s.FetchTargets(r.Context(), claims.UserID)
	if err != nil && err != sql.ErrNoRows {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}

	// if changed, snapshot old
	changed := false
	if err != sql.ErrNoRows {
		if existing.Calories != in.Calories || existing.ProteinG != in.ProteinG || existing.CarbsG != in.CarbsG || existing.FatG != in.FatG || existing.FiberG != in.FiberG {
			changed = true
		}
	}
	// perform snapshot + upsert via store methods (transaction handled inside store)
	if changed {
		snap := &models.TargetSnapshot{ID: uuid.New().String(), UserID: claims.UserID, EffectiveDate: time.Now().UTC().Format("2006-01-02"), Calories: existing.Calories, ProteinG: existing.ProteinG, CarbsG: existing.CarbsG, FatG: existing.FatG, FiberG: existing.FiberG, CreatedAt: now}
		if err := h.s.CreateTargetSnapshot(r.Context(), snap); err != nil {
			respond.Error(w, http.StatusInternalServerError, "database error")
			return
		}
	}
	in.UpdatedAt = now
	if err := h.s.UpsertTargets(r.Context(), &in); err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
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
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	respond.JSON(w, http.StatusOK, out)
}
