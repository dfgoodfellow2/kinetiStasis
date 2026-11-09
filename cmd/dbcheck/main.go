// Command dbcheck reports what is inside a Diet Tracker SQLite database.
//
// It is meant for restores and backups: the file is opened strictly read-only
// and no migrations are run, so a missing schema reliably means "wrong file"
// rather than "this checker migrated it for you". Builds with CGO_ENABLED=0.
//
// Usage:
//
//	go run ./cmd/dbcheck <sqlite-file>
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// expectedTables mirrors the tables created by migration 001 (plus the
// add-only tables and the target_history rebuild that production always has).
// A restored file containing all of these is a real migrated database.
var expectedTables = []string{
	"users",
	"profiles",
	"refresh_tokens",
	"nutrition_logs",
	"biometric_logs",
	"workouts",
	"targets",
	"target_history",
	"saved_meals",
	"meal_templates",
	"body_measurements",
	"check_in_logs",
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: dbcheck <sqlite-file>")
		os.Exit(2)
	}
	if err := run(os.Args[1]); err != nil {
		fmt.Fprintf(os.Stderr, "dbcheck: %v\n", err)
		os.Exit(1)
	}
}

func run(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory, expected a SQLite file", path)
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", path, err)
	}

	db, err := sql.Open("sqlite", readOnlyDSN(abs))
	if err != nil {
		return fmt.Errorf("open %s read-only: %w", abs, err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("open %s read-only: %w", abs, err)
	}

	missing, err := missingTables(ctx, db)
	if err != nil {
		return err
	}
	if len(missing) > 0 {
		return fmt.Errorf(
			"missing tables (%s) — this does not look like a migrated Diet Tracker database; "+
				"the restore likely failed or the wrong file was checked",
			strings.Join(missing, ", "))
	}

	counts := make(map[string]int64, len(expectedTables))
	for _, table := range expectedTables {
		var n int64
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+quoteIdent(table)).Scan(&n); err != nil {
			return fmt.Errorf("count %s: %w", table, err)
		}
		counts[table] = n
	}

	var minDate, maxDate sql.NullString
	if err := db.QueryRowContext(ctx,
		"SELECT MIN(date), MAX(date) FROM nutrition_logs").Scan(&minDate, &maxDate); err != nil {
		return fmt.Errorf("nutrition_logs date range: %w", err)
	}

	var (
		checkinDate   string
		checkinCals   int64
		latestCheckin = "(none)"
	)
	err = db.QueryRowContext(ctx,
		`SELECT check_in_date, COALESCE(calories_after, 0)
		   FROM check_in_logs
		  ORDER BY check_in_date DESC, created_at DESC
		  LIMIT 1`).Scan(&checkinDate, &checkinCals)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// leave the placeholder
	case err != nil:
		return fmt.Errorf("latest check-in: %w", err)
	default:
		latestCheckin = fmt.Sprintf("%s (calories_after=%d)", checkinDate, checkinCals)
	}

	fmt.Printf("DB check: %s\n", abs)
	fmt.Printf("Size:     %s (%d bytes)\n", humanBytes(info.Size()), info.Size())
	fmt.Println("Tables:")
	for _, table := range expectedTables {
		fmt.Printf("  %-22s %d\n", table, counts[table])
	}
	fmt.Printf("nutrition_logs dates: %s\n", dateRange(minDate, maxDate))
	fmt.Printf("latest check-in:      %s\n", latestCheckin)
	fmt.Printf("users:                %d\n", counts["users"])
	return nil
}

// missingTables returns every expected table that is absent from sqlite_master.
func missingTables(ctx context.Context, db *sql.DB) ([]string, error) {
	var missing []string
	for _, table := range expectedTables {
		var name string
		err := db.QueryRowContext(ctx,
			"SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&name)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			missing = append(missing, table)
		case err != nil:
			return nil, fmt.Errorf("look up table %s: %w", table, err)
		}
	}
	return missing, nil
}

// readOnlyDSN builds a SQLite URI that cannot create, migrate, or write the
// database. Escaping the path keeps spaces and other special characters safe.
func readOnlyDSN(absPath string) string {
	escaped := (&url.URL{Path: filepath.ToSlash(absPath)}).EscapedPath()
	return "file:" + escaped + "?mode=ro"
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func dateRange(minDate, maxDate sql.NullString) string {
	if !minDate.Valid || !maxDate.Valid || minDate.String == "" {
		return "(empty)"
	}
	return minDate.String + " .. " + maxDate.String
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
