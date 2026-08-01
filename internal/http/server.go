// Package http wires the stdlib router, middleware, and the embedded
// frontend into a single handler. Handlers never write SQL; store never
// imports this package.
package http

import (
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/gianlucanotaro/wardy/internal/store"
)

// loginRateLimit caps login attempts per IP and per email to 5 per 5
// minutes — enough to stop credential-guessing against the two known
// accounts without needing an external rate-limit service.
const (
	loginRateLimitAttempts = 5
	loginRateLimitWindow   = 5 * time.Minute
)

// New builds the application's http.Handler: API routes plus the embedded
// frontend, served as a single-page app. secureCookies should be true in
// production (HTTPS via Caddy) and false for local HTTP development.
// photosDir is where uploaded photo derivatives are written and served
// from.
func New(s *store.Store, frontend fs.FS, secureCookies bool, photosDir string) http.Handler {
	mux := http.NewServeMux()
	limiter := newRateLimiter(loginRateLimitAttempts, loginRateLimitWindow)

	mux.HandleFunc("GET /healthz", healthzHandler(s))
	mux.HandleFunc("POST /api/login", loginHandler(s, secureCookies, limiter))
	mux.HandleFunc("POST /api/logout", logoutHandler(s, secureCookies))
	mux.Handle("GET /api/me", requireAuth(s, http.HandlerFunc(meHandler)))

	mux.Handle("POST /api/items", requireAuth(s, uploadItemHandler(s, photosDir)))
	mux.Handle("GET /api/items", requireAuth(s, listItemsHandler(s)))
	mux.Handle("GET /api/items/{id}", requireAuth(s, getItemHandler(s)))
	mux.Handle("PATCH /api/items/{id}", requireAuth(s, updateItemHandler(s)))
	mux.Handle("PATCH /api/items/{id}/tags", requireAuth(s, updateItemTagsHandler(s)))
	mux.Handle("DELETE /api/items/{id}", requireAuth(s, deleteItemHandler(s, photosDir)))
	mux.Handle("GET /api/items/{id}/photo", requireAuth(s, servePhotoHandler(s, photosDir, false)))
	mux.Handle("GET /api/items/{id}/thumb", requireAuth(s, servePhotoHandler(s, photosDir, true)))

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
