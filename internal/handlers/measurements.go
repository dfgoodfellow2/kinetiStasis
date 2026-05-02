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

type MeasurementsHandler struct{ s store.Store }

func NewMeasurementsHandler(s store.Store) *MeasurementsHandler { return &MeasurementsHandler{s: s} }

// GET /v1/measurements
func (h *MeasurementsHandler) List(w http.ResponseWriter, r *http.Request) {
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
	out, err := h.s.FetchMeasurementsRange(r.Context(), claims.UserID, from, to)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	respond.JSON(w, http.StatusOK, out)
}

// POST /v1/measurements — upsert (insert or update)
func (h *MeasurementsHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	var in models.BodyMeasurement
	if !respond.Decode(w, r, &in) {
		return
	}
	in.UserID = claims.UserID
	if in.ID == "" {
		in.ID = uuid.New().String()
	}
	now := time.Now().UTC().Format(constants.TimeFormat)
	// UPSERT: insert or replace if date already exists for this user
	if err := h.s.CreateMeasurement(r.Context(), &in); err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	in.CreatedAt = now
	respond.JSON(w, http.StatusOK, in)
}

// PUT /v1/measurements/{date} — update existing
func (h *MeasurementsHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	date := chi.URLParam(r, "date")
	var in models.BodyMeasurement
	if !respond.Decode(w, r, &in) {
		return
	}
	in.UserID = claims.UserID
	in.Date = date
	if err := h.s.UpdateMeasurement(r.Context(), &in); err != nil {
		if err == sql.ErrNoRows {
			respond.Error(w, http.StatusNotFound, "measurement not found")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	respond.JSON(w, http.StatusOK, in)
}

// DELETE /v1/measurements/{date} — delete by date
func (h *MeasurementsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	date := chi.URLParam(r, "date")
	if err := h.s.DeleteMeasurement(r.Context(), claims.UserID, date); err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
