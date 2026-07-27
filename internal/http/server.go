package http

import (
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/gianlucanotaro/wardy/internal/store"
)

type Config struct {
	Addr   string
	Secure bool
}

type Server struct {
	cfg    Config
	db     *store.DB
	mux    *http.ServeMux
	logger *slog.Logger
}

func NewServer(cfg Config, db *store.DB, staticFS fs.FS, logger *slog.Logger) *Server {
	s := &Server{
		cfg:    cfg,
		db:     db,
		mux:    http.NewServeMux(),
		logger: logger,
	}
	s.routes(staticFS)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes(staticFS fs.FS) {
	// Public
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("POST /api/auth/login", s.handleLogin)

	// Authenticated
	s.mux.Handle("POST /api/auth/logout", s.requireAuth(http.HandlerFunc(s.handleLogout)))
	s.mux.Handle("GET /api/auth/me", s.requireAuth(http.HandlerFunc(s.handleMe)))

	// Serve embedded frontend; fall through to index.html for SPA routing
	s.mux.Handle("/", http.FileServer(http.FS(staticFS)))
}

func (s *Server) internalError(w http.ResponseWriter, err error) {
	s.logger.Error("internal error", "err", err)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}
