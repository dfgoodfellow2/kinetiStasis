package cli

// The structs below mirror the JSON shapes documented in internal/models. They
// are kept local so the CLI's request bodies contain only the fields the write
// whitelist needs (no invented or stale fields).

type parsedWorkout struct {
	Title          string     `json:"title"`
	Slot           string     `json:"slot"`
	DurationMin    float64    `json:"durationMin"`
	CaloriesBurned float64    `json:"caloriesBurned"`
	Exercises      []exercise `json:"exercises"`
	Notes          string     `json:"notes,omitempty"`
	CoachNotes     string     `json:"coachNotes,omitempty"`
	Type           string     `json:"type"`
	Surface        string     `json:"surface"`
	Focus          []string   `json:"focus"`
	RestInterval   string     `json:"restInterval"`
	RPE            float64    `json:"rpe"`
	AvgHR          int        `json:"avgHr"`
	MaxHR          int        `json:"maxHr"`
	Recovers       string     `json:"recovers"`
	Day            int        `json:"day"`
}

type exercise struct {
	Name            string          `json:"name"`
	Category        string          `json:"category,omitempty"`
	Sets            []exerciseSet   `json:"sets"`
	METValue        float64         `json:"metValue,omitempty"`
	Surface         string          `json:"surface,omitempty"`
	Notes           string          `json:"notes,omitempty"`
	DistanceKm      float64         `json:"distanceKm,omitempty"`
	ElevationM      float64         `json:"elevationM,omitempty"`
	Pace            string          `json:"pace,omitempty"`
	RPE             float64         `json:"rpe,omitempty"`
	LoadRaw         string          `json:"loadRaw,omitempty"`
	DurationRaw     string          `json:"durationRaw,omitempty"`
	Tempo           string          `json:"tempo,omitempty"`
	Bias            string          `json:"bias,omitempty"`
	IntensityRelMax float64         `json:"intensityRelMax,omitempty"`
	IntensitySource string          `json:"intensitySource,omitempty"`
	Phase           string          `json:"phase,omitempty"`
	GroupID         string          `json:"groupId,omitempty"`
	GroupOrder      int             `json:"groupOrder,omitempty"`
	GroupStyle      string          `json:"groupStyle,omitempty"`
	TargetHRZone    int             `json:"targetHrZone,omitempty"`
	HRZoneTime      map[int]float64 `json:"hrZoneTime,omitempty"`
}

type exerciseSet struct {
	Reps        int     `json:"reps"`
	LoadKg      float64 `json:"loadKg"`
	LoadLbs     float64 `json:"loadLbs"`
	TUTSeconds  float64 `json:"tutSeconds"`
	RestSeconds float64 `json:"restSeconds"`
}

type workoutMetadata struct {
	Type         string   `json:"type"`
	Surface      string   `json:"surface"`
	Focus        []string `json:"focus"`
	RestInterval string   `json:"restInterval"`
	RPE          float64  `json:"rpe"`
	AvgHR        int      `json:"avgHr"`
	MaxHR        int      `json:"maxHr"`
	Recovers     string   `json:"recovers"`
	Day          int      `json:"day"`
	CoachNotes   string   `json:"coachNotes,omitempty"`
}

type workoutEntry struct {
	Date           string          `json:"date"`
	Slot           string          `json:"slot"`
	Title          string          `json:"title"`
	RawNotes       string          `json:"rawNotes"`
	DurationMin    float64         `json:"durationMin"`
	CaloriesBurned float64         `json:"caloriesBurned"`
	Exercises      []exercise      `json:"exercises"`
	Metadata       workoutMetadata `json:"metadata"`
}

// mapParsedWorkout projects the parser's ParsedWorkout onto the WorkoutEntry
// body accepted by POST /v1/workouts.
func mapParsedWorkout(parsed parsedWorkout, date string) workoutEntry {
	slot := parsed.Slot
	if slot == "" {
		slot = "1"
	}

	exercises := make([]exercise, 0, len(parsed.Exercises))
	for _, src := range parsed.Exercises {
		ex := src
		if ex.Sets == nil {
			ex.Sets = []exerciseSet{}
		}
		exercises = append(exercises, ex)
	}

	return workoutEntry{
		Date:           date,
		Slot:           slot,
		Title:          parsed.Title,
		RawNotes:       parsed.Notes,
		DurationMin:    parsed.DurationMin,
		CaloriesBurned: parsed.CaloriesBurned,
		Exercises:      exercises,
		Metadata: workoutMetadata{
			Type:         parsed.Type,
			Surface:      parsed.Surface,
			Focus:        parsed.Focus,
			RestInterval: parsed.RestInterval,
			RPE:          parsed.RPE,
			AvgHR:        parsed.AvgHR,
			MaxHR:        parsed.MaxHR,
			Recovers:     parsed.Recovers,
			Day:          parsed.Day,
			CoachNotes:   parsed.CoachNotes,
		},
	}
}
