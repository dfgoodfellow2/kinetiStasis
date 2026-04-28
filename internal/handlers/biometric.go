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

type BiometricHandler struct{ db *sql.DB }

func NewBiometricHandler(db *sql.DB) *BiometricHandler { return &BiometricHandler{db: db} }

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
	rows, err := h.db.QueryContext(r.Context(), `SELECT id,user_id,date,weight_kg,waist_cm,grip_kg,bolt_score,sleep_hours,sleep_quality,subjective_feel,notes,updated_at FROM biometric_logs WHERE user_id = ? AND date >= ? AND date <= ? ORDER BY date ASC`, claims.UserID, from, to)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()
	var out []models.BiometricLog
	for rows.Next() {
		var b models.BiometricLog
		if err := rows.Scan(&b.ID, &b.UserID, &b.Date, &b.WeightKg, &b.WaistCm, &b.GripKg, &b.BoltScore, &b.SleepHours, &b.SleepQuality, &b.SubjectiveFeel, &b.Notes, &b.UpdatedAt); err != nil {
			respond.Error(w, http.StatusInternalServerError, "database error")
			return
		}
		out = append(out, b)
	}
	respond.JSON(w, http.StatusOK, out)
}

// GET /v1/biometric/logs/{date}
func (h *BiometricHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	date := chi.URLParam(r, "date")
	var b models.BiometricLog
	err := h.db.QueryRowContext(r.Context(), `SELECT id,user_id,date,weight_kg,waist_cm,grip_kg,bolt_score,sleep_hours,sleep_quality,subjective_feel,notes,updated_at FROM biometric_logs WHERE user_id = ? AND date = ?`, claims.UserID, date).Scan(&b.ID, &b.UserID, &b.Date, &b.WeightKg, &b.WaistCm, &b.GripKg, &b.BoltScore, &b.SleepHours, &b.SleepQuality, &b.SubjectiveFeel, &b.Notes, &b.UpdatedAt)
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
	_, err := h.db.ExecContext(r.Context(), `
        INSERT INTO biometric_logs
          (id,user_id,date,weight_kg,waist_cm,grip_kg,bolt_score,sleep_hours,sleep_quality,subjective_feel,notes,updated_at)
        VALUES (?,?,?,?,?,?,?,?,?,?,?,?)
        ON CONFLICT(user_id,date) DO UPDATE SET
          weight_kg      = CASE WHEN excluded.weight_kg      != 0   THEN excluded.weight_kg      ELSE weight_kg      END,
          waist_cm       = CASE WHEN excluded.waist_cm       != 0   THEN excluded.waist_cm       ELSE waist_cm       END,
          grip_kg        = CASE WHEN excluded.grip_kg        != 0   THEN excluded.grip_kg        ELSE grip_kg        END,
          bolt_score     = CASE WHEN excluded.bolt_score     != 0   THEN excluded.bolt_score     ELSE bolt_score     END,
          sleep_hours    = CASE WHEN excluded.sleep_hours    != 0   THEN excluded.sleep_hours    ELSE sleep_hours    END,
          sleep_quality  = CASE WHEN excluded.sleep_quality  != 0   THEN excluded.sleep_quality  ELSE sleep_quality  END,
          subjective_feel= CASE WHEN excluded.subjective_feel!= 0   THEN excluded.subjective_feel ELSE subjective_feel END,
          notes          = CASE WHEN excluded.notes          != ''  THEN excluded.notes          ELSE notes          END,
          updated_at     = excluded.updated_at`,
		in.ID, in.UserID, in.Date, in.WeightKg, in.WaistCm, in.GripKg, in.BoltScore,
		in.SleepHours, in.SleepQuality, in.SubjectiveFeel, in.Notes, in.UpdatedAt,
	)
	if err != nil {
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
	var id string
	if err := h.db.QueryRowContext(r.Context(), `SELECT id FROM biometric_logs WHERE user_id = ? AND date = ?`, claims.UserID, date).Scan(&id); err == sql.ErrNoRows {
		respond.Error(w, http.StatusNotFound, "biometric not found")
		return
	} else if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := h.db.ExecContext(r.Context(), `UPDATE biometric_logs SET weight_kg=?,waist_cm=?,grip_kg=?,bolt_score=?,sleep_hours=?,sleep_quality=?,subjective_feel=?,notes=?,updated_at=? WHERE user_id=? AND date=?`, in.WeightKg, in.WaistCm, in.GripKg, in.BoltScore, in.SleepHours, in.SleepQuality, in.SubjectiveFeel, in.Notes, now, claims.UserID, date)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	var out models.BiometricLog
	if err := h.db.QueryRowContext(r.Context(),
		`SELECT id,user_id,date,weight_kg,waist_cm,grip_kg,bolt_score,sleep_hours,sleep_quality,subjective_feel,notes,updated_at FROM biometric_logs WHERE user_id = ? AND date = ?`,
		claims.UserID, date,
	).Scan(&out.ID, &out.UserID, &out.Date, &out.WeightKg, &out.WaistCm, &out.GripKg, &out.BoltScore, &out.SleepHours, &out.SleepQuality, &out.SubjectiveFeel, &out.Notes, &out.UpdatedAt); err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to retrieve updated record")
		return
	}
	respond.JSON(w, http.StatusOK, out)
}

// DELETE /v1/biometric/logs/{date} — delete by date
func (h *BiometricHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	date := chi.URLParam(r, "date")
	result, err := h.db.ExecContext(r.Context(), `DELETE FROM biometric_logs WHERE user_id = ? AND date = ?`, claims.UserID, date)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		respond.Error(w, http.StatusNotFound, "biometric not found")
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
