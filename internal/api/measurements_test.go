package api_test

// Measurement-specific integrity coverage: legacy NULL rows must survive the
// reader, PUT must tell the truth about missing rows, replace semantics must
// persist intentional clears, and POST must echo the stored row rather than a
// freshly minted identity.

import (
	"net/http"
	"strings"
	"testing"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
)

// A row written by an older schema version (NULL shoulders/calves, NULL notes)
// must still list cleanly with zero sentinels instead of failing the scan.
func TestMeasurementListToleratesLegacyNullRow(t *testing.T) {
	h := newHarness(t)

	var userID string
	if err := h.db.QueryRow(`SELECT id FROM users LIMIT 1`).Scan(&userID); err != nil {
		t.Fatalf("lookup user id: %v", err)
	}
	if _, err := h.db.Exec(`
		INSERT INTO body_measurements (id,user_id,date,neck_cm,chest_cm,waist_cm,hips_cm,thigh_cm,bicep_cm,notes,created_at,shoulders_cm,calves_cm)
		VALUES ('legacy',?,'2025-03-04',NULL,NULL,NULL,NULL,NULL,NULL,NULL,'now',NULL,NULL)`, userID); err != nil {
		t.Fatalf("insert legacy NULL row: %v", err)
	}

	got := decodeJSON[[]models.BodyMeasurement](t, h.do(t, http.MethodGet,
		"/v1/measurements?from=2025-03-04&to=2025-03-04", nil, http.StatusOK))
	if len(got) != 1 {
		t.Fatalf("rows = %d, want 1", len(got))
	}
	if got[0].ShouldersCm != 0 || got[0].CalvesCm != 0 || got[0].Notes != "" {
		t.Fatalf("legacy row not normalised: %+v", got[0])
	}
}

// PUT against a date with no row must be a truthful 404, not a 200 with a
// payload the database never accepted.
func TestMeasurementPUTMissingDateReturns404(t *testing.T) {
	h := newHarness(t)
	h.do(t, http.MethodPut, "/v1/measurements/2025-03-05", map[string]any{
		"waistCm": 90,
	}, http.StatusNotFound)
}

// PUT is a true replace: blanking a previously stored field must persist as 0,
// including notes clearing to the empty string.
func TestMeasurementPUTReplaceClearsFields(t *testing.T) {
	h := newHarness(t)

	h.do(t, http.MethodPost, "/v1/measurements", map[string]any{
		"date":    testDate,
		"waistCm": 90,
		"neckCm":  40,
		"notes":   "morning",
	}, http.StatusOK)

	h.do(t, http.MethodPut, "/v1/measurements/"+testDate, map[string]any{
		"waistCm": 92,
	}, http.StatusOK)

	got := decodeJSON[[]models.BodyMeasurement](t, h.do(t, http.MethodGet,
		"/v1/measurements?from="+testDate+"&to="+testDate, nil, http.StatusOK))
	if len(got) != 1 {
		t.Fatalf("rows = %d, want 1", len(got))
	}
	if got[0].WaistCm != 92 {
		t.Fatalf("waist = %v, want 92", got[0].WaistCm)
	}
	if got[0].NeckCm != 0 || got[0].Notes != "" {
		t.Fatalf("PUT did not clear omitted fields: neck=%v notes=%q", got[0].NeckCm, got[0].Notes)
	}
}

// POST responds with the persisted row: a second upsert that carries a different
// client id must echo the original id/created_at, not the request's values.
func TestMeasurementPOSTRespondsWithStoredRow(t *testing.T) {
	h := newHarness(t)

	first := decodeJSON[models.BodyMeasurement](t, h.do(t, http.MethodPost, "/v1/measurements", map[string]any{
		"id":      "client-id-1",
		"date":    testDate,
		"waistCm": 90,
	}, http.StatusOK))
	if first.ID != "client-id-1" {
		t.Fatalf("first POST id = %q, want client-id-1", first.ID)
	}
	if first.CreatedAt == "" {
		t.Fatalf("first POST createdAt empty; want persisted timestamp")
	}

	second := decodeJSON[models.BodyMeasurement](t, h.do(t, http.MethodPost, "/v1/measurements", map[string]any{
		"id":      "client-id-2",
		"date":    testDate,
		"waistCm": 95,
	}, http.StatusOK))
	if second.ID != first.ID {
		t.Fatalf("second POST id = %q, want persisted %q", second.ID, first.ID)
	}
	if second.CreatedAt != first.CreatedAt {
		t.Fatalf("second POST createdAt = %q, want persisted %q", second.CreatedAt, first.CreatedAt)
	}
	if second.WaistCm != 95 {
		t.Fatalf("second POST waist = %v, want 95", second.WaistCm)
	}
}

// An empty result window serialises as [] rather than database null.
func TestMeasurementListEmptyIsArray(t *testing.T) {
	h := newHarness(t)
	raw := h.do(t, http.MethodGet, "/v1/measurements?from=2020-01-01&to=2020-01-02", nil, http.StatusOK)
	if body := strings.TrimSpace(string(raw)); body != "[]" {
		t.Fatalf("empty list body = %q, want []", body)
	}
}
