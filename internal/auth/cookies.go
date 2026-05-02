package auth

import (
	"net/http"
	"time"
)

// SetAccessCookie writes the JWT access token as a httpOnly cookie.
func SetAccessCookie(w http.ResponseWriter, token string, isProd bool) {
	s := http.SameSiteLaxMode
	if isProd {
		// In production we require cross-site cookies for the SPA -> API flow
		// (e.g. frontend on a different origin). Browsers require Secure when
		// SameSite=None, so set None only in prod where Secure=true.
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
	// Use Lax when clearing — safe default for development. Clearing is best-effort.
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
