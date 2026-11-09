package api_test

// Two-step login coverage for opt-in TOTP 2FA, exercised through the real
// router, cookie middleware and SQLite. The suite pins the security contract:
//
//   - a correct password alone issues NO cookies and NO session rows when the
//     account has 2FA enabled; it only returns a single-use challenge
//   - a wrong code is a 401 but leaves the challenge reusable until it expires
//     or a code succeeds (single-consumption on success only)
//   - a correct TOTP code issues the normal cookie pair and /auth/me now
//     reports mfaEnabled=true
//   - a fabricated/expired challenge is rejected; a fresh login produces a
//     working challenge again
//   - rememberDevice sets dt_device and the next login with that cookie skips
//     the challenge entirely
//   - the verify endpoint shares the password-grade limiter (burst 5)
//
// Every test builds its own harness so it gets a fresh rate-limit bucket.

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/auth"
	"github.com/pquerna/otp/totp"
)

// mfaEnrollment is the one-time material returned by the enrollment flow.
type mfaEnrollment struct {
	Secret        string
	RecoveryCodes []string
}

// newCookieClient returns a client with an empty jar, suitable for simulating
// a fresh browser that has no session or device cookies.
func newCookieClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	return &http.Client{Jar: jar}
}

// totpCodeNow returns the current 6-digit code for secret.
func totpCodeNow(t *testing.T, secret string) string {
	t.Helper()
	code, err := totp.GenerateCode(secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("generate totp code: %v", err)
	}
	return code
}

// wrongTOTPCode returns a syntactically valid code that is not the current
// code, avoiding the astronomically unlikely "000000 is correct" flake.
func wrongTOTPCode(t *testing.T, secret string) string {
	t.Helper()
	if good := totpCodeNow(t, secret); good != "000000" {
		return "000000"
	}
	return "111111"
}

// enableMFA drives the real enrollment endpoints for an already-authed client:
// setup → confirm with a live code → recovery codes from the response. It
// returns the persisted secret so tests can mint codes for later logins.
func enableMFA(t *testing.T, h *harness, c *http.Client) mfaEnrollment {
	t.Helper()

	raw := doAs(t, h, c, http.MethodPost, "/v1/auth/mfa/setup", nil, http.StatusOK)
	setup := decodeJSON[struct {
		Secret     string `json:"secret"`
		OtpauthURL string `json:"otpauthUrl"`
	}](t, raw)
	if setup.Secret == "" {
		t.Fatal("setup returned an empty secret")
	}
	if !strings.HasPrefix(setup.OtpauthURL, "otpauth://totp/") {
		t.Fatalf("setup otpauthUrl = %q", setup.OtpauthURL)
	}

	confirm := decodeJSON[struct {
		OK            bool     `json:"ok"`
		RecoveryCodes []string `json:"recoveryCodes"`
	}](t, doAs(t, h, c, http.MethodPost, "/v1/auth/mfa/confirm", map[string]any{
		"secret": setup.Secret,
		"code":   totpCodeNow(t, setup.Secret),
	}, http.StatusOK))
	if !confirm.OK {
		t.Fatalf("confirm did not return ok=true: %+v", confirm)
	}
	if len(confirm.RecoveryCodes) != auth.RecoveryCodeCount {
		t.Fatalf("recovery code count = %d, want %d", len(confirm.RecoveryCodes), auth.RecoveryCodeCount)
	}

	return mfaEnrollment{Secret: setup.Secret, RecoveryCodes: confirm.RecoveryCodes}
}

// doRaw sends a JSON request and returns the status, body and headers without
// asserting anything, so tests can inspect Set-Cookie on success paths.
func doRaw(t *testing.T, h *harness, c *http.Client, method, path string, payload any) (int, []byte, http.Header) {
	t.Helper()

	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal %s %s: %v", method, path, err)
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, h.server.URL+path, body)
	if err != nil {
		t.Fatalf("new request %s %s: %v", method, path, err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s %s: %v", method, path, err)
	}
	return resp.StatusCode, data, resp.Header.Clone()
}

// postLogin authenticates password-only and returns the raw status/body/headers
// so tests can assert the absence or presence of Set-Cookie.
func postLogin(t *testing.T, h *harness, c *http.Client, login, password string) (int, []byte, http.Header) {
	t.Helper()
	return doRaw(t, h, c, http.MethodPost, "/v1/auth/login", map[string]any{
		"login":    login,
		"password": password,
	})
}

// loginChallenge starts a password-only login and returns the challenge.
func loginChallenge(t *testing.T, h *harness, c *http.Client, login, password string) string {
	t.Helper()
	status, body, headers := postLogin(t, h, c, login, password)
	if status != http.StatusOK {
		t.Fatalf("login %s: status = %d, want 200, body = %s", login, status, body)
	}
	if got := headers.Values("Set-Cookie"); len(got) != 0 {
		t.Fatalf("login %s issued Set-Cookie headers before 2FA: %v", login, got)
	}
	out := decodeJSON[struct {
		Status    string `json:"status"`
		Challenge string `json:"challenge"`
	}](t, body)
	if out.Status != "mfa_required" {
		t.Fatalf("login status = %q, want mfa_required, body = %s", out.Status, body)
	}
	if out.Challenge == "" {
		t.Fatal("login returned an empty challenge")
	}
	return out.Challenge
}

// setCookieName reports whether any Set-Cookie header contains name=.
func setCookieName(headers http.Header, name string) bool {
	for _, cookie := range headers.Values("Set-Cookie") {
		if strings.HasPrefix(cookie, name+"=") {
			return true
		}
	}
	return false
}

// cookieValueFromJar returns the named cookie from a client's jar for the
// harness origin (empty when absent).
func cookieValueFromJar(t *testing.T, h *harness, c *http.Client, name string) string {
	t.Helper()
	u, err := url.Parse(h.server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	for _, ck := range c.Jar.Cookies(u) {
		if ck.Name == name {
			return ck.Value
		}
	}
	return ""
}

func totpSecretFor(t *testing.T, h *harness, userID string) string {
	t.Helper()
	var secret string
	if err := h.db.QueryRow(`SELECT totp_secret FROM users WHERE id = ?`, userID).Scan(&secret); err != nil {
		t.Fatalf("lookup totp secret: %v", err)
	}
	return secret
}

// TestMFALoginChallengeIssuedWithoutSession proves password success alone
// never mints a session for a 2FA-enabled account.
func TestMFALoginChallengeIssuedWithoutSession(t *testing.T) {
	h := newHarness(t)
	erin := newAuthedClient(t, h, "erin")
	enableMFA(t, h, erin)
	erinID := userIDFor(t, h, "erin")
	baseline := refreshTokenCount(t, h, erinID) // enrollment session from registration

	fresh := newCookieClient(t)
	status, body, headers := postLogin(t, h, fresh, "erin", "supersecret123")
	if status != http.StatusOK {
		t.Fatalf("login: status = %d, want 200, body = %s", status, body)
	}
	if got := headers.Values("Set-Cookie"); len(got) != 0 {
		t.Fatalf("mfa_required login must not set cookies, got %v", got)
	}
	out := decodeJSON[struct {
		Status    string `json:"status"`
		Challenge string `json:"challenge"`
	}](t, body)
	if out.Status != "mfa_required" || out.Challenge == "" {
		t.Fatalf("unexpected login body: %s", body)
	}
	if strings.Contains(string(body), "userId") {
		t.Fatalf("mfa_required login leaked a userId: %s", body)
	}

	// No refresh-token row may be added before the second factor completes.
	if n := refreshTokenCount(t, h, erinID); n != baseline {
		t.Fatalf("refresh token rows after password step = %d, want %d (unchanged)", n, baseline)
	}

	// The fresh client holds no usable session.
	doAs(t, h, fresh, http.MethodGet, "/v1/auth/me", nil, http.StatusUnauthorized)
}

// TestMFALoginWrongCodeKeepsChallengeReusable proves a wrong code is rejected
// without consuming the challenge, so a typo is not a lockout.
func TestMFALoginWrongCodeKeepsChallengeReusable(t *testing.T) {
	h := newHarness(t)
	frank := newAuthedClient(t, h, "frank")
	enrollment := enableMFA(t, h, frank)
	frankID := userIDFor(t, h, "frank")
	baseline := refreshTokenCount(t, h, frankID) // enrollment session from registration

	fresh := newCookieClient(t)
	challenge := loginChallenge(t, h, fresh, "frank", "supersecret123")

	status, body, headers := doRaw(t, h, fresh, http.MethodPost, "/v1/auth/mfa/verify", map[string]any{
		"challenge": challenge,
		"code":      wrongTOTPCode(t, enrollment.Secret),
	})
	if status != http.StatusUnauthorized {
		t.Fatalf("wrong code: status = %d, want 401, body = %s", status, body)
	}
	if got := headers.Values("Set-Cookie"); len(got) != 0 {
		t.Fatalf("wrong code set cookies: %v", got)
	}
	if n := refreshTokenCount(t, h, frankID); n != baseline {
		t.Fatalf("refresh token rows after wrong code = %d, want %d (unchanged)", n, baseline)
	}

	// The same challenge still accepts a correct code: single-use only applies
	// once verification succeeds.
	doAs(t, h, fresh, http.MethodPost, "/v1/auth/mfa/verify", map[string]any{
		"challenge": challenge,
		"code":      totpCodeNow(t, enrollment.Secret),
	}, http.StatusOK)
	if n := refreshTokenCount(t, h, frankID); n != baseline+1 {
		t.Fatalf("refresh token rows after correct code = %d, want %d", n, baseline+1)
	}

	// A successfully consumed challenge cannot be replayed for a second session.
	doAs(t, h, fresh, http.MethodPost, "/v1/auth/mfa/verify", map[string]any{
		"challenge": challenge,
		"code":      totpCodeNow(t, enrollment.Secret),
	}, http.StatusUnauthorized)
	if n := refreshTokenCount(t, h, frankID); n != baseline+1 {
		t.Fatalf("replayed challenge minted another session: rows = %d, want %d", n, baseline+1)
	}
}

// TestMFALoginCorrectCodeIssuesSession covers the happy path: cookies are set,
// the session works and /auth/me reports mfaEnabled=true.
func TestMFALoginCorrectCodeIssuesSession(t *testing.T) {
	h := newHarness(t)
	grace := newAuthedClient(t, h, "grace")
	enrollment := enableMFA(t, h, grace)
	graceID := userIDFor(t, h, "grace")
	baseline := refreshTokenCount(t, h, graceID) // enrollment session from registration

	fresh := newCookieClient(t)
	challenge := loginChallenge(t, h, fresh, "grace", "supersecret123")

	status, body, headers := doRaw(t, h, fresh, http.MethodPost, "/v1/auth/mfa/verify", map[string]any{
		"challenge": challenge,
		"code":      totpCodeNow(t, enrollment.Secret),
	})
	if status != http.StatusOK {
		t.Fatalf("verify: status = %d, want 200, body = %s", status, body)
	}
	if !setCookieName(headers, auth.AccessCookieName) || !setCookieName(headers, auth.RefreshCookieName) {
		t.Fatalf("verify did not issue both session cookies: %v", headers.Values("Set-Cookie"))
	}
	if got := cookieValueFromJar(t, h, fresh, auth.AccessCookieName); got == "" {
		t.Fatal("access cookie missing from client jar after verify")
	}
	if n := refreshTokenCount(t, h, graceID); n != baseline+1 {
		t.Fatalf("refresh token rows after verify = %d, want %d", n, baseline+1)
	}

	me := doAs(t, h, fresh, http.MethodGet, "/v1/auth/me", nil, http.StatusOK)
	profile := decodeJSON[struct {
		Username   string `json:"username"`
		MFAEnabled bool   `json:"mfaEnabled"`
	}](t, me)
	if profile.Username != "grace" {
		t.Fatalf("/me username = %q, want grace", profile.Username)
	}
	if !profile.MFAEnabled {
		t.Fatalf("/me mfaEnabled = false, want true: %s", me)
	}
}

// TestMFALoginUnknownChallengeRejectedAndReloginWorks proves a fabricated
// challenge cannot be used and that a fresh login always mints a new one.
func TestMFALoginUnknownChallengeRejectedAndReloginWorks(t *testing.T) {
	h := newHarness(t)
	heidi := newAuthedClient(t, h, "heidi")
	enrollment := enableMFA(t, h, heidi)

	fresh := newCookieClient(t)
	doAs(t, h, fresh, http.MethodPost, "/v1/auth/mfa/verify", map[string]any{
		"challenge": strings.Repeat("ab", 32), // well-formed-looking, never issued
		"code":      totpCodeNow(t, enrollment.Secret),
	}, http.StatusUnauthorized)

	// A real login mints a challenge that works normally.
	challenge := loginChallenge(t, h, fresh, "heidi", "supersecret123")
	doAs(t, h, fresh, http.MethodPost, "/v1/auth/mfa/verify", map[string]any{
		"challenge": challenge,
		"code":      totpCodeNow(t, enrollment.Secret),
	}, http.StatusOK)
}

// TestMFALoginRememberDeviceSkipsSecondFactor proves the remember-device
// cookie skips the challenge on the next password login.
func TestMFALoginRememberDeviceSkipsSecondFactor(t *testing.T) {
	h := newHarness(t)
	ivan := newAuthedClient(t, h, "ivan")
	enrollment := enableMFA(t, h, ivan)
	ivanID := userIDFor(t, h, "ivan")

	fresh := newCookieClient(t)
	challenge := loginChallenge(t, h, fresh, "ivan", "supersecret123")

	status, body, headers := doRaw(t, h, fresh, http.MethodPost, "/v1/auth/mfa/verify", map[string]any{
		"challenge":      challenge,
		"code":           totpCodeNow(t, enrollment.Secret),
		"rememberDevice": true,
	})
	if status != http.StatusOK {
		t.Fatalf("verify with rememberDevice: status = %d, body = %s", status, body)
	}
	if !setCookieName(headers, auth.TrustedDeviceCookieName) {
		t.Fatalf("verify did not set %s: %v", auth.TrustedDeviceCookieName, headers.Values("Set-Cookie"))
	}

	deviceToken := cookieValueFromJar(t, h, fresh, auth.TrustedDeviceCookieName)
	if deviceToken == "" {
		t.Fatal("trusted-device cookie missing from jar")
	}

	// Only a hash is stored, never the plaintext token.
	var storedHash string
	if err := h.db.QueryRow(`SELECT device_hash FROM trusted_devices WHERE user_id = ?`, ivanID).Scan(&storedHash); err != nil {
		t.Fatalf("lookup trusted device: %v", err)
	}
	if storedHash != auth.HashOpaqueToken(deviceToken) {
		t.Fatal("trusted device was not stored as the expected hash")
	}
	if storedHash == deviceToken {
		t.Fatal("trusted device token was stored in plaintext")
	}

	// Next login from the same client (which carries dt_device) skips 2FA.
	status, body, headers = postLogin(t, h, fresh, "ivan", "supersecret123")
	if status != http.StatusOK {
		t.Fatalf("trusted-device login: status = %d, body = %s", status, body)
	}
	if strings.Contains(string(body), "mfa_required") {
		t.Fatalf("trusted-device login still required 2FA: %s", body)
	}
	out := decodeJSON[struct {
		UserID string `json:"userId"`
	}](t, body)
	if out.UserID != ivanID {
		t.Fatalf("trusted-device login userId = %q, want %q", out.UserID, ivanID)
	}
	if !setCookieName(headers, auth.AccessCookieName) || !setCookieName(headers, auth.RefreshCookieName) {
		t.Fatalf("trusted-device login did not issue session cookies: %v", headers.Values("Set-Cookie"))
	}
}

// TestMFAVerifyRateLimited pins the burst-5 limiter shared with the password
// endpoints: the sixth immediate wrong code is a 429.
func TestMFAVerifyRateLimited(t *testing.T) {
	h := newHarness(t)
	judy := newAuthedClient(t, h, "judy")
	enrollment := enableMFA(t, h, judy)

	fresh := newCookieClient(t)
	challenge := loginChallenge(t, h, fresh, "judy", "supersecret123")
	wrong := wrongTOTPCode(t, enrollment.Secret)

	for i := 0; i < 5; i++ {
		doAs(t, h, fresh, http.MethodPost, "/v1/auth/mfa/verify", map[string]any{
			"challenge": challenge,
			"code":      wrong,
		}, http.StatusUnauthorized)
	}

	status, body, _ := doRaw(t, h, fresh, http.MethodPost, "/v1/auth/mfa/verify", map[string]any{
		"challenge": challenge,
		"code":      wrong,
	})
	if status != http.StatusTooManyRequests {
		t.Fatalf("sixth verify: status = %d, want 429, body = %s", status, body)
	}
	if !strings.Contains(string(body), "rate limit exceeded") {
		t.Fatalf("429 body = %s, want rate limit message", body)
	}
}
