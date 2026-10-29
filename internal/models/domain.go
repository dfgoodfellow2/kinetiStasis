package models

import "time"

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
	UserID            string  `json:"userId"`
	Name              string  `json:"name"`
	Age               int     `json:"age"`
	Sex               string  `json:"sex"`
	HeightCm          float64 `json:"heightCm"`
	Activity          string  `json:"activity"`
	ExerciseFreq      int     `json:"exerciseFreq"`
	RunningKm         float64 `json:"runningKm"`
	IsLifter          bool    `json:"isLifter"`
	Goal              string  `json:"goal"`
	PrioritizeCarbs   bool    `json:"prioritizeCarbs"`
	BfPct             float64 `json:"bfPct"`
	HRRest            int     `json:"hrRest"`
	HRMax             int     `json:"hrMax"`
	GripWeight        float64 `json:"gripWeight"`
	TDEELookbackDays  int     `json:"tdeeLookbackDays"`
	SleepQualityMax   float64 `json:"sleepQualityMax"`
	Units             string  `json:"units"`
	UpdatedAt         string  `json:"updatedAt"`
	Birthdate         string  `json:"birthdate"`         // "YYYY-MM-DD" — derive age from this
	MacroDistribution string  `json:"macroDistribution"` // "keto", "low_carb", "balanced", "high_carb"
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
	// For backwards compatibility with older DB rows that used snake_case
	// (duration_raw) when storing exercises_json. When decoding from the
	// DB we capture that value here and copy it into DurationRaw if the
	// camelCase field is empty.
	DurationRawLegacy string `json:"duration_raw,omitempty"`
	// Tempo preserves the original tempo string (e.g. "2-0-2-0") for form display
	Tempo string `json:"tempo,omitempty"`
	// Bias is the bilateral/unilateral indicator for this exercise
	Bias string `json:"bias,omitempty"` // "bilateral" | "unilateral" | ""
	// IntensityRelMax is the estimated % of 1RM (0.0-1.0)
	IntensityRelMax float64 `json:"intensityRelMax,omitempty"`
	// IntensitySource indicates where intensity was derived from: "1rm", "rpe", "hr", or "default"
	IntensitySource string `json:"intensitySource,omitempty"`

	// Phase indicates session phase: "warmup" | "main" | "cooldown" | "" (default main)
	Phase string `json:"phase,omitempty"`

	// GroupID links exercises in a circuit/superset/cluster.
	// Exercises with the same GroupID belong to the same group.
	GroupID string `json:"groupId,omitempty"`

	// GroupOrder is the 0-based position within the group (circuit rotation order).
	GroupOrder int `json:"groupOrder,omitempty"`

	// GroupStyle overrides the workout-level style for this group.
	// "circuit" | "superset" | "cluster" | "straight" | "" (inherits from workout)
	GroupStyle string `json:"groupStyle,omitempty"`

	// TargetHRZone is the intended heart rate zone (1-5) for prescriptive tracking.
	TargetHRZone int `json:"targetHrZone,omitempty"`

	// HRZoneTime stores actual seconds spent in each HR zone, keyed by zone number.
	// e.g. {"1": 60, "2": 720, "3": 180}
	HRZoneTime map[int]float64 `json:"hrZoneTime,omitempty"`
}

// WorkoutMetadata holds session-level fields not tracked in individual columns.
type WorkoutMetadata struct {
	Type         string   `json:"type"` // strength|conditioning|hiit|cardio|zone2|mobility|sport|yoga
	Surface      string   `json:"surface"`
	Focus        []string `json:"focus"` // e.g. ["Hinge(B)", "Push(U)"]
	RestInterval string   `json:"restInterval"`
	RPE          float64  `json:"rpe"`
	AvgHR        int      `json:"avgHr"`
	MaxHR        int      `json:"maxHr"`
	Recovers     string   `json:"recovers"`
	Day          int      `json:"day"`
	// CoachNotes is coach's feedback/context for the session
	CoachNotes string `json:"coachNotes,omitempty"`
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
	ID              string  `json:"id"`
	UserID          string  `json:"userId"`
	EffectiveDate   string  `json:"effectiveDate"`
	Calories        float64 `json:"calories"`
	ProteinG        float64 `json:"proteinG"`
	CarbsG          float64 `json:"carbsG"`
	FatG            float64 `json:"fatG"`
	FiberG          float64 `json:"fiberG"`
	WaterMl         float64 `json:"waterMl"`
	EatBackExercise bool    `json:"eatBackExercise"`
	CreatedAt       string  `json:"createdAt"`
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

	// Adaptive TDEE snapshot captured at check-in time. These are persisted
	// exactly as returned by the adaptive engine for that run.
	ObservedTDEE   float64 `json:"observedTdee"`
	TDEEConfidence string  `json:"tdeeConfidence"`
	LookbackDays   int     `json:"lookbackDays"`
}

// CheckinHistoryEntry is the lean response shape for GET /v1/checkin/history.
type CheckinHistoryEntry struct {
	CheckInDate       string  `json:"checkInDate"`
	ObservedTDEE      float64 `json:"observedTdee"`
	TDEEConfidence    string  `json:"tdeeConfidence"`
	LookbackDays      int     `json:"lookbackDays"`
	ResultingCalories int     `json:"resultingCalories"`
}

// CheckinPreview is returned by GET /v1/checkin
type CheckinPreview struct {
	// Readiness gating
	CanCheckIn           bool `json:"canCheckIn"`
	DaysSinceLastCheckIn int  `json:"daysSinceLastCheckIn"`

	// Adaptive TDEE result
	ObservedTDEE         float64 `json:"observedTdee"`
	EstimatedTDEE        float64 `json:"estimatedTdee"`
	Confidence           string  `json:"confidence"`
	TDEEMethod           string  `json:"tdeeMethod"`
	DaysOfData           int     `json:"daysOfData"`
	LowConfidenceWarning bool    `json:"lowConfidenceWarning"`
	// TDEEConfidence and LookbackDays mirror the engine result under the
	// names used by the check-in history API.
	TDEEConfidence string `json:"tdeeConfidence"`
	LookbackDays   int    `json:"lookbackDays"`

	// Weight trend (display context only)
	WeightStart  float64 `json:"weightStart"`
	WeightEnd    float64 `json:"weightEnd"`
	WeightChange float64 `json:"weightChange"`

	// Current nutritional state
	CurrentCalories float64 `json:"currentCalories"`
	EatBackExercise bool    `json:"eatBackExercise"`

	// Recommendation from adaptive engine
	RecommendedCalories int     `json:"recommendedCalories"`
	CalorieAdjustment   float64 `json:"calorieAdjustment"`
}

// SavedMeal is a named, reusable meal with its macro breakdown.
type SavedMeal struct {
	ID       string  `json:"id"`
	UserID   string  `json:"userId"`
	Name     string  `json:"name"`
	Calories float64 `json:"calories"`
	ProteinG float64 `json:"proteinG"`
	CarbsG   float64 `json:"carbsG"`
	FatG     float64 `json:"fatG"`
	FiberG   float64 `json:"fiberG"`
	// Notes is the meal description captured when the meal was saved. Applying
	// a saved meal restores it alongside the macros.
	Notes     string `json:"notes"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// MealTemplate is a named collection of meal items.
type MealTemplate struct {
	ID     string      `json:"id"`
	UserID string      `json:"userId"`
	Name   string      `json:"name"`
	Meals  []SavedMeal `json:"meals"`
	// GroupID is empty for a private template. When set, members of that
	// group can see and apply the template.
	GroupID string `json:"groupId,omitempty"`
	// SharedBy is populated by list queries for templates owned by another
	// member so the client can render a provenance badge.
	SharedBy  string `json:"sharedBy,omitempty"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// Group is a private cohort whose members may share logging signals and meal
// templates. MyRole and MemberCount are populated by list/get queries.
type Group struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	CreatedBy   string `json:"createdBy"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
	MyRole      string `json:"myRole,omitempty"`
	MemberCount int    `json:"memberCount,omitempty"`
}

// GroupMember is a stored membership row, including the owner-controlled
// sharing toggles that gate which signals appear in the group summary.
type GroupMember struct {
	GroupID        string `json:"groupId"`
	UserID         string `json:"userId"`
	Role           string `json:"role"`
	ShareClock     bool   `json:"shareClock"`
	ShareStreak    bool   `json:"shareStreak"`
	ShareAdherence bool   `json:"shareAdherence"`
	JoinedAt       string `json:"joinedAt"`
}

// GroupInvite is a single-use join code for a group.
type GroupInvite struct {
	Code      string `json:"code"`
	GroupID   string `json:"groupId"`
	CreatedBy string `json:"createdBy"`
	ExpiresAt string `json:"expiresAt"`
	UsedBy    string `json:"usedBy,omitempty"`
	CreatedAt string `json:"createdAt"`
}

// SharedFlags mirrors a member's sharing toggles for the summary response.
type SharedFlags struct {
	Clock     bool `json:"clock"`
	Streak    bool `json:"streak"`
	Adherence bool `json:"adherence"`
}

// PresenceFlags reports whether a member logged anything today. It is only
// included when the member shares their clock.
type PresenceFlags struct {
	LoggedFood    bool `json:"loggedFood"`
	LoggedWeight  bool `json:"loggedWeight"`
	LoggedWorkout bool `json:"loggedWorkout"`
}

// MemberCard is the lean, privacy-safe shape returned by a group summary.
// User IDs and emails are deliberately absent; display names come from the
// profile and fall back to a generic label. Pointer fields are nil (and are
// omitted from JSON) when the member has not shared the corresponding signal.
type MemberCard struct {
	Name         string         `json:"name"`
	Role         string         `json:"role"`
	Shared       *SharedFlags   `json:"shared,omitempty"`
	Presence     *PresenceFlags `json:"presence,omitempty"`
	Streak       *int           `json:"streak,omitempty"`
	AdherencePct *float64       `json:"adherencePct,omitempty"`
}

// GroupMemberInfo is the owner-only member roster entry. Unlike MemberCard it
// carries UserID so the owner can target the remove endpoint; it is only ever
// served to the group owner.
type GroupMemberInfo struct {
	UserID   string `json:"userId"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	JoinedAt string `json:"joinedAt,omitempty"`
}

// GroupSummary is the response for GET /v1/groups/{id}/summary.
type GroupSummary struct {
	GroupID   string       `json:"groupId"`
	GroupName string       `json:"groupName"`
	MyRole    string       `json:"myRole"`
	Members   []MemberCard `json:"members"`
	// MySharing is the requesting member's own toggles so the client can seed
	// its switch controls without a second round-trip.
	MySharing *SharedFlags `json:"mySharing,omitempty"`
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

	// Standard deviation of the 14-day grip values used for z-score calculation
	GripSD float64 `json:"gripSD"`
	// Exponential moving average of the 14-day grip values
	GripEMA float64 `json:"gripEMA"`
	// Standard deviation of the 14-day BOLT values used for z-score calculation
	BoltSD float64 `json:"boltSD"`
	// Exponential moving average of the 14-day BOLT values
	BoltEMA float64 `json:"boltEMA"`

	// Notes / warnings
	Notes []string `json:"notes"`
	// ConsecutiveDays tracks how many consecutive days the current alert level has been active (0 if green)
	ConsecutiveDays int `json:"consecutiveDays"`

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

	// Weight trend derived from the adaptive engine's linear regression.
	// WeightTrendKg is 0 when there are fewer than seven weight observations.
	WeightTrendKg       float64 `json:"weightTrendKg"`
	WeightRateKgPerWeek float64 `json:"weightRateKgPerWeek"`
	TrendDays           int     `json:"trendDays"`

	// 60-day nutrition logging calendar and streaks.
	LoggingStreak int    `json:"loggingStreak"`
	LongestStreak int    `json:"longestStreak"`
	LoggedDays    []bool `json:"loggedDays"`

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
	// CoachNotes is coach's feedback/context for the session
	CoachNotes string `json:"coachNotes,omitempty"`
	// Session metadata (maps to WorkoutMetadata)
	Type         string   `json:"type"`
	Surface      string   `json:"surface"`
	Focus        []string `json:"focus"`
	RestInterval string   `json:"restInterval"`
	RPE          float64  `json:"rpe"`
	AvgHR        int      `json:"avgHr"`
	MaxHR        int      `json:"maxHr"`
	Recovers     string   `json:"recovers"`
	Day          int      `json:"day"`
}

// AgeFromBirthdate derives age from a birthdate string ("YYYY-MM-DD").
// Returns 0 if birthdate is empty or unparseable, or if it is in the future.
func AgeFromBirthdate(birthdate string) int {
	return ageFromBirthdateAt(birthdate, time.Now())
}

// ageFromBirthdateAt is the testable core of AgeFromBirthdate. It derives age
// relative to the supplied now, returning 0 for empty/unparseable/future dates.
func ageFromBirthdateAt(birthdate string, now time.Time) int {
	if birthdate == "" {
		return 0
	}
	bd, err := time.Parse("2006-01-02", birthdate)
	if err != nil {
		return 0
	}
	// Future birthdates are nonsensical; report 0 rather than a negative age.
	if bd.After(now) {
		return 0
	}
	age := now.Year() - bd.Year()
	// Compare month/day directly (YearDay is unreliable across leap years)
	if now.Month() < bd.Month() || (now.Month() == bd.Month() && now.Day() < bd.Day()) {
		age--
	}
	return age
}
