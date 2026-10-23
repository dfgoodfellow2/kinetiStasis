package gemini

import (
	"context"
	"strings"
	"testing"
)

func TestBuildMealPromptTextOnly(t *testing.T) {
	prompt := buildMealPrompt("1 banana, 1 scoop protein", false)
	if strings.Contains(prompt, "IMAGE RULES") {
		t.Fatal("text-only prompt must not contain image instructions")
	}
	if !strings.Contains(prompt, "Meal: 1 banana, 1 scoop protein") {
		t.Fatal("text-only prompt missing meal text")
	}
	if !strings.Contains(prompt, `"mealNotes"`) {
		t.Fatal("text-only prompt missing response contract")
	}
}

func TestBuildMealPromptWithImage(t *testing.T) {
	prompt := buildMealPrompt("with dressing", true)
	for _, want := range []string{
		"IMAGE RULES",
		"low confidence:",
		"mealNotes",
		"Meal description: with dressing",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("image prompt missing %q", want)
		}
	}
}

func TestBuildMealPromptImageWithoutText(t *testing.T) {
	prompt := buildMealPrompt("   ", true)
	if !strings.Contains(prompt, "identify the meal from the image alone") {
		t.Fatal("image-only prompt should state that no text was supplied")
	}
}

func TestParseMealWithImageRequiresInput(t *testing.T) {
	c := NewClient("test-key")
	if _, err := c.ParseMealWithImage(context.Background(), "   ", nil); err == nil {
		t.Fatal("expected error for empty text and no image")
	}
	if _, err := c.ParseMealWithImage(context.Background(), "", &Image{}); err == nil {
		t.Fatal("expected error for empty image data and no text")
	}
}

func TestImagePresent(t *testing.T) {
	var nilImage *Image
	if nilImage.Present() {
		t.Fatal("nil image should not be present")
	}
	if (&Image{}).Present() {
		t.Fatal("image without data should not be present")
	}
	if !(&Image{Data: []byte{1}}).Present() {
		t.Fatal("image with data should be present")
	}
}

func TestCleanAndExtractJSON(t *testing.T) {
	in := "```json\n{\"calories\": 225, \"mealNotes\": \"ok\"}\n```"
	got := cleanAndExtractJSON(in)
	if got != `{"calories": 225, "mealNotes": "ok"}` {
		t.Fatalf("unexpected cleaned JSON: %q", got)
	}
}
