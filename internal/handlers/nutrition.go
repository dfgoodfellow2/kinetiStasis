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

type NutritionHandler struct{ db *sql.DB }

func NewNutritionHandler(db *sql.DB) *NutritionHandler { return &NutritionHandler{db: db} }

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

	rows, err := h.db.QueryContext(r.Context(), `
        SELECT id,user_id,date,calories,protein_g,carbs_g,fat_g,fiber_g,water_ml,meal_notes,updated_at
        FROM nutrition_logs WHERE user_id = ? AND date >= ? AND date <= ? ORDER BY date ASC`,
		claims.UserID, from, to,
	)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()
	var out []models.NutritionLog
	for rows.Next() {
		var n models.NutritionLog
		if err := rows.Scan(&n.ID, &n.UserID, &n.Date, &n.Calories, &n.ProteinG, &n.CarbsG, &n.FatG, &n.FiberG, &n.WaterMl, &n.MealNotes, &n.UpdatedAt); err != nil {
			respond.Error(w, http.StatusInternalServerError, "database error")
			return
		}
		out = append(out, n)
	}
	respond.JSON(w, http.StatusOK, out)
}

// GET /v1/nutrition/logs/{date}
func (h *NutritionHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	date := chi.URLParam(r, "date")
	var n models.NutritionLog
	err := h.db.QueryRowContext(r.Context(), `
        SELECT id,user_id,date,calories,protein_g,carbs_g,fat_g,fiber_g,water_ml,meal_notes,updated_at
        FROM nutrition_logs WHERE user_id = ? AND date = ?`, claims.UserID, date,
	).Scan(&n.ID, &n.UserID, &n.Date, &n.Calories, &n.ProteinG, &n.CarbsG, &n.FatG, &n.FiberG, &n.WaterMl, &n.MealNotes, &n.UpdatedAt)
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
	err := h.db.QueryRowContext(r.Context(), `SELECT id,calories,protein_g,carbs_g,fat_g,fiber_g,water_ml,meal_notes FROM nutrition_logs WHERE user_id = ? AND date = ?`, claims.UserID, in.Date).Scan(
		&existing.ID, &existing.Calories, &existing.ProteinG, &existing.CarbsG, &existing.FatG, &existing.FiberG, &existing.WaterMl, &existing.MealNotes,
	)
	if err != nil && err != sql.ErrNoRows {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	if err == sql.ErrNoRows {
		// insert new
		in.ID = uuid.New().String()
		in.UpdatedAt = now
		_, err := h.db.ExecContext(r.Context(), `INSERT INTO nutrition_logs (id,user_id,date,calories,protein_g,carbs_g,fat_g,fiber_g,water_ml,meal_notes,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			in.ID, in.UserID, in.Date, in.Calories, in.ProteinG, in.CarbsG, in.FatG, in.FiberG, in.WaterMl, in.MealNotes, in.UpdatedAt,
		)
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

	_, err = h.db.ExecContext(r.Context(), `UPDATE nutrition_logs SET calories = ?, protein_g = ?, carbs_g = ?, fat_g = ?, fiber_g = ?, water_ml = ?, meal_notes = ?, updated_at = ? WHERE user_id = ? AND date = ?`,
		merged.Calories, merged.ProteinG, merged.CarbsG, merged.FatG, merged.FiberG, merged.WaterMl, merged.MealNotes, merged.UpdatedAt, claims.UserID, in.Date,
	)
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
	var id string
	if err := h.db.QueryRowContext(r.Context(), `SELECT id FROM nutrition_logs WHERE user_id = ? AND date = ?`, claims.UserID, date).Scan(&id); err == sql.ErrNoRows {
		respond.Error(w, http.StatusNotFound, "nutrition log not found")
		return
	} else if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := h.db.ExecContext(r.Context(), `UPDATE nutrition_logs SET calories=?,protein_g=?,carbs_g=?,fat_g=?,fiber_g=?,water_ml=?,meal_notes=?,updated_at=? WHERE user_id=? AND date=?`,
		in.Calories, in.ProteinG, in.CarbsG, in.FatG, in.FiberG, in.WaterMl, in.MealNotes, now, claims.UserID, date,
	)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	// return updated
	var out models.NutritionLog
	if err := h.db.QueryRowContext(r.Context(),
		`SELECT id,user_id,date,calories,protein_g,carbs_g,fat_g,fiber_g,water_ml,meal_notes,updated_at FROM nutrition_logs WHERE user_id = ? AND date = ?`,
		claims.UserID, date,
	).Scan(&out.ID, &out.UserID, &out.Date, &out.Calories, &out.ProteinG, &out.CarbsG, &out.FatG, &out.FiberG, &out.WaterMl, &out.MealNotes, &out.UpdatedAt); err != nil {
		respond.Error(w, http.StatusInternalServerError, "failed to retrieve updated record")
		return
	}
	respond.JSON(w, http.StatusOK, out)
}

// DELETE /v1/nutrition/logs/{date}
func (h *NutritionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	date := chi.URLParam(r, "date")
	_, err := h.db.ExecContext(r.Context(), `DELETE FROM nutrition_logs WHERE user_id = ? AND date = ?`, claims.UserID, date)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
