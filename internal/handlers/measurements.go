package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/auth"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/constants"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/respond"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type MeasurementsHandler struct{ db *sql.DB }

func NewMeasurementsHandler(db *sql.DB) *MeasurementsHandler { return &MeasurementsHandler{db: db} }

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
	rows, err := h.db.QueryContext(r.Context(), `SELECT id,user_id,date,neck_cm,chest_cm,waist_cm,hips_cm,thigh_cm,bicep_cm,notes,created_at,shoulders_cm,calves_cm FROM body_measurements WHERE user_id = ? AND date >= ? AND date <= ? ORDER BY date ASC`, claims.UserID, from, to)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()
	var out []models.BodyMeasurement
	for rows.Next() {
		var m models.BodyMeasurement
		if err := rows.Scan(&m.ID, &m.UserID, &m.Date, &m.NeckCm, &m.ChestCm, &m.WaistCm, &m.HipsCm, &m.ThighCm, &m.BicepCm, &m.Notes, &m.CreatedAt, &m.ShouldersCm, &m.CalvesCm); err != nil {
			respond.Error(w, http.StatusInternalServerError, "database error")
			return
		}
		out = append(out, m)
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
	_, err := h.db.ExecContext(r.Context(), `INSERT INTO body_measurements (id,user_id,date,neck_cm,chest_cm,waist_cm,hips_cm,thigh_cm,bicep_cm,notes,created_at,shoulders_cm,calves_cm) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(user_id,date) DO UPDATE SET neck_cm=excluded.neck_cm,chest_cm=excluded.chest_cm,waist_cm=excluded.waist_cm,hips_cm=excluded.hips_cm,thigh_cm=excluded.thigh_cm,bicep_cm=excluded.bicep_cm,notes=excluded.notes,shoulders_cm=excluded.shoulders_cm,calves_cm=excluded.calves_cm`, in.ID, in.UserID, in.Date, in.NeckCm, in.ChestCm, in.WaistCm, in.HipsCm, in.ThighCm, in.BicepCm, in.Notes, now, in.ShouldersCm, in.CalvesCm)
	if err != nil {
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
	res, err := h.db.ExecContext(r.Context(), `UPDATE body_measurements SET neck_cm=?,chest_cm=?,waist_cm=?,hips_cm=?,thigh_cm=?,bicep_cm=?,notes=?,shoulders_cm=?,calves_cm=? WHERE user_id=? AND date=?`, in.NeckCm, in.ChestCm, in.WaistCm, in.HipsCm, in.ThighCm, in.BicepCm, in.Notes, in.ShouldersCm, in.CalvesCm, claims.UserID, date)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	if ra, _ := res.RowsAffected(); ra == 0 {
		respond.Error(w, http.StatusNotFound, "measurement not found")
		return
	}
	respond.JSON(w, http.StatusOK, in)
}

// DELETE /v1/measurements/{date} — delete by date
func (h *MeasurementsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	date := chi.URLParam(r, "date")
	result, err := h.db.ExecContext(r.Context(), `DELETE FROM body_measurements WHERE user_id = ? AND date = ?`, claims.UserID, date)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		respond.Error(w, http.StatusNotFound, "measurement not found")
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
