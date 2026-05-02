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
	// ensure exists
	if _, err := h.s.GetBiometricLog(r.Context(), claims.UserID, date); err == sql.ErrNoRows {
		respond.Error(w, http.StatusNotFound, "biometric not found")
		return
	} else if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	in.UpdatedAt = now
	in.UserID = claims.UserID
	in.Date = date
	if err := h.s.UpdateBiometricLog(r.Context(), &in); err != nil {
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
