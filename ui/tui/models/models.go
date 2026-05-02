package models

// Domain models extracted from client package

type NutritionLog struct {
	ID        string  `json:"id"`
	Date      string  `json:"date"`
	Calories  float64 `json:"calories"`
	ProteinG  float64 `json:"protein_g"`
	CarbsG    float64 `json:"carbs_g"`
	FatG      float64 `json:"fat_g"`
	FiberG    float64 `json:"fiber_g"`
	WaterMl   float64 `json:"water_ml"`
	MealNotes string  `json:"meal_notes"`
}

type BiometricLog struct {
	Date           string  `json:"date"`
	WeightKg       float64 `json:"weight_kg"`
	WaistCm        float64 `json:"waist_cm"`
	GripKg         float64 `json:"grip_kg"`
	BoltScore      float64 `json:"bolt_score"`
	SleepHours     float64 `json:"sleep_hours"`
	SleepQuality   float64 `json:"sleep_quality"`
	SubjectiveFeel int     `json:"subjective_feel"`
	BodyFatPct     float64 `json:"body_fat_pct"`
	Notes          string  `json:"notes"`
}

type ExerciseSet struct {
	Reps        int     `json:"reps"`
	LoadKg      float64 `json:"load_kg"`
	TUTSeconds  float64 `json:"tut_seconds"`
	RestSeconds float64 `json:"rest_seconds"`
}

type ExerciseEntry struct {
	Name        string        `json:"name"`
	Category    string        `json:"category"`
	Sets        []ExerciseSet `json:"sets"`
	Notes       string        `json:"notes"`
	DistanceKm  float64       `json:"distance_km,omitempty"`
	ElevationM  float64       `json:"elevation_m,omitempty"`
	Pace        string        `json:"pace,omitempty"`
	RPE         float64       `json:"rpe,omitempty"`
	LoadRaw     string        `json:"load_raw,omitempty"`
	DurationRaw string        `json:"duration_raw,omitempty"`
	Tempo       string        `json:"tempo,omitempty"`
}

type WorkoutMetadata struct {
	Type    string   `json:"type"`
	Style   string   `json:"style"`
	Surface string   `json:"surface"`
	Focus   []string `json:"focus"`
	RPE     float64  `json:"rpe"`
	AvgHR   int      `json:"avg_hr"`
	MaxHR   int      `json:"max_hr"`
}

type WorkoutEntry struct {
	ID             string          `json:"id"`
	Date           string          `json:"date"`
	Slot           string          `json:"slot"`
	Title          string          `json:"title"`
	DurationMin    float64         `json:"duration_min"`
	CaloriesBurned float64         `json:"calories_burned"`
	MWV            float64         `json:"mwv"`
	NDS            float64         `json:"nds"`
	SessionDensity float64         `json:"session_density"`
	Exercises      []ExerciseEntry `json:"exercises"`
	Metadata       WorkoutMetadata `json:"metadata"`
	RawNotes       string          `json:"raw_notes"`
}

type Profile struct {
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
}

type Targets struct {
	Calories        float64 `json:"calories"`
	ProteinG        float64 `json:"protein_g"`
	CarbsG          float64 `json:"carbs_g"`
	FatG            float64 `json:"fat_g"`
	FiberG          float64 `json:"fiber_g"`
	WaterMl         float64 `json:"water_ml"`
	EatBackExercise bool    `json:"eat_back_exercise"`
}

type BodyMeasurement struct {
	Date    string  `json:"date"`
	NeckCm  float64 `json:"neck_cm"`
	ChestCm float64 `json:"chest_cm"`
	WaistCm float64 `json:"waist_cm"`
	HipsCm  float64 `json:"hips_cm"`
	ThighCm float64 `json:"thigh_cm"`
	BicepCm float64 `json:"bicep_cm"`
	Notes   string  `json:"notes"`
}

type ParsedMeal struct {
	Calories  float64 `json:"calories"`
	ProteinG  float64 `json:"protein_g"`
	CarbsG    float64 `json:"carbs_g"`
	FatG      float64 `json:"fat_g"`
	FiberG    float64 `json:"fiber_g"`
	WaterMl   float64 `json:"water_ml"`
	MealNotes string  `json:"meal_notes"`
}

type ParsedWorkout struct {
	Title          string          `json:"title"`
	Slot           string          `json:"slot"`
	DurationMin    float64         `json:"duration_min"`
	CaloriesBurned float64         `json:"calories_burned"`
	Exercises      []ExerciseEntry `json:"exercises"`
	Notes          string          `json:"notes"`
	Type           string          `json:"type"`
	Style          string          `json:"style"`
	RPE            float64         `json:"rpe"`
}

type BodyFatResult struct {
	Method     string  `json:"method"`
	BfPct      float64 `json:"bf_pct"`
	LeanMassKg float64 `json:"lean_mass_kg"`
	FatMassKg  float64 `json:"fat_mass_kg"`
}

type TDEEResult struct {
	EstimatedTDEE float64 `json:"estimated_tdee"`
	ObservedTDEE  float64 `json:"observed_tdee"`
	Confidence    string  `json:"confidence"`
	DaysOfData    int     `json:"days_of_data"`
	Method        string  `json:"method"`
}

type ReadinessResult struct {
	Level         string   `json:"level"`
	Message       string   `json:"message"`
	Score         float64  `json:"score"`
	VelocityTrend string   `json:"velocity_trend"`
	VelocityDelta float64  `json:"velocity_delta"`
	GripZ         float64  `json:"grip_z"`
	BoltZ         float64  `json:"bolt_z"`
	Notes         []string `json:"notes"`
}

type WeeklyStats struct {
	AvgCalories   float64 `json:"avg_calories"`
	AvgProteinG   float64 `json:"avg_protein_g"`
	TotalWorkouts int     `json:"total_workouts"`
	TotalMWV      float64 `json:"total_mwv"`
	AvgSleepHours float64 `json:"avg_sleep_hours"`
	AvgWeightKg   float64 `json:"avg_weight_kg"`
}

type TodaySummary struct {
	Date         string       `json:"date"`
	Consumed     NutritionLog `json:"consumed"`
	Targets      Targets      `json:"targets"`
	CaloriesLeft float64      `json:"calories_left"`
	ProteinLeft  float64      `json:"protein_left"`
	ProgressPct  float64      `json:"progress_pct"`
}

type WeightPoint struct {
	Date     string  `json:"date"`
	WeightKg float64 `json:"weight_kg"`
}

type DashboardData struct {
	Today        TodaySummary    `json:"today"`
	TDEE         TDEEResult      `json:"tdee"`
	Readiness    ReadinessResult `json:"readiness"`
	WeeklyStats  WeeklyStats     `json:"weekly_stats"`
	WeightTrend  []WeightPoint   `json:"weight_trend"`
	TodayBio     *BiometricLog   `json:"today_bio"`
	WorkoutToday bool            `json:"workout_today"`
}
