package metrics

import (
	"math"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/constants"
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

// weeklyWindowDays is the inclusive length of the weekly stats window: today
// plus the six preceding calendar days. Callers must not shrink their query
// window to match it, because the same inputs also feed the 30-day streak
// block below. It aliases the shared constant so the dashboard handler cannot
// drift from this package.
const weeklyWindowDays = constants.WeeklyStatsWindowDays

// inWeeklyWindow reports whether a YYYY-MM-DD date belongs to the inclusive
// window [weekStart, today]. Dates in that format sort lexicographically, so a
// plain string comparison is exact. An empty weekStart (unparseable today)
// deliberately accepts every date, preserving the old unfiltered behaviour
// instead of silently returning zeroes.
func inWeeklyWindow(date, weekStart, today string) bool {
	if weekStart == "" {
		return true
	}
	return date >= weekStart && date <= today
}

// WeeklyStats summarizes last 7 days with 30-day logging streak data
// today is the client's local date in YYYY-MM-DD format (used to avoid UTC mismatch)
//
// Window ownership: the 7-day metrics (nutrition averages, biometric averages,
// workout count, and MWV total) are filtered here to [today-6, today], so the
// caller is expected to pass the full 30-day slices. Passing 30-day data keeps
// the streak block able to see a 30-day history; the weekly figures are
// narrowed internally so the numbers always match the "Weekly Stats" card
// regardless of how wide a slice the caller supplies. Previously the caller
// passed a 30-day workout slice straight into TotalWorkouts, while the card
// claimed a week.
func WeeklyStats(nutritionLogs []models.NutritionLog, biometrics []models.BiometricLog, workouts []models.WorkoutEntry, today string) models.WeeklyStats {
	ws := models.WeeklyStats{}

	// Parse the caller-supplied today (YYYY-MM-DD) once: both the 7-day metric
	// window and the 30-day streak window key off it, and parsing here (rather
	// than depending on the handler) keeps the two windows consistent.
	todayTime, parseErr := time.Parse(constants.DateFormat, today)

	weekStart := ""
	if parseErr == nil {
		weekStart = todayTime.AddDate(0, 0, -(weeklyWindowDays - 1)).Format(constants.DateFormat)
	}

	// 7-day averages from nutrition
	sumC, sumP, countNutrition := 0.0, 0.0, 0
	for _, l := range nutritionLogs {
		if !inWeeklyWindow(l.Date, weekStart, today) {
			continue
		}
		sumC += l.Calories
		sumP += l.ProteinG
		countNutrition++
	}
	if countNutrition > 0 {
		ws.AvgCalories = sumC / float64(countNutrition)
		ws.AvgProteinG = sumP / float64(countNutrition)
	}

	// 7-day workouts: count and MWV must both reflect the same window the
	// card advertises, so out-of-window entries contribute to neither.
	for _, w := range workouts {
		if !inWeeklyWindow(w.Date, weekStart, today) {
			continue
		}
		ws.TotalWorkouts++
		ws.TotalMWV += w.MWV
	}

	// biometrics: avg sleep and weight over the same 7-day window
	sumSleep, sumWeight := 0.0, 0.0
	countSleep, countWeight := 0, 0
	for _, b := range biometrics {
		if !inWeeklyWindow(b.Date, weekStart, today) {
			continue
		}
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

	// 30-day logging streak calculation
	// Check all data sources (biometrics, nutrition, workouts) for the last 30 days.
	// These blocks intentionally use the full input slices, not the 7-day
	// filtered view, so streaks can span the whole 30-day history.
	thirtyDaysAgo := todayTime.AddDate(0, 0, -30).Format(constants.DateFormat)

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
		dateStr := day.Format(constants.DateFormat)

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

// LoggingStreaks derives a nutrition-logging calendar and streak counts in a
// single pass over the supplied logs.
//
// loggedDays has window entries ordered oldest -> newest, so the final element
// always represents today; true means at least one nutrition log exists for
// that date. The current streak ends today when today is logged, otherwise it
// counts consecutive logged days back from yesterday. longestStreak is the
// longest consecutive run inside the same window.
func LoggingStreaks(nutritionLogs []models.NutritionLog, today string, window int) (loggedDays []bool, currentStreak, longestStreak int) {
	if window <= 0 {
		window = 60
	}
	loggedDays = make([]bool, window)

	todayTime, err := time.Parse("2006-01-02", today)
	if err != nil {
		return loggedDays, 0, 0
	}

	loggedDates := make(map[string]bool, len(nutritionLogs))
	for _, n := range nutritionLogs {
		if n.Date != "" {
			loggedDates[n.Date] = true
		}
	}

	run := 0
	for i := 0; i < window; i++ {
		date := todayTime.AddDate(0, 0, -(window - 1 - i)).Format("2006-01-02")
		if loggedDates[date] {
			loggedDays[i] = true
			run++
			if run > longestStreak {
				longestStreak = run
			}
		} else {
			run = 0
		}
	}

	// Walk backwards from today, or from yesterday when today has not been
	// logged yet, so the streak survives an unlogged current day.
	start := window - 1
	if !loggedDays[start] {
		start--
	}
	for i := start; i >= 0 && loggedDays[i]; i-- {
		currentStreak++
	}

	return loggedDays, currentStreak, longestStreak
}
