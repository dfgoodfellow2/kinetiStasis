package exercise

import (
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

// NeuralDemandScore computes weighted TUT×load for hinge/squat patterns across entries.
func NeuralDemandScore(entries []models.ExerciseEntry) float64 {
	total := 0.0
	for _, e := range entries {
		// sum per-entry effective volume
		ev := 0.0
		for _, s := range e.Sets {
			ev += s.TUTSeconds * s.LoadLbs
		}
		factor := 1.0
		switch stringsToLower(e.Category) {
		case "hinge", "squat":
			factor = 2.0
		case "push", "pull":
			factor = 1.5
		}
		total += ev * factor
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

// SummarizeWorkout fills MWV, NDS, SessionDensity, CaloriesBurned
func SummarizeWorkout(entry models.WorkoutEntry, weightLbs float64, profile models.Profile) models.WorkoutEntry {
	totalMWV := 0.0
	for _, ex := range entry.Exercises {
		// MWV: sum load*reps*sets
		for _, s := range ex.Sets {
			if s.LoadLbs > 0 && s.Reps > 0 {
				totalMWV += s.LoadLbs * float64(s.Reps)
			}
		}
	}
	nds := NeuralDemandScore(entry.Exercises)
	sd := SessionDensity(entry.Exercises, entry.DurationMin, weightLbs, profile)
	// Estimate calories burned by summing per-exercise MET×time share
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
