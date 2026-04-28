package nutrition

import (
	"math"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
)

func EstimateBMRMifflin(weightKg, heightCm float64, age int, isMale bool) float64 {
	bmr := 10*weightKg + 6.25*heightCm - 5*float64(age)
	if isMale {
		return bmr + 5
	}
	return bmr - 161
}

func AdjustBMRForSex(bmr float64, sex string) float64 {
	if sex == "female" {
		return bmr * 0.95
	}
	return bmr
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
	if p.BfPct > 0 {
		bmr := EstimateBMRKatchMcArdle(weightKg, p.BfPct)
		if bmr <= 0 {
			return EstimateBMRMifflin(weightKg, p.HeightCm, p.Age, isMale)
		}
		return bmr
	}
	return EstimateBMRMifflin(weightKg, p.HeightCm, p.Age, isMale)
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
	// weightKg is provided in kilograms
	inDeficit := p.Goal == "cut_10" || p.Goal == "cut_20" || p.Goal == "cut_30" || p.Goal == "cut_40"
	proteinG := CalcProtein(weightKg, p.IsLifter, inDeficit, p.BfPct, int(math.Round(p.RunningKm)), p.PrioritizeCarbs)
	proteinCals := proteinG * 4

	fatMinG := CalcFatMinimum(p.HeightCm)

	carbMinG := 0
	if p.RunningKm >= 24 {
		carbMinG = int(math.Round(weightKg * 5))
	}

	remainingCals := int(math.Round(targetCalories)) - proteinCals

	fatG := int(math.Round(targetCalories * 0.25 / 9))
	if fatG < fatMinG {
		fatG = fatMinG
	}
	fatCals := fatG * 9

	carbCals := remainingCals - fatCals
	carbG := int(math.Round(float64(carbCals) / 4))
	if carbG < 0 {
		carbG = 0
	}

	if carbG < carbMinG {
		deficitCals := (carbMinG - carbG) * 4
		newFatG := fatG - int(math.Round(float64(deficitCals)/9))
		if newFatG < fatMinG {
			fatG = fatMinG
		} else {
			fatG = newFatG
		}
		fatCals = fatG * 9
		carbCals = remainingCals - fatCals
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
		tdee = (static + observedTDEE) / 2
	} else {
		tdee = static
	}
	target := ComputeTargetCalories(tdee, p.Goal)
	return ComputeMacros(p, target, weightKg)
}
