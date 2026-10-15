package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gianlucanotaro/wardy/internal/store"
	"github.com/gianlucanotaro/wardy/internal/suggest"
	"github.com/gianlucanotaro/wardy/internal/weather"
)

// currentSeason computes today's actual season so these tests stay valid
// whenever they're run, rather than hardcoding a season that would go
// stale (exactly the bug fixed in internal/suggest's own tests).
func currentSeason() string {
	return string(suggest.CurrentSeason(time.Now(), true))
}

func tagItemForSuggest(t *testing.T, s *store.Store, userID int64, category string, formality, warmth int, seasons, occasions []string) {
	t.Helper()
	ctx := context.Background()
	itemID, err := s.CreateItem(ctx, userID, "photo.jpg", "thumb.jpg")
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}
	warmthMin, warmthMax := legacyRanges(category, warmth)
	if err := s.UpdateItemTags(ctx, userID, itemID, store.ItemTags{
		Category: category, Subcategory: "generic", ColorPrimary: "blue",
		Pattern: "solid", Material: "cotton", Fit: "regular",
		FormalityMin: formality, FormalityMax: formality, WarmthMin: warmthMin, WarmthMax: warmthMax,
		Seasons: seasons, Occasions: occasions,
	}); err != nil {
		t.Fatalf("UpdateItemTags: %v", err)
	}
}

func getSuggest(h http.Handler, cookie *http.Cookie, occasion string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/suggest?occasion="+occasion, nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func getSuggestWithParams(h http.Handler, cookie *http.Cookie, query string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/suggest?"+query, nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestSuggestEndpointRejectsInvalidOccasion(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")

	rec := getSuggest(h, cookie, "bogus")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("occasion=bogus: status %d, want %d", rec.Code, http.StatusBadRequest)
	}

	rec = getSuggest(h, cookie, "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("occasion=(empty): status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestSuggestEndpointReturnsOutfitsWithoutLocation(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")

	season := currentSeason()
	tagItemForSuggest(t, s, 1, "top", 2, 2, []string{season}, []string{"casual"})
	tagItemForSuggest(t, s, 1, "bottom", 2, 2, []string{season}, []string{"casual"})

	rec := getSuggest(h, cookie, "casual")
	if rec.Code != http.StatusOK {
		t.Fatalf("get suggest: status %d, body %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["weather_available"] != false {
		t.Errorf("weather_available = %v, want false (no location set)", resp["weather_available"])
	}
	outfits, _ := resp["outfits"].([]any)
	if len(outfits) == 0 {
		t.Errorf("expected at least one outfit, got none (response: %v)", resp)
	}
}

func TestSuggestEndpointWithWeather(t *testing.T) {
	s := newTestStore(t)

	fakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"current": {"wind_speed_10m": 10.0, "weather_code": 0},
			"daily": {
				"temperature_2m_max": [28.0],
				"temperature_2m_min": [20.0],
				"precipitation_probability_max": [0]
			}
		}`))
	}))
	t.Cleanup(fakeServer.Close)

	h := newTestHandlerFull(t, s, true, weather.NewClientWithBaseURL(fakeServer.URL))
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")
	patchSettings(h, cookie, 47.3769, 8.5417)

	season := currentSeason()
	tagItemForSuggest(t, s, 1, "top", 2, 1, []string{season}, []string{"casual"})
	tagItemForSuggest(t, s, 1, "bottom", 2, 1, []string{season}, []string{"casual"})

	rec := getSuggest(h, cookie, "casual")
	if rec.Code != http.StatusOK {
		t.Fatalf("get suggest: status %d, body %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["weather_available"] != true {
		t.Errorf("weather_available = %v, want true", resp["weather_available"])
	}
}

func TestSuggestEndpointNothingMatchesIsStill200(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")
	// No items tagged "formal" at all.

	rec := getSuggest(h, cookie, "formal")
	if rec.Code != http.StatusOK {
		t.Fatalf("nothing-matches case: status %d, want %d (never an error)", rec.Code, http.StatusOK)
	}

	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	outfits, _ := resp["outfits"].([]any)
	if len(outfits) != 0 {
		t.Errorf("expected no outfits, got %d", len(outfits))
	}
	if resp["no_match_reason"] == nil || resp["no_match_reason"] == "" {
		t.Error("expected a non-empty no_match_reason")
	}
}

func TestSuggestEndpointSourceIsAIWhenEnabled(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s) // aiEnabled=true, mock provider (see newTestHandlerFull)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")

	season := currentSeason()
	tagItemForSuggest(t, s, 1, "top", 2, 2, []string{season}, []string{"casual"})
	tagItemForSuggest(t, s, 1, "bottom", 2, 2, []string{season}, []string{"casual"})

	rec := getSuggestWithParams(h, cookie, "occasion=casual&mood=relaxed&time_of_day=morning")
	if rec.Code != http.StatusOK {
		t.Fatalf("get suggest: status %d, body %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["source"] != "ai" {
		t.Errorf("source = %v, want %q", resp["source"], "ai")
	}
	outfits, _ := resp["outfits"].([]any)
	if len(outfits) == 0 {
		t.Fatalf("expected at least one outfit, got none (response: %v)", resp)
	}
}

func TestSuggestEndpointSourceIsRulesFallbackWhenAIDisabled(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandlerAI(t, s, false) // aiEnabled=false (AI_PROVIDER=off equivalent)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")

	season := currentSeason()
	tagItemForSuggest(t, s, 1, "top", 2, 2, []string{season}, []string{"casual"})
	tagItemForSuggest(t, s, 1, "bottom", 2, 2, []string{season}, []string{"casual"})

	rec := getSuggest(h, cookie, "casual")
	if rec.Code != http.StatusOK {
		t.Fatalf("get suggest: status %d, body %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["source"] != "rules_fallback" {
		t.Errorf("source = %v, want %q", resp["source"], "rules_fallback")
	}
}

func TestSuggestEndpointOwnershipScoped(t *testing.T) {
	s := newTestStore(t)
	h := newTestHandler(t, s)

	season := currentSeason()
	cookieA := loginAndGetCookie(t, h, s, "a@example.com", "correct horse battery staple")
	tagItemForSuggest(t, s, 1, "top", 2, 2, []string{season}, []string{"casual"})
	tagItemForSuggest(t, s, 1, "bottom", 2, 2, []string{season}, []string{"casual"})

	cookieB := loginAndGetCookie(t, h, s, "b@example.com", "correct horse battery staple")

	rec := getSuggest(h, cookieB, "casual")
	if rec.Code != http.StatusOK {
		t.Fatalf("get suggest as user B: status %d", rec.Code)
	}
	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	outfits, _ := resp["outfits"].([]any)
	if len(outfits) != 0 {
		t.Errorf("user B should not see user A's items in suggestions, got %d outfits", len(outfits))
	}
	_ = cookieA
}

// legacyRanges spreads a single formality/warmth value into ranges the
// way migrations/0006_item_ranges.sql does for existing items, so fixtures
// written as single values behave like a migrated wardrobe.
func legacyRanges(category string, warmth int) (warmthMin, warmthMax int) {
	switch category {
	case "top":
		return max(1, warmth-1), 5
	case "outerwear":
		return max(1, warmth-1), min(5, warmth+1)
	case "bottom", "dress", "shoes":
		return max(1, warmth-1), min(5, warmth+2)
	default:
		return 1, 5
	}
}

// The device's current position is enough for weather-aware suggestions,
// with no home location saved.
func TestSuggestEndpointUsesCurrentLocation(t *testing.T) {
	s := newTestStore(t)
	wc, asked := weatherAt(t)
	h := newTestHandlerFull(t, s, true, wc)
	cookie := loginAndGetCookie(t, h, s, "me@example.com", "correct horse battery staple")

	season := currentSeason()
	tagItemForSuggest(t, s, 1, "top", 2, 1, []string{season}, []string{"casual"})
	tagItemForSuggest(t, s, 1, "bottom", 2, 1, []string{season}, []string{"casual"})

	rec := getSuggestWithParams(h, cookie, "occasion=casual&lat=40.4168&lon=-3.7038")
	if rec.Code != http.StatusOK {
		t.Fatalf("get suggest: status %d, body %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["weather_available"] != true {
		t.Errorf("weather_available = %v, want true", resp["weather_available"])
	}
	if len(*asked) != 1 || (*asked)[0] != "40.416800,-3.703800" {
		t.Errorf("upstream asked for %v, want the current location", *asked)
	}
}
