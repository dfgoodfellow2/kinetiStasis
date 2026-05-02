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

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
)

const modelName = "gemini-2.5-flash-lite"

// Client holds API configuration
type Client struct {
	apiKey string
}

// NewClient constructs a Gemini client wrapper
func NewClient(apiKey string) *Client {
	return &Client{apiKey: apiKey}
}

// ParseMeal sends meal text to Gemini and parses a JSON response into models.ParsedMeal
func (c *Client) ParseMeal(ctx context.Context, text string) (models.ParsedMeal, error) {
	var out models.ParsedMeal
	if strings.TrimSpace(text) == "" {
		return out, errors.New("input text empty")
	}
	if c == nil || c.apiKey == "" {
		return out, errors.New("api key not provided")
	}

	prompt := fmt.Sprintf(`You are a nutrition expert. Parse this meal description and return ONLY a JSON object.

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

Meal: %s`, text)
	respText, err := callGemini(ctx, c.apiKey, prompt)
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

// ParseWorkout sends workout text to Gemini and parses a JSON response into models.ParsedWorkout
// ParseWorkout sends workout text to Gemini and parses a JSON response into models.ParsedWorkout.
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
  "style": "circuit|emom|amrap|for-time|hiit|cardio|\"\"",
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
      "durationRaw": "string"
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

WORKOUT: %s`, text)

	respText, err := callGemini(ctx, c.apiKey, prompt)
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

// callGemini calls the Gemini API with the given prompt and returns the raw text response.
// It automatically retries once on 429 (rate-limited) or 503 (service unavailable) errors.
func callGemini(ctx context.Context, apiKey, prompt string) (string, error) {
	attempt := func() (string, error) {
		client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
		if err != nil {
			return "", fmt.Errorf("failed to create gemini client: %w", err)
		}
		defer client.Close()

		mdl := client.GenerativeModel(modelName)
		resp, err := mdl.GenerateContent(ctx, genai.Text(prompt))
		if err != nil {
			return "", err
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
