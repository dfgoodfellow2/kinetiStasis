package api_test

// Password management coverage: self-service change and admin one-time reset,
// exercised through the real router (cookies, middleware, SQLite). The suite
// pins the security contract:
//
//   - a wrong current password is a generic 401, never a 500
//   - a successful change revokes every prior refresh token and leaves the
//     current device with a fresh pair of cookies
//   - the old password stops working at /auth/login while the new one works
//   - admin reset returns a 12-char temp password, revokes the member's
//     sessions, and is forbidden below admin / for self-reset

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/auth"
)

func userIDFor(t *testing.T, h *harness, username string) string {
	t.Helper()
	var id string
	if err := h.db.QueryRow(`SELECT id FROM users WHERE username = ?`, username).Scan(&id); err != nil {
		t.Fatalf("lookup user id for %q: %v", username, err)
	}
	return id
}

func refreshTokenCount(t *testing.T, h *harness, userID string) int {
	t.Helper()
	var n int
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM refresh_tokens WHERE user_id = ?`, userID).Scan(&n); err != nil {
		t.Fatalf("count refresh tokens for %q: %v", userID, err)
	}
	return n
}

func refreshTokenValue(t *testing.T, h *harness, c *http.Client) string {
	t.Helper()
	// The refresh cookie is scoped to /v1/auth, so the jar only yields it for
	// a URL under that path (asking for "/" returns no refresh cookie).
	u, err := url.Parse(h.server.URL + "/v1/auth")
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	for _, ck := range c.Jar.Cookies(u) {
		if ck.Name == auth.RefreshCookieName {
			return ck.Value
		}
	}
	return ""
}

func hashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// loginClient posts /v1/auth/login with a dedicated cookie jar and asserts the
// expected status, so old/new password checks cannot be masked by a live
// session in another jar.
func loginClient(t *testing.T, h *harness, login, password string, wantStatus int) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	c := &http.Client{Jar: jar}
	doAs(t, h, c, http.MethodPost, "/v1/auth/login", map[string]any{
		"login":    login,
		"password": password,
	}, wantStatus)
	return c
}

func TestChangePasswordFlow(t *testing.T) {
	h := newHarness(t)
	carol := newAuthedClient(t, h, "carol")
	carolID := userIDFor(t, h, "carol")

	// A wrong current password must be the same generic 401 as Login.
	body := doAs(t, h, carol, http.MethodPost, "/v1/auth/password", map[string]any{
		"currentPassword": "not-the-password",
		"newPassword":     "newsecret123",
	}, http.StatusUnauthorized)
	if !strings.Contains(string(body), "invalid credentials") {
		t.Fatalf("wrong current password body = %s, want generic invalid credentials", body)
	}

	// The new password still has to satisfy register's minimum length.
	doAs(t, h, carol, http.MethodPost, "/v1/auth/password", map[string]any{
		"currentPassword": "supersecret123",
		"newPassword":     "short",
	}, http.StatusBadRequest)

	oldRefresh := refreshTokenValue(t, h, carol)
	if oldRefresh == "" {
		t.Fatal("carol has no refresh cookie before password change")
	}
	if got := refreshTokenCount(t, h, carolID); got != 1 {
		t.Fatalf("refresh token rows before change = %d, want 1", got)
	}

	doAs(t, h, carol, http.MethodPost, "/v1/auth/password", map[string]any{
		"currentPassword": "supersecret123",
		"newPassword":     "newsecret123",
	}, http.StatusOK)

	// The pre-change refresh token row is gone and only the freshly issued
	// session remains.
	var oldRows int
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM refresh_tokens WHERE token_hash = ?`, hashRefreshToken(oldRefresh)).Scan(&oldRows); err != nil {
		t.Fatalf("count old refresh token rows: %v", err)
	}
	if oldRows != 0 {
		t.Fatalf("old refresh token rows = %d, want 0 after password change", oldRows)
	}
	if got := refreshTokenCount(t, h, carolID); got != 1 {
		t.Fatalf("refresh token rows after change = %d, want 1 (fresh session only)", got)
	}

	// The current device keeps working and received a new refresh cookie.
	if me := doAs(t, h, carol, http.MethodGet, "/v1/auth/me", nil, http.StatusOK); !strings.Contains(string(me), "carol") {
		t.Fatalf("unexpected /me body after change: %s", me)
	}
	newRefresh := refreshTokenValue(t, h, carol)
	if newRefresh == "" || newRefresh == oldRefresh {
		t.Fatalf("current device did not receive a fresh refresh cookie")
	}

	// Old credentials are dead; the new ones work.
	loginClient(t, h, "carol", "supersecret123", http.StatusUnauthorized)
	loginClient(t, h, "carol", "newsecret123", http.StatusOK)
}

func TestAdminResetPasswordFlow(t *testing.T) {
	h := newHarness(t) // "integrity" is the auto-admin owner
	dave := newAuthedClient(t, h, "dave")
	daveID := userIDFor(t, h, "dave")
	adminID := userIDFor(t, h, "integrity")

	// Non-admin may not call the reset endpoint at all.
	doAs(t, h, dave, http.MethodPost, "/v1/admin/users/"+daveID+"/reset-password", nil, http.StatusForbidden)

	// Admins must use the change-password flow for their own account.
	doAs(t, h, h.client, http.MethodPost, "/v1/admin/users/"+adminID+"/reset-password", nil, http.StatusBadRequest)

	// Unknown target is a clean 404.
	doAs(t, h, h.client, http.MethodPost, "/v1/admin/users/no-such-user/reset-password", nil, http.StatusNotFound)

	// Happy path: temp password returned once, member sessions revoked.
	raw := doAs(t, h, h.client, http.MethodPost, "/v1/admin/users/"+daveID+"/reset-password", nil, http.StatusOK)
	out := decodeJSON[struct {
		TempPassword string `json:"tempPassword"`
	}](t, raw)
	if len(out.TempPassword) != 12 {
		t.Fatalf("temp password length = %d, want 12 (%q)", len(out.TempPassword), out.TempPassword)
	}

	if got := refreshTokenCount(t, h, daveID); got != 0 {
		t.Fatalf("dave refresh token rows after admin reset = %d, want 0", got)
	}

	// Dave's old password no longer signs in; the temp password does.
	loginClient(t, h, "dave", "supersecret123", http.StatusUnauthorized)
	loginClient(t, h, "dave", out.TempPassword, http.StatusOK)
}
