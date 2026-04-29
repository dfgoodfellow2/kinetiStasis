package handlers

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/auth"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/constants"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/respond"
	workoutsvc "github.com/dfgoodfellow2/diet-tracker/v2/internal/services/workout"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type WorkoutHandler struct{ db *sql.DB }

func NewWorkoutHandler(db *sql.DB) *WorkoutHandler { return &WorkoutHandler{db: db} }

const workoutSelectCols = `id, user_id, date, slot, title, raw_notes, duration_min, calories_burned, mwv, nds, session_density, exercises_json, metadata_json, updated_at`

func scanWorkout(row interface {
	Scan(...interface{}) error
}, w *models.WorkoutEntry) error {
	var exercisesJSON, metadataJSON string
	err := row.Scan(
		&w.ID, &w.UserID, &w.Date, &w.Slot, &w.Title, &w.RawNotes,
		&w.DurationMin, &w.CaloriesBurned, &w.MWV, &w.NDS, &w.SessionDensity,
		&exercisesJSON, &metadataJSON, &w.UpdatedAt,
	)
	if err != nil {
		return err
	}
	if exercisesJSON != "" {
		_ = json.Unmarshal([]byte(exercisesJSON), &w.Exercises)
	}
	if metadataJSON != "" {
		_ = json.Unmarshal([]byte(metadataJSON), &w.Metadata)
	}
	return nil
}

// GET /v1/workouts
func (h *WorkoutHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	today := time.Now().UTC().Format(constants.DateFormat)
	if from == "" {
		from = today
	}
	if to == "" {
		to = today
	}
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT `+workoutSelectCols+` FROM workouts WHERE user_id = ? AND date >= ? AND date <= ? ORDER BY date ASC, slot ASC`,
		claims.UserID, from, to)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()
	var out []models.WorkoutEntry
	for rows.Next() {
		var went models.WorkoutEntry
		if err := scanWorkout(rows, &went); err != nil {
			respond.Error(w, http.StatusInternalServerError, "database error")
			return
		}
		out = append(out, went)
	}
	respond.JSON(w, http.StatusOK, out)
}

// GET /v1/workouts/{date}/{slot}
func (h *WorkoutHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	date := chi.URLParam(r, "date")
	slot := chi.URLParam(r, "slot")
	var went models.WorkoutEntry
	row := h.db.QueryRowContext(r.Context(),
		`SELECT `+workoutSelectCols+` FROM workouts WHERE user_id = ? AND date = ? AND slot = ?`,
		claims.UserID, date, slot)
	if err := scanWorkout(row, &went); err == sql.ErrNoRows {
		respond.Error(w, http.StatusNotFound, "workout not found")
		return
	} else if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	respond.JSON(w, http.StatusOK, went)
}

// POST /v1/workouts
func (h *WorkoutHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	var in models.WorkoutEntry
	if !respond.Decode(w, r, &in) {
		return
	}
	in.UserID = claims.UserID
	in.UpdatedAt = time.Now().UTC().Format(constants.TimeFormat)
	if in.ID == "" {
		in.ID = uuid.New().String()
	}
	// Auto-calculate calories from MET if not provided and we have a recent weight
	if in.CaloriesBurned == 0 && len(in.Exercises) > 0 {
		weightKg := fetchLatestWeight(r.Context(), h.db, claims.UserID)
		if weightKg > 0 {
			in.CaloriesBurned = workoutsvc.CalculateCaloriesBurned(in.Exercises, weightKg, in.DurationMin)
		}
	}

	exb, _ := json.Marshal(in.Exercises)
	metb, _ := json.Marshal(in.Metadata)

	// Upsert: try UPDATE first, then INSERT if not found
	res, err := h.db.ExecContext(r.Context(),
		`UPDATE workouts SET title=?, raw_notes=?, duration_min=?, calories_burned=?, exercises_json=?, metadata_json=?, updated_at=? WHERE user_id=? AND date=? AND slot=?`,
		in.Title, in.RawNotes, in.DurationMin, in.CaloriesBurned, string(exb), string(metb), in.UpdatedAt, claims.UserID, in.Date, in.Slot)
	if err != nil {
		slog.Error("workout update", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	if ra, _ := res.RowsAffected(); ra == 0 {
		// Not found, INSERT
		_, err := h.db.ExecContext(r.Context(),
			`INSERT INTO workouts (id,user_id,date,slot,title,raw_notes,duration_min,calories_burned,mwv,nds,session_density,exercises_json,metadata_json,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			in.ID, claims.UserID, in.Date, in.Slot, in.Title, in.RawNotes, in.DurationMin, in.CaloriesBurned, in.MWV, in.NDS, in.SessionDensity, string(exb), string(metb), in.UpdatedAt)
		if err != nil {
			slog.Error("workout insert", "err", err)
			respond.Error(w, http.StatusInternalServerError, "database error")
			return
		}
	}
	respond.JSON(w, http.StatusOK, in)
}

// PUT /v1/workouts/{date}/{slot}
func (h *WorkoutHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	date := chi.URLParam(r, "date")
	slot := chi.URLParam(r, "slot")
	var in models.WorkoutEntry
	if !respond.Decode(w, r, &in) {
		return
	}
	var id string
	if err := h.db.QueryRowContext(r.Context(),
		`SELECT id FROM workouts WHERE user_id = ? AND date = ? AND slot = ?`,
		claims.UserID, date, slot).Scan(&id); err == sql.ErrNoRows {
		respond.Error(w, http.StatusNotFound, "workout not found")
		return
	} else if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	// Auto-calc calories on update if not provided
	if in.CaloriesBurned == 0 && len(in.Exercises) > 0 {
		weightKg := fetchLatestWeight(r.Context(), h.db, claims.UserID)
		if weightKg > 0 {
			in.CaloriesBurned = workoutsvc.CalculateCaloriesBurned(in.Exercises, weightKg, in.DurationMin)
		}
	}

	exb, _ := json.Marshal(in.Exercises)
	metb, _ := json.Marshal(in.Metadata)
	now := time.Now().UTC().Format(constants.TimeFormat)
	_, err := h.db.ExecContext(r.Context(),
		`UPDATE workouts SET title=?,raw_notes=?,duration_min=?,calories_burned=?,mwv=?,nds=?,session_density=?,exercises_json=?,metadata_json=?,updated_at=?
         WHERE user_id=? AND date=? AND slot=?`,
		in.Title, in.RawNotes, in.DurationMin, in.CaloriesBurned, in.MWV, in.NDS, in.SessionDensity,
		string(exb), string(metb), now, claims.UserID, date, slot)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	var out models.WorkoutEntry
	row := h.db.QueryRowContext(r.Context(),
		`SELECT `+workoutSelectCols+` FROM workouts WHERE user_id = ? AND date = ? AND slot = ?`,
		claims.UserID, date, slot)
	if err := scanWorkout(row, &out); err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to retrieve updated record")
		return
	}
	respond.JSON(w, http.StatusOK, out)
}

// DELETE /v1/workouts/{date}/{slot}
func (h *WorkoutHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	date := chi.URLParam(r, "date")
	slot := chi.URLParam(r, "slot")
	_, err := h.db.ExecContext(r.Context(),
		`DELETE FROM workouts WHERE user_id = ? AND date = ? AND slot = ?`,
		claims.UserID, date, slot)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
