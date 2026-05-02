package handlers

import (
	"database/sql"
	"log/slog"
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
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"
)

type CalcHandler struct{ s store.Store }

func NewCalcHandler(s store.Store) *CalcHandler { return &CalcHandler{s: s} }

// GET /v1/calc/tdee
func (h *CalcHandler) TDEE(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	daysStr := r.URL.Query().Get("days")
	// fetch profile
	profile, err := h.s.FetchProfile(r.Context(), claims.UserID)
	if err != nil {
		if err == sql.ErrNoRows {
			// New user with no profile yet — return an empty dashboard rather than 404.
			respond.JSON(w, http.StatusOK, models.DashboardData{})
			return
		}
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
	nutLogs, err := h.s.FetchNutritionLogs(r.Context(), claims.UserID, since)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	bioLogs, err := h.s.FetchBiometricLogs(r.Context(), claims.UserID, since)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}

	res := calculator.ComputeObservedTDEE(nutLogs, bioLogs, profile)
	respond.JSON(w, http.StatusOK, res)
}

// GET /v1/calc/macros
func (h *CalcHandler) Macros(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	profile, err := h.s.FetchProfile(r.Context(), claims.UserID)
	if err == sql.ErrNoRows {
		respond.Error(w, http.StatusNotFound, "profile not found")
		return
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}

	weight, err := h.s.FetchLatestWeight(r.Context(), claims.UserID)
	if err != nil && err != sql.ErrNoRows {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}

	nutLogs, err := h.s.FetchNutritionLogs(r.Context(), claims.UserID, time.Now().UTC().AddDate(0, 0, -constants.DefaultTDEELookbackDays).Format(constants.DateFormat))
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	bioLogs, err := h.s.FetchBiometricLogs(r.Context(), claims.UserID, time.Now().UTC().AddDate(0, 0, -constants.DefaultTDEELookbackDays).Format(constants.DateFormat))
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	observed := calculator.ComputeObservedTDEE(nutLogs, bioLogs, profile)
	plan := nutrition.FullDietPlan(profile, weight, observed.ObservedTDEE)
	respond.JSON(w, http.StatusOK, plan)
}

// GET /v1/calc/readiness
func (h *CalcHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	profile, err := h.s.FetchProfile(r.Context(), claims.UserID)
	if err == sql.ErrNoRows {
		respond.Error(w, http.StatusNotFound, "profile not found")
		return
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	bioLogs, err := h.s.FetchBiometricLogs(r.Context(), claims.UserID, time.Now().UTC().AddDate(0, 0, -constants.DefaultReadinessLookbackDays).Format(constants.DateFormat))
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
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
	profile, err := h.s.FetchProfile(r.Context(), claims.UserID)
	if err == sql.ErrNoRows {
		respond.Error(w, http.StatusNotFound, "profile not found")
		return
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	weight, err := h.s.FetchLatestWeight(r.Context(), claims.UserID)
	if err != nil && err != sql.ErrNoRows {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}

	// most recent body measurements
	neckCm, waistCm, hipsCm, err := h.s.FetchBodyMeasurements(r.Context(), claims.UserID)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}

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
	profile, err := h.s.FetchProfile(r.Context(), claims.UserID)
	if err == sql.ErrNoRows {
		respond.Error(w, http.StatusNotFound, "profile not found")
		return
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}

	weight, err := h.s.FetchLatestWeight(r.Context(), claims.UserID)
	if err != nil && err != sql.ErrNoRows {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}

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

	nut90, err := h.s.FetchNutritionLogs(r.Context(), claims.UserID, since90)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	bio30, err := h.s.FetchBiometricLogs(r.Context(), claims.UserID, since30)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	workouts7, err := h.s.FetchWorkouts(r.Context(), claims.UserID, since7)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	workouts30, err := h.s.FetchWorkouts(r.Context(), claims.UserID, since30)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}

	tdeeRes := calculator.ComputeObservedTDEE(nut90, bio30, profile)
	macros := nutrition.FullDietPlan(profile, weight, tdeeRes.ObservedTDEE)

	targets, terr := h.s.FetchTargets(r.Context(), claims.UserID)
	if terr != nil {
		// If targets not found in DB (or any error), fall back to computed macros
		// so the dashboard always returns consistent targets/macros.
		targets = models.Targets{
			UserID:   claims.UserID,
			Calories: macros.Calories,
			ProteinG: macros.ProteinG,
			CarbsG:   macros.CarbsG,
			FatG:     macros.FatG,
			FiberG:   macros.FiberG,
			WaterMl:  macros.WaterMl,
			// EatBackExercise defaults to false when not stored
			UpdatedAt: "",
		}
	}
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
		WeightKg float64 `json:"weightKg"`
	}
	for _, b := range bio30 {
		if b.WeightKg > 0 {
			weightTrend = append(weightTrend, struct {
				Date     string  `json:"date"`
				WeightKg float64 `json:"weightKg"`
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

	// Grip PB: max grip in last 120 days (SQLite date syntax)
	var gripPB float64
	if pb, err := h.s.FetchGripPB(r.Context(), claims.UserID, today); err != nil {
		slog.Warn("failed to fetch grip PB", "err", err)
	} else {
		gripPB = pb
	}

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

	// include check-in readiness (last checkin)
	if lastCheckInDate, err := h.s.FetchLastCheckin(r.Context(), claims.UserID); err == nil {
		if lastCheckInDate != "" {
			t, _ := time.Parse(constants.DateFormat, lastCheckInDate)
			days := int(time.Since(t).Hours() / 24)
			can := days >= 5
			dash.CanChangeTargets = can
			if !can {
				dash.DaysUntilCheckin = 5 - days
			}
		} else {
			dash.CanChangeTargets = true
		}
	} else {
		slog.Warn("failed to fetch last checkin", "err", err)
		dash.CanChangeTargets = true
	}
	respond.JSON(w, http.StatusOK, dash)
}
