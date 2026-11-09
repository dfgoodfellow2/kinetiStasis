package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/respond"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"
)

// IntegrityHandler serves the admin-only, read-only data-integrity report. It
// never writes to the database; the owner reads the findings and decides what
// to heal.
type IntegrityHandler struct {
	s store.Store
}

// NewIntegrityHandler creates a new IntegrityHandler.
func NewIntegrityHandler(s store.Store) *IntegrityHandler {
	return &IntegrityHandler{s: s}
}

// integrityResponse is the stable wire shape for GET /v1/admin/integrity.
// generatedAt is emitted on every successful scan; errors is always present
// (possibly empty) so the PWA can render partial results without branching on
// key presence.
type integrityResponse struct {
	GeneratedAt string                   `json:"generatedAt"`
	Results     []store.IntegrityFinding `json:"results"`
	Errors      []string                 `json:"errors"`
}

// Report handles GET /v1/admin/integrity.
func (h *IntegrityHandler) Report(w http.ResponseWriter, r *http.Request) {
	results, errs := h.s.IntegrityReport(r.Context())
	if results == nil {
		results = []store.IntegrityFinding{}
	}
	if errs == nil {
		errs = []string{}
	}

	if len(errs) > 0 {
		// The report is intentionally partial rather than failing whole: every
		// check that succeeded is still useful evidence.
		slog.Warn("integrity report completed with errors", "failed_checks", len(errs))
	}

	respond.JSON(w, http.StatusOK, integrityResponse{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Results:     results,
		Errors:      errs,
	})
}
