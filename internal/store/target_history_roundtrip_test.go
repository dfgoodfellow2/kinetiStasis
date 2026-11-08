package store_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/db"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"
)

// applyMigration015Up runs the Up half of migration 015 against an already-open
// database. The real migration only sees rows that predate it; tests insert a
// legacy row after db.Open and then replay the same normalisation to prove the
// backfill and the fixed-width writers agree on a sortable layout.
func applyMigration015Up(t *testing.T, d *sql.DB) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "db", "migrations", "015_fixed_timestamp_order.sql"))
	if err != nil {
		t.Fatalf("read migration 015: %v", err)
	}
	text := string(raw)
	start := strings.Index(text, "-- +goose StatementBegin")
	if start < 0 {
		t.Fatal("migration 015 has no StatementBegin marker")
	}
	start += len("-- +goose StatementBegin")
	end := strings.Index(text[start:], "-- +goose StatementEnd")
	if end < 0 {
		t.Fatal("migration 015 has no StatementEnd marker")
	}
	if _, err := d.ExecContext(context.Background(), text[start:start+end]); err != nil {
		t.Fatalf("apply migration 015 Up: %v", err)
	}
}

func TestTargetHistoryRoundTripsWaterAndEatBack(t *testing.T) {
	ctx := context.Background()
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if _, err := d.ExecContext(ctx, `INSERT INTO users (id,username,email,password,created_at,updated_at) VALUES ('u1','u1','u1@example.com','x','now','now')`); err != nil {
		t.Fatal(err)
	}
	s := store.NewSQLiteStore(d)
	snap := &models.TargetSnapshot{ID: "s1", UserID: "u1", EffectiveDate: "2026-10-23", Calories: 2000, ProteinG: 150, CarbsG: 200, FatG: 67, FiberG: 30, WaterMl: 3125, EatBackExercise: true, CreatedAt: "now"}
	if err := s.CreateTargetSnapshot(ctx, snap); err != nil {
		t.Fatal(err)
	}
	out, err := s.FetchTargetHistory(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].WaterMl != 3125 || !out[0].EatBackExercise {
		t.Fatalf("unexpected history: %+v", out)
	}
	b, _ := json.Marshal(out[0])
	if !strings.Contains(string(b), `"waterMl":3125`) || !strings.Contains(string(b), `"eatBackExercise":true`) {
		t.Fatalf("json missing fields: %s", b)
	}
	t.Logf("json: %s", b)
}

// TestTargetHistoryAllowsSameDaySnapshots proves the migration 011 fix: the
// uniqueness key includes created_at, so a check-in snapshot and a manual edit
// on the same effective date both persist instead of colliding.
func TestTargetHistoryAllowsSameDaySnapshots(t *testing.T) {
	ctx := context.Background()
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if _, err := d.ExecContext(ctx, `INSERT INTO users (id,username,email,password,created_at,updated_at) VALUES ('u1','u1','u1@example.com','x','now','now')`); err != nil {
		t.Fatal(err)
	}
	s := store.NewSQLiteStore(d)

	first := &models.TargetSnapshot{
		ID: "same-day-1", UserID: "u1", EffectiveDate: "2026-10-23",
		Calories: 2000, ProteinG: 150, CarbsG: 200, FatG: 67, FiberG: 30,
		WaterMl: 3000, EatBackExercise: false, CreatedAt: "2026-10-23T08:00:00Z",
	}
	second := &models.TargetSnapshot{
		ID: "same-day-2", UserID: "u1", EffectiveDate: "2026-10-23",
		Calories: 2200, ProteinG: 160, CarbsG: 220, FatG: 70, FiberG: 31,
		WaterMl: 3125, EatBackExercise: true, CreatedAt: "2026-10-23T09:30:00Z",
	}
	if err := s.CreateTargetSnapshot(ctx, first); err != nil {
		t.Fatalf("first same-day snapshot: %v", err)
	}
	if err := s.CreateTargetSnapshot(ctx, second); err != nil {
		t.Fatalf("second same-day snapshot collided (migration 011 not applied?): %v", err)
	}

	out, err := s.FetchTargetHistory(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("history length = %d, want 2 same-day snapshots: %+v", len(out), out)
	}
	seen := map[string]models.TargetSnapshot{}
	for _, snap := range out {
		seen[snap.ID] = snap
	}
	if _, ok := seen["same-day-1"]; !ok {
		t.Fatalf("missing first snapshot in %+v", out)
	}
	got, ok := seen["same-day-2"]
	if !ok {
		t.Fatalf("missing second snapshot in %+v", out)
	}
	if got.CreatedAt != "2026-10-23T09:30:00Z" || got.WaterMl != 3125 || !got.EatBackExercise {
		t.Fatalf("second snapshot did not round-trip: %+v", got)
	}
}

// TestTargetHistoryMixedPrecisionBackfillOrdersNewerFirst proves the two halves
// of the timestamp-ordering fix work together: legacy rows written at second
// precision are normalised by migration 015 to the same fixed 9-digit fraction
// the current writers emit, so lexical created_at DESC is chronological even
// for two snapshots inside the same second.
func TestTargetHistoryMixedPrecisionBackfillOrdersNewerFirst(t *testing.T) {
	ctx := context.Background()
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if _, err := d.ExecContext(ctx, `INSERT INTO users (id,username,email,password,created_at,updated_at) VALUES ('u1','u1','u1@example.com','x','now','now')`); err != nil {
		t.Fatal(err)
	}
	s := store.NewSQLiteStore(d)

	// Pre-015 shape: second precision. Lexically this is GREATER than the
	// padded form ('Z' > '.'), so raw mixed precision sorts the older row first.
	if _, err := d.ExecContext(ctx, `
		INSERT INTO target_history (id,user_id,effective_date,calories,protein_g,carbs_g,fat_g,fiber_g,water_ml,eat_back_exercise,created_at)
		VALUES ('legacy','u1','2026-10-23',2000,150,200,67,30,3000,0,'2026-10-23T09:00:05Z')`); err != nil {
		t.Fatalf("insert legacy second-precision snapshot: %v", err)
	}
	// Current writer shape: fixed nanosecond width, actually 123ns NEWER.
	fixed := &models.TargetSnapshot{
		ID: "fixed", UserID: "u1", EffectiveDate: "2026-10-23",
		Calories: 2200, ProteinG: 160, CarbsG: 220, FatG: 70, FiberG: 31,
		WaterMl: 3125, EatBackExercise: true, CreatedAt: "2026-10-23T09:00:05.000000123Z",
	}
	if err := s.CreateTargetSnapshot(ctx, fixed); err != nil {
		t.Fatalf("insert fixed-nano snapshot: %v", err)
	}

	// Premise: before the backfill, mixed precision inverts the intended order.
	before, err := s.FetchTargetHistory(ctx, "u1")
	if err != nil {
		t.Fatalf("fetch history before backfill: %v", err)
	}
	if len(before) != 2 || before[0].ID != "legacy" {
		t.Fatalf("expected legacy row lexically first before backfill, got %+v", before)
	}

	applyMigration015Up(t, d)

	after, err := s.FetchTargetHistory(ctx, "u1")
	if err != nil {
		t.Fatalf("fetch history after backfill: %v", err)
	}
	if len(after) != 2 {
		t.Fatalf("history length = %d, want 2", len(after))
	}
	if after[0].ID != "fixed" || after[1].ID != "legacy" {
		t.Fatalf("DESC order after backfill = [%s, %s], want [fixed, legacy]", after[0].ID, after[1].ID)
	}
	if after[1].CreatedAt != "2026-10-23T09:00:05.000000000Z" {
		t.Fatalf("legacy created_at = %q, want fixed-zero padding", after[1].CreatedAt)
	}
}
