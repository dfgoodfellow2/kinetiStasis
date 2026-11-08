package store_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/db"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"
)

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
