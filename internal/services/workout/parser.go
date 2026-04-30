package workout

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/services/met"
)

// ParseYAML parses a workout YAML string into a ParsedWorkout.
// Uses a hand-rolled line-by-line parser — no external YAML dependency.
// Supports the full schema defined in the Obsidian training doc.
func ParseYAML(text string) (models.ParsedWorkout, error) {
	if strings.TrimSpace(text) == "" {
		return models.ParsedWorkout{}, fmt.Errorf("empty input")
	}

	var out models.ParsedWorkout
	out.RawInput = text

	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")

	// Parser state
	inExercises := false
	inFocus := false

	type rawEx struct {
		name        string
		sets        int
		reps        int
		durationRaw string
		tempo       string
		load        string
		rpe         float64
		pattern     string
		bias        string
		metValue    float64
		distanceKm  float64
		elevationM  float64
		pace        string
		notes       string
	}

	var exercises []rawEx
	var cur *rawEx

	flushExercise := func() {
		if cur != nil {
			exercises = append(exercises, *cur)
			cur = nil
		}
	}

	applyExField := func(rx *rawEx, k, v string) {
		switch k {
		case "name":
			rx.name = v
		case "sets":
			rx.sets, _ = strconv.Atoi(v)
		case "reps":
			rx.reps, _ = strconv.Atoi(v)
		case "duration":
			rx.durationRaw = v
		case "tempo":
			rx.tempo = v
		case "load":
			rx.load = v
		case "rpe", "rep": // "rep" is a known typo in source notes
			rx.rpe, _ = strconv.ParseFloat(v, 64)
		case "pattern":
			rx.pattern, rx.bias = parsePatternBias(v)
		case "bias":
			switch strings.ToLower(v) {
			case "b", "bilateral":
				rx.bias = "bilateral"
			case "u", "unilateral":
				rx.bias = "unilateral"
			}
		case "met":
			rx.metValue, _ = strconv.ParseFloat(v, 64)
		case "distance_km":
			rx.distanceKm, _ = strconv.ParseFloat(v, 64)
		case "distance":
			// Accept both `distance_km` and `distance` in source YAML
			rx.distanceKm, _ = strconv.ParseFloat(v, 64)
		case "elevation_m":
			rx.elevationM, _ = strconv.ParseFloat(v, 64)
		case "elevation":
			// Accept both `elevation_m` and `elevation` in source YAML
			rx.elevationM, _ = strconv.ParseFloat(v, 64)
		case "pace":
			rx.pace = v
		case "notes":
			rx.notes = v
		}
	}

	for _, rawLine := range lines {
		// Strip trailing inline comments (space + #)
		line := rawLine
		if ci := strings.Index(line, " #"); ci >= 0 {
			line = line[:ci]
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "---" || trimmed == "..." {
			continue
		}

		indent := indentLevel(rawLine)

		// ── Top-level keys (indent == 0) ─────────────────────────
		if indent == 0 {
			inFocus = false

			if trimmed == "exercises:" {
				inExercises = true
				flushExercise()
				continue
			}
			if trimmed == "focus:" {
				inFocus = true
				inExercises = false
				continue
			}
			// Any other top-level key exits exercises block
			inExercises = false

			k, v := splitKV(trimmed)
			switch k {
			case "name", "title":
				out.Title = v
			case "type":
				out.Type = v
			case "slot":
				out.Slot = v
			case "style":
				out.Style = v
			case "surface", "surface_condition":
				out.Surface = v
			case "rest_interval", "rest":
				out.RestInterval = v
			case "day":
				out.Day, _ = strconv.Atoi(v)
			case "duration", "duration_min":
				out.DurationMin, _ = strconv.ParseFloat(v, 64)
			case "rpe":
				out.RPE, _ = strconv.ParseFloat(v, 64)
			case "avg_hr":
				n, _ := strconv.ParseFloat(v, 64)
				out.AvgHR = int(n)
			case "max_hr":
				n, _ := strconv.ParseFloat(v, 64)
				out.MaxHR = int(n)
			case "calories", "calories_burned":
				out.CaloriesBurned, _ = strconv.ParseFloat(v, 64)
			case "recovers":
				out.Recovers = v
			case "notes":
				out.Notes = v
			case "focus":
				// Inline array: focus: ["Hinge(B)", "Push(U)"]
				if v != "" {
					out.Focus = parseInlineArray(v)
				}
			}
			continue
		}

		// ── Focus block items ─────────────────────────────────────
		if inFocus && strings.HasPrefix(trimmed, "- ") {
			item := unquote(strings.TrimPrefix(trimmed, "- "))
			out.Focus = append(out.Focus, item)
			continue
		}

		// ── Exercise block ────────────────────────────────────────
		if inExercises {
			if strings.HasPrefix(trimmed, "- ") {
				flushExercise()
				cur = &rawEx{}
				rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
				if rest != "" {
					k, v := splitKV(rest)
					applyExField(cur, k, v)
				}
			} else if cur != nil {
				k, v := splitKV(trimmed)
				applyExField(cur, k, v)
			}
		}
	}
	flushExercise()

	// ── Convert rawEx → models.ExerciseEntry ─────────────────────
	for _, rx := range exercises {
		entry := models.ExerciseEntry{
			Name:        rx.name,
			Category:    rx.pattern,
			Bias:        rx.bias,
			Surface:     out.Surface,
			Notes:       rx.notes,
			DistanceKm:  rx.distanceKm,
			ElevationM:  rx.elevationM,
			Pace:        rx.pace,
			RPE:         rx.rpe,
			LoadRaw:     rx.load,
			DurationRaw: rx.durationRaw,
			Tempo:       rx.tempo,
		}

		// MET: explicit override wins; else RPE fallback, then name-based lookup
		if rx.metValue > 0 {
			entry.METValue = rx.metValue
		} else if entry.RPE > 0 {
			entry.METValue = EstimateMETFromRPE(entry.RPE)
		} else {
			entry.METValue = met.LookupMET(rx.name)
		}

		// Auto-infer pattern for common conditioning exercises if not explicitly set
		if entry.Category == "" {
			entry.Category = inferPattern(rx.name)
		}

		sets := rx.sets
		if sets <= 0 {
			sets = 1
		}
		loadLbs := parseLoad(rx.load)

		var tutPerSet float64
		if rx.durationRaw != "" {
			tutPerSet = parseDurationToSeconds(rx.durationRaw)
		} else if rx.tempo != "" && rx.reps > 0 {
			tutPerSet = parseTUTPerRep(rx.tempo) * float64(rx.reps)
		}

		for i := 0; i < sets; i++ {
			entry.Sets = append(entry.Sets, models.ExerciseSet{
				Reps:       rx.reps,
				LoadLbs:    loadLbs,
				LoadKg:     loadLbs * 0.45359237,
				TUTSeconds: tutPerSet,
			})
		}

		out.Exercises = append(out.Exercises, entry)
	}

	return out, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// indentLevel counts leading spaces (tabs count as 2).
func indentLevel(line string) int {
	n := 0
	for _, c := range line {
		switch c {
		case ' ':
			n++
		case '\t':
			n += 2
		default:
			return n
		}
	}
	return n
}

// splitKV splits "key: value" → ("key", "value"), unquoting the value.
func splitKV(s string) (string, string) {
	idx := strings.Index(s, ":")
	if idx < 0 {
		return strings.TrimSpace(s), ""
	}
	k := strings.TrimSpace(s[:idx])
	v := unquote(strings.TrimSpace(s[idx+1:]))
	return k, v
}

// unquote strips surrounding double or single quotes.
func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// parseInlineArray parses ["item1", "item2"] or [item1, item2].
func parseInlineArray(s string) []string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		item := unquote(strings.TrimSpace(p))
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

// parseLoad converts load strings to lbs.
// "BW" or "" → 0, "50 lbs" → 50, "35+35 lbs" → 70.
func parseLoad(load string) float64 {
	s := strings.TrimSpace(load)
	if s == "" || strings.EqualFold(s, "BW") {
		return 0
	}
	// Strip unit suffix (case-insensitive)
	s = strings.TrimRight(s, " ")
	s = strings.TrimSuffix(strings.ToLower(s), "lbs")
	s = strings.TrimSuffix(s, "lb")
	s = strings.TrimSpace(s)

	total := 0.0
	for _, part := range strings.Split(s, "+") {
		n, _ := strconv.ParseFloat(strings.TrimSpace(part), 64)
		total += n
	}
	return total
}

// parseDurationToSeconds converts duration strings to seconds.
// "35 sec" → 35, "2:30 min" → 150, "2:00 min" → 120, "23:00 min" → 1380.
func parseDurationToSeconds(s string) float64 {
	s = strings.TrimSpace(strings.ToLower(s))

	hasMM := strings.Contains(s, "min")
	hasSec := strings.Contains(s, "sec")

	// Strip unit words
	s = strings.ReplaceAll(s, "min", "")
	s = strings.ReplaceAll(s, "sec", "")
	s = strings.TrimSpace(s)

	if strings.Contains(s, ":") {
		// M:SS format
		parts := strings.SplitN(s, ":", 2)
		m, _ := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		sec, _ := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		total := m*60 + sec
		if hasMM {
			return total // already in seconds (M:SS where M=minutes)
		}
		return total
	}

	n, _ := strconv.ParseFloat(s, 64)
	if hasMM {
		return n * 60
	}
	if hasSec || n > 0 {
		return n
	}
	return 0
}

// parseTUTPerRep sums tempo parts: "2-0-2-0" → 4.0 seconds per rep.
func parseTUTPerRep(tempo string) float64 {
	total := 0.0
	for _, p := range strings.Split(tempo, "-") {
		n, _ := strconv.ParseFloat(strings.TrimSpace(p), 64)
		total += n
	}
	return total
}

// parsePatternBias splits a pattern string into movement pattern and bilateral/unilateral bias.
// Handles these formats:
//
//	"squat B"    → ("squat", "bilateral")
//	"hinge U"    → ("hinge", "unilateral")
//	"Squat(B)"   → ("squat", "bilateral")
//	"Push(U)"    → ("push", "unilateral")
//	"conditioning" → ("conditioning", "")
//	"core"         → ("core", "")
func parsePatternBias(s string) (pattern, bias string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}

	// Parenthetical form: "Squat(B)", "Hinge(U)"
	if open := strings.Index(s, "("); open >= 0 {
		close := strings.Index(s, ")")
		if close > open {
			pattern = strings.ToLower(strings.TrimSpace(s[:open]))
			switch strings.ToUpper(strings.TrimSpace(s[open+1 : close])) {
			case "B":
				bias = "bilateral"
			case "U":
				bias = "unilateral"
			}
			return
		}
	}

	// Space-separated form: "squat B", "hinge U"
	parts := strings.Fields(s)
	if len(parts) == 2 {
		switch strings.ToUpper(parts[1]) {
		case "B":
			pattern = strings.ToLower(parts[0])
			bias = "bilateral"
			return
		case "U":
			pattern = strings.ToLower(parts[0])
			bias = "unilateral"
			return
		}
	}

	// No bias indicator — return as-is (lowercased)
	pattern = strings.ToLower(s)
	return
}

// inferPattern returns a best-guess movement pattern for well-known exercise names.
// Used as fallback when no explicit pattern: field is provided.
func inferPattern(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	conditioningKeywords := []string{
		"sled", "sprint", "run", "jog", "row", "bike", "cycle", "swim",
		"battle rope", "jump rope", "burpee", "hiit", "cardio",
	}
	for _, kw := range conditioningKeywords {
		if strings.Contains(n, kw) {
			return "conditioning"
		}
	}
	hingeKeywords := []string{"deadlift", "hip thrust", "rdl", "good morning", "swing", "clean"}
	for _, kw := range hingeKeywords {
		if strings.Contains(n, kw) {
			return "hinge"
		}
	}
	squatKeywords := []string{"squat", "lunge", "split squat", "step up", "leg press"}
	for _, kw := range squatKeywords {
		if strings.Contains(n, kw) {
			return "squat"
		}
	}
	pushKeywords := []string{"push", "press", "dip", "planche"}
	for _, kw := range pushKeywords {
		if strings.Contains(n, kw) {
			return "push"
		}
	}
	pullKeywords := []string{"pull", "row", "chin", "curl"}
	for _, kw := range pullKeywords {
		if strings.Contains(n, kw) {
			return "pull"
		}
	}
	return ""
}

// EstimateMETFromRPE estimates MET from RPE (0-10) when no explicit MET provided.
// Maps RPE to typical MET values for strength work:
// RPE 9-10: 8 METs (high intensity)
// RPE 7-8: 6 METs (moderate-high)
// RPE 5-6: 5 METs (moderate)
// RPE <5: 3-4 METs (light)
func EstimateMETFromRPE(rpe float64) float64 {
	switch {
	case rpe >= 9:
		return 8.0
	case rpe >= 7:
		return 6.0
	case rpe >= 5:
		return 5.0
	case rpe > 0:
		return 4.0
	default:
		return 4.0 // default for strength
	}
}

// CalculateCaloriesBurned computes calories burned from MET values.
// Formula: calories = MET × weight_kg × duration_hours
// If no MET provided, returns 0 so caller can use user-reported value.
func CalculateCaloriesBurned(exercises []models.ExerciseEntry, weightKg, durationMin float64) float64 {
	if len(exercises) == 0 || weightKg <= 0 || durationMin <= 0 {
		return 0
	}

	// Use average MET if we have exercises with MET values
	var totalMET float64
	var metCount int
	for _, ex := range exercises {
		if ex.METValue > 0 {
			totalMET += ex.METValue
			metCount++
		}
	}

	avgMET := 4.0 // default for strength work
	if metCount > 0 {
		avgMET = totalMET / float64(metCount)
	}

	durationHours := durationMin / 60.0
	calories := avgMET * weightKg * durationHours
	return calories
}
