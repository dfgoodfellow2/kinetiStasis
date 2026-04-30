package metrics

import (
	"math"
	"time"

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

// WeeklyStats summarizes last 7 days with 30-day logging streak data
// today is the client's local date in YYYY-MM-DD format (used to avoid UTC mismatch)
func WeeklyStats(nutritionLogs []models.NutritionLog, biometrics []models.BiometricLog, workouts []models.WorkoutEntry, today string) models.WeeklyStats {
	ws := models.WeeklyStats{}

	// 7-day averages from nutrition
	if len(nutritionLogs) > 0 {
		sumC, sumP := 0.0, 0.0
		for _, l := range nutritionLogs {
			sumC += l.Calories
			sumP += l.ProteinG
		}
		ws.AvgCalories = sumC / float64(len(nutritionLogs))
		ws.AvgProteinG = sumP / float64(len(nutritionLogs))
	}

	// 7-day workouts
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

	// 30-day logging streak calculation
	// Check all data sources (biometrics, nutrition, workouts) for the last 30 days
	// Parse the caller-supplied today (YYYY-MM-DD) so streaks are computed relative to the
	// same local date the front-end / handler is using.
	todayTime, _ := time.Parse("2006-01-02", today)
	thirtyDaysAgo := todayTime.AddDate(0, 0, -30).Format("2006-01-02")

	// Build maps of dates that have data
	bioDates := make(map[string]bool)
	for _, b := range biometrics {
		if b.Date >= thirtyDaysAgo {
			bioDates[b.Date] = true
		}
	}
	nutDates := make(map[string]bool)
	for _, n := range nutritionLogs {
		if n.Date >= thirtyDaysAgo {
			nutDates[n.Date] = true
		}
	}
	workoutDates := make(map[string]bool)
	for _, w := range workouts {
		if w.Date >= thirtyDaysAgo {
			workoutDates[w.Date] = true
		}
	}

	// Build 30-day array (index 0 = 30 days ago, index 29 = today)
	dailyLogged := make([]int, 30)
	currentStreak := 0
	longestStreak := 0
	tempStreak := 0

	for i := 0; i < 30; i++ {
		day := todayTime.AddDate(0, 0, -29+i) // -29 to get 30 days ending today
		dateStr := day.Format("2006-01-02")

		// Check if ANY data was logged that day
		hasData := bioDates[dateStr] || nutDates[dateStr] || workoutDates[dateStr]
		if hasData {
			dailyLogged[i] = 1
		} else {
			dailyLogged[i] = 0
		}

		// Track streaks
		if hasData {
			tempStreak++
			if tempStreak > longestStreak {
				longestStreak = tempStreak
			}
		} else {
			tempStreak = 0
		}
	}

	// Current streak (consecutive days with data, with 1-day grace)
	// Allow counting yesterday if today has no data yet
	currentStreak = 0
	graceUsed := false
	for i := 29; i >= 0; i-- {
		if dailyLogged[i] == 1 {
			currentStreak++
		} else if !graceUsed && i < 29 {
			// Allow one gap (grace day)
			graceUsed = true
			continue
		} else {
			break
		}
	}

	// Track if today is logged for fire streak indicator
	todayLogged := dailyLogged[29] == 1

	ws.DailyLogged = dailyLogged
	ws.CurrentStreak = currentStreak
	ws.LongestStreak = longestStreak
	ws.TodayLogged = todayLogged

	return ws
}
