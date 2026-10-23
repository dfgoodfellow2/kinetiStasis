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

func TestCheckinTDEERoundTrip(t *testing.T) {
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

	in := &models.CheckInLog{
		ID:             "c1",
		UserID:         "u1",
		CheckInDate:    "2026-10-23",
		WeightBefore:   81.2,
		WeightAfter:    80.9,
		CaloriesBefore: 2200,
		CaloriesAfter:  2100,
		Reason:         "cut",
		CreatedAt:      "now",
		ObservedTDEE:   2480.5,
		TDEEConfidence: "high",
		LookbackDays:   90,
	}
	if err := s.CreateCheckinLog(ctx, in); err != nil {
		t.Fatal(err)
	}

	last, err := s.GetLastCheckin(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if last.ObservedTDEE != 2480.5 || last.TDEEConfidence != "high" || last.LookbackDays != 90 {
		t.Fatalf("unexpected last checkin: %+v", last)
	}

	history, err := s.FetchCheckinHistory(ctx, "u1", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 {
		t.Fatalf("history length = %d, want 1", len(history))
	}
	got := history[0]
	if got.CheckInDate != "2026-10-23" || got.ObservedTDEE != 2480.5 ||
		got.TDEEConfidence != "high" || got.LookbackDays != 90 || got.ResultingCalories != 2100 {
		t.Fatalf("unexpected history entry: %+v", got)
	}

	b, _ := json.Marshal(got)
	for _, want := range []string{
		`"checkInDate":"2026-10-23"`,
		`"observedTdee":2480.5`,
		`"tdeeConfidence":"high"`,
		`"lookbackDays":90`,
		`"resultingCalories":2100`,
	} {
		if !strings.Contains(string(b), want) {
			t.Fatalf("json missing %s: %s", want, b)
		}
	}
	t.Logf("json: %s", b)
}
