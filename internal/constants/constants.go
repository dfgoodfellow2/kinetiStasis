// Package constants defines named constants used across the diet-tracker backend.
// Centralising magic numbers here prevents silent drift when thresholds change.
package constants

import "time"

const (
	// DateFormat is the canonical YYYY-MM-DD string layout used for all date fields.
	DateFormat = "2006-01-02"

	// TimeFormat is the canonical RFC3339 layout used for all timestamp fields.
	TimeFormat = time.RFC3339

	// DefaultTDEELookbackDays is the default number of days used to compute observed TDEE
	// when the user has not set a custom value in their profile.
	DefaultTDEELookbackDays = 90

	// DefaultReadinessLookbackDays is the number of days of biometric data used
	// when computing the readiness score.
	DefaultReadinessLookbackDays = 30

	// DefaultExportLookbackDays is the default date range (in days back from today)
	// applied when no "from" query param is provided on export endpoints.
	DefaultExportLookbackDays = 30

	// MinCalorieFloor is the absolute minimum daily calorie target that the adaptive
	// TDEE algorithm will ever recommend, regardless of goal or adjustment.
	MinCalorieFloor = 1200.0

	// ReadinessEMAAlpha is the exponential moving average smoothing factor used
	// when computing grip and BOLT score trends for the readiness score.
	// Lower values = more smoothing, higher values = more reactive.
	ReadinessEMAAlpha = 0.3

	// MaxRequestBodyBytes is the maximum allowed HTTP request body size (10 MB).
	MaxRequestBodyBytes int64 = 10 << 20
)
