package store_test

// Legacy body_measurements rows can carry SQL NULL circumferences: the columns
// are nullable REAL and shoulders/calves were added by a later migration
// (003_add_measurements.sql). No API path can produce such a row today, so these
// tests write one with raw SQL to pin the reader's tolerance.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/db"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"
)

func newMeasurementsStore(t *testing.T) (*store.SQLiteStore, *sql.DB) {
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

func TestFetchMeasurementsRangeLegacyNullColumns(t *testing.T) {
	ctx := context.Background()
	s, rawDB := newMeasurementsStore(t)

	// The legacy shape: only the NOT NULL columns are populated; every
	// circumference and notes are NULL.
	if _, err := rawDB.ExecContext(ctx, `
		INSERT INTO body_measurements (id,user_id,date,neck_cm,chest_cm,waist_cm,hips_cm,thigh_cm,bicep_cm,notes,created_at,shoulders_cm,calves_cm)
		VALUES ('legacy','u1','2025-01-02',NULL,NULL,NULL,NULL,NULL,NULL,NULL,'now',NULL,NULL)`); err != nil {
		t.Fatalf("insert legacy NULL row: %v", err)
	}

	rows, err := s.FetchMeasurementsRange(ctx, "u1", "2025-01-01", "2025-01-31")
	if err != nil {
		t.Fatalf("fetch measurements range: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	got := rows[0]
	if got.NeckCm != 0 || got.ChestCm != 0 || got.WaistCm != 0 || got.HipsCm != 0 ||
		got.ThighCm != 0 || got.BicepCm != 0 || got.ShouldersCm != 0 || got.CalvesCm != 0 {
		t.Fatalf("NULL circumferences must scan as 0, got %+v", got)
	}
	if got.Notes != "" {
		t.Fatalf("NULL notes must scan as empty string, got %q", got.Notes)
	}

	// Full JSON round-trip: the API contract must expose numbers, not nulls.
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal measurement: %v", err)
	}
	var decoded models.BodyMeasurement
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal measurement: %v", err)
	}
	if decoded.ShouldersCm != 0 || decoded.CalvesCm != 0 || decoded.Notes != "" {
		t.Fatalf("JSON round-trip lost the zero sentinel: %s", raw)
	}
}

func TestUpdateMeasurementMissingDateReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	s, _ := newMeasurementsStore(t)

	err := s.UpdateMeasurement(ctx, &models.BodyMeasurement{
		UserID:  "u1",
		Date:    "2025-02-01",
		WaistCm: 90,
	})
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("UpdateMeasurement on missing date err = %v, want ErrNotFound", err)
	}
}
