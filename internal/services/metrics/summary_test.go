package metrics

// WeeklyStats window coverage. The dashboard card is labelled "Weekly Stats",
// so TotalWorkouts and TotalMWV must count only [today-6, today] even though
// the handler passes 30-day slices (the same inputs feed the 30-day streak
// block). The boundary cases below pin the inclusive start day (today-6) and
// the excluded first day outside the window (today-7).

import (
	"testing"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
)

const weeklyToday = "2026-11-13" // window: 2026-11-07 .. 2026-11-13

func workout(date string, mwv float64) models.WorkoutEntry {
	return models.WorkoutEntry{Date: date, MWV: mwv}
}

func TestWeeklyStatsWorkoutWindow(t *testing.T) {
	cases := []struct {
		name      string
		workouts  []models.WorkoutEntry
		wantCount int
		wantMWV   float64
	}{
		{
			name: "empty slice",
			// No workouts at all: the card must read zero, not NaN.
			workouts:  nil,
			wantCount: 0,
			wantMWV:   0,
		},
		{
			name: "all inside the window",
			workouts: []models.WorkoutEntry{
				workout("2026-11-07", 10), // first in-window day
				workout("2026-11-10", 20),
				workout("2026-11-13", 30), // today
			},
			wantCount: 3,
			wantMWV:   60,
		},
		{
			name: "boundary today-6 included and today-7 excluded",
			workouts: []models.WorkoutEntry{
				workout("2026-11-06", 100), // today-7: outside
				workout("2026-11-07", 7),   // today-6: inside (inclusive start)
			},
			wantCount: 1,
			wantMWV:   7,
		},
		{
			name: "all outside the window",
			workouts: []models.WorkoutEntry{
				workout("2026-10-20", 50),
				workout("2026-11-06", 60), // the off-by-one day
			},
			wantCount: 0,
			wantMWV:   0,
		},
		{
			name: "future-dated workout excluded",
			// Entries after today cannot be part of a window ending today.
			workouts: []models.WorkoutEntry{
				workout("2026-11-12", 5),
				workout("2026-11-14", 500),
			},
			wantCount: 1,
			wantMWV:   5,
		},
		{
			name: "30-day slice counts only the week",
			// Regression for the original bug: the handler's 30-day workout
			// slice must not inflate the weekly count. 30 daily entries, one
			// per day ending today, collapse to the 7 in-window days.
			workouts:  dailyWorkouts(30),
			wantCount: 7,
			wantMWV:   28, // 7 in-window entries at 4 MWV each
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := WeeklyStats(nil, nil, tc.workouts, weeklyToday)
			if got.TotalWorkouts != tc.wantCount {
				t.Fatalf("TotalWorkouts = %d, want %d", got.TotalWorkouts, tc.wantCount)
			}
			if got.TotalMWV != tc.wantMWV {
				t.Fatalf("TotalMWV = %v, want %v (same window as TotalWorkouts)", got.TotalMWV, tc.wantMWV)
			}
		})
	}
}

// dailyWorkouts builds one entry per day ending on weeklyToday, oldest first,
// so a 30-day slice contains exactly 7 in-window days.
func dailyWorkouts(n int) []models.WorkoutEntry {
	// Date arithmetic through the package's own windowing would be circular;
	// hard-coding the relevant days keeps this helper about slice shape only.
	days := make([]string, 0, n)
	for i := n - 1; i >= 0; i-- {
		days = append(days, addDays(weeklyToday, -i))
	}
	out := make([]models.WorkoutEntry, 0, n)
	for _, d := range days {
		out = append(out, workout(d, 4))
	}
	return out
}

// addDays shifts a YYYY-MM-DD date by n calendar days. Kept local to the test
// so the assertions do not depend on the implementation's window arithmetic.
func addDays(date string, n int) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		panic(err)
	}
	return t.AddDate(0, 0, n).Format("2006-01-02")
}

func TestWeeklyStatsAveragesUseSevenDayWindow(t *testing.T) {
	nutritionLogs := []models.NutritionLog{
		{Date: "2026-11-01", Calories: 4000, ProteinG: 400}, // outside
		{Date: "2026-11-06", Calories: 4000, ProteinG: 400}, // today-7: outside
		{Date: "2026-11-07", Calories: 2000, ProteinG: 150}, // today-6: inside
		{Date: "2026-11-13", Calories: 2400, ProteinG: 170}, // today: inside
		{Date: "2026-11-14", Calories: 9000, ProteinG: 900}, // future: outside
	}
	biometrics := []models.BiometricLog{
		{Date: "2026-11-05", WeightKg: 100, SleepHours: 9},  // outside
		{Date: "2026-11-08", WeightKg: 80, SleepHours: 7.0}, // inside
		{Date: "2026-11-12", WeightKg: 79, SleepHours: 7.5}, // inside
	}

	got := WeeklyStats(nutritionLogs, biometrics, nil, weeklyToday)

	if got.AvgCalories != 2200 {
		t.Fatalf("AvgCalories = %v, want 2200 (2 in-window logs)", got.AvgCalories)
	}
	if got.AvgProteinG != 160 {
		t.Fatalf("AvgProteinG = %v, want 160 (2 in-window logs)", got.AvgProteinG)
	}
	if got.AvgWeightKg != 79.5 {
		t.Fatalf("AvgWeightKg = %v, want 79.5 (2 in-window weigh-ins)", got.AvgWeightKg)
	}
	if got.AvgSleepHours != 7.25 {
		t.Fatalf("AvgSleepHours = %v, want 7.25 (2 in-window nights)", got.AvgSleepHours)
	}
}

// TestWeeklyStatsStreakStillUsesFullThirtyDays guards the reason the handler
// keeps passing 30-day slices: narrowing the weekly metrics must not truncate
// the streak map. A full month of workouts should read as a 30-day streak while
// TotalWorkouts still reports only the 7 in-window sessions.
func TestWeeklyStatsStreakStillUsesFullThirtyDays(t *testing.T) {
	workouts := dailyWorkouts(30)

	got := WeeklyStats(nil, nil, workouts, weeklyToday)

	if got.TotalWorkouts != 7 {
		t.Fatalf("TotalWorkouts = %d, want 7", got.TotalWorkouts)
	}
	if got.CurrentStreak != 30 {
		t.Fatalf("CurrentStreak = %d, want 30 (streak must use the full slice)", got.CurrentStreak)
	}
	if got.LongestStreak != 30 {
		t.Fatalf("LongestStreak = %d, want 30", got.LongestStreak)
	}
	if len(got.DailyLogged) != 30 {
		t.Fatalf("DailyLogged len = %d, want 30", len(got.DailyLogged))
	}
	if !got.TodayLogged {
		t.Fatal("TodayLogged = false, want true for a workout logged today")
	}
}
