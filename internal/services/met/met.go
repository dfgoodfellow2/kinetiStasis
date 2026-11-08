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

// CalculateCaloriesBurned computes calories burned by summing the ACSM
// per-exercise energy expenditure:
//
//	kcal = Σ_i duration_i(min) × MET_i × 3.5 × weightKg / 200
//
// A time-unweighted mean MET (the old behaviour) only agrees with this when
// every exercise occupies an equal share of the session; whenever high-MET and
// low-MET work are split unevenly it overstates burn.
//
// Each exercise's share of the session clock is derived from its recorded set
// time (the sum of TUTSeconds, which doubles as total duration for timed sets).
// When any MET-bearing exercise has no recorded set time, weighting by a mix of
// recorded and missing durations would skew the result, so the session duration
// is split evenly across MET-bearing exercises instead — the same implicit split
// the previous mean-MET implementation used. If no exercise declares a MET, the
// historical 4-MET default is applied to the whole session.
func CalculateCaloriesBurned(exercises []models.ExerciseEntry, weightKg, durationMin float64) float64 {
	if len(exercises) == 0 || weightKg <= 0 || durationMin <= 0 {
		return 0
	}

	durations := make([]float64, len(exercises)) // minutes, per exercise
	totalRecorded := 0.0
	metCount := 0
	for i, ex := range exercises {
		if ex.METValue <= 0 {
			continue
		}
		metCount++
		for _, s := range ex.Sets {
			if s.TUTSeconds > 0 {
				durations[i] += s.TUTSeconds / 60.0
			}
		}
		totalRecorded += durations[i]
	}

	if metCount == 0 {
		// No MET declared: preserve the historical 4-MET default so callers that
		// rely on auto-calculation still get a plausible estimate.
		return 4.0 * 3.5 * weightKg / 200.0 * durationMin
	}

	// Use recorded durations only when every MET-bearing exercise has one.
	useRecorded := totalRecorded > 0
	if useRecorded {
		for i, ex := range exercises {
			if ex.METValue > 0 && durations[i] <= 0 {
				useRecorded = false
				break
			}
		}
	}

	var calories float64
	for i, ex := range exercises {
		if ex.METValue <= 0 {
			continue
		}
		minutes := durationMin / float64(metCount)
		if useRecorded {
			minutes = durationMin * durations[i] / totalRecorded
		}
		// ACSM: kcal/min = MET × 3.5 × kg / 200
		calories += minutes * ex.METValue * 3.5 * weightKg / 200.0
	}
	return calories
}
