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
// Body: { "text"?: string, "imageBase64"?: string, "mimeType"?: string }
// At least one of text or imageBase64 is required. imageBase64 is raw base64
// (no data: prefix) and mimeType must be image/jpeg, image/png, or image/webp.
// Image bytes are parsed and discarded — never persisted or logged.
func (h *ParseHandler) Meal(w http.ResponseWriter, r *http.Request) {
	var body mealParseRequest
	if !respond.Decode(w, r, &body) {
		return
	}
	input, err := validateMealRequest(body)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if h.gemini == nil {
		respond.Error(w, http.StatusServiceUnavailable, "AI parsing unavailable")
		return
	}
	parsed, err := h.gemini.ParseMealWithImage(r.Context(), input.Text, input.Image)
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
