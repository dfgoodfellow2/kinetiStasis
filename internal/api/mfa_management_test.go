package api_test

// 2FA management coverage: enrollment guards, confirm-without-commit,
// password+code disable, recovery-code rotation and QR rendering. All requests
// go through the real router and SQLite so wiring, auth middleware and
// persistence are exercised together.

import (
	"bytes"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/auth"
)

func recoveryCodeCount(t *testing.T, h *harness, userID string) int {
	t.Helper()
	var n int
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM recovery_codes WHERE user_id = ? AND used_at = ''`, userID).Scan(&n); err != nil {
		t.Fatalf("count recovery codes: %v", err)
	}
	return n
}

func trustedDeviceCount(t *testing.T, h *harness, userID string) int {
	t.Helper()
	var n int
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM trusted_devices WHERE user_id = ?`, userID).Scan(&n); err != nil {
		t.Fatalf("count trusted devices: %v", err)
	}
	return n
}

// TestMFASetupRejectedWhenEnabled proves setup is a one-shot enrollment door:
// an already-enabled account cannot generate a second pending secret.
func TestMFASetupRejectedWhenEnabled(t *testing.T) {
	h := newHarness(t)
	olivia := newAuthedClient(t, h, "olivia")
	enableMFA(t, h, olivia)

	status, body, _ := doRaw(t, h, olivia, http.MethodPost, "/v1/auth/mfa/setup", nil)
	if status != http.StatusBadRequest {
		t.Fatalf("setup on enabled account: status = %d, want 400, body = %s", status, body)
	}
	if !strings.Contains(string(body), "already enabled") {
		t.Fatalf("setup on enabled account body = %s, want already enabled", body)
	}
}

// TestMFAConfirmInvalidCodeDoesNotCommit proves a bad confirmation code stores
// nothing: the account stays disabled and a fresh setup is still possible.
func TestMFAConfirmInvalidCodeDoesNotCommit(t *testing.T) {
	h := newHarness(t)
	pete := newAuthedClient(t, h, "pete")
	peteID := userIDFor(t, h, "pete")

	setup := decodeJSON[struct {
		Secret string `json:"secret"`
	}](t, doAs(t, h, pete, http.MethodPost, "/v1/auth/mfa/setup", nil, http.StatusOK))
	if setup.Secret == "" {
		t.Fatal("setup returned an empty secret")
	}

	status, body, _ := doRaw(t, h, pete, http.MethodPost, "/v1/auth/mfa/confirm", map[string]any{
		"secret": setup.Secret,
		"code":   wrongTOTPCode(t, setup.Secret),
	})
	if status != http.StatusBadRequest {
		t.Fatalf("confirm with bad code: status = %d, want 400, body = %s", status, body)
	}

	// Nothing was committed.
	if got := totpSecretFor(t, h, peteID); got != "" {
		t.Fatalf("totp_secret after failed confirm = %q, want empty", got)
	}
	if n := recoveryCodeCount(t, h, peteID); n != 0 {
		t.Fatalf("recovery codes after failed confirm = %d, want 0", n)
	}

	// Setup is still available (an enabled account would get a 400) and the
	// full flow still works with a fresh secret.
	setup2 := decodeJSON[struct {
		Secret string `json:"secret"`
	}](t, doAs(t, h, pete, http.MethodPost, "/v1/auth/mfa/setup", nil, http.StatusOK))
	if setup2.Secret == "" {
		t.Fatal("re-setup returned an empty secret")
	}
	doAs(t, h, pete, http.MethodPost, "/v1/auth/mfa/confirm", map[string]any{
		"secret": setup2.Secret,
		"code":   totpCodeNow(t, setup2.Secret),
	}, http.StatusOK)
	if got := totpSecretFor(t, h, peteID); got != setup2.Secret {
		t.Fatalf("stored secret = %q, want confirmed secret", got)
	}
}

// TestMFADisableRejectsWrongPassword pins that the password gate fires before
// the second factor is checked, so 2FA cannot be removed with a stolen session
// alone.
//
// Note: the task sketch said 400, but the implemented handler (and the sibling
// ChangePassword endpoint) treats a wrong current password as an authentication
// failure, i.e. 401 "invalid credentials". The test asserts that behaviour.
func TestMFADisableRejectsWrongPassword(t *testing.T) {
	h := newHarness(t)
	quinn := newAuthedClient(t, h, "quinn")
	enrollment := enableMFA(t, h, quinn)
	quinnID := userIDFor(t, h, "quinn")

	status, body, _ := doRaw(t, h, quinn, http.MethodPost, "/v1/auth/mfa/disable", map[string]any{
		"currentPassword": "not-the-password",
		"code":            totpCodeNow(t, enrollment.Secret),
	})
	if status != http.StatusUnauthorized {
		t.Fatalf("disable with wrong password: status = %d, want 401, body = %s", status, body)
	}
	if !strings.Contains(string(body), "invalid credentials") {
		t.Fatalf("wrong-password body = %s, want invalid credentials", body)
	}
	if got := totpSecretFor(t, h, quinnID); got == "" {
		t.Fatal("failed disable cleared the totp secret")
	}
	if n := recoveryCodeCount(t, h, quinnID); n != auth.RecoveryCodeCount {
		t.Fatalf("recovery codes after failed disable = %d, want %d", n, auth.RecoveryCodeCount)
	}
}

// TestMFADisableWithPasswordAndCode covers the happy path and proves every
// piece of second-factor state is cleared: secret, recovery codes and devices.
func TestMFADisableWithPasswordAndCode(t *testing.T) {
	h := newHarness(t)
	rita := newAuthedClient(t, h, "rita")
	enrollment := enableMFA(t, h, rita)
	ritaID := userIDFor(t, h, "rita")

	doAs(t, h, rita, http.MethodPost, "/v1/auth/mfa/disable", map[string]any{
		"currentPassword": "supersecret123",
		"code":            totpCodeNow(t, enrollment.Secret),
	}, http.StatusOK)

	if got := totpSecretFor(t, h, ritaID); got != "" {
		t.Fatalf("totp_secret after disable = %q, want empty", got)
	}
	if n := recoveryCodeCount(t, h, ritaID); n != 0 {
		t.Fatalf("recovery codes after disable = %d, want 0", n)
	}
	if n := trustedDeviceCount(t, h, ritaID); n != 0 {
		t.Fatalf("trusted devices after disable = %d, want 0", n)
	}

	// Password-only login works again and issues a session immediately.
	fresh := newCookieClient(t)
	status, body, headers := postLogin(t, h, fresh, "rita", "supersecret123")
	if status != http.StatusOK {
		t.Fatalf("post-disable login: status = %d, body = %s", status, body)
	}
	if strings.Contains(string(body), "mfa_required") {
		t.Fatalf("post-disable login still required 2FA: %s", body)
	}
	out := decodeJSON[struct {
		UserID string `json:"userId"`
	}](t, body)
	if out.UserID != ritaID {
		t.Fatalf("post-disable login userId = %q, want %q", out.UserID, ritaID)
	}
	if !setCookieName(headers, auth.AccessCookieName) || !setCookieName(headers, auth.RefreshCookieName) {
		t.Fatalf("post-disable login did not issue session cookies: %v", headers.Values("Set-Cookie"))
	}
}

// TestMFARegenerateRecoveryCodesBurnsOldSet proves rotation invalidates the
// previous set at login-verify time while the new set works.
func TestMFARegenerateRecoveryCodesBurnsOldSet(t *testing.T) {
	h := newHarness(t)
	sam := newAuthedClient(t, h, "sam")
	enrollment := enableMFA(t, h, sam)
	samID := userIDFor(t, h, "sam")

	rotated := decodeJSON[struct {
		OK            bool     `json:"ok"`
		RecoveryCodes []string `json:"recoveryCodes"`
	}](t, doAs(t, h, sam, http.MethodPost, "/v1/auth/mfa/recovery-codes/regenerate", map[string]any{
		"code": totpCodeNow(t, enrollment.Secret),
	}, http.StatusOK))
	if !rotated.OK {
		t.Fatal("regenerate did not return ok=true")
	}
	if len(rotated.RecoveryCodes) != auth.RecoveryCodeCount {
		t.Fatalf("rotated code count = %d, want %d", len(rotated.RecoveryCodes), auth.RecoveryCodeCount)
	}
	if n := recoveryCodeCount(t, h, samID); n != auth.RecoveryCodeCount {
		t.Fatalf("live recovery codes after rotate = %d, want %d", n, auth.RecoveryCodeCount)
	}

	// Begin a login, then try an old code: it must fail and leave the challenge
	// usable; the new code then succeeds on the same challenge.
	fresh := newCookieClient(t)
	challenge := loginChallenge(t, h, fresh, "sam", "supersecret123")

	doAs(t, h, fresh, http.MethodPost, "/v1/auth/mfa/verify", map[string]any{
		"challenge": challenge,
		"code":      enrollment.RecoveryCodes[0],
	}, http.StatusUnauthorized)

	status, body, headers := doRaw(t, h, fresh, http.MethodPost, "/v1/auth/mfa/verify", map[string]any{
		"challenge": challenge,
		"code":      rotated.RecoveryCodes[0],
	})
	if status != http.StatusOK {
		t.Fatalf("new recovery code: status = %d, want 200, body = %s", status, body)
	}
	if !setCookieName(headers, auth.RefreshCookieName) {
		t.Fatalf("recovery-code login did not issue a session: %v", headers.Values("Set-Cookie"))
	}
}

// TestMFAQRReturnsPNG pins the content type and PNG magic of the enrollment
// QR route.
func TestMFAQRReturnsPNG(t *testing.T) {
	h := newHarness(t)
	tara := newAuthedClient(t, h, "tara")

	setup := decodeJSON[struct {
		Secret string `json:"secret"`
	}](t, doAs(t, h, tara, http.MethodPost, "/v1/auth/mfa/setup", nil, http.StatusOK))

	status, body, headers := doRaw(t, h, tara, http.MethodGet,
		"/v1/auth/mfa/qr?secret="+url.QueryEscape(setup.Secret), nil)
	if status != http.StatusOK {
		t.Fatalf("qr: status = %d, want 200, body = %s", status, body)
	}
	if got := headers.Get("Content-Type"); got != "image/png" {
		t.Fatalf("qr Content-Type = %q, want image/png", got)
	}
	if !bytes.HasPrefix(body, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		t.Fatalf("qr body is not a PNG (first bytes % x)", body[:min(8, len(body))])
	}
}
