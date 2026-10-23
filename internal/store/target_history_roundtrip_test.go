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
