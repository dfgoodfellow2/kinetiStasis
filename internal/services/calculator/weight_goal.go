package calculator

import (
	"fmt"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/constants"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
)

// WeightGoalAdjustment contains the computed adjustment from weight data
type WeightGoalAdjustment struct {
	DaysBetween          int     `json:"daysBetween"`
	WeightStart          float64 `json:"weightStart"`
	WeightEnd            float64 `json:"weightEnd"`
	WeightChange         float64 `json:"weightChange"`
	ExpectedWeightChange float64 `json:"expectedWeightChange"`
	WeightDiff           float64 `json:"weightDiff"` // actual - expected
	CalorieAdjustment    float64 `json:"calorieAdjustment"`
	RecommendedCalories  int     `json:"recommendedCalories"`
	Reason               string  `json:"reason"`
	CanCheckIn           bool    `json:"canCheckIn"`
	DaysSinceLastCheckIn int     `json:"daysSinceLastCheckIn"`
}

// ComputeWeightGoalAdjustment calculates target adjustments based on weight change vs expected
func ComputeWeightGoalAdjustment(
	profile models.Profile,
	currentTargets models.Targets,
	nutritionLogs []models.NutritionLog,
	biometricLogs []models.BiometricLog,
	lastCheckIn *models.CheckInLog,
	bodyFatPct float64,
) WeightGoalAdjustment {

	// 1. Check if we have enough weight data (need at least 3 days)
	hasEnoughWeightData := len(biometricLogs) >= 3

	// 2. Check if user can check in (5 days since last)
	daysSinceCheckIn := 999
	canCheckIn := true
	if lastCheckIn != nil {
		lastDate, _ := time.Parse("2006-01-02", lastCheckIn.CheckInDate)
		daysSinceCheckIn = int(time.Since(lastDate).Hours() / 24)
		canCheckIn = daysSinceCheckIn >= 5
	}

	// 3. Can only check in if we have both: time elapsed AND weight data
	canCheckIn = canCheckIn && hasEnoughWeightData

	// 4. If no weight data, return early with appropriate status
	if !hasEnoughWeightData {
		return WeightGoalAdjustment{
			CanCheckIn:           canCheckIn,
			DaysSinceLastCheckIn: daysSinceCheckIn,
			Reason:               "Insufficient weight data (need 5+ days)",
		}
	}

	// Sort by date ascending — assume input is already sorted
	weightStart := biometricLogs[0].WeightKg
	weightEnd := biometricLogs[len(biometricLogs)-1].WeightKg
	weightChange := weightEnd - weightStart
	// Actual calendar span between first and last biometric entries (inclusive)
	firstDate, _ := time.Parse("2006-01-02", biometricLogs[0].Date)
	lastDate, _ := time.Parse("2006-01-02", biometricLogs[len(biometricLogs)-1].Date)
	daysBetween := int(lastDate.Sub(firstDate).Hours()/24) + 1

	// 3. Calculate average intake over same period
	var totalCalories float64
	for _, log := range nutritionLogs {
		totalCalories += log.Calories
	}
	var avgIntake float64
	if len(nutritionLogs) > 0 {
		avgIntake = totalCalories / float64(len(nutritionLogs))
	}

	// 4. Get TDEE (observed from existing adaptive.go)
	// For now use estimated TDEE from profile
	tdee := computeEstimatedTDEE(profile, weightEnd, bodyFatPct)

	// 5. Calculate expected weight change
	// calorieDiff = intake - TDEE (positive = surplus, negative = deficit)
	calorieDiff := avgIntake - tdee
	expectedWeightChange := (calorieDiff * float64(daysBetween)) / 7700.0 // 7700 kcal per kg

	// 6. Compare actual vs expected
	weightDiff := weightChange - expectedWeightChange

	// 7. Calculate calorie adjustment
	// If weightDiff > 0: gained more than expected → need fewer calories
	// If weightDiff < 0: lost more than expected → need more calories
	calorieAdjustment := -weightDiff * 7700.0 / float64(daysBetween)

	// 8. Apply damping (max ±500 per week)
	// Scale to daily: 500 / 7 ≈ 71.4
	maxDailySwing := MaxWeeklyCalorieSwing / 7.0
	if calorieAdjustment > maxDailySwing {
		calorieAdjustment = maxDailySwing
	} else if calorieAdjustment < -maxDailySwing {
		calorieAdjustment = -maxDailySwing
	}

	// 9. Calculate recommended calories
	recommended := currentTargets.Calories + calorieAdjustment

	// 10. Apply floor
	if recommended < constants.MinCalorieFloor {
		recommended = constants.MinCalorieFloor
	}

	// 11. Determine reason string
	reason := determineReason(profile.Goal, weightChange, expectedWeightChange, int(currentTargets.Calories), int(recommended))

	return WeightGoalAdjustment{
		DaysBetween:          daysBetween,
		WeightStart:          weightStart,
		WeightEnd:            weightEnd,
		WeightChange:         weightChange,
		ExpectedWeightChange: expectedWeightChange,
		WeightDiff:           weightDiff,
		CalorieAdjustment:    calorieAdjustment,
		RecommendedCalories:  int(recommended),
		Reason:               reason,
		CanCheckIn:           canCheckIn,
		DaysSinceLastCheckIn: daysSinceCheckIn,
	}
}

func computeEstimatedTDEE(profile models.Profile, weightKg float64, bodyFatPct float64) float64 {
	var bmr float64

	// Use Katch-McArdle if body fat % is available and valid
	if bodyFatPct > 0 && bodyFatPct < 60 {
		// Lean Body Mass = weight × (1 - body fat %)
		lbm := weightKg * (1 - bodyFatPct/100)
		// Katch-McArdle: BMR = 370 + (21.6 × LBM)
		bmr = 370 + (21.6 * lbm)
	} else {
		// Mifflin-St Jeor fallback
		bmr = 10*weightKg + 6.25*float64(profile.HeightCm) - 5*float64(profile.Age)
		if profile.Sex == "male" {
			bmr += 5
		} else {
			bmr -= 161
		}
	}

	// Activity multiplier
	neat := 1.0
	switch profile.Activity {
	case "lightly_active":
		neat = 1.1
	case "moderately_active":
		neat = 1.2
	case "very_active":
		neat = 1.3
	}

	return bmr * neat
}

func determineReason(goal string, actualWeightChange, expectedWeightChange float64, currentCalories, recommended int) string {
	diff := recommended - currentCalories
	sign := "+"
	if diff < 0 {
		sign = "-"
		diff = -diff
	}

	switch goal {
	case "cut_10", "cut_20", "cut_30", "cut_40":
		if actualWeightChange > expectedWeightChange {
			return fmt.Sprintf("Weight not dropping as expected. Adjusting %s%d kcal to reach goal.", sign, diff)
		}
		return fmt.Sprintf("On track for cut. Adjusting %s%d kcal.", sign, diff)
	case "maintenance":
		if actualWeightChange > expectedWeightChange {
			return fmt.Sprintf("Gaining weight at maintenance. Adjusting %s%d kcal.", sign, diff)
		} else if actualWeightChange < expectedWeightChange {
			return fmt.Sprintf("Losing weight at maintenance. Adjusting %s%d kcal.", sign, diff)
		}
		return "Maintenance goals on track. Minor adjustment."
	case "bulk_10", "bulk_20":
		if actualWeightChange < expectedWeightChange {
			return fmt.Sprintf("Not gaining as expected. Adjusting %s%d kcal.", sign, diff)
		}
		return fmt.Sprintf("On track for bulk. Adjusting %s%d kcal.", sign, diff)
	default:
		return fmt.Sprintf("Adjusting %s%d kcal based on weight data.", sign, diff)
	}
}
