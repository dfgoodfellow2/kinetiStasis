package met

import (
	"math"
	"testing"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
)

// TestCalculateCaloriesBurnedDurationWeighted verifies the ACSM per-exercise
// sum for an uneven split:
//
//	20 min @ 8 MET + 80 min @ 3 MET, 80 kg
//	kcal = Σ duration_min × MET × 3.5 × kg / 200
//	     = 20 × 8 × 3.5 × 80 / 200  +  80 × 3 × 3.5 × 80 / 200
//	     = 224 + 336 = 560 kcal
//
// The old time-unweighted mean MET gave (8+3)/2 = 5.5 MET across all 100 min:
//
//	5.5 × 3.5 × 80 / 200 × 100 = 770 kcal
//
// so the mean overstated the burn by 210 kcal (~27%). The per-exercise
// durations are represented by each exercise's recorded set time (TUTSeconds).
func TestCalculateCaloriesBurnedDurationWeighted(t *testing.T) {
	exercises := []models.ExerciseEntry{
		{
			Name:     "sprints",
			METValue: 8,
			Sets:     []models.ExerciseSet{{TUTSeconds: 20 * 60}},
		},
		{
			Name:     "walk",
			METValue: 3,
			Sets:     []models.ExerciseSet{{TUTSeconds: 80 * 60}},
		},
	}

	got := CalculateCaloriesBurned(exercises, 80, 100)
	const want = 560.0
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("CalculateCaloriesBurned = %v, want %v (ACSM per-exercise sum)", got, want)
	}
	if math.Abs(got-770.0) < 1e-9 {
		t.Fatalf("CalculateCaloriesBurned = %v, still matches the mean-MET overestimate", got)
	}
}

// TestCalculateCaloriesBurnedEvenSplitFallback pins the fallback: when no
// exercise records set time, session time is split evenly across MET-bearing
// exercises, which is equivalent to the mean MET over the whole session.
func TestCalculateCaloriesBurnedEvenSplitFallback(t *testing.T) {
	exercises := []models.ExerciseEntry{
		{METValue: 8},
		{METValue: 3},
	}
	got := CalculateCaloriesBurned(exercises, 80, 100)
	// Mean MET 5.5 × 3.5 × 80 / 200 × 100 = 770 kcal.
	const want = 770.0
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("CalculateCaloriesBurned = %v, want %v (even split)", got, want)
	}
}

// TestCalculateCaloriesBurnedIgnoresMissingMET confirms exercises without a MET
// value do not dilute the weighted sum, and that a session with no MET at all
// keeps the historical 4-MET default.
func TestCalculateCaloriesBurnedIgnoresMissingMET(t *testing.T) {
	got := CalculateCaloriesBurned([]models.ExerciseEntry{
		{METValue: 0, Sets: []models.ExerciseSet{{TUTSeconds: 600}}},
		{METValue: 6, Sets: []models.ExerciseSet{{TUTSeconds: 600}}},
	}, 80, 10)
	// Only the 6-MET exercise contributes: 10 × 6 × 3.5 × 80 / 200 = 84.
	if math.Abs(got-84.0) > 1e-9 {
		t.Fatalf("CalculateCaloriesBurned = %v, want 84", got)
	}

	got = CalculateCaloriesBurned([]models.ExerciseEntry{{METValue: 0}}, 80, 60)
	// 4 MET × 3.5 × 80 / 200 × 60 = 336.
	if math.Abs(got-336.0) > 1e-9 {
		t.Fatalf("default MET calories = %v, want 336", got)
	}
}
