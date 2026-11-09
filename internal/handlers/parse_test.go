package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/constants"
)

func TestValidateMealRequestTextOnly(t *testing.T) {
	in, err := validateMealRequest(mealParseRequest{Text: "1 banana"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if in.Text != "1 banana" || in.Image != nil {
		t.Fatalf("unexpected input: %+v", in)
	}
}

func TestValidateMealRequestRequiresTextOrImage(t *testing.T) {
	for _, req := range []mealParseRequest{
		{},
		{Text: "   "},
		{Text: "\n\t"},
	} {
		if _, err := validateMealRequest(req); err == nil {
			t.Fatalf("expected error for empty request %+v", req)
		}
	}
}

func TestValidateMealRequestImageOnly(t *testing.T) {
	raw := []byte("fake-png-bytes")
	req := mealParseRequest{
		ImageBase64: base64.StdEncoding.EncodeToString(raw),
		MimeType:    "image/png",
	}
	in, err := validateMealRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if in.Image == nil {
		t.Fatal("expected decoded image")
	}
	if in.Image.MIMEType != "image/png" {
		t.Fatalf("unexpected mime type: %q", in.Image.MIMEType)
	}
	if !bytes.Equal(in.Image.Data, raw) {
		t.Fatalf("decoded image mismatch: %q", in.Image.Data)
	}
}

func TestValidateMealRequestImageWithText(t *testing.T) {
	req := mealParseRequest{
		Text:        "half the plate was rice",
		ImageBase64: base64.StdEncoding.EncodeToString([]byte{0x01}),
		MimeType:    "image/jpeg",
	}
	in, err := validateMealRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if in.Text != req.Text || in.Image == nil {
		t.Fatalf("unexpected input: %+v", in)
	}
}

func TestValidateMealRequestRejectsUnsupportedMime(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte{0x01})
	for _, mime := range []string{"", "image/gif", "image/jpg", "IMAGE/PNG", "text/plain"} {
		req := mealParseRequest{ImageBase64: encoded, MimeType: mime}
		if _, err := validateMealRequest(req); err == nil {
			t.Fatalf("expected error for mime type %q", mime)
		}
	}
	if _, err := validateMealRequest(mealParseRequest{Text: "1 banana", MimeType: "image/gif"}); err == nil {
		t.Fatal("expected error for unsupported mime type on text-only request")
	}
	if _, err := validateMealRequest(mealParseRequest{Text: "1 banana", MimeType: "image/png"}); err != nil {
		t.Fatalf("text-only request with supported mime type should pass: %v", err)
	}
}

func TestValidateMealRequestRejectsInvalidBase64(t *testing.T) {
	req := mealParseRequest{
		ImageBase64: "data:image/png;base64,AAAA",
		MimeType:    "image/png",
	}
	if _, err := validateMealRequest(req); err == nil {
		t.Fatal("expected error for data: prefix")
	} else if strings.Contains(err.Error(), "data:image") {
		t.Fatal("validation error must not echo image data")
	}
}

func TestValidateMealRequestRejectsOversizedImage(t *testing.T) {
	raw := make([]byte, maxMealImageBytes+1)
	req := mealParseRequest{
		ImageBase64: base64.StdEncoding.EncodeToString(raw),
		MimeType:    "image/webp",
	}
	if _, err := validateMealRequest(req); err == nil {
		t.Fatal("expected error for image over 4MB")
	}
}

func TestValidateMealRequestRejectsOversizedText(t *testing.T) {
	overLimit := strings.Repeat("a", constants.MaxParseTextChars+1)
	if _, err := validateMealRequest(mealParseRequest{Text: overLimit}); err == nil {
		t.Fatal("expected error for text over the parse cap")
	} else if !strings.Contains(err.Error(), "description too long") {
		t.Fatalf("unexpected error: %v", err)
	}

	atLimit := strings.Repeat("a", constants.MaxParseTextChars)
	in, err := validateMealRequest(mealParseRequest{Text: atLimit})
	if err != nil {
		t.Fatalf("text exactly at the cap should pass: %v", err)
	}
	if len(in.Text) != constants.MaxParseTextChars {
		t.Fatalf("text truncated unexpectedly: %d chars", len(in.Text))
	}
}

func TestValidateMealRequestPhotoOnlyUnaffectedByTextCap(t *testing.T) {
	req := mealParseRequest{
		ImageBase64: base64.StdEncoding.EncodeToString([]byte("fake-jpeg-bytes")),
		MimeType:    "image/jpeg",
	}
	in, err := validateMealRequest(req)
	if err != nil {
		t.Fatalf("photo-only request must pass validation: %v", err)
	}
	if strings.TrimSpace(in.Text) != "" {
		t.Fatalf("unexpected text: %q", in.Text)
	}
	if in.Image == nil || len(in.Image.Data) == 0 {
		t.Fatal("expected decoded image for photo-only request")
	}
}

func postParseJSON(t *testing.T, h http.HandlerFunc, body any) *httptest.ResponseRecorder {
	t.Helper()
	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(buf))
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func TestParseMealHandlerRejectsOversizedText(t *testing.T) {
	h := &ParseHandler{}
	rec := postParseJSON(t, h.Meal, mealParseRequest{
		Text: strings.Repeat("a", constants.MaxParseTextChars+1),
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "description too long") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestParseMealHandlerPhotoOnlyReachesModelPath(t *testing.T) {
	h := &ParseHandler{}
	rec := postParseJSON(t, h.Meal, mealParseRequest{
		ImageBase64: base64.StdEncoding.EncodeToString([]byte("fake-png-bytes")),
		MimeType:    "image/png",
	})
	// No Gemini client is configured, so validation succeeding surfaces as a
	// 503 (AI unavailable) rather than the 400 used for an empty request.
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}
}

func TestParseWorkoutHandlerRejectsOversizedText(t *testing.T) {
	h := &ParseHandler{}
	rec := postParseJSON(t, h.Workout, struct {
		Text   string `json:"text"`
		Format string `json:"format"`
	}{
		Text: strings.Repeat("a", constants.MaxParseTextChars+1),
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "description too long") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestValidateMealRequestAcceptsImageAtLimit(t *testing.T) {
	raw := make([]byte, maxMealImageBytes)
	req := mealParseRequest{
		ImageBase64: base64.StdEncoding.EncodeToString(raw),
		MimeType:    "image/jpeg",
	}
	in, err := validateMealRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(in.Image.Data) != maxMealImageBytes {
		t.Fatalf("expected %d bytes, got %d", maxMealImageBytes, len(in.Image.Data))
	}
}
