package handlers

import (
	"encoding/base64"
	"errors"
	"strings"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/services/gemini"
)

// maxMealImageBytes is the maximum decoded size of an inline meal photo.
const maxMealImageBytes = 4 << 20 // 4 MiB

// allowedMealImageMIMEs lists the content types accepted for inline meal photos.
var allowedMealImageMIMEs = map[string]struct{}{
	"image/jpeg": {},
	"image/png":  {},
	"image/webp": {},
}

// mealParseRequest mirrors the JSON body accepted by POST /v1/parse/meal.
// ImageBase64 must be raw base64 (the client strips any data: prefix).
type mealParseRequest struct {
	Text        string `json:"text"`
	ImageBase64 string `json:"imageBase64"`
	MimeType    string `json:"mimeType"`
}

// mealParseInput is the validated, decoded form handed to the Gemini service.
type mealParseInput struct {
	Text  string
	Image *gemini.Image
}

// validateMealRequest enforces the parse/meal contract: at least one of text or
// imageBase64, a supported mime type for images, and a decoded image no larger
// than 4 MiB. Image bytes are decoded in memory only and are never logged.
func validateMealRequest(req mealParseRequest) (mealParseInput, error) {
	var in mealParseInput
	in.Text = req.Text

	hasImage := req.ImageBase64 != ""
	if strings.TrimSpace(req.Text) == "" && !hasImage {
		return in, errors.New("text or imageBase64 is required")
	}

	mimeType := strings.TrimSpace(req.MimeType)
	if mimeType != "" {
		if _, ok := allowedMealImageMIMEs[mimeType]; !ok {
			return in, errors.New("mimeType must be image/jpeg, image/png, or image/webp")
		}
	}
	if !hasImage {
		return in, nil
	}
	if mimeType == "" {
		return in, errors.New("mimeType must be image/jpeg, image/png, or image/webp")
	}

	data, err := base64.StdEncoding.DecodeString(req.ImageBase64)
	if err != nil {
		return in, errors.New("imageBase64 must be valid base64 without a data: prefix")
	}
	if len(data) > maxMealImageBytes {
		return in, errors.New("imageBase64 exceeds 4MB decoded limit")
	}
	if len(data) == 0 {
		return in, errors.New("imageBase64 must not be empty")
	}

	in.Image = &gemini.Image{MIMEType: mimeType, Data: data}
	return in, nil
}
