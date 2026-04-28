package web

import "net/http"

// Handler returns the SPA http.Handler. In dev builds without the dist
// directory, this returns nil and the server simply won't serve the PWA.
// In production, use `-tags pwa` build which provides the embedded handler.
func Handler() http.Handler {
	return devOrProdHandler()
}
