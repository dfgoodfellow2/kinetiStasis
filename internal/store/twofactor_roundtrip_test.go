package store_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/db"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"
)

// TestTwoFactorRoundTrip exercises migration 016 end to end at the store layer:
// enable/disable transactions, single-use recovery-code burns, challenge
// lifecycle, and trusted-device persistence.
func TestTwoFactorRoundTrip(t *testing.T) {
	ctx := context.Background()
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	const userID = "u2fa"
	if _, err := d.ExecContext(ctx,
		`INSERT INTO users (id,username,email,password,created_at,updated_at) VALUES (?,?,?,'x','now','now')`,
		userID, "twofa", "twofa@example.com",
	); err != nil {
		t.Fatal(err)
	}

	s := store.NewSQLiteStore(d)

	secret, err := s.GetUserTOTPSecret(ctx, userID)
	if err != nil {
		t.Fatalf("get unset secret: %v", err)
	}
	if secret != "" {
		t.Fatalf("unset secret = %q, want empty", secret)
	}

	// EnableTOTP installs the secret and all ten hashes atomically.
	hashes := make([]string, 10)
	for i := range hashes {
		hashes[i] = "hash-" + string(rune('A'+i))
	}
	if err := s.EnableTOTP(ctx, userID, "JBSWY3DPEHPK3PXP", hashes, "2026-01-01T00:00:00Z"); err != nil {
		t.Fatalf("enable totp: %v", err)
	}
	if got, err := s.GetUserTOTPSecret(ctx, userID); err != nil || got != "JBSWY3DPEHPK3PXP" {
		t.Fatalf("enabled secret = %q, err %v", got, err)
	}
	if n, err := s.CountUnusedRecoveryCodes(ctx, userID); err != nil || n != 10 {
		t.Fatalf("unused codes after enable = %d, err %v, want 10", n, err)
	}

	// A recovery code burns exactly once.
	burned, err := s.BurnRecoveryCode(ctx, userID, hashes[0], "2026-01-02T00:00:00Z")
	if err != nil || !burned {
		t.Fatalf("first burn = %v, err %v, want true", burned, err)
	}
	burned, err = s.BurnRecoveryCode(ctx, userID, hashes[0], "2026-01-02T00:00:00Z")
	if err != nil || burned {
		t.Fatalf("second burn = %v, err %v, want false", burned, err)
	}
	if n, err := s.CountUnusedRecoveryCodes(ctx, userID); err != nil || n != 9 {
		t.Fatalf("unused codes after burn = %d, err %v, want 9", n, err)
	}

	// Replace burns the entire old set and installs the new one.
	if err := s.ReplaceRecoveryCodes(ctx, userID, []string{"new-1", "new-2"}, "2026-01-03T00:00:00Z"); err != nil {
		t.Fatalf("replace codes: %v", err)
	}
	if n, err := s.CountUnusedRecoveryCodes(ctx, userID); err != nil || n != 2 {
		t.Fatalf("unused codes after replace = %d, err %v, want 2", n, err)
	}
	if burned, err := s.BurnRecoveryCode(ctx, userID, hashes[1], "2026-01-03T00:00:00Z"); err != nil || burned {
		t.Fatalf("old code burn after replace = %v, err %v, want false", burned, err)
	}
	if burned, err := s.BurnRecoveryCode(ctx, userID, "new-1", "2026-01-03T00:00:00Z"); err != nil || !burned {
		t.Fatalf("new code burn = %v, err %v, want true", burned, err)
	}

	// Challenge lifecycle: create, find, single-use delete.
	if err := s.CreateLoginChallenge(ctx, "ch-1", userID, "2026-01-04T00:00:00Z"); err != nil {
		t.Fatalf("create challenge: %v", err)
	}
	gotUser, expires, err := s.FindLoginChallenge(ctx, "ch-1")
	if err != nil || gotUser != userID || expires != "2026-01-04T00:00:00Z" {
		t.Fatalf("find challenge = %q/%q, err %v", gotUser, expires, err)
	}
	if err := s.DeleteLoginChallenge(ctx, "ch-1"); err != nil {
		t.Fatalf("delete challenge: %v", err)
	}
	if _, _, err := s.FindLoginChallenge(ctx, "ch-1"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("find deleted challenge err = %v, want sql.ErrNoRows", err)
	}

	// Expired challenges are cleaned up lazily.
	if err := s.CreateLoginChallenge(ctx, "ch-expired", userID, "2026-01-01T00:00:00Z"); err != nil {
		t.Fatalf("create expired challenge: %v", err)
	}
	if err := s.DeleteExpiredLoginChallenges(ctx, userID, "2026-01-05T00:00:00Z"); err != nil {
		t.Fatalf("delete expired challenges: %v", err)
	}
	if _, _, err := s.FindLoginChallenge(ctx, "ch-expired"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("find expired challenge err = %v, want sql.ErrNoRows", err)
	}

	// Trusted-device lifecycle.
	if err := s.CreateTrustedDevice(ctx, "d1", userID, "device-hash", "2026-01-01T00:00:00Z", "2026-01-31T00:00:00Z"); err != nil {
		t.Fatalf("create trusted device: %v", err)
	}
	exp, err := s.FindTrustedDevice(ctx, userID, "device-hash")
	if err != nil || exp != "2026-01-31T00:00:00Z" {
		t.Fatalf("find trusted device = %q, err %v", exp, err)
	}
	if err := s.TouchTrustedDevice(ctx, userID, "device-hash", "2026-01-10T00:00:00Z"); err != nil {
		t.Fatalf("touch trusted device: %v", err)
	}
	var lastUsed string
	if err := d.QueryRowContext(ctx, `SELECT last_used_at FROM trusted_devices WHERE id = 'd1'`).Scan(&lastUsed); err != nil {
		t.Fatalf("read last_used_at: %v", err)
	}
	if lastUsed != "2026-01-10T00:00:00Z" {
		t.Fatalf("last_used_at = %q, want touched timestamp", lastUsed)
	}

	// Disable clears the secret, burns all codes and revokes every device in
	// one transaction.
	if err := s.DisableTOTP(ctx, userID, "2026-02-01T00:00:00Z"); err != nil {
		t.Fatalf("disable totp: %v", err)
	}
	if got, err := s.GetUserTOTPSecret(ctx, userID); err != nil || got != "" {
		t.Fatalf("secret after disable = %q, err %v", got, err)
	}
	if n, err := s.CountUnusedRecoveryCodes(ctx, userID); err != nil || n != 0 {
		t.Fatalf("unused codes after disable = %d, err %v, want 0", n, err)
	}
	if _, err := s.FindTrustedDevice(ctx, userID, "device-hash"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("trusted device after disable err = %v, want sql.ErrNoRows", err)
	}

	// Unknown users surface ErrNotFound rather than silently succeeding.
	if err := s.EnableTOTP(ctx, "missing", "SECRET", hashes, "2026-01-01T00:00:00Z"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("enable missing user err = %v, want ErrNotFound", err)
	}
}
