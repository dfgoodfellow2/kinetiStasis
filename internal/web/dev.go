//go:build !pwa

package web

import "net/http"

func devOrProdHandler() http.Handler {
	return nil
}
