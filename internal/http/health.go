package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"
)

// healthzHandler reports the app healthy only if the database actually
// responds — a wedged DB should show up in the health check, not just a
// green "process is running" signal.
func healthzHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		w.Header().Set("Content-Type", "application/json")

		if err := db.PingContext(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "down"})
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}
