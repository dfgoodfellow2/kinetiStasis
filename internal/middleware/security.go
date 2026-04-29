package middleware

import (
	"net/http"
	"os"
)

// SecureHeaders adds security-related HTTP response headers.
func SecureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// CSP: stricter in prod, relaxed for Vite HMR in dev
		if os.Getenv("ENV") == "production" {
			w.Header().Set("Content-Security-Policy",
				"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:")
		} else {
			// Dev: allow inline scripts, eval, and WebSocket for HMR
			w.Header().Set("Content-Security-Policy",
				"default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self' ws://localhost:*")
		}
		next.ServeHTTP(w, r)
	})
}
