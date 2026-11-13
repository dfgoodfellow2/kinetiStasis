package handlers

import (
	"database/sql"
	"errors"
	"log/slog"
	"math"
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

// requestDate resolves the client's local calendar day from the ?date query
// parameter (YYYY-MM-DD). A missing or malformed value falls back to the
// server's local day. Date-only strings carry no timezone, so the resolved
// value can be threaded into rolling windows without UTC/local drift.
func requestDate(r *http.Request) string {
	if d := r.URL.Query().Get("date"); d != "" {
		if _, err := time.Parse(constants.DateFormat, d); err == nil {
			return d
		}
	}
	return time.Now().Format(constants.DateFormat)
}

// weightPoint mirrors the element type of models.DashboardData.WeightTrend.
// Declaring it as an alias keeps the helper's return type assignable to the
// anonymous struct slice the model uses.
type weightPoint = struct {
	Date     string  `json:"date"`
	WeightKg float64 `json:"weightKg"`
}

// buildWeightTrendSeries emits the dashboard's raw display points and the
// engine trend levels in a single pass over one source: the deduplicated
// observation list the headline regression already uses (WeightSeriesForWindow).
//
// Both slices are appended together inside the same loop, so they are the same
// length and index-aligned by construction. The previous two-path projection
// (raw bio30 rows keyed by date onto a separately built trend map) could
// silently leave nils wherever a raw date had been collapsed or fell outside
// the trend window. Dates older than the display window (since30) are skipped;
// the trend levels themselves are still fitted over the full sinceTrend window,
// preserving the headline contract. Levels before MinWeightTrendDays distinct
// observations remain nil (burn-in), and the final non-nil level equals the
// headline trend by construction.
func buildWeightTrendSeries(bioAll []models.BiometricLog, since30, sinceTrend string) ([]weightPoint, []*float64) {
	weights, dates := calculator.WeightSeriesForWindow(bioAll, sinceTrend)
	levels := calculator.RollingWeightTrendLevels(weights, dates)

	points := make([]weightPoint, 0, len(dates))
	series := make([]*float64, 0, len(dates))
	for i, date := range dates {
		if date < since30 {
			continue
		}
		points = append(points, weightPoint{Date: date, WeightKg: weights[i]})
		series = append(series, levels[i])
	}
	return points, series
}

// daysSinceLocalDate returns the whole calendar-day distance from a
// date-only string to today's local date. Both sides are projected onto local
// midnights and then compared in UTC, so a UTC-midnight parse can no longer
// shift the check-in cooldown by the caller's offset (and DST length changes
// cannot truncate a day).
func daysSinceLocalDate(date string) (int, bool) {
	return daysSinceLocalDateAt(time.Now(), date)
}

// daysSinceLocalDateAt is the deterministic core of daysSinceLocalDate. Tests
// can pin now to the end of a local day (or across a DST boundary) without
// sleeping until midnight.
func daysSinceLocalDateAt(now time.Time, date string) (int, bool) {
	d, err := time.ParseInLocation(constants.DateFormat, date, now.Location())
	if err != nil {
		return 0, false
	}
	from := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
	to := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return int(to.Sub(from).Hours() / 24), true
}

// GET /v1/calc/tdee
func (h *CalcHandler) TDEE(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	daysStr := r.URL.Query().Get("days")
	// fetch profile
	profile, err := h.s.FetchProfile(r.Context(), claims.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// New user with no profile yet — return an empty dashboard rather than 404.
			respond.JSON(w, http.StatusOK, models.DashboardData{})
			return
		}
		slog.Error("fetch profile for tdee", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
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
		slog.Error("fetch nutrition logs for tdee", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	bioLogs, err := h.s.FetchBiometricLogs(r.Context(), claims.UserID, since)
	if err != nil {
		slog.Error("fetch biometric logs for tdee", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}

	res := calculator.ComputeObservedTDEE(nutLogs, bioLogs, profile, 0)
	respond.JSON(w, http.StatusOK, res)
}

// GET /v1/calc/macros
func (h *CalcHandler) Macros(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	profile, err := h.s.FetchProfile(r.Context(), claims.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "profile not found")
		return
	}
	if err != nil {
		slog.Error("fetch profile for macros", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}

	weight, err := h.s.FetchLatestWeight(r.Context(), claims.UserID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Error("fetch latest weight for macros", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}

	nutLogs, err := h.s.FetchNutritionLogs(r.Context(), claims.UserID, time.Now().UTC().AddDate(0, 0, -constants.DefaultTDEELookbackDays).Format(constants.DateFormat))
	if err != nil {
		slog.Error("fetch nutrition logs for macros", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	bioLogs, err := h.s.FetchBiometricLogs(r.Context(), claims.UserID, time.Now().UTC().AddDate(0, 0, -constants.DefaultTDEELookbackDays).Format(constants.DateFormat))
	if err != nil {
		slog.Error("fetch biometric logs for macros", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	observed := calculator.ComputeObservedTDEE(nutLogs, bioLogs, profile, 0)
	basis := nutrition.ComputeTargetBasis(profile, weight, observed.ObservedTDEE, observed.WeightReliable)
	respond.JSON(w, http.StatusOK, nutrition.ComputeTargetPlan(profile, weight, basis))
}

type DistributeMacrosRequest struct {
	Calories float64 `json:"calories"`
}

// POST /v1/calc/macros/distribute — compute macro distribution for a given calorie target
func (h *CalcHandler) DistributeMacros(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)

	var req DistributeMacrosRequest
	if !respond.Decode(w, r, &req) {
		return
	}

	if math.IsNaN(req.Calories) || math.IsInf(req.Calories, 0) {
		respond.Error(w, http.StatusBadRequest, "calories must be a finite number")
		return
	}
	if req.Calories <= 0 {
		respond.Error(w, http.StatusBadRequest, "calories must be > 0")
		return
	}
	if req.Calories > 10000 {
		respond.Error(w, http.StatusBadRequest, "calories must be <= 10000")
		return
	}

	profile, err := h.s.FetchProfile(r.Context(), claims.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "profile not found")
		return
	}
	if err != nil {
		slog.Error("fetch profile for distribute macros", "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	weight, err := h.s.FetchLatestWeight(r.Context(), claims.UserID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Error("fetch latest weight for distribute macros", "err", err)
		respond.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if weight <= 0 {
		weight = 70 // fallback weight if none logged
	}

	result := nutrition.DistributeMacros(profile, req.Calories, weight)
	respond.JSON(w, http.StatusOK, result)
}

// GET /v1/calc/readiness
func (h *CalcHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	profile, err := h.s.FetchProfile(r.Context(), claims.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "profile not found")
		return
	}
	if err != nil {
		slog.Error("fetch profile for readiness", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	asOf := requestDate(r)
	asOfTime, _ := time.Parse(constants.DateFormat, asOf)
	since := asOfTime.AddDate(0, 0, -constants.DefaultReadinessLookbackDays).Format(constants.DateFormat)
	bioLogs, err := h.s.FetchBiometricLogs(r.Context(), claims.UserID, since)
	if err != nil {
		slog.Error("fetch biometric logs for readiness", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	res := readiness.ComputeReadiness(bioLogs, profile, asOf)
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
	if errors.Is(err, sql.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "profile not found")
		return
	}
	if err != nil {
		slog.Error("fetch profile for bodyfat", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	weight, err := h.s.FetchLatestWeight(r.Context(), claims.UserID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Error("fetch latest weight for bodyfat", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}

	// most recent body measurements
	neckCm, waistCm, hipsCm, err := h.s.FetchBodyMeasurements(r.Context(), claims.UserID)
	if err != nil {
		slog.Error("fetch body measurements for bodyfat", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
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
	if errors.Is(err, sql.ErrNoRows) {
		respond.Error(w, http.StatusNotFound, "profile not found")
		return
	}
	if err != nil {
		slog.Error("fetch profile for dashboard", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}

	weight, err := h.s.FetchLatestWeight(r.Context(), claims.UserID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Error("fetch latest weight for dashboard", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}

	// Use client-supplied local date if valid (avoids UTC timezone mismatch).
	// Frontend passes ?date=YYYY-MM-DD reflecting the user's local calendar day.
	today := requestDate(r)

	// Derive lookback windows relative to the resolved today.
	// since7 is the first day of an inclusive 7-calendar-day window ending on
	// today: [today-6, today]. Subtracting 7 here would span 8 days.
	todayTime, _ := time.Parse(constants.DateFormat, today)
	since7 := todayTime.AddDate(0, 0, -(constants.WeeklyStatsWindowDays - 1)).Format(constants.DateFormat)
	since30 := todayTime.AddDate(0, 0, -constants.DefaultReadinessLookbackDays).Format(constants.DateFormat)
	since90 := todayTime.AddDate(0, 0, -constants.DefaultTDEELookbackDays).Format(constants.DateFormat)

	// The weight trend uses the profile's TDEE lookback window, which can be
	// longer than the 30-day readiness window. Fetch the widest span once and
	// derive the 30-day slice in memory.
	trendLookback := profile.TDEELookbackDays
	if trendLookback <= 0 {
		trendLookback = constants.DefaultTDEELookbackDays
	}
	sinceTrend := todayTime.AddDate(0, 0, -trendLookback).Format(constants.DateFormat)
	bioSince := since30
	if sinceTrend < bioSince {
		bioSince = sinceTrend
	}

	nut90, err := h.s.FetchNutritionLogs(r.Context(), claims.UserID, since90)
	if err != nil {
		slog.Error("fetch nutrition logs for dashboard", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	bioAll, err := h.s.FetchBiometricLogs(r.Context(), claims.UserID, bioSince)
	if err != nil {
		slog.Error("fetch biometric logs for dashboard", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	var bio30 []models.BiometricLog
	for _, b := range bioAll {
		if b.Date >= since30 {
			bio30 = append(bio30, b)
		}
	}
	workouts7, err := h.s.FetchWorkouts(r.Context(), claims.UserID, since7)
	if err != nil {
		slog.Error("fetch workouts (7d) for dashboard", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	workouts30, err := h.s.FetchWorkouts(r.Context(), claims.UserID, since30)
	if err != nil {
		slog.Error("fetch workouts (30d) for dashboard", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}

	tdeeRes := calculator.ComputeObservedTDEE(nut90, bio30, profile, 0)
	basis := nutrition.ComputeTargetBasis(profile, weight, tdeeRes.ObservedTDEE, tdeeRes.WeightReliable)
	macros := nutrition.ComputeTargetPlan(profile, weight, basis)

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
	readinessRes := readiness.ComputeReadiness(bio30, profile, today)
	_, _, velDelta, velArrow := readiness.ComputeReadinessVelocity(bio30, profile, today)
	velTrend := "stable"
	if velArrow == "↑" {
		velTrend = "improving"
	} else if velArrow == "↓" {
		velTrend = "declining"
	}
	readinessRes.VelocityTrend = velTrend
	readinessRes.VelocityDelta = velDelta

	// Filter nutrition to the 30-day window for the weekly stats' streak map.
	var nut30 []models.NutritionLog
	for _, n := range nut90 {
		if n.Date >= since30 {
			nut30 = append(nut30, n)
		}
	}
	// weekly stats: the 7-day metrics are narrowed inside metrics.WeeklyStats
	// from the 30-day slices below, which it also needs for the 30-day streak
	// map. Passing workouts30 used to inflate TotalWorkouts/TotalMWV beyond the
	// week the card advertises, so the window is now owned in one place.
	weekly := metrics.WeeklyStats(nut30, bio30, workouts30, today)
	todaySummary := metrics.TodaySummary(today, nut90, targets)

	// Insight layer: least-squares weight trend from the adaptive engine's
	// regression helper (level at the newest observation, not a forecast) and
	// a single-pass 60-day nutrition streak calendar.
	weightTrendKg, weightRateKgPerWeek, trendDays := calculator.ComputeWeightTrend(bioAll, sinceTrend)
	loggedDays, loggingStreak, longestStreak := metrics.LoggingStreaks(nut90, today, 60)

	// Weight trend display points and the engine trend series are built from a
	// single source (the same deduplicated observations the headline fit uses),
	// so the two arrays are index-aligned and date-key drift is impossible.
	// The fit still spans the full trend window; only the emitted display points
	// are limited to the 30-day window.
	weightTrend, weightTrendSeries := buildWeightTrendSeries(bioAll, since30, sinceTrend)

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
		Today:               todaySummary,
		TDEE:                tdeeRes,
		Macros:              macros,
		Readiness:           readinessRes,
		WeeklyStats:         weekly,
		WeightTrend:         weightTrend,
		TodayBio:            todayBio,
		GripPersonalBest:    gripPB,
		WorkoutToday:        workoutToday,
		WeightTrendKg:       weightTrendKg,
		WeightRateKgPerWeek: weightRateKgPerWeek,
		TrendDays:           trendDays,
		WeightTrendSeries:   weightTrendSeries,
		LoggingStreak:       loggingStreak,
		LongestStreak:       longestStreak,
		LoggedDays:          loggedDays,
	}

	// include check-in readiness (last checkin)
	if lastCheckInDate, err := h.s.FetchLastCheckin(r.Context(), claims.UserID); err == nil {
		if lastCheckInDate != "" {
			if days, ok := daysSinceLocalDate(lastCheckInDate); ok {
				can := days >= constants.MinCheckinIntervalDays
				dash.CanChangeTargets = can
				if !can {
					dash.DaysUntilCheckin = constants.MinCheckinIntervalDays - days
				}
			} else {
				dash.CanChangeTargets = true
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
