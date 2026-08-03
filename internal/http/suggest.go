package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gianlucanotaro/wardy/internal/store"
	"github.com/gianlucanotaro/wardy/internal/suggest"
	"github.com/gianlucanotaro/wardy/internal/taxonomy"
	"github.com/gianlucanotaro/wardy/internal/weather"
)

type outfitResponse struct {
	Items []store.Item `json:"items"`
	Note  string       `json:"note"`
}

type suggestResponse struct {
	Outfits          []outfitResponse `json:"outfits"`
	NoMatchReason    string           `json:"no_match_reason,omitempty"`
	WeatherAvailable bool             `json:"weather_available"`
}

// suggestHandler runs the rules-based (zero-AI) outfit pre-filter for
// ?occasion=. Weather is fetched best-effort — a missing location or a
// failed fetch both just mean Suggest runs without warmth-matching
// (docs/ai.md: "Weather down -> suggest anyway"), never an error to the
// client.
func suggestHandler(s *store.Store, wc *weather.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		occasion := taxonomy.Occasion(r.URL.Query().Get("occasion"))
		if !occasion.Valid() {
			http.Error(w, "invalid or missing occasion", http.StatusBadRequest)
			return
		}

		var forecast *weather.Forecast
		if user.HomeLat.Valid && user.HomeLon.Valid {
			if f, err := wc.Forecast(r.Context(), user.HomeLat.Float64, user.HomeLon.Float64); err != nil {
				slog.Warn("suggest: fetch weather", "error", err)
			} else {
				forecast = &f
			}
		}

		northernHemisphere := !user.HomeLat.Valid || user.HomeLat.Float64 >= 0

		result, err := suggest.Suggest(r.Context(), s, user.ID, occasion, forecast, northernHemisphere, time.Now())
		if err != nil {
			slog.Error("suggest", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		resp := suggestResponse{
			Outfits:          make([]outfitResponse, 0, len(result.Outfits)),
			NoMatchReason:    result.NoMatchReason,
			WeatherAvailable: forecast != nil,
		}
		for _, outfit := range result.Outfits {
			resp.Outfits = append(resp.Outfits, outfitResponse{Items: outfit.Items, Note: outfit.Note})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
