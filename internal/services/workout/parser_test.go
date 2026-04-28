package workout

import (
	"testing"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
)

func TestParseYAML(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		validate func(t *testing.T, out models.ParsedWorkout, err error)
	}{
		{
			name: "Empty input",
			in:   "",
			validate: func(t *testing.T, out models.ParsedWorkout, err error) {
				if err == nil {
					t.Fatalf("expected error for empty input")
				}
			},
		},
		{
			name: "Minimal workout",
			in: `title: Morning Lift
exercises:
  - name: Back Squat
    sets: 3
    reps: 5
    load: 100 lbs
    pattern: squat B
`,
			validate: func(t *testing.T, out models.ParsedWorkout, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if out.Title != "Morning Lift" {
					t.Errorf("Title: got %q want %q", out.Title, "Morning Lift")
				}
				if len(out.Exercises) != 1 {
					t.Fatalf("expected 1 exercise, got %d", len(out.Exercises))
				}
				ex := out.Exercises[0]
				if ex.Name != "Back Squat" {
					t.Errorf("exercise name: got %q want %q", ex.Name, "Back Squat")
				}
				if ex.Category != "squat" {
					t.Errorf("category: got %q want %q", ex.Category, "squat")
				}
				if ex.Bias != "bilateral" {
					t.Errorf("bias: got %q want %q", ex.Bias, "bilateral")
				}
				if len(ex.Sets) != 3 {
					t.Fatalf("sets length: got %d want %d", len(ex.Sets), 3)
				}
				for i, s := range ex.Sets {
					if s.Reps != 5 {
						t.Errorf("set %d reps: got %d want %d", i, s.Reps, 5)
					}
					if s.LoadLbs != 100 {
						t.Errorf("set %d load: got %v want %v", i, s.LoadLbs, 100)
					}
				}
			},
		},
		{
			name: "Full session metadata",
			in: `name: Push Day
type: strength
slot: "1"
style: circuit
surface: gym
rest_interval: "90s"
duration: 60
rpe: 7.5
avg_hr: 142
max_hr: 175
calories_burned: 480
recovers: lower
day: 3
focus:
  - Push(U)
  - Core
notes: Felt strong today
exercises:
  - name: Bench Press
    sets: 4
    reps: 8
    load: 135 lbs
    pattern: push U
    tempo: 2-0-1-1
  - name: Sled Push
    sets: 3
    duration: 35 sec
    met: 8.5
    distance_km: 0.1
`,
			validate: func(t *testing.T, out models.ParsedWorkout, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if out.Title != "Push Day" {
					t.Errorf("Title: got %q want %q", out.Title, "Push Day")
				}
				if out.Type != "strength" {
					t.Errorf("Type: got %q want %q", out.Type, "strength")
				}
				if out.Slot != "1" {
					t.Errorf("Slot: got %q want %q", out.Slot, "1")
				}
				if out.Style != "circuit" {
					t.Errorf("Style: got %q want %q", out.Style, "circuit")
				}
				if out.Surface != "gym" {
					t.Errorf("Surface: got %q want %q", out.Surface, "gym")
				}
				if out.RestInterval != "90s" {
					t.Errorf("RestInterval: got %q want %q", out.RestInterval, "90s")
				}
				if out.DurationMin != 60 {
					t.Errorf("DurationMin: got %v want %v", out.DurationMin, 60)
				}
				if out.RPE != 7.5 {
					t.Errorf("RPE: got %v want %v", out.RPE, 7.5)
				}
				if out.AvgHR != 142 {
					t.Errorf("AvgHR: got %v want %v", out.AvgHR, 142)
				}
				if out.MaxHR != 175 {
					t.Errorf("MaxHR: got %v want %v", out.MaxHR, 175)
				}
				if out.CaloriesBurned != 480 {
					t.Errorf("CaloriesBurned: got %v want %v", out.CaloriesBurned, 480)
				}
				if out.Recovers != "lower" {
					t.Errorf("Recovers: got %q want %q", out.Recovers, "lower")
				}
				if out.Day != 3 {
					t.Errorf("Day: got %v want %v", out.Day, 3)
				}
				if len(out.Focus) != 2 || out.Focus[0] != "Push(U)" || out.Focus[1] != "Core" {
					t.Errorf("Focus: got %v want %v", out.Focus, []string{"Push(U)", "Core"})
				}
				if out.Notes != "Felt strong today" {
					t.Errorf("Notes: got %q want %q", out.Notes, "Felt strong today")
				}
				if len(out.Exercises) != 2 {
					t.Fatalf("expected 2 exercises, got %d", len(out.Exercises))
				}
				bench := out.Exercises[0]
				if bench.Name != "Bench Press" {
					t.Errorf("bench name: got %q want %q", bench.Name, "Bench Press")
				}
				if bench.Category != "push" {
					t.Errorf("bench category: got %q want %q", bench.Category, "push")
				}
				if bench.Bias != "unilateral" {
					t.Errorf("bench bias: got %q want %q", bench.Bias, "unilateral")
				}
				if bench.Tempo != "2-0-1-1" {
					t.Errorf("bench tempo: got %q want %q", bench.Tempo, "2-0-1-1")
				}
				if len(bench.Sets) != 4 {
					t.Fatalf("bench sets: got %d want %d", len(bench.Sets), 4)
				}
				for i, s := range bench.Sets {
					if s.Reps != 8 {
						t.Errorf("bench set %d reps: got %d want %d", i, s.Reps, 8)
					}
					if s.LoadLbs != 135 {
						t.Errorf("bench set %d load: got %v want %v", i, s.LoadLbs, 135)
					}
					if s.TUTSeconds != 32 {
						t.Errorf("bench set %d tut: got %v want %v", i, s.TUTSeconds, 32)
					}
				}

				sled := out.Exercises[1]
				if sled.Name != "Sled Push" {
					t.Errorf("sled name: got %q want %q", sled.Name, "Sled Push")
				}
				if sled.Category != "conditioning" {
					t.Errorf("sled category: got %q want %q", sled.Category, "conditioning")
				}
				if sled.METValue != 8.5 {
					t.Errorf("sled met: got %v want %v", sled.METValue, 8.5)
				}
				if sled.DistanceKm != 0.1 {
					t.Errorf("sled distance: got %v want %v", sled.DistanceKm, 0.1)
				}
				if len(sled.Sets) != 3 {
					t.Fatalf("sled sets: got %d want %d", len(sled.Sets), 3)
				}
				for i, s := range sled.Sets {
					if s.TUTSeconds != 35 {
						t.Errorf("sled set %d tut: got %v want %v", i, s.TUTSeconds, 35)
					}
				}
			},
		},
		{
			name: "Inline focus array",
			in: `title: Test
focus: ["Hinge(B)", "Squat(U)"]
exercises:
  - name: Deadlift
    sets: 1
    reps: 1
`,
			validate: func(t *testing.T, out models.ParsedWorkout, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if len(out.Focus) != 2 || out.Focus[0] != "Hinge(B)" || out.Focus[1] != "Squat(U)" {
					t.Errorf("Focus: got %v want %v", out.Focus, []string{"Hinge(B)", "Squat(U)"})
				}
			},
		},
		{
			name: "Load variants",
			in: `title: Bodyweight Day
exercises:
  - name: Pull Up
    sets: 3
    reps: 10
    load: BW
  - name: Farmer Carry
    sets: 4
    reps: 1
    load: 35+35 lbs
`,
			validate: func(t *testing.T, out models.ParsedWorkout, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if len(out.Exercises) != 2 {
					t.Fatalf("expected 2 exercises, got %d", len(out.Exercises))
				}
				pu := out.Exercises[0]
				if len(pu.Sets) != 3 {
					t.Fatalf("pull up sets: got %d want %d", len(pu.Sets), 3)
				}
				for i, s := range pu.Sets {
					if s.LoadLbs != 0 {
						t.Errorf("pull up set %d load: got %v want %v", i, s.LoadLbs, 0)
					}
				}
				fc := out.Exercises[1]
				if len(fc.Sets) != 4 {
					t.Fatalf("farmer carry sets: got %d want %d", len(fc.Sets), 4)
				}
				for i, s := range fc.Sets {
					if s.LoadLbs != 70 {
						t.Errorf("farmer carry set %d load: got %v want %v", i, s.LoadLbs, 70)
					}
				}
			},
		},
		{
			name: "Direct bias field",
			in: `title: Test
exercises:
  - name: Romanian Deadlift
    sets: 3
    reps: 8
    pattern: hinge
    bias: U
`,
			validate: func(t *testing.T, out models.ParsedWorkout, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if len(out.Exercises) != 1 {
					t.Fatalf("expected 1 exercise, got %d", len(out.Exercises))
				}
				ex := out.Exercises[0]
				if ex.Category != "hinge" {
					t.Errorf("category: got %q want %q", ex.Category, "hinge")
				}
				if ex.Bias != "unilateral" {
					t.Errorf("bias: got %q want %q", ex.Bias, "unilateral")
				}
			},
		},
		{
			name: "Inline comments stripped",
			in: `title: Test # this is a comment
exercises:
  - name: Squat # bilateral
    sets: 3 # three sets
    reps: 5
`,
			validate: func(t *testing.T, out models.ParsedWorkout, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if out.Title != "Test" {
					t.Errorf("Title: got %q want %q", out.Title, "Test")
				}
				if len(out.Exercises) != 1 {
					t.Fatalf("expected 1 exercise, got %d", len(out.Exercises))
				}
				ex := out.Exercises[0]
				if ex.Name != "Squat" {
					t.Errorf("exercise name: got %q want %q", ex.Name, "Squat")
				}
				if len(ex.Sets) != 3 {
					t.Fatalf("sets length: got %d want %d", len(ex.Sets), 3)
				}
				for i, s := range ex.Sets {
					if s.Reps != 5 {
						t.Errorf("set %d reps: got %d want %d", i, s.Reps, 5)
					}
				}
			},
		},
		{
			name: "inferPattern fallback",
			in: `title: Test
exercises:
  - name: Barbell Deadlift
    sets: 3
    reps: 5
  - name: Sprint Intervals
    sets: 8
    duration: 30 sec
`,
			validate: func(t *testing.T, out models.ParsedWorkout, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if len(out.Exercises) != 2 {
					t.Fatalf("expected 2 exercises, got %d", len(out.Exercises))
				}
				if out.Exercises[0].Category != "hinge" {
					t.Errorf("deadlift category: got %q want %q", out.Exercises[0].Category, "hinge")
				}
				if out.Exercises[1].Category != "conditioning" {
					t.Errorf("sprint category: got %q want %q", out.Exercises[1].Category, "conditioning")
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			out, err := ParseYAML(tc.in)
			tc.validate(t, out, err)
		})
	}
}

func TestParsePatternBias(t *testing.T) {
	tests := []struct {
		in          string
		wantPattern string
		wantBias    string
	}{
		{"squat B", "squat", "bilateral"},
		{"hinge U", "hinge", "unilateral"},
		{"Squat(B)", "squat", "bilateral"},
		{"Push(U)", "push", "unilateral"},
		{"conditioning", "conditioning", ""},
		{"", "", ""},
		{"core", "core", ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.in, func(t *testing.T) {
			p, b := parsePatternBias(tt.in)
			if p != tt.wantPattern {
				t.Errorf("pattern: got %q want %q", p, tt.wantPattern)
			}
			if b != tt.wantBias {
				t.Errorf("bias: got %q want %q", b, tt.wantBias)
			}
		})
	}
}

func TestParseLoad(t *testing.T) {
	tests := []struct {
		in   string
		want float64
	}{
		{"", 0},
		{"BW", 0},
		{"50 lbs", 50},
		{"35+35 lbs", 70},
		{"100 lb", 100},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.in, func(t *testing.T) {
			got := parseLoad(tt.in)
			if got != tt.want {
				t.Errorf("parseLoad(%q): got %v want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseDurationToSeconds(t *testing.T) {
	tests := []struct {
		in   string
		want float64
	}{
		{"35 sec", 35},
		{"2:30 min", 150},
		{"2:00 min", 120},
		{"23:00 min", 1380},
		{"60 sec", 60},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.in, func(t *testing.T) {
			got := parseDurationToSeconds(tt.in)
			if got != tt.want {
				t.Errorf("parseDurationToSeconds(%q): got %v want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseTUTPerRep(t *testing.T) {
	tests := []struct {
		in   string
		want float64
	}{
		{"2-0-2-0", 4},
		{"3-1-2-1", 7},
		{"1-0-1-0", 2},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.in, func(t *testing.T) {
			got := parseTUTPerRep(tt.in)
			if got != tt.want {
				t.Errorf("parseTUTPerRep(%q): got %v want %v", tt.in, got, tt.want)
			}
		})
	}
}
