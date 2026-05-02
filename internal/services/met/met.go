package met

import "github.com/dfgoodfellow2/diet-tracker/v2/internal/models"

// LookupMET returns a best-effort MET value for a given exercise name.
// This is a minimal implementation used by the workout YAML parser when
// no explicit `met` value is provided. It returns 0 for unknown names.
func LookupMET(name string) float64 {
	if name == "" {
		return 0
	}
	// Very small heuristic mapping for common activities.
	switch {
	case containsIgnoreCase(name, "run") || containsIgnoreCase(name, "jog"):
		return 9.8
	case containsIgnoreCase(name, "walk"):
		return 3.5
	case containsIgnoreCase(name, "bike") || containsIgnoreCase(name, "cycling"):
		return 7.5
	case containsIgnoreCase(name, "kb") || containsIgnoreCase(name, "kettlebell"):
		return 6.0
	case containsIgnoreCase(name, "deadlift") || containsIgnoreCase(name, "squat") || containsIgnoreCase(name, "press"):
		return 6.0
	case containsIgnoreCase(name, "row"):
		return 7.0
	default:
		return 0
	}
}

func containsIgnoreCase(s, sub string) bool {
	// simple case-insensitive contains
	if len(s) < len(sub) {
		return false
	}
	ss := []rune(s)
	subr := []rune(sub)
	ls := len(ss)
	lsub := len(subr)
	for i := 0; i <= ls-lsub; i++ {
		match := true
		for j := 0; j < lsub; j++ {
			a := ss[i+j]
			b := subr[j]
			if a >= 'A' && a <= 'Z' {
				a = a + ('a' - 'A')
			}
			if b >= 'A' && b <= 'Z' {
				b = b + ('a' - 'A')
			}
			if a != b {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// EstimateMETFromRPE estimates MET from RPE (0-10) when no explicit MET provided.
// Maps RPE to typical MET values for strength work:
// RPE 9-10: 8 METs (high intensity)
// RPE 7-8: 6 METs (moderate-high)
// RPE 5-6: 5 METs (moderate)
// RPE <5: 3-4 METs (light)
func EstimateMETFromRPE(rpe float64) float64 {
	switch {
	case rpe >= 9:
		return 8.0
	case rpe >= 7:
		return 6.0
	case rpe >= 5:
		return 5.0
	case rpe > 0:
		return 4.0
	default:
		return 4.0
	}
}

// CalculateCaloriesBurned computes calories burned from MET values.
// Formula: calories = MET × weight_kg × duration_hours
// If no MET provided, returns 0 so caller can use user-reported value.
func CalculateCaloriesBurned(exercises []models.ExerciseEntry, weightKg, durationMin float64) float64 {
	if len(exercises) == 0 || weightKg <= 0 || durationMin <= 0 {
		return 0
	}
	var totalMET float64
	var metCount int
	for _, ex := range exercises {
		if ex.METValue > 0 {
			totalMET += ex.METValue
			metCount++
		}
	}
	avgMET := 4.0
	if metCount > 0 {
		avgMET = totalMET / float64(metCount)
	}
	durationHours := durationMin / 60.0
	calories := avgMET * weightKg * durationHours
	return calories
}
