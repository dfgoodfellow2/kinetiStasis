package bodyfat

import (
	"math"
	"strings"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
)

// NavyMethod computes bodyfat using navy circumference method and returns BodyFatResult
// weightKg should be provided in kilograms and returned lean/fat masses are in kg.
func NavyMethod(profile models.Profile, weightKg float64, neckCm float64, waistCm float64, hipsCm float64) models.BodyFatResult {
	isMale := stringsToLower(profile.Sex) == "male"
	bf := 0.0
	if isMale {
		diff := waistCm - neckCm
		if diff > 0 && waistCm > 0 && neckCm > 0 && profile.HeightCm > 0 {
			bf = 495/(1.0324-0.19077*math.Log10(diff)+0.15456*math.Log10(profile.HeightCm)) - 450
		}
	} else {
		sum := waistCm + hipsCm - neckCm
		if sum > 0 && profile.HeightCm > 0 {
			bf = 495/(1.29579-0.35004*math.Log10(sum)+0.22100*math.Log10(profile.HeightCm)) - 450
		}
	}
	if bf < 2 || bf > 80 {
		bf = 0
	}
	lean := 0.0
	fat := 0.0
	if weightKg > 0 && bf > 0 {
		fat = weightKg * bf / 100.0
		lean = weightKg - fat
	}
	return models.BodyFatResult{Method: "navy", BfPct: bf, LeanMassKg: lean, FatMassKg: fat}
}

// SkinfoldMethod computes bodyfat via Jackson-Pollock style skinfolds
// weightKg should be provided in kilograms and returned lean/fat masses are in kg.
func SkinfoldMethod(profile models.Profile, weightKg float64, measurements ...float64) models.BodyFatResult {
	// expect 5-site
	bf := 0.0
	if len(measurements) != 5 {
		return models.BodyFatResult{Method: "skinfold", BfPct: 0}
	}
	sum := 0.0
	for _, v := range measurements {
		if v <= 0 {
			return models.BodyFatResult{Method: "skinfold", BfPct: 0}
		}
		sum += v
	}
	age := profile.Age
	isMale := stringsToLower(profile.Sex) == "male"
	density := 0.0
	if isMale {
		density = 1.200 - 0.00109*sum + 0.00000143*sum*sum - 0.000139*float64(age)
	} else {
		density = 1.170 - 0.00094*sum + 0.00000112*sum*sum - 0.000131*float64(age)
	}
	if density > 0 {
		bf = 495/density - 450
	}
	if bf < 2 || bf > 60 {
		bf = 0
	}
	lean := 0.0
	fat := 0.0
	if weightKg > 0 && bf > 0 {
		fat = weightKg * bf / 100.0
		lean = weightKg - fat
	}
	return models.BodyFatResult{Method: "skinfold", BfPct: bf, LeanMassKg: lean, FatMassKg: fat}
}

// helper
func stringsToLower(s string) string {
	return strings.ToLower(s)
}
