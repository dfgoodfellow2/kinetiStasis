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

	prompt := fmt.Sprintf(`Parse this meal description and return ONLY a JSON object with these exact keys:
- calories (number)
- protein_g (number)
- carbs_g (number)
- fat_g (number)
- fiber_g (number)
- water_ml (number)
- meal_notes (string)

If you cannot determine exact values, make reasonable estimates. Return ONLY the JSON object, no other text.

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
  "rest_interval": "string",
  "duration_min": number,
  "rpe": number,
  "avg_hr": number,
  "max_hr": number,
  "calories_burned": number,
  "recovers": "string",
  "exercises": [
    {
      "name": "string",
      "category": "squat|hinge|push|pull|conditioning|core|carry",
      "bias": "bilateral|unilateral|\"\"",
      "tempo": "string e.g. 2-0-2-0 or \"\"",
      "sets": [{"reps": number, "load_lbs": number, "tut_seconds": number, "rest_seconds": number}],
      "met_value": number,
      "distance_km": number,
      "elevation_m": number,
      "pace": "string",
      "rpe": number,
      "load_raw": "string",
      "duration_raw": "string"
    }
  ]
}

Rules:
- sets is an ARRAY of set objects, one element per set (repeat identical sets)
- load_raw: preserve the original load string ("BW", "35+35 lbs", "50 lbs")
- tut_seconds per set: sum tempo phases × reps, OR duration in seconds for timed sets
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
