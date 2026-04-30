package exercise

import (
	"math"
	"strings"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
)

// MechanicalWorkVolume sums load_lbs × reps × sets across all sets in an entry.
func MechanicalWorkVolume(e models.ExerciseEntry) float64 {
	total := 0.0
	for _, s := range e.Sets {
		if s.LoadLbs > 0 && s.Reps > 0 && s.Reps <= 10000 && s.RestSeconds >= 0 {
			total += s.LoadLbs * float64(s.Reps)
		}
	}
	return total
}

// EffectiveVolume sums TUT_seconds × load_lbs × surface_multiplier
func EffectiveVolume(e models.ExerciseEntry) float64 {
	surfaceMul := 1.0
	switch stringsToLower(e.Surface) {
	case "pavement":
		surfaceMul = 1.0
	case "wet_grass":
		surfaceMul = 0.9
	case "standard_grass":
		surfaceMul = 1.0
	case "sticky_grass":
		surfaceMul = 1.2
	}
	total := 0.0
	for _, s := range e.Sets {
		if s.LoadLbs > 0 && s.TUTSeconds > 0 {
			total += s.TUTSeconds * s.LoadLbs * surfaceMul
		}
	}
	return total
}

// NeuralDemandScore computes NDS = (Volume × MultiJointFactor × UnilateralFactor) × IntensityScalar
// For strength: Volume = Load × Reps × Sets, Intensity always squared
// For time-based: Volume = (Load × SurfaceMultiplier × TUT^0.85) / 50, Intensity squared if > 0.7
func NeuralDemandScore(entries []models.ExerciseEntry) float64 {
	total := 0.0
	for _, e := range entries {
		// Get surface multiplier
		surfaceMul := 1.0
		switch stringsToLower(e.Surface) {
		case "pavement":
			surfaceMul = 1.0
		case "wet_grass":
			surfaceMul = 0.9
		case "standard_grass":
			surfaceMul = 1.0
		case "sticky_grass":
			surfaceMul = 1.2
		}

		// Get unilateral factor (1.2 for unilateral, 1.0 for bilateral)
		unilateralFactor := 1.0
		if stringsToLower(e.Bias) == "unilateral" {
			unilateralFactor = 1.2
		}

		// Get multi-joint factor
		multiJointFactor := 1.0
		switch stringsToLower(e.Category) {
		case "hinge", "squat":
			multiJointFactor = 2.0
		case "push", "pull":
			multiJointFactor = 1.5
		}

		// Determine if duration-based (time-based) or strength
		// Duration-based: any set has TUT > 0 with reps == 1, or category is conditioning
		isDurationBased := false
		for _, s := range e.Sets {
			if s.TUTSeconds > 0 && s.Reps == 1 {
				isDurationBased = true
				break
			}
		}
		if stringsToLower(e.Category) == "conditioning" {
			isDurationBased = true
		}

		// Calculate volume based on type
		var volume float64
		if isDurationBased {
			// Time-based: use time decay factor TUT^0.85
			totalTUT := 0.0
			for _, s := range e.Sets {
				totalTUT += s.TUTSeconds
			}
			effectiveLoad := 0.0
			for _, s := range e.Sets {
				effectiveLoad += s.LoadLbs
			}
			timeFactor := math.Pow(totalTUT, 0.85)
			volume = (effectiveLoad * surfaceMul * timeFactor) / 50.0
		} else {
			// Strength-based: load * reps * sets
			for _, s := range e.Sets {
				if s.LoadLbs > 0 && s.Reps > 0 {
					volume += s.LoadLbs * float64(s.Reps)
				}
			}
		}

		// Get intensity
		intensity := e.IntensityRelMax
		if intensity <= 0 && e.RPE > 0 {
			// Fall back to RPE if no explicit intensity
			intensity = e.RPE / 10.0
		}

		// Calculate intensity scalar (squared for strength, conditional for duration-based)
		intensityScalar := CalculateIntensityScalar(intensity, isDurationBased)

		exNDS := (volume * multiJointFactor * unilateralFactor) * intensityScalar
		total += exNDS
	}
	return total
}

// SessionDensity returns MET-minutes / total clock time (minutes)
func SessionDensity(entries []models.ExerciseEntry, totalMinutes float64, weightLbs float64, profile models.Profile) float64 {
	if totalMinutes <= 0 {
		return 0
	}
	// approximate MET-minutes by summing per-entry METValue × duration fraction
	metMin := 0.0
	for _, e := range entries {
		// assume uniform split of session time across exercises
		if e.METValue > 0 {
			metMin += e.METValue * totalMinutes / float64(len(entries))
		}
	}
	return metMin / totalMinutes
}

// SummarizeWorkout fills MWV, NDS, SessionDensity, CaloriesBurned and calculates intensity for each exercise
func SummarizeWorkout(entry models.WorkoutEntry, weightLbs float64, profile models.Profile) models.WorkoutEntry {
	totalMWV := 0.0

	// Get workout-level intensity data from metadata
	workoutRPE := entry.Metadata.RPE
	avgHR := entry.Metadata.AvgHR
	maxHR := entry.Metadata.MaxHR

	// Calculate intensity for each exercise first
	for i := range entry.Exercises {
		ex := &entry.Exercises[i]

		// Use first set's load and reps for 1RM estimation
		var weight float64
		var reps int
		if len(ex.Sets) > 0 {
			weight = ex.Sets[0].LoadLbs
			reps = ex.Sets[0].Reps
		}

		intensity, source := CalculateRelativeIntensity(
			weight,
			reps,
			ex.RPE,     // per-exercise RPE
			workoutRPE, // workout-level RPE
			avgHR,
			maxHR,
			ex.IntensityRelMax, // existing intensity from AI parser
		)

		// If still no intensity, use default
		if intensity <= 0 {
			intensity = CalculateDefaultIntensity()
			source = "default"
		}

		ex.IntensityRelMax = intensity
		ex.IntensitySource = source
	}

	// Calculate MWV
	for _, ex := range entry.Exercises {
		for _, s := range ex.Sets {
			if s.LoadLbs > 0 && s.Reps > 0 {
				totalMWV += s.LoadLbs * float64(s.Reps)
			}
		}
	}

	nds := NeuralDemandScore(entry.Exercises)
	sd := SessionDensity(entry.Exercises, entry.DurationMin, weightLbs, profile)

	// Estimate calories burned
	calories := 0.0
	if entry.DurationMin > 0 && len(entry.Exercises) > 0 {
		for _, ex := range entry.Exercises {
			if ex.METValue <= 0 {
				continue
			}
			minutes := entry.DurationMin / float64(len(entry.Exercises))
			calories += METToCalories(ex.METValue, weightLbs, minutes)
		}
	}

	entry.MWV = totalMWV
	entry.NDS = nds
	entry.SessionDensity = sd
	entry.CaloriesBurned = calories
	return entry
}

// helper: MET -> calories for lbs
func METToCalories(met, weightLbs, minutes float64) float64 {
	if met <= 0 || weightLbs <= 0 || minutes <= 0 {
		return 0
	}
	kg := weightLbs * 0.45359237
	kcalPerMin := (met * 3.5 * kg) / 200.0
	return kcalPerMin * minutes
}

// small helper to avoid importing strings widely
func stringsToLower(s string) string {
	return strings.ToLower(s)
}
