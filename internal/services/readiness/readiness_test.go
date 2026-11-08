package readiness

import (
	"strings"
	"testing"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
)

// gripSeries builds one daily grip log per value, oldest first and ending
// today, so the readiness streak window sees consecutive calendar days.
func gripSeries(values []float64) []models.BiometricLog {
	today := time.Now().UTC()
	logs := make([]models.BiometricLog, len(values))
	for i, v := range values {
		date := today.AddDate(0, 0, -(len(values) - 1 - i)).Format("2006-01-02")
		logs[i] = models.BiometricLog{Date: date, GripKg: v}
	}
	return logs
}

// repeat builds a slice of n copies of v.
func repeat(v float64, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = v
	}
	return out
}

// hasNote reports whether want appears among the readiness notes.
func hasNote(notes []string, want string) bool {
	for _, note := range notes {
		if strings.Contains(note, want) {
			return true
		}
	}
	return false
}

// TestPositiveGripStreakStaysGreen pins the direction rule: a positive
// deviation run is informational and must never escalate readiness, even
// though the three trailing days are all above baseline by |z| >= 2.
func TestPositiveGripStreakStaysGreen(t *testing.T) {
	values := repeat(120, 11)
	values = append(values, 133.18, 145.40, 190.68)

	res := ComputeReadiness(gripSeries(values), models.Profile{})

	if res.GripZ < 1.0 {
		t.Fatalf("GripZ = %v, want positive informational deviation >= 1", res.GripZ)
	}
	if res.Level != "green" {
		t.Fatalf("Level = %q, want green (positive z must not escalate)", res.Level)
	}
	if res.ConsecutiveDays != 0 {
		t.Fatalf("ConsecutiveDays = %d, want 0 (only negative z counts)", res.ConsecutiveDays)
	}
	if !hasNote(res.Notes, "Grip above baseline (informational)") {
		t.Fatalf("Notes = %v, want an informational above-baseline note", res.Notes)
	}
	for _, note := range res.Notes {
		if strings.Contains(note, "Rz ≤") {
			t.Fatalf("Notes = %v, positive streak must not add a negative-deviation note", res.Notes)
		}
	}
}

// TestNegativeGripStreakEscalatesToRed verifies that three consecutive days at
// |z| >= 2 BELOW baseline trip the severe zone and escalate to red. The series
// is tuned (120 kg baseline, then 115/90/60 kg) so every trailing day clears
// the -2 threshold; the last value is deliberately still plausible.
func TestNegativeGripStreakEscalatesToRed(t *testing.T) {
	values := repeat(120, 14)
	values = append(values, 115, 90, 60)

	res := ComputeReadiness(gripSeries(values), models.Profile{})

	if res.GripZ > -2.0 {
		t.Fatalf("GripZ = %v, want <= -2 for the severe zone", res.GripZ)
	}
	if res.Level != "red" {
		t.Fatalf("Level = %q, want red for three |z| >= 2 negative days", res.Level)
	}
	if res.ConsecutiveDays != 3 {
		t.Fatalf("ConsecutiveDays = %d, want 3", res.ConsecutiveDays)
	}
	if !strings.Contains(res.Message, "Severe deviation 3 days") {
		t.Fatalf("Message = %q, want a severe-deviation message", res.Message)
	}
	if !hasNote(res.Notes, "Rz ≤ -2 for 3 consecutive days") {
		t.Fatalf("Notes = %v, want a consecutive negative-deviation note", res.Notes)
	}
}
