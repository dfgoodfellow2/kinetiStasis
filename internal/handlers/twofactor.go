package handlers

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/auth"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/constants"
	respond "github.com/dfgoodfellow2/diet-tracker/v2/internal/respond"
	"github.com/google/uuid"
	qrcode "github.com/skip2/go-qrcode"
)

// Two-factor authentication HTTP surface.
//
// Lockout-safety contract:
//   - Setup never stores anything. The secret is committed only by Confirm,
//     after a live code proves the authenticator is working.
//   - Enabling returns the recovery codes exactly once; all are hashed at rest.
//   - Disable requires BOTH the current password and a live TOTP/recovery code.
//   - Break-glass lives in cmd/reset-2fa and never touches the password.

// trustedDeviceValid reports whether the request carries a dt_device cookie
// that is bound to userID and unexpired. Any lookup error fails closed, which
// means the caller is challenged for a code instead of being admitted.
func (h *AuthHandler) trustedDeviceValid(r *http.Request, userID string) bool {
	cookie, err := r.Cookie(auth.TrustedDeviceCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}

	deviceHash := auth.HashOpaqueToken(cookie.Value)
	expiresAt, err := h.s.FindTrustedDevice(r.Context(), userID, deviceHash)
	if errors.Is(err, sql.ErrNoRows) {
		return false
	}
	if err != nil {
		slog.Warn("lookup trusted device during login", "user_id", userID, "err", err)
		return false
	}

	exp, err := time.Parse(constants.TimeFormat, expiresAt)
	if err != nil || time.Now().After(exp) {
		if derr := h.s.DeleteTrustedDevice(r.Context(), userID, deviceHash); derr != nil {
			slog.Warn("delete expired trusted device", "user_id", userID, "err", derr)
		}
		return false
	}

	if err := h.s.TouchTrustedDevice(r.Context(), userID, deviceHash, time.Now().UTC().Format(constants.TimeFormat)); err != nil {
		slog.Warn("touch trusted device", "user_id", userID, "err", err)
	}
	return true
}

// MFAVerify handles POST /v1/auth/mfa/verify.
//
// It exchanges a single-use challenge (issued by Login after the password
// check) plus a TOTP or recovery code for a real cookie pair. A wrong code
// leaves the challenge intact until its 5-minute expiry so a mistyped digit is
// not a lockout; a correct code consumes it immediately.
func (h *AuthHandler) MFAVerify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Challenge      string `json:"challenge"`
		Code           string `json:"code"`
		RememberDevice bool   `json:"rememberDevice"`
	}
	if !respond.Decode(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Challenge) == "" || strings.TrimSpace(req.Code) == "" {
		respond.Error(w, http.StatusBadRequest, "challenge and code are required")
		return
	}

	challengeHash := auth.HashOpaqueToken(req.Challenge)
	userID, expiresAt, err := h.s.FindLoginChallenge(r.Context(), challengeHash)
	if errors.Is(err, sql.ErrNoRows) {
		respond.Error(w, http.StatusUnauthorized, "invalid or expired challenge")
		return
	}
	if err != nil {
		slog.Error("find login challenge", "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	exp, err := time.Parse(constants.TimeFormat, expiresAt)
	if err != nil || time.Now().After(exp) {
		if derr := h.s.DeleteLoginChallenge(r.Context(), challengeHash); derr != nil {
			slog.Warn("delete expired login challenge", "user_id", userID, "err", derr)
		}
		respond.Error(w, http.StatusUnauthorized, "invalid or expired challenge")
		return
	}

	user, err := h.s.GetUserByID(r.Context(), userID)
	if errors.Is(err, sql.ErrNoRows) {
		respond.Error(w, http.StatusUnauthorized, "invalid or expired challenge")
		return
	}
	if err != nil {
		slog.Error("get user during mfa verify", "user_id", userID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	secret, err := h.s.GetUserTOTPSecret(r.Context(), userID)
	if err != nil {
		slog.Error("read totp secret during mfa verify", "user_id", userID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if secret == "" {
		// 2FA was disabled after this challenge was minted; it is no longer
		// meaningful. Force a fresh login rather than silently bypassing.
		if derr := h.s.DeleteLoginChallenge(r.Context(), challengeHash); derr != nil {
			slog.Warn("delete orphaned login challenge", "user_id", userID, "err", derr)
		}
		respond.Error(w, http.StatusUnauthorized, "invalid or expired challenge")
		return
	}

	now := time.Now().UTC()
	verified := auth.ValidateTOTP(secret, req.Code)
	if !verified {
		burned, berr := h.s.BurnRecoveryCode(r.Context(), userID, auth.HashRecoveryCode(req.Code), now.Format(constants.TimeFormat))
		if berr != nil {
			slog.Error("burn recovery code during mfa verify", "user_id", userID, "err", berr)
			respond.Error(w, http.StatusInternalServerError, "internal server error")
			return
		}
		verified = burned
	}
	if !verified {
		respond.Error(w, http.StatusUnauthorized, "invalid code")
		return
	}

	// Consume the challenge before issuing any session so a replay cannot mint
	// a second pair of cookies.
	if err := h.s.DeleteLoginChallenge(r.Context(), challengeHash); err != nil {
		slog.Error("consume login challenge", "user_id", userID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := h.issueTokenPair(w, r, userID, user.IsAdmin); err != nil {
		slog.Error("issue token pair during mfa verify", "user_id", userID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "could not issue tokens")
		return
	}

	if req.RememberDevice {
		h.rememberDevice(w, r, userID)
	}

	slog.Info("mfa login verified", "user_id", userID, "remember_device", req.RememberDevice)
	respond.JSON(w, http.StatusOK, map[string]any{
		"userId":   userID,
		"username": user.Username,
		"isAdmin":  user.IsAdmin,
	})
}

// rememberDevice stores and sets a trusted-device token. A persistence error
// is logged but does NOT fail the already-successful login.
func (h *AuthHandler) rememberDevice(w http.ResponseWriter, r *http.Request, userID string) {
	token, err := auth.GenerateDeviceToken()
	if err != nil {
		slog.Warn("generate device token", "user_id", userID, "err", err)
		return
	}

	now := time.Now().UTC()
	if err := h.s.CreateTrustedDevice(
		r.Context(),
		uuid.NewString(),
		userID,
		auth.HashOpaqueToken(token),
		now.Format(constants.TimeFormat),
		now.Add(auth.TrustedDeviceTTL).Format(constants.TimeFormat),
	); err != nil {
		slog.Warn("store trusted device", "user_id", userID, "err", err)
		return
	}
	auth.SetTrustedDeviceCookie(w, token, h.cfg.IsProd())
}

// MFASetup handles POST /v1/auth/mfa/setup.
// It returns a pending secret and enrollment URL. Nothing is committed until
// Confirm receives a live code, so an abandoned setup cannot lock anyone out.
func (h *AuthHandler) MFASetup(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	if claims == nil {
		respond.Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	secret, err := h.s.GetUserTOTPSecret(r.Context(), claims.UserID)
	if err != nil {
		slog.Error("read totp secret during setup", "user_id", claims.UserID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if secret != "" {
		respond.Error(w, http.StatusBadRequest, "already enabled")
		return
	}

	user, err := h.s.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		slog.Error("get user during setup", "user_id", claims.UserID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	pendingSecret, otpauthURL, err := auth.GenerateTOTPKey(user.Email)
	if err != nil {
		slog.Error("generate totp key", "user_id", claims.UserID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "could not generate secret")
		return
	}

	respond.JSON(w, http.StatusOK, map[string]string{
		"secret":     pendingSecret,
		"otpauthUrl": otpauthURL,
	})
}

// MFAConfirm handles POST /v1/auth/mfa/confirm.
// The supplied secret is committed only when code validates against it; the
// response carries the ten recovery codes exactly once.
func (h *AuthHandler) MFAConfirm(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	if claims == nil {
		respond.Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var req struct {
		Secret string `json:"secret"`
		Code   string `json:"code"`
	}
	if !respond.Decode(w, r, &req) {
		return
	}
	secret := strings.TrimSpace(req.Secret)
	if secret == "" || strings.TrimSpace(req.Code) == "" {
		respond.Error(w, http.StatusBadRequest, "secret and code are required")
		return
	}

	existing, err := h.s.GetUserTOTPSecret(r.Context(), claims.UserID)
	if err != nil {
		slog.Error("read totp secret during confirm", "user_id", claims.UserID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if existing != "" {
		respond.Error(w, http.StatusBadRequest, "already enabled")
		return
	}

	if !auth.ValidateTOTP(secret, req.Code) {
		respond.Error(w, http.StatusBadRequest, "invalid code")
		return
	}

	codes, err := auth.GenerateRecoveryCodes()
	if err != nil {
		slog.Error("generate recovery codes", "user_id", claims.UserID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "could not enable 2FA")
		return
	}
	hashes := make([]string, len(codes))
	for i, code := range codes {
		hashes[i] = auth.HashRecoveryCode(code)
	}

	now := time.Now().UTC().Format(constants.TimeFormat)
	if err := h.s.EnableTOTP(r.Context(), claims.UserID, secret, hashes, now); err != nil {
		slog.Error("enable totp", "user_id", claims.UserID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "could not enable 2FA")
		return
	}

	// Codes are returned to the client's own response, never logged.
	slog.Info("mfa enabled", "user_id", claims.UserID)
	respond.JSON(w, http.StatusOK, map[string]any{
		"ok":            true,
		"recoveryCodes": codes,
	})
}

// MFADisable handles POST /v1/auth/mfa/disable.
// Both the account password and a live TOTP or recovery code are required.
func (h *AuthHandler) MFADisable(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	if claims == nil {
		respond.Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var req struct {
		CurrentPassword string `json:"currentPassword"`
		Code            string `json:"code"`
	}
	if !respond.Decode(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.CurrentPassword) == "" || strings.TrimSpace(req.Code) == "" {
		respond.Error(w, http.StatusBadRequest, "currentPassword and code are required")
		return
	}

	passwordHash, err := h.s.GetUserPasswordHash(r.Context(), claims.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		slog.Error("get password hash during mfa disable", "user_id", claims.UserID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	// A wrong account password is authentication failure, not a malformed
	// request: return 401 "invalid credentials" to stay consistent with
	// ChangePassword (and to avoid leaking which field was wrong).
	if err := auth.CheckPassword(passwordHash, req.CurrentPassword); err != nil {
		respond.Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	secret, err := h.s.GetUserTOTPSecret(r.Context(), claims.UserID)
	if err != nil {
		slog.Error("read totp secret during disable", "user_id", claims.UserID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if secret == "" {
		respond.Error(w, http.StatusBadRequest, "2FA is not enabled")
		return
	}

	if !h.consumeSecondFactor(r, claims.UserID, secret, req.Code) {
		respond.Error(w, http.StatusUnauthorized, "invalid code")
		return
	}

	now := time.Now().UTC().Format(constants.TimeFormat)
	if err := h.s.DisableTOTP(r.Context(), claims.UserID, now); err != nil {
		slog.Error("disable totp", "user_id", claims.UserID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "could not disable 2FA")
		return
	}
	auth.ClearTrustedDeviceCookie(w, h.cfg.IsProd())

	slog.Info("mfa disabled", "user_id", claims.UserID)
	respond.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// MFARegenerateRecoveryCodes handles POST /v1/auth/mfa/recovery-codes/regenerate.
// A live second factor is required so a stolen session cannot rotate the
// backup codes. Every existing code is burned and replaced.
func (h *AuthHandler) MFARegenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	if claims == nil {
		respond.Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if !respond.Decode(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Code) == "" {
		respond.Error(w, http.StatusBadRequest, "code is required")
		return
	}

	secret, err := h.s.GetUserTOTPSecret(r.Context(), claims.UserID)
	if err != nil {
		slog.Error("read totp secret during regenerate", "user_id", claims.UserID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if secret == "" {
		respond.Error(w, http.StatusBadRequest, "2FA is not enabled")
		return
	}

	if !h.consumeSecondFactor(r, claims.UserID, secret, req.Code) {
		respond.Error(w, http.StatusUnauthorized, "invalid code")
		return
	}

	codes, err := auth.GenerateRecoveryCodes()
	if err != nil {
		slog.Error("generate recovery codes", "user_id", claims.UserID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "could not regenerate codes")
		return
	}
	hashes := make([]string, len(codes))
	for i, code := range codes {
		hashes[i] = auth.HashRecoveryCode(code)
	}
	now := time.Now().UTC().Format(constants.TimeFormat)
	if err := h.s.ReplaceRecoveryCodes(r.Context(), claims.UserID, hashes, now); err != nil {
		slog.Error("replace recovery codes", "user_id", claims.UserID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "could not regenerate codes")
		return
	}

	slog.Info("mfa recovery codes regenerated", "user_id", claims.UserID)
	respond.JSON(w, http.StatusOK, map[string]any{
		"ok":            true,
		"recoveryCodes": codes,
	})
}

// MFAQR handles GET /v1/auth/mfa/qr?secret=<pending>&account=<email>.
// It renders the otpauth:// URL as an inline PNG. The pending secret is never
// persisted by this route.
func (h *AuthHandler) MFAQR(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	if claims == nil {
		respond.Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	secret := strings.TrimSpace(r.URL.Query().Get("secret"))
	if secret == "" {
		respond.Error(w, http.StatusBadRequest, "secret is required")
		return
	}

	account := strings.TrimSpace(r.URL.Query().Get("account"))
	if account == "" {
		user, err := h.s.GetUserByID(r.Context(), claims.UserID)
		if err != nil {
			slog.Error("get user during qr", "user_id", claims.UserID, "err", err)
			respond.Error(w, http.StatusInternalServerError, "internal server error")
			return
		}
		account = user.Email
	}

	otpauthURL, err := auth.TOTPURL(secret, account)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	png, err := qrcode.Encode(otpauthURL, qrcode.Medium, 256)
	if err != nil {
		slog.Error("encode mfa qr", "user_id", claims.UserID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "could not render QR")
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(png); err != nil {
		slog.Warn("write mfa qr", "user_id", claims.UserID, "err", err)
	}
}

// consumeSecondFactor validates a TOTP code or, failing that, burns a live
// recovery code. It returns true when either path succeeded.
func (h *AuthHandler) consumeSecondFactor(r *http.Request, userID, secret, code string) bool {
	if auth.ValidateTOTP(secret, code) {
		return true
	}
	burned, err := h.s.BurnRecoveryCode(
		r.Context(),
		userID,
		auth.HashRecoveryCode(code),
		time.Now().UTC().Format(constants.TimeFormat),
	)
	if err != nil {
		slog.Error("burn recovery code", "user_id", userID, "err", err)
		return false
	}
	return burned
}
