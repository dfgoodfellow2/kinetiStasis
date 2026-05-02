package handlers

import (
	"net/http"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/respond"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/services/gemini"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/services/workout"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"
)

type ParseHandler struct {
	s      store.Store
	gemini *gemini.Client
}

func NewParseHandler(s store.Store, geminiClient *gemini.Client) *ParseHandler {
	return &ParseHandler{s: s, gemini: geminiClient}
}

// POST /v1/parse/meal
func (h *ParseHandler) Meal(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text string `json:"text"`
	}
	if !respond.Decode(w, r, &body) {
		return
	}
	if body.Text == "" {
		respond.Error(w, http.StatusBadRequest, "text is required")
		return
	}
	if h.gemini == nil {
		respond.Error(w, http.StatusServiceUnavailable, "AI parsing unavailable")
		return
	}
	parsed, err := h.gemini.ParseMeal(r.Context(), body.Text)
	if err != nil {
		respond.Error(w, http.StatusBadGateway, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, parsed)
}

// POST /v1/parse/workout
// Body: { "text": "...", "format": "yaml" | "ai" }
// format defaults to "ai" when omitted or unrecognised.
func (h *ParseHandler) Workout(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text   string `json:"text"`
		Format string `json:"format"`
	}
	if !respond.Decode(w, r, &body) {
		return
	}
	if body.Text == "" {
		respond.Error(w, http.StatusBadRequest, "text is required")
		return
	}

	if body.Format == "yaml" {
		parsed, err := workout.ParseYAML(body.Text)
		if err != nil {
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		respond.JSON(w, http.StatusOK, parsed)
		return
	}

	// Default: AI / Gemini path
	if h.gemini == nil {
		respond.Error(w, http.StatusServiceUnavailable, "AI parsing unavailable")
		return
	}
	parsed, err := h.gemini.ParseWorkout(r.Context(), body.Text)
	if err != nil {
		respond.Error(w, http.StatusBadGateway, err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, parsed)
}
