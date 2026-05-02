package handlers

import (
	"net/http"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/auth"
	respond "github.com/dfgoodfellow2/diet-tracker/v2/internal/respond"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"
	"github.com/go-chi/chi/v5"
)

// AdminHandler holds dependencies for admin endpoints.
type AdminHandler struct {
	s store.Store
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(s store.Store) *AdminHandler {
	return &AdminHandler{s: s}
}

// ListUsers handles GET /v1/admin/users
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.s.ListUsers(r.Context())
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	respond.JSON(w, http.StatusOK, users)
}

// DeleteUser handles DELETE /v1/admin/users/{userID}
func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	targetID := chi.URLParam(r, "userID")
	claims := auth.ClaimsFromCtx(r)

	// Prevent self-deletion
	if claims != nil && claims.UserID == targetID {
		respond.Error(w, http.StatusBadRequest, "cannot delete your own account")
		return
	}

	if err := h.s.DeleteUser(r.Context(), targetID); err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// PromoteUser handles POST /v1/admin/users/{userID}/promote
func (h *AdminHandler) PromoteUser(w http.ResponseWriter, r *http.Request) {
	targetID := chi.URLParam(r, "userID")
	now := time.Now().UTC().Format(time.RFC3339)

	if err := h.s.PromoteUser(r.Context(), targetID, now); err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "promoted"})
}

// DemoteUser handles POST /v1/admin/users/{userID}/demote
func (h *AdminHandler) DemoteUser(w http.ResponseWriter, r *http.Request) {
	targetID := chi.URLParam(r, "userID")
	claims := auth.ClaimsFromCtx(r)

	// Prevent self-demotion
	if claims != nil && claims.UserID == targetID {
		respond.Error(w, http.StatusBadRequest, "cannot demote yourself")
		return
	}

	// Ensure at least one admin remains
	adminCount, err := h.s.CountAdmins(r.Context())
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	if adminCount <= 1 {
		respond.Error(w, http.StatusBadRequest, "cannot demote the last admin")
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if err := h.s.DemoteUser(r.Context(), targetID, now); err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "demoted"})
}
