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

type BiometricHandler struct{ s store.Store }

func NewBiometricHandler(s store.Store) *BiometricHandler { return &BiometricHandler{s: s} }

// GET /v1/biometric/logs
func (h *BiometricHandler) List(w http.ResponseWriter, r *http.Request) {
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
	out, err := h.s.FetchBiometricLogsRange(r.Context(), claims.UserID, from, to)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	respond.JSON(w, http.StatusOK, out)
}

// GET /v1/biometric/logs/{date}
func (h *BiometricHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	date := chi.URLParam(r, "date")
	var b models.BiometricLog
	b, err := h.s.GetBiometricLog(r.Context(), claims.UserID, date)
	if err == sql.ErrNoRows {
		respond.Error(w, http.StatusNotFound, "biometric not found")
		return
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	respond.JSON(w, http.StatusOK, b)
}

// POST /v1/biometric/logs — upsert last-write-wins
func (h *BiometricHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	var in models.BiometricLog
	if !respond.Decode(w, r, &in) {
		return
	}
	// Validate weight only if positive (0 means not logged)
	if in.WeightKg > 0 {
		if in.WeightKg < 20 || in.WeightKg > 300 {
			respond.Error(w, http.StatusBadRequest, "weight must be between 20 and 300 kg")
			return
		}
	}
	in.UserID = claims.UserID
	in.UpdatedAt = time.Now().UTC().Format(constants.TimeFormat)
	if in.ID == "" {
		in.ID = uuid.New().String()
	}
	if err := h.s.CreateBiometricLog(r.Context(), &in); err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	respond.JSON(w, http.StatusOK, in)
}

// PUT /v1/biometric/logs/{date} — full replace
func (h *BiometricHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	date := chi.URLParam(r, "date")
	var in models.BiometricLog
	if !respond.Decode(w, r, &in) {
		return
	}
	// Validate weight only if positive (0 means not logged)
	if in.WeightKg > 0 {
		if in.WeightKg < 20 || in.WeightKg > 300 {
			respond.Error(w, http.StatusBadRequest, "weight must be between 20 and 300 kg")
			return
		}
	}
	// fetch existing to perform a merge (PUT should allow partial fields like POST/UPSERT)
	existing, err := h.s.GetBiometricLog(r.Context(), claims.UserID, date)
	if err == sql.ErrNoRows {
		respond.Error(w, http.StatusNotFound, "biometric not found")
		return
	} else if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}

	// Merge: only overwrite fields that are explicitly provided (non-zero / non-empty)
	// Note: 0 means "not provided" for numeric fields in this API
	if in.WeightKg > 0 {
		existing.WeightKg = in.WeightKg
	}
	if in.WaistCm > 0 {
		existing.WaistCm = in.WaistCm
	}
	if in.GripKg > 0 {
		existing.GripKg = in.GripKg
	}
	if in.BoltScore > 0 {
		existing.BoltScore = in.BoltScore
	}
	if in.SleepHours > 0 {
		existing.SleepHours = in.SleepHours
	}
	if in.SleepQuality > 0 {
		existing.SleepQuality = in.SleepQuality
	}
	if in.SubjectiveFeel > 0 {
		existing.SubjectiveFeel = in.SubjectiveFeel
	}
	if in.BodyFatPct > 0 {
		existing.BodyFatPct = in.BodyFatPct
	}
	if in.Notes != "" {
		existing.Notes = in.Notes
	}

	now := time.Now().UTC().Format(constants.TimeFormat)
	existing.UpdatedAt = now
	existing.UserID = claims.UserID
	existing.Date = date

	if err := h.s.UpdateBiometricLog(r.Context(), &existing); err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	out, err := h.s.GetBiometricLog(r.Context(), claims.UserID, date)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to retrieve updated record")
		return
	}
	respond.JSON(w, http.StatusOK, out)
}

// DELETE /v1/biometric/logs/{date} — delete by date
func (h *BiometricHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	date := chi.URLParam(r, "date")
	if err := h.s.DeleteBiometricLog(r.Context(), claims.UserID, date); err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	// Note: store.DeleteBiometricLog could return sql.ErrNoRows if nothing deleted; currently implementation doesn't — handlers assume success
	respond.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
