package handlers

import (
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/auth"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/respond"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"
)

type ProfileHandler struct{ s store.Store }

func NewProfileHandler(s store.Store) *ProfileHandler { return &ProfileHandler{s: s} }

// GET /v1/profile — return authenticated user's profile
func (h *ProfileHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	p, err := h.s.FetchProfile(r.Context(), claims.UserID)
	if err == sql.ErrNoRows {
		respond.Error(w, http.StatusNotFound, "profile not found")
		return
	}
	if err != nil {
		slog.Error("profile fetch failed", "user_id", claims.UserID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "profile fetch failed: "+err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, p)
}

// PUT /v1/profile — upsert profile
func (h *ProfileHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	var p models.Profile
	if !respond.Decode(w, r, &p) {
		return
	}
	p.UserID = claims.UserID
	p.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	isLifter := 0
	if p.IsLifter {
		isLifter = 1
	}
	prioritize := 0
	if p.PrioritizeCarbs {
		prioritize = 1
	}
	db := h.s.(*store.SQLiteStore).DB()
	_, err := db.ExecContext(r.Context(), `
        INSERT INTO profiles (user_id,name,age,sex,height_cm,activity,exercise_freq,running_km,
          is_lifter,goal,prioritize_carbs,bf_pct,hr_rest,hr_max,grip_weight,tdee_lookback_days,
          sleep_quality_max,units,updated_at)
        VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
        ON CONFLICT(user_id) DO UPDATE SET
          name=excluded.name, age=excluded.age, sex=excluded.sex, height_cm=excluded.height_cm,
          activity=excluded.activity, exercise_freq=excluded.exercise_freq,
          running_km=excluded.running_km, is_lifter=excluded.is_lifter, goal=excluded.goal,
          prioritize_carbs=excluded.prioritize_carbs, bf_pct=excluded.bf_pct, hr_rest=excluded.hr_rest,
          hr_max=excluded.hr_max, grip_weight=excluded.grip_weight,
          tdee_lookback_days=excluded.tdee_lookback_days, sleep_quality_max=excluded.sleep_quality_max,
          units=excluded.units, updated_at=excluded.updated_at`,
		p.UserID, p.Name, p.Age, p.Sex, p.HeightCm, p.Activity, p.ExerciseFreq, p.RunningKm,
		isLifter, p.Goal, prioritize, p.BfPct, p.HRRest, p.HRMax, p.GripWeight,
		p.TDEELookbackDays, p.SleepQualityMax, p.Units, p.UpdatedAt,
	)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}

	// Do NOT auto-compute or update macro targets here. Targets are managed via
	// onboarding, weekly check-ins, or manual overrides.
	respond.JSON(w, http.StatusOK, p)
}
