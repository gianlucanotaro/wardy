package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gianlucanotaro/wardy/internal/ai"
	"github.com/gianlucanotaro/wardy/internal/inspo"
	"github.com/gianlucanotaro/wardy/internal/store"
	"github.com/gianlucanotaro/wardy/internal/suggest"
	"github.com/gianlucanotaro/wardy/internal/taxonomy"
	"github.com/gianlucanotaro/wardy/internal/weather"
)

type outfitResponse struct {
	Items []store.Item `json:"items"`
	Note  string       `json:"note"`
	// InspoMatches counts, per item id, the inspo fits showing a piece
	// like it (inspo.Matches) — only items with at least one, and only
	// when the style profile was used.
	InspoMatches map[int64]int `json:"inspo_matches,omitempty"`
}

type suggestResponse struct {
	Outfits          []outfitResponse `json:"outfits"`
	Source           string           `json:"source,omitempty"`
	NoMatchReason    string           `json:"no_match_reason,omitempty"`
	WeatherAvailable bool             `json:"weather_available"`
	// StyleUsed: the user's inspo style profile was sent to the AI.
	StyleUsed bool `json:"style_used,omitempty"`
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
// (weatherLocation). ?mood= (one of suggest.Moods), ?time_of_day= and
// ?prompt= (the user's own words, suggest.CleanPrompt) are optional prompt
// context for the AI, not filters on the candidate set. The prompt is
// never logged. So is the user's inspo style profile (#49), when they have
// it switched on and enough tagged fits (styleContext).
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
		prompt, err := suggest.CleanPrompt(r.URL.Query().Get("prompt"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		sc := ai.SuggestContext{Mood: mood, TimeOfDay: r.URL.Query().Get("time_of_day"), Prompt: prompt}
		var fits []store.InspoFit
		if aiEnabled && user.UseInspo {
			sc.Style, fits = styleContext(r.Context(), s, user.ID)
		}

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

		result, err := suggest.SuggestAI(r.Context(), s, provider, aiEnabled, monthlyBudgetUSD, suggest.DailySuggestionCap, user.ID, occasion, sc, forecast, northernHemisphere, time.Now())
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
			StyleUsed:        sc.Style != "" && result.Source == suggest.SourceAI,
		}
		for _, outfit := range result.Outfits {
			o := outfitResponse{Items: outfit.Items, Note: outfit.Note}
			if resp.StyleUsed {
				for _, it := range outfit.Items {
					if n := inspo.Matches(it, fits); n > 0 {
						if o.InspoMatches == nil {
							o.InspoMatches = map[int64]int{}
						}
						o.InspoMatches[it.ID] = n
					}
				}
			}
			resp.Outfits = append(resp.Outfits, o)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// styleContext is the user's style profile as suggestion context — the
// counted profile plus the cached AI summary, when it was written for
// this profile — and the fits it came from,
// or "" when the board has too few tagged fits to say anything yet.
func styleContext(ctx context.Context, s *store.Store, userID int64) (string, []store.InspoFit) {
	fits, err := s.InspoFitsByUser(ctx, userID)
	if err != nil {
		slog.Error("suggest: inspo fits", "error", err)
		return "", nil
	}
	profile := inspo.Build(fits)
	if !profile.UsableForSuggestions() {
		return "", nil
	}
	style := profile.Describe()
	summary, key, err := s.StyleSummary(ctx, userID)
	if err != nil {
		slog.Error("suggest: style summary", "error", err)
	} else if summary != "" && key == profile.Key() {
		style = summary + " " + style
	}
	return style, fits
}
