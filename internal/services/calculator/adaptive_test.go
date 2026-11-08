package calculator

import (
	"math"
	"testing"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/services/nutrition"
)

// TestComputeObservedTDEEStaticFallbackMatchesNutrition guards the shared
// static-estimate path: when the observed-TDEE tiering runs, the accompanying
// EstimatedTDEE must be exactly what the nutrition service produces for the
// same profile and weight. This covers birthdate-derived age, legacy stored
// age, the no-age default of 30, the BF% gate, and the running-calorie add.
func TestComputeObservedTDEEStaticFallbackMatchesNutrition(t *testing.T) {
	const weightKg = 82.0

	today := time.Now().Format("2006-01-02")
	logs := []models.NutritionLog{{Date: today, Calories: 2000}}
	biometrics := []models.BiometricLog{{Date: today, WeightKg: weightKg}}

	base := models.Profile{
		Sex:          "male",
		HeightCm:     180,
		Activity:     "moderately_active",
		ExerciseFreq: 3,
		Goal:         "maintenance",
	}

	cases := []struct {
		name    string
		mutate  func(*models.Profile)
		bfParam float64
	}{
		{
			name:   "age derived from birthdate",
			mutate: func(p *models.Profile) { p.Birthdate = "1990-01-15" },
		},
		{
			name:   "legacy profile with stored age",
			mutate: func(p *models.Profile) { p.Age = 47 },
		},
		{
			name:   "legacy profile with no age defaults to 30",
			mutate: func(p *models.Profile) {},
		},
		{
			name:   "body fat over 60 uses Katch-McArdle",
			mutate: func(p *models.Profile) { p.BfPct = 65 },
		},
		{
			name:   "runner includes running calories",
			mutate: func(p *models.Profile) { p.RunningKm = 40 },
		},
		{
			name:    "biometric body fat argument fills empty profile body fat",
			mutate:  func(p *models.Profile) {},
			bfParam: 22,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			profile := base
			tc.mutate(&profile)

			expectedProfile := profile
			if tc.bfParam > 0 && expectedProfile.BfPct <= 0 {
				expectedProfile.BfPct = tc.bfParam
			}

			want := nutrition.ComputeStaticTDEE(expectedProfile, weightKg)
			got := ComputeObservedTDEE(logs, biometrics, profile, tc.bfParam)

			if got.EstimatedTDEE != want {
				t.Fatalf("EstimatedTDEE = %v, want %v (nutrition.ComputeStaticTDEE)", got.EstimatedTDEE, want)
			}

			// The sub-four-day tier now uses a plain mean; the statistical
			// tiering must remain untouched by the static delegation.
			if got.Method != "mean" {
				t.Errorf("Method = %q, want %q", got.Method, "mean")
			}
			if got.ObservedTDEE != 2000 {
				t.Errorf("ObservedTDEE = %v, want 2000", got.ObservedTDEE)
			}
		})
	}
}

// TestComputeObservedTDEEHighBodyFatDiffersFromMifflin documents the intended
// behavior change: profiles with BF% >= 60 now route through Katch-McArdle
// (nutrition's < 100 gate) instead of the previous inline < 60 gate.
func TestComputeObservedTDEEHighBodyFatDiffersFromMifflin(t *testing.T) {
	const (
		weightKg = 82.0
		bfPct    = 65.0
	)

	today := time.Now().Format("2006-01-02")
	logs := []models.NutritionLog{{Date: today, Calories: 2000}}
	biometrics := []models.BiometricLog{{Date: today, WeightKg: weightKg}}

	profile := models.Profile{
		Sex:          "male",
		HeightCm:     180,
		Age:          36,
		Activity:     "moderately_active",
		ExerciseFreq: 3,
		BfPct:        bfPct,
	}

	got := ComputeObservedTDEE(logs, biometrics, profile, 0).EstimatedTDEE

	katch := nutrition.EstimateBMRKatchMcArdle(weightKg, bfPct)
	if katch <= 0 {
		t.Fatalf("expected a valid Katch-McArdle BMR, got %v", katch)
	}
	want := nutrition.ComputeStaticTDEE(profile, weightKg)
	if got != want {
		t.Fatalf("EstimatedTDEE = %v, want %v", got, want)
	}

	mifflinOnly := nutrition.EstimateBMRMifflin(weightKg, profile.HeightCm, profile.Age, true)
	mifflinTDEE := math.Round(nutrition.EstimateTDEEBase(mifflinOnly, profile.Activity)*
		nutrition.EstimateEATFactor(profile.ExerciseFreq)) +
		float64(nutrition.CalcRunningCalories(weightKg, profile.RunningKm))
	if got == mifflinTDEE {
		t.Errorf("EstimatedTDEE = %v matches Mifflin-only estimate; expected Katch-McArdle", got)
	}
}

// windowDates returns n consecutive YYYY-MM-DD dates ending today, oldest
// first. Tests deliberately anchor to the wall clock because the estimator
// applies its own lookback cutoff internally.
func windowDates(n int) []string {
	now := time.Now()
	out := make([]string, n)
	for i := range out {
		out[i] = now.AddDate(0, 0, -(n - 1 - i)).Format("2006-01-02")
	}
	return out
}

func nutritionSeries(dates []string, calories ...float64) []models.NutritionLog {
	logs := make([]models.NutritionLog, len(dates))
	for i, date := range dates {
		cals := 0.0
		if len(calories) > 0 {
			cals = calories[0]
		}
		if i < len(calories) {
			cals = calories[i]
		}
		logs[i] = models.NutritionLog{Date: date, Calories: cals}
	}
	return logs
}

func weightSeries(dates []string, startKg, totalChangeKg float64) []models.BiometricLog {
	bios := make([]models.BiometricLog, len(dates))
	span := float64(len(dates) - 1)
	for i, date := range dates {
		w := startKg
		if span > 0 {
			w += totalChangeKg * float64(i) / span
		}
		bios[i] = models.BiometricLog{Date: date, WeightKg: w}
	}
	return bios
}

// TestComputeObservedTDEEFlatWeightEqualsIntake pins the energy-balance model
// at zero weight change: expenditure must equal the intake estimate.
func TestComputeObservedTDEEFlatWeightEqualsIntake(t *testing.T) {
	dates := windowDates(8)
	logs := nutritionSeries(dates, 2900)
	bios := weightSeries(dates, 82.0, 0)

	res := ComputeObservedTDEE(logs, bios, models.Profile{}, 0)

	if !res.WeightReliable {
		t.Fatal("WeightReliable = false, want true for 8 weigh-ins spanning 7 days")
	}
	if math.Abs(res.ObservedTDEE-2900) > 25 {
		t.Fatalf("ObservedTDEE = %v, want 2900 ±25 (flat weight => intake)", res.ObservedTDEE)
	}
}

// TestComputeObservedTDEEGainSubtractsStoredEnergy pins the SBS/MacroFactor
// identity: intake - (weight slope x 7700 kcal/kg). A +0.5 kg/week trend stores
// 550 kcal/day, so observed expenditure is 550 below intake.
func TestComputeObservedTDEEGainSubtractsStoredEnergy(t *testing.T) {
	dates := windowDates(8)
	logs := nutritionSeries(dates, 2900)
	bios := weightSeries(dates, 82.0, 0.5) // +0.5 kg across 7 days

	res := ComputeObservedTDEE(logs, bios, models.Profile{}, 0)

	if !res.WeightReliable {
		t.Fatal("WeightReliable = false, want true")
	}
	// 2900 - (0.5/7)*7700 = 2900 - 550 = 2350.
	if math.Abs(res.ObservedTDEE-2350) > 30 {
		t.Fatalf("ObservedTDEE = %v, want 2350 ±30 (intake - 550 kcal/day stored)", res.ObservedTDEE)
	}
}

// TestComputeObservedTDEEShortWeightHistoryIsIntakeOnly verifies that fewer
// than seven weight-bearing log days disables the body-energy term entirely.
func TestComputeObservedTDEEShortWeightHistoryIsIntakeOnly(t *testing.T) {
	dates := windowDates(8)
	logs := nutritionSeries(dates, 2900)
	bios := weightSeries(dates[2:], 82.0, 6.0) // only 6 weigh-ins, no term

	res := ComputeObservedTDEE(logs, bios, models.Profile{}, 0)

	if res.WeightReliable {
		t.Fatal("WeightReliable = true, want false for 6 weight days")
	}
	if !res.TargetUpdateBlocked {
		t.Error("TargetUpdateBlocked = false, want true when weight is unreliable")
	}
	// Intake-only estimate over a constant series is exactly 2900.
	if math.Abs(res.ObservedTDEE-2900) > 25 {
		t.Fatalf("ObservedTDEE = %v, want intake-only 2900 ±25", res.ObservedTDEE)
	}
}

// TestComputeObservedTDEEShortSeriesUsesFittedLevelNotForecast guards the raise
// bug: [3000,3200,3400,4600] must yield the regression level at the last
// observation (≈4300), not the one-step forecast beyond the series (4800).
func TestComputeObservedTDEEShortSeriesUsesFittedLevelNotForecast(t *testing.T) {
	dates := windowDates(4)
	spikes := []float64{3000, 3200, 3400, 4600}
	logs := nutritionSeries(dates, spikes...)

	res := ComputeObservedTDEE(logs, nil, models.Profile{}, 0)

	if res.Method != "linear_regression" {
		t.Fatalf("Method = %q, want linear_regression", res.Method)
	}
	slope, intercept, forecast := LinearRegression(spikes)
	fittedLevel := slope*float64(len(spikes)-1) + intercept

	if math.Abs(res.ObservedTDEE-fittedLevel) > 1 {
		t.Fatalf("ObservedTDEE = %v, want fitted level %v", res.ObservedTDEE, fittedLevel)
	}
	if res.ObservedTDEE >= forecast {
		t.Fatalf("ObservedTDEE = %v must be below the one-step forecast %v", res.ObservedTDEE, forecast)
	}
	// No forecast amplification: 4800 was the bug; the fitted level stays in a
	// sane band around the observed series.
	if res.ObservedTDEE > 4400 {
		t.Fatalf("ObservedTDEE = %v, want <= 4400 (no forecast amplification)", res.ObservedTDEE)
	}
	if res.ObservedTDEE < 3400 {
		t.Fatalf("ObservedTDEE = %v, want >= 3400 (fitted level, not collapsed)", res.ObservedTDEE)
	}
}

// TestComputeWeightTrendWeeklySparseRate guards the row-index vs calendar-day
// bug: seven weekly weigh-ins a kilogram apart are a truthful -1 kg/week trend,
// not the -7 kg/week that index-based regression produced.
func TestComputeWeightTrendWeeklySparseRate(t *testing.T) {
	start := time.Now().AddDate(0, 0, -42)
	bios := make([]models.BiometricLog, 7)
	for i := range bios {
		bios[i] = models.BiometricLog{
			Date:     start.AddDate(0, 0, 7*i).Format("2006-01-02"),
			WeightKg: 80 - float64(i),
		}
	}

	trend, rate, days := ComputeWeightTrend(bios, start.Format("2006-01-02"))

	if days != 7 {
		t.Fatalf("days = %d, want 7 distinct weigh-in days", days)
	}
	if math.Abs(rate-(-1.0)) > 0.3 {
		t.Fatalf("rateKgPerWeek = %v, want ≈ -1 kg/week ±0.3", rate)
	}
	// Explicitly reject the historical index-spacing artifact.
	if math.Abs(rate-(-7.0)) < 1.0 {
		t.Fatalf("rateKgPerWeek = %v matches the old -7 kg/week bug", rate)
	}
	if math.Abs(trend-74.0) > 0.01 {
		t.Fatalf("trendKg = %v, want 74 (fit evaluated at the newest weigh-in)", trend)
	}
}

// TestRollingWeightTrendLevelsMatchesHeadline guards the dashboard curve and
// the headline trend against drifting apart: a synthetic flat-then-dip-then-
// rise window must produce a nil burn-in prefix, a smooth curve that tracks the
// drift, and a final value numerically identical to ComputeWeightTrend.
func TestRollingWeightTrendLevelsMatchesHeadline(t *testing.T) {
	dates := windowDates(12)
	// Flat start, a water-weight dip, then a genuine rise.
	weights := []float64{82.0, 82.1, 82.0, 81.2, 82.0, 82.1, 82.3, 82.6, 82.8, 83.1, 83.3, 83.6}

	bios := make([]models.BiometricLog, len(dates))
	for i := range dates {
		bios[i] = models.BiometricLog{Date: dates[i], WeightKg: weights[i]}
	}

	levels := RollingWeightTrendLevels(weights, dates)
	if len(levels) != len(weights) {
		t.Fatalf("len(levels) = %d, want %d (same length as the raw series)", len(levels), len(weights))
	}

	// Burn-in: fewer than MinWeightTrendDays observations means no level.
	for i := 0; i < MinWeightTrendDays-1; i++ {
		if levels[i] != nil {
			t.Fatalf("levels[%d] = %v, want nil during burn-in", i, *levels[i])
		}
	}
	if levels[MinWeightTrendDays-1] == nil {
		t.Fatalf("levels[%d] is nil, want the first fitted level", MinWeightTrendDays-1)
	}

	// Smoothness: the fitted level must move gradually, unlike the raw dip.
	prev := *levels[MinWeightTrendDays-1]
	for i := MinWeightTrendDays; i < len(levels); i++ {
		if levels[i] == nil {
			t.Fatalf("levels[%d] is nil after burn-in", i)
		}
		if step := math.Abs(*levels[i] - prev); step > 0.6 {
			t.Fatalf("levels[%d] jumped %.2f kg from the previous fit", i, step)
		}
		prev = *levels[i]
	}
	// Drift tracking: the final fitted level sits above the flat/dip region.
	if last := *levels[len(levels)-1]; last < 82.5 {
		t.Fatalf("last level = %v, want the fit to track the rise above 82.5", last)
	}

	// Equality by construction: same series, same regression, same final level.
	trendKg, _, days := ComputeWeightTrend(bios, dates[0])
	if days != len(weights) {
		t.Fatalf("ComputeWeightTrend days = %d, want %d", days, len(weights))
	}
	last := levels[len(levels)-1]
	if last == nil {
		t.Fatal("last rolling level is nil, want a fitted value")
	}
	if math.Abs(*last-trendKg) > 1e-9 {
		t.Fatalf("last rolling level = %.12f, headline trendKg = %.12f; want identical by construction", *last, trendKg)
	}
}

// TestComputeObservedTDEEImplausibleFallsBackToStatic guards the <1000 kcal
// fallback that prevents nonsensical targets.
func TestComputeObservedTDEEImplausibleFallsBackToStatic(t *testing.T) {
	dates := windowDates(8)
	logs := nutritionSeries(dates, 500) // low intake
	bios := weightSeries(dates, 82.0, 0)
	profile := models.Profile{Sex: "male", HeightCm: 180, Age: 35, Activity: "moderately_active", ExerciseFreq: 3}

	res := ComputeObservedTDEE(logs, bios, profile, 0)

	if res.Method != "static_fallback" {
		t.Fatalf("Method = %q, want static_fallback", res.Method)
	}
	want := nutrition.ComputeStaticTDEE(profile, 82.0)
	if math.Abs(res.ObservedTDEE-want) > 1 {
		t.Fatalf("ObservedTDEE = %v, want static %v", res.ObservedTDEE, want)
	}
	if res.Warning == "" {
		t.Error("Warning is empty, want implausible-estimate notice")
	}
}
