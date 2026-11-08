package store_test

// Legacy log rows predate the non-null numeric contract. biometric_logs'
// metrics, nutrition_logs.meal_notes and check_in_logs' 004 columns were all
// declared nullable, and migration 014 only normalises rows that existed when
// it ran. These tests write fresh NULL rows with raw SQL so every reader's
// COALESCE tolerance is pinned independently of the backfill.

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/db"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"
)

func newLogStore(t *testing.T) (*store.SQLiteStore, *sql.DB) {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	if _, err := d.ExecContext(context.Background(),
		`INSERT INTO users (id,username,email,password,created_at,updated_at) VALUES ('u1','u1','u1@example.com','x','now','now')`); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return store.NewSQLiteStore(d), d
}

func TestLegacyNullBiometricLogs(t *testing.T) {
	ctx := context.Background()
	s, rawDB := newLogStore(t)

	if _, err := rawDB.ExecContext(ctx, `
		INSERT INTO biometric_logs (id,user_id,date,weight_kg,waist_cm,grip_kg,bolt_score,sleep_hours,sleep_quality,subjective_feel,body_fat_pct,notes,updated_at)
		VALUES ('legacy-bio','u1','2025-01-02',NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,'now')`); err != nil {
		t.Fatalf("insert legacy NULL biometric row: %v", err)
	}

	rows, err := s.FetchBiometricLogs(ctx, "u1", "2025-01-01")
	if err != nil {
		t.Fatalf("fetch biometric logs: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("biometric rows = %d, want 1", len(rows))
	}
	got := rows[0]
	if got.WeightKg != 0 || got.WaistCm != 0 || got.GripKg != 0 || got.BoltScore != 0 ||
		got.SleepHours != 0 || got.SleepQuality != 0 || got.SubjectiveFeel != 0 || got.BodyFatPct != 0 {
		t.Fatalf("NULL biometric metrics must scan as 0, got %+v", got)
	}
	if got.Notes != "" {
		t.Fatalf("NULL biometric notes must scan as empty string, got %q", got.Notes)
	}

	one, err := s.GetBiometricLog(ctx, "u1", "2025-01-02")
	if err != nil {
		t.Fatalf("get biometric log: %v", err)
	}
	if one.WaistCm != 0 || one.GripKg != 0 || one.BoltScore != 0 || one.SleepHours != 0 || one.Notes != "" {
		t.Fatalf("GetBiometricLog lost the zero sentinel: %+v", one)
	}
}

func TestLegacyNullNutritionAndCheckinLogs(t *testing.T) {
	ctx := context.Background()
	s, rawDB := newLogStore(t)

	// nutrition_logs: only meal_notes is nullable and unset; the numeric
	// columns are NOT NULL DEFAULT 0.
	if _, err := rawDB.ExecContext(ctx, `
		INSERT INTO nutrition_logs (id,user_id,date,meal_notes,updated_at)
		VALUES ('legacy-nut','u1','2025-01-02',NULL,'now')`); err != nil {
		t.Fatalf("insert legacy NULL nutrition row: %v", err)
	}
	nutRows, err := s.FetchNutritionLogs(ctx, "u1", "2025-01-01")
	if err != nil {
		t.Fatalf("fetch nutrition logs: %v", err)
	}
	if len(nutRows) != 1 {
		t.Fatalf("nutrition rows = %d, want 1", len(nutRows))
	}
	if nutRows[0].MealNotes != "" {
		t.Fatalf("NULL meal_notes must scan as empty string, got %q", nutRows[0].MealNotes)
	}
	oneNut, err := s.GetNutritionLog(ctx, "u1", "2025-01-02")
	if err != nil {
		t.Fatalf("get nutrition log: %v", err)
	}
	if oneNut.MealNotes != "" {
		t.Fatalf("GetNutritionLog lost the empty-string sentinel: %+v", oneNut)
	}

	// check_in_logs: the 004/008 columns are omitted, so the nullable 004
	// columns arrive as SQL NULL; the 008 columns are NOT NULL DEFAULT 0.
	if _, err := rawDB.ExecContext(ctx, `
		INSERT INTO check_in_logs (id,user_id,check_in_date,created_at)
		VALUES ('legacy-chk','u1','2025-01-03','now')`); err != nil {
		t.Fatalf("insert legacy NULL check-in row: %v", err)
	}
	last, err := s.GetLastCheckin(ctx, "u1")
	if err != nil {
		t.Fatalf("get last check-in: %v", err)
	}
	if last.WeightBefore != 0 || last.WeightAfter != 0 || last.CaloriesBefore != 0 || last.CaloriesAfter != 0 {
		t.Fatalf("NULL check-in numerics must scan as 0, got %+v", last)
	}
	if last.Reason != "" {
		t.Fatalf("NULL check-in reason must scan as empty string, got %q", last.Reason)
	}

	history, err := s.FetchCheckinHistory(ctx, "u1", 10)
	if err != nil {
		t.Fatalf("fetch check-in history: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("check-in history rows = %d, want 1", len(history))
	}
	if history[0].ResultingCalories != 0 {
		t.Fatalf("NULL calories_after must scan as 0, got %+v", history[0])
	}
}
