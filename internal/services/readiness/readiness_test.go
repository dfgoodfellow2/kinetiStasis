package readiness

import (
	"math"
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

	asOf := time.Now().UTC().Format("2006-01-02")
	res := ComputeReadiness(gripSeries(values), models.Profile{}, asOf)

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

// TestVelocityExcludesMissingDays pins the gap semantics: the 7-day averages
// that drive readiness velocity must use only days that actually carry a
// biometric reading. Zero-filling a missing day (the old behaviour) mixes "no
// measurement" with a genuine Rz of 0 ("at baseline") and biases the trend
// toward zero.
func TestVelocityExcludesMissingDays(t *testing.T) {
	asOf := time.Now().UTC().Format("2006-01-02")
	today, err := time.Parse("2006-01-02", asOf)
	if err != nil {
		t.Fatal(err)
	}

	// Day offsets from 13 days ago (0) to today (13). Absent offsets are gaps:
	// day 9 in the prior window, and days 4 and 1 in the current window.
	grip := map[int]float64{
		13: 100, 12: 101, 11: 102, 10: 101, 8: 103, 7: 100,
		6: 104, 5: 105, 3: 103, 2: 106, 0: 107,
	}
	var logs []models.BiometricLog
	for off := 13; off >= 0; off-- {
		v, ok := grip[off]
		if !ok {
			continue
		}
		logs = append(logs, models.BiometricLog{
			Date:   today.AddDate(0, 0, -off).Format("2006-01-02"),
			GripKg: v,
		})
	}

	currAvg, _, delta, _ := ComputeReadinessVelocity(logs, models.Profile{}, asOf)

	// Reproduce the expected per-day Rz independently, then average only the
	// days that were measured.
	offsets := []int{13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0}
	var prevRz, currRz []float64
	for i, off := range offsets {
		if _, ok := grip[off]; !ok {
			continue
		}
		date := today.AddDate(0, 0, -off).Format("2006-01-02")
		rz := ComputeReadiness(filterLogsUpTo(logs, date), models.Profile{}, date).Rz
		if i < 7 {
			prevRz = append(prevRz, rz)
		} else {
			currRz = append(currRz, rz)
		}
	}
	if len(prevRz) != 6 || len(currRz) != 5 {
		t.Fatalf("measured days = %d prior / %d current, want 6 / 5", len(prevRz), len(currRz))
	}
	sum := func(vs []float64) float64 {
		total := 0.0
		for _, v := range vs {
			total += v
		}
		return total
	}
	wantCurr := sum(currRz) / float64(len(currRz))
	wantPrev := sum(prevRz) / float64(len(prevRz))

	if math.Abs(currAvg-wantCurr) > 1e-9 {
		t.Fatalf("currAvg = %v, want %v (measured days only)", currAvg, wantCurr)
	}
	if math.Abs(delta-(wantCurr-wantPrev)) > 1e-9 {
		t.Fatalf("delta = %v, want %v", delta, wantCurr-wantPrev)
	}
	// With 5 current readings, dividing the same sum by 7 would differ whenever
	// the measured Rz values are not all zero — proving gaps are excluded.
	if oldCurr := sum(currRz) / 7.0; math.Abs(currAvg-oldCurr) < 1e-9 {
		t.Fatalf("currAvg = %v matches the zero-filled 7-day average; gaps were not excluded", currAvg)
	}
}

// TestVelocityNoReadingsInWindowStaysFlat verifies the division-by-zero guard:
// when one window contains no measured day there is no comparable trend, so
// delta must stay 0 rather than measuring against an empty window's zero.
func TestVelocityNoReadingsInWindowStaysFlat(t *testing.T) {
	asOf := time.Now().UTC().Format("2006-01-02")
	today, err := time.Parse("2006-01-02", asOf)
	if err != nil {
		t.Fatal(err)
	}

	// Readings exist only in the prior window (days 13..7).
	var logs []models.BiometricLog
	for off := 13; off >= 7; off-- {
		logs = append(logs, models.BiometricLog{
			Date:   today.AddDate(0, 0, -off).Format("2006-01-02"),
			GripKg: 100 + float64(13-off),
		})
	}

	_, _, delta, arrow := ComputeReadinessVelocity(logs, models.Profile{}, asOf)
	if delta != 0 {
		t.Fatalf("delta = %v, want 0 with no current-window readings", delta)
	}
	if arrow != "→" {
		t.Fatalf("arrow = %q, want flat arrow", arrow)
	}
}

// TestNegativeGripStreakEscalatesToRed verifies that three consecutive days at
// |z| >= 2 BELOW baseline trip the severe zone and escalate to red. The series
// is tuned (120 kg baseline, then 115/90/60 kg) so every trailing day clears
// the -2 threshold; the last value is deliberately still plausible.
func TestNegativeGripStreakEscalatesToRed(t *testing.T) {
	values := repeat(120, 14)
	values = append(values, 115, 90, 60)

	asOf := time.Now().UTC().Format("2006-01-02")
	res := ComputeReadiness(gripSeries(values), models.Profile{}, asOf)

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
