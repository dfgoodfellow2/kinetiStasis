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

			// The statistical tiering must remain untouched by the delegation.
			if got.Method != "linear_regression" {
				t.Errorf("Method = %q, want %q", got.Method, "linear_regression")
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
