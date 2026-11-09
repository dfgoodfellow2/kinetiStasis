package export

import (
	"encoding/csv"
	"fmt"
	"sort"
	"strings"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	unitsconv "github.com/dfgoodfellow2/diet-tracker/v2/internal/services/units"
)

// utf8BOM is written once at the start of every CSV payload. Excel needs it to
// decode non-ASCII text (for example the em-dash in workout titles) as UTF-8
// instead of the system code page.
const utf8BOM = "\ufeff"

// nutritionRowStyle selects how writeNutritionDayRow renders a day's nutrition
// values.
type nutritionRowStyle int

const (
	nutritionTableRow nutritionRowStyle = iota
	nutritionBulletRow
)

// writeNutritionDayRow is the single renderer for a day's nutrition values.
// It emits either a Markdown table row (NutritionMarkdown) or a bullet block
// (CombinedMarkdown), always including fiber and water so the two exports
// cannot drift apart.
func writeNutritionDayRow(b *strings.Builder, n models.NutritionLog, style nutritionRowStyle) {
	if style == nutritionTableRow {
		notes := strings.ReplaceAll(n.MealNotes, "\n", " ")
		// escape pipe characters
		notes = strings.ReplaceAll(notes, "|", "\\|")
		b.WriteString(fmt.Sprintf("| %s | %.0f | %.0f | %.0f | %.0f | %.0f | %.0f | %s |\n",
			n.Date, n.Calories, n.ProteinG, n.CarbsG, n.FatG, n.FiberG, n.WaterMl, notes))
		return
	}

	b.WriteString(fmt.Sprintf("- Calories: %.0f (P: %.0f g, C: %.0f g, F: %.0f g, Fiber: %.0f g)\n",
		n.Calories, n.ProteinG, n.CarbsG, n.FatG, n.FiberG))
	b.WriteString(fmt.Sprintf("- Water: %.0f ml\n", n.WaterMl))
	if n.MealNotes != "" {
		b.WriteString(fmt.Sprintf("- Meals: %s\n", n.MealNotes))
	}
}

// NutritionMarkdown generates a Markdown table of daily nutrition logs between from/to dates.
func NutritionMarkdown(logs []models.NutritionLog, targets models.Targets, from, to string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Nutrition Log: %s to %s\n\n", from, to))

	b.WriteString("| Date | Calories | Protein (g) | Carbs (g) | Fat (g) | Fiber (g) | Water (ml) | Meal notes |\n")
	b.WriteString("|------|----------:|------------:|----------:|--------:|---------:|----------:|------------|\n")

	var totalCalories float64
	var totalProtein, totalCarbs, totalFat, totalFiber, totalWater float64
	var count int

	for _, l := range logs {
		if l.Date < from || l.Date > to {
			continue
		}
		count++
		totalCalories += l.Calories
		totalProtein += l.ProteinG
		totalCarbs += l.CarbsG
		totalFat += l.FatG
		totalFiber += l.FiberG
		totalWater += l.WaterMl

		writeNutritionDayRow(&b, l, nutritionTableRow)
	}

	if count == 0 {
		b.WriteString("\nNo nutrition logs in this range.\n")
		return b.String()
	}

	avgCals := totalCalories / float64(count)

	b.WriteString("\n---\n\n")
	b.WriteString("## Summary\n\n")
	b.WriteString(fmt.Sprintf("- **Days:** %d\n", count))
	b.WriteString(fmt.Sprintf("- **Avg calories:** %.0f\n", avgCals))
	b.WriteString(fmt.Sprintf("- **Total calories:** %.0f\n", totalCalories))
	b.WriteString(fmt.Sprintf("- **Total protein:** %.0f g\n", totalProtein))
	b.WriteString(fmt.Sprintf("- **Total carbs:** %.0f g\n", totalCarbs))
	b.WriteString(fmt.Sprintf("- **Total fat:** %.0f g\n", totalFat))
	b.WriteString(fmt.Sprintf("- **Total fiber:** %.0f g\n", totalFiber))
	b.WriteString(fmt.Sprintf("- **Total water:** %.0f ml\n", totalWater))

	if targets.Calories > 0 {
		b.WriteString(fmt.Sprintf("- **Target calories:** %.0f\n", targets.Calories))
	}

	return b.String()
}

// NutritionCSV outputs RFC 4180 CSV:
// date,calories,protein_g,carbs_g,fat_g,fiber_g,water_ml,meal_notes
//
// encoding/csv handles field quoting (embedded quotes are doubled, commas and
// newlines force quotes) and CRLF record separators. A UTF-8 BOM is prepended
// so Excel decodes the file as UTF-8.
func NutritionCSV(logs []models.NutritionLog, from, to string) string {
	var b strings.Builder
	b.WriteString(utf8BOM)
	writeNutritionCSVRows(&b, logs, from, to)
	return b.String()
}

func writeNutritionCSVRows(b *strings.Builder, logs []models.NutritionLog, from, to string) {
	w := csv.NewWriter(b)
	w.UseCRLF = true
	_ = w.Write([]string{
		"date", "calories", "protein_g", "carbs_g", "fat_g", "fiber_g", "water_ml", "meal_notes",
	})
	for _, l := range logs {
		if l.Date < from || l.Date > to {
			continue
		}
		notes := strings.ReplaceAll(l.MealNotes, "\n", " ")
		_ = w.Write([]string{
			l.Date,
			fmt.Sprintf("%.0f", l.Calories),
			fmt.Sprintf("%.0f", l.ProteinG),
			fmt.Sprintf("%.0f", l.CarbsG),
			fmt.Sprintf("%.0f", l.FatG),
			fmt.Sprintf("%.0f", l.FiberG),
			fmt.Sprintf("%.0f", l.WaterMl),
			notes,
		})
	}
	w.Flush()
}

// WorkoutsMarkdown generates a Markdown document of workouts between from/to
func WorkoutsMarkdown(workouts []models.WorkoutEntry, from, to, units string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Workout Log: %s to %s\n\n", from, to))

	var filtered []models.WorkoutEntry
	for _, w := range workouts {
		if w.Date >= from && w.Date <= to {
			filtered = append(filtered, w)
		}
	}
	if len(filtered) == 0 {
		b.WriteString("No workouts in this range.\n")
		return b.String()
	}

	for _, w := range filtered {
		b.WriteString(fmt.Sprintf("## %s — %s\n\n", w.Date, w.Title))
		b.WriteString(fmt.Sprintf("- Slot: %s\n", w.Slot))
		b.WriteString(fmt.Sprintf("- Duration: %.0f min\n", w.DurationMin))
		// session-level metadata: RPE and focus tags
		if w.Metadata.RPE > 0 {
			b.WriteString(fmt.Sprintf("- Session RPE: %.1f\n", w.Metadata.RPE))
		}
		if len(w.Metadata.Focus) > 0 {
			b.WriteString(fmt.Sprintf("- Focus: %s\n", strings.Join(w.Metadata.Focus, ", ")))
		}
		// average exercise RPE (from exercises that have RPE)
		var sumRPE float64
		var rpeCount int
		for _, ex := range w.Exercises {
			if ex.RPE > 0 {
				sumRPE += ex.RPE
				rpeCount++
			}
		}
		if rpeCount > 0 {
			avg := sumRPE / float64(rpeCount)
			b.WriteString(fmt.Sprintf("- Avg exercise RPE: %.1f\n", avg))
		}
		if w.CaloriesBurned > 0 {
			b.WriteString(fmt.Sprintf("- Calories burned: %.0f\n", w.CaloriesBurned))
		}
		if w.MWV > 0 {
			b.WriteString(fmt.Sprintf("- MWV: %.0f\n", w.MWV))
		}
		if w.NDS > 0 {
			b.WriteString(fmt.Sprintf("- NDS: %.0f\n", w.NDS))
		}
		if len(w.Exercises) > 0 {
			b.WriteString("- Exercises:\n")
			for _, ex := range w.Exercises {
				phaseTag := ""
				if ex.Phase == "warmup" {
					phaseTag = " [Warmup]"
				} else if ex.Phase == "cooldown" {
					phaseTag = " [Cooldown]"
				}
				groupTag := ""
				if ex.GroupID != "" {
					groupTag = fmt.Sprintf(" (group: %s #%d)", ex.GroupID, ex.GroupOrder)
				}
				b.WriteString(fmt.Sprintf("  - %s%s%s\n", ex.Name, phaseTag, groupTag))
				// list sets if available
				if len(ex.Sets) > 0 {
					for i, s := range ex.Sets {
						// display canonical kg value, convert for imperial if requested
						loadVal := s.LoadKg
						loadUnit := "kg"
						if units == "imperial" {
							loadVal = s.LoadKg * unitsconv.KgToLbs
							loadUnit = "lbs"
						}
						if ex.RPE > 0 {
							b.WriteString(fmt.Sprintf("    - Set %d: %dx @ %.1f %s, TUT: %.0fs, RPE: %.1f\n", i+1, s.Reps, loadVal, loadUnit, s.TUTSeconds, ex.RPE))
						} else {
							b.WriteString(fmt.Sprintf("    - Set %d: %dx @ %.1f %s, TUT: %.0fs\n", i+1, s.Reps, loadVal, loadUnit, s.TUTSeconds))
						}
					}
				}
				if ex.METValue > 0 {
					b.WriteString(fmt.Sprintf("    - MET: %.1f\n", ex.METValue))
				}
				if ex.TargetHRZone > 0 {
					b.WriteString(fmt.Sprintf("    - Target HR Zone: %d\n", ex.TargetHRZone))
				}
				if len(ex.HRZoneTime) > 0 {
					var parts []string
					for z := 1; z <= 5; z++ {
						if sec, ok := ex.HRZoneTime[z]; ok && sec > 0 {
							parts = append(parts, fmt.Sprintf("Zone %d: %.0fs", z, sec))
						}
					}
					if len(parts) > 0 {
						b.WriteString(fmt.Sprintf("    - HR Zone Time: %s\n", strings.Join(parts, ", ")))
					}
				}
				if ex.Notes != "" {
					b.WriteString(fmt.Sprintf("    - Notes: %s\n", ex.Notes))
				}
			}
		}
		if w.SessionDensity > 0 {
			b.WriteString(fmt.Sprintf("- Session density: %.1f\n", w.SessionDensity))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// heaviestLoadKg returns the heaviest individual set load in a workout. A
// workout row has no single "the load" value, so the peak set is the honest
// summary: it is the number a lifter recognises from the session, and it
// avoids inflating a 3x5 @ 135 lb session into a 405 lb "load".
func heaviestLoadKg(w models.WorkoutEntry) float64 {
	var max float64
	for _, ex := range w.Exercises {
		for _, s := range ex.Sets {
			if s.LoadKg > max {
				max = s.LoadKg
			}
		}
	}
	return max
}

// WorkoutsCSV outputs RFC 4180 CSV:
// date,slot,title,duration_min,calories_burned,mwv,nds,session_density,rpe,focus,load_<unit>
//
// The load column is the heaviest set in the workout, converted from canonical
// kilograms to the profile's units. An empty cell means no set carried load.
func WorkoutsCSV(workouts []models.WorkoutEntry, from, to, units string) string {
	var b strings.Builder
	b.WriteString(utf8BOM)
	writeWorkoutsCSVRows(&b, workouts, from, to, units)
	return b.String()
}

func writeWorkoutsCSVRows(b *strings.Builder, workouts []models.WorkoutEntry, from, to, units string) {
	loadHeader := "load_kg"
	if units == "imperial" {
		loadHeader = "load_lbs"
	}
	w := csv.NewWriter(b)
	w.UseCRLF = true
	_ = w.Write([]string{
		"date", "slot", "title", "duration_min", "calories_burned", "mwv", "nds",
		"session_density", "rpe", "focus", loadHeader,
	})
	for _, workout := range workouts {
		if workout.Date < from || workout.Date > to {
			continue
		}
		focus := ""
		if len(workout.Metadata.Focus) > 0 {
			focus = strings.Join(workout.Metadata.Focus, ";")
		}
		rpe := ""
		if workout.Metadata.RPE > 0 {
			rpe = fmt.Sprintf("%.1f", workout.Metadata.RPE)
		}
		load := ""
		if kg := heaviestLoadKg(workout); kg > 0 {
			if units == "imperial" {
				// Pounds are conventionally reported to one decimal, which
				// also rounds the canonical 61.23 kg example to 135.0 lb.
				load = fmt.Sprintf("%.1f", kg*unitsconv.KgToLbs)
			} else {
				// Keep the stored kilogram precision (61.23 kg stays 61.23)
				// instead of silently losing a digit to one-decimal rounding.
				load = fmt.Sprintf("%.2f", kg)
			}
		}
		_ = w.Write([]string{
			workout.Date,
			workout.Slot,
			workout.Title,
			fmt.Sprintf("%.0f", workout.DurationMin),
			fmt.Sprintf("%.0f", workout.CaloriesBurned),
			fmt.Sprintf("%.0f", workout.MWV),
			fmt.Sprintf("%.0f", workout.NDS),
			fmt.Sprintf("%.1f", workout.SessionDensity),
			rpe,
			focus,
			load,
		})
	}
	w.Flush()
}

// CombinedCSV renders the combined export as a docs-style document containing
// two CSV sections: a daily nutrition + body-metrics section followed by the
// workout section. Two tables cannot be one parseable RFC 4180 stream, so the
// sections are explicitly delimited with `#` comment lines instead of being
// silently concatenated. Each section is a valid CSV once its comment lines
// are removed; the first section remains the one Excel presents cleanly.
//
// The daily section is keyed by the union of nutrition and biometric dates.
// Nutrition cells are empty on body-only days, and weight/waist cells are
// empty when that measurement was not logged (stored 0 means "not logged").
// Weight/waist are converted to the profile's units.
func CombinedCSV(logs []models.NutritionLog, biometrics []models.BiometricLog, workouts []models.WorkoutEntry, from, to, units string) string {
	var b strings.Builder
	b.WriteString(utf8BOM)
	b.WriteString("# combined export\r\n")
	b.WriteString("# nutrition: daily nutrition + body metrics\r\n")
	writeCombinedDailyCSVRows(&b, logs, biometrics, from, to, units)
	b.WriteString("# workouts\r\n")
	writeWorkoutsCSVRows(&b, workouts, from, to, units)
	return b.String()
}

func writeCombinedDailyCSVRows(b *strings.Builder, logs []models.NutritionLog, biometrics []models.BiometricLog, from, to, units string) {
	type dailyRow struct {
		nut            models.NutritionLog
		bio            models.BiometricLog
		hasNut, hasBio bool
	}
	days := make(map[string]*dailyRow)
	get := func(date string) *dailyRow {
		row, ok := days[date]
		if !ok {
			row = &dailyRow{}
			days[date] = row
		}
		return row
	}
	for _, l := range logs {
		if l.Date < from || l.Date > to {
			continue
		}
		row := get(l.Date)
		row.nut = l
		row.hasNut = true
	}
	for _, m := range biometrics {
		if m.Date < from || m.Date > to {
			continue
		}
		row := get(m.Date)
		row.bio = m
		row.hasBio = true
	}

	dates := make([]string, 0, len(days))
	for d := range days {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	weightHeader, waistHeader := "weight_kg", "waist_cm"
	if units == "imperial" {
		weightHeader, waistHeader = "weight_lbs", "waist_in"
	}

	w := csv.NewWriter(b)
	w.UseCRLF = true
	_ = w.Write([]string{
		"date", "calories", "protein_g", "carbs_g", "fat_g", "fiber_g", "water_ml",
		"meal_notes", weightHeader, waistHeader,
	})
	for _, d := range dates {
		row := days[d]
		vals := make([]string, 10)
		vals[0] = d
		if row.hasNut {
			vals[1] = fmt.Sprintf("%.0f", row.nut.Calories)
			vals[2] = fmt.Sprintf("%.0f", row.nut.ProteinG)
			vals[3] = fmt.Sprintf("%.0f", row.nut.CarbsG)
			vals[4] = fmt.Sprintf("%.0f", row.nut.FatG)
			vals[5] = fmt.Sprintf("%.0f", row.nut.FiberG)
			vals[6] = fmt.Sprintf("%.0f", row.nut.WaterMl)
			vals[7] = strings.ReplaceAll(row.nut.MealNotes, "\n", " ")
		}
		if row.hasBio {
			if row.bio.WeightKg > 0 {
				v := row.bio.WeightKg
				if units == "imperial" {
					v *= unitsconv.KgToLbs
				}
				vals[8] = fmt.Sprintf("%.1f", v)
			}
			if row.bio.WaistCm > 0 {
				v := row.bio.WaistCm
				if units == "imperial" {
					v *= unitsconv.CmToInch
				}
				vals[9] = fmt.Sprintf("%.1f", v)
			}
		}
		_ = w.Write(vals)
	}
	w.Flush()
}

// CombinedMarkdown generates a single Markdown document combining nutrition + biometrics + workouts grouped by date
func CombinedMarkdown(logs []models.NutritionLog, biometrics []models.BiometricLog, workouts []models.WorkoutEntry, targets models.Targets, from, to, units string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Diet & Training Log: %s to %s\n\n", from, to))

	// build maps by date
	nutMap := make(map[string]models.NutritionLog)
	for _, n := range logs {
		if n.Date < from || n.Date > to {
			continue
		}
		nutMap[n.Date] = n
	}
	bioMap := make(map[string]models.BiometricLog)
	for _, m := range biometrics {
		if m.Date < from || m.Date > to {
			continue
		}
		bioMap[m.Date] = m
	}
	workMap := make(map[string][]models.WorkoutEntry)
	for _, w := range workouts {
		if w.Date < from || w.Date > to {
			continue
		}
		workMap[w.Date] = append(workMap[w.Date], w)
	}

	// collect dates
	datesMap := make(map[string]struct{})
	for d := range nutMap {
		datesMap[d] = struct{}{}
	}
	for d := range bioMap {
		datesMap[d] = struct{}{}
	}
	for d := range workMap {
		datesMap[d] = struct{}{}
	}
	if len(datesMap) == 0 {
		b.WriteString("No data in this range.\n")
		return b.String()
	}
	// build sorted slice of dates
	var dates []string
	for d := range datesMap {
		dates = append(dates, d)
	}
	// Lexicographic sort is chronological for YYYY-MM-DD.
	sort.Strings(dates)

	for _, d := range dates {
		b.WriteString(fmt.Sprintf("## %s\n\n", d))
		if bio, ok := bioMap[d]; ok {
			// display canonical stored metric values, convert for imperial if requested
			weightVal := bio.WeightKg
			weightUnit := "kg"
			waistVal := bio.WaistCm
			waistUnit := "cm"
			if units == "imperial" {
				weightVal = bio.WeightKg * unitsconv.KgToLbs
				weightUnit = "lbs"
				waistVal = bio.WaistCm * unitsconv.CmToInch
				waistUnit = "in"
			}
			// 0 means "not logged" for both fields. Never render a fake
			// "Weight: 0.0 lbs"; a measurement-only merge row should show only
			// the fields it actually carries.
			if bio.WeightKg > 0 {
				b.WriteString(fmt.Sprintf("- Weight: %.1f %s\n", weightVal, weightUnit))
			}
			// Waist may be tracked in body_measurements rather than biometric_logs.
			if bio.WaistCm > 0 {
				b.WriteString(fmt.Sprintf("- Waist: %.1f %s\n", waistVal, waistUnit))
			}
			// additional biometric fields
			if bio.GripKg > 0 {
				gripVal := bio.GripKg
				gripUnit := "kg"
				if units == "imperial" {
					gripVal = bio.GripKg * unitsconv.KgToLbs
					gripUnit = "lbs"
				}
				b.WriteString(fmt.Sprintf("- Grip: %.1f %s\n", gripVal, gripUnit))
			}
			if bio.BoltScore > 0 {
				b.WriteString(fmt.Sprintf("- BOLT: %.0f\n", bio.BoltScore))
			}
			if bio.BodyFatPct > 0 {
				b.WriteString(fmt.Sprintf("- Body Fat: %.1f%%\n", bio.BodyFatPct))
			}
			if bio.SleepHours > 0 {
				b.WriteString(fmt.Sprintf("- Sleep: %.1f hrs (quality: %.1f)\n", bio.SleepHours, bio.SleepQuality))
			}
			if bio.SubjectiveFeel != 0 {
				b.WriteString(fmt.Sprintf("- Feel: %d/10\n", bio.SubjectiveFeel))
			}
			if bio.Notes != "" {
				b.WriteString(fmt.Sprintf("- Notes: %s\n", bio.Notes))
			}
		}
		if nut, ok := nutMap[d]; ok {
			writeNutritionDayRow(&b, nut, nutritionBulletRow)
		}
		if ws, ok := workMap[d]; ok {
			for _, w := range ws {
				b.WriteString(fmt.Sprintf("- Workout: %s (%s) — %.0f min, calories: %.0f, MWV: %.0f, NDS: %.0f\n", w.Title, w.Slot, w.DurationMin, w.CaloriesBurned, w.MWV, w.NDS))
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}
