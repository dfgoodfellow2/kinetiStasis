// Package validate provides small, dependency-free validation helpers shared
// by HTTP handlers. Helpers return descriptive errors so callers can surface
// them directly with respond.Error(w, http.StatusBadRequest, err.Error()).
package validate

import (
	"fmt"
	"math"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/constants"
)

// Date enforces a strict YYYY-MM-DD calendar date. Values such as "2024-1-2"
// (non-padded) or "2024-02-30" (out of range) are rejected.
func Date(s string) error {
	const msg = "date must be in YYYY-MM-DD format"
	if len(s) != len(constants.DateFormat) {
		return fmt.Errorf("%s", msg)
	}
	t, err := time.Parse(constants.DateFormat, s)
	if err != nil || t.Format(constants.DateFormat) != s {
		return fmt.Errorf("%s", msg)
	}
	return nil
}

// NonNegativeBounded checks that v is a finite number in [0, max].
// NaN and ±Inf are rejected, as are negative values and values above max.
func NonNegativeBounded(name string, v, max float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return fmt.Errorf("%s must be a finite number", name)
	}
	if v < 0 {
		return fmt.Errorf("%s must be >= 0", name)
	}
	if v > max {
		return fmt.Errorf("%s must be <= %g", name, max)
	}
	return nil
}

// NonNegativeBoundedInt checks that v is in [0, max].
func NonNegativeBoundedInt(name string, v, max int) error {
	if v < 0 {
		return fmt.Errorf("%s must be >= 0", name)
	}
	if v > max {
		return fmt.Errorf("%s must be <= %d", name, max)
	}
	return nil
}
