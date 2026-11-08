package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/auth"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/config"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/constants"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	respond "github.com/dfgoodfellow2/diet-tracker/v2/internal/respond"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// AuthHandler holds dependencies for auth endpoints.
type AuthHandler struct {
	s   store.Store
	cfg *config.Config
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(s store.Store, cfg *config.Config) *AuthHandler {
	return &AuthHandler{s: s, cfg: cfg}
}

// Register handles POST /v1/auth/register
// The first user to register is automatically made admin.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !respond.Decode(w, r, &req) {
		return
	}
	if req.Username == "" || req.Email == "" || req.Password == "" {
		respond.Error(w, http.StatusBadRequest, "username, email and password are required")
		return
	}
	if len(req.Password) < 8 {
		respond.Error(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	// Determine if this is the first user (auto-admin)
	var count int
	cnt, err := h.s.CountUsers(r.Context())
	if err != nil {
		slog.Error("count users during register", "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	count = cnt
	isAdmin := 0
	if count == 0 {
		isAdmin = 1
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		slog.Error("hash password", "err", err)
		respond.Error(w, http.StatusInternalServerError, "could not hash password")
		return
	}

	now := time.Now().UTC().Format(constants.TimeFormat)
	userID := uuid.New().String()

	if err := h.s.CreateUser(r.Context(), userID, req.Username, req.Email, hash, isAdmin == 1, now, now); err != nil {
		slog.Error("create user during register", "err", err)
		respond.Error(w, http.StatusConflict, "username or email already exists")
		return
	}

	// Create empty profile row — non-fatal if it fails (user can set profile on first login)
	if err := h.s.UpsertProfile(r.Context(), &models.Profile{UserID: userID, UpdatedAt: now}); err != nil {
		slog.Error("create empty profile on register", "user_id", userID, "err", err)
	}

	// Issue tokens
	if err := h.issueTokenPair(w, r, userID, isAdmin == 1); err != nil {
		slog.Error("issue token pair during register", "err", err)
		respond.Error(w, http.StatusInternalServerError, "could not issue tokens")
		return
	}

	respond.JSON(w, http.StatusCreated, map[string]any{
		"userId":   userID,
		"username": req.Username,
		"isAdmin":  isAdmin == 1,
	})
}

// Login handles POST /v1/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Login    string `json:"login"` // username or email
		Password string `json:"password"`
	}
	if !respond.Decode(w, r, &req) {
		return
	}
	if req.Login == "" || req.Password == "" {
		respond.Error(w, http.StatusBadRequest, "login and password are required")
		return
	}

	var (
		userID   string
		username string
		hash     string
		isAdmin  bool
		err      error
	)
	userID, username, hash, isAdmin, err = h.s.FindUserByLogin(r.Context(), req.Login)
	if errors.Is(err, sql.ErrNoRows) {
		respond.Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if err != nil {
		slog.Error("find user by login", "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := auth.CheckPassword(hash, req.Password); err != nil {
		respond.Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if err := h.issueTokenPair(w, r, userID, isAdmin); err != nil {
		slog.Error("issue token pair during login", "err", err)
		respond.Error(w, http.StatusInternalServerError, "could not issue tokens")
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"userId":   userID,
		"username": username,
		"isAdmin":  isAdmin,
	})
}

// Refresh handles POST /v1/auth/refresh
// Validates the refresh token cookie, issues a new access token.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(auth.RefreshCookieName)
	if err != nil {
		respond.Error(w, http.StatusUnauthorized, "missing refresh token")
		return
	}

	tokenHash := hashToken(cookie.Value)

	var (
		userID    string
		isAdmin   bool
		expiresAt string
	)
	userID, isAdmin, expiresAt, err = h.s.FindRefreshToken(r.Context(), tokenHash)
	if errors.Is(err, sql.ErrNoRows) {
		respond.Error(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}
	if err != nil {
		slog.Error("find refresh token", "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	exp, err := time.Parse(constants.TimeFormat, expiresAt)
	if err != nil || time.Now().After(exp) {
		// Clean up expired token
		if err := h.s.DeleteRefreshToken(r.Context(), tokenHash); err != nil {
			slog.Warn("failed to delete expired refresh token", "err", err)
		}
		respond.Error(w, http.StatusUnauthorized, "refresh token expired")
		return
	}

	accessToken, err := auth.IssueAccessToken(h.cfg.JWTSecret, userID, isAdmin)
	if err != nil {
		slog.Error("issue access token during refresh", "err", err)
		respond.Error(w, http.StatusInternalServerError, "could not issue token")
		return
	}
	auth.SetAccessCookie(w, accessToken, h.cfg.IsProd())

	respond.JSON(w, http.StatusOK, map[string]string{"status": "refreshed"})
}

// Logout handles POST /v1/auth/logout
// Deletes the refresh token from DB and clears cookies.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(auth.RefreshCookieName); err == nil {
		tokenHash := hashToken(cookie.Value)
		if err := h.s.DeleteRefreshToken(r.Context(), tokenHash); err != nil {
			slog.Warn("failed to delete refresh token on logout", "err", err)
		}
	}
	auth.ClearAuthCookies(w, h.cfg.IsProd())
	respond.JSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

// ChangePassword handles POST /v1/auth/password. It verifies the current
// password, stores a fresh bcrypt hash, revokes every refresh token for the
// user (all devices, including this one) and then issues a fresh token pair so
// the caller stays signed in.
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	if claims == nil {
		respond.Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var req struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if !respond.Decode(w, r, &req) {
		return
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		respond.Error(w, http.StatusBadRequest, "currentPassword and newPassword are required")
		return
	}

	hash, err := h.s.GetUserPasswordHash(r.Context(), claims.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		slog.Error("get password hash for change", "user_id", claims.UserID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// A wrong current password is an auth failure, not a server error — use
	// the same generic message as Login so the response reveals nothing.
	if err := auth.CheckPassword(hash, req.CurrentPassword); err != nil {
		respond.Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if len(req.NewPassword) < 8 {
		respond.Error(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	newHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		slog.Error("hash new password", "user_id", claims.UserID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "could not hash password")
		return
	}

	if err := h.s.UpdateUserPassword(r.Context(), claims.UserID, newHash); err != nil {
		slog.Error("update user password", "user_id", claims.UserID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Every existing refresh token dies, including the caller's. The fresh
	// pair issued below becomes the only live session for this device.
	if err := h.s.DeleteUserRefreshTokens(r.Context(), claims.UserID); err != nil {
		slog.Error("delete refresh tokens after password change", "user_id", claims.UserID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := h.issueTokenPair(w, r, claims.UserID, claims.IsAdmin); err != nil {
		slog.Error("issue token pair after password change", "user_id", claims.UserID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "could not issue tokens")
		return
	}

	slog.Info("password changed", "user_id", claims.UserID)
	respond.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// AdminResetPassword handles POST /v1/admin/users/{userID}/reset-password.
// It sets a random one-time temporary password, returns it exactly once, and
// revokes the target user's refresh tokens. The temporary password is never
// written to logs.
func (h *AuthHandler) AdminResetPassword(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	targetID := chi.URLParam(r, "userID")

	// Admins change their own password through the self-service flow; this
	// endpoint exists for household members who cannot sign in.
	if claims != nil && claims.UserID == targetID {
		respond.Error(w, http.StatusBadRequest, "use the change-password flow to change your own password")
		return
	}

	if _, err := h.s.GetUserByID(r.Context(), targetID); errors.Is(err, sql.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "user not found")
		return
	} else if err != nil {
		slog.Error("get user for admin password reset", "target_user_id", targetID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	tempPassword, err := generateTempPassword()
	if err != nil {
		slog.Error("generate temporary password", "target_user_id", targetID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "could not generate temporary password")
		return
	}

	hash, err := auth.HashPassword(tempPassword)
	if err != nil {
		slog.Error("hash temporary password", "target_user_id", targetID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "could not hash password")
		return
	}

	if err := h.s.UpdateUserPassword(r.Context(), targetID, hash); err != nil {
		slog.Error("update user password during admin reset", "target_user_id", targetID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := h.s.DeleteUserRefreshTokens(r.Context(), targetID); err != nil {
		slog.Error("delete refresh tokens during admin reset", "target_user_id", targetID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	adminID := ""
	if claims != nil {
		adminID = claims.UserID
	}
	slog.Info("password reset by admin", "admin_id", adminID, "target_user_id", targetID)

	respond.JSON(w, http.StatusOK, map[string]string{"tempPassword": tempPassword})
}

// tempPasswordLen is long enough to be a real password, short enough to read
// over the phone. It uses the same no-ambiguous alphabet as group invites.
const tempPasswordLen = 12

// generateTempPassword returns a cryptographically random temporary password.
func generateTempPassword() (string, error) {
	buf := make([]byte, tempPasswordLen)
	max := big.NewInt(int64(len(inviteAlphabet)))
	for i := range buf {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		buf[i] = inviteAlphabet[n.Int64()]
	}
	return string(buf), nil
}

// Me handles GET /v1/auth/me — returns current user info.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	if claims == nil {
		respond.Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var username, email string
	u, err := h.s.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		slog.Error("get user by id for /me", "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	username = u.Username
	email = u.Email

	respond.JSON(w, http.StatusOK, map[string]any{
		"userId":   claims.UserID,
		"username": username,
		"email":    email,
		"isAdmin":  claims.IsAdmin,
	})
}

// issueTokenPair creates and sets both access + refresh token cookies.
func (h *AuthHandler) issueTokenPair(w http.ResponseWriter, r *http.Request, userID string, isAdmin bool) error {
	accessToken, err := auth.IssueAccessToken(h.cfg.JWTSecret, userID, isAdmin)
	if err != nil {
		return fmt.Errorf("issue access token: %w", err)
	}

	refreshToken, err := auth.GenerateRefreshToken()
	if err != nil {
		return fmt.Errorf("generate refresh token: %w", err)
	}

	tokenHash := hashToken(refreshToken)
	now := time.Now().UTC()
	expires := now.Add(auth.RefreshTokenDuration)

	if err := h.s.SaveRefreshToken(r.Context(), uuid.New().String(), userID, tokenHash, expires.Format(constants.TimeFormat), now.Format(constants.TimeFormat)); err != nil {
		return fmt.Errorf("store refresh token: %w", err)
	}

	auth.SetAccessCookie(w, accessToken, h.cfg.IsProd())
	auth.SetRefreshCookie(w, refreshToken, h.cfg.IsProd())
	return nil
}

// hashToken creates a SHA-256 hex hash of a token for safe DB storage.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
