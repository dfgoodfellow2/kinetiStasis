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
	db := h.s.DB()
	if err := db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
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

	_, err = db.ExecContext(r.Context(),
		`INSERT INTO users (id, username, email, password, is_admin, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?)`,
		userID, req.Username, req.Email, hash, isAdmin, now, now,
	)
	if err != nil {
		respond.Error(w, http.StatusConflict, "username or email already exists")
		return
	}

	// Create empty profile row — non-fatal if it fails (user can set profile on first login)
	if _, err := db.ExecContext(r.Context(), `INSERT INTO profiles (user_id, updated_at) VALUES (?, ?)`, userID, now); err != nil {
		slog.Error("create empty profile on register", "user_id", userID, "err", err)
	}

	// Issue tokens
	if err := h.issueTokenPair(w, r, userID, isAdmin == 1); err != nil {
		respond.Error(w, http.StatusInternalServerError, "could not issue tokens")
		return
	}

	respond.JSON(w, http.StatusCreated, map[string]any{
		"user_id":  userID,
		"username": req.Username,
		"is_admin": isAdmin == 1,
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
		isAdmin  int
	)
	db := h.s.DB()
	err := db.QueryRowContext(r.Context(), `SELECT id, username, password, is_admin FROM users WHERE username = ? OR email = ? LIMIT 1`, req.Login, req.Login).Scan(&userID, &username, &hash, &isAdmin)
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

	if err := h.issueTokenPair(w, r, userID, isAdmin == 1); err != nil {
		respond.Error(w, http.StatusInternalServerError, "could not issue tokens")
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"user_id":  userID,
		"username": username,
		"is_admin": isAdmin == 1,
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
		isAdmin   int
		expiresAt string
	)
	db := h.s.DB()
	err = db.QueryRowContext(r.Context(),
		`SELECT u.id, u.is_admin, rt.expires_at
         FROM refresh_tokens rt
         JOIN users u ON u.id = rt.user_id
         WHERE rt.token_hash = ?`,
		tokenHash,
	).Scan(&userID, &isAdmin, &expiresAt)
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
		if _, err := h.s.DB().ExecContext(r.Context(), `DELETE FROM refresh_tokens WHERE token_hash = ?`, tokenHash); err != nil {
			slog.Warn("failed to delete expired refresh token", "err", err)
		}
		respond.Error(w, http.StatusUnauthorized, "refresh token expired")
		return
	}

	accessToken, err := auth.IssueAccessToken(h.cfg.JWTSecret, userID, isAdmin == 1)
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
		if _, err := h.s.DB().ExecContext(r.Context(), `DELETE FROM refresh_tokens WHERE token_hash = ?`, tokenHash); err != nil {
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
	db := h.s.DB()
	err := db.QueryRowContext(r.Context(), `SELECT username, email FROM users WHERE id = ?`, claims.UserID).Scan(&username, &email)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"user_id":  claims.UserID,
		"username": username,
		"email":    email,
		"is_admin": claims.IsAdmin,
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

	db := h.s.DB()
	_, err = db.ExecContext(r.Context(),
		`INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, created_at)
         VALUES (?, ?, ?, ?, ?)`,
		uuid.New().String(), userID, tokenHash,
		expires.Format(constants.TimeFormat),
		now.Format(constants.TimeFormat),
	)
	if err != nil {
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
