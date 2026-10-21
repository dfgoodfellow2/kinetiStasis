package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	// removed temporary debug logging
	"log/slog"
	"net/http"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/auth"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/respond"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"
)

type ProfileHandler struct{ s store.Store }

func NewProfileHandler(s store.Store) *ProfileHandler { return &ProfileHandler{s: s} }

// validMacroDistributions is the closed set of supported macro distributions.
var validMacroDistributions = map[string]struct{}{
	"keto":      {},
	"low_carb":  {},
	"balanced":  {},
	"high_carb": {},
}

// normalizeMacroDistribution maps an incoming macroDistribution value to one of
// the supported distributions. An empty value defaults to "balanced"; any other
// unknown value is rejected (ok == false).
func normalizeMacroDistribution(v string) (string, bool) {
	if v == "" {
		return "balanced", true
	}
	if _, ok := validMacroDistributions[v]; ok {
		return v, true
	}
	return "", false
}

// validateBirthdate enforces a strict YYYY-MM-DD birthdate that is neither in the
// future nor more than 120 years ago. An empty birthdate is allowed (legacy).
func validateBirthdate(v string, now time.Time) error {
	if v == "" {
		return nil
	}
	if len(v) != 10 {
		return fmt.Errorf("birthdate must be in YYYY-MM-DD format")
	}
	bd, err := time.Parse("2006-01-02", v)
	if err != nil || bd.Format("2006-01-02") != v {
		return fmt.Errorf("birthdate must be in YYYY-MM-DD format")
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if bd.After(today) {
		return fmt.Errorf("birthdate cannot be in the future")
	}
	if bd.Before(today.AddDate(-120, 0, 0)) {
		return fmt.Errorf("birthdate cannot be more than 120 years ago")
	}
	return nil
}

// GET /v1/profile — return authenticated user's profile
func (h *ProfileHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	p, err := h.s.FetchProfile(r.Context(), claims.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "profile not found")
		return
	}
	if err != nil {
		slog.Error("fetch profile", "err", err)
		respond.Error(w, http.StatusInternalServerError, "profile fetch failed: "+err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, p)
}

// PUT /v1/profile — upsert profile
func (h *ProfileHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	var p models.Profile
	if !respond.Decode(w, r, &p) {
		return
	}
	p.UserID = claims.UserID
	p.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	if err := validateBirthdate(p.Birthdate, time.Now().UTC()); err != nil {
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	dist, ok := normalizeMacroDistribution(p.MacroDistribution)
	if !ok {
		respond.Error(w, http.StatusBadRequest, "macroDistribution must be one of keto, low_carb, balanced, high_carb")
		return
	}
	p.MacroDistribution = dist

	// Upsert via store
	if err := h.s.UpsertProfile(r.Context(), &p); err != nil {
		slog.Error("upsert profile", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}

	// Do NOT auto-compute or update macro targets here. Targets are managed via
	// onboarding, weekly check-ins, or manual overrides.
	respond.JSON(w, http.StatusOK, p)
}
