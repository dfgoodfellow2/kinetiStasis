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
	met "github.com/dfgoodfellow2/diet-tracker/v2/internal/services/met"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type WorkoutHandler struct{ s store.Store }

func NewWorkoutHandler(s store.Store) *WorkoutHandler { return &WorkoutHandler{s: s} }

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
		if err := json.Unmarshal([]byte(exercisesJSON), &w.Exercises); err != nil {
			slog.Warn("unmarshal exercises_json failed", "err", err)
		} else {
			// Backwards compat: copy legacy snake_case duration_raw into DurationRaw
			for i := range w.Exercises {
				if w.Exercises[i].DurationRaw == "" && w.Exercises[i].DurationRawLegacy != "" {
					w.Exercises[i].DurationRaw = w.Exercises[i].DurationRawLegacy
				}
			}
		}
	}
	if metadataJSON != "" {
		if err := json.Unmarshal([]byte(metadataJSON), &w.Metadata); err != nil {
			slog.Warn("unmarshal metadata_json failed", "err", err)
		}
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
	out, err := h.s.FetchWorkoutsRange(r.Context(), claims.UserID, from, to)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	respond.JSON(w, http.StatusOK, out)
}

// GET /v1/workouts/{date}/{slot}
func (h *WorkoutHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	date := chi.URLParam(r, "date")
	slot := chi.URLParam(r, "slot")
	went, err := h.s.GetWorkout(r.Context(), claims.UserID, date, slot)
	if err == sql.ErrNoRows {
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
		if wkg, err := h.s.FetchLatestWeight(r.Context(), claims.UserID); err == nil && wkg > 0 {
			in.CaloriesBurned = met.CalculateCaloriesBurned(in.Exercises, wkg, in.DurationMin)
		}
	}

	// prepare json blobs
	_, _ = json.Marshal(in.Exercises)
	_, _ = json.Marshal(in.Metadata)

	if err := h.s.UpsertWorkout(r.Context(), &in); err != nil {
		slog.Error("workout upsert", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
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
	// ensure exists
	if _, err := h.s.GetWorkout(r.Context(), claims.UserID, date, slot); err == sql.ErrNoRows {
		respond.Error(w, http.StatusNotFound, "workout not found")
		return
	} else if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	// Auto-calc calories on update if not provided
	if in.CaloriesBurned == 0 && len(in.Exercises) > 0 {
		if wkg, err := h.s.FetchLatestWeight(r.Context(), claims.UserID); err == nil && wkg > 0 {
			in.CaloriesBurned = met.CalculateCaloriesBurned(in.Exercises, wkg, in.DurationMin)
		}
	}

	_, _ = json.Marshal(in.Exercises)
	_, _ = json.Marshal(in.Metadata)
	now := time.Now().UTC().Format(constants.TimeFormat)
	in.UserID = claims.UserID
	in.Date = date
	in.UpdatedAt = now
	if _, err := h.s.UpdateWorkout(r.Context(), &in); err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	out, err := h.s.GetWorkout(r.Context(), claims.UserID, date, slot)
	if err != nil {
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
	if err := h.s.DeleteWorkout(r.Context(), claims.UserID, date, slot); err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
