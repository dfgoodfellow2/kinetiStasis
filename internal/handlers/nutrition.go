package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/auth"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/constants"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/respond"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type NutritionHandler struct{ s store.Store }

func NewNutritionHandler(s store.Store) *NutritionHandler { return &NutritionHandler{s: s} }

// GET /v1/nutrition/logs
func (h *NutritionHandler) List(w http.ResponseWriter, r *http.Request) {
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

	out, err := h.s.FetchNutritionLogsRange(r.Context(), claims.UserID, from, to)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	respond.JSON(w, http.StatusOK, out)
}

// GET /v1/nutrition/logs/{date}
func (h *NutritionHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	date := chi.URLParam(r, "date")
	var n models.NutritionLog
	n, err := h.s.GetNutritionLog(r.Context(), claims.UserID, date)
	if err == sql.ErrNoRows {
		respond.Error(w, http.StatusNotFound, "nutrition log not found")
		return
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	respond.JSON(w, http.StatusOK, n)
}

// POST /v1/nutrition/logs — additive merge
func (h *NutritionHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	var in models.NutritionLog
	if !respond.Decode(w, r, &in) {
		return
	}
	in.UserID = claims.UserID
	now := time.Now().UTC().Format(constants.TimeFormat)

	// Check existing
	var existing models.NutritionLog
	existing, err := h.s.GetNutritionLog(r.Context(), claims.UserID, in.Date)
	if err != nil && err != sql.ErrNoRows {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	if err == sql.ErrNoRows {
		// insert new
		in.ID = uuid.New().String()
		in.UpdatedAt = now
		if err := h.s.CreateNutritionLog(r.Context(), &in); err != nil {
			respond.Error(w, http.StatusInternalServerError, "database error")
			return
		}
		if err != nil {
			respond.Error(w, http.StatusInternalServerError, "database error")
			return
		}
		respond.JSON(w, http.StatusCreated, in)
		return
	}

	// merge
	merged := existing
	merged.Calories += in.Calories
	merged.ProteinG += in.ProteinG
	merged.CarbsG += in.CarbsG
	merged.FatG += in.FatG
	merged.FiberG += in.FiberG
	merged.WaterMl += in.WaterMl
	if existing.MealNotes != "" && in.MealNotes != "" {
		merged.MealNotes = existing.MealNotes + "\n" + in.MealNotes
	} else if in.MealNotes != "" {
		merged.MealNotes = in.MealNotes
	}
	merged.UpdatedAt = now

	merged.UserID = claims.UserID
	merged.Date = in.Date
	if err := h.s.UpdateNutritionLog(r.Context(), &merged); err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	// return merged row
	merged.UserID = claims.UserID
	merged.Date = in.Date
	respond.JSON(w, http.StatusOK, merged)
}

// PUT /v1/nutrition/logs/{date} — full replace
func (h *NutritionHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	date := chi.URLParam(r, "date")
	var in models.NutritionLog
	if !respond.Decode(w, r, &in) {
		return
	}
	// ensure exists
	if _, err := h.s.GetNutritionLog(r.Context(), claims.UserID, date); err == sql.ErrNoRows {
		respond.Error(w, http.StatusNotFound, "nutrition log not found")
		return
	} else if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	in.UserID = claims.UserID
	in.Date = date
	in.UpdatedAt = now
	if err := h.s.UpdateNutritionLog(r.Context(), &in); err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	// return updated via store
	out, err := h.s.GetNutritionLog(r.Context(), claims.UserID, date)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to retrieve updated record")
		return
	}
	respond.JSON(w, http.StatusOK, out)
}

// DELETE /v1/nutrition/logs/{date}
func (h *NutritionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	date := chi.URLParam(r, "date")
	if err := h.s.DeleteNutritionLog(r.Context(), claims.UserID, date); err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
