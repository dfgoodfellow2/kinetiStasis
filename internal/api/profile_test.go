package api_test

// Profile validation tests. bfPct feeds Katch-McArdle BMR, so the profile
// endpoint must reject implausible percentages instead of persisting them.
// The matrix pinned here is:
//
//   95   -> 400 (above the 70% ceiling)
//   24.5 -> 200 (normal value)
//   0    -> 200 (not provided)

import (
	"net/http"
	"testing"
)

func TestProfileRejectsImplausibleBodyFat(t *testing.T) {
	h := newHarness(t)

	h.do(t, http.MethodPut, "/v1/profile", map[string]any{
		"sex":   "male",
		"bfPct": 95,
	}, http.StatusBadRequest)

	h.do(t, http.MethodPut, "/v1/profile", map[string]any{
		"sex":   "male",
		"bfPct": 24.5,
	}, http.StatusOK)

	h.do(t, http.MethodPut, "/v1/profile", map[string]any{
		"sex":   "male",
		"bfPct": 0,
	}, http.StatusOK)
}
