package export

// These tests pin the Wave P3 export contract at the renderer boundary:
//
//   - RFC 4180 quoting ("" inside quoted fields, never backslash-escaped)
//   - a UTF-8 BOM at the start of every CSV payload
//   - unit-aware load/weight/waist columns with empty cells for unlogged values
//   - explicit # nutrition / # workouts section markers in the combined CSV
//   - positive-only biometric rendering (no fake "Weight: 0.0 lbs")
//
// They deliberately call the exported renderers directly so quoting and column
// layout are asserted from the exact strings a client would receive.

import (
	"encoding/csv"
	"strings"
	"testing"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
)

func parseCSV(t *testing.T, content string) [][]string {
	t.Helper()
	content = strings.TrimPrefix(content, utf8BOM)
	r := csv.NewReader(strings.NewReader(content))
	r.Comment = '#'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v\ncontent:\n%s", err, content)
	}
	return rows
}

func columnIndex(t *testing.T, header []string, name string) int {
	t.Helper()
	for i, h := range header {
		if h == name {
			return i
		}
	}
	t.Fatalf("CSV header %v is missing column %q", header, name)
	return -1
}

// RFC 4180 requires embedded quotes to be doubled, and commas inside a field to
// force that field into quotes. The old hand-rolled writer emitted \" sequences,
// which is not valid CSV and breaks spreadsheet importers.
func TestCSVQuotesRFC4180(t *testing.T) {
	const date = "2026-10-20"

	nutrition := NutritionCSV([]models.NutritionLog{{
		Date:      date,
		Calories:  500,
		MealNotes: `He said "eat, more"`,
	}}, date, date)

	if strings.Contains(nutrition, `\"`) {
		t.Fatalf("nutrition CSV contains backslash-escaped quotes:\n%s", nutrition)
	}
	if !strings.Contains(nutrition, `""eat, more""`) {
		t.Fatalf("nutrition CSV did not double embedded quotes:\n%s", nutrition)
	}
	if rows := parseCSV(t, nutrition); len(rows) != 2 || rows[1][7] != `He said "eat, more"` {
		t.Fatalf("round-tripped nutrition notes = %#v, want the original string", rows)
	}

	workouts := WorkoutsCSV([]models.WorkoutEntry{{
		Date:  date,
		Slot:  "1",
		Title: `Squat "PR", day`,
	}}, date, date, "metric")

	if strings.Contains(workouts, `\"`) {
		t.Fatalf("workouts CSV contains backslash-escaped quotes:\n%s", workouts)
	}
	rows := parseCSV(t, workouts)
	if len(rows) != 2 {
		t.Fatalf("workouts CSV rows = %d, want 2\n%s", len(rows), workouts)
	}
	if got := rows[1][columnIndex(t, rows[0], "title")]; got != `Squat "PR", day` {
		t.Fatalf("round-tripped workout title = %q, want %q", got, `Squat "PR", day`)
	}
}

func TestCSVStartsWithUTF8BOM(t *testing.T) {
	const date = "2026-10-20"
	payloads := map[string]string{
		"nutrition": NutritionCSV(nil, date, date),
		"workouts":  WorkoutsCSV(nil, date, date, "metric"),
		"combined": CombinedCSV(
			[]models.NutritionLog{{Date: date, Calories: 100}},
			[]models.BiometricLog{{Date: date, WeightKg: 80}},
			nil, date, date, "metric",
		),
	}
	for name, got := range payloads {
		if !strings.HasPrefix(got, utf8BOM) {
			t.Errorf("%s CSV does not start with a UTF-8 BOM", name)
		}
	}
}

// Worked example from the export spec: a set stored canonically as 61.23 kg
// must be 135.0 lb for an imperial profile, while a metric profile keeps the
// stored kilogram value (two decimals, not a lossy one-decimal rounding).
func TestWorkoutsCSVUnitAwareLoad(t *testing.T) {
	const date = "2026-10-20"
	workout := models.WorkoutEntry{
		Date:  date,
		Slot:  "1",
		Title: "Back Squat",
		Exercises: []models.ExerciseEntry{{
			Name: "Back Squat",
			Sets: []models.ExerciseSet{{Reps: 5, LoadKg: 61.23}},
		}},
	}

	metricRows := parseCSV(t, WorkoutsCSV([]models.WorkoutEntry{workout}, date, date, "metric"))
	if got := metricRows[1][columnIndex(t, metricRows[0], "load_kg")]; got != "61.23" {
		t.Fatalf("metric load_kg = %q, want 61.23", got)
	}

	imperialRows := parseCSV(t, WorkoutsCSV([]models.WorkoutEntry{workout}, date, date, "imperial"))
	if got := imperialRows[1][columnIndex(t, imperialRows[0], "load_lbs")]; got != "135.0" {
		t.Fatalf("imperial load_lbs = %q, want 135.0", got)
	}
}

func TestCombinedCSVSectionMarkersAndUnitColumns(t *testing.T) {
	const (
		first  = "2026-10-20"
		second = "2026-10-21"
	)

	logs := []models.NutritionLog{
		{Date: first, Calories: 2000, ProteinG: 150},
		{Date: second, Calories: 2100, ProteinG: 160},
	}
	bios := []models.BiometricLog{{Date: first, WeightKg: 61.23, WaistCm: 90}}

	content := CombinedCSV(logs, bios, nil, first, second, "imperial")
	if !strings.Contains(content, "# nutrition") {
		t.Fatalf("combined CSV is missing the # nutrition section marker:\n%s", content)
	}
	if !strings.Contains(content, "# workouts") {
		t.Fatalf("combined CSV is missing the # workouts section marker:\n%s", content)
	}
	if strings.Contains(content, "0.0 lbs") {
		t.Fatalf("combined CSV rendered a fake zero measurement:\n%s", content)
	}

	parts := strings.SplitN(content, "# workouts", 2)
	dailyRows := parseCSV(t, parts[0])
	weightCol := columnIndex(t, dailyRows[0], "weight_lbs")
	waistCol := columnIndex(t, dailyRows[0], "waist_in")

	byDate := make(map[string][]string, len(dailyRows)-1)
	for _, row := range dailyRows[1:] {
		byDate[row[0]] = row
	}

	firstRow, ok := byDate[first]
	if !ok {
		t.Fatalf("combined daily section is missing %s: %v", first, byDate)
	}
	if got := firstRow[weightCol]; got != "135.0" {
		t.Fatalf("imperial weight_lbs = %q, want 135.0", got)
	}
	if got := firstRow[waistCol]; got != "35.4" {
		t.Fatalf("imperial waist_in = %q, want 35.4", got)
	}

	secondRow, ok := byDate[second]
	if !ok {
		t.Fatalf("combined daily section is missing %s: %v", second, byDate)
	}
	if got := secondRow[weightCol]; got != "" {
		t.Fatalf("unlogged weight cell = %q, want empty", got)
	}
	if got := secondRow[waistCol]; got != "" {
		t.Fatalf("unlogged waist cell = %q, want empty", got)
	}

	workoutRows := parseCSV(t, parts[1])
	if len(workoutRows) != 1 { // header only: there were no workouts
		t.Fatalf("workout section rows = %d, want 1 header row", len(workoutRows))
	}
}

func TestCombinedMarkdownSkipsUnloggedWeight(t *testing.T) {
	const date = "2026-10-20"
	md := CombinedMarkdown(nil, []models.BiometricLog{{
		Date:    date,
		WaistCm: 90,
	}}, nil, models.Targets{}, date, date, "imperial")

	if strings.Contains(md, "Weight:") {
		t.Fatalf("measurement-only row rendered a weight line:\n%s", md)
	}
	if strings.Contains(md, "Weight: 0.0 lbs") {
		t.Fatalf("measurement-only row rendered the old fake weight:\n%s", md)
	}
	if !strings.Contains(md, "Waist: 35.4 in") {
		t.Fatalf("measurement-only row did not render its waist:\n%s", md)
	}
}
