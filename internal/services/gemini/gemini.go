package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	genai "github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/config"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
)

const (
	modelName       = config.DefaultGeminiModel
	visionModelName = config.DefaultGeminiVisionModel
)

// Image is an inline image supplied to a multimodal parse request.
// Data holds decoded bytes, never a base64 string or data: URL.
type Image struct {
	MIMEType string
	Data     []byte
}

// Present reports whether the image carries usable data.
func (i *Image) Present() bool {
	return i != nil && len(i.Data) > 0
}

// Client holds API configuration
type Client struct {
	apiKey      string
	textModel   string
	visionModel string
}

// NewClient constructs a Gemini client wrapper using the model names resolved
// from configuration (GEMINI_MODEL / GEMINI_VISION_MODEL with built-in defaults).
func NewClient(apiKey string) *Client {
	textModel, visionModel := config.GeminiModels()
	return NewClientWithModels(apiKey, textModel, visionModel)
}

// NewClientWithModels constructs a client with explicit model names. Empty
// model names fall back to the package defaults.
func NewClientWithModels(apiKey, textModel, visionModel string) *Client {
	if textModel == "" {
		textModel = modelName
	}
	if visionModel == "" {
		visionModel = visionModelName
	}
	return &Client{apiKey: apiKey, textModel: textModel, visionModel: visionModel}
}

// ParseMeal sends text-only meal input to Gemini and parses a JSON response
// into models.ParsedMeal. It is kept as a convenience wrapper around
// ParseMealWithImage so existing callers do not need to change.
func (c *Client) ParseMeal(ctx context.Context, text string) (models.ParsedMeal, error) {
	return c.ParseMealWithImage(ctx, text, nil)
}

// ParseMealWithImage parses a meal description and/or an inline photo into
// models.ParsedMeal. At least one of text or image must be supplied. Image
// bytes are sent to the model and discarded; they are never persisted or
// logged. The vision model is selected whenever an image is present.
func (c *Client) ParseMealWithImage(ctx context.Context, text string, image *Image) (models.ParsedMeal, error) {
	var out models.ParsedMeal
	hasImage := image.Present()
	if strings.TrimSpace(text) == "" && !hasImage {
		return out, errors.New("input text or image required")
	}
	if hasImage && strings.TrimSpace(image.MIMEType) == "" {
		return out, errors.New("image content type required")
	}
	if c == nil || c.apiKey == "" {
		return out, errors.New("api key not provided")
	}

	prompt := buildMealPrompt(text, hasImage)
	model := c.textModel
	if hasImage {
		model = c.visionModel
	}
	respText, err := callGemini(ctx, c.apiKey, model, prompt, image)
	if err != nil {
		return out, err
	}

	respText = cleanAndExtractJSON(respText)

	var parsed models.ParsedMeal
	if err := json.Unmarshal([]byte(respText), &parsed); err != nil {
		return out, fmt.Errorf("failed to parse JSON: %w (text: %s)", err, respText)
	}
	parsed.RawInput = text
	return parsed, nil
}

const textMealPrompt = `You are a nutrition expert. Parse this meal description and return ONLY a JSON object.

Required JSON keys (use EXACTLY these names):
- calories (number) - total calories
- proteinG (number) - protein in grams  
- carbsG (number) - carbohydrates in grams
- fatG (number) - fat in grams
- fiberG (number) - fiber in grams
- waterMl (number) - water in milliliters
- mealNotes (string) - brief description of the meal

CALCULATION RULES:
1. For each food item, estimate calories and macros based on standard nutrition data
2. Banana (1 medium, ~120g): ~105 cal, 1g protein, 27g carbs, 0g fat, 3g fiber
3. Protein powder (1 scoop, ~30g): ~120 cal, 24g protein, 3g carbs, 1g fat, 0g fiber
4. Sum up all ingredients
5. If uncertain, provide reasonable estimates based on typical portions

EXAMPLE:
Input: "1 banana, 1 scoop protein"
Output: {"calories": 225, "proteinG": 25, "carbsG": 30, "fatG": 1, "fiberG": 3, "waterMl": 0, "mealNotes": "1 banana, 1 scoop protein"}

Return ONLY the JSON object, no other text.

Meal: %s`

const imageMealPrompt = `You are a nutrition expert. Identify the foods and drinks in the attached meal photo and return ONLY a JSON object.

Required JSON keys (use EXACTLY these names):
- calories (number) - total calories
- proteinG (number) - protein in grams
- carbsG (number) - carbohydrates in grams
- fatG (number) - fat in grams
- fiberG (number) - fiber in grams
- waterMl (number) - water in milliliters
- mealNotes (string) - brief description of the meal

IMAGE RULES:
1. Identify every distinct food and drink item visible in the image
2. Estimate portions from visual context: plate and utensil scale, packaging, slice counts, fill level, and typical serving sizes
3. Use any supplied text description to disambiguate items or portions
4. Estimate calories and macros from standard nutrition data, then sum all items
5. If an item is visually ambiguous or its portion size is uncertain, include a best estimate and flag it in mealNotes prefixed with "low confidence: " (for example, "low confidence: dressing amount unclear")
6. Do not omit an item just because it is uncertain

Return ONLY the JSON object, no other text.

Meal description: %s`

// buildMealPrompt returns the original text-only prompt unchanged, or a prompt
// extended with visual-estimation instructions when an image is supplied.
func buildMealPrompt(text string, hasImage bool) string {
	if !hasImage {
		return fmt.Sprintf(textMealPrompt, text)
	}
	description := strings.TrimSpace(text)
	if description == "" {
		description = "(none - identify the meal from the image alone)"
	}
	return fmt.Sprintf(imageMealPrompt, description)
}

// ParseWorkout sends workout text to Gemini and parses a JSON response into models.ParsedWorkout
func (c *Client) ParseWorkout(ctx context.Context, text string) (models.ParsedWorkout, error) {
	var out models.ParsedWorkout
	if strings.TrimSpace(text) == "" {
		return out, errors.New("input text empty")
	}
	if c == nil || c.apiKey == "" {
		return out, errors.New("api key not provided")
	}

	prompt := fmt.Sprintf(`You are a fitness coach. Parse this workout and return ONLY a JSON object with these exact fields:
{
  "title": "string",
  "slot": "string",
  "type": "strength|conditioning|hiit|cardio|zone2|mobility|sport|yoga",
  "surface": "string",
  "focus": ["string"],
  "restInterval": "string",
  "durationMin": number,
  "rpe": number,
  "avgHr": number,
  "maxHr": number,
  "caloriesBurned": number,
  "recovers": "string",
  "exercises": [
    {
      "name": "string",
      "category": "squat|hinge|push|pull|conditioning|core|carry",
      "bias": "bilateral|unilateral|\"\"",
      "tempo": "string e.g. 2-0-2-0 or \"\"",
      "sets": [{"reps": number, "loadLbs": number, "tutSeconds": number, "restSeconds": number}],
      "metValue": number,
      "distanceKm": number,
      "elevationM": number,
      "pace": "string",
      "rpe": number,
      "loadRaw": "string",
      "durationRaw": "string",
      "phase": "\"warmup\"|\"main\"|\"cooldown\"|\"\"",
      "groupId": "string",
      "groupOrder": "number",
      "groupStyle": "\"circuit\"|\"superset\"|\"cluster\"|\"straight\"|\"\"",
      "targetHrZone": "number",
      "hrZoneTime": "object"
    }
  ]
}

Rules:
- sets is an ARRAY of set objects, one element per set (repeat identical sets)
- loadRaw: preserve the original load string ("BW", "35+35 lbs", "50 lbs")
- tutSeconds per set: sum tempo phases × reps, OR duration in seconds for timed sets
- focus: array of movement patterns with bilateral/unilateral bias e.g. ["Hinge(B)", "Push(U)"]
- bias: "bilateral" for two-limb movements (B), "unilateral" for single-limb (U), "" if unknown
- tempo: preserve as string e.g. "3-1-2-1" (eccentric-pause-concentric-pause), "" if not specified
- Use 0 for unknown numbers, "" for unknown strings, [] for unknown arrays
- Return ONLY valid JSON, no explanation

- phase: "warmup"|"main"|"cooldown"|"" — identifies if exercise is a warmup, main work, or cooldown
- groupId: if exercises belong to a circuit or superset, give them the same groupId
- groupOrder: position within the circuit/superset (0-based)
- groupStyle: "circuit" for circuits, "superset" for supersets, "straight" for standalone sets
- targetHrZone: intended heart rate zone (1-5) for the exercise
- hrZoneTime: object with zone numbers as keys and seconds as values e.g. {"2": 720, "3": 180}

WORKOUT: %s`, text)

	respText, err := callGemini(ctx, c.apiKey, c.textModel, prompt, nil)
	if err != nil {
		return out, err
	}

	respText = cleanAndExtractJSON(respText)

	// Unmarshal directly into ParsedWorkout (fields now match 1:1)
	if err := json.Unmarshal([]byte(respText), &out); err != nil {
		return out, fmt.Errorf("failed to parse JSON: %w (text: %s)", err, respText)
	}
	out.RawInput = text
	return out, nil
}

// callGemini calls the Gemini API with the given prompt (and optional inline
// image) and returns the raw text response. It automatically retries once on
// 429 (rate-limited) or 503 (service unavailable) errors.
func callGemini(ctx context.Context, apiKey, model, prompt string, image *Image) (string, error) {
	if model == "" {
		model = modelName
	}
	attempt := func() (string, error) {
		client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
		if err != nil {
			return "", fmt.Errorf("failed to create gemini client: %w", err)
		}
		defer client.Close()

		mdl := client.GenerativeModel(model)
		parts := make([]genai.Part, 0, 2)
		if image.Present() {
			parts = append(parts, genai.Blob{MIMEType: image.MIMEType, Data: image.Data})
		}
		parts = append(parts, genai.Text(prompt))

		resp, err := mdl.GenerateContent(ctx, parts...)
		if err != nil {
			return "", fmt.Errorf("generate content: %w", err)
		}
		if resp == nil || len(resp.Candidates) == 0 {
			return "", errors.New("empty response from API")
		}
		var out string
		for _, part := range resp.Candidates[0].Content.Parts {
			if t, ok := part.(genai.Text); ok {
				out += string(t)
			}
		}
		return strings.TrimSpace(out), nil
	}

	text, err := attempt()
	if err != nil && (strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "503")) {
		time.Sleep(2 * time.Second)
		text, err = attempt()
	}
	if err != nil {
		return "", fmt.Errorf("gemini error: %w", err)
	}
	return text, nil
}

// cleanAndExtractJSON strips markdown code fences and extracts the first JSON object
// from the string by brace-counting. Returns the cleaned JSON string.
func cleanAndExtractJSON(s string) string {
	s = strings.TrimSpace(s)
	// Strip code fences
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
	}
	if strings.HasSuffix(s, "```") {
		s = strings.TrimSuffix(s, "```")
	}
	s = strings.TrimSpace(s)

	// Extract first JSON object by brace-counting
	idx := strings.Index(s, "{")
	if idx < 0 {
		return s
	}
	depth := 0
	start := idx
	for i := idx; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return s
}
