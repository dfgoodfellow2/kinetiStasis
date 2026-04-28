package auth

import (
	"net/http"
	"time"
)

// SetAccessCookie writes the JWT access token as a httpOnly cookie.
func SetAccessCookie(w http.ResponseWriter, token string, isProd bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     AccessCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(AccessTokenDuration.Seconds()),
		HttpOnly: true,
		Secure:   isProd,
		SameSite: http.SameSiteStrictMode,
	})
}

// SetRefreshCookie writes the opaque refresh token as a httpOnly cookie.
func SetRefreshCookie(w http.ResponseWriter, token string, isProd bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshCookieName,
		Value:    token,
		Path:     "/v1/auth",
		MaxAge:   int(RefreshTokenDuration.Seconds()),
		HttpOnly: true,
		Secure:   isProd,
		SameSite: http.SameSiteStrictMode,
	})
}

// ClearAuthCookies removes both auth cookies from the browser.
func ClearAuthCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     AccessCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshCookieName,
		Value:    "",
		Path:     "/v1/auth",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}
