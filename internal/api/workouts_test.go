package api_test

// Wave M3 pins the workout write-path semantics: a PUT replaces the row named
// by the URL (not the body), a POST is an idempotent upsert keyed on
// (user, date, slot), malformed keys are rejected at the HTTP edge, and stored
// JSON that no longer decodes surfaces as a 500 "corrupt" error instead of an
// empty workout.

import (
	"net/http"
	"strings"
	"testing"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
)

// 1. The URL identifies the row. A body carrying a different slot must not
// create, move, or silently leave the URL row untouched while returning 200.
func TestWorkoutPUTUsesURLKeysNotBodyKeys(t *testing.T) {
	h := newHarness(t)

	h.do(t, http.MethodPost, "/v1/workouts", map[string]any{
		"date":  testDate,
		"slot":  "1",
		"title": "original",
	}, http.StatusOK)

	// Body claims slot "2"; the URL says slot "1". The URL must win.
	h.do(t, http.MethodPut, "/v1/workouts/"+testDate+"/1", map[string]any{
		"date":  testDate,
		"slot":  "2",
		"title": "replaced",
	}, http.StatusOK)

	got := decodeJSON[models.WorkoutEntry](t, h.do(t, http.MethodGet, "/v1/workouts/"+testDate+"/1", nil, http.StatusOK))
	if got.Title != "replaced" {
		t.Fatalf("title at URL slot after PUT = %q, want %q (PUT must target the URL row)", got.Title, "replaced")
	}

	// No row may have been created at the slot named only in the body.
	h.do(t, http.MethodGet, "/v1/workouts/"+testDate+"/2", nil, http.StatusNotFound)

	var n int
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM workouts WHERE date = ?`, testDate).Scan(&n); err != nil {
		t.Fatalf("count workouts: %v", err)
	}
	if n != 1 {
		t.Fatalf("workouts rows for %s = %d, want 1 (body slot must not create a row)", testDate, n)
	}
}

// 2. Malformed keys are rejected before the store is touched.
func TestWorkoutRejectsBadDateAndEmptySlot(t *testing.T) {
	h := newHarness(t)

	h.do(t, http.MethodPost, "/v1/workouts", map[string]any{
		"date":  "2026-1-2",
		"slot":  "1",
		"title": "bad date",
	}, http.StatusBadRequest)

	h.do(t, http.MethodPost, "/v1/workouts", map[string]any{
		"date":  testDate,
		"slot":  "",
		"title": "missing slot",
	}, http.StatusBadRequest)

	// PUT validates the URL keys, so an impossible date in the path is a 400.
	h.do(t, http.MethodPut, "/v1/workouts/2026-1-2/1", map[string]any{
		"title": "bad date",
	}, http.StatusBadRequest)

	var n int
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM workouts`).Scan(&n); err != nil {
		t.Fatalf("count workouts: %v", err)
	}
	if n != 0 {
		t.Fatalf("workouts rows after rejected writes = %d, want 0", n)
	}
}

// 3. Two POSTs for the same (date, slot) are one row, last write wins. The
// previous non-atomic read-then-write could insert twice and fail on the
// unique constraint.
func TestWorkoutPOSTIsIdempotentUpsert(t *testing.T) {
	h := newHarness(t)

	h.do(t, http.MethodPost, "/v1/workouts", map[string]any{
		"date":  testDate,
		"slot":  "am",
		"title": "first",
	}, http.StatusOK)

	h.do(t, http.MethodPost, "/v1/workouts", map[string]any{
		"date":  testDate,
		"slot":  "am",
		"title": "second",
	}, http.StatusOK)

	var n int
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM workouts WHERE date = ? AND slot = ?`, testDate, "am").Scan(&n); err != nil {
		t.Fatalf("count workouts: %v", err)
	}
	if n != 1 {
		t.Fatalf("workouts rows for %s/am = %d, want 1 (POST must upsert)", testDate, n)
	}

	got := decodeJSON[models.WorkoutEntry](t, h.do(t, http.MethodGet, "/v1/workouts/"+testDate+"/am", nil, http.StatusOK))
	if got.Title != "second" {
		t.Fatalf("title after second POST = %q, want %q", got.Title, "second")
	}
}

// 4. Corrupt stored JSON must be reported, never rendered as a workout with an
// empty exercise/session payload.
func TestWorkoutCorruptStoredJSONReturns500(t *testing.T) {
	h := newHarness(t)

	h.do(t, http.MethodPost, "/v1/workouts", map[string]any{
		"date":  testDate,
		"slot":  "1",
		"title": "will corrupt",
	}, http.StatusOK)

	if _, err := h.db.Exec(`UPDATE workouts SET exercises_json = ? WHERE date = ?`, "{not json", testDate); err != nil {
		t.Fatalf("corrupt exercises_json: %v", err)
	}

	body := h.do(t, http.MethodGet, "/v1/workouts/"+testDate+"/1", nil, http.StatusInternalServerError)
	if !strings.Contains(strings.ToLower(string(body)), "corrupt") {
		t.Fatalf("GET corrupt row body = %s, want an error mentioning corrupt", body)
	}

	// The list endpoint reads the same column and must fail the same way.
	body = h.do(t, http.MethodGet, "/v1/workouts?from="+testDate+"&to="+testDate, nil, http.StatusInternalServerError)
	if !strings.Contains(strings.ToLower(string(body)), "corrupt") {
		t.Fatalf("GET corrupt list body = %s, want an error mentioning corrupt", body)
	}
}
