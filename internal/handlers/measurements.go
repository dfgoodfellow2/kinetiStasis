package handlers

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/auth"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/constants"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/respond"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/validate"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type MeasurementsHandler struct{ s store.Store }

func NewMeasurementsHandler(s store.Store) *MeasurementsHandler { return &MeasurementsHandler{s: s} }

// maxCircumferenceCm is the upper bound for a single body circumference in
// centimetres. Anything beyond this is a data-entry error, not a real body.
const maxCircumferenceCm = 250

func validateMeasurement(m *models.BodyMeasurement) error {
	fields := []struct {
		name string
		v    float64
	}{
		{"neckCm", m.NeckCm},
		{"chestCm", m.ChestCm},
		{"waistCm", m.WaistCm},
		{"hipsCm", m.HipsCm},
		{"thighCm", m.ThighCm},
		{"bicepCm", m.BicepCm},
		{"shouldersCm", m.ShouldersCm},
		{"calvesCm", m.CalvesCm},
	}
	for _, f := range fields {
		if err := validate.NonNegativeBounded(f.name, f.v, maxCircumferenceCm); err != nil {
			return err
		}
	}
	return nil
}

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
		slog.Error("fetch measurements range", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
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
	if err := validateMeasurement(&in); err != nil {
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	in.UserID = claims.UserID
	if in.ID == "" {
		in.ID = uuid.New().String()
	}
	now := time.Now().UTC().Format(constants.TimeFormat)
	// UPSERT: insert or replace if date already exists for this user
	if err := h.s.CreateMeasurement(r.Context(), &in); err != nil {
		slog.Error("create measurement", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
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
	if err := validateMeasurement(&in); err != nil {
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	in.UserID = claims.UserID
	in.Date = date
	if err := h.s.UpdateMeasurement(r.Context(), &in); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respond.Error(w, http.StatusNotFound, "measurement not found")
			return
		}
		slog.Error("update measurement", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, in)
}

// DELETE /v1/measurements/{date} — delete by date
func (h *MeasurementsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	date := chi.URLParam(r, "date")
	if err := h.s.DeleteMeasurement(r.Context(), claims.UserID, date); err != nil {
		slog.Error("delete measurement", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
