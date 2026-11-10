package api_test

// Calibration-shield policy tests. The shield is a 14-day window measured from
// the account's earliest weight-bearing date. Inside it (and only when the
// check-in and weight-reliability gates are BOTH open) the weekly ±500 kcal
// adjustment is tightened to ±150, with a raise-only escape when the unclamped
// ideal sits more than 1200 kcal above the current target.
//
// All dates here are relative to the real clock, not the fixed testDate used
// by the older check-in tests, because the shield compares the first weigh-in
// against time.Now(). The weight history still spans >= 7 calendar days so the
// engine's reliability gate is satisfied.
//
// Seeding uses doFromIP with distinct addresses: the full stack runs the
// production rate limiter (100 req/min, burst 20), and a dense seed loop from
// one client would be rejected.

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
)

// localToday is the same calendar day the handler's daysSinceLocalDate helper
// projects onto local midnights.
func localToday() string {
	return time.Now().Format("2006-01-02")
}

// seedDailyWeights writes one plausible weight day for every offset in
// [firstDay, 0], inclusive, relative to today. The first row is therefore the
// account's first weigh-in (and the calibration clock's start).
func seedDailyWeights(t *testing.T, h *harness, today string, firstDay int) {
	t.Helper()
	for i := firstDay; i <= 0; i++ {
		doFromIP(t, h, fmt.Sprintf("198.51.100.%d", i+120), http.MethodPost, "/v1/biometrics", map[string]any{
			"date":     shiftDate(today, i),
			"weightKg": 82.0,
		}, http.StatusOK)
	}
}

// seedDailyIntake writes a constant calorie intake for every offset in
// [firstDay, 0]. Flat weight plus constant intake makes the observed TDEE equal
// the intake, so the expected ideal target is deterministic.
func seedDailyIntake(t *testing.T, h *harness, today string, firstDay int, calories float64) {
	t.Helper()
	for i := firstDay; i <= 0; i++ {
		doFromIP(t, h, fmt.Sprintf("203.0.113.%d", i+120), http.MethodPost, "/v1/nutrition/logs", map[string]any{
			"date":     shiftDate(today, i),
			"calories": calories,
		}, http.StatusCreated)
	}
}

// TestCalibrationShieldCapsRaiseMidWindow is the core shield case: reliably
// measured, still calibrating, unclamped ideal is +765. The cap must hold the
// recommendation to current+150, and Create must persist the same value.
//
// Note: the shield starts at the FIRST weigh-in, so an account can only be
// calibrating AND weight-reliable once its history spans the 7 calendar days
// the reliability gate requires. Ten days back is the earliest comfortable
// in-window date; five days back cannot be weight-reliable by construction.
func TestCalibrationShieldCapsRaiseMidWindow(t *testing.T) {
	h := newHarness(t)
	seedCheckinProfile(t, h)
	seedCheckinTargets(t, h, 2000)

	today := localToday()
	const firstDay = -10
	seedDailyWeights(t, h, today, firstDay)
	seedDailyIntake(t, h, today, firstDay, 2765) // maintenance goal: ideal 2765

	preview := decodeJSON[models.CheckinPreview](t, h.do(t, http.MethodGet, "/v1/checkin", nil, http.StatusOK))
	if !preview.CanCheckIn {
		t.Fatalf("CanCheckIn = false, want true: %+v", preview)
	}
	if !preview.WeightReliable {
		t.Fatalf("WeightReliable = false, want true with 11 weight days spanning 10 days: %+v", preview)
	}
	if !preview.Calibrating {
		t.Fatalf("Calibrating = false, want true on day %d of the shield", preview.CalibrationDay)
	}
	if preview.CalibrationDay != 11 {
		t.Errorf("CalibrationDay = %d, want 11 (first weigh-in 10 days ago)", preview.CalibrationDay)
	}
	if preview.CalorieAdjustment != 150 {
		t.Errorf("CalorieAdjustment = %v, want 150 (shield cap)", preview.CalorieAdjustment)
	}
	if preview.RecommendedCalories != 2150 {
		t.Fatalf("RecommendedCalories = %d, want 2000 + 150 = 2150", preview.RecommendedCalories)
	}

	// Create must mirror Preview exactly: no second, divergent clamp.
	created := decodeJSON[models.CheckInLog](t, h.do(t, http.MethodPost, "/v1/checkin", map[string]any{
		"caloriesAfter": preview.RecommendedCalories,
	}, http.StatusCreated))
	if created.CaloriesAfter != 2150 {
		t.Errorf("persisted CaloriesAfter = %d, want 2150", created.CaloriesAfter)
	}
	targets := decodeJSON[models.Targets](t, h.do(t, http.MethodGet, "/v1/targets", nil, http.StatusOK))
	if targets.Calories != 2150 {
		t.Errorf("targets calories = %v, want 2150", targets.Calories)
	}
}

// TestCalibrationShieldExpiresAfterWindow pins the expiry: once the first
// weigh-in is 15 days old the normal ±500 weekly clamp applies again.
func TestCalibrationShieldExpiresAfterWindow(t *testing.T) {
	h := newHarness(t)
	seedCheckinProfile(t, h)
	seedCheckinTargets(t, h, 2000)

	today := localToday()
	const firstDay = -15
	seedDailyWeights(t, h, today, firstDay)
	seedDailyIntake(t, h, today, firstDay, 2900) // ideal +900, normal clamp +500

	preview := decodeJSON[models.CheckinPreview](t, h.do(t, http.MethodGet, "/v1/checkin", nil, http.StatusOK))
	if !preview.WeightReliable {
		t.Fatalf("WeightReliable = false, want true with 16 weight days: %+v", preview)
	}
	if preview.Calibrating {
		t.Errorf("Calibrating = true after the 14-day window (day %d), want false", preview.CalibrationDay)
	}
	if preview.CalibrationDay != 0 {
		t.Errorf("CalibrationDay = %d, want 0 outside the window", preview.CalibrationDay)
	}
	if preview.RecommendedCalories != 2500 {
		t.Fatalf("RecommendedCalories = %d, want 2000 + 500 = 2500 after expiry", preview.RecommendedCalories)
	}
}

// TestCalibrationShieldSevereDeficitBackstop is the safety escape: a +1400
// unclamped ideal must bypass the ±150 shield and raise by the full +500.
func TestCalibrationShieldSevereDeficitBackstop(t *testing.T) {
	h := newHarness(t)
	seedCheckinProfile(t, h)
	seedCheckinTargets(t, h, 2000)

	today := localToday()
	const firstDay = -10
	seedDailyWeights(t, h, today, firstDay)
	seedDailyIntake(t, h, today, firstDay, 3400) // ideal +1400 > 1200 backstop

	preview := decodeJSON[models.CheckinPreview](t, h.do(t, http.MethodGet, "/v1/checkin", nil, http.StatusOK))
	if !preview.Calibrating {
		t.Fatalf("Calibrating = false, want true inside the shield: %+v", preview)
	}
	if preview.RecommendedCalories != 2500 {
		t.Fatalf("RecommendedCalories = %d, want 2000 + 500 = 2500 via backstop", preview.RecommendedCalories)
	}
}

// TestCalibrationShieldCapsDecreaseEvenWhenLarge pins that the backstop is
// raise-only: a -1400 ideal during the shield is still capped at -150.
func TestCalibrationShieldCapsDecreaseEvenWhenLarge(t *testing.T) {
	h := newHarness(t)
	seedCheckinProfile(t, h)
	seedCheckinTargets(t, h, 2500)

	today := localToday()
	const firstDay = -10
	seedDailyWeights(t, h, today, firstDay)
	// 1100 is above the engine's plausibility floor (1000), so it is not
	// replaced by the static estimate. Ideal = 1100, delta = -1400.
	seedDailyIntake(t, h, today, firstDay, 1100)

	preview := decodeJSON[models.CheckinPreview](t, h.do(t, http.MethodGet, "/v1/checkin", nil, http.StatusOK))
	if !preview.Calibrating {
		t.Fatalf("Calibrating = false, want true inside the shield: %+v", preview)
	}
	if preview.RecommendedCalories != 2350 {
		t.Fatalf("RecommendedCalories = %d, want 2500 - 150 = 2350 (no downward backstop)", preview.RecommendedCalories)
	}
}

// TestCalibrationDoesNotUnfreezeUnreliableWeights guards the badge-priority
// rule: inside the 14-day window but with <7 weight days, the frozen path wins
// and calibrating stays false.
func TestCalibrationDoesNotUnfreezeUnreliableWeights(t *testing.T) {
	h := newHarness(t)
	seedCheckinProfile(t, h)
	seedCheckinTargets(t, h, 2000)

	today := localToday()
	// Three weigh-ins, all inside the shield window and close together: the
	// check-in gate opens but the reliability gate does not.
	seedDailyWeights(t, h, today, -2)

	preview := decodeJSON[models.CheckinPreview](t, h.do(t, http.MethodGet, "/v1/checkin", nil, http.StatusOK))
	if !preview.CanCheckIn {
		t.Fatalf("CanCheckIn = false, want true with 3 weigh-ins: %+v", preview)
	}
	if preview.WeightReliable {
		t.Fatalf("WeightReliable = true with 3 close weight days, want false: %+v", preview)
	}
	if !preview.TargetUpdateBlocked {
		t.Error("TargetUpdateBlocked = false, want true on the frozen path")
	}
	if preview.Calibrating {
		t.Error("Calibrating = true with unreliable weights, want false")
	}
	if preview.RecommendedCalories != 2000 {
		t.Errorf("RecommendedCalories = %d, want current target 2000 (frozen)", preview.RecommendedCalories)
	}
}
