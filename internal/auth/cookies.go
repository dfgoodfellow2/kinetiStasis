package auth

import (
	"net/http"
	"time"
)

// cookieFlags returns the SameSite mode and Secure flag matching the
// environment. In development (isProd==false) cookies use SameSite=Lax over
// plain HTTP so http://localhost works; in production the SPA and API may be
// cross-site over HTTPS, so SameSite=None and Secure=true are required.
func cookieFlags(isProd bool) (http.SameSite, bool) {
	if isProd {
		return http.SameSiteNoneMode, true
	}
	return http.SameSiteLaxMode, false
}

// SetAccessCookie writes the JWT access token as a httpOnly cookie.
func SetAccessCookie(w http.ResponseWriter, token string, isProd bool) {
	sameSite, secure := cookieFlags(isProd)
	http.SetCookie(w, &http.Cookie{
		Name:     AccessCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(AccessTokenDuration.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	})
}

// SetRefreshCookie writes the opaque refresh token as a httpOnly cookie.
func SetRefreshCookie(w http.ResponseWriter, token string, isProd bool) {
	sameSite, secure := cookieFlags(isProd)
	http.SetCookie(w, &http.Cookie{
		Name:  RefreshCookieName,
		Value: token,
		// Keep refresh cookie scoped to the auth path.
		Path:     "/v1/auth",
		MaxAge:   int(RefreshTokenDuration.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	})
}

// ClearAuthCookies removes both auth cookies from the browser. The flags must
// mirror those used when setting the cookies, otherwise browsers may refuse to
// overwrite/expire them (e.g. SameSite=None; Secure cookies in production).
func ClearAuthCookies(w http.ResponseWriter, isProd bool) {
	sameSite, secure := cookieFlags(isProd)
	http.SetCookie(w, &http.Cookie{
		Name:     AccessCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshCookieName,
		Value:    "",
		Path:     "/v1/auth",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	})
}
