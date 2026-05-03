package db

import (
	"database/sql"
	"embed"
	"fmt"
	"log/slog"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Open opens a SQLite database at the given path, enables WAL mode and
// foreign keys, then runs all pending goose migrations.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// SQLite tuning: WAL mode for concurrent reads, foreign keys on
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA cache_size=-20000",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return nil, fmt.Errorf("pragma %q: %w", p, err)
		}
	}

	// Connection pool: SQLite doesn't benefit from many writers
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// Run migrations
	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("migrations: %w", err)
	}

	// Ensure any columns that may be missing on some deployments are present.
	// Don't fail startup if this check/alter fails; just log the error.
	if err := ensureColumns(db); err != nil {
		slog.Error("failed to ensure columns", "error", err)
	}

	slog.Info("database ready", "path", path)
	return db, nil
}

func runMigrations(db *sql.DB) error {
	goose.SetBaseFS(migrations)

	if err := goose.SetDialect("sqlite3"); err != nil {
		return err
	}

	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}

	return nil
}

// ensureColumns verifies the presence of optional columns and adds them
// if missing. This is used because some deployment environments (eg. Fly.io
// Alpine images) may not run local sqlite3 migrations before the app starts.
func ensureColumns(db *sql.DB) error {
	// Check if body_fat_pct column exists
	rows, err := db.Query("PRAGMA table_info(biometric_logs)")
	if err != nil {
		return fmt.Errorf("pragma table_info: %w", err)
	}
	defer rows.Close()

	found := false
	for rows.Next() {
		var cid int
		var name string
		var type_ string
		var notnull int
		var dflt_value interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &type_, &notnull, &dflt_value, &pk); err != nil {
			return fmt.Errorf("scan pragma: %w", err)
		}
		if name == "body_fat_pct" {
			found = true
			break
		}
	}

	if !found {
		slog.Info("adding missing body_fat_pct column to biometric_logs")
		if _, err := db.Exec("ALTER TABLE biometric_logs ADD COLUMN body_fat_pct REAL DEFAULT 0"); err != nil {
			return fmt.Errorf("alter table: %w", err)
		}
	}

	return nil
}
