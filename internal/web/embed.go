//go:build pwa

package web

import (
	"embed"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

//go:embed dist
var distFS embed.FS

func devOrProdHandler() http.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}

	r := chi.NewRouter()

	// file server against the embedded filesystem
	fsHandler := http.FileServer(http.FS(sub))

	// Single handler for both GET and HEAD
	handler := func(w http.ResponseWriter, r *http.Request) {
		upath := r.URL.Path
		if upath == "/" {
			upath = "/index.html"
		}

		filePath := strings.TrimPrefix(path.Clean(upath), "/")

		f, err := sub.Open(filePath)
		if err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			fsHandler.ServeHTTP(w, r2)
			return
		}
		_ = f.Close()

		ext := strings.ToLower(path.Ext(filePath))
		switch ext {
		case ".html", ".htm":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		case ".js", ".mjs":
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		case ".css":
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
		case ".json":
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
		case ".png":
			w.Header().Set("Content-Type", "image/png")
		case ".svg":
			w.Header().Set("Content-Type", "image/svg+xml")
		case ".ico":
			w.Header().Set("Content-Type", "image/x-icon")
		case ".webmanifest":
			w.Header().Set("Content-Type", "application/manifest+json")
		case ".woff":
			w.Header().Set("Content-Type", "font/woff")
		case ".woff2":
			w.Header().Set("Content-Type", "font/woff2")
		case ".txt":
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		}

		r2 := r.Clone(r.Context())
		r2.URL.Path = "/" + filePath
		fsHandler.ServeHTTP(w, r2)
	}

	// Serve index.html directly for root - read from embedded FS
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		f, err := sub.Open("index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeContent(w, r, "index.html", time.Now(), f.(io.ReadSeeker))
	})
	r.Head("/", func(w http.ResponseWriter, r *http.Request) {
		f, err := sub.Open("index.html")
		if err != nil {
			return
		}
		defer f.Close()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeContent(w, r, "index.html", time.Now(), f.(io.ReadSeeker))
	})

	r.Get("/*", handler)
	r.Head("/*", handler)

	return r
}
