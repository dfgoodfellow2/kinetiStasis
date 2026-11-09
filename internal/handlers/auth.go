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
	"strings"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/auth"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/config"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/constants"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	respond "github.com/dfgoodfellow2/diet-tracker/v2/internal/respond"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/validate"
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

// Registration modes stored in instance_settings under "registration_mode".
const (
	RegistrationModeInviteOnly     = "invite_only"
	RegistrationModePublic         = "public"
	RegistrationModePublicApproval = "public_approval"

	registrationModeSetting = "registration_mode"
)

// validRegistrationMode reports whether mode is one of the three supported
// registration postures.
func validRegistrationMode(mode string) bool {
	switch mode {
	case RegistrationModeInviteOnly, RegistrationModePublic, RegistrationModePublicApproval:
		return true
	default:
		return false
	}
}

// RegistrationInfo handles GET /v1/auth/registration-info. It is public so the
// register form knows whether to ask for an invite code and whether a new
// account will need approval.
func (h *AuthHandler) RegistrationInfo(w http.ResponseWriter, r *http.Request) {
	mode := RegistrationModeInviteOnly
	stored, err := h.s.GetSetting(r.Context(), registrationModeSetting)
	switch {
	case err == nil:
		if validRegistrationMode(stored) {
			mode = stored
		}
	case errors.Is(err, sql.ErrNoRows):
		// Legacy database without the seed row: fail closed.
	default:
		slog.Error("read registration mode", "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"mode":            mode,
		"pendingApproval": mode == RegistrationModePublicApproval,
	})
}

// Register handles POST /v1/auth/register.
//
// Gate rules, in order:
//  1. Zero users on the instance: registration is always allowed and the first
//     account becomes the admin/owner. This preserves the bootstrap flow.
//  2. invite_only (default): a valid, unused, unexpired inviteCode is required.
//  3. public: any registration is accepted immediately.
//  4. public_approval: the account row is created pending approval and Login
//     refuses to issue tokens until an admin approves it.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username   string `json:"username"`
		Email      string `json:"email"`
		Password   string `json:"password"`
		InviteCode string `json:"inviteCode"`
	}
	if !respond.Decode(w, r, &req) {
		return
	}
	if req.Username == "" || req.Email == "" || req.Password == "" {
		respond.Error(w, http.StatusBadRequest, "username, email and password are required")
		return
	}
	if err := validate.Username(req.Username); err != nil {
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validate.Email(req.Email); err != nil {
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.Password) < 8 {
		respond.Error(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	count, err := h.s.CountUsers(r.Context())
	if err != nil {
		slog.Error("count users during register", "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	bootstrap := count == 0

	mode := RegistrationModeInviteOnly
	if !bootstrap {
		stored, err := h.s.GetSetting(r.Context(), registrationModeSetting)
		switch {
		case err == nil:
			if validRegistrationMode(stored) {
				mode = stored
			}
		case errors.Is(err, sql.ErrNoRows):
			// Missing seed: keep the closed default.
		default:
			slog.Error("read registration mode", "err", err)
			respond.Error(w, http.StatusInternalServerError, "internal server error")
			return
		}
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		slog.Error("hash password", "err", err)
		respond.Error(w, http.StatusInternalServerError, "could not hash password")
		return
	}

	now := time.Now().UTC().Format(constants.TimeFormat)
	userID := uuid.New().String()

	// Invite-only gate. The code is consumed before the user row is created so
	// two concurrent registrations cannot both spend it. If CreateUser then
	// fails, the burn is compensated back below, so a failed attempt does not
	// cost the caller their code.
	var consumedInviteCode string
	if !bootstrap && mode == RegistrationModeInviteOnly {
		code := strings.ToUpper(strings.TrimSpace(req.InviteCode))
		if code == "" {
			respond.Error(w, http.StatusForbidden, "registration requires a valid invite code")
			return
		}
		if err := h.s.ConsumeRegistrationInvite(r.Context(), code, userID, now); err != nil {
			switch {
			case errors.Is(err, store.ErrInviteInvalid),
				errors.Is(err, store.ErrInviteUsed),
				errors.Is(err, store.ErrInviteExpired):
				slog.Warn("registration invite rejected", "reason", err)
				respond.Error(w, http.StatusForbidden, "registration requires a valid invite code")
			default:
				slog.Error("consume registration invite", "err", err)
				respond.Error(w, http.StatusInternalServerError, "internal server error")
			}
			return
		}
		consumedInviteCode = code
	}

	pending := !bootstrap && mode == RegistrationModePublicApproval
	isAdmin := bootstrap

	if err := h.s.CreateUser(r.Context(), userID, req.Username, req.Email, hash, isAdmin, pending, now, now); err != nil {
		slog.Error("create user during register", "err", err)
		// The invite was burned for this exact attempt before the insert. If the
		// insert failed (duplicate username/email, constraint, etc.) release it
		// so the caller can retry with the same code. Compensation only clears
		// the burn when used_by still matches this userID, so a concurrent
		// winner's code is never released. A compensation failure is logged but
		// must not mask the original conflict.
		if consumedInviteCode != "" {
			if cerr := h.s.CompensateRegistrationInvite(r.Context(), consumedInviteCode, userID); cerr != nil {
				slog.Error("compensate registration invite after create failure", "user_id", userID, "err", cerr)
			}
		}
		respond.Error(w, http.StatusConflict, "username or email already exists")
		return
	}

	// Pending accounts get no profile, no tokens and no session.
	if pending {
		slog.Info("registration pending approval", "user_id", userID)
		respond.JSON(w, http.StatusCreated, map[string]string{
			"status":  "pending",
			"message": "account is awaiting approval",
		})
		return
	}

	// Create empty profile row — non-fatal if it fails (user can set profile on first login)
	if err := h.s.UpsertProfile(r.Context(), &models.Profile{UserID: userID, UpdatedAt: now}); err != nil {
		slog.Error("create empty profile on register", "user_id", userID, "err", err)
	}

	// Issue tokens
	if err := h.issueTokenPair(w, r, userID, isAdmin); err != nil {
		slog.Error("issue token pair during register", "err", err)
		respond.Error(w, http.StatusInternalServerError, "could not issue tokens")
		return
	}

	respond.JSON(w, http.StatusCreated, map[string]any{
		"userId":   userID,
		"username": req.Username,
		"isAdmin":  isAdmin,
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

	// Accounts created under public_approval exist but cannot sign in until an
	// admin approves them. Reject before minting any tokens or refresh rows.
	pending, err := h.s.IsUserPendingApproval(r.Context(), userID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Error("check pending approval during login", "user_id", userID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if pending {
		respond.Error(w, http.StatusForbidden, "account is awaiting approval")
		return
	}

	// Opt-in second factor. A remembered device (valid dt_device cookie bound
	// to this user and unexpired) skips the code and receives a full session.
	// Otherwise password success alone issues NO cookies or tokens: the caller
	// must complete POST /v1/auth/mfa/verify with a TOTP or recovery code.
	secret, err := h.s.GetUserTOTPSecret(r.Context(), userID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Error("read totp secret during login", "user_id", userID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if secret != "" && !h.trustedDeviceValid(r, userID) {
		challenge, err := auth.GenerateChallengeToken()
		if err != nil {
			slog.Error("generate mfa challenge", "user_id", userID, "err", err)
			respond.Error(w, http.StatusInternalServerError, "could not start verification")
			return
		}
		now := time.Now().UTC()
		// Best-effort shed of this user's dead challenges; a failure must not
		// block a legitimate login.
		if err := h.s.DeleteExpiredLoginChallenges(r.Context(), userID, now.Format(constants.TimeFormat)); err != nil {
			slog.Warn("delete expired login challenges", "user_id", userID, "err", err)
		}
		if err := h.s.CreateLoginChallenge(r.Context(), auth.HashOpaqueToken(challenge), userID, now.Add(auth.MFAChallengeTTL).Format(constants.TimeFormat)); err != nil {
			slog.Error("create mfa challenge", "user_id", userID, "err", err)
			respond.Error(w, http.StatusInternalServerError, "could not start verification")
			return
		}
		respond.JSON(w, http.StatusOK, map[string]any{
			"status":    "mfa_required",
			"challenge": challenge,
		})
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
		// Reject first, then lazily shed every expired row this user has
		// accumulated. Deleting by user_id + expiry keeps live sessions intact
		// and avoids relying on the expired token being presented again.
		if err := h.s.DeleteRefreshToken(r.Context(), tokenHash); err != nil {
			slog.Warn("failed to delete expired refresh token", "user_id", userID, "err", err)
		}
		if err := h.s.DeleteExpiredRefreshTokens(r.Context(), userID); err != nil {
			slog.Warn("failed to delete expired refresh tokens", "user_id", userID, "err", err)
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

	totpSecret, err := h.s.GetUserTOTPSecret(r.Context(), claims.UserID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Error("read totp status for /me", "user_id", claims.UserID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"userId":     claims.UserID,
		"username":   username,
		"email":      email,
		"isAdmin":    claims.IsAdmin,
		"mfaEnabled": totpSecret != "",
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
