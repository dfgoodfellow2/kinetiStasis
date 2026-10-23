package validate

import (
	"math"
	"testing"
)

func TestDate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid", input: "2026-10-23"},
		{name: "valid leap day", input: "2024-02-29"},
		{name: "empty", input: "", wantErr: true},
		{name: "non-padded month", input: "2026-1-23", wantErr: true},
		{name: "non-padded day", input: "2026-10-3", wantErr: true},
		{name: "out of range day", input: "2026-02-30", wantErr: true},
		{name: "invalid month", input: "2026-13-01", wantErr: true},
		{name: "timestamp", input: "2026-10-23T12:00:00Z", wantErr: true},
		{name: "garbage", input: "not-a-date", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Date(tt.input)
			if tt.wantErr && err == nil {
				t.Fatalf("Date(%q) = nil, want error", tt.input)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Date(%q) = %v, want nil", tt.input, err)
			}
		})
	}
}

func TestNonNegativeBounded(t *testing.T) {
	tests := []struct {
		name    string
		v       float64
		max     float64
		wantErr bool
	}{
		{name: "valid middle", v: 50, max: 100},
		{name: "valid lower bound", v: 0, max: 100},
		{name: "valid upper bound", v: 100, max: 100},
		{name: "negative", v: -0.01, max: 100, wantErr: true},
		{name: "over max", v: 100.01, max: 100, wantErr: true},
		{name: "NaN", v: math.NaN(), max: 100, wantErr: true},
		{name: "positive infinity", v: math.Inf(1), max: 100, wantErr: true},
		{name: "negative infinity", v: math.Inf(-1), max: 100, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NonNegativeBounded("field", tt.v, tt.max)
			if tt.wantErr && err == nil {
				t.Fatalf("NonNegativeBounded(%v, %v) = nil, want error", tt.v, tt.max)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("NonNegativeBounded(%v, %v) = %v, want nil", tt.v, tt.max, err)
			}
		})
	}
}

func TestNonNegativeBoundedInt(t *testing.T) {
	tests := []struct {
		name    string
		v       int
		max     int
		wantErr bool
	}{
		{name: "valid middle", v: 5, max: 10},
		{name: "valid lower bound", v: 0, max: 10},
		{name: "valid upper bound", v: 10, max: 10},
		{name: "negative", v: -1, max: 10, wantErr: true},
		{name: "over max", v: 11, max: 10, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NonNegativeBoundedInt("field", tt.v, tt.max)
			if tt.wantErr && err == nil {
				t.Fatalf("NonNegativeBoundedInt(%d, %d) = nil, want error", tt.v, tt.max)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("NonNegativeBoundedInt(%d, %d) = %v, want nil", tt.v, tt.max, err)
			}
		})
	}
}
