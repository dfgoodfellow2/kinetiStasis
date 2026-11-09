package api_test

// Wave P3 export coverage exercised through the real router (cookies,
// middleware, SQLite). These tests pin the user-visible contract:
//
//   - malformed, inverted, or over-long date ranges are 400s, never huge scans
//   - the resolved range is inclusive at both ends and the `to` bound reaches
//     the SQL query (a row outside it must not leak into an export)
//   - format=csv selects a CSV attachment with a UTF-8 BOM
//   - body-measurement waists are merged into weight rows regardless of the
//     order the measurement dates are appended in, and a waist-only day never
//     renders a fake 0-weight line

import (
	"encoding/csv"
	"io"
	"net/http"
	"strings"
	"testing"
)

type rawResponse struct {
	status int
	header http.Header
	body   []byte
}

// getRaw issues a GET through the harness client without asserting a status, so
// callers can inspect headers and the exact (BOM-bearing) byte payload.
func (h *harness) getRaw(t *testing.T, path string) rawResponse {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, h.server.URL+path, nil)
	if err != nil {
		t.Fatalf("new request GET %s: %v", path, err)
	}
	resp, err := h.client.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read GET %s: %v", path, err)
	}
	return rawResponse{status: resp.StatusCode, header: resp.Header.Clone(), body: body}
}

func csvRows(t *testing.T, content string) [][]string {
	t.Helper()
	content = strings.TrimPrefix(content, "\ufeff")
	r := csv.NewReader(strings.NewReader(content))
	r.Comment = '#'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v\ncontent:\n%s", err, content)
	}
	return rows
}

func TestExportDateRangeValidation(t *testing.T) {
	h := newHarness(t)

	cases := []struct {
		name string
		path string
	}{
		{
			name: "bad from date",
			path: "/v1/export/nutrition?from=2026-1-2&to=2026-10-20",
		},
		{
			name: "impossible to date",
			path: "/v1/export/nutrition?from=2026-10-20&to=2026-02-30",
		},
		{
			name: "from after to",
			path: "/v1/export/nutrition?from=2026-10-21&to=2026-10-20",
		},
		{
			name: "span 367 days",
			path: "/v1/export/nutrition?from=2025-01-01&to=2026-01-03",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h.do(t, http.MethodGet, tc.path, nil, http.StatusBadRequest)
		})
	}

	// The same validation is shared by every export endpoint.
	h.do(t, http.MethodGet, "/v1/export/workouts?from=2026-10-21&to=2026-10-20", nil, http.StatusBadRequest)
	h.do(t, http.MethodGet, "/v1/export/combined?from=not-a-date&to=2026-10-20", nil, http.StatusBadRequest)

	// A 366-day difference is the inclusive cap and must still be accepted.
	h.do(t, http.MethodGet, "/v1/export/nutrition?from=2025-01-01&to=2026-01-02&format=csv", nil, http.StatusOK)
}

func TestExportRangeBoundsInclusiveAndBounded(t *testing.T) {
	h := newHarness(t)

	h.do(t, http.MethodPost, "/v1/nutrition/logs", map[string]any{
		"date":     testDate,
		"calories": 500,
	}, http.StatusCreated)
	h.do(t, http.MethodPost, "/v1/nutrition/logs", map[string]any{
		"date":     "2026-10-21",
		"calories": 600,
	}, http.StatusCreated)

	// from == to must include that single day (inclusive SQL lower/upper bound).
	resp := h.getRaw(t, "/v1/export/nutrition?from="+testDate+"&to="+testDate+"&format=csv")
	if resp.status != http.StatusOK {
		t.Fatalf("export status = %d, body = %s", resp.status, resp.body)
	}
	body := string(resp.body)
	if !strings.HasPrefix(body, "\ufeff") {
		t.Fatalf("CSV body does not start with a UTF-8 BOM: %q", body)
	}
	if got := resp.header.Get("Content-Type"); got != "text/csv" {
		t.Fatalf("Content-Type = %q, want text/csv", got)
	}
	if got := resp.header.Get("Content-Disposition"); !strings.Contains(got, ".csv") {
		t.Fatalf("Content-Disposition = %q, want a .csv attachment", got)
	}
	if !strings.Contains(body, testDate) {
		t.Fatalf("from==to export omitted %s:\n%s", testDate, body)
	}
	if strings.Contains(body, "2026-10-21") {
		t.Fatalf("export leaked the day after the `to` bound:\n%s", body)
	}

	rows := csvRows(t, body)
	if len(rows) != 2 {
		t.Fatalf("CSV rows = %d, want header + one day", len(rows))
	}
	if rows[1][0] != testDate {
		t.Fatalf("CSV row date = %q, want %q", rows[1][0], testDate)
	}
}

func markdownDaySection(t *testing.T, doc, date string) string {
	t.Helper()
	marker := "## " + date
	idx := strings.Index(doc, marker)
	if idx < 0 {
		t.Fatalf("markdown document has no section for %s:\n%s", date, doc)
	}
	rest := doc[idx+len(marker):]
	if next := strings.Index(rest, "\n## "); next >= 0 {
		rest = rest[:next]
	}
	return rest
}

func TestCombinedExportMergesWeightAndWaist(t *testing.T) {
	h := newHarness(t)

	// Weight lands first; the waist lives in body_measurements. A second,
	// later waist-only date forces the handler to append after the weight row,
	// which is the order that used to invalidate the merge pointers.
	h.do(t, http.MethodPost, "/v1/biometrics", map[string]any{
		"date":     testDate,
		"weightKg": 80,
	}, http.StatusOK)
	h.do(t, http.MethodPost, "/v1/measurements", map[string]any{
		"date":    testDate,
		"waistCm": 90,
	}, http.StatusOK)
	h.do(t, http.MethodPost, "/v1/measurements", map[string]any{
		"date":    "2026-10-21",
		"waistCm": 91,
	}, http.StatusOK)

	resp := h.getRaw(t, "/v1/export/combined?from="+testDate+"&to=2026-10-21")
	if resp.status != http.StatusOK {
		t.Fatalf("combined export status = %d, body = %s", resp.status, resp.body)
	}
	doc := string(resp.body)

	weightDay := markdownDaySection(t, doc, testDate)
	if !strings.Contains(weightDay, "- Weight: 80.0") {
		t.Fatalf("merged weight day lost its weight:\n%s", weightDay)
	}
	if !strings.Contains(weightDay, "- Waist: 90.0") {
		t.Fatalf("merged weight day lost its waist (append/invalidation order):\n%s", weightDay)
	}

	waistOnlyDay := markdownDaySection(t, doc, "2026-10-21")
	if !strings.Contains(waistOnlyDay, "- Waist: 91.0") {
		t.Fatalf("waist-only day did not render its waist:\n%s", waistOnlyDay)
	}
	if strings.Contains(waistOnlyDay, "Weight:") {
		t.Fatalf("waist-only day rendered a fake weight:\n%s", waistOnlyDay)
	}
}
