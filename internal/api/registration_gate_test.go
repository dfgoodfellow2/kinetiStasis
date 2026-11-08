package api_test

// Registration gate coverage. The suite drives the real router, cookies and
// SQLite database, and pins the user-approved mode matrix:
//
//   invite_only (default) — no code is rejected; a valid single-use code is
//                            accepted once; expired/revoked codes fail
//   zero users            — the first registration is always allowed and the
//                            account is the admin/owner (bootstrap)
//   public_approval       — registration creates a pending account, login is
//                            403 with no tokens, admin approval unlocks login,
//                            and admin denial deletes the pending row
//   public                — registration is immediate and login works
//
// Admin-only surfaces (invite mint/list/revoke, pending queue, mode changes)
// are exercised through the same HTTP stack.

import (
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
)

// registrationInfo mirrors GET /v1/auth/registration-info.
type registrationInfo struct {
	Mode            string `json:"mode"`
	PendingApproval bool   `json:"pendingApproval"`
}

// registerFresh posts a registration as a brand-new (cookie-less) client. The
// returned client keeps any cookies a successful registration sets.
func registerFresh(t *testing.T, h *harness, username string, extra map[string]any, wantStatus int) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	c := &http.Client{Jar: jar}

	payload := map[string]any{
		"username": username,
		"email":    username + "@example.com",
		"password": "supersecret123",
	}
	for k, v := range extra {
		payload[k] = v
	}
	status, body := postRegister(t, h, c, payload)
	if status != wantStatus {
		t.Fatalf("register %s: status = %d, want %d, body = %s", username, status, wantStatus, body)
	}
	return c
}

// createRegistrationInvite mints a code as the harness admin and returns it.
func createRegistrationInvite(t *testing.T, h *harness) string {
	t.Helper()
	raw := doAs(t, h, h.client, http.MethodPost, "/v1/admin/register-invites", nil, http.StatusCreated)
	inv := decodeJSON[models.RegistrationInvite](t, raw)
	if len(inv.Code) != 10 {
		t.Fatalf("registration invite code length = %d, want 10 (%q)", len(inv.Code), inv.Code)
	}
	if inv.ExpiresAt == "" {
		t.Fatalf("registration invite missing expiresAt: %+v", inv)
	}
	return inv.Code
}

func registrationInfoFor(t *testing.T, h *harness) registrationInfo {
	t.Helper()
	raw := doAs(t, h, &http.Client{}, http.MethodGet, "/v1/auth/registration-info", nil, http.StatusOK)
	return decodeJSON[registrationInfo](t, raw)
}

func TestRegistrationInviteOnlyGate(t *testing.T) {
	h := newHarness(t) // "integrity" bootstrapped as the admin under invite_only

	info := registrationInfoFor(t, h)
	if info.Mode != "invite_only" || info.PendingApproval {
		t.Fatalf("unexpected registration-info: %+v", info)
	}

	// No code → rejected with the gate message.
	status, body := postRegister(t, h, &http.Client{}, map[string]any{
		"username": "bob",
		"email":    "bob@example.com",
		"password": "supersecret123",
	})
	if status != http.StatusForbidden {
		t.Fatalf("no-code register status = %d, want 403, body = %s", status, body)
	}
	if !strings.Contains(string(body), "registration requires a valid invite code") {
		t.Fatalf("no-code register body = %s", body)
	}

	// A valid code is accepted exactly once.
	code := createRegistrationInvite(t, h)
	registerFresh(t, h, "bob", map[string]any{"inviteCode": code}, http.StatusCreated)

	bobID := userIDFor(t, h, "bob")
	var usedBy string
	if err := h.db.QueryRow(`SELECT used_by FROM registration_invites WHERE code = ?`, code).Scan(&usedBy); err != nil {
		t.Fatalf("read consumed invite: %v", err)
	}
	if usedBy != bobID {
		t.Fatalf("invite used_by = %q, want bob %q", usedBy, bobID)
	}

	// Reuse is refused.
	status, body = postRegister(t, h, &http.Client{}, map[string]any{
		"username":   "mallory",
		"email":      "mallory@example.com",
		"password":   "supersecret123",
		"inviteCode": code,
	})
	if status != http.StatusForbidden {
		t.Fatalf("reused-code register status = %d, want 403, body = %s", status, body)
	}

	// Expired codes are refused even when unused.
	expiredCode := createRegistrationInvite(t, h)
	past := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	if _, err := h.db.Exec(`UPDATE registration_invites SET expires_at = ? WHERE code = ?`, past, expiredCode); err != nil {
		t.Fatalf("expire invite: %v", err)
	}
	status, body = postRegister(t, h, &http.Client{}, map[string]any{
		"username":   "eve",
		"email":      "eve@example.com",
		"password":   "supersecret123",
		"inviteCode": expiredCode,
	})
	if status != http.StatusForbidden {
		t.Fatalf("expired-code register status = %d, want 403, body = %s", status, body)
	}

	// Revoked codes are removed and cannot be used.
	revokedCode := createRegistrationInvite(t, h)
	doAs(t, h, h.client, http.MethodDelete, "/v1/admin/register-invites/"+revokedCode, nil, http.StatusOK)
	status, body = postRegister(t, h, &http.Client{}, map[string]any{
		"username":   "revoked",
		"email":      "revoked@example.com",
		"password":   "supersecret123",
		"inviteCode": revokedCode,
	})
	if status != http.StatusForbidden {
		t.Fatalf("revoked-code register status = %d, want 403, body = %s", status, body)
	}

	// The admin can list the audit trail; a regular member cannot.
	invites := decodeJSON[[]models.RegistrationInvite](t, doAs(t, h, h.client, http.MethodGet, "/v1/admin/register-invites", nil, http.StatusOK))
	if len(invites) == 0 {
		t.Fatal("admin invite list is empty after minting codes")
	}
	bob := newAuthedClient(t, h, "bob2")
	doAs(t, h, bob, http.MethodGet, "/v1/admin/register-invites", nil, http.StatusForbidden)
}

func TestRegistrationZeroUserBootstrap(t *testing.T) {
	h := newHarness(t)

	// Default mode is invite_only, yet the harness's first registration
	// succeeded without a code and produced the admin/owner.
	var users, admins int
	if err := h.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(is_admin),0) FROM users`).Scan(&users, &admins); err != nil {
		t.Fatalf("count users/admins: %v", err)
	}
	if users != 1 || admins != 1 {
		t.Fatalf("bootstrap users=%d admins=%d, want 1/1", users, admins)
	}

	// A second registration without a code is now gated.
	status, body := postRegister(t, h, &http.Client{}, map[string]any{
		"username": "second",
		"email":    "second@example.com",
		"password": "supersecret123",
	})
	if status != http.StatusForbidden {
		t.Fatalf("second no-code register status = %d, want 403, body = %s", status, body)
	}
}

func TestRegistrationPublicApprovalFlow(t *testing.T) {
	h := newHarness(t)

	doAs(t, h, h.client, http.MethodPut, "/v1/admin/registration-mode", map[string]any{"mode": "public_approval"}, http.StatusOK)
	if info := registrationInfoFor(t, h); info.Mode != "public_approval" || !info.PendingApproval {
		t.Fatalf("registration-info after mode change = %+v", info)
	}

	// Registering creates a pending row but no session.
	dave := registerFresh(t, h, "dave", nil, http.StatusCreated)
	doAs(t, h, dave, http.MethodGet, "/v1/auth/me", nil, http.StatusUnauthorized)

	body := doAs(t, h, dave, http.MethodPost, "/v1/auth/login", map[string]any{
		"login":    "dave",
		"password": "supersecret123",
	}, http.StatusForbidden)
	if !strings.Contains(string(body), "awaiting approval") {
		t.Fatalf("pending login body = %s", body)
	}

	// Pending queue exposes the request to the admin only.
	pending := decodeJSON[[]models.User](t, doAs(t, h, h.client, http.MethodGet, "/v1/admin/pending-users", nil, http.StatusOK))
	if len(pending) != 1 || pending[0].Username != "dave" || !pending[0].PendingApproval {
		t.Fatalf("unexpected pending queue: %+v", pending)
	}
	daveID := userIDFor(t, h, "dave")
	doAs(t, h, h.client, http.MethodPost, "/v1/admin/users/"+daveID+"/approve", nil, http.StatusOK)

	// Approval unlocks login; the queue empties. Keep the authenticated
	// client: the pre-approval dave jar never received tokens.
	dave = loginClient(t, h, "dave", "supersecret123", http.StatusOK)
	if left := decodeJSON[[]models.User](t, doAs(t, h, h.client, http.MethodGet, "/v1/admin/pending-users", nil, http.StatusOK)); len(left) != 0 {
		t.Fatalf("pending queue after approval = %+v", left)
	}

	// --- denial path -------------------------------------------------------
	registerFresh(t, h, "erin", nil, http.StatusCreated)
	erinID := userIDFor(t, h, "erin")

	// A non-admin (freshly approved dave) cannot approve or deny.
	doAs(t, h, dave, http.MethodPost, "/v1/admin/users/"+erinID+"/approve", nil, http.StatusForbidden)
	doAs(t, h, dave, http.MethodDelete, "/v1/admin/users/"+erinID, nil, http.StatusForbidden)

	// Admin denies by deleting the pending row.
	doAs(t, h, h.client, http.MethodDelete, "/v1/admin/users/"+erinID, nil, http.StatusOK)

	// No account remains, so login is the generic invalid-credentials 401.
	body = doAs(t, h, &http.Client{}, http.MethodPost, "/v1/auth/login", map[string]any{
		"login":    "erin",
		"password": "supersecret123",
	}, http.StatusUnauthorized)
	if !strings.Contains(string(body), "invalid credentials") {
		t.Fatalf("denied account login body = %s", body)
	}

	// Unknown modes are rejected.
	doAs(t, h, h.client, http.MethodPut, "/v1/admin/registration-mode", map[string]any{"mode": "bogus"}, http.StatusBadRequest)
}

func TestRegistrationPublicMode(t *testing.T) {
	h := newHarness(t)

	doAs(t, h, h.client, http.MethodPut, "/v1/admin/registration-mode", map[string]any{"mode": "public"}, http.StatusOK)
	if info := registrationInfoFor(t, h); info.Mode != "public" || info.PendingApproval {
		t.Fatalf("registration-info in public mode = %+v", info)
	}

	// Immediate account, no code and no approval.
	registerFresh(t, h, "frank", nil, http.StatusCreated)
	loginClient(t, h, "frank", "supersecret123", http.StatusOK)

	// The admin mode readback reflects the setting.
	mode := decodeJSON[struct {
		Mode string `json:"mode"`
	}](t, doAs(t, h, h.client, http.MethodGet, "/v1/admin/registration-mode", nil, http.StatusOK))
	if mode.Mode != "public" {
		t.Fatalf("admin mode readback = %q, want public", mode.Mode)
	}

	// Non-admins cannot change the posture.
	frank := loginClient(t, h, "frank", "supersecret123", http.StatusOK)
	doAs(t, h, frank, http.MethodPut, "/v1/admin/registration-mode", map[string]any{"mode": "invite_only"}, http.StatusForbidden)
}

// TestRegistrationInviteCompensationOnDuplicateUsername pins the invite burn
// compensation path. The code is consumed before the user insert so two
// concurrent registrations cannot both spend it; when that insert fails (here,
// a duplicate username) the burn must be released so the caller can retry with
// the same code and a fresh identity.
func TestRegistrationInviteCompensationOnDuplicateUsername(t *testing.T) {
	h := newHarness(t)
	newAuthedClient(t, h, "bob") // second account: bootstrap is over
	code := createRegistrationInvite(t, h)

	// Duplicate username: the code burns first, then CreateUser fails with 409.
	status, body := postRegister(t, h, &http.Client{}, map[string]any{
		"username":   "bob",
		"email":      "duplicate@example.com",
		"password":   "supersecret123",
		"inviteCode": code,
	})
	if status != http.StatusConflict {
		t.Fatalf("duplicate-username register status = %d, want 409, body = %s", status, body)
	}

	var usedBy string
	if err := h.db.QueryRow(`SELECT used_by FROM registration_invites WHERE code = ?`, code).Scan(&usedBy); err != nil {
		t.Fatalf("read invite after failed registration: %v", err)
	}
	if usedBy != "" {
		t.Fatalf("invite used_by = %q after failed registration, want unused", usedBy)
	}

	// The compensated code still works for a fresh username.
	registerFresh(t, h, "carol", map[string]any{"inviteCode": code}, http.StatusCreated)
	if err := h.db.QueryRow(`SELECT used_by FROM registration_invites WHERE code = ?`, code).Scan(&usedBy); err != nil {
		t.Fatalf("read invite after successful registration: %v", err)
	}
	if usedBy != userIDFor(t, h, "carol") {
		t.Fatalf("invite used_by = %q, want carol", usedBy)
	}
}

// TestApprovalCreatesProfileAndLiftsWaveRBlocker covers the Wave-R blocker:
// a public_approval account is created without a profile, so approval must
// create the empty profile row that GET /v1/profile and GET /v1/targets need.
func TestApprovalCreatesProfileAndLiftsWaveRBlocker(t *testing.T) {
	h := newHarness(t)

	doAs(t, h, h.client, http.MethodPut, "/v1/admin/registration-mode", map[string]any{"mode": "public_approval"}, http.StatusOK)
	dave := registerFresh(t, h, "dave", nil, http.StatusCreated)

	// Pending: no session, so protected reads are unauthenticated and login is
	// refused with the approval message.
	doAs(t, h, dave, http.MethodGet, "/v1/profile", nil, http.StatusUnauthorized)
	body := doAs(t, h, dave, http.MethodPost, "/v1/auth/login", map[string]any{
		"login":    "dave",
		"password": "supersecret123",
	}, http.StatusForbidden)
	if !strings.Contains(string(body), "awaiting approval") {
		t.Fatalf("pending login body = %s", body)
	}

	daveID := userIDFor(t, h, "dave")
	doAs(t, h, h.client, http.MethodPost, "/v1/admin/users/"+daveID+"/approve", nil, http.StatusOK)

	// Approval created the profile; the freshly logged-in member can read both
	// the profile and the computed fallback targets.
	dave = loginClient(t, h, "dave", "supersecret123", http.StatusOK)
	profile := decodeJSON[models.Profile](t, doAs(t, h, dave, http.MethodGet, "/v1/profile", nil, http.StatusOK))
	if profile.UserID != daveID {
		t.Fatalf("profile userId = %q, want dave %q", profile.UserID, daveID)
	}
	doAs(t, h, dave, http.MethodGet, "/v1/targets", nil, http.StatusOK)
}

// TestAdminDenyGuards pins the deny endpoint's safety rails: approved accounts
// are not deniable, unknown ids are 404, and the last-admin guard still fires
// defensively (exercised with a stale admin token after a raw demotion).
func TestAdminDenyGuards(t *testing.T) {
	h := newHarness(t)
	newAuthedClient(t, h, "bob")
	bobID := userIDFor(t, h, "bob")
	integrityID := userIDFor(t, h, "integrity")

	// An approved member is not a pending request.
	body := doAs(t, h, h.client, http.MethodDelete, "/v1/admin/users/"+bobID, nil, http.StatusBadRequest)
	if !strings.Contains(string(body), "only pending accounts can be denied") {
		t.Fatalf("approved-user deny body = %s", body)
	}

	// Unknown ids get a stable 404.
	doAs(t, h, h.client, http.MethodDelete, "/v1/admin/users/no-such-user", nil, http.StatusNotFound)

	// Make bob an admin, then log in so his token carries the admin claim.
	doAs(t, h, h.client, http.MethodPost, "/v1/admin/users/"+bobID+"/promote", nil, http.StatusOK)
	bobAdmin := loginClient(t, h, "bob", "supersecret123", http.StatusOK)

	// Simulate the defensive invariant break the guard protects against: bob's
	// token stays admin, the DB no longer agrees, and the only remaining admin
	// is a (pending) target. The deny path must refuse before deleting.
	if _, err := h.db.Exec(`UPDATE users SET is_admin = 0 WHERE id = ?`, bobID); err != nil {
		t.Fatalf("demote bob in DB: %v", err)
	}
	if _, err := h.db.Exec(`UPDATE users SET pending_approval = 1 WHERE id = ?`, integrityID); err != nil {
		t.Fatalf("mark integrity pending in DB: %v", err)
	}

	body = doAs(t, h, bobAdmin, http.MethodDelete, "/v1/admin/users/"+integrityID, nil, http.StatusBadRequest)
	if !strings.Contains(string(body), "cannot deny the last admin") {
		t.Fatalf("last-admin deny body = %s", body)
	}
}
