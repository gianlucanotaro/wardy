// Package http wires the stdlib router, middleware, and the embedded
// frontend into a single handler. Handlers never write SQL; store never
// imports this package.
package http

import (
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/gianlucanotaro/wardy/internal/ai"
	"github.com/gianlucanotaro/wardy/internal/store"
	"github.com/gianlucanotaro/wardy/internal/weather"
)

// loginRateLimit caps login attempts per IP and per email to 5 per 5
// minutes — enough to stop credential-guessing against the two known
// accounts without needing an external rate-limit service.
const (
	loginRateLimitAttempts = 5
	loginRateLimitWindow   = 5 * time.Minute
)

// aiDailyTagLimit caps AI tagging jobs at ~50 per user per day (docs/ai.md)
// — prevents both accidents and a stuck client retrying in a loop.
const (
	aiDailyTagLimitAttempts = 50
	aiDailyTagLimitWindow   = 24 * time.Hour
)

// New builds the application's http.Handler: API routes plus the embedded
// frontend, served as a single-page app. secureCookies should be true in
// production (HTTPS via Caddy) and false for local HTTP development.
// photosDir is where uploaded photo derivatives are written and served
// from. aiEnabled controls whether uploads enqueue a tagging job at all
// and whether /api/suggest attempts an AI-written rationale. provider and
// monthlyBudgetUSD are the same values passed to jobs.NewWorker, reused
// here for outfit suggestions (see docs/ai.md's budget guards). sso turns on
// "Sign in with …" when configured (see oidc.go); its zero value leaves it off.
func New(s *store.Store, frontend fs.FS, secureCookies bool, photosDir string, aiEnabled bool, weatherClient *weather.Client, provider ai.Provider, monthlyBudgetUSD float64, sso OIDCConfig) http.Handler {
	mux := http.NewServeMux()
	limiter := newRateLimiter(loginRateLimitAttempts, loginRateLimitWindow)
	jobLimiter := newRateLimiter(aiDailyTagLimitAttempts, aiDailyTagLimitWindow)

	mux.HandleFunc("GET /healthz", healthzHandler(s))
	mux.HandleFunc("POST /api/login", loginHandler(s, secureCookies, limiter))
	mux.HandleFunc("POST /api/logout", logoutHandler(s, secureCookies))
	mux.HandleFunc("GET /api/auth/providers", authProvidersHandler(sso))
	oidc := newOIDCClient(sso)
	mux.HandleFunc("GET /api/auth/oidc/start", oidcStartHandler(oidc, secureCookies))
	mux.HandleFunc("GET /api/auth/oidc/callback", oidcCallbackHandler(oidc, s, secureCookies, limiter))
	mux.Handle("GET /api/me", requireAuth(s, http.HandlerFunc(meHandler)))
	mux.Handle("GET /api/settings", requireAuth(s, http.HandlerFunc(getSettingsHandler)))
	mux.Handle("PATCH /api/settings", requireAuth(s, updateSettingsHandler(s)))
	mux.Handle("GET /api/weather", requireAuth(s, weatherHandler(weatherClient)))
	mux.Handle("GET /api/suggest", requireAuth(s, suggestHandler(s, weatherClient, provider, aiEnabled, monthlyBudgetUSD)))

	mux.Handle("POST /api/items", requireAuth(s, uploadItemHandler(s, photosDir, aiEnabled, jobLimiter)))
	mux.Handle("GET /api/items", requireAuth(s, listItemsHandler(s)))
	mux.Handle("GET /api/items/{id}", requireAuth(s, getItemHandler(s)))
	mux.Handle("PATCH /api/items/{id}", requireAuth(s, updateItemHandler(s)))
	mux.Handle("PATCH /api/items/{id}/tags", requireAuth(s, updateItemTagsHandler(s)))
	mux.Handle("PATCH /api/items/{id}/archive", requireAuth(s, archiveItemHandler(s)))
	mux.Handle("DELETE /api/items/{id}", requireAuth(s, deleteItemHandler(s, photosDir)))
	mux.Handle("GET /api/items/{id}/photo", requireAuth(s, servePhotoHandler(s, photosDir, false)))
	mux.Handle("GET /api/items/{id}/thumb", requireAuth(s, servePhotoHandler(s, photosDir, true)))

	mux.Handle("POST /api/looks", requireAuth(s, createLookHandler(s)))
	mux.Handle("GET /api/looks", requireAuth(s, listLooksHandler(s)))
	mux.Handle("GET /api/looks/{id}", requireAuth(s, getLookHandler(s)))
	mux.Handle("PATCH /api/looks/{id}", requireAuth(s, updateLookHandler(s)))
	mux.Handle("DELETE /api/looks/{id}", requireAuth(s, deleteLookHandler(s)))

	mux.Handle("POST /api/wears", requireAuth(s, createWearHandler(s)))
	mux.Handle("GET /api/wears", requireAuth(s, listWearsHandler(s)))
	mux.Handle("GET /api/wears/{id}", requireAuth(s, getWearHandler(s)))
	mux.Handle("DELETE /api/wears/{id}", requireAuth(s, deleteWearHandler(s)))
	mux.Handle("GET /api/history", requireAuth(s, historyHandler(s)))

	mux.Handle("GET /api/plan", requireAuth(s, planHandler(s)))
	mux.Handle("PUT /api/plan/{date}", requireAuth(s, setPlanDayHandler(s)))

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
