package handlers

// Cooldown day-boundary coverage. daysSinceLocalDateAt is the deterministic
// core of the check-in interval gate; it must count calendar days rather than
// elapsed 24-hour chunks, otherwise a late-evening check-in followed by an
// early-morning request 25 hours later would still report one day.

import (
	"testing"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/services/calculator"
)

func TestDaysSinceLocalDateAtUsesCalendarDays(t *testing.T) {
	// 01:00 on 20 Oct, check-in dated 18 Oct: 25 elapsed hours but two local
	// calendar days. Elapsed-hours division would report 1.
	now := time.Date(2026, 10, 20, 1, 0, 0, 0, time.UTC)
	got, ok := daysSinceLocalDateAt(now, "2026-10-18")
	if !ok {
		t.Fatal("valid date rejected")
	}
	if got != 2 {
		t.Fatalf("days since 2026-10-18 at 01:00 on 2026-10-20 = %d, want 2 calendar days", got)
	}

	// The boundary the caller cares about: just before midnight is still the
	// same day, not the next one.
	late := time.Date(2026, 10, 20, 23, 59, 59, 0, time.UTC)
	if got, _ := daysSinceLocalDateAt(late, "2026-10-20"); got != 0 {
		t.Fatalf("same-day distance = %d, want 0", got)
	}
	if got, _ := daysSinceLocalDateAt(late, "2026-10-19"); got != 1 {
		t.Fatalf("previous-day distance = %d, want 1", got)
	}

	// A non-UTC location still projects both date components onto midnights,
	// so the answer is independent of the caller's offset.
	loc := time.FixedZone("UTC-7", -7*60*60)
	localNoon := time.Date(2026, 10, 20, 12, 0, 0, 0, loc)
	if got, _ := daysSinceLocalDateAt(localNoon, "2026-10-18"); got != 2 {
		t.Fatalf("offset-local distance = %d, want 2", got)
	}

	if _, ok := daysSinceLocalDateAt(now, "not-a-date"); ok {
		t.Fatal("malformed date accepted")
	}
}

// TestBuildWeightTrendSeriesUnifiesRawAndTrend pins the single-source
// contract: raw display points and engine trend levels come out of the same
// pass, so they are index-aligned and the last fitted level equals the
// headline regression. It also covers duplicate-date collapsing and the
// display-window cutoff while the fit keeps its full-window span.
func TestBuildWeightTrendSeriesUnifiesRawAndTrend(t *testing.T) {
	const (
		since30    = "2026-10-11"
		sinceTrend = "2026-08-12"
	)

	bioAll := []models.BiometricLog{
		// Inside the trend window but outside the display window: contributes
		// to the fit (and burn-in index) but must not be emitted.
		{Date: "2026-10-01", WeightKg: 82.0},
		// Flat -> dip -> rise, nine distinct display days.
		{Date: "2026-11-01", WeightKg: 80.0},
		{Date: "2026-11-02", WeightKg: 80.0},
		{Date: "2026-11-03", WeightKg: 80.0},
		{Date: "2026-11-03", WeightKg: 79.8}, // duplicate date: latest wins
		{Date: "2026-11-04", WeightKg: 79.5},
		{Date: "2026-11-05", WeightKg: 79.0},
		{Date: "2026-11-06", WeightKg: 79.5},
		{Date: "2026-11-07", WeightKg: 80.0},
		{Date: "2026-11-07", WeightKg: 80.1}, // duplicate date: latest wins
		{Date: "2026-11-08", WeightKg: 80.5},
		{Date: "2026-11-09", WeightKg: 81.0},
	}

	points, series := buildWeightTrendSeries(bioAll, since30, sinceTrend)

	// Ten distinct trend-window dates collapse to nine display points; the two
	// duplicate rows must not inflate the raw array.
	if len(points) != 9 {
		t.Fatalf("points len = %d, want 9 distinct display dates", len(points))
	}
	if len(series) != len(points) {
		t.Fatalf("series len = %d, want %d (index-aligned with points)", len(series), len(points))
	}

	wantWeights := []float64{80.0, 80.0, 79.8, 79.5, 79.0, 79.5, 80.1, 80.5, 81.0}
	for i, want := range wantWeights {
		if points[i].WeightKg != want {
			t.Fatalf("points[%d].WeightKg = %v, want %v (latest value per date)", i, points[i].WeightKg, want)
		}
	}
	if points[0].Date != "2026-11-01" || points[len(points)-1].Date != "2026-11-09" {
		t.Fatalf("unexpected display span %s..%s", points[0].Date, points[len(points)-1].Date)
	}

	// Burn-in is the first MinTrendCurveDays-1 distinct points of the full
	// window. With the pre-window date consuming the first slot, display
	// indices 0..3 stay nil and indices 4..8 are fitted (the curve starts at
	// the 6th weigh-in so a two-point line exists by the 7th).
	for i := 0; i < calculator.MinTrendCurveDays-2; i++ {
		if series[i] != nil {
			t.Fatalf("series[%d] = %v, want nil burn-in", i, *series[i])
		}
	}
	for i := calculator.MinTrendCurveDays - 2; i < len(series); i++ {
		if series[i] == nil {
			t.Fatalf("series[%d] is nil, want fitted level", i)
		}
	}

	headline, _, days := calculator.ComputeWeightTrend(bioAll, sinceTrend)
	if days != 10 {
		t.Fatalf("trend days = %d, want 10 distinct trend-window days", days)
	}
	last := series[len(series)-1]
	if last == nil || *last != headline {
		t.Fatalf("last series level = %v, want headline %v", last, headline)
	}
}
