package models

// User represents an authenticated user account.
type User struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	IsAdmin   bool   `json:"is_admin"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// Profile holds all personal and physiological data for a user.
type Profile struct {
	UserID           string  `json:"user_id"`
	Name             string  `json:"name"`
	Age              int     `json:"age"`
	Sex              string  `json:"sex"`
	HeightCm         float64 `json:"height_cm"`
	Activity         string  `json:"activity"`
	ExerciseFreq     int     `json:"exercise_freq"`
	RunningKm        float64 `json:"running_km"`
	IsLifter         bool    `json:"is_lifter"`
	Goal             string  `json:"goal"`
	PrioritizeCarbs  bool    `json:"prioritize_carbs"`
	BfPct            float64 `json:"bf_pct"`
	HRRest           int     `json:"hr_rest"`
	HRMax            int     `json:"hr_max"`
	GripWeight       float64 `json:"grip_weight"`
	TDEELookbackDays int     `json:"tdee_lookback_days"`
	SleepQualityMax  float64 `json:"sleep_quality_max"`
	Units            string  `json:"units"`
	UpdatedAt        string  `json:"updated_at"`
}

// NutritionLog is one day's nutrition intake.
type NutritionLog struct {
	ID        string  `json:"id"`
	UserID    string  `json:"user_id"`
	Date      string  `json:"date"`
	Calories  float64 `json:"calories"`
	ProteinG  float64 `json:"protein_g"`
	CarbsG    float64 `json:"carbs_g"`
	FatG      float64 `json:"fat_g"`
	FiberG    float64 `json:"fiber_g"`
	WaterMl   float64 `json:"water_ml"`
	MealNotes string  `json:"meal_notes"`
	UpdatedAt string  `json:"updated_at"`
}

// BiometricLog is one day's body metrics.
type BiometricLog struct {
	ID             string  `json:"id"`
	UserID         string  `json:"user_id"`
	Date           string  `json:"date"`
	WeightKg       float64 `json:"weight_kg"`
	WaistCm        float64 `json:"waist_cm"`
	GripKg         float64 `json:"grip_kg"`
	BoltScore      float64 `json:"bolt_score"`
	SleepHours     float64 `json:"sleep_hours"`
	SleepQuality   float64 `json:"sleep_quality"`
	SubjectiveFeel int     `json:"subjective_feel"`
	BodyFatPct     float64 `json:"body_fat_pct"` // Manual body fat % entry
	Notes          string  `json:"notes"`
	UpdatedAt      string  `json:"updated_at"`
}

// ExerciseSet represents one set of an exercise within a workout.
// TUTSeconds serves dual purpose: time-under-tension for tempo-based sets,
// and total set duration for timed/isometric sets.
type ExerciseSet struct {
	Reps        int     `json:"reps"`
	LoadKg      float64 `json:"load_kg"`
	LoadLbs     float64 `json:"load_lbs"`
	TUTSeconds  float64 `json:"tut_seconds"`
	RestSeconds float64 `json:"rest_seconds"`
}

// ExerciseEntry represents one exercise (multiple sets) within a workout.
type ExerciseEntry struct {
	Name     string        `json:"name"`
	Category string        `json:"category"` // squat|hinge|push|pull|conditioning
	Sets     []ExerciseSet `json:"sets"`
	METValue float64       `json:"met_value"`
	Surface  string        `json:"surface"`
	Notes    string        `json:"notes"`
	// Additional fields for conditioning / carry / run exercises
	DistanceKm float64 `json:"distance_km,omitempty"`
	ElevationM float64 `json:"elevation_m,omitempty"`
	Pace       string  `json:"pace,omitempty"`
	RPE        float64 `json:"rpe,omitempty"`
	// LoadRaw preserves the original load string (e.g. "35+35 lbs", "BW")
	LoadRaw string `json:"load_raw,omitempty"`
	// DurationRaw preserves the original duration string (e.g. "35 sec", "2:30 min")
	DurationRaw string `json:"duration_raw,omitempty"`
	// Tempo preserves the original tempo string (e.g. "2-0-2-0") for form display
	Tempo string `json:"tempo,omitempty"`
	// Bias is the bilateral/unilateral indicator for this exercise
	Bias string `json:"bias,omitempty"` // "bilateral" | "unilateral" | ""
	// IntensityRelMax is the estimated % of 1RM (0.0-1.0)
	IntensityRelMax float64 `json:"intensity_rel_max,omitempty"`
	// IntensitySource indicates where intensity was derived from: "1rm", "rpe", "hr", or "default"
	IntensitySource string `json:"intensity_source,omitempty"`
}

// WorkoutMetadata holds session-level fields not tracked in individual columns.
type WorkoutMetadata struct {
	Type         string   `json:"type"`  // strength|conditioning|hiit|cardio|zone2|mobility|sport|yoga
	Style        string   `json:"style"` // circuit|emom|amrap|for-time|hiit|cardio
	Surface      string   `json:"surface"`
	Focus        []string `json:"focus"` // e.g. ["Hinge(B)", "Push(U)"]
	RestInterval string   `json:"rest_interval"`
	RPE          float64  `json:"rpe"`
	AvgHR        int      `json:"avg_hr"`
	MaxHR        int      `json:"max_hr"`
	Recovers     string   `json:"recovers"`
	Day          int      `json:"day"`
}

// WorkoutEntry is a single training session (identified by date + slot).
type WorkoutEntry struct {
	ID             string          `json:"id"`
	UserID         string          `json:"user_id"`
	Date           string          `json:"date"`
	Slot           string          `json:"slot"`
	Title          string          `json:"title"`
	RawNotes       string          `json:"raw_notes"`
	DurationMin    float64         `json:"duration_min"`
	CaloriesBurned float64         `json:"calories_burned"`
	MWV            float64         `json:"mwv"`
	NDS            float64         `json:"nds"`
	SessionDensity float64         `json:"session_density"`
	Exercises      []ExerciseEntry `json:"exercises"`
	Metadata       WorkoutMetadata `json:"metadata"`
	UpdatedAt      string          `json:"updated_at"`
}

// Targets holds the current daily nutrition targets for a user.
type Targets struct {
	UserID          string  `json:"user_id"`
	Calories        float64 `json:"calories"`
	ProteinG        float64 `json:"protein_g"`
	CarbsG          float64 `json:"carbs_g"`
	FatG            float64 `json:"fat_g"`
	FiberG          float64 `json:"fiber_g"`
	WaterMl         float64 `json:"water_ml"`
	EatBackExercise bool    `json:"eat_back_exercise"`
	UpdatedAt       string  `json:"updated_at"`
}

// TargetSnapshot is a historical record of targets on a given date.
type TargetSnapshot struct {
	ID            string  `json:"id"`
	UserID        string  `json:"user_id"`
	EffectiveDate string  `json:"effective_date"`
	Calories      float64 `json:"calories"`
	ProteinG      float64 `json:"protein_g"`
	CarbsG        float64 `json:"carbs_g"`
	FatG          float64 `json:"fat_g"`
	FiberG        float64 `json:"fiber_g"`
	CreatedAt     string  `json:"created_at"`
}

// SavedMeal is a named, reusable meal with its macro breakdown.
type SavedMeal struct {
	ID        string  `json:"id"`
	UserID    string  `json:"user_id"`
	Name      string  `json:"name"`
	Calories  float64 `json:"calories"`
	ProteinG  float64 `json:"protein_g"`
	CarbsG    float64 `json:"carbs_g"`
	FatG      float64 `json:"fat_g"`
	FiberG    float64 `json:"fiber_g"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

// MealTemplate is a named collection of meal items.
type MealTemplate struct {
	ID        string      `json:"id"`
	UserID    string      `json:"user_id"`
	Name      string      `json:"name"`
	Meals     []SavedMeal `json:"meals"`
	CreatedAt string      `json:"created_at"`
	UpdatedAt string      `json:"updated_at"`
}

// BodyMeasurement is a set of body circumference measurements on a given date.
type BodyMeasurement struct {
	ID          string  `json:"id"`
	UserID      string  `json:"user_id"`
	Date        string  `json:"date"`
	NeckCm      float64 `json:"neck_cm"`
	ChestCm     float64 `json:"chest_cm"`
	WaistCm     float64 `json:"waist_cm"`
	HipsCm      float64 `json:"hips_cm"`
	ThighCm     float64 `json:"thigh_cm"`
	BicepCm     float64 `json:"bicep_cm"`
	ShouldersCm float64 `json:"shoulders_cm"`
	CalvesCm    float64 `json:"calves_cm"`
	Notes       string  `json:"notes"`
	CreatedAt   string  `json:"created_at"`
}

// --- Computed/derived types ---

type TDEEResult struct {
	EstimatedTDEE  float64 `json:"estimated_tdee"`
	ObservedTDEE   float64 `json:"observed_tdee"`
	Confidence     string  `json:"confidence"`
	DaysOfData     int     `json:"days_of_data"`
	LookbackDays   int     `json:"lookback_days"`
	Method         string  `json:"method"`
	EmergencyAlert bool    `json:"emergency_alert"`
}

type MacroResult struct {
	Calories  float64 `json:"calories"`
	ProteinG  float64 `json:"protein_g"`
	CarbsG    float64 `json:"carbs_g"`
	FatG      float64 `json:"fat_g"`
	FiberG    float64 `json:"fiber_g"`
	WaterMl   float64 `json:"water_ml"`
	GoalLabel string  `json:"goal_label"`
}

type ReadinessResult struct {
	// Primary signal — what the UI should use
	Level   string  `json:"level"`   // "green", "yellow", "red"
	Message string  `json:"message"` // contextual coaching text e.g. "Ready", "Mildly fatigued"
	Rz      float64 `json:"rz"`      // combined weighted z-score (context-relative, not 0-100)

	// Velocity
	VelocityTrend string  `json:"velocity_trend"` // "improving", "stable", "declining"
	VelocityDelta float64 `json:"velocity_delta"` // Rz delta between current and prior 7-day avg

	// Sub-scores for detail views
	GripZ float64 `json:"grip_z"`
	BoltZ float64 `json:"bolt_z"`

	// Notes / warnings
	Notes []string `json:"notes"`

	// Legacy compat fields (derived, kept so any existing callers don't break)
	Score    float64 `json:"score"`    // 50 + (Rz/3)*50 clamped 0-100
	Category string  `json:"category"` // mirrors Level: "low"/"moderate"/"high"
}

type BodyFatResult struct {
	Method     string  `json:"method"`
	BfPct      float64 `json:"bf_pct"`
	LeanMassKg float64 `json:"lean_mass_kg"`
	FatMassKg  float64 `json:"fat_mass_kg"`
}

type TodaySummary struct {
	Date         string       `json:"date"`
	Consumed     NutritionLog `json:"consumed"`
	Targets      Targets      `json:"targets"`
	CaloriesLeft float64      `json:"calories_left"`
	ProteinLeft  float64      `json:"protein_left"`
	ProgressPct  float64      `json:"progress_pct"`
}

type WeeklyStats struct {
	AvgCalories   float64 `json:"avg_calories"`
	AvgProteinG   float64 `json:"avg_protein_g"`
	TotalWorkouts int     `json:"total_workouts"`
	TotalMWV      float64 `json:"total_mwv"`
	AvgSleepHours float64 `json:"avg_sleep_hours"`
	AvgWeightKg   float64 `json:"avg_weight_kg"`
	DailyLogged   []int   `json:"daily_logged"`   // 30-day array: 1 if any data logged, 0 if not
	CurrentStreak int     `json:"current_streak"` // Consecutive days logged ending today
	LongestStreak int     `json:"longest_streak"` // Longest consecutive logging streak in 30 days
	TodayLogged   bool    `json:"today_logged"`   // Whether today has any logged data (for fire indicator)
}

type DashboardData struct {
	Today       TodaySummary    `json:"today"`
	TDEE        TDEEResult      `json:"tdee"`
	Macros      MacroResult     `json:"macros"`
	Readiness   ReadinessResult `json:"readiness"`
	WeeklyStats WeeklyStats     `json:"weekly_stats"`
	WeightTrend []struct {
		Date     string  `json:"date"`
		WeightKg float64 `json:"weight_kg"`
	} `json:"weight_trend"`
	TodayBio         *BiometricLog `json:"today_bio"`
	GripPersonalBest float64       `json:"grip_personal_best"`
	WorkoutToday     bool          `json:"workout_today"`
}

type ParsedMeal struct {
	Calories  float64 `json:"calories"`
	ProteinG  float64 `json:"protein_g"`
	CarbsG    float64 `json:"carbs_g"`
	FatG      float64 `json:"fat_g"`
	FiberG    float64 `json:"fiber_g"`
	WaterMl   float64 `json:"water_ml"`
	MealNotes string  `json:"meal_notes"`
	RawInput  string  `json:"raw_input"`
}

// ParsedWorkout is returned by POST /v1/parse/workout (YAML or AI).
// It is a superset of WorkoutEntry — includes all session metadata.
type ParsedWorkout struct {
	// Core fields (map directly to WorkoutEntry)
	Title          string          `json:"title"`
	Slot           string          `json:"slot"`
	DurationMin    float64         `json:"duration_min"`
	CaloriesBurned float64         `json:"calories_burned"`
	Exercises      []ExerciseEntry `json:"exercises"`
	RawInput       string          `json:"raw_input"`
	Notes          string          `json:"notes,omitempty"`
	// Session metadata (maps to WorkoutMetadata)
	Type         string   `json:"type"`
	Style        string   `json:"style"`
	Surface      string   `json:"surface"`
	Focus        []string `json:"focus"`
	RestInterval string   `json:"rest_interval"`
	RPE          float64  `json:"rpe"`
	AvgHR        int      `json:"avg_hr"`
	MaxHR        int      `json:"max_hr"`
	Recovers     string   `json:"recovers"`
	Day          int      `json:"day"`
}
