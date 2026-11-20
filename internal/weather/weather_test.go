package weather

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestForecastParsesDailyDays(t *testing.T) {
	recorded, err := os.ReadFile("testdata/open-meteo-zurich.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	client, _ := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("forecast_days") != "7" {
			t.Errorf("forecast_days = %q, want 7", r.URL.Query().Get("forecast_days"))
		}
		w.Write(recorded)
	})

	f, err := client.Forecast(context.Background(), 47.37, 8.54)
	if err != nil {
		t.Fatalf("Forecast: %v", err)
	}
	if len(f.Days) != 7 {
		t.Fatalf("len(Days) = %d, want 7", len(f.Days))
	}
	want := []Day{
		{Date: "2026-11-12", TempHighC: 12.4, TempLowC: 6.9, PrecipProbability: 100, Condition: "rain"},
		{Date: "2026-11-14", TempHighC: 5.7, TempLowC: 0.9, PrecipProbability: 0, Condition: "fog"},
		{Date: "2026-11-18", TempHighC: 13.4, TempLowC: 8.5, PrecipProbability: 81, Condition: "showers"},
	}
	for i, idx := range []int{0, 2, 6} {
		if f.Days[idx] != want[i] {
			t.Errorf("Days[%d] = %+v, want %+v", idx, f.Days[idx], want[i])
		}
	}
	if f.TempHighC != 12.4 || f.Condition != "rain" {
		t.Errorf("today's fields: high %v condition %q, want 12.4 rain", f.TempHighC, f.Condition)
	}
}

// Ragged daily arrays (one missing a value) stop the list at the shortest
// one instead of indexing past it; today's forecast still comes back.
func TestForecastDailyArraysOfDifferentLengths(t *testing.T) {
	client, _ := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"current": {"wind_speed_10m": 3, "weather_code": 0},
			"daily": {
				"time": ["2026-11-12", "2026-11-13", "2026-11-14"],
				"temperature_2m_max": [10, 11, 12],
				"temperature_2m_min": [1, 2, 3],
				"precipitation_probability_max": [5, 6],
				"weather_code": [0, 0, 0]
			}
		}`))
	})

	f, err := client.Forecast(context.Background(), 47.37, 8.54)
	if err != nil {
		t.Fatalf("Forecast: %v", err)
	}
	if len(f.Days) != 2 || f.Days[1].Date != "2026-11-13" {
		t.Errorf("Days = %+v, want the first two dates", f.Days)
	}
	if f.TempHighC != 10 {
		t.Errorf("TempHighC = %v, want 10", f.TempHighC)
	}
}

const fixtureResponse = `{
	"current": {"wind_speed_10m": 14.2, "weather_code": 61},
	"daily": {
		"temperature_2m_max": [22.5],
		"temperature_2m_min": [14.0],
		"precipitation_probability_max": [70]
	}
}`

func newTestClient(handler http.HandlerFunc) (*Client, *int) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		handler(w, r)
	}))
	client := NewClientWithBaseURL(server.URL)
	client.httpClient = server.Client()
	return client, &requests
}

func TestForecastParsesOpenMeteoResponse(t *testing.T) {
	client, _ := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(fixtureResponse))
	})

	f, err := client.Forecast(context.Background(), 47.3769, 8.5417)
	if err != nil {
		t.Fatalf("Forecast: %v", err)
	}

	if f.TempHighC != 22.5 {
		t.Errorf("TempHighC = %v, want 22.5", f.TempHighC)
	}
	if f.TempLowC != 14.0 {
		t.Errorf("TempLowC = %v, want 14.0", f.TempLowC)
	}
	if f.PrecipProbability != 70 {
		t.Errorf("PrecipProbability = %v, want 70", f.PrecipProbability)
	}
	if f.WindSpeedKPH != 14.2 {
		t.Errorf("WindSpeedKPH = %v, want 14.2", f.WindSpeedKPH)
	}
	if f.Condition != "rain" {
		t.Errorf("Condition = %q, want rain (WMO code 61)", f.Condition)
	}
}

func TestForecastCachesWithinTTL(t *testing.T) {
	client, requests := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(fixtureResponse))
	})

	ctx := context.Background()
	if _, err := client.Forecast(ctx, 47.3769, 8.5417); err != nil {
		t.Fatalf("first Forecast: %v", err)
	}
	if _, err := client.Forecast(ctx, 47.3769, 8.5417); err != nil {
		t.Fatalf("second Forecast: %v", err)
	}

	if *requests != 1 {
		t.Errorf("upstream requests = %d, want 1 (second call should hit cache)", *requests)
	}
}

func TestForecastRefetchesAfterExpiry(t *testing.T) {
	client, requests := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(fixtureResponse))
	})

	ctx := context.Background()
	if _, err := client.Forecast(ctx, 47.3769, 8.5417); err != nil {
		t.Fatalf("first Forecast: %v", err)
	}

	// Force the cached entry to look expired.
	key := cacheKey(47.3769, 8.5417)
	client.mu.Lock()
	entry := client.cache[key]
	entry.expires = time.Now().Add(-time.Minute)
	client.cache[key] = entry
	client.mu.Unlock()

	if _, err := client.Forecast(ctx, 47.3769, 8.5417); err != nil {
		t.Fatalf("second Forecast: %v", err)
	}

	if *requests != 2 {
		t.Errorf("upstream requests = %d, want 2 (expired entry should refetch)", *requests)
	}
}

func TestForecastDifferentLocationsDontShareCache(t *testing.T) {
	client, requests := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(fixtureResponse))
	})

	ctx := context.Background()
	if _, err := client.Forecast(ctx, 47.3769, 8.5417); err != nil {
		t.Fatalf("first Forecast: %v", err)
	}
	if _, err := client.Forecast(ctx, 40.7128, -74.0060); err != nil {
		t.Fatalf("second Forecast (different location): %v", err)
	}

	if *requests != 2 {
		t.Errorf("upstream requests = %d, want 2 (different locations shouldn't share a cache entry)", *requests)
	}
}

func TestForecastUpstreamError(t *testing.T) {
	client, _ := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	if _, err := client.Forecast(context.Background(), 47.3769, 8.5417); err == nil {
		t.Error("Forecast should return an error when the upstream fails, not silently succeed")
	}
}

func TestForecastMalformedResponse(t *testing.T) {
	client, _ := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	})

	if _, err := client.Forecast(context.Background(), 47.3769, 8.5417); err == nil {
		t.Error("Forecast should return an error on a malformed response")
	}
}

func TestConditionFromWMOCode(t *testing.T) {
	cases := []struct {
		code int
		want string
	}{
		{0, "clear"},
		{2, "cloudy"},
		{45, "fog"},
		{55, "drizzle"},
		{63, "rain"},
		{75, "snow"},
		{81, "showers"},
		{86, "snow"},
		{97, "thunderstorm"},
		{-1, "unknown"},
	}
	for _, c := range cases {
		if got := conditionFromWMOCode(c.code); got != c.want {
			t.Errorf("conditionFromWMOCode(%d) = %q, want %q", c.code, got, c.want)
		}
	}
}
