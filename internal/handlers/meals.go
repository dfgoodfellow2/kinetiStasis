package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/auth"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/respond"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/validate"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type MealsHandler struct{ s store.Store }

func NewMealsHandler(s store.Store) *MealsHandler { return &MealsHandler{s: s} }

const (
	maxTemplateNameLen   = 64
	maxTemplateMeals     = 20
	maxTemplateMealName  = 64
	maxTemplateMealNotes = 2000
)

// GET /v1/meals/saved
func (h *MealsHandler) ListSaved(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	out, err := h.s.FetchSavedMeals(r.Context(), claims.UserID)
	if err != nil {
		slog.Error("fetch saved meals", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
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
	if err := h.s.CreateSavedMeal(r.Context(), &m); err != nil {
		slog.Error("create saved meal", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	respond.JSON(w, http.StatusCreated, m)
}

// DELETE /v1/meals/saved/{id}
func (h *MealsHandler) DeleteSaved(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	id := chi.URLParam(r, "id")
	// delete via store (store will ensure ownership)
	if err := h.s.DeleteSavedMeal(r.Context(), claims.UserID, id); err != nil {
		slog.Error("delete saved meal", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// GET /v1/meals/templates
func (h *MealsHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	out, err := h.s.FetchMealTemplates(r.Context(), claims.UserID)
	if err != nil {
		slog.Error("fetch meal templates", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
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

	// Templates are a curated bundle, so enforce a human-scale shape before
	// touching the store. Names are trimmed first so a whitespace-only name
	// cannot masquerade as a valid one.
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		respond.Error(w, http.StatusBadRequest, "template name is required")
		return
	}
	if utf8.RuneCountInString(in.Name) > maxTemplateNameLen {
		respond.Error(w, http.StatusBadRequest, fmt.Sprintf("template name must be at most %d characters", maxTemplateNameLen))
		return
	}
	if len(in.Meals) == 0 {
		respond.Error(w, http.StatusBadRequest, "template must contain at least one meal")
		return
	}
	if len(in.Meals) > maxTemplateMeals {
		respond.Error(w, http.StatusBadRequest, fmt.Sprintf("template can contain at most %d meals", maxTemplateMeals))
		return
	}
	for i := range in.Meals {
		m := &in.Meals[i]
		m.UserID = claims.UserID
		m.Name = strings.TrimSpace(m.Name)
		if m.Name == "" {
			respond.Error(w, http.StatusBadRequest, fmt.Sprintf("meal %d needs a name", i+1))
			return
		}
		if utf8.RuneCountInString(m.Name) > maxTemplateMealName {
			respond.Error(w, http.StatusBadRequest, fmt.Sprintf("meal %d name must be at most %d characters", i+1, maxTemplateMealName))
			return
		}
		if utf8.RuneCountInString(m.Notes) > maxTemplateMealNotes {
			respond.Error(w, http.StatusBadRequest, fmt.Sprintf("meal %d notes must be at most %d characters", i+1, maxTemplateMealNotes))
			return
		}
		// Reuse the nutrition bounds so a template entry cannot smuggle in
		// negative or absurd macros that the log endpoint would reject.
		for _, f := range []struct {
			name string
			v    float64
			max  float64
		}{
			{"calories", m.Calories, maxNutritionCalories},
			{"proteinG", m.ProteinG, maxNutritionMacroG},
			{"carbsG", m.CarbsG, maxNutritionMacroG},
			{"fatG", m.FatG, maxNutritionMacroG},
			{"fiberG", m.FiberG, maxNutritionFiberG},
		} {
			if err := validate.NonNegativeBounded(f.name, f.v, f.max); err != nil {
				respond.Error(w, http.StatusBadRequest, fmt.Sprintf("meal %d: %v", i+1, err))
				return
			}
		}
	}

	in.UserID = claims.UserID
	// Templates are always born private; group sharing goes exclusively through
	// the owner-only share endpoint so a client-supplied groupId is ignored.
	in.GroupID = ""
	now := time.Now().UTC().Format(time.RFC3339)
	if in.ID == "" {
		in.ID = uuid.New().String()
	}
	in.CreatedAt = now
	in.UpdatedAt = now
	if err := h.s.CreateMealTemplate(r.Context(), &in); err != nil {
		slog.Error("create meal template", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	respond.JSON(w, http.StatusCreated, in)
}

// DELETE /v1/meals/templates/{id}
func (h *MealsHandler) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	id := chi.URLParam(r, "id")
	// verify ownership via templates list
	templates, err := h.s.FetchMealTemplates(r.Context(), claims.UserID)
	if err != nil {
		slog.Error("fetch meal templates for delete", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	// find the template (own or shared to one of the caller's groups) so the
	// response distinguishes "missing" from "not yours".
	found := (*models.MealTemplate)(nil)
	for i := range templates {
		if templates[i].ID == id {
			found = &templates[i]
			break
		}
	}
	if found == nil {
		respond.Error(w, http.StatusNotFound, "template not found")
		return
	}
	// Group owners may unshare a template but never delete it; only the owner
	// may delete.
	if found.UserID != claims.UserID {
		respond.Error(w, http.StatusForbidden, "only the template owner can delete a template")
		return
	}
	if err := h.s.DeleteMealTemplate(r.Context(), claims.UserID, id); err != nil {
		slog.Error("delete meal template", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
