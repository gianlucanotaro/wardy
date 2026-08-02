// Package weather fetches forecasts from Open-Meteo — free, no API key, no
// signup (see docs/ai.md). Forecasts are cached in-memory by rounded
// lat/lon with a 1 hour TTL; callers are responsible for degrading
// gracefully if Forecast returns an error — the wardrobe must stay
// browsable and suggestible without it.
package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const (
	defaultBaseURL = "https://api.open-meteo.com/v1/forecast"
	cacheTTL       = time.Hour
)

// Forecast is today's weather at one location — just what outfit
// suggestions need (docs/ai.md): temp high/low, precipitation
// probability, wind, and a coarse human-readable condition.
type Forecast struct {
	TempHighC         float64
	TempLowC          float64
	PrecipProbability int // 0-100
	WindSpeedKPH      float64
	Condition         string
	FetchedAt         time.Time
}

type cacheEntry struct {
	forecast Forecast
	expires  time.Time
}

// Client fetches and caches forecasts. Zero value is not usable; use
// NewClient.
type Client struct {
	httpClient *http.Client
	baseURL    string

	mu    sync.Mutex
	cache map[string]cacheEntry
}

func NewClient() *Client {
	return NewClientWithBaseURL(defaultBaseURL)
}

// NewClientWithBaseURL builds a Client against a custom Open-Meteo-compatible
// endpoint — exported for tests that stand in a fake server rather than
// hitting the real API.
func NewClientWithBaseURL(baseURL string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    baseURL,
		cache:      make(map[string]cacheEntry),
	}
}

// Forecast returns today's forecast for lat/lon, serving from the
// in-memory cache when a fresh-enough entry exists.
func (c *Client) Forecast(ctx context.Context, lat, lon float64) (Forecast, error) {
	key := cacheKey(lat, lon)

	c.mu.Lock()
	entry, ok := c.cache[key]
	c.mu.Unlock()
	if ok && time.Now().Before(entry.expires) {
		return entry.forecast, nil
	}

	forecast, err := c.fetch(ctx, lat, lon)
	if err != nil {
		return Forecast{}, err
	}

	c.mu.Lock()
	c.cache[key] = cacheEntry{forecast: forecast, expires: time.Now().Add(cacheTTL)}
	c.mu.Unlock()

	return forecast, nil
}

// cacheKey rounds to one decimal degree (~11km) so nearby requests share a
// cache entry instead of each pinning their own.
func cacheKey(lat, lon float64) string {
	return fmt.Sprintf("%.1f,%.1f", lat, lon)
}

func (c *Client) fetch(ctx context.Context, lat, lon float64) (Forecast, error) {
	q := url.Values{
		"latitude":  {fmt.Sprintf("%f", lat)},
		"longitude": {fmt.Sprintf("%f", lon)},
		"current":   {"wind_speed_10m,weather_code"},
		"daily":     {"temperature_2m_max,temperature_2m_min,precipitation_probability_max"},
		"timezone":  {"auto"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"?"+q.Encode(), nil)
	if err != nil {
		return Forecast{}, fmt.Errorf("build weather request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Forecast{}, fmt.Errorf("fetch weather: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Forecast{}, fmt.Errorf("open-meteo returned status %d", resp.StatusCode)
	}

	var raw struct {
		Current struct {
			WindSpeed10m float64 `json:"wind_speed_10m"`
			WeatherCode  int     `json:"weather_code"`
		} `json:"current"`
		Daily struct {
			Temperature2mMax            []float64 `json:"temperature_2m_max"`
			Temperature2mMin            []float64 `json:"temperature_2m_min"`
			PrecipitationProbabilityMax []int     `json:"precipitation_probability_max"`
		} `json:"daily"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return Forecast{}, fmt.Errorf("decode weather response: %w", err)
	}

	f := Forecast{
		WindSpeedKPH: raw.Current.WindSpeed10m,
		Condition:    conditionFromWMOCode(raw.Current.WeatherCode),
		FetchedAt:    time.Now(),
	}
	if len(raw.Daily.Temperature2mMax) > 0 {
		f.TempHighC = raw.Daily.Temperature2mMax[0]
	}
	if len(raw.Daily.Temperature2mMin) > 0 {
		f.TempLowC = raw.Daily.Temperature2mMin[0]
	}
	if len(raw.Daily.PrecipitationProbabilityMax) > 0 {
		f.PrecipProbability = raw.Daily.PrecipitationProbabilityMax[0]
	}

	return f, nil
}
