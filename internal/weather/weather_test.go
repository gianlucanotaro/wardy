package weather

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

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
