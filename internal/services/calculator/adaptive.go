package calculator

import (
	"math"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/constants"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
)

const (
	TDEEEMAAlpha          = 0.10
	RLSForgettingFactor   = 0.98
	DefaultFallbackTDEE   = 2500.0
	MaxWeeklyCalorieSwing = 500.0
)

// ComputeObservedTDEE implements tiered adaptive TDEE estimation.
func ComputeObservedTDEE(logs []models.NutritionLog, biometrics []models.BiometricLog, profile models.Profile) models.TDEEResult {
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
	linearRegression := func(values []float64) (m, b, predicted float64) {
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
		_, _, pred := linearRegression(calories)
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

	// estimated TDEE from latest biometric weight if available
	est := DefaultFallbackTDEE
	if len(biometrics) > 0 {
		// find last with weight
		for i := len(biometrics) - 1; i >= 0; i-- {
			if biometrics[i].WeightKg > 0 {
				// approximate BMR via Mifflin using profile height/age
				weightKg := biometrics[i].WeightKg
				bmr := 10*weightKg + 6.25*profile.HeightCm - 5*float64(profile.Age)
				if profile.Sex == "male" {
					bmr += 5
				} else {
					bmr -= 161
				}
				// NEAT multiplier
				neat := 1.1
				switch profile.Activity {
				case "sedentary":
					neat = 1.0
				case "lightly_active":
					neat = 1.1
				case "moderately_active":
					neat = 1.2
				case "very_active":
					neat = 1.3
				}
				est = math.Round(bmr * neat)
				break
			}
		}
	}

	emergency := false
	if confidence != "low" && est > 0 {
		if math.Abs(observed-est)/est > 0.15 {
			emergency = true
		}
	}

	return models.TDEEResult{EstimatedTDEE: est, ObservedTDEE: observed, Confidence: confidence, DaysOfData: days, LookbackDays: capDays, Method: method, EmergencyAlert: emergency}
}

// ComputeWeeklyAdjustment computes a damped daily calorie recommendation
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
