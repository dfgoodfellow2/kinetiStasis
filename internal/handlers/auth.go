package handlers

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/auth"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/config"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/constants"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	respond "github.com/dfgoodfellow2/diet-tracker/v2/internal/respond"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"
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
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	count = cnt
	isAdmin := 0
	if count == 0 {
		isAdmin = 1
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "could not hash password")
		return
	}

	now := time.Now().UTC().Format(constants.TimeFormat)
	userID := uuid.New().String()

	if err := h.s.CreateUser(r.Context(), userID, req.Username, req.Email, hash, isAdmin == 1, now, now); err != nil {
		respond.Error(w, http.StatusConflict, "username or email already exists")
		return
	}

	// Create empty profile row — non-fatal if it fails (user can set profile on first login)
	if err := h.s.UpsertProfile(r.Context(), &models.Profile{UserID: userID, UpdatedAt: now}); err != nil {
		slog.Error("create empty profile on register", "user_id", userID, "err", err)
	}

	// Issue tokens
	if err := h.issueTokenPair(w, r, userID, isAdmin == 1); err != nil {
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
	if err == sql.ErrNoRows {
		respond.Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}

	if err := auth.CheckPassword(hash, req.Password); err != nil {
		respond.Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if err := h.issueTokenPair(w, r, userID, isAdmin); err != nil {
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
	if err == sql.ErrNoRows {
		respond.Error(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
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
	auth.ClearAuthCookies(w)
	respond.JSON(w, http.StatusOK, map[string]string{"status": "logged out"})
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
		respond.Error(w, http.StatusInternalServerError, "database error")
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
