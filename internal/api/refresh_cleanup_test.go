package api_test

// Expired-refresh-token cleanup coverage: presenting an expired token must
// still be a 401, and the rejection path lazily removes the user's dead rows
// while preserving live sessions.

import (
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/auth"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/constants"
)

func TestRefreshExpiredTokenCleansUpRows(t *testing.T) {
	h := newHarness(t)
	userID := userIDFor(t, h, "integrity")

	expiredAt := time.Now().Add(-time.Hour).UTC().Format(constants.TimeFormat)
	validAt := time.Now().Add(time.Hour).UTC().Format(constants.TimeFormat)
	now := time.Now().UTC().Format(constants.TimeFormat)

	expiredTokens := []string{"expired-one", "expired-two"}
	validToken := "still-valid"

	insert := func(id, token, expiresAt string) {
		t.Helper()
		if _, err := h.db.Exec(
			`INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, created_at) VALUES (?,?,?,?,?)`,
			id, userID, hashRefreshToken(token), expiresAt, now,
		); err != nil {
			t.Fatalf("insert refresh token %q: %v", id, err)
		}
	}
	for i, token := range expiredTokens {
		insert("expired-"+string(rune('a'+i)), token, expiredAt)
	}
	insert("valid-row", validToken, validAt)

	req, err := http.NewRequest(http.MethodPost, h.server.URL+"/v1/auth/refresh", nil)
	if err != nil {
		t.Fatalf("new refresh request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: auth.RefreshCookieName, Value: expiredTokens[0]})

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("refresh request: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("refresh status = %d, want 401, body = %s", resp.StatusCode, body)
	}

	var expiredRows int
	if err := h.db.QueryRow(
		`SELECT COUNT(*) FROM refresh_tokens WHERE user_id = ? AND expires_at <= ?`,
		userID, time.Now().UTC().Format(constants.TimeFormat),
	).Scan(&expiredRows); err != nil {
		t.Fatalf("count expired rows: %v", err)
	}
	if expiredRows != 0 {
		t.Fatalf("expired refresh rows = %d, want 0 after rejected refresh", expiredRows)
	}

	var validRows int
	if err := h.db.QueryRow(
		`SELECT COUNT(*) FROM refresh_tokens WHERE token_hash = ?`,
		hashRefreshToken(validToken),
	).Scan(&validRows); err != nil {
		t.Fatalf("count valid rows: %v", err)
	}
	if validRows != 1 {
		t.Fatalf("live refresh rows = %d, want 1 (cleanup must keep live sessions)", validRows)
	}
}
