package models

// Domain models extracted from client package

type NutritionLog struct {
	ID        string  `json:"id"`
	Date      string  `json:"date"`
	Calories  float64 `json:"calories"`
	ProteinG  float64 `json:"proteinG"`
	CarbsG    float64 `json:"carbsG"`
	FatG      float64 `json:"fatG"`
	FiberG    float64 `json:"fiberG"`
	WaterMl   float64 `json:"waterMl"`
	MealNotes string  `json:"mealNotes"`
}

type BiometricLog struct {
	Date           string  `json:"date"`
	WeightKg       float64 `json:"weightKg"`
	WaistCm        float64 `json:"waistCm"`
	GripKg         float64 `json:"gripKg"`
	BoltScore      float64 `json:"boltScore"`
	SleepHours     float64 `json:"sleepHours"`
	SleepQuality   float64 `json:"sleepQuality"`
	SubjectiveFeel int     `json:"subjectiveFeel"`
	BodyFatPct     float64 `json:"bodyFatPct"`
	Notes          string  `json:"notes"`
}

type ExerciseSet struct {
	Reps        int     `json:"reps"`
	LoadKg      float64 `json:"loadKg"`
	TUTSeconds  float64 `json:"tutSeconds"`
	RestSeconds float64 `json:"restSeconds"`
}

type ExerciseEntry struct {
	Name        string        `json:"name"`
	Category    string        `json:"category"`
	Sets        []ExerciseSet `json:"sets"`
	Notes       string        `json:"notes"`
	DistanceKm  float64       `json:"distanceKm,omitempty"`
	ElevationM  float64       `json:"elevationM,omitempty"`
	Pace        string        `json:"pace,omitempty"`
	RPE         float64       `json:"rpe,omitempty"`
	LoadRaw     string        `json:"loadRaw,omitempty"`
	DurationRaw string        `json:"durationRaw,omitempty"`
	Tempo       string        `json:"tempo,omitempty"`
}

type WorkoutMetadata struct {
	Type    string   `json:"type"`
	Style   string   `json:"style"`
	Surface string   `json:"surface"`
	Focus   []string `json:"focus"`
	RPE     float64  `json:"rpe"`
	AvgHR   int      `json:"avgHr"`
	MaxHR   int      `json:"maxHr"`
}

type WorkoutEntry struct {
	ID             string          `json:"id"`
	Date           string          `json:"date"`
	Slot           string          `json:"slot"`
	Title          string          `json:"title"`
	DurationMin    float64         `json:"durationMin"`
	CaloriesBurned float64         `json:"caloriesBurned"`
	MWV            float64         `json:"mwv"`
	NDS            float64         `json:"nds"`
	SessionDensity float64         `json:"sessionDensity"`
	Exercises      []ExerciseEntry `json:"exercises"`
	Metadata       WorkoutMetadata `json:"metadata"`
	RawNotes       string          `json:"rawNotes"`
}

type Profile struct {
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
}

type Targets struct {
	Calories        float64 `json:"calories"`
	ProteinG        float64 `json:"proteinG"`
	CarbsG          float64 `json:"carbsG"`
	FatG            float64 `json:"fatG"`
	FiberG          float64 `json:"fiberG"`
	WaterMl         float64 `json:"waterMl"`
	EatBackExercise bool    `json:"eatBackExercise"`
}

type BodyMeasurement struct {
	Date    string  `json:"date"`
	NeckCm  float64 `json:"neckCm"`
	ChestCm float64 `json:"chestCm"`
	WaistCm float64 `json:"waistCm"`
	HipsCm  float64 `json:"hipsCm"`
	ThighCm float64 `json:"thighCm"`
	BicepCm float64 `json:"bicepCm"`
	Notes   string  `json:"notes"`
}

type ParsedMeal struct {
	Calories  float64 `json:"calories"`
	ProteinG  float64 `json:"proteinG"`
	CarbsG    float64 `json:"carbsG"`
	FatG      float64 `json:"fatG"`
	FiberG    float64 `json:"fiberG"`
	WaterMl   float64 `json:"waterMl"`
	MealNotes string  `json:"mealNotes"`
}

type ParsedWorkout struct {
	Title          string          `json:"title"`
	Slot           string          `json:"slot"`
	DurationMin    float64         `json:"durationMin"`
	CaloriesBurned float64         `json:"caloriesBurned"`
	Exercises      []ExerciseEntry `json:"exercises"`
	Notes          string          `json:"notes"`
	Type           string          `json:"type"`
	Style          string          `json:"style"`
	RPE            float64         `json:"rpe"`
}

type BodyFatResult struct {
	Method     string  `json:"method"`
	BfPct      float64 `json:"bfPct"`
	LeanMassKg float64 `json:"leanMassKg"`
	FatMassKg  float64 `json:"fatMassKg"`
}

type TDEEResult struct {
	EstimatedTDEE float64 `json:"estimatedTdee"`
	ObservedTDEE  float64 `json:"observedTdee"`
	Confidence    string  `json:"confidence"`
	DaysOfData    int     `json:"daysOfData"`
	Method        string  `json:"method"`
}

type ReadinessResult struct {
	Level         string   `json:"level"`
	Message       string   `json:"message"`
	Score         float64  `json:"score"`
	VelocityTrend string   `json:"velocityTrend"`
	VelocityDelta float64  `json:"velocityDelta"`
	GripZ         float64  `json:"gripZ"`
	BoltZ         float64  `json:"boltZ"`
	Notes         []string `json:"notes"`
}

type WeeklyStats struct {
	AvgCalories   float64 `json:"avgCalories"`
	AvgProteinG   float64 `json:"avgProteinG"`
	TotalWorkouts int     `json:"totalWorkouts"`
	TotalMWV      float64 `json:"totalMwv"`
	AvgSleepHours float64 `json:"avgSleepHours"`
	AvgWeightKg   float64 `json:"avgWeightKg"`
}

type TodaySummary struct {
	Date         string       `json:"date"`
	Consumed     NutritionLog `json:"consumed"`
	Targets      Targets      `json:"targets"`
	CaloriesLeft float64      `json:"caloriesLeft"`
	ProteinLeft  float64      `json:"proteinLeft"`
	ProgressPct  float64      `json:"progressPct"`
}

type WeightPoint struct {
	Date     string  `json:"date"`
	WeightKg float64 `json:"weightKg"`
}

type DashboardData struct {
	Today        TodaySummary    `json:"today"`
	TDEE         TDEEResult      `json:"tdee"`
	Readiness    ReadinessResult `json:"readiness"`
	WeeklyStats  WeeklyStats     `json:"weeklyStats"`
	WeightTrend  []WeightPoint   `json:"weightTrend"`
	TodayBio     *BiometricLog   `json:"todayBio"`
	WorkoutToday bool            `json:"workoutToday"`
}
