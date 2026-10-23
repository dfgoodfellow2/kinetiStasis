package handlers

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
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
