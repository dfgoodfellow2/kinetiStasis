// Command reset-2fa is the break-glass path for a user who has enabled TOTP
// two-factor authentication and lost every device and recovery code.
//
// It is intentionally tiny and dependency-light. It opens the SQLite file
// directly (NOT internal/db.Open, which would run goose migrations) and clears
// only the second-factor state for one account:
//
//   - users.totp_secret is reset to the empty string (password-only login)
//   - every recovery code for the user is deleted
//   - every trusted-device row for the user is revoked
//
// The account password is never touched, and the command never prints a
// secret. Non-existent emails exit 1 with a clear message so an operator can
// distinguish a typo from a broken database.
//
// Usage:
//
//	reset-2fa <db-path> <email>
//
// In production this is run inside the machine:
//
//	fly ssh console -a diet-tracker-v2 -C "/app/reset-2fa /data/diet.db user@example.com"
package main

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	_ "modernc.org/sqlite"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "reset-2fa: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 2 {
		return errors.New("usage: reset-2fa <db-path> <email>")
	}

	dbPath := strings.TrimSpace(args[0])
	email := strings.TrimSpace(args[1])
	if dbPath == "" {
		return errors.New("database path is required")
	}
	if email == "" {
		return errors.New("email is required")
	}

	// Open the file directly, read-write, without migrations. busy_timeout
	// lets this coexist with the running server's SQLite writer.
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("open database %q: %w", dbPath, err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(`PRAGMA busy_timeout=5000`); err != nil {
		return fmt.Errorf("set busy timeout: %w", err)
	}

	userID, err := lookupUserID(db, email)
	if err != nil {
		return err
	}

	recoveryDeleted, devicesDeleted, err := clearSecondFactor(db, email)
	if err != nil {
		return err
	}

	fmt.Printf("2FA reset for %s (user %s):\n", email, userID)
	fmt.Println("  totp_secret cleared — password-only login is now possible")
	fmt.Printf("  recovery codes deleted: %d\n", recoveryDeleted)
	fmt.Printf("  trusted devices revoked: %d\n", devicesDeleted)
	fmt.Println("The user can now sign in with their password and re-enrol an authenticator.")
	return nil
}

func lookupUserID(db *sql.DB, email string) (string, error) {
	var userID string
	if err := db.QueryRow(`SELECT id FROM users WHERE email = ?`, email).Scan(&userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("no user with email %q", email)
		}
		return "", fmt.Errorf("look up user %q: %w", email, err)
	}
	return userID, nil
}

// clearSecondFactor performs the three writes in one transaction so the
// account never ends up half-reset (e.g. secret cleared but old recovery codes
// still live, which would be confusing during re-enrolment).
func clearSecondFactor(db *sql.DB, email string) (recoveryDeleted, devicesDeleted int64, err error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.Exec(`UPDATE users SET totp_secret = '' WHERE email = ?`, email)
	if err != nil {
		return 0, 0, fmt.Errorf("clear totp secret: %w", err)
	}
	updated, err := res.RowsAffected()
	if err != nil {
		return 0, 0, fmt.Errorf("clear totp secret rows: %w", err)
	}
	if updated == 0 {
		// The earlier lookup succeeded, so zero rows here means the account was
		// deleted in between. Fail loudly rather than reporting a false reset.
		return 0, 0, fmt.Errorf("no user with email %q", email)
	}

	res, err = tx.Exec(`DELETE FROM recovery_codes WHERE user_id = (SELECT id FROM users WHERE email = ?)`, email)
	if err != nil {
		return 0, 0, fmt.Errorf("delete recovery codes: %w", err)
	}
	recoveryDeleted, err = res.RowsAffected()
	if err != nil {
		return 0, 0, fmt.Errorf("delete recovery code rows: %w", err)
	}

	res, err = tx.Exec(`DELETE FROM trusted_devices WHERE user_id = (SELECT id FROM users WHERE email = ?)`, email)
	if err != nil {
		return 0, 0, fmt.Errorf("delete trusted devices: %w", err)
	}
	devicesDeleted, err = res.RowsAffected()
	if err != nil {
		return 0, 0, fmt.Errorf("delete trusted device rows: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("commit reset: %w", err)
	}
	return recoveryDeleted, devicesDeleted, nil
}
