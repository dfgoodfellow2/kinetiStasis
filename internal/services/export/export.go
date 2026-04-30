package export

import (
	"fmt"
	"strings"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	unitsconv "github.com/dfgoodfellow2/diet-tracker/v2/internal/services/units"
)

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

		notes := strings.ReplaceAll(l.MealNotes, "\n", " ")
		// escape pipe characters
		notes = strings.ReplaceAll(notes, "|", "\\|")

		b.WriteString(fmt.Sprintf("| %s | %.0f | %.0f | %.0f | %.0f | %.0f | %.0f | %s |\n",
			l.Date, l.Calories, l.ProteinG, l.CarbsG, l.FatG, l.FiberG, l.WaterMl, notes))
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

// NutritionCSV outputs CSV: date,calories,protein_g,carbs_g,fat_g,fiber_g,water_ml,meal_notes
func NutritionCSV(logs []models.NutritionLog, from, to string) string {
	var b strings.Builder
	b.WriteString("date,calories,protein_g,carbs_g,fat_g,fiber_g,water_ml,meal_notes\n")
	for _, l := range logs {
		if l.Date < from || l.Date > to {
			continue
		}
		notes := strings.ReplaceAll(l.MealNotes, "\n", " ")
		// quote notes
		notes = fmt.Sprintf("%q", notes)
		b.WriteString(fmt.Sprintf("%s,%.0f,%.0f,%.0f,%.0f,%.0f,%.0f,%s\n",
			l.Date, l.Calories, l.ProteinG, l.CarbsG, l.FatG, l.FiberG, l.WaterMl, notes))
	}
	return b.String()
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
				b.WriteString(fmt.Sprintf("  - %s\n", ex.Name))
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

// WorkoutsCSV: date,slot,title,duration_min,calories_burned,mwv,nds,session_density
func WorkoutsCSV(workouts []models.WorkoutEntry, from, to, units string) string {
	var b strings.Builder
	// include load column header with appropriate units
	loadHeader := "load_kg"
	if units == "imperial" {
		loadHeader = "load_lbs"
	}
	// include rpe and focus columns
	b.WriteString(fmt.Sprintf("date,slot,title,duration_min,calories_burned,mwv,nds,session_density,rpe,focus,%s\n", loadHeader))
	for _, w := range workouts {
		if w.Date < from || w.Date > to {
			continue
		}
		// for CSV include a single load value per workout row (sum of loads? keep simple: leave blank)
		// Historically this CSV didn't include set-level loads; we keep prior behavior but include header.
		focus := ""
		if len(w.Metadata.Focus) > 0 {
			focus = strings.Join(w.Metadata.Focus, ";")
		}
		rpeStr := ""
		if w.Metadata.RPE > 0 {
			rpeStr = fmt.Sprintf("%.1f", w.Metadata.RPE)
		}
		b.WriteString(fmt.Sprintf("%s,%s,%q,%.0f,%.0f,%.0f,%.0f,%.1f,%s,%q,%q\n",
			w.Date, w.Slot, w.Title, w.DurationMin, w.CaloriesBurned, w.MWV, w.NDS, w.SessionDensity, rpeStr, focus, ""))
	}
	return b.String()
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
	// simple lexicographic (YYYY-MM-DD) sorts correctly
	// sort
	// Importing sort would be extra; use bubble? but small; simpler: convert to slice and use built-in sort
	// We'll import sort at top if needed; but to avoid changing imports again, do simple insertion sort
	for i := 1; i < len(dates); i++ {
		key := dates[i]
		j := i - 1
		for j >= 0 && dates[j] > key {
			dates[j+1] = dates[j]
			j--
		}
		dates[j+1] = key
	}

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
			b.WriteString(fmt.Sprintf("- Weight: %.1f %s\n", weightVal, weightUnit))
			// Waist may be tracked in body_measurements rather than biometric_logs;
			// waistVal will be 0 if not present in biometric_logs.
			b.WriteString(fmt.Sprintf("- Waist: %.1f %s\n", waistVal, waistUnit))
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
			b.WriteString(fmt.Sprintf("- Calories: %.0f (P: %.0f g, C: %.0f g, F: %.0f g)\n", nut.Calories, nut.ProteinG, nut.CarbsG, nut.FatG))
			if nut.WaterMl > 0 {
				b.WriteString(fmt.Sprintf("- Water: %.0f ml\n", nut.WaterMl))
			}
			if nut.MealNotes != "" {
				b.WriteString(fmt.Sprintf("- Meals: %s\n", nut.MealNotes))
			}
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
