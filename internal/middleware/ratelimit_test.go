package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		xff        string
		remoteAddr string
		want       string
	}{
		{
			name:       "spoofed first entry ignored, last hop is key",
			xff:        "1.2.3.4, 5.6.7.8",
			remoteAddr: "10.0.0.1:1234",
			want:       "5.6.7.8",
		},
		{
			name:       "multiple hops uses final entry",
			xff:        " 1.1.1.1 , 2.2.2.2 , 3.3.3.3 ",
			remoteAddr: "10.0.0.1:1234",
			want:       "3.3.3.3",
		},
		{
			name:       "single xff entry",
			xff:        "9.9.9.9",
			remoteAddr: "10.0.0.1:1234",
			want:       "9.9.9.9",
		},
		{
			name:       "trailing empty entries skipped",
			xff:        "1.2.3.4, ",
			remoteAddr: "10.0.0.1:1234",
			want:       "1.2.3.4",
		},
		{
			name:       "empty xff falls back to remote addr host",
			remoteAddr: "192.0.2.10:54321",
			want:       "192.0.2.10",
		},
		{
			name:       "remote addr without port",
			remoteAddr: "192.0.2.11",
			want:       "192.0.2.11",
		},
		{
			name:       "ipv6 remote addr with port",
			remoteAddr: "[2001:db8::1]:443",
			want:       "2001:db8::1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
			r.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				r.Header.Set("X-Forwarded-For", tt.xff)
			}
			if got := clientIP(r); got != tt.want {
				t.Fatalf("clientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}
