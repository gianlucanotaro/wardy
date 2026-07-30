package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gianlucanotaro/wardy/internal/store"
)

// healthzHandler reports the app healthy only if the database actually
// responds — a wedged DB should show up in the health check, not just a
// green "process is running" signal.
func healthzHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		w.Header().Set("Content-Type", "application/json")

		if err := s.Ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "down"})
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}
