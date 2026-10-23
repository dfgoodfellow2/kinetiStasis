package calculator

import (
	"math"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/constants"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/services/nutrition"
)

const (
	TDEEEMAAlpha          = 0.10
	RLSForgettingFactor   = 0.98
	DefaultFallbackTDEE   = 2500.0
	MaxWeeklyCalorieSwing = 500.0

	// MinWeightTrendDays is the burn-in period before the dashboard reports a
	// weight trend. Below this, weightTrendKg stays 0 (insufficient data).
	MinWeightTrendDays = 7
)

// LinearRegression fits values to y = m*x + b using the 0-based index as x.
// It returns the slope, intercept and the value predicted at x = len(values).
// Shared by the adaptive TDEE tiering and the dashboard weight trend so both
// use the same regression implementation.
func LinearRegression(values []float64) (m, b, predicted float64) {
	n := float64(len(values))
	if n < 2 {
		if n == 1 {
			return 0, values[0], values[0]
		}
		return 0, 0, 0
	}
	var sumX, sumY float64
	for i, v := range values {
		sumX += float64(i)
		sumY += v
	}
	meanX := sumX / n
	meanY := sumY / n
	var num, den float64
	for i, v := range values {
		x := float64(i)
		num += (x - meanX) * (v - meanY)
		den += (x - meanX) * (x - meanX)
	}
	var mval float64
	if den != 0 {
		mval = num / den
	}
	b = meanY - mval*meanX
	predicted = mval*n + b
	return mval, b, predicted
}

// ComputeWeightTrend fits a least-squares line over the weight observations on
// or after since. trendKg is the fitted value at the newest observation and
// rateKgPerWeek is the slope expressed per week. days reports how many weight
// observations fed the fit, even during burn-in (when trendKg is 0).
//
// biometrics must be ordered oldest to newest, which is how the store returns
// them. Observations outside the plausible human range (20-300 kg) are skipped
// to match the dashboard's existing weight-trend filtering.
func ComputeWeightTrend(biometrics []models.BiometricLog, since string) (trendKg, rateKgPerWeek float64, days int) {
	weights := make([]float64, 0, len(biometrics))
	for _, bio := range biometrics {
		if bio.Date < since {
			continue
		}
		if bio.WeightKg <= 20 || bio.WeightKg >= 300 {
			continue
		}
		weights = append(weights, bio.WeightKg)
	}

	days = len(weights)
	if days < MinWeightTrendDays {
		return 0, 0, days
	}

	slope, intercept, _ := LinearRegression(weights)
	// Observations are 0-based, so the newest one is at x = days-1.
	// Evaluate the fit there; LinearRegression's third return value is an
	// x = n one-day-forward forecast and must not be used as the level.
	trendKg = slope*float64(days-1) + intercept
	// rateKgPerWeek assumes one weigh-in per day; sparse weighing understates it.
	return trendKg, slope * 7, days
}

// ComputeObservedTDEE implements tiered adaptive TDEE estimation.
// bodyFatPct is optional (0 means unavailable) — used for Katch-McArdle estimated TDEE.
func ComputeObservedTDEE(logs []models.NutritionLog, biometrics []models.BiometricLog, profile models.Profile, bodyFatPct float64) models.TDEEResult {
	// determine lookback days
	capDays := profile.TDEELookbackDays
	if capDays <= 0 {
		// profile.RunningKm used historically; map threshold to km (20 miles ≈ 32 km)
		if profile.RunningKm > 32 {
			capDays = 28
		} else {
			capDays = constants.DefaultTDEELookbackDays
		}
	}

	// cutoff
	today := time.Now()
	startDate := today.AddDate(0, 0, -capDays).Format("2006-01-02")

	// collect calorie values oldest->newest
	var calories []float64
	for _, l := range logs {
		if l.Date >= startDate && l.Calories > 0 {
			calories = append(calories, l.Calories)
		}
	}
	days := len(calories)
	if days == 0 {
		return models.TDEEResult{EstimatedTDEE: DefaultFallbackTDEE, ObservedTDEE: DefaultFallbackTDEE, Confidence: "low", DaysOfData: 0, LookbackDays: capDays, Method: "fallback", EmergencyAlert: false}
	}

	// helper functions
	rlsFilter := func(values []float64, lambda float64) float64 {
		if len(values) == 0 {
			return 0
		}
		x := values[0]
		gain := 1.0
		for i := 1; i < len(values); i++ {
			y := values[i]
			err := y - x
			x = x + gain*err
			gain = gain * lambda / (lambda + gain)
		}
		return x
	}

	computeEMAValue := func(values []float64, alpha float64) float64 {
		if len(values) == 0 {
			return 0
		}
		ema := values[0]
		for i := 1; i < len(values); i++ {
			ema = alpha*values[i] + (1-alpha)*ema
		}
		return ema
	}

	var observed float64
	method := ""
	confidence := "low"

	switch {
	case days < 7:
		_, _, pred := LinearRegression(calories)
		if pred <= 0 {
			pred = DefaultFallbackTDEE
		}
		observed = pred
		method = "linear_regression"
		if days >= 4 {
			confidence = "medium"
		}
	case days < 30:
		rls := rlsFilter(calories, RLSForgettingFactor)
		if rls <= 0 {
			var s float64
			for _, c := range calories {
				s += c
			}
			rls = s / float64(len(calories))
		}
		observed = math.Round(rls)
		method = "rls"
		if days >= 14 {
			confidence = "high"
		} else {
			confidence = "medium"
		}
	default:
		ema := computeEMAValue(calories, TDEEEMAAlpha)
		if ema <= 0 {
			ema = DefaultFallbackTDEE
		}
		observed = math.Round(ema)
		method = "ema"
		confidence = "high"
	}

	// Estimated TDEE from the latest biometric weight, if available.
	//
	// Delegate the static BMR -> base-TDEE computation to the nutrition
	// service so this path and the primary macro path share one implementation:
	// age derivation (birthdate -> stored age -> 30), the BF% gate for
	// Katch-McArdle, the NEAT/EAT factors, and the running-calorie add.
	// Only the observed-TDEE tiering above is unique to this function.
	est := DefaultFallbackTDEE
	if len(biometrics) > 0 {
		for i := len(biometrics) - 1; i >= 0; i-- {
			if biometrics[i].WeightKg > 0 {
				p := profile
				// Callers may pass body fat from the latest biometric entry;
				// treat it as the profile BF% when the profile has none.
				if bodyFatPct > 0 && p.BfPct <= 0 {
					p.BfPct = bodyFatPct
				}
				est = nutrition.ComputeStaticTDEE(p, biometrics[i].WeightKg)
				break
			}
		}
	}

	emergency := false

	return models.TDEEResult{EstimatedTDEE: est, ObservedTDEE: observed, Confidence: confidence, DaysOfData: days, LookbackDays: capDays, Method: method, EmergencyAlert: emergency}
}

// ComputeWeeklyAdjustment computes a damped daily calorie recommendation
// using estimated TDEE as baseline. Kept for alternate uses (dashboard etc.).
func ComputeWeeklyAdjustment(observed models.TDEEResult, profile models.Profile, exerciseCalories float64, eatBack bool) float64 {
	baseline := observed.EstimatedTDEE
	if baseline <= 0 {
		baseline = DefaultFallbackTDEE
	}

	dailyExercise := exerciseCalories

	var ideal float64
	if profile.Goal == "maintenance" {
		ideal = observed.ObservedTDEE + dailyExercise
	} else {
		if eatBack {
			ideal = observed.ObservedTDEE + dailyExercise
		} else {
			ideal = observed.ObservedTDEE - dailyExercise
		}
	}

	delta := ideal - baseline
	if delta > MaxWeeklyCalorieSwing {
		delta = MaxWeeklyCalorieSwing
	} else if delta < -MaxWeeklyCalorieSwing {
		delta = -MaxWeeklyCalorieSwing
	}
	final := baseline + delta
	if final < constants.MinCalorieFloor {
		final = constants.MinCalorieFloor
	}
	return final
}

// ComputeCheckinRecommendation computes a damped daily calorie recommendation
// for the weekly check-in. Uses current targets as the baseline and observed TDEE
// as the ideal, clamped to ±500 kcal swing.
// ComputeCheckinRecommendation computes a damped daily calorie recommendation
// for the weekly check-in. Uses current targets as the baseline and the
// goal-adjusted observed TDEE as the ideal, clamped to ±500 kcal swing.
func ComputeCheckinRecommendation(observed models.TDEEResult, profile models.Profile, currentCalories float64, exerciseCalories float64, eatBack bool) (recommendedCalories int, adjustment float64) {
	// Apply goal multiplier to observed TDEE so check-in converges toward
	// the correct cut/bulk/maintenance target, not raw maintenance.
	goalAdjusted := nutrition.ComputeTargetCalories(observed.ObservedTDEE, profile.Goal)

	var ideal float64
	if eatBack {
		ideal = goalAdjusted + exerciseCalories
	} else {
		ideal = goalAdjusted
	}

	delta := ideal - currentCalories
	if delta > MaxWeeklyCalorieSwing {
		delta = MaxWeeklyCalorieSwing
	} else if delta < -MaxWeeklyCalorieSwing {
		delta = -MaxWeeklyCalorieSwing
	}
	final := currentCalories + delta
	if final < constants.MinCalorieFloor {
		final = constants.MinCalorieFloor
	}
	return int(math.Round(final)), delta
}
