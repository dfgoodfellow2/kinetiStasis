package nutrition

import (
	"math"
	"testing"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
)

// baseProfile returns a non-runner, non-lifter maintenance profile that makes
// the distribution math easy to reason about (protein = 1.5 g/kg).
func baseProfile() models.Profile {
	return models.Profile{
		Sex:               "male",
		HeightCm:          180,
		Activity:          "moderately_active",
		Goal:              "maintenance",
		MacroDistribution: "balanced",
	}
}

func TestDistributeMacros(t *testing.T) {
	const (
		calories = 2000.0
		weightKg = 80.0
	)

	cases := []struct {
		name     string
		calories float64
		weight   float64
		mutate   func(p *models.Profile)
		validate func(t *testing.T, out models.MacroResult)
	}{
		{
			name:     "balanced",
			calories: calories,
			weight:   weightKg,
			mutate:   func(p *models.Profile) { p.MacroDistribution = "balanced" },
			validate: func(t *testing.T, out models.MacroResult) {
				if out.ProteinG != 120 {
					t.Errorf("protein: got %v want 120", out.ProteinG)
				}
				if out.CarbsG <= 0 || out.FatG <= 0 {
					t.Errorf("balanced macros must be positive: carbs=%v fat=%v", out.CarbsG, out.FatG)
				}
			},
		},
		{
			name:     "keto yields genuinely low carbs",
			calories: calories,
			weight:   weightKg,
			mutate:   func(p *models.Profile) { p.MacroDistribution = "keto" },
			validate: func(t *testing.T, out models.MacroResult) {
				if out.CarbsG >= 50 {
					t.Errorf("keto carbs: got %v want < 50", out.CarbsG)
				}
				if out.FatG <= 0 {
					t.Errorf("keto fat must be positive: got %v", out.FatG)
				}
			},
		},
		{
			name:     "low_carb caps carbs",
			calories: calories,
			weight:   weightKg,
			mutate:   func(p *models.Profile) { p.MacroDistribution = "low_carb" },
			validate: func(t *testing.T, out models.MacroResult) {
				if out.CarbsG >= 125 {
					t.Errorf("low_carb carbs: got %v want < 125", out.CarbsG)
				}
				if out.FatG <= 0 {
					t.Errorf("low_carb fat must be positive: got %v", out.FatG)
				}
			},
		},
		{
			name:     "high_carb",
			calories: calories,
			weight:   weightKg,
			mutate:   func(p *models.Profile) { p.MacroDistribution = "high_carb" },
			validate: func(t *testing.T, out models.MacroResult) {
				if out.CarbsG <= 0 || out.FatG <= 0 {
					t.Errorf("high_carb macros must be positive: carbs=%v fat=%v", out.CarbsG, out.FatG)
				}
			},
		},
		{
			name:     "fat ordering keto > low_carb > balanced > high_carb",
			calories: calories,
			weight:   weightKg,
			validate: func(t *testing.T, _ models.MacroResult) {
				keto := DistributeMacros(withDist(baseProfile(), "keto"), calories, weightKg)
				low := DistributeMacros(withDist(baseProfile(), "low_carb"), calories, weightKg)
				bal := DistributeMacros(withDist(baseProfile(), "balanced"), calories, weightKg)
				high := DistributeMacros(withDist(baseProfile(), "high_carb"), calories, weightKg)
				if !(keto.FatG > low.FatG && low.FatG > bal.FatG && bal.FatG > high.FatG) {
					t.Errorf("fat ordering violated: keto=%v low_carb=%v balanced=%v high_carb=%v",
						keto.FatG, low.FatG, bal.FatG, high.FatG)
				}
			},
		},
		{
			name:     "empty distribution defaults to balanced",
			calories: calories,
			weight:   weightKg,
			mutate:   func(p *models.Profile) { p.MacroDistribution = "" },
			validate: func(t *testing.T, out models.MacroResult) {
				want := DistributeMacros(withDist(baseProfile(), "balanced"), calories, weightKg)
				if out.ProteinG != want.ProteinG || out.CarbsG != want.CarbsG || out.FatG != want.FatG {
					t.Errorf("empty distribution != balanced: got %+v want %+v", out, want)
				}
			},
		},
		{
			name:     "unknown distribution defaults to balanced",
			calories: calories,
			weight:   weightKg,
			mutate:   func(p *models.Profile) { p.MacroDistribution = "paleo" },
			validate: func(t *testing.T, out models.MacroResult) {
				want := DistributeMacros(withDist(baseProfile(), "balanced"), calories, weightKg)
				if out.ProteinG != want.ProteinG || out.CarbsG != want.CarbsG || out.FatG != want.FatG {
					t.Errorf("unknown distribution != balanced: got %+v want %+v", out, want)
				}
			},
		},
		{
			name:     "protein exceeds calories clamps macros non-negative",
			calories: 100,
			weight:   weightKg,
			mutate:   func(p *models.Profile) { p.MacroDistribution = "keto" },
			validate: func(t *testing.T, out models.MacroResult) {
				if out.ProteinG < 0 || out.CarbsG < 0 || out.FatG < 0 {
					t.Errorf("macros must be non-negative: %+v", out)
				}
			},
		},
		{
			name:     "Katch-McArdle branch (BfPct > 0)",
			calories: calories,
			weight:   weightKg,
			mutate: func(p *models.Profile) {
				p.IsLifter = true
				p.BfPct = 30 // > 25 → protein factor drops to 1.6 g/kg
				p.MacroDistribution = "balanced"
			},
			validate: func(t *testing.T, out models.MacroResult) {
				// CalcProtein: lifter, bf% > 25 → base 1.6 → 80 * 1.6 = 128g
				if out.ProteinG != 128 {
					t.Errorf("protein: got %v want 128", out.ProteinG)
				}
				// Katch-McArdle BMR branch (BfPct > 0) must produce a positive BMR.
				katch := baseProfile()
				katch.BfPct = 30
				if bmr := ComputeBMR(katch, weightKg); bmr <= 0 {
					t.Errorf("ComputeBMR with BfPct > 0: got %v want > 0", bmr)
				}
			},
		},
		{
			name:     "zero calories yields non-negative macros",
			calories: 0,
			weight:   weightKg,
			mutate:   func(p *models.Profile) { p.MacroDistribution = "keto" },
			validate: func(t *testing.T, out models.MacroResult) {
				if out.ProteinG < 0 || out.CarbsG < 0 || out.FatG < 0 {
					t.Errorf("macros must be non-negative: %+v", out)
				}
			},
		},
		{
			name:     "negative calories yields non-negative macros",
			calories: -500,
			weight:   weightKg,
			mutate:   func(p *models.Profile) { p.MacroDistribution = "balanced" },
			validate: func(t *testing.T, out models.MacroResult) {
				if out.ProteinG < 0 || out.CarbsG < 0 || out.FatG < 0 {
					t.Errorf("macros must be non-negative: %+v", out)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := baseProfile()
			if tc.mutate != nil {
				tc.mutate(&p)
			}
			out := DistributeMacros(p, tc.calories, tc.weight)
			tc.validate(t, out)
		})
	}
}

func withDist(p models.Profile, dist string) models.Profile {
	p.MacroDistribution = dist
	return p
}

// TestComputeStaticTDEESedentaryTotalRatio pins the new total-TDEE index
// convention: an 80 kg / 180 cm / 38-year-old sedentary male has a Mifflin BMR
// of 1740 kcal, and the sedentary factor is a TOTAL TDEE/BMR ratio of 1.25, so
// the static estimate is 2175 kcal — not the old 1740 "BMR-as-TDEE" value.
func TestComputeStaticTDEESedentaryTotalRatio(t *testing.T) {
	const (
		weightKg = 80.0
		wantBMR  = 1740.0
		wantTDEE = 2175.0
	)
	p := models.Profile{
		Sex:          "male",
		HeightCm:     180,
		Age:          38,
		Activity:     "sedentary",
		ExerciseFreq: 0,
		Goal:         "maintenance",
	}

	if bmr := ComputeBMR(p, weightKg); math.Abs(bmr-wantBMR) > 0.5 {
		t.Fatalf("ComputeBMR = %v, want %v", bmr, wantBMR)
	}

	got := ComputeStaticTDEE(p, weightKg)
	if math.Abs(got-wantTDEE) > 25 {
		t.Fatalf("ComputeStaticTDEE = %v, want %v ±25 (sedentary TOTAL ratio 1.25)", got, wantTDEE)
	}
	if want := math.Round(wantBMR * NEATFactorSedentary); got != want {
		t.Fatalf("ComputeStaticTDEE = %v, want %v", got, want)
	}
}

// TestStaticTDEEActivityLadderUsesTotalRatios checks each rung of the activity
// ladder at ExerciseFreq 0 (no running, no EAT adjustment), where the estimate
// must equal BMR x the new total-activity multiplier exactly.
func TestStaticTDEEActivityLadderUsesTotalRatios(t *testing.T) {
	const weightKg = 80.0
	base := models.Profile{
		Sex:          "male",
		HeightCm:     180,
		Age:          38,
		ExerciseFreq: 0,
		Goal:         "maintenance",
	}
	bmr := ComputeBMR(base, weightKg)

	ladder := []struct {
		activity string
		factor   float64
	}{
		{"lightly_active", NEATFactorLightlyActive},
		{"moderately_active", NEATFactorModeratelyActive},
		{"very_active", NEATFactorVeryActive},
	}

	for _, rung := range ladder {
		t.Run(rung.activity, func(t *testing.T) {
			p := base
			p.Activity = rung.activity

			got := ComputeStaticTDEE(p, weightKg)
			want := math.Round(bmr * rung.factor)
			if got != want {
				t.Fatalf("ComputeStaticTDEE = %v, want %v (BMR %v × %v)", got, want, bmr, rung.factor)
			}
			if want := math.Round(bmr * 1.0); got <= want {
				t.Fatalf("ComputeStaticTDEE = %v, want above the old BMR-as-TDEE value %v", got, want)
			}
		})
	}
}

// TestRunnerTDEENoDoubleCount proves that a runner's dedicated running days are
// netted out of the reported exercise frequency before the EAT factor is
// applied: the same 40 km/week is added exactly once, via running calories.
func TestRunnerTDEENoDoubleCount(t *testing.T) {
	const (
		weightKg  = 80.0
		runningKm = 40.0
	)
	p := models.Profile{
		Sex:          "male",
		HeightCm:     180,
		Age:          38,
		Activity:     "moderately_active",
		ExerciseFreq: 5,
		RunningKm:    runningKm,
		Goal:         "maintenance",
	}

	bmr := ComputeBMR(p, weightKg)
	runningAdd := float64(CalcRunningCalories(weightKg, runningKm))

	effectiveFreq := p.ExerciseFreq - int(math.Round(runningKm/RunningKmPerSession))
	if effectiveFreq != 0 {
		t.Fatalf("effectiveExerciseFreq = %d, want 0 after netting five 8 km sessions", effectiveFreq)
	}

	got := ComputeStaticTDEE(p, weightKg)
	want := math.Round(bmr*NEATFactorModeratelyActive*EstimateEATFactor(effectiveFreq)) + runningAdd
	if got != want {
		t.Fatalf("ComputeStaticTDEE = %v, want %v (base × EAT(0) + running once)", got, want)
	}

	// The hard ceiling: total activity can never exceed 1.6× BMR plus the
	// running add. Anything above that means running/EAT was counted twice.
	ceiling := bmr*1.6 + runningAdd
	if got > ceiling {
		t.Fatalf("ComputeStaticTDEE = %v, want <= %v (BMR×1.6 + running add)", got, ceiling)
	}

	// The old double-count applied EAT(5) to the full frequency AND added the
	// running calories. Assert we are materially below that inflated value.
	doubleCounted := math.Round(bmr*NEATFactorModeratelyActive*EstimateEATFactor(p.ExerciseFreq)) + runningAdd
	if got >= doubleCounted-200 {
		t.Fatalf("ComputeStaticTDEE = %v, want well below the double-counted %v", got, doubleCounted)
	}
}
