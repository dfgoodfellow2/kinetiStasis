package auth

import (
	"net/http"
	"time"
)

// SetAccessCookie writes the JWT access token as a httpOnly cookie.
func SetAccessCookie(w http.ResponseWriter, token string, isProd bool) {
	// Use SameSite=Lax by default (works on http://localhost in dev).
	// In production (isProd==true) the SPA and API may be cross-site over HTTPS,
	// so use SameSite=None and Secure=true to allow cross-site cookies.
	s := http.SameSiteLaxMode
	if isProd {
		s = http.SameSiteNoneMode
	}
	http.SetCookie(w, &http.Cookie{
		Name:     AccessCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(AccessTokenDuration.Seconds()),
		HttpOnly: true,
		Secure:   isProd,
		SameSite: s,
	})
}

// SetRefreshCookie writes the opaque refresh token as a httpOnly cookie.
func SetRefreshCookie(w http.ResponseWriter, token string, isProd bool) {
	// Use SameSite=Lax by default (dev). In prod use None + Secure.
	s := http.SameSiteLaxMode
	if isProd {
		s = http.SameSiteNoneMode
	}
	http.SetCookie(w, &http.Cookie{
		Name:  RefreshCookieName,
		Value: token,
		// Keep refresh cookie scoped to the auth path.
		Path:     "/v1/auth",
		MaxAge:   int(RefreshTokenDuration.Seconds()),
		HttpOnly: true,
		Secure:   isProd,
		SameSite: s,
	})
}

// ClearAuthCookies removes both auth cookies from the browser.
func ClearAuthCookies(w http.ResponseWriter) {
	// Match the SameSite setting used when setting cookies so the browser
	// properly removes them. Use Lax for clearing which is compatible in dev
	// and prod.
	s := http.SameSiteLaxMode
	http.SetCookie(w, &http.Cookie{
		Name:     AccessCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: s,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshCookieName,
		Value:    "",
		Path:     "/v1/auth",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: s,
	})
}
