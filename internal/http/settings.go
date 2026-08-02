package http

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gianlucanotaro/wardy/internal/store"
)

type settingsResponse struct {
	HomeLat store.NullFloat64 `json:"home_lat"`
	HomeLon store.NullFloat64 `json:"home_lon"`
}

func getSettingsHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settingsResponse{HomeLat: user.HomeLat, HomeLon: user.HomeLon})
}

// updateSettingsHandler sets the user's home location — used to fetch
// weather without ever demanding browser geolocation (docs/ai.md).
func updateSettingsHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req struct {
			HomeLat float64 `json:"home_lat"`
			HomeLon float64 `json:"home_lon"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		if req.HomeLat < -90 || req.HomeLat > 90 {
			http.Error(w, "home_lat must be between -90 and 90", http.StatusBadRequest)
			return
		}
		if req.HomeLon < -180 || req.HomeLon > 180 {
			http.Error(w, "home_lon must be between -180 and 180", http.StatusBadRequest)
			return
		}

		if err := s.UpdateUserLocation(r.Context(), user.ID, req.HomeLat, req.HomeLon); err != nil {
			slog.Error("update settings", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(settingsResponse{
			HomeLat: store.NullFloat64{Float64: req.HomeLat, Valid: true},
			HomeLon: store.NullFloat64{Float64: req.HomeLon, Valid: true},
		})
	}
}
