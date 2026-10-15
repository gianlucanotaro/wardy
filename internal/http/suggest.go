package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gianlucanotaro/wardy/internal/ai"
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
	Source           string           `json:"source,omitempty"`
	NoMatchReason    string           `json:"no_match_reason,omitempty"`
	WeatherAvailable bool             `json:"weather_available"`
}

// suggestHandler runs the outfit pre-filter for ?occasion=, layering
// AI-written rationale on top via suggest.SuggestAI when aiEnabled, under
// budget, and under the per-user daily cap — otherwise it degrades to the
// same rules-only result the pre-AI pre-filter produces (docs/ai.md: "the
// app must stay fully usable with AI_PROVIDER=off"). Weather is fetched
// best-effort — a missing location or a failed fetch both just mean
// Suggest runs without warmth-matching (docs/ai.md: "Weather down ->
// suggest anyway"), never an error to the client. ?lat=&lon= is the
// device's current position, used over the home location when sent
// (weatherLocation). ?mood= (one of suggest.Moods) and
// ?time_of_day= are optional prompt context for the AI rationale, not
// filters on the candidate set.
func suggestHandler(s *store.Store, wc *weather.Client, provider ai.Provider, aiEnabled bool, monthlyBudgetUSD float64) http.HandlerFunc {
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
		mood := r.URL.Query().Get("mood")
		if !suggest.ValidMood(mood) {
			http.Error(w, "invalid mood", http.StatusBadRequest)
			return
		}
		timeOfDay := r.URL.Query().Get("time_of_day")

		var forecast *weather.Forecast
		lat, lon, hasLocation := weatherLocation(r, user)
		if hasLocation {
			if f, err := wc.Forecast(r.Context(), lat, lon); err != nil {
				slog.Warn("suggest: fetch weather", "error", err)
			} else {
				forecast = &f
			}
		}

		northernHemisphere := !hasLocation || lat >= 0

		result, err := suggest.SuggestAI(r.Context(), s, provider, aiEnabled, monthlyBudgetUSD, suggest.DailySuggestionCap, user.ID, occasion, mood, timeOfDay, forecast, northernHemisphere, time.Now())
		if err != nil {
			slog.Error("suggest", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		resp := suggestResponse{
			Outfits:          make([]outfitResponse, 0, len(result.Outfits)),
			Source:           result.Source,
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
