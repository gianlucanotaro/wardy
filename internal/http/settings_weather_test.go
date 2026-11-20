package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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

// #80: the plan reads the daily forecast from the same response. The
// fixture is a recorded Open-Meteo reply.
func TestWeatherDailyForecast(t *testing.T) {
	s := newTestStore(t)
	fixture, err := os.ReadFile("../weather/testdata/open-meteo-zurich.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	fakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture)
	}))
	t.Cleanup(fakeServer.Close)

	h := newTestHandlerFull(t, s, true, weather.NewClientWithBaseURL(fakeServer.URL))
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")
	patchSettings(h, cookie, 47.3769, 8.5417)

	var resp struct {
		Available bool
		Days      []map[string]any
	}
	json.Unmarshal(getWeather(h, cookie).Body.Bytes(), &resp)
	if !resp.Available || len(resp.Days) != 7 {
		t.Fatalf("want available with 7 days, got %+v", resp)
	}
	want := map[string]any{"date": "2026-11-14", "temp_high_c": 5.7, "temp_low_c": 0.9, "precip_probability": 0.0, "condition": "fog"}
	for k, v := range want {
		if resp.Days[2][k] != v {
			t.Errorf("days[2].%s = %v, want %v", k, resp.Days[2][k], v)
		}
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

// weatherAt serves weatherFixture and records the latitude/longitude each
// upstream request asked for, so a test can check whose location was used.
func weatherAt(t *testing.T) (*weather.Client, *[]string) {
	t.Helper()
	var asked []string
	fakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Query().Get("latitude")+","+r.URL.Query().Get("longitude"))
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(weatherFixture))
	}))
	t.Cleanup(fakeServer.Close)
	return weather.NewClientWithBaseURL(fakeServer.URL), &asked
}

func getWeatherAt(h http.Handler, cookie *http.Cookie, query string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/weather?"+query, nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// The device's current position (?lat=&lon=) wins over the home location,
// and works with no home location set at all; anything unusable falls back
// to home.
func TestWeatherCurrentLocation(t *testing.T) {
	cases := []struct {
		name      string
		home      bool
		query     string
		wantAsked string // "" = no upstream call
	}{
		{"current, no home set", false, "lat=40.4168&lon=-3.7038", "40.416800,-3.703800"},
		{"current beats home", true, "lat=40.4168&lon=-3.7038", "40.416800,-3.703800"},
		{"no query uses home", true, "", "47.376900,8.541700"},
		{"out of range falls back to home", true, "lat=91&lon=0", "47.376900,8.541700"},
		{"NaN falls back to home", true, "lat=NaN&lon=NaN", "47.376900,8.541700"},
		{"only lat falls back to home", true, "lat=40.4", "47.376900,8.541700"},
		{"unusable and no home", false, "lat=abc&lon=1", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newTestStore(t)
			wc, asked := weatherAt(t)
			h := newTestHandlerFull(t, s, true, wc)
			cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")
			if c.home {
				patchSettings(h, cookie, 47.3769, 8.5417)
			}

			rec := getWeatherAt(h, cookie, c.query)
			var resp map[string]any
			json.Unmarshal(rec.Body.Bytes(), &resp)

			if c.wantAsked == "" {
				if resp["available"] != false || resp["reason"] != "no_location_set" || len(*asked) != 0 {
					t.Errorf("want no_location_set and no upstream call, got %v, asked %v", resp, *asked)
				}
				return
			}
			if resp["available"] != true {
				t.Fatalf("want available:true, got %v", resp)
			}
			if len(*asked) != 1 || (*asked)[0] != c.wantAsked {
				t.Errorf("upstream asked for %v, want [%s]", *asked, c.wantAsked)
			}
		})
	}
}
