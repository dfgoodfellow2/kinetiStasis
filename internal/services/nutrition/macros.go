package nutrition

import (
	"log"
	"math"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/constants"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
)

func EstimateBMRMifflin(weightKg, heightCm float64, age int, isMale bool) float64 {
	bmr := 10*weightKg + 6.25*heightCm - 5*float64(age)
	if isMale {
		return bmr + 5
	}
	return bmr - 161
}

func EstimateBMRKatchMcArdle(weightKg, bfPct float64) float64 {
	if weightKg <= 0 || bfPct < 0 || bfPct >= 100 {
		return 0
	}
	lbm := weightKg * (1 - bfPct/100)
	return 370 + 21.6*lbm
}

// ComputeBMR implements the Mifflin-St Jeor with Katch-McArdle override when bf% > 0
func ComputeBMR(p models.Profile, weightKg float64) float64 {
	isMale := p.Sex == "male"
	age := models.AgeFromBirthdate(p.Birthdate)
	if age <= 0 {
		age = p.Age // fallback to stored age for legacy users
	}
	if age <= 0 {
		age = 30 // default if no age available at all
	}
	if p.BfPct > 0 {
		bmr := EstimateBMRKatchMcArdle(weightKg, p.BfPct)
		if bmr <= 0 {
			return EstimateBMRMifflin(weightKg, p.HeightCm, age, isMale)
		}
		return bmr
	}
	return EstimateBMRMifflin(weightKg, p.HeightCm, age, isMale)
}

func EstimateTDEEBase(bmr float64, activity string) float64 {
	neatFactors := map[string]float64{
		"sedentary":         1.0,
		"lightly_active":    1.1,
		"moderately_active": 1.2,
		"very_active":       1.3,
	}
	factor, ok := neatFactors[activity]
	if !ok {
		factor = 1.1
	}
	return bmr * factor
}

func EstimateEATFactor(exerciseFreq int) float64 {
	eatFactors := map[int]float64{
		0: 1.0, 1: 1.05, 2: 1.08, 3: 1.12, 4: 1.15, 5: 1.18, 6: 1.21, 7: 1.24,
	}
	if exerciseFreq > 7 {
		exerciseFreq = 7
	}
	if exerciseFreq < 0 {
		exerciseFreq = 0
	}
	return eatFactors[exerciseFreq]
}

func CalcRunningCalories(weightKg, kmPerWeek float64) int {
	if kmPerWeek <= 0 {
		return 0
	}
	weekly := kmPerWeek * 1.0 * weightKg
	return int(math.Round(weekly / 7))
}

func CalcProtein(weightKg float64, isLifter, inDeficit bool, bfPct float64, runningKm int, prioritizeCarbs bool) int {
	if prioritizeCarbs && runningKm >= 24 {
		base := 1.6
		if inDeficit {
			base = 1.8
		}
		return int(math.Round(weightKg * base))
	}
	if isLifter {
		base := 1.9
		if bfPct > 0 {
			if bfPct > 25 {
				base = 1.6
			} else if bfPct < 12 {
				base = 2.2
			}
		}
		if inDeficit {
			base += 0.3
		}
		return int(math.Round(weightKg * base))
	}
	return int(math.Round(weightKg * 1.5))
}

func CalcFatMinimum(heightCm float64) int {
	if heightCm < 150 {
		return 30
	}
	return int(math.Round(30 + (heightCm-150)*0.5))
}

func CalcFiberTarget(calories int, sex string) int {
	perCal := int(math.Round(float64(calories) / 1000 * 14))
	absMin := 36
	if sex == "female" {
		absMin = 28
	}
	if perCal > absMin {
		return perCal
	}
	return absMin
}

// ComputeStaticTDEE computes profile-based static TDEE: BMR × NEAT × EAT + running calories
func ComputeStaticTDEE(p models.Profile, weightKg float64) float64 {
	bmr := ComputeBMR(p, weightKg)
	baseNEAT := EstimateTDEEBase(bmr, p.Activity)
	eatMult := EstimateEATFactor(p.ExerciseFreq)
	tdeeBase := math.Round(baseNEAT * eatMult)
	runningCalsDay := CalcRunningCalories(weightKg, p.RunningKm)
	return tdeeBase + float64(runningCalsDay)
}

// ComputeTargetCalories applies the goal multiplier map to a tdee
func ComputeTargetCalories(tdee float64, goal string) float64 {
	goalMult := map[string]float64{
		"cut_10":      0.90,
		"cut_20":      0.80,
		"cut_30":      0.70,
		"cut_40":      0.60,
		"maintenance": 1.0,
		"bulk_10":     1.10,
		"bulk_20":     1.20,
	}
	mult, ok := goalMult[goal]
	if !ok {
		mult = 1.0
	}
	return math.Round(tdee * mult)
}

// ComputeMacros calculates protein/fat/carb/fiber/water with runner/lifter/BF% logic
func ComputeMacros(p models.Profile, targetCalories float64, weightKg float64) models.MacroResult {
	return DistributeMacros(p, targetCalories, weightKg)
}

// getFatPctForDistribution returns the fat percentage of remaining calories
// (after protein) for the percentage-based macro distributions. Low-carb
// distributions (keto, low_carb) do not use this — they cap carbohydrate at an
// absolute gram target instead (see getCarbTargetForDistribution).
func getFatPctForDistribution(distribution string) float64 {
	switch distribution {
	case "high_carb":
		return 25
	default: // "balanced"
		return 40
	}
}

// Absolute daily carbohydrate gram targets for the low-carbohydrate
// distributions. Capping carbs at a fixed gram target (and giving the rest of
// the post-protein calories to fat) is what makes keto/low_carb genuinely
// low-carb, instead of leaving a percentage of the remainder as carbohydrate.
const (
	ketoCarbTargetG    = 30  // keto: ~25-30g carbs from the post-protein remainder
	lowCarbCarbTargetG = 100 // low_carb: ~75-100g carbs from the post-protein remainder
)

// getCarbTargetForDistribution returns the absolute carb gram target for the
// capped (low-carb) distributions. The bool is false for percentage-based
// distributions (balanced, high_carb), which split the remainder by percentage.
func getCarbTargetForDistribution(distribution string) (int, bool) {
	switch distribution {
	case "keto":
		return ketoCarbTargetG, true
	case "low_carb":
		return lowCarbCarbTargetG, true
	default:
		return 0, false
	}
}

// DistributeMacros distributes macros given a target calorie count and macro distribution.
// Protein is calculated independently (based on lifter/BF%/deficit), then remaining
// calories are split between fat and carbs according to the distribution.
// This is used by the "Auto-Calculate" button in the UI, and also by full diet plan.
func DistributeMacros(p models.Profile, targetCalories float64, weightKg float64) models.MacroResult {
	inDeficit := p.Goal == "cut_10" || p.Goal == "cut_20" || p.Goal == "cut_30" || p.Goal == "cut_40"
	runningKm := int(math.Round(p.RunningKm))

	// Same protein logic as ComputeMacros
	proteinG := CalcProtein(weightKg, p.IsLifter, inDeficit, p.BfPct, runningKm, p.PrioritizeCarbs)
	proteinCals := proteinG * 4

	// Fat minimum
	fatMinG := CalcFatMinimum(p.HeightCm)

	// Carb minimum for runners
	carbMinG := 0
	if p.RunningKm >= 24 {
		carbMinG = int(math.Round(weightKg * 5))
	}

	remainingCals := int(math.Round(targetCalories)) - proteinCals
	if remainingCals < 0 {
		remainingCals = 0
	}

	var fatG, carbG int
	if carbTargetG, capped := getCarbTargetForDistribution(p.MacroDistribution); capped {
		// Low-carb distributions (keto, low_carb): cap carbohydrate at an
		// absolute gram target and let fat take the rest of the post-protein
		// calories. Never let the cap exceed what the remaining calories can
		// supply.
		carbG = carbTargetG
		if maxCarbG := remainingCals / 4; carbG > maxCarbG {
			carbG = maxCarbG
		}
		fatG = int(math.Round(float64(remainingCals-carbG*4) / 9.0))
		if fatG < fatMinG {
			// Fat floor wins; trim carbs so total energy still matches.
			fatG = fatMinG
			carbG = int(math.Round(float64(remainingCals-fatG*9) / 4.0))
			if carbG < 0 {
				carbG = 0
			}
		}
	} else {
		// Percentage-based distributions (balanced, high_carb): fat takes a
		// share of the remaining calories and carbs get whatever is left.
		fatPct := getFatPctForDistribution(p.MacroDistribution)
		fatG = int(math.Round(float64(remainingCals) * fatPct / 100.0 / 9.0))
		if fatG < fatMinG {
			fatG = fatMinG
		}
		fatCals := fatG * 9
		carbCals := remainingCals - fatCals
		carbG = int(math.Round(float64(carbCals) / 4))
		if carbG < 0 {
			carbG = 0
		}
	}

	// Enforce carb minimum for runners by stealing from fat
	if carbG < carbMinG {
		neededCals := (carbMinG - carbG) * 4
		newFatG := fatG - int(math.Round(float64(neededCals)/9))
		if newFatG < fatMinG {
			fatG = fatMinG
		} else {
			fatG = newFatG
		}
		fatCals := fatG * 9
		carbCals := remainingCals - fatCals
		carbG = int(math.Round(float64(carbCals) / 4))
		if carbG < 0 {
			carbG = 0
		}
	}

	fiberG := CalcFiberTarget(int(math.Round(targetCalories)), p.Sex)
	waterMl := 35 * weightKg

	return models.MacroResult{
		Calories:  targetCalories,
		ProteinG:  float64(proteinG),
		CarbsG:    float64(carbG),
		FatG:      float64(fatG),
		FiberG:    float64(fiberG),
		WaterMl:   waterMl,
		GoalLabel: p.Goal,
	}
}

// FullDietPlan combines static+observed TDEE and returns macro plan
func FullDietPlan(p models.Profile, weightKg float64, observedTDEE float64) models.MacroResult {
	static := ComputeStaticTDEE(p, weightKg)
	// Combine static and observed: if observed provided (>0) average them
	var tdee float64
	if observedTDEE > 0 {
		// Safety: if observed TDEE is implausibly low, prefer static estimate and log
		if observedTDEE < 1000 {
			log.Printf("observedTDEE is very low (%v) - using static TDEE (%v) instead", observedTDEE, static)
			tdee = static
		} else {
			tdee = (static + observedTDEE) / 2
		}
	} else {
		tdee = static
	}
	target := ComputeTargetCalories(tdee, p.Goal)
	// Enforce absolute minimum calorie floor
	if target < constants.MinCalorieFloor {
		log.Printf("Computed calories %v below MinCalorieFloor %v - enforcing floor", target, constants.MinCalorieFloor)
		target = constants.MinCalorieFloor
	}
	return ComputeMacros(p, target, weightKg)
}
