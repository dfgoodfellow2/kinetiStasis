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
	db := h.s.DB()
	rows, err := db.QueryContext(r.Context(), `SELECT id, username, email, is_admin, created_at FROM users ORDER BY created_at ASC`)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	type userRow struct {
		ID        string `json:"id"`
		Username  string `json:"username"`
		Email     string `json:"email"`
		IsAdmin   bool   `json:"is_admin"`
		CreatedAt string `json:"created_at"`
	}

	users := []userRow{}
	for rows.Next() {
		var u userRow
		var isAdmin int
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &isAdmin, &u.CreatedAt); err != nil {
			respond.Error(w, http.StatusInternalServerError, "scan error")
			return
		}
		u.IsAdmin = isAdmin == 1
		users = append(users, u)
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

	db := h.s.DB()
	_, err := db.ExecContext(r.Context(), `DELETE FROM users WHERE id = ?`, targetID)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// PromoteUser handles POST /v1/admin/users/{userID}/promote
func (h *AdminHandler) PromoteUser(w http.ResponseWriter, r *http.Request) {
	targetID := chi.URLParam(r, "userID")
	now := time.Now().UTC().Format(time.RFC3339)

	db := h.s.DB()
	_, err := db.ExecContext(r.Context(), `UPDATE users SET is_admin = 1, updated_at = ? WHERE id = ?`, now, targetID)
	if err != nil {
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
	var adminCount int
	if err := h.s.DB().QueryRowContext(r.Context(), `SELECT COUNT(*) FROM users WHERE is_admin = 1`).Scan(&adminCount); err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	if adminCount <= 1 {
		respond.Error(w, http.StatusBadRequest, "cannot demote the last admin")
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := h.s.DB().ExecContext(r.Context(), `UPDATE users SET is_admin = 0, updated_at = ? WHERE id = ?`, now, targetID)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "demoted"})
}
