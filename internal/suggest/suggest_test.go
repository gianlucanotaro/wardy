package suggest

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gianlucanotaro/wardy/internal/store"
	"github.com/gianlucanotaro/wardy/internal/taxonomy"
	"github.com/gianlucanotaro/wardy/internal/weather"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// createTaggedItem builds a fixture wardrobe item with the given
// category and attributes, going through the same store methods the real
// app uses (CreateItem + UpdateItemTags), so these tests exercise the
// same path production traffic does.
func createTaggedItem(t *testing.T, s *store.Store, userID int64, category, subcategory string, formality, warmth int, seasons, occasions []string) int64 {
	t.Helper()
	ctx := context.Background()

	itemID, err := s.CreateItem(ctx, userID, "photo.jpg", "thumb.jpg")
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	if err := s.UpdateItemTags(ctx, userID, itemID, store.ItemTags{
		Category: category, Subcategory: subcategory, ColorPrimary: "blue",
		Pattern: "solid", Material: "cotton", Fit: "regular",
		Formality: formality, Warmth: warmth, Seasons: seasons, Occasions: occasions,
	}); err != nil {
		t.Fatalf("UpdateItemTags: %v", err)
	}

	return itemID
}

// A fixture wardrobe covering casual and work basics across warm and cold
// gear, used by most of the tests below.
func buildFixtureWardrobe(t *testing.T, s *store.Store, userID int64) {
	t.Helper()
	// Summer casual basics.
	createTaggedItem(t, s, userID, "top", "t-shirt", 2, 1, []string{"summer"}, []string{"casual"})
	createTaggedItem(t, s, userID, "bottom", "shorts", 2, 1, []string{"summer"}, []string{"casual"})
	createTaggedItem(t, s, userID, "shoes", "sneakers", 2, 1, []string{"summer"}, []string{"casual"})

	// Winter casual basics, including outerwear.
	createTaggedItem(t, s, userID, "top", "sweater", 2, 5, []string{"winter"}, []string{"casual"})
	createTaggedItem(t, s, userID, "bottom", "jeans", 2, 4, []string{"winter"}, []string{"casual"})
	createTaggedItem(t, s, userID, "outerwear", "coat", 2, 5, []string{"winter"}, []string{"casual"})
	createTaggedItem(t, s, userID, "shoes", "boots", 2, 4, []string{"winter"}, []string{"casual"})

	// Work basics, summer only.
	createTaggedItem(t, s, userID, "top", "button-down", 4, 2, []string{"summer"}, []string{"work"})
	createTaggedItem(t, s, userID, "bottom", "trousers", 4, 2, []string{"summer"}, []string{"work"})
	createTaggedItem(t, s, userID, "shoes", "dress-shoes", 4, 2, []string{"summer"}, []string{"work"})
}

func hotSummerForecast() weather.Forecast {
	return weather.Forecast{TempHighC: 30, TempLowC: 22}
}

func freezingForecast() weather.Forecast {
	return weather.Forecast{TempHighC: 2, TempLowC: -5}
}

// Fixed dates for the two hemispheres' seasons, so these tests don't
// depend on what day it actually is when they run — Suggest takes "now"
// as an explicit parameter for exactly this reason.
var (
	julyDate    = time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	januaryDate = time.Date(2026, time.January, 15, 12, 0, 0, 0, time.UTC)
)

func TestSuggestBasicMatch(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	buildFixtureWardrobe(t, s, userID)

	forecast := hotSummerForecast()
	result, err := Suggest(ctx, s, userID, taxonomy.OccasionCasual, &forecast, true, julyDate)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if len(result.Outfits) == 0 {
		t.Fatalf("expected at least one outfit, got none (reason: %q)", result.NoMatchReason)
	}

	// Every item in the outfit should actually be tagged casual/summer —
	// spot check the assembled outfit isn't pulling in the winter or work
	// items.
	for _, outfit := range result.Outfits {
		for _, item := range outfit.Items {
			if !contains(item.Seasons, "summer") {
				t.Errorf("outfit included a non-summer item: %+v", item)
			}
		}
	}
}

func TestSuggestColdWeatherAddsOuterwear(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	buildFixtureWardrobe(t, s, userID)

	forecast := freezingForecast()
	result, err := Suggest(ctx, s, userID, taxonomy.OccasionCasual, &forecast, true, januaryDate)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if len(result.Outfits) == 0 {
		t.Fatalf("expected at least one outfit, got none (reason: %q)", result.NoMatchReason)
	}

	found := false
	for _, outfit := range result.Outfits {
		for _, item := range outfit.Items {
			if item.Category == "outerwear" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected at least one outfit to include outerwear when it's freezing")
	}
}

func TestSuggestWeatherUnavailableStillSuggests(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	buildFixtureWardrobe(t, s, userID)

	// forecast is nil — weather fetch failed or no location set. Docs:
	// "Weather down -> suggest anyway."
	result, err := Suggest(ctx, s, userID, taxonomy.OccasionCasual, nil, true, julyDate)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if len(result.Outfits) == 0 {
		t.Fatalf("expected outfits even with no weather data, got none (reason: %q)", result.NoMatchReason)
	}
}

func TestSuggestOccasionMatchesWork(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	buildFixtureWardrobe(t, s, userID)

	forecast := hotSummerForecast()
	result, err := Suggest(ctx, s, userID, taxonomy.OccasionWork, &forecast, true, julyDate)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if len(result.Outfits) == 0 {
		t.Fatalf("expected a work outfit, got none (reason: %q)", result.NoMatchReason)
	}
	for _, outfit := range result.Outfits {
		for _, item := range outfit.Items {
			if !contains(item.Occasions, "work") {
				t.Errorf("work suggestion included a non-work item: %+v", item)
			}
		}
	}
}

func TestSuggestNothingMatchesNamesOccasion(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	buildFixtureWardrobe(t, s, userID) // no "formal" occasion items at all

	forecast := hotSummerForecast()
	result, err := Suggest(ctx, s, userID, taxonomy.OccasionFormal, &forecast, true, julyDate)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if len(result.Outfits) != 0 {
		t.Fatalf("expected no outfits for an occasion with zero tagged items, got %d", len(result.Outfits))
	}
	if result.NoMatchReason == "" {
		t.Fatal("expected a non-empty no-match reason")
	}
	if !strings.Contains(result.NoMatchReason, "occasion") {
		t.Errorf("expected the reason to point at the occasion, got: %q", result.NoMatchReason)
	}
}

func TestSuggestNothingMatchesWornRecently(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")

	// A single matching top+bottom combo, worn today. Uses the real
	// time.Now() (not the fixed julyDate) because the store layer's
	// "worn recently" exclusion is genuinely wall-clock-relative, unlike
	// season, which only depends on the calendar month.
	topID := createTaggedItem(t, s, userID, "top", "t-shirt", 2, 1, []string{"summer"}, []string{"casual"})
	bottomID := createTaggedItem(t, s, userID, "bottom", "shorts", 2, 1, []string{"summer"}, []string{"casual"})

	if err := s.LogWear(ctx, userID, []int64{topID, bottomID}, time.Now()); err != nil {
		t.Fatalf("LogWear: %v", err)
	}

	forecast := hotSummerForecast()
	result, err := Suggest(ctx, s, userID, taxonomy.OccasionCasual, &forecast, true, julyDate)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if len(result.Outfits) != 0 {
		t.Fatalf("expected no outfits (only match was worn today), got %d", len(result.Outfits))
	}
	if !strings.Contains(strings.ToLower(result.NoMatchReason), "worn") {
		t.Errorf("expected the reason to mention it was worn recently, got: %q", result.NoMatchReason)
	}
}

func TestSuggestCapsAtMaxOutfits(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")

	for i := 0; i < 10; i++ {
		createTaggedItem(t, s, userID, "top", "t-shirt", 2, 1, []string{"summer"}, []string{"casual"})
		createTaggedItem(t, s, userID, "bottom", "shorts", 2, 1, []string{"summer"}, []string{"casual"})
	}

	forecast := hotSummerForecast()
	result, err := Suggest(ctx, s, userID, taxonomy.OccasionCasual, &forecast, true, julyDate)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if len(result.Outfits) != MaxOutfits {
		t.Errorf("got %d outfits, want exactly MaxOutfits (%d)", len(result.Outfits), MaxOutfits)
	}
}

func TestSuggestOwnershipScoped(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userA, _ := s.CreateUser(ctx, "a@example.com", "hash", "A")
	userB, _ := s.CreateUser(ctx, "b@example.com", "hash", "B")
	buildFixtureWardrobe(t, s, userA)
	// userB has no items at all.

	forecast := hotSummerForecast()
	result, err := Suggest(ctx, s, userB, taxonomy.OccasionCasual, &forecast, true, julyDate)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if len(result.Outfits) != 0 {
		t.Errorf("user B should get no outfits from user A's wardrobe, got %d", len(result.Outfits))
	}
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
