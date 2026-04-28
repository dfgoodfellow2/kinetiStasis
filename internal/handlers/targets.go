package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/auth"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/respond"
	"github.com/google/uuid"
)

type TargetsHandler struct{ db *sql.DB }

func NewTargetsHandler(db *sql.DB) *TargetsHandler { return &TargetsHandler{db: db} }

// GET /v1/targets
func (h *TargetsHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	var t models.Targets
	err := h.db.QueryRowContext(r.Context(), `SELECT user_id,calories,protein_g,carbs_g,fat_g,fiber_g,water_ml,eat_back_exercise,updated_at FROM targets WHERE user_id = ?`, claims.UserID).Scan(&t.UserID, &t.Calories, &t.ProteinG, &t.CarbsG, &t.FatG, &t.FiberG, &t.WaterMl, &t.EatBackExercise, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		respond.Error(w, http.StatusNotFound, "targets not found")
		return
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	respond.JSON(w, http.StatusOK, t)
}

// PUT /v1/targets — upsert and snapshot if changed
func (h *TargetsHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	var in models.Targets
	if !respond.Decode(w, r, &in) {
		return
	}
	in.UserID = claims.UserID
	now := time.Now().UTC().Format(time.RFC3339)

	// load existing
	var existing models.Targets
	err := h.db.QueryRowContext(r.Context(), `SELECT user_id,calories,protein_g,carbs_g,fat_g,fiber_g,water_ml,eat_back_exercise,updated_at FROM targets WHERE user_id = ?`, claims.UserID).Scan(&existing.UserID, &existing.Calories, &existing.ProteinG, &existing.CarbsG, &existing.FatG, &existing.FiberG, &existing.WaterMl, &existing.EatBackExercise, &existing.UpdatedAt)
	if err != nil && err != sql.ErrNoRows {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}

	// if changed, snapshot old
	changed := false
	if err != sql.ErrNoRows {
		if existing.Calories != in.Calories || existing.ProteinG != in.ProteinG || existing.CarbsG != in.CarbsG || existing.FatG != in.FatG || existing.FiberG != in.FiberG {
			changed = true
		}
	}
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	if changed {
		// insert snapshot
		_, err := tx.ExecContext(r.Context(), `INSERT INTO target_history (id,user_id,effective_date,calories,protein_g,carbs_g,fat_g,fiber_g,created_at) VALUES (?,?,?,?,?,?,?,?,?)`, uuid.New().String(), claims.UserID, time.Now().UTC().Format("2006-01-02"), existing.Calories, existing.ProteinG, existing.CarbsG, existing.FatG, existing.FiberG, now)
		if err != nil {
			tx.Rollback()
			respond.Error(w, http.StatusInternalServerError, "database error")
			return
		}
	}

	// upsert
	_, err = tx.ExecContext(r.Context(), `INSERT INTO targets (user_id,calories,protein_g,carbs_g,fat_g,fiber_g,water_ml,eat_back_exercise,updated_at) VALUES (?,?,?,?,?,?,?,?,?) ON CONFLICT(user_id) DO UPDATE SET calories=excluded.calories,protein_g=excluded.protein_g,carbs_g=excluded.carbs_g,fat_g=excluded.fat_g,fiber_g=excluded.fiber_g,water_ml=excluded.water_ml,eat_back_exercise=excluded.eat_back_exercise,updated_at=excluded.updated_at`, in.UserID, in.Calories, in.ProteinG, in.CarbsG, in.FatG, in.FiberG, in.WaterMl, in.EatBackExercise, now)
	if err != nil {
		tx.Rollback()
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	if err := tx.Commit(); err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	in.UpdatedAt = now
	respond.JSON(w, http.StatusOK, in)
}

// GET /v1/targets/history
func (h *TargetsHandler) History(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	rows, err := h.db.QueryContext(r.Context(), `SELECT id,user_id,effective_date,calories,protein_g,carbs_g,fat_g,fiber_g,created_at FROM target_history WHERE user_id = ? ORDER BY effective_date DESC`, claims.UserID)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()
	var out []models.TargetSnapshot
	for rows.Next() {
		var t models.TargetSnapshot
		if err := rows.Scan(&t.ID, &t.UserID, &t.EffectiveDate, &t.Calories, &t.ProteinG, &t.CarbsG, &t.FatG, &t.FiberG, &t.CreatedAt); err != nil {
			respond.Error(w, http.StatusInternalServerError, "database error")
			return
		}
		out = append(out, t)
	}
	respond.JSON(w, http.StatusOK, out)
}
