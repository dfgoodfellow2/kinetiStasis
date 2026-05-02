package models

// User represents an authenticated user account.
type User struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	IsAdmin   bool   `json:"isAdmin"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// Profile holds all personal and physiological data for a user.
type Profile struct {
	UserID           string  `json:"userId"`
	Name             string  `json:"name"`
	Age              int     `json:"age"`
	Sex              string  `json:"sex"`
	HeightCm         float64 `json:"heightCm"`
	Activity         string  `json:"activity"`
	ExerciseFreq     int     `json:"exerciseFreq"`
	RunningKm        float64 `json:"runningKm"`
	IsLifter         bool    `json:"isLifter"`
	Goal             string  `json:"goal"`
	PrioritizeCarbs  bool    `json:"prioritizeCarbs"`
	BfPct            float64 `json:"bfPct"`
	HRRest           int     `json:"hrRest"`
	HRMax            int     `json:"hrMax"`
	GripWeight       float64 `json:"gripWeight"`
	TDEELookbackDays int     `json:"tdeeLookbackDays"`
	SleepQualityMax  float64 `json:"sleepQualityMax"`
	Units            string  `json:"units"`
	UpdatedAt        string  `json:"updatedAt"`
}

// NutritionLog is one day's nutrition intake.
type NutritionLog struct {
	ID        string  `json:"id"`
	UserID    string  `json:"userId"`
	Date      string  `json:"date"`
	Calories  float64 `json:"calories"`
	ProteinG  float64 `json:"proteinG"`
	CarbsG    float64 `json:"carbsG"`
	FatG      float64 `json:"fatG"`
	FiberG    float64 `json:"fiberG"`
	WaterMl   float64 `json:"waterMl"`
	MealNotes string  `json:"mealNotes"`
	UpdatedAt string  `json:"updatedAt"`
}

// BiometricLog is one day's body metrics.
type BiometricLog struct {
	ID             string  `json:"id"`
	UserID         string  `json:"userId"`
	Date           string  `json:"date"`
	WeightKg       float64 `json:"weightKg"`
	WaistCm        float64 `json:"waistCm"`
	GripKg         float64 `json:"gripKg"`
	BoltScore      float64 `json:"boltScore"`
	SleepHours     float64 `json:"sleepHours"`
	SleepQuality   float64 `json:"sleepQuality"`
	SubjectiveFeel int     `json:"subjectiveFeel"`
	BodyFatPct     float64 `json:"bodyFatPct"` // Manual body fat % entry
	Notes          string  `json:"notes"`
	UpdatedAt      string  `json:"updatedAt"`
}

// ExerciseSet represents one set of an exercise within a workout.
// TUTSeconds serves dual purpose: time-under-tension for tempo-based sets,
// and total set duration for timed/isometric sets.
type ExerciseSet struct {
	Reps        int     `json:"reps"`
	LoadKg      float64 `json:"loadKg"`
	LoadLbs     float64 `json:"loadLbs"`
	TUTSeconds  float64 `json:"tutSeconds"`
	RestSeconds float64 `json:"restSeconds"`
}

// ExerciseEntry represents one exercise (multiple sets) within a workout.
type ExerciseEntry struct {
	Name     string        `json:"name"`
	Category string        `json:"category"` // squat|hinge|push|pull|conditioning
	Sets     []ExerciseSet `json:"sets"`
	METValue float64       `json:"metValue"`
	Surface  string        `json:"surface"`
	Notes    string        `json:"notes"`
	// Additional fields for conditioning / carry / run exercises
	DistanceKm float64 `json:"distanceKm,omitempty"`
	ElevationM float64 `json:"elevationM,omitempty"`
	Pace       string  `json:"pace,omitempty"`
	RPE        float64 `json:"rpe,omitempty"`
	// LoadRaw preserves the original load string (e.g. "35+35 lbs", "BW")
	LoadRaw string `json:"loadRaw,omitempty"`
	// DurationRaw preserves the original duration string (e.g. "35 sec", "2:30 min")
	DurationRaw string `json:"durationRaw,omitempty"`
	// Tempo preserves the original tempo string (e.g. "2-0-2-0") for form display
	Tempo string `json:"tempo,omitempty"`
	// Bias is the bilateral/unilateral indicator for this exercise
	Bias string `json:"bias,omitempty"` // "bilateral" | "unilateral" | ""
	// IntensityRelMax is the estimated % of 1RM (0.0-1.0)
	IntensityRelMax float64 `json:"intensityRelMax,omitempty"`
	// IntensitySource indicates where intensity was derived from: "1rm", "rpe", "hr", or "default"
	IntensitySource string `json:"intensitySource,omitempty"`
}

// WorkoutMetadata holds session-level fields not tracked in individual columns.
type WorkoutMetadata struct {
	Type         string   `json:"type"`  // strength|conditioning|hiit|cardio|zone2|mobility|sport|yoga
	Style        string   `json:"style"` // circuit|emom|amrap|for-time|hiit|cardio
	Surface      string   `json:"surface"`
	Focus        []string `json:"focus"` // e.g. ["Hinge(B)", "Push(U)"]
	RestInterval string   `json:"restInterval"`
	RPE          float64  `json:"rpe"`
	AvgHR        int      `json:"avgHr"`
	MaxHR        int      `json:"maxHr"`
	Recovers     string   `json:"recovers"`
	Day          int      `json:"day"`
}

// WorkoutEntry is a single training session (identified by date + slot).
type WorkoutEntry struct {
	ID             string          `json:"id"`
	UserID         string          `json:"userId"`
	Date           string          `json:"date"`
	Slot           string          `json:"slot"`
	Title          string          `json:"title"`
	RawNotes       string          `json:"rawNotes"`
	DurationMin    float64         `json:"durationMin"`
	CaloriesBurned float64         `json:"caloriesBurned"`
	MWV            float64         `json:"mwv"`
	NDS            float64         `json:"nds"`
	SessionDensity float64         `json:"sessionDensity"`
	Exercises      []ExerciseEntry `json:"exercises"`
	Metadata       WorkoutMetadata `json:"metadata"`
	UpdatedAt      string          `json:"updatedAt"`
}

// Targets holds the current daily nutrition targets for a user.
type Targets struct {
	UserID          string  `json:"userId"`
	Calories        float64 `json:"calories"`
	ProteinG        float64 `json:"proteinG"`
	CarbsG          float64 `json:"carbsG"`
	FatG            float64 `json:"fatG"`
	FiberG          float64 `json:"fiberG"`
	WaterMl         float64 `json:"waterMl"`
	EatBackExercise bool    `json:"eatBackExercise"`
	UpdatedAt       string  `json:"updatedAt"`
}

// TargetSnapshot is a historical record of targets on a given date.
type TargetSnapshot struct {
	ID            string  `json:"id"`
	UserID        string  `json:"userId"`
	EffectiveDate string  `json:"effectiveDate"`
	Calories      float64 `json:"calories"`
	ProteinG      float64 `json:"proteinG"`
	CarbsG        float64 `json:"carbsG"`
	FatG          float64 `json:"fatG"`
	FiberG        float64 `json:"fiberG"`
	CreatedAt     string  `json:"createdAt"`
}

// CheckInLog records a weekly check-in event and the state before/after
type CheckInLog struct {
	ID             string  `json:"id"`
	UserID         string  `json:"userId"`
	CheckInDate    string  `json:"checkInDate"`
	WeightBefore   float64 `json:"weightBefore"`
	WeightAfter    float64 `json:"weightAfter"`
	CaloriesBefore int     `json:"caloriesBefore"`
	CaloriesAfter  int     `json:"caloriesAfter"`
	Reason         string  `json:"reason"`
	CreatedAt      string  `json:"createdAt"`
}

// SavedMeal is a named, reusable meal with its macro breakdown.
type SavedMeal struct {
	ID        string  `json:"id"`
	UserID    string  `json:"userId"`
	Name      string  `json:"name"`
	Calories  float64 `json:"calories"`
	ProteinG  float64 `json:"proteinG"`
	CarbsG    float64 `json:"carbsG"`
	FatG      float64 `json:"fatG"`
	FiberG    float64 `json:"fiberG"`
	CreatedAt string  `json:"createdAt"`
	UpdatedAt string  `json:"updatedAt"`
}

// MealTemplate is a named collection of meal items.
type MealTemplate struct {
	ID        string      `json:"id"`
	UserID    string      `json:"userId"`
	Name      string      `json:"name"`
	Meals     []SavedMeal `json:"meals"`
	CreatedAt string      `json:"createdAt"`
	UpdatedAt string      `json:"updatedAt"`
}

// BodyMeasurement is a set of body circumference measurements on a given date.
type BodyMeasurement struct {
	ID          string  `json:"id"`
	UserID      string  `json:"userId"`
	Date        string  `json:"date"`
	NeckCm      float64 `json:"neckCm"`
	ChestCm     float64 `json:"chestCm"`
	WaistCm     float64 `json:"waistCm"`
	HipsCm      float64 `json:"hipsCm"`
	ThighCm     float64 `json:"thighCm"`
	BicepCm     float64 `json:"bicepCm"`
	ShouldersCm float64 `json:"shouldersCm"`
	CalvesCm    float64 `json:"calvesCm"`
	Notes       string  `json:"notes"`
	CreatedAt   string  `json:"createdAt"`
}

// --- Computed/derived types ---

type TDEEResult struct {
	EstimatedTDEE  float64 `json:"estimatedTdee"`
	ObservedTDEE   float64 `json:"observedTdee"`
	Confidence     string  `json:"confidence"`
	DaysOfData     int     `json:"daysOfData"`
	LookbackDays   int     `json:"lookbackDays"`
	Method         string  `json:"method"`
	EmergencyAlert bool    `json:"emergencyAlert"`
}

type MacroResult struct {
	Calories  float64 `json:"calories"`
	ProteinG  float64 `json:"proteinG"`
	CarbsG    float64 `json:"carbsG"`
	FatG      float64 `json:"fatG"`
	FiberG    float64 `json:"fiberG"`
	WaterMl   float64 `json:"waterMl"`
	GoalLabel string  `json:"goalLabel"`
}

type ReadinessResult struct {
	// Primary signal — what the UI should use
	Level   string  `json:"level"`   // "green", "yellow", "red"
	Message string  `json:"message"` // contextual coaching text e.g. "Ready", "Mildly fatigued"
	Rz      float64 `json:"rz"`      // combined weighted z-score (context-relative, not 0-100)

	// Velocity
	VelocityTrend string  `json:"velocityTrend"` // "improving", "stable", "declining"
	VelocityDelta float64 `json:"velocityDelta"` // Rz delta between current and prior 7-day avg

	// Sub-scores for detail views
	GripZ float64 `json:"gripZ"`
	BoltZ float64 `json:"boltZ"`

	// Notes / warnings
	Notes []string `json:"notes"`

	// Legacy compat fields (derived, kept so any existing callers don't break)
	Score    float64 `json:"score"`    // 50 + (Rz/3)*50 clamped 0-100
	Category string  `json:"category"` // mirrors Level: "low"/"moderate"/"high"
}

type BodyFatResult struct {
	Method     string  `json:"method"`
	BfPct      float64 `json:"bfPct"`
	LeanMassKg float64 `json:"leanMassKg"`
	FatMassKg  float64 `json:"fatMassKg"`
}

type TodaySummary struct {
	Date         string       `json:"date"`
	Consumed     NutritionLog `json:"consumed"`
	Targets      Targets      `json:"targets"`
	CaloriesLeft float64      `json:"caloriesLeft"`
	ProteinLeft  float64      `json:"proteinLeft"`
	ProgressPct  float64      `json:"progressPct"`
}

type WeeklyStats struct {
	AvgCalories   float64 `json:"avgCalories"`
	AvgProteinG   float64 `json:"avgProteinG"`
	TotalWorkouts int     `json:"totalWorkouts"`
	TotalMWV      float64 `json:"totalMwv"`
	AvgSleepHours float64 `json:"avgSleepHours"`
	AvgWeightKg   float64 `json:"avgWeightKg"`
	DailyLogged   []int   `json:"dailyLogged"`   // 30-day array: 1 if any data logged, 0 if not
	CurrentStreak int     `json:"currentStreak"` // Consecutive days logged ending today
	LongestStreak int     `json:"longestStreak"` // Longest consecutive logging streak in 30 days
	TodayLogged   bool    `json:"todayLogged"`   // Whether today has any logged data (for fire indicator)
}

type DashboardData struct {
	Today       TodaySummary    `json:"today"`
	TDEE        TDEEResult      `json:"tdee"`
	Macros      MacroResult     `json:"macros"`
	Readiness   ReadinessResult `json:"readiness"`
	WeeklyStats WeeklyStats     `json:"weeklyStats"`
	WeightTrend []struct {
		Date     string  `json:"date"`
		WeightKg float64 `json:"weightKg"`
	} `json:"weightTrend"`
	TodayBio         *BiometricLog `json:"todayBio"`
	GripPersonalBest float64       `json:"gripPersonalBest"`
	WorkoutToday     bool          `json:"workoutToday"`
	// Check-in readiness
	CanChangeTargets bool `json:"canChangeTargets"`
	DaysUntilCheckin int  `json:"daysUntilCheckin"`
}

type ParsedMeal struct {
	Calories  float64 `json:"calories"`
	ProteinG  float64 `json:"proteinG"`
	CarbsG    float64 `json:"carbsG"`
	FatG      float64 `json:"fatG"`
	FiberG    float64 `json:"fiberG"`
	WaterMl   float64 `json:"waterMl"`
	MealNotes string  `json:"mealNotes"`
	RawInput  string  `json:"rawInput"`
}

// ParsedWorkout is returned by POST /v1/parse/workout (YAML or AI).
// It is a superset of WorkoutEntry — includes all session metadata.
type ParsedWorkout struct {
	// Core fields (map directly to WorkoutEntry)
	Title          string          `json:"title"`
	Slot           string          `json:"slot"`
	DurationMin    float64         `json:"durationMin"`
	CaloriesBurned float64         `json:"caloriesBurned"`
	Exercises      []ExerciseEntry `json:"exercises"`
	RawInput       string          `json:"rawInput"`
	Notes          string          `json:"notes,omitempty"`
	// Session metadata (maps to WorkoutMetadata)
	Type         string   `json:"type"`
	Style        string   `json:"style"`
	Surface      string   `json:"surface"`
	Focus        []string `json:"focus"`
	RestInterval string   `json:"restInterval"`
	RPE          float64  `json:"rpe"`
	AvgHR        int      `json:"avgHr"`
	MaxHR        int      `json:"maxHr"`
	Recovers     string   `json:"recovers"`
	Day          int      `json:"day"`
}
