package handlers

import (
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/auth"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/respond"
)

type ProfileHandler struct{ db *sql.DB }

func NewProfileHandler(db *sql.DB) *ProfileHandler { return &ProfileHandler{db: db} }

// GET /v1/profile — return authenticated user's profile
func (h *ProfileHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	var p models.Profile
	err := h.db.QueryRowContext(r.Context(), `
        SELECT user_id, COALESCE(name,''), COALESCE(age,0), COALESCE(sex,''),
               COALESCE(height_cm,0), COALESCE(activity,'sedentary'), COALESCE(exercise_freq,0),
               COALESCE(running_km,0), COALESCE(is_lifter,0), COALESCE(goal,'maintenance'),
               COALESCE(prioritize_carbs,0), COALESCE(bf_pct,0), COALESCE(hr_rest,0),
               COALESCE(hr_max,0), COALESCE(grip_weight,0.5), COALESCE(tdee_lookback_days,90),
               COALESCE(sleep_quality_max,10.0), COALESCE(units,'imperial'), updated_at
        FROM profiles WHERE user_id = ?`, claims.UserID,
	).Scan(&p.UserID, &p.Name, &p.Age, &p.Sex, &p.HeightCm, &p.Activity, &p.ExerciseFreq,
		&p.RunningKm, &p.IsLifter, &p.Goal, &p.PrioritizeCarbs, &p.BfPct, &p.HRRest,
		&p.HRMax, &p.GripWeight, &p.TDEELookbackDays, &p.SleepQualityMax, &p.Units, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		respond.Error(w, http.StatusNotFound, "profile not found")
		return
	}
	if err != nil {
		slog.Error("profile upsert failed", "user_id", claims.UserID, "err", err)
		respond.Error(w, http.StatusInternalServerError, "profile save failed: "+err.Error())
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
	_, err := h.db.ExecContext(r.Context(), `
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
