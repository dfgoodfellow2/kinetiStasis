package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCookieFlags(t *testing.T) {
	tests := []struct {
		name       string
		isProd     bool
		wantSite   http.SameSite
		wantSecure bool
	}{
		{name: "production", isProd: true, wantSite: http.SameSiteLaxMode, wantSecure: true},
		{name: "development", isProd: false, wantSite: http.SameSiteLaxMode, wantSecure: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			site, secure := cookieFlags(tt.isProd)
			if site != tt.wantSite {
				t.Fatalf("SameSite = %v, want %v", site, tt.wantSite)
			}
			if secure != tt.wantSecure {
				t.Fatalf("Secure = %v, want %v", secure, tt.wantSecure)
			}
		})
	}
}

// TestAuthCookieFlags pins the wire format for both set and clear paths: every
// auth cookie must be httpOnly, SameSite=Lax, and Secure only in production.
func TestAuthCookieFlags(t *testing.T) {
	tests := []struct {
		name   string
		isProd bool
		write  func(w http.ResponseWriter, isProd bool)
		names  []string
	}{
		{name: "set prod", isProd: true, write: func(w http.ResponseWriter, p bool) {
			SetAccessCookie(w, "access", p)
			SetRefreshCookie(w, "refresh", p)
		}, names: []string{AccessCookieName, RefreshCookieName}},
		{name: "set dev", isProd: false, write: func(w http.ResponseWriter, p bool) {
			SetAccessCookie(w, "access", p)
			SetRefreshCookie(w, "refresh", p)
		}, names: []string{AccessCookieName, RefreshCookieName}},
		{name: "clear prod", isProd: true, write: ClearAuthCookies, names: []string{AccessCookieName, RefreshCookieName}},
		{name: "clear dev", isProd: false, write: ClearAuthCookies, names: []string{AccessCookieName, RefreshCookieName}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tt.write(rec, tt.isProd)
			cookies := rec.Result().Cookies()
			if len(cookies) != len(tt.names) {
				t.Fatalf("cookie count = %d, want %d", len(cookies), len(tt.names))
			}
			for i, c := range cookies {
				if c.Name != tt.names[i] {
					t.Fatalf("cookie[%d] name = %q, want %q", i, c.Name, tt.names[i])
				}
				if c.SameSite != http.SameSiteLaxMode {
					t.Fatalf("%s SameSite = %v, want Lax", c.Name, c.SameSite)
				}
				if !c.HttpOnly {
					t.Fatalf("%s is not HttpOnly", c.Name)
				}
				if c.Secure != tt.isProd {
					t.Fatalf("%s Secure = %v, want %v", c.Name, c.Secure, tt.isProd)
				}
			}
		})
	}
}
