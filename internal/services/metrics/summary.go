package metrics

import (
	"math"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
)

// EMA returns the final value of an exponential moving average
func EMA(values []float64, alpha float64) float64 {
	if len(values) == 0 {
		return 0
	}
	ema := values[0]
	for i := 1; i < len(values); i++ {
		ema = alpha*values[i] + (1-alpha)*ema
	}
	return ema
}

// computeEMASeries was an internal helper to build a full EMA time series.
// It is unused and has been removed to keep the codebase tidy.

// StandardDeviation calculates population SD
func StandardDeviation(values []float64) float64 {
	n := len(values)
	if n < 2 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(n)
	var varsum float64
	for _, v := range values {
		d := v - mean
		varsum += d * d
	}
	variance := varsum / float64(n)
	return math.Sqrt(variance)
}

// ZScore computes how many SDs current is from ema (ema used as mean)
func ZScore(current, ema, sd float64) float64 {
	if sd == 0 {
		return 0
	}
	return (current - ema) / sd
}

// TodaySummary produces today's nutrition progress
func TodaySummary(date string, logs []models.NutritionLog, targets models.Targets) models.TodaySummary {
	var consumed models.NutritionLog
	for _, l := range logs {
		if l.Date == date {
			consumed = l
			break
		}
	}
	caloriesLeft := targets.Calories - consumed.Calories
	proteinLeft := targets.ProteinG - consumed.ProteinG
	progress := 0.0
	if targets.Calories > 0 {
		progress = consumed.Calories / targets.Calories * 100.0
	}
	return models.TodaySummary{
		Date:         date,
		Consumed:     consumed,
		Targets:      targets,
		CaloriesLeft: caloriesLeft,
		ProteinLeft:  proteinLeft,
		ProgressPct:  progress,
	}
}

// WeeklyStats summarizes last 7 days
func WeeklyStats(nutritionLogs []models.NutritionLog, biometrics []models.BiometricLog, workouts []models.WorkoutEntry) models.WeeklyStats {
	ws := models.WeeklyStats{}
	// Avg calories/protein from nutrition logs
	if len(nutritionLogs) > 0 {
		sumC, sumP := 0.0, 0.0
		for _, l := range nutritionLogs {
			sumC += l.Calories
			sumP += l.ProteinG
		}
		ws.AvgCalories = sumC / float64(len(nutritionLogs))
		ws.AvgProteinG = sumP / float64(len(nutritionLogs))
	}
	// workouts
	ws.TotalWorkouts = len(workouts)
	totalMWV := 0.0
	for _, w := range workouts {
		totalMWV += w.MWV
	}
	ws.TotalMWV = totalMWV

	// biometrics: avg sleep and weight
	if len(biometrics) > 0 {
		sumSleep, sumWeight := 0.0, 0.0
		countSleep, countWeight := 0, 0
		for _, b := range biometrics {
			if b.SleepHours > 0 {
				sumSleep += b.SleepHours
				countSleep++
			}
			if b.WeightKg > 0 {
				sumWeight += b.WeightKg
				countWeight++
			}
		}
		if countSleep > 0 {
			ws.AvgSleepHours = sumSleep / float64(countSleep)
		}
		if countWeight > 0 {
			ws.AvgWeightKg = sumWeight / float64(countWeight)
		}
	}
	return ws
}
