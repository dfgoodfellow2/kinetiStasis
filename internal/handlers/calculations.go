package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/auth"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/constants"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/respond"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/services/bodyfat"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/services/calculator"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/services/metrics"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/services/nutrition"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/services/readiness"
)

type CalcHandler struct{ db *sql.DB }

func NewCalcHandler(db *sql.DB) *CalcHandler { return &CalcHandler{db: db} }

// --- helpers ---
func fetchProfile(ctx context.Context, db *sql.DB, userID string) (models.Profile, error) {
	var p models.Profile
	err := db.QueryRowContext(ctx, `
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

func fetchLatestWeight(ctx context.Context, db *sql.DB, userID string) float64 {
	var w float64
	err := db.QueryRowContext(ctx, `SELECT weight_kg FROM biometric_logs WHERE user_id=? AND weight_kg > 0 ORDER BY date DESC LIMIT 1`, userID).Scan(&w)
	if err != nil {
		return 0
	}
	return w
}

func fetchNutritionLogs(ctx context.Context, db *sql.DB, userID, since string) ([]models.NutritionLog, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, user_id, date, calories, protein_g, carbs_g, fat_g, fiber_g, water_ml, COALESCE(meal_notes,''), updated_at FROM nutrition_logs WHERE user_id=? AND date >= ? ORDER BY date ASC`, userID, since)
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

func fetchBiometricLogs(ctx context.Context, db *sql.DB, userID, since string) ([]models.BiometricLog, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, user_id, date, COALESCE(weight_kg,0), COALESCE(waist_cm,0), COALESCE(grip_kg,0), COALESCE(bolt_score,0), COALESCE(sleep_hours,0), COALESCE(sleep_quality,0), COALESCE(subjective_feel,0), COALESCE(body_fat_pct,0), COALESCE(notes,''), updated_at FROM biometric_logs WHERE user_id=? AND date >= ? ORDER BY date ASC`, userID, since)
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

func fetchWorkouts(ctx context.Context, db *sql.DB, userID, since string) ([]models.WorkoutEntry, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, user_id, date, slot, COALESCE(title,''), COALESCE(raw_notes,''), COALESCE(duration_min,0), COALESCE(calories_burned,0), COALESCE(mwv,0), COALESCE(nds,0), COALESCE(session_density,0), COALESCE(exercises_json,'[]'), COALESCE(metadata_json,'{}'), updated_at FROM workouts WHERE user_id=? AND date >= ? ORDER BY date ASC`, userID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.WorkoutEntry
	for rows.Next() {
		var w models.WorkoutEntry
		var exercisesJSON, metadataJSON string
		if err := rows.Scan(&w.ID, &w.UserID, &w.Date, &w.Slot, &w.Title, &w.RawNotes, &w.DurationMin, &w.CaloriesBurned, &w.MWV, &w.NDS, &w.SessionDensity, &exercisesJSON, &metadataJSON, &w.UpdatedAt); err != nil {
			return nil, err
		}
		if exercisesJSON != "" && exercisesJSON != "[]" {
			_ = json.Unmarshal([]byte(exercisesJSON), &w.Exercises)
		}
		if metadataJSON != "" && metadataJSON != "{}" {
			_ = json.Unmarshal([]byte(metadataJSON), &w.Metadata)
		}
		out = append(out, w)
	}
	return out, nil
}

func fetchTargets(ctx context.Context, db *sql.DB, userID string) (models.Targets, error) {
	var t models.Targets
	var eatBack int
	err := db.QueryRowContext(ctx, `SELECT user_id, calories, protein_g, carbs_g, fat_g, fiber_g, water_ml, COALESCE(eat_back_exercise,0), updated_at FROM targets WHERE user_id=?`, userID).Scan(&t.UserID, &t.Calories, &t.ProteinG, &t.CarbsG, &t.FatG, &t.FiberG, &t.WaterMl, &eatBack, &t.UpdatedAt)
	if err != nil {
		return t, err
	}
	t.EatBackExercise = eatBack == 1
	return t, nil
}

// --- handlers ---
// GET /v1/calc/tdee
func (h *CalcHandler) TDEE(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	daysStr := r.URL.Query().Get("days")
	// fetch profile
	profile, err := fetchProfile(r.Context(), h.db, claims.UserID)
	if err == sql.ErrNoRows {
		// New user with no profile yet — return an empty dashboard rather than 404.
		respond.JSON(w, http.StatusOK, models.DashboardData{})
		return
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}

	lookback := profile.TDEELookbackDays
	if daysStr != "" {
		if v, err := strconv.Atoi(daysStr); err == nil && v > 0 {
			lookback = v
		}
	}
	if lookback <= 0 {
		lookback = constants.DefaultTDEELookbackDays
	}

	since := time.Now().UTC().AddDate(0, 0, -lookback).Format(constants.DateFormat)
	nutLogs, _ := fetchNutritionLogs(r.Context(), h.db, claims.UserID, since)
	bioLogs, _ := fetchBiometricLogs(r.Context(), h.db, claims.UserID, since)

	res := calculator.ComputeObservedTDEE(nutLogs, bioLogs, profile)
	respond.JSON(w, http.StatusOK, res)
}

// GET /v1/calc/macros
func (h *CalcHandler) Macros(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	profile, err := fetchProfile(r.Context(), h.db, claims.UserID)
	if err == sql.ErrNoRows {
		respond.Error(w, http.StatusNotFound, "profile not found")
		return
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}

	weight := fetchLatestWeight(r.Context(), h.db, claims.UserID)
	nutLogs, _ := fetchNutritionLogs(r.Context(), h.db, claims.UserID, time.Now().UTC().AddDate(0, 0, -constants.DefaultTDEELookbackDays).Format(constants.DateFormat))
	bioLogs, _ := fetchBiometricLogs(r.Context(), h.db, claims.UserID, time.Now().UTC().AddDate(0, 0, -constants.DefaultTDEELookbackDays).Format(constants.DateFormat))
	observed := calculator.ComputeObservedTDEE(nutLogs, bioLogs, profile)
	plan := nutrition.FullDietPlan(profile, weight, observed.ObservedTDEE)
	respond.JSON(w, http.StatusOK, plan)
}

// GET /v1/calc/readiness
func (h *CalcHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	profile, err := fetchProfile(r.Context(), h.db, claims.UserID)
	if err == sql.ErrNoRows {
		respond.Error(w, http.StatusNotFound, "profile not found")
		return
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	bioLogs, _ := fetchBiometricLogs(r.Context(), h.db, claims.UserID, time.Now().UTC().AddDate(0, 0, -constants.DefaultReadinessLookbackDays).Format(constants.DateFormat))
	res := readiness.ComputeReadiness(bioLogs, profile)
	respond.JSON(w, http.StatusOK, res)
}

// GET /v1/calc/bodyfat
func (h *CalcHandler) BodyFat(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	method := r.URL.Query().Get("method")
	if method == "" {
		method = "navy"
	}
	profile, err := fetchProfile(r.Context(), h.db, claims.UserID)
	if err == sql.ErrNoRows {
		respond.Error(w, http.StatusNotFound, "profile not found")
		return
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	weight := fetchLatestWeight(r.Context(), h.db, claims.UserID)

	// most recent body measurements
	var neckCm, waistCm, hipsCm float64
	_ = h.db.QueryRowContext(r.Context(), `SELECT COALESCE(neck_cm,0), COALESCE(waist_cm,0), COALESCE(hips_cm,0) FROM body_measurements WHERE user_id=? ORDER BY date DESC LIMIT 1`, claims.UserID).Scan(&neckCm, &waistCm, &hipsCm)

	switch method {
	case "navy":
		out := bodyfat.NavyMethod(profile, weight, neckCm, waistCm, hipsCm)
		respond.JSON(w, http.StatusOK, out)
		return
	case "skinfold":
		respond.Error(w, http.StatusUnprocessableEntity, "skinfold requires POST with measurement values")
		return
	default:
		respond.Error(w, http.StatusBadRequest, "unknown method")
		return
	}
}

// GET /v1/dashboard
func (h *CalcHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	profile, err := fetchProfile(r.Context(), h.db, claims.UserID)
	if err == sql.ErrNoRows {
		respond.Error(w, http.StatusNotFound, "profile not found")
		return
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}

	weight := fetchLatestWeight(r.Context(), h.db, claims.UserID)

	// Use client-supplied local date if valid (avoids UTC timezone mismatch).
	// Frontend passes ?date=YYYY-MM-DD reflecting the user's local calendar day.
	today := r.URL.Query().Get("date")
	if len(today) != 10 {
		today = time.Now().UTC().Format(constants.DateFormat)
	} else if _, parseErr := time.Parse(constants.DateFormat, today); parseErr != nil {
		today = time.Now().UTC().Format(constants.DateFormat)
	}

	// Derive lookback windows relative to the resolved today
	todayTime, _ := time.Parse(constants.DateFormat, today)
	since7 := todayTime.AddDate(0, 0, -7).Format(constants.DateFormat)
	since30 := todayTime.AddDate(0, 0, -constants.DefaultReadinessLookbackDays).Format(constants.DateFormat)
	since90 := todayTime.AddDate(0, 0, -constants.DefaultTDEELookbackDays).Format(constants.DateFormat)

	nut90, _ := fetchNutritionLogs(r.Context(), h.db, claims.UserID, since90)
	bio30, _ := fetchBiometricLogs(r.Context(), h.db, claims.UserID, since30)
	workouts7, _ := fetchWorkouts(r.Context(), h.db, claims.UserID, since7)
	workouts30, _ := fetchWorkouts(r.Context(), h.db, claims.UserID, since30)

	targets, terr := fetchTargets(r.Context(), h.db, claims.UserID)
	if terr != nil {
		targets = models.Targets{}
	}

	tdeeRes := calculator.ComputeObservedTDEE(nut90, bio30, profile)
	macros := nutrition.FullDietPlan(profile, weight, tdeeRes.ObservedTDEE)
	readinessRes := readiness.ComputeReadiness(bio30, profile)
	_, _, velDelta, velArrow := readiness.ComputeReadinessVelocity(bio30, profile)
	velTrend := "stable"
	if velArrow == "↑" {
		velTrend = "improving"
	} else if velArrow == "↓" {
		velTrend = "declining"
	}
	readinessRes.VelocityTrend = velTrend
	readinessRes.VelocityDelta = velDelta

	// weekly stats: filter to last 7 days
	var nut7 []models.NutritionLog
	for _, n := range nut90 {
		if n.Date >= since7 {
			nut7 = append(nut7, n)
		}
	}
	// Filter to last 30 days for streak calculation
	var nut30 []models.NutritionLog
	for _, n := range nut90 {
		if n.Date >= since30 {
			nut30 = append(nut30, n)
		}
	}
	var bio7 []models.BiometricLog
	for _, b := range bio30 {
		if b.Date >= since7 {
			bio7 = append(bio7, b)
		}
	}
	var workouts7filtered []models.WorkoutEntry
	for _, w := range workouts7 {
		if w.Date >= since7 {
			workouts7filtered = append(workouts7filtered, w)
		}
	}
	weekly := metrics.WeeklyStats(nut30, bio30, workouts30, today)
	todaySummary := metrics.TodaySummary(today, nut90, targets)

	// weight trend from bio30
	var weightTrend []struct {
		Date     string  `json:"date"`
		WeightKg float64 `json:"weight_kg"`
	}
	for _, b := range bio30 {
		if b.WeightKg > 0 {
			weightTrend = append(weightTrend, struct {
				Date     string  `json:"date"`
				WeightKg float64 `json:"weight_kg"`
			}{Date: b.Date, WeightKg: b.WeightKg})
		}
	}

	// Find today's biometric entry (if logged)
	var todayBio *models.BiometricLog
	for i := range bio30 {
		if bio30[i].Date == today {
			b := bio30[i]
			todayBio = &b
			break
		}
	}

	// Grip PB: max grip in last 120 days
	var gripPB float64
	gripRow := h.db.QueryRowContext(r.Context(), `SELECT COALESCE(MAX(grip_kg),0) FROM biometric_logs WHERE user_id = ? AND grip_kg > 0 AND date >= DATE_SUB(?, INTERVAL 120 DAY)`, claims.UserID, today)
	gripRow.Scan(&gripPB)

	// Determine if a workout was logged today
	workoutToday := false
	for _, w := range workouts7 {
		if w.Date == today {
			workoutToday = true
			break
		}
	}

	dash := models.DashboardData{
		Today:            todaySummary,
		TDEE:             tdeeRes,
		Macros:           macros,
		Readiness:        readinessRes,
		WeeklyStats:      weekly,
		WeightTrend:      weightTrend,
		TodayBio:         todayBio,
		GripPersonalBest: gripPB,
		WorkoutToday:     workoutToday,
	}
	respond.JSON(w, http.StatusOK, dash)
}
