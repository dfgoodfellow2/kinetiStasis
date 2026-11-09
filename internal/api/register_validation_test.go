package api_test

// Registration input validation coverage. Validation runs before the invite
// gate, so junk input is a clean 400 even on an invite-only instance. The
// happy path still registers and issues a session.

import (
	"net/http"
	"strings"
	"testing"
)

func TestRegisterRejectsInvalidUsername(t *testing.T) {
	h := newHarness(t) // invite-only by default; validation must fire first

	usernames := []string{
		"ab",                    // too short
		strings.Repeat("a", 33), // too long
		"bad name",              // space
		"bad/name",              // disallowed character
		"bad@name",              // disallowed character
		"usér",                  // non-ASCII
	}
	for _, username := range usernames {
		status, body := postRegister(t, h, &http.Client{}, map[string]any{
			"username": username,
			"email":    "valid@example.com",
			"password": "supersecret123",
		})
		if status != http.StatusBadRequest {
			t.Fatalf("register username %q: status = %d, want 400, body = %s", username, status, body)
		}
		if !strings.Contains(string(body), "username") {
			t.Fatalf("register username %q: body = %s, want username error", username, body)
		}
	}
}

func TestRegisterRejectsInvalidEmail(t *testing.T) {
	h := newHarness(t)

	emails := []string{
		"not-an-email",
		"two@@example.com",
		"spaced address@example.com",
		"user@",
		"@example.com",
		strings.Repeat("a", 245) + "@example.com", // >254 bytes
	}
	for _, email := range emails {
		status, body := postRegister(t, h, &http.Client{}, map[string]any{
			"username": "validuser",
			"email":    email,
			"password": "supersecret123",
		})
		if status != http.StatusBadRequest {
			t.Fatalf("register email %q: status = %d, want 400, body = %s", email, status, body)
		}
		if !strings.Contains(string(body), "email") {
			t.Fatalf("register email %q: body = %s, want email error", email, body)
		}
	}
}

func TestRegisterHappyPathWithValidatedInput(t *testing.T) {
	h := newHarness(t)
	code := createRegistrationInvite(t, h)

	registerFresh(t, h, "valid.user-1", map[string]any{"inviteCode": code}, http.StatusCreated)

	if id := userIDFor(t, h, "valid.user-1"); id == "" {
		t.Fatal("valid registration did not create a user")
	}
}
