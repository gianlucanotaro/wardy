// Package http wires the stdlib router, middleware, and the embedded
// frontend into a single handler. Handlers never write SQL; store never
// imports this package.
package http

import (
	"database/sql"
	"io/fs"
	"net/http"
	"strings"
)

// New builds the application's http.Handler: API routes plus the embedded
// frontend, served as a single-page app.
func New(db *sql.DB, frontend fs.FS) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", healthzHandler(db))
	mux.Handle("/", spaHandler(frontend))

	return withMiddleware(mux)
}

// spaHandler serves embedded static assets directly and falls back to
// index.html for any path that isn't a real file, so client-side routes
// (e.g. /items/42) still load the app shell.
func spaHandler(frontend fs.FS) http.Handler {
	fileServer := http.FileServerFS(frontend)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(frontend, path); err != nil {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}
