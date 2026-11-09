// Package validate provides small, dependency-free validation helpers shared
// by HTTP handlers. Helpers return descriptive errors so callers can surface
// them directly with respond.Error(w, http.StatusBadRequest, err.Error()).
package validate

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/constants"
)

// Username enforces the registration username shape: 3–32 characters from
// [a-zA-Z0-9._-]. It is deliberately ASCII-only so usernames stay predictable
// in logs, URLs and the UI.
func Username(s string) error {
	if len(s) < 3 || len(s) > 32 {
		return fmt.Errorf("username must be 3-32 characters")
	}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			continue
		}
		return fmt.Errorf("username may only contain letters, digits, dot, underscore and hyphen")
	}
	return nil
}

// Email is a deliberately lightweight sanity check, not an RFC parser: at most
// 254 bytes, exactly one '@', no whitespace, and non-empty local/domain parts.
// Requiring deliverability is out of scope; this only blocks obvious junk
// before it reaches the database.
func Email(s string) error {
	const msg = "email address is invalid"
	if len(s) > 254 || strings.Count(s, "@") != 1 {
		return fmt.Errorf("%s", msg)
	}
	if strings.ContainsAny(s, " \t\r\n") {
		return fmt.Errorf("%s", msg)
	}
	local, domain, _ := strings.Cut(s, "@")
	if local == "" || domain == "" {
		return fmt.Errorf("%s", msg)
	}
	return nil
}

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

// NumericRange checks that v is a finite number in [min, max]. NaN and ±Inf
// are rejected up front so they can never slip past the bounds comparisons.
func NumericRange(name string, v, min, max float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return fmt.Errorf("%s must be a finite number", name)
	}
	if v < min {
		return fmt.Errorf("%s must be >= %g", name, min)
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
