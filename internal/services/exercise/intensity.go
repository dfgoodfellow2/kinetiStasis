package exercise

import (
	"math"
)

// Estimate1RM estimates a one-rep max from weight and reps using
// Brzycki for <10 reps and Epley for >=10 reps. Special cases handled:
// - reps == 1 -> weight
// - reps <= 0 -> 0
func Estimate1RM(weight float64, reps int) float64 {
	if reps <= 0 || weight <= 0 {
		return 0
	}
	if reps == 1 {
		return weight
	}

	if reps < 10 {
		// Brzycki
		return weight * (36.0 / (37.0 - float64(reps)))
	}

	// Epley
	return weight * (1.0 + float64(reps)/30.0)
}

// CalculateExerciseIntensity computes the intensity for a single exercise
// considering multiple sources: 1RM formula, RPE, HR, and existing IntensityRelMax.
// Returns intensity (0-1) and source string.
func CalculateExerciseIntensity(
	weight float64,
	reps int,
	rpe float64, // per-exercise RPE (1-10)
	workoutRPE float64, // workout-level RPE (1-10)
	avgHR int,
	maxHR int,
	existingIntensity float64, // from AI parser or previous calculation
) (float64, string) {

	// Start with existing intensity if provided
	var iFormula float64
	if existingIntensity > 0 {
		// If parser provided explicit intensity, use it as base
		iFormula = existingIntensity
	} else if weight > 0 && reps > 0 {
		// Estimate 1RM and calculate intensity
		est1RM := Estimate1RM(weight, reps)
		if est1RM > 0 {
			iFormula = weight / est1RM
		}
	}

	var iRPE float64
	if rpe > 0 {
		iRPE = rpe / 10.0
	} else if workoutRPE > 0 {
		iRPE = workoutRPE / 10.0
	}

	var iHR float64
	if maxHR > 0 && avgHR > 0 {
		iHR = float64(avgHR) / float64(maxHR)
	}

	// If all measures are zero, return default
	if iFormula == 0 && iRPE == 0 && iHR == 0 {
		return 0, "default"
	}

	// Select max and source
	maxVal := iFormula
	source := "1rm"

	if iRPE > maxVal {
		maxVal = iRPE
		source = "rpe"
	}
	if iHR > maxVal {
		maxVal = iHR
		source = "hr"
	}

	// Cap at 1.0
	if maxVal > 1.0 {
		maxVal = 1.0
	}

	return maxVal, source
}

// CalculateDefaultIntensity returns the default intensity when none can be calculated
func CalculateDefaultIntensity() float64 {
	return 0.65
}

// CalculateRelativeIntensity is an updated wrapper that accepts the richer set of
// inputs (weight/reps, per-exercise RPE, workout-level RPE, HR, and existing
// intensity) and delegates to CalculateExerciseIntensity. It preserves the
// original return semantics: intensity (0-1) and a source string.
func CalculateRelativeIntensity(weight float64, reps int, rpe float64, workoutRPE float64, avgHR, maxHR int, existingIntensity float64) (float64, string) {
	return CalculateExerciseIntensity(weight, reps, rpe, workoutRPE, avgHR, maxHR, existingIntensity)
}

// CalculateIntensityScalar determines the intensity multiplier for NDS calculation.
// For duration-based exercises: if intensity > 0.7, square it; otherwise use linear.
// For strength exercises: always square the intensity.
func CalculateIntensityScalar(intensity float64, isDurationBased bool) float64 {
	// Ensure intensity bounded [0,1]
	if intensity < 0 {
		intensity = 0
	}
	if intensity > 1 {
		intensity = 1
	}

	if isDurationBased {
		if intensity > 0.7 {
			return intensity * intensity
		}
		return intensity
	}
	// strength: always squared
	return intensity * intensity
}

// EstimateIntensityFromReps estimates intensity based on reps performed
// using a simple linear falloff from 1RM (100% at 1 rep down to ~60% at 10 reps)
func EstimateIntensityFromReps(reps int) float64 {
	if reps <= 0 {
		return 0
	}
	if reps == 1 {
		return 1.0
	}
	if reps >= 10 {
		return 0.6
	}
	// Linear interpolation between 1 and 10 reps
	return 1.0 - (float64(reps-1) * 0.4 / 9.0)
}

// EstimateIntensityFromRPE estimates intensity from RPE (rate of perceived exertion)
// Formula: intensity = RPE / 10
func EstimateIntensityFromRPE(rpe float64) float64 {
	if rpe <= 0 {
		return 0
	}
	if rpe > 10 {
		rpe = 10
	}
	return rpe / 10.0
}

// TimeDecayFactor calculates the time decay for duration-based exercises
// Uses TUT^0.85 to model diminishing returns on neural drive over time
func TimeDecayFactor(tutSeconds float64) float64 {
	if tutSeconds <= 0 {
		return 0
	}
	return math.Pow(tutSeconds, 0.85)
}
