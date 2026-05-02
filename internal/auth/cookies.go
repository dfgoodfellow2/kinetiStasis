package auth

import (
	"net/http"
	"time"
)

// SetAccessCookie writes the JWT access token as a httpOnly cookie.
func SetAccessCookie(w http.ResponseWriter, token string, isProd bool) {
	// Always use SameSite=None so cookies are sent when SPA and API are on
	// different ports (treated as cross-site). Only set Secure in prod.
	s := http.SameSiteNoneMode
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
	// Always use SameSite=None so refresh cookie is sent for cross-site calls.
	s := http.SameSiteNoneMode
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
	// properly removes them. Clearing is best-effort.
	s := http.SameSiteNoneMode
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
