package handlers

import (
	"database/sql"
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

// GET /v1/profile — return authenticated user's profile
func (h *ProfileHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	p, err := h.s.FetchProfile(r.Context(), claims.UserID)
	if err == sql.ErrNoRows {
		respond.Error(w, http.StatusNotFound, "profile not found")
		return
	}
	if err != nil {
		slog.Error("profile fetch failed", "user_id", claims.UserID, "err", err)
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
	// Upsert via store
	if err := h.s.UpsertProfile(r.Context(), &p); err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}

	// Do NOT auto-compute or update macro targets here. Targets are managed via
	// onboarding, weekly check-ins, or manual overrides.
	respond.JSON(w, http.StatusOK, p)
}
