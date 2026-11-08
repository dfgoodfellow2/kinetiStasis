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

// helper
func stringsToLower(s string) string {
	return strings.ToLower(s)
}
