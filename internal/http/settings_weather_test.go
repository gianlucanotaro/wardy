package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gianlucanotaro/wardy/internal/weather"
)

const weatherFixture = `{
	"current": {"wind_speed_10m": 10.0, "weather_code": 0},
	"daily": {
		"temperature_2m_max": [25.0],
		"temperature_2m_min": [15.0],
		"precipitation_probability_max": [5]
	}
}`

// weatherFixtureAllZero exercises a forecast where every numeric field is
// legitimately 0 (dead calm, no rain, freezing) — none of them mean
// "absent", so they must all survive JSON serialization.
const weatherFixtureAllZero = `{
	"current": {"wind_speed_10m": 0, "weather_code": 0},
	"daily": {
		"temperature_2m_max": [0],
		"temperature_2m_min": [0],
		"precipitation_probability_max": [0]
	}
}`

func patchSettings(h http.Handler, cookie *http.Cookie, lat, lon float64) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]float64{"home_lat": lat, "home_lon": lon})
	req := httptest.NewRequest(http.MethodPatch, "/api/settings", bytes.NewReader(body))
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func getSettings(h http.Handler, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func getWeather(h http.Handler, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/weather", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestGetSettingsInitiallyEmpty(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")

	rec := getSettings(h, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("get settings: status %d", rec.Code)
	}

	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["home_lat"] != nil || resp["home_lon"] != nil {
		t.Errorf("new user should have no home location set, got %v", resp)
	}
}

func TestUpdateSettingsValidLocation(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")

	rec := patchSettings(h, cookie, 47.3769, 8.5417)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch settings: status %d, body %s", rec.Code, rec.Body.String())
	}

	getRec := getSettings(h, cookie)
	var resp map[string]any
	json.Unmarshal(getRec.Body.Bytes(), &resp)
	if resp["home_lat"] != 47.3769 || resp["home_lon"] != 8.5417 {
		t.Errorf("settings not persisted, got %v", resp)
	}
}

func TestUpdateSettingsRejectsOutOfRangeLatitude(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")

	rec := patchSettings(h, cookie, 200, 8.5417)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("patch settings with lat=200: status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUpdateSettingsRejectsOutOfRangeLongitude(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")

	rec := patchSettings(h, cookie, 47.3769, 999)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("patch settings with lon=999: status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestWeatherNoLocationSet(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")

	rec := getWeather(h, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("get weather: status %d", rec.Code)
	}
	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["available"] != false || resp["reason"] != "no_location_set" {
		t.Errorf("expected available:false reason:no_location_set, got %v", resp)
	}
}

func TestWeatherSuccess(t *testing.T) {
	s := newTestStore(t)

	fakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(weatherFixture))
	}))
	t.Cleanup(fakeServer.Close)

	h := newTestHandlerFull(t, s, true, weather.NewClientWithBaseURL(fakeServer.URL))
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")
	patchSettings(h, cookie, 47.3769, 8.5417)

	rec := getWeather(h, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("get weather: status %d, body %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["available"] != true {
		t.Fatalf("expected available:true, got %v", resp)
	}
	if resp["temp_high_c"] != 25.0 || resp["condition"] != "clear" {
		t.Errorf("unexpected weather response: %v", resp)
	}
}

// TestWeatherZeroValuesSurviveSerialization guards against the omitempty
// bug: a struct tag of `json:"...,omitempty"` on a numeric field silently
// drops it from the response whenever the real value is exactly 0, which
// is indistinguishable from "field absent" to the frontend. All of these
// forecast fields are meaningful at 0 and must always be present.
func TestWeatherZeroValuesSurviveSerialization(t *testing.T) {
	s := newTestStore(t)

	fakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(weatherFixtureAllZero))
	}))
	t.Cleanup(fakeServer.Close)

	h := newTestHandlerFull(t, s, true, weather.NewClientWithBaseURL(fakeServer.URL))
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")
	patchSettings(h, cookie, 47.3769, 8.5417)

	rec := getWeather(h, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("get weather: status %d, body %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	for _, field := range []string{"temp_high_c", "temp_low_c", "precip_probability", "wind_speed_kph"} {
		if _, present := resp[field]; !present {
			t.Errorf("%s missing from response entirely (omitempty dropped a legitimate zero): %v", field, resp)
		}
	}
}

func TestWeatherUpstreamFailureDegradesGracefully(t *testing.T) {
	s := newTestStore(t)

	fakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(fakeServer.Close)

	h := newTestHandlerFull(t, s, true, weather.NewClientWithBaseURL(fakeServer.URL))
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")
	patchSettings(h, cookie, 47.3769, 8.5417)

	rec := getWeather(h, cookie)
	// Never a 500 that blocks the caller — a graceful available:false instead.
	if rec.Code != http.StatusOK {
		t.Fatalf("get weather on upstream failure: status %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["available"] != false || resp["reason"] != "fetch_failed" {
		t.Errorf("expected available:false reason:fetch_failed, got %v", resp)
	}
}
