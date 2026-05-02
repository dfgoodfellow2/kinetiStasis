package handlers

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/auth"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/respond"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type MealsHandler struct{ s store.Store }

func NewMealsHandler(s store.Store) *MealsHandler { return &MealsHandler{s: s} }

// GET /v1/meals/saved
func (h *MealsHandler) ListSaved(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	db := h.s.DB()
	rows, err := db.QueryContext(r.Context(), `SELECT id,user_id,name,calories,protein_g,carbs_g,fat_g,fiber_g,created_at,updated_at FROM saved_meals WHERE user_id = ? ORDER BY name ASC`, claims.UserID)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()
	var out []models.SavedMeal
	for rows.Next() {
		var m models.SavedMeal
		if err := rows.Scan(&m.ID, &m.UserID, &m.Name, &m.Calories, &m.ProteinG, &m.CarbsG, &m.FatG, &m.FiberG, &m.CreatedAt, &m.UpdatedAt); err != nil {
			respond.Error(w, http.StatusInternalServerError, "database error")
			return
		}
		out = append(out, m)
	}
	respond.JSON(w, http.StatusOK, out)
}

// POST /v1/meals/saved
func (h *MealsHandler) CreateSaved(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	var m models.SavedMeal
	if !respond.Decode(w, r, &m) {
		return
	}
	m.UserID = claims.UserID
	now := time.Now().UTC().Format(time.RFC3339)
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	m.CreatedAt = now
	m.UpdatedAt = now
	db := h.s.DB()
	_, err := db.ExecContext(r.Context(), `INSERT INTO saved_meals (id,user_id,name,calories,protein_g,carbs_g,fat_g,fiber_g,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?)`, m.ID, m.UserID, m.Name, m.Calories, m.ProteinG, m.CarbsG, m.FatG, m.FiberG, m.CreatedAt, m.UpdatedAt)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	respond.JSON(w, http.StatusCreated, m)
}

// DELETE /v1/meals/saved/{id}
func (h *MealsHandler) DeleteSaved(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	id := chi.URLParam(r, "id")
	// verify ownership
	var uid string
	db := h.s.DB()
	if err := db.QueryRowContext(r.Context(), `SELECT user_id FROM saved_meals WHERE id = ?`, id).Scan(&uid); err == sql.ErrNoRows {
		respond.Error(w, http.StatusNotFound, "meal not found")
		return
	} else if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	if uid != claims.UserID {
		respond.Error(w, http.StatusForbidden, "not allowed")
		return
	}
	_, err := db.ExecContext(r.Context(), `DELETE FROM saved_meals WHERE id = ?`, id)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// GET /v1/meals/templates
func (h *MealsHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	db := h.s.DB()
	rows, err := db.QueryContext(r.Context(), `SELECT id,user_id,name,meals_json,created_at,updated_at FROM meal_templates WHERE user_id = ? ORDER BY name ASC`, claims.UserID)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()
	var out []models.MealTemplate
	for rows.Next() {
		var id, uid, name, mealsJSON, createdAt, updatedAt string
		if err := rows.Scan(&id, &uid, &name, &mealsJSON, &createdAt, &updatedAt); err != nil {
			respond.Error(w, http.StatusInternalServerError, "database error")
			return
		}
		var meals []models.SavedMeal
		if mealsJSON != "" {
			if err := json.Unmarshal([]byte(mealsJSON), &meals); err != nil {
				slog.Warn("unmarshal meals_json failed", "err", err)
			}
		}
		out = append(out, models.MealTemplate{ID: id, UserID: uid, Name: name, Meals: meals, CreatedAt: createdAt, UpdatedAt: updatedAt})
	}
	respond.JSON(w, http.StatusOK, out)
}

// POST /v1/meals/templates
func (h *MealsHandler) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	var in models.MealTemplate
	if !respond.Decode(w, r, &in) {
		return
	}
	in.UserID = claims.UserID
	now := time.Now().UTC().Format(time.RFC3339)
	if in.ID == "" {
		in.ID = uuid.New().String()
	}
	in.CreatedAt = now
	in.UpdatedAt = now
	b, _ := json.Marshal(in.Meals)
	db := h.s.DB()
	_, err := db.ExecContext(r.Context(), `INSERT INTO meal_templates (id,user_id,name,meals_json,created_at,updated_at) VALUES (?,?,?,?,?,?)`, in.ID, in.UserID, in.Name, string(b), in.CreatedAt, in.UpdatedAt)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	respond.JSON(w, http.StatusCreated, in)
}

// DELETE /v1/meals/templates/{id}
func (h *MealsHandler) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	id := chi.URLParam(r, "id")
	var uid string
	db := h.s.DB()
	if err := db.QueryRowContext(r.Context(), `SELECT user_id FROM meal_templates WHERE id = ?`, id).Scan(&uid); err == sql.ErrNoRows {
		respond.Error(w, http.StatusNotFound, "template not found")
		return
	} else if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	if uid != claims.UserID {
		respond.Error(w, http.StatusForbidden, "not allowed")
		return
	}
	_, err := db.ExecContext(r.Context(), `DELETE FROM meal_templates WHERE id = ?`, id)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
