package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/constants"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/db"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/services/calculator"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/services/nutrition"
)

func fetchProfile(ctxDB *sql.DB, userID string) (models.Profile, error) {
	var p models.Profile
	err := ctxDB.QueryRow(`
        SELECT user_id, COALESCE(name,''), COALESCE(age,0), COALESCE(sex,''),
               COALESCE(height_cm,0), COALESCE(activity,'sedentary'), COALESCE(exercise_freq,0),
               COALESCE(running_km,0), COALESCE(is_lifter,0), COALESCE(goal,'maintenance'),
               COALESCE(prioritize_carbs,0), COALESCE(bf_pct,0), COALESCE(hr_rest,0),
               COALESCE(hr_max,0), COALESCE(grip_weight,0.5), COALESCE(tdee_lookback_days,90),
               COALESCE(sleep_quality_max,10.0), COALESCE(units,'imperial'), updated_at
        FROM profiles WHERE user_id = ?`, userID,
	).Scan(&p.UserID, &p.Name, &p.Age, &p.Sex, &p.HeightCm, &p.Activity, &p.ExerciseFreq,
		&p.RunningKm, &p.IsLifter, &p.Goal, &p.PrioritizeCarbs, &p.BfPct, &p.HRRest,
		&p.HRMax, &p.GripWeight, &p.TDEELookbackDays, &p.SleepQualityMax, &p.Units, &p.UpdatedAt)
	return p, err
}

func fetchLatestWeight(ctxDB *sql.DB, userID string) float64 {
	var w float64
	err := ctxDB.QueryRow(`SELECT weight_kg FROM biometric_logs WHERE user_id=? AND weight_kg > 0 ORDER BY date DESC LIMIT 1`, userID).Scan(&w)
	if err != nil {
		return 0
	}
	return w
}

func fetchNutritionLogs(ctxDB *sql.DB, userID, since string) ([]models.NutritionLog, error) {
	rows, err := ctxDB.Query(`SELECT id, user_id, date, calories, protein_g, carbs_g, fat_g, fiber_g, water_ml, COALESCE(meal_notes,''), updated_at FROM nutrition_logs WHERE user_id=? AND date >= ? ORDER BY date ASC`, userID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.NutritionLog
	for rows.Next() {
		var n models.NutritionLog
		if err := rows.Scan(&n.ID, &n.UserID, &n.Date, &n.Calories, &n.ProteinG, &n.CarbsG, &n.FatG, &n.FiberG, &n.WaterMl, &n.MealNotes, &n.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

func fetchBiometricLogs(ctxDB *sql.DB, userID, since string) ([]models.BiometricLog, error) {
	rows, err := ctxDB.Query(`SELECT id, user_id, date, COALESCE(weight_kg,0), COALESCE(waist_cm,0), COALESCE(grip_kg,0), COALESCE(bolt_score,0), COALESCE(sleep_hours,0), COALESCE(sleep_quality,0), COALESCE(subjective_feel,0), COALESCE(body_fat_pct,0), COALESCE(notes,''), updated_at FROM biometric_logs WHERE user_id=? AND date >= ? ORDER BY date ASC`, userID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.BiometricLog
	for rows.Next() {
		var b models.BiometricLog
		if err := rows.Scan(&b.ID, &b.UserID, &b.Date, &b.WeightKg, &b.WaistCm, &b.GripKg, &b.BoltScore, &b.SleepHours, &b.SleepQuality, &b.SubjectiveFeel, &b.BodyFatPct, &b.Notes, &b.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, nil
}

func main() {
	uid := flag.String("user", "9a4629f8-2f76-4850-83d3-fc233594ad7f", "user id to fix targets for")
	dbPath := flag.String("db", "./data/diet.db", "path to sqlite db")
	flag.Parse()

	conn, err := db.Open(*dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	profile, err := fetchProfile(conn, *uid)
	if err != nil {
		log.Fatalf("fetch profile: %v", err)
	}

	weight := fetchLatestWeight(conn, *uid)
	if weight <= 0 {
		log.Printf("warning: no latest weight found, defaulting to 0 (will use profile BMR if applicable)")
	}

	since := time.Now().UTC().AddDate(0, 0, -constants.DefaultTDEELookbackDays).Format(constants.DateFormat)
	nutLogs, err := fetchNutritionLogs(conn, *uid, since)
	if err != nil {
		log.Fatalf("fetch nutrition logs: %v", err)
	}
	bioLogs, err := fetchBiometricLogs(conn, *uid, since)
	if err != nil {
		log.Fatalf("fetch biometric logs: %v", err)
	}

	tdeeRes := calculator.ComputeObservedTDEE(nutLogs, bioLogs, profile)

	// If observed TDEE looks clearly wrong (very low or too few days), prefer static
	useObserved := true
	if tdeeRes.DaysOfData < 14 || tdeeRes.ObservedTDEE < 1000 {
		useObserved = false
	}

	var plan models.MacroResult
	if useObserved {
		plan = nutrition.FullDietPlan(profile, weight, tdeeRes.ObservedTDEE)
	} else {
		// pass 0 to FullDietPlan so it falls back to static TDEE only
		log.Printf("ignoring observed TDEE (days=%d observed=%.0f), using static TDEE instead", tdeeRes.DaysOfData, tdeeRes.ObservedTDEE)
		plan = nutrition.FullDietPlan(profile, weight, 0)
	}

	// Ensure MinCalorieFloor enforced
	if plan.Calories < constants.MinCalorieFloor {
		log.Printf("Computed calories %v below MinCalorieFloor %v - enforcing floor", plan.Calories, constants.MinCalorieFloor)
		plan.Calories = constants.MinCalorieFloor
		// Recompute macros with the floored calorie value
		plan = nutrition.ComputeMacros(profile, plan.Calories, weight)
	}

	fmt.Printf("Observed TDEE: %+v\n", tdeeRes)
	fmt.Printf("Computed plan: calories=%.0f protein=%.0f carbs=%.0f fat=%.0f fiber=%.0f water=%.0f\n",
		plan.Calories, plan.ProteinG, plan.CarbsG, plan.FatG, plan.FiberG, plan.WaterMl)

	// Basic sanity check
	if plan.Calories < 1200 {
		log.Fatalf("sanity check failed: final calorie target %.0f is below 1200", plan.Calories)
	}

	// Update targets table
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := conn.Exec(`UPDATE targets SET calories=?, protein_g=?, carbs_g=?, fat_g=?, updated_at=? WHERE user_id=?`, plan.Calories, plan.ProteinG, plan.CarbsG, plan.FatG, now, *uid)
	if err != nil {
		log.Fatalf("update targets: %v", err)
	}
	ra, _ := res.RowsAffected()
	if ra == 0 {
		// insert fallback
		_, err := conn.Exec(`INSERT INTO targets (user_id, calories, protein_g, carbs_g, fat_g, fiber_g, water_ml, eat_back_exercise, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?)`, *uid, plan.Calories, plan.ProteinG, plan.CarbsG, plan.FatG, plan.FiberG, plan.WaterMl, now)
		if err != nil {
			log.Fatalf("insert targets: %v", err)
		}
		fmt.Println("Inserted new targets row")
	} else {
		fmt.Printf("Updated targets rows: %d\n", ra)
	}

	// Verify
	var c, p, ca, f float64
	var u string
	err = conn.QueryRow(`SELECT calories, protein_g, carbs_g, fat_g, updated_at FROM targets WHERE user_id=?`, *uid).Scan(&c, &p, &ca, &f, &u)
	if err != nil {
		log.Fatalf("verify read targets: %v", err)
	}
	fmt.Printf("New targets in DB: calories=%.0f protein=%.0f carbs=%.0f fat=%.0f updated_at=%s\n", c, p, ca, f, u)
}
