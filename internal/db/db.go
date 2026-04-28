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
