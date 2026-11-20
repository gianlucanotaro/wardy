package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gianlucanotaro/wardy/internal/store"
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
	// Days: the daily forecast from today on, for the plan (#80). Always
	// a list (maybe empty) when available is true.
	Days []dayForecastResponse `json:"days"`
}

type dayForecastResponse struct {
	Date              string  `json:"date"`
	TempHighC         float64 `json:"temp_high_c"`
	TempLowC          float64 `json:"temp_low_c"`
	PrecipProbability int     `json:"precip_probability"`
	Condition         string  `json:"condition"`
}

// weatherLocation picks where to fetch weather for: the device's current
// position when the client sends ?lat=&lon= (the app does whenever the
// browser shares it), otherwise the user's saved home location. Bad or
// half-sent coordinates fall back to home rather than failing the request.
func weatherLocation(r *http.Request, user *store.User) (lat, lon float64, ok bool) {
	q := r.URL.Query()
	lat, latErr := strconv.ParseFloat(q.Get("lat"), 64)
	lon, lonErr := strconv.ParseFloat(q.Get("lon"), 64)
	// Written as "inside the range" so NaN never passes.
	if latErr == nil && lonErr == nil && lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180 {
		return lat, lon, true
	}
	if user.HomeLat.Valid && user.HomeLon.Valid {
		return user.HomeLat.Float64, user.HomeLon.Float64, true
	}
	return 0, 0, false
}

// weatherHandler returns the forecast for the device's current position
// or the user's home location (weatherLocation).
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

		lat, lon, ok := weatherLocation(r, user)
		if !ok {
			json.NewEncoder(w).Encode(weatherResponse{Available: false, Reason: "no_location_set"})
			return
		}

		forecast, err := wc.Forecast(r.Context(), lat, lon)
		if err != nil {
			slog.Warn("fetch weather", "error", err)
			json.NewEncoder(w).Encode(weatherResponse{Available: false, Reason: "fetch_failed"})
			return
		}

		days := make([]dayForecastResponse, 0, len(forecast.Days))
		for _, d := range forecast.Days {
			days = append(days, dayForecastResponse{
				Date:              d.Date,
				TempHighC:         d.TempHighC,
				TempLowC:          d.TempLowC,
				PrecipProbability: d.PrecipProbability,
				Condition:         d.Condition,
			})
		}

		json.NewEncoder(w).Encode(weatherResponse{
			Available:         true,
			TempHighC:         forecast.TempHighC,
			TempLowC:          forecast.TempLowC,
			PrecipProbability: forecast.PrecipProbability,
			WindSpeedKPH:      forecast.WindSpeedKPH,
			Condition:         forecast.Condition,
			Days:              days,
		})
	}
}
