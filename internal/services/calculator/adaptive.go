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

	// BodyEnergyDensityKcalPerKg is the energy content of one kilogram of body
	// mass. Energy balance treats changes in stored chemical energy and weight
	// change as proportional at roughly 7700 kcal/kg, the conventional value:
	//
	//   expenditure = energy intake - change in stored body energy
	//
	// This is the model MacroFactor describes: "changes in body mass reflect
	// changes in stored chemical energy ... given intake over time and the
	// caloric content of weight change (from the weight trend), we can estimate
	// daily energy expenditure with high accuracy."
	// Source: strongerbyscience.com/macrofactor-algorithms-philosophy/
	BodyEnergyDensityKcalPerKg = 7700.0

	// MinWeightReliableDays is the minimum number of distinct weight-bearing log
	// days required before the weight-energy term is trusted. Below this the
	// estimator falls back to intake alone and flags the result.
	MinWeightReliableDays = 7

	// MinWeightReliableSpanDays is the minimum calendar span (first to last
	// in-window weight) required in addition to the observation count. Seven
	// consecutive weigh-ins span only six days, so both gates must pass.
	MinWeightReliableSpanDays = 7.0

	// MinPlausibleObservedTDEE is the guard below which an energy-balance
	// estimate is treated as implausible and replaced by the static profile
	// estimate (mirrors the historical observed-TDEE < 1000 safety check).
	MinPlausibleObservedTDEE = 1000.0
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

// weightSeriesForWindow collects plausible weight observations inside the
// lookback window, oldest to newest, and expresses each observation as days
// since the first one. Real calendar offsets (rather than row indices) keep the
// slope correct when the user weighs in sparsely. Duplicate dates collapse to
// the latest value so "weight-bearing log days" counts distinct days.
func weightSeriesForWindow(biometrics []models.BiometricLog, since string) (weights, dayOffsets []float64, spanDays float64) {
	seen := make(map[string]int, len(biometrics))
	var dates []string

	for _, bio := range biometrics {
		if bio.Date < since {
			continue
		}
		if bio.WeightKg <= 20 || bio.WeightKg >= 300 {
			continue
		}
		if idx, ok := seen[bio.Date]; ok {
			weights[idx] = bio.WeightKg
			continue
		}
		seen[bio.Date] = len(weights)
		weights = append(weights, bio.WeightKg)
		dates = append(dates, bio.Date)
	}
	if len(weights) == 0 {
		return nil, nil, 0
	}

	dayOffsets = make([]float64, len(weights))
	base, baseErr := time.Parse(constants.DateFormat, dates[0])
	minOffset, maxOffset := math.MaxFloat64, -math.MaxFloat64
	for i, date := range dates {
		offset := float64(i) // fallback: index spacing if a date is malformed
		if baseErr == nil {
			if d, err := time.Parse(constants.DateFormat, date); err == nil {
				offset = d.Sub(base).Hours() / 24
			}
		}
		dayOffsets[i] = offset
		if offset < minOffset {
			minOffset = offset
		}
		if offset > maxOffset {
			maxOffset = offset
		}
	}
	if maxOffset > minOffset {
		spanDays = maxOffset - minOffset
	}
	return weights, dayOffsets, spanDays
}

// regressionSlopeKgPerDay fits weight against elapsed days and returns the
// slope. It is the same least-squares math as LinearRegression, parameterised
// by real day offsets so sparse weigh-ins are not understated.
func regressionSlopeKgPerDay(weights, dayOffsets []float64) float64 {
	n := float64(len(weights))
	if n < 2 {
		return 0
	}
	var sumX, sumY float64
	for i, w := range weights {
		sumX += dayOffsets[i]
		sumY += w
	}
	meanX := sumX / n
	meanY := sumY / n
	var num, den float64
	for i, w := range weights {
		x := dayOffsets[i]
		num += (x - meanX) * (w - meanY)
		den += (x - meanX) * (x - meanX)
	}
	if den == 0 {
		return 0
	}
	return num / den
}

// ComputeObservedTDEE implements the energy-balance estimator:
//
//	observed expenditure = intake estimate - body-energy change rate
//
// The body-energy rate is the weight-trend slope (kg/day) times 7700 kcal/kg.
// The weight term is only applied when the weight history is reliable (>= 7
// distinct weight-bearing log days spanning >= 7 calendar days); otherwise the
// result is intake alone and WeightReliable is false. This is the
// MacroFactor/SBS approach, not a raw intake average.
//
// bodyFatPct is optional (0 means unavailable) — used for Katch-McArdle
// estimated TDEE.
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
	startDate := today.AddDate(0, 0, -capDays).Format(constants.DateFormat)

	// Weight-energy term inputs. Reliability requires both a minimum number of
	// distinct weight days and a real calendar span: seven consecutive daily
	// weigh-ins only span six days, which is too short to distinguish water
	// shifts from a genuine trend.
	weights, dayOffsets, weightSpanDays := weightSeriesForWindow(biometrics, startDate)
	weightReliable := len(weights) >= MinWeightReliableDays && weightSpanDays >= MinWeightReliableSpanDays

	// Static profile estimate. Used for display, and as the safety fallback
	// when the observed energy balance is implausibly low.
	staticTDEE := DefaultFallbackTDEE
	latestWeight := 0.0
	if len(biometrics) > 0 {
		for i := len(biometrics) - 1; i >= 0; i-- {
			if biometrics[i].WeightKg > 0 {
				latestWeight = biometrics[i].WeightKg
				p := profile
				// Callers may pass body fat from the latest biometric entry;
				// treat it as the profile BF% when the profile has none.
				if bodyFatPct > 0 && p.BfPct <= 0 {
					p.BfPct = bodyFatPct
				}
				staticTDEE = nutrition.ComputeStaticTDEE(p, latestWeight)
				break
			}
		}
	}

	// collect calorie values oldest->newest
	var calories []float64
	for _, l := range logs {
		if l.Date >= startDate && l.Calories > 0 {
			calories = append(calories, l.Calories)
		}
	}
	days := len(calories)
	if days == 0 {
		return models.TDEEResult{
			EstimatedTDEE:       staticTDEE,
			ObservedTDEE:        DefaultFallbackTDEE,
			Confidence:          "low",
			DaysOfData:          0,
			LookbackDays:        capDays,
			Method:              "fallback",
			EmergencyAlert:      false,
			WeightReliable:      weightReliable,
			TargetUpdateBlocked: true,
		}
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

	var intakeEstimate float64
	method := ""
	confidence := "low"

	switch {
	case days < 4:
		// Tiny series: a mean is the only defensible level. A regression on
		// three points can imply absurd slopes.
		var sum float64
		for _, c := range calories {
			sum += c
		}
		intakeEstimate = sum / float64(days)
		method = "mean"
	case days < 7:
		// Short series: use the fitted LEVEL at the last observation. Do NOT
		// use LinearRegression's predicted value (evaluate at x = n), which is
		// a one-step forecast beyond the data and amplifies the last intake
		// spike. The historical bug: [3000,3200,3400,4600] forecast 4800
		// instead of the fitted level near the series.
		slope, intercept, _ := LinearRegression(calories)
		intakeEstimate = slope*float64(days-1) + intercept
		method = "linear_regression"
		confidence = "medium"
	case days < 30:
		rls := rlsFilter(calories, RLSForgettingFactor)
		if rls <= 0 {
			var s float64
			for _, c := range calories {
				s += c
			}
			rls = s / float64(len(calories))
		}
		intakeEstimate = rls
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
		intakeEstimate = ema
		method = "ema"
		confidence = "high"
	}

	// Energy-balance correction: a rising weight trend means stored energy is
	// increasing, so expenditure is below intake by that energy rate, and vice
	// versa. Without a reliable weight history the term is zero and the result
	// is flagged so the check-in handler can suppress target changes.
	observed := intakeEstimate
	warning := ""
	if weightReliable {
		slopeKgPerDay := regressionSlopeKgPerDay(weights, dayOffsets)
		bodyEnergyRateKcalPerDay := slopeKgPerDay * BodyEnergyDensityKcalPerKg
		observed = intakeEstimate - bodyEnergyRateKcalPerDay
	}

	// Implausibly low estimates indicate noisy/incomplete logs or a bad weight
	// trend. Fall back to the static profile estimate and flag it rather than
	// letting downstream damping chase a nonsensical number.
	if observed < MinPlausibleObservedTDEE {
		observed = staticTDEE
		method = "static_fallback"
		warning = "Observed energy expenditure was implausibly low; using the profile-based estimate instead."
	}

	observed = math.Round(observed)

	emergency := false

	return models.TDEEResult{
		EstimatedTDEE:       staticTDEE,
		ObservedTDEE:        observed,
		Confidence:          confidence,
		DaysOfData:          days,
		LookbackDays:        capDays,
		Method:              method,
		EmergencyAlert:      emergency,
		WeightReliable:      weightReliable,
		TargetUpdateBlocked: !weightReliable,
		Warning:             warning,
	}
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
