package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

// --- Two-factor authentication persistence ---
//
// All lookups return the raw *sql.ErrNoRows (wrapped with %w) so handlers can
// distinguish "not configured" from a real database failure with errors.Is.

// GetUserTOTPSecret returns the stored TOTP secret, or "" when 2FA is not
// enabled. The value is never logged by this package.
func (s *SQLiteStore) GetUserTOTPSecret(ctx context.Context, userID string) (string, error) {
	var secret string
	if err := s.db.QueryRowContext(ctx, `SELECT totp_secret FROM users WHERE id = ?`, userID).Scan(&secret); err != nil {
		return "", fmt.Errorf("get user totp secret: %w", err)
	}
	return secret, nil
}

// EnableTOTP commits a confirmed secret and installs a fresh recovery-code set
// atomically. A failed insert rolls the secret back, so the account is either
// fully enabled or unchanged.
func (s *SQLiteStore) EnableTOTP(ctx context.Context, userID, secret string, codeHashes []string, now string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin enable totp: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `UPDATE users SET totp_secret = ?, updated_at = ? WHERE id = ?`, secret, now, userID)
	if err != nil {
		return fmt.Errorf("set totp secret: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("set totp secret rows: %w", err)
	} else if n == 0 {
		return ErrNotFound
	}

	if err := deleteRecoveryCodesTx(ctx, tx, userID); err != nil {
		return err
	}
	for _, hash := range codeHashes {
		if err := insertRecoveryCodeTx(ctx, tx, userID, hash, now); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit enable totp: %w", err)
	}
	return nil
}

// DisableTOTP clears the secret and revokes recovery codes plus trusted
// devices in one transaction. A partial disable could leave a device able to
// skip a now-disabled second factor, so all three writes succeed or none do.
func (s *SQLiteStore) DisableTOTP(ctx context.Context, userID, now string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin disable totp: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `UPDATE users SET totp_secret = '', updated_at = ? WHERE id = ?`, now, userID)
	if err != nil {
		return fmt.Errorf("clear totp secret: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("clear totp secret rows: %w", err)
	} else if n == 0 {
		return ErrNotFound
	}

	if err := deleteRecoveryCodesTx(ctx, tx, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM trusted_devices WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("delete trusted devices: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit disable totp: %w", err)
	}
	return nil
}

// ReplaceRecoveryCodes burns the entire existing set and installs a new one.
func (s *SQLiteStore) ReplaceRecoveryCodes(ctx context.Context, userID string, codeHashes []string, createdAt string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin replace recovery codes: %w", err)
	}
	defer tx.Rollback()

	if err := deleteRecoveryCodesTx(ctx, tx, userID); err != nil {
		return err
	}
	for _, hash := range codeHashes {
		if err := insertRecoveryCodeTx(ctx, tx, userID, hash, createdAt); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit replace recovery codes: %w", err)
	}
	return nil
}

// BurnRecoveryCode consumes a single unused recovery code. The UPDATE is
// guarded by used_at = ” so two concurrent verifies cannot both succeed.
func (s *SQLiteStore) BurnRecoveryCode(ctx context.Context, userID, codeHash, usedAt string) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE recovery_codes SET used_at = ? WHERE user_id = ? AND code_hash = ? AND used_at = ''`,
		usedAt, userID, codeHash)
	if err != nil {
		return false, fmt.Errorf("burn recovery code: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("burn recovery code rows: %w", err)
	}
	return n > 0, nil
}

// DeleteRecoveryCodes removes every recovery code for a user.
func (s *SQLiteStore) DeleteRecoveryCodes(ctx context.Context, userID string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM recovery_codes WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("delete recovery codes: %w", err)
	}
	return nil
}

// CountUnusedRecoveryCodes reports how many backup codes remain.
func (s *SQLiteStore) CountUnusedRecoveryCodes(ctx context.Context, userID string) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM recovery_codes WHERE user_id = ? AND used_at = ''`, userID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count unused recovery codes: %w", err)
	}
	return n, nil
}

// CreateLoginChallenge stores the hash of a short-lived two-step login token.
func (s *SQLiteStore) CreateLoginChallenge(ctx context.Context, challengeHash, userID, expiresAt string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO login_challenges (challenge_hash, user_id, expires_at) VALUES (?,?,?)`,
		challengeHash, userID, expiresAt)
	if err != nil {
		return fmt.Errorf("create login challenge: %w", err)
	}
	return nil
}

// FindLoginChallenge returns the user bound to a challenge hash.
func (s *SQLiteStore) FindLoginChallenge(ctx context.Context, challengeHash string) (userID, expiresAt string, err error) {
	err = s.db.QueryRowContext(ctx,
		`SELECT user_id, expires_at FROM login_challenges WHERE challenge_hash = ?`, challengeHash).
		Scan(&userID, &expiresAt)
	if err != nil {
		return "", "", fmt.Errorf("find login challenge: %w", err)
	}
	return userID, expiresAt, nil
}

// DeleteLoginChallenge consumes a challenge. Deleting a missing row is not an
// error: the desired end state, no usable challenge, already holds.
func (s *SQLiteStore) DeleteLoginChallenge(ctx context.Context, challengeHash string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM login_challenges WHERE challenge_hash = ?`, challengeHash); err != nil {
		return fmt.Errorf("delete login challenge: %w", err)
	}
	return nil
}

// DeleteExpiredLoginChallenges lazily sheds a user's dead challenges.
func (s *SQLiteStore) DeleteExpiredLoginChallenges(ctx context.Context, userID, now string) error {
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM login_challenges WHERE user_id = ? AND expires_at <= ?`, userID, now); err != nil {
		return fmt.Errorf("delete expired login challenges: %w", err)
	}
	return nil
}

// CreateTrustedDevice stores the hash of a remember-device token.
func (s *SQLiteStore) CreateTrustedDevice(ctx context.Context, id, userID, deviceHash, createdAt, expiresAt string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO trusted_devices (id, user_id, device_hash, created_at, last_used_at, expires_at) VALUES (?,?,?,?,?,?)`,
		id, userID, deviceHash, createdAt, createdAt, expiresAt)
	if err != nil {
		return fmt.Errorf("create trusted device: %w", err)
	}
	return nil
}

// FindTrustedDevice returns the expiry of the device row bound to userID.
func (s *SQLiteStore) FindTrustedDevice(ctx context.Context, userID, deviceHash string) (expiresAt string, err error) {
	err = s.db.QueryRowContext(ctx,
		`SELECT expires_at FROM trusted_devices WHERE user_id = ? AND device_hash = ?`, userID, deviceHash).
		Scan(&expiresAt)
	if err != nil {
		return "", fmt.Errorf("find trusted device: %w", err)
	}
	return expiresAt, nil
}

// TouchTrustedDevice records that a remembered device was used.
func (s *SQLiteStore) TouchTrustedDevice(ctx context.Context, userID, deviceHash, lastUsedAt string) error {
	if _, err := s.db.ExecContext(ctx,
		`UPDATE trusted_devices SET last_used_at = ? WHERE user_id = ? AND device_hash = ?`,
		lastUsedAt, userID, deviceHash); err != nil {
		return fmt.Errorf("touch trusted device: %w", err)
	}
	return nil
}

// DeleteTrustedDevice revokes one remembered device.
func (s *SQLiteStore) DeleteTrustedDevice(ctx context.Context, userID, deviceHash string) error {
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM trusted_devices WHERE user_id = ? AND device_hash = ?`, userID, deviceHash); err != nil {
		return fmt.Errorf("delete trusted device: %w", err)
	}
	return nil
}

// DeleteUserTrustedDevices revokes every remembered device for a user.
func (s *SQLiteStore) DeleteUserTrustedDevices(ctx context.Context, userID string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM trusted_devices WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("delete user trusted devices: %w", err)
	}
	return nil
}

func deleteRecoveryCodesTx(ctx context.Context, tx *sql.Tx, userID string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM recovery_codes WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("delete recovery codes: %w", err)
	}
	return nil
}

func insertRecoveryCodeTx(ctx context.Context, tx *sql.Tx, userID, codeHash, createdAt string) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO recovery_codes (id, user_id, code_hash, used_at, created_at) VALUES (?,?,?,'',?)`,
		uuid.NewString(), userID, codeHash, createdAt); err != nil {
		return fmt.Errorf("insert recovery code: %w", err)
	}
	return nil
}
