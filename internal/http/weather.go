package http

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gianlucanotaro/wardy/internal/weather"
)

// Only Reason is omitempty — it's genuinely absent when available is true.
// The forecast fields are never omitempty: 0 is a meaningful value for all
// of them (0% chance of rain, 0°C, dead calm wind), not "absent", and
// omitempty would silently drop them whenever the real value happened to
// be exactly zero.
type weatherResponse struct {
	Available         bool    `json:"available"`
	Reason            string  `json:"reason,omitempty"`
	TempHighC         float64 `json:"temp_high_c"`
	TempLowC          float64 `json:"temp_low_c"`
	PrecipProbability int     `json:"precip_probability"`
	WindSpeedKPH      float64 `json:"wind_speed_kph"`
	Condition         string  `json:"condition"`
}

// weatherHandler returns the forecast for the user's home location.
// Never a 500 that blocks the caller — a missing location or an upstream
// failure both come back as a plain {available:false, reason} the
// frontend can show a graceful message for (docs/ai.md: degrade, don't
// crash).
func weatherHandler(wc *weather.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if !user.HomeLat.Valid || !user.HomeLon.Valid {
			json.NewEncoder(w).Encode(weatherResponse{Available: false, Reason: "no_location_set"})
			return
		}

		forecast, err := wc.Forecast(r.Context(), user.HomeLat.Float64, user.HomeLon.Float64)
		if err != nil {
			slog.Warn("fetch weather", "error", err)
			json.NewEncoder(w).Encode(weatherResponse{Available: false, Reason: "fetch_failed"})
			return
		}

		json.NewEncoder(w).Encode(weatherResponse{
			Available:         true,
			TempHighC:         forecast.TempHighC,
			TempLowC:          forecast.TempLowC,
			PrecipProbability: forecast.PrecipProbability,
			WindSpeedKPH:      forecast.WindSpeedKPH,
			Condition:         forecast.Condition,
		})
	}
}
