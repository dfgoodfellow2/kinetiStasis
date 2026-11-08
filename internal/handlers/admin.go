package handlers

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/auth"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
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
		slog.Error("list users", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
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
		slog.Error("delete user", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// PromoteUser handles POST /v1/admin/users/{userID}/promote
func (h *AdminHandler) PromoteUser(w http.ResponseWriter, r *http.Request) {
	targetID := chi.URLParam(r, "userID")
	now := time.Now().UTC().Format(time.RFC3339)

	if err := h.s.PromoteUser(r.Context(), targetID, now); err != nil {
		slog.Error("promote user", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
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
		slog.Error("count admins", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	if adminCount <= 1 {
		respond.Error(w, http.StatusBadRequest, "cannot demote the last admin")
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if err := h.s.DemoteUser(r.Context(), targetID, now); err != nil {
		slog.Error("demote user", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "demoted"})
}

// --- Registration gate (admin) ----------------------------------------------

// CreateRegistrationInvite handles POST /v1/admin/register-invites. It mints a
// single-use, 7-day code from the same no-ambiguous alphabet as group invites.
// The raw code is returned once and deliberately never logged.
func (h *AdminHandler) CreateRegistrationInvite(w http.ResponseWriter, r *http.Request) {
	code, err := generateInviteCode()
	if err != nil {
		slog.Error("generate registration invite code", "err", err)
		respond.Error(w, http.StatusInternalServerError, "unable to create invite")
		return
	}

	adminID := ""
	if claims := auth.ClaimsFromCtx(r); claims != nil {
		adminID = claims.UserID
	}
	now := time.Now().UTC()
	inv := &models.RegistrationInvite{
		Code:      code,
		CreatedBy: adminID,
		ExpiresAt: now.Add(inviteTTL).Format(time.RFC3339),
		CreatedAt: now.Format(time.RFC3339),
	}
	if err := h.s.CreateRegistrationInvite(r.Context(), inv); err != nil {
		slog.Error("create registration invite", "admin_id", adminID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "unable to create invite")
		return
	}
	slog.Info("registration invite created", "admin_id", adminID)

	respond.JSON(w, http.StatusCreated, map[string]string{
		"code":      inv.Code,
		"expiresAt": inv.ExpiresAt,
	})
}

// ListRegistrationInvites handles GET /v1/admin/register-invites.
func (h *AdminHandler) ListRegistrationInvites(w http.ResponseWriter, r *http.Request) {
	invites, err := h.s.ListRegistrationInvites(r.Context())
	if err != nil {
		slog.Error("list registration invites", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	if invites == nil {
		invites = []models.RegistrationInvite{}
	}
	respond.JSON(w, http.StatusOK, invites)
}

// RevokeRegistrationInvite handles DELETE /v1/admin/register-invites/{code}.
func (h *AdminHandler) RevokeRegistrationInvite(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(strings.TrimSpace(chi.URLParam(r, "code")))
	if code == "" {
		respond.Error(w, http.StatusBadRequest, "invite code is required")
		return
	}
	if err := h.s.RevokeRegistrationInvite(r.Context(), code); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "invite not found")
			return
		}
		slog.Error("revoke registration invite", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// ListPendingUsers handles GET /v1/admin/pending-users.
func (h *AdminHandler) ListPendingUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.s.ListPendingUsers(r.Context())
	if err != nil {
		slog.Error("list pending users", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	if users == nil {
		users = []models.User{}
	}
	respond.JSON(w, http.StatusOK, users)
}

// ApproveUser handles POST /v1/admin/users/{userID}/approve. It clears the
// pending flag so the account can log in.
func (h *AdminHandler) ApproveUser(w http.ResponseWriter, r *http.Request) {
	targetID := chi.URLParam(r, "userID")
	if err := h.s.SetUserPendingApproval(r.Context(), targetID, false); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "user not found")
			return
		}
		slog.Error("approve user", "target_user_id", targetID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	slog.Info("pending user approved", "target_user_id", targetID)
	respond.JSON(w, http.StatusOK, map[string]string{"status": "approved"})
}

// GetRegistrationMode handles GET /v1/admin/registration-mode.
func (h *AdminHandler) GetRegistrationMode(w http.ResponseWriter, r *http.Request) {
	mode := RegistrationModeInviteOnly
	stored, err := h.s.GetSetting(r.Context(), registrationModeSetting)
	switch {
	case err == nil:
		if validRegistrationMode(stored) {
			mode = stored
		}
	case errors.Is(err, sql.ErrNoRows):
	default:
		slog.Error("get registration mode", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"mode": mode})
}

// SetRegistrationMode handles PUT /v1/admin/registration-mode.
func (h *AdminHandler) SetRegistrationMode(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Mode string `json:"mode"`
	}
	if !respond.Decode(w, r, &in) {
		return
	}
	mode := strings.TrimSpace(in.Mode)
	if !validRegistrationMode(mode) {
		respond.Error(w, http.StatusBadRequest, "mode must be one of invite_only, public, public_approval")
		return
	}
	if err := h.s.AddSetting(r.Context(), registrationModeSetting, mode); err != nil {
		slog.Error("set registration mode", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	slog.Info("registration mode changed", "mode", mode)
	respond.JSON(w, http.StatusOK, map[string]string{"mode": mode})
}
