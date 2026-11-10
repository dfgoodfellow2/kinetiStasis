// Package constants defines named constants used across the diet-tracker backend.
// Centralising magic numbers here prevents silent drift when thresholds change.
package constants

import "time"

const (
	// DateFormat is the canonical YYYY-MM-DD string layout used for all date fields.
	DateFormat = "2006-01-02"

	// TimeFormat is the canonical RFC3339 layout used for all timestamp fields.
	TimeFormat = time.RFC3339

	// FormatResourceTimestamp is the fixed-width timestamp layout used for
	// resource ordering keys (target_history.created_at, check_in_logs.created_at).
	// A constant 9-digit fraction keeps lexical string comparison chronological;
	// RFC3339Nano's variable-length fraction does not.
	FormatResourceTimestamp = "2006-01-02T15:04:05.000000000Z07:00"

	// DefaultTDEELookbackDays is the default number of days used to compute observed TDEE
	// when the user has not set a custom value in their profile.
	DefaultTDEELookbackDays = 90

	// DefaultReadinessLookbackDays is the number of days of biometric data used
	// when computing the readiness score.
	DefaultReadinessLookbackDays = 30

	// DefaultExportLookbackDays is the default date range (in days back from today)
	// applied when no "from" query param is provided on export endpoints.
	DefaultExportLookbackDays = 30

	// MinCheckinIntervalDays is the minimum number of days that must elapse
	// between weekly check-ins to prevent reactive adjustments from noisy data.
	MinCheckinIntervalDays = 5

	// MinWeighInsForCheckin is the minimum number of distinct weight-bearing
	// biometric logs (WeightKg > 0) inside the lookback window required to
	// perform a weekly check-in. The gate is intentionally about WEIGH-INS, not
	// arbitrary biometric rows: sleep/grip-only rows must never unlock it.
	MinWeighInsForCheckin = 3

	// MinBiometricLogsForCheckin is retained as a compatibility alias for the
	// old name. It means the same thing as MinWeighInsForCheckin.
	MinBiometricLogsForCheckin = MinWeighInsForCheckin

	// MinTrendCurveDays is the display-curve burn-in: the drawn weight-trend
	// series starts one weigh-in earlier than the headline gate (6 distinct
	// days) so a two-point line exists by the 7th weigh-in. The headline's
	// 7-day reliability gate is unchanged.
	MinTrendCurveDays = 6

	// MinCalorieFloor is the absolute minimum daily calorie target that the adaptive
	// TDEE algorithm will ever recommend, regardless of goal or adjustment.
	MinCalorieFloor = 1200.0

	// CalibrationShieldDays is the length of the post-first-weigh-in calibration
	// window, measured in calendar days from the account's earliest
	// weight-bearing date. The SBS/MacroFactor calibration period is typically
	// 14-30 days of consistent logging before the energy-balance model is
	// trusted to make large changes. We deliberately pick the SHORT end (14):
	// it still covers the water-weight and glycogen noise that makes early trend
	// estimates unreliable, while a longer shield feels like the app is ignoring
	// the user's data — the family-friction failure mode where people abandon
	// logging because targets never move.
	CalibrationShieldDays = 14

	// CalibrationMaxAdjustment caps a target change to ±150 kcal while the
	// account is inside the calibration shield. This is the small, reversible
	// step phase: early progress can still move the target, but a noisy
	// two-week trend cannot swing calories by the full weekly maximum.
	CalibrationMaxAdjustment = 150.0

	// CalibrationBackstopKcal is the severe-undereating escape hatch. When the
	// unclamped ideal target is more than 1200 kcal ABOVE the current target,
	// the shield's ±150 cap is ignored and the normal weekly swing applies to
	// raises. It never loosens a decrease: the backstop is raise-only so a
	// bad high estimate cannot crash someone's calories during calibration.
	CalibrationBackstopKcal = 1200.0

	// ReadinessEMAAlpha is the exponential moving average smoothing factor used
	// when computing grip and BOLT score trends for the readiness score.
	// Lower values = more smoothing, higher values = more reactive.
	ReadinessEMAAlpha = 0.3

	// MaxRequestBodyBytes is the maximum allowed HTTP request body size (10 MB).
	MaxRequestBodyBytes int64 = 10 << 20

	// GeminiParseTimeout bounds a single Gemini parse call (including the one
	// automatic retry for 429/503). It is deliberately shorter than the server
	// WriteTimeout so a model call can never outlive the HTTP response budget.
	GeminiParseTimeout = 45 * time.Second

	// GeminiMealMaxOutputTokens / GeminiWorkoutMaxOutputTokens cap generated
	// output so a runaway model response cannot blow up latency or cost.
	GeminiMealMaxOutputTokens    int32 = 2048
	GeminiWorkoutMaxOutputTokens int32 = 4096

	// MaxParseTextChars is the maximum accepted length of user-supplied parse
	// text (meal descriptions and workout text/YAML). Long inputs are rejected
	// with 400 before any model call is made.
	MaxParseTextChars = 4000

	// Meal plausibility bounds applied to AI-parsed results before they are
	// returned for human review. Zero calories are preserved for the explicit
	// "no food visible" result; the floor only applies to non-zero estimates.
	MinMealCalories   = 50.0
	MaxMealCalories   = 6000.0
	MaxMealMacroGrams = 1500.0

	// Workout plausibility bounds applied to AI-parsed results.
	MaxWorkoutSets       = 100
	MaxRestSecondsPerSet = 600.0
	MaxRepsPerSet        = 100
)
