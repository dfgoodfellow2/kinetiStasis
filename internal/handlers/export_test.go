package handlers

// Unit coverage for the export response writer. The filename allowlist is the
// only thing standing between a generated attachment name and header
// injection, so it is pinned directly as well as through the router-level
// export tests.

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteExportSanitizesFilename(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/export/nutrition", nil)

	writeExport(rec, req, "date,calories\r\n", "../../evil?x", "text/csv")

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got, want := rec.Header().Get("Content-Type"), "text/csv"; got != want {
		t.Fatalf("Content-Type = %q, want %q", got, want)
	}
	const want = `attachment; filename=".._.._evil_x"`
	if got := rec.Header().Get("Content-Disposition"); got != want {
		t.Fatalf("Content-Disposition = %q, want %q", got, want)
	}
	if got, want := rec.Body.String(), "date,calories\r\n"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestSanitizeFilenameFallbackAndAllowlist(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"nutrition-2026-10-20-2026-10-20.csv", "nutrition-2026-10-20-2026-10-20.csv"},
		{`bad"name` + "\n" + `.csv`, "bad_name_.csv"},
		{"???", "___"},
		{"", "export"},
	}
	for _, tc := range cases {
		if got := sanitizeFilename(tc.in); got != tc.want {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
