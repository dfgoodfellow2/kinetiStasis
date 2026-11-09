package validate

import (
	"strings"
	"testing"
)

func TestUsername(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid lowercase", input: "alice"},
		{name: "valid allowed punctuation", input: "a.b_c-d"},
		{name: "valid digits", input: "user123"},
		{name: "valid minimum length", input: "abc"},
		{name: "valid maximum length", input: strings.Repeat("a", 32)},
		{name: "too short", input: "ab", wantErr: true},
		{name: "too long", input: strings.Repeat("a", 33), wantErr: true},
		{name: "space", input: "bad name", wantErr: true},
		{name: "at sign", input: "bad@name", wantErr: true},
		{name: "slash", input: "bad/name", wantErr: true},
		{name: "non-ascii", input: "usér", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Username(tt.input)
			if tt.wantErr && err == nil {
				t.Fatalf("Username(%q) = nil, want error", tt.input)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Username(%q) = %v, want nil", tt.input, err)
			}
		})
	}
}

func TestEmail(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid", input: "user@example.com"},
		{name: "valid plus tag", input: "user+tag@example.com"},
		{name: "valid subdomain", input: "a@b.example.com"},
		{name: "valid maximum length", input: strings.Repeat("a", 242) + "@example.com"},
		{name: "missing at", input: "not-an-email", wantErr: true},
		{name: "two at signs", input: "two@@example.com", wantErr: true},
		{name: "space", input: "spaced address@example.com", wantErr: true},
		{name: "missing domain", input: "user@", wantErr: true},
		{name: "missing local part", input: "@example.com", wantErr: true},
		{name: "too long", input: strings.Repeat("a", 245) + "@example.com", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Email(tt.input)
			if tt.wantErr && err == nil {
				t.Fatalf("Email(%q) = nil, want error", tt.input)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Email(%q) = %v, want nil", tt.input, err)
			}
		})
	}
}
