package suggest

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gianlucanotaro/wardy/internal/ai"
	"github.com/gianlucanotaro/wardy/internal/store"
	"github.com/gianlucanotaro/wardy/internal/taxonomy"
	"github.com/gianlucanotaro/wardy/internal/weather"
)

// fakeAIProvider lets each SuggestAI test script exactly what
// SuggestOutfits returns, and records whether it was called at all — the
// budget/cap/kill-switch tests need to assert the model was never even
// invoked.
type fakeAIProvider struct {
	suggestions []ai.AISuggestion
	usage       ai.Usage
	err         error
	called      bool
	// gotCandidates captures the last candidates argument SuggestOutfits
	// was called with, so tests can inspect what suggest.go actually
	// sent the model without needing toAICandidates to be exported.
	gotCandidates []ai.SuggestionCandidate
	gotContext    ai.SuggestContext
}

func (f *fakeAIProvider) TagItem(_ context.Context, _ []byte) (ai.TagResult, ai.Usage, error) {
	return ai.TagResult{}, ai.Usage{}, errors.New("TagItem not used by suggest package tests")
}

func (f *fakeAIProvider) SuggestOutfits(_ context.Context, candidates []ai.SuggestionCandidate, _ string, sc ai.SuggestContext) ([]ai.AISuggestion, ai.Usage, error) {
	f.called = true
	f.gotCandidates = candidates
	f.gotContext = sc
	return f.suggestions, f.usage, f.err
}

func (f *fakeAIProvider) TagInspo(_ context.Context, _ []byte) (ai.InspoTagResult, ai.Usage, error) {
	return ai.InspoTagResult{}, ai.Usage{}, errors.New("TagInspo not used by suggest package tests")
}

func (f *fakeAIProvider) SummarizeStyle(_ context.Context, _ string) (string, ai.Usage, error) {
	return "", ai.Usage{}, errors.New("SummarizeStyle not used by suggest package tests")
}

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

	warmthMin, warmthMax := legacyRanges(category, warmth)
	if err := s.UpdateItemTags(ctx, userID, itemID, store.ItemTags{
		Category: category, Subcategory: subcategory, ColorPrimary: "blue",
		Pattern: "solid", Material: "cotton", Fit: "regular",
		FormalityMin: formality, FormalityMax: formality, WarmthMin: warmthMin, WarmthMax: warmthMax,
		Seasons: seasons, Occasions: occasions,
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

// Casual summer basics only (formality 2) — nothing a formal occasion
// (4-5) can use.
func buildCasualOnlyWardrobe(t *testing.T, s *store.Store, userID int64) {
	t.Helper()
	createTaggedItem(t, s, userID, "top", "t-shirt", 2, 1, []string{"summer"}, []string{"casual"})
	createTaggedItem(t, s, userID, "bottom", "shorts", 2, 1, []string{"summer"}, []string{"casual"})
	createTaggedItem(t, s, userID, "shoes", "sneakers", 2, 1, []string{"summer"}, []string{"casual"})
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

	// Nothing too warm for a hot day: the fixture's sweater, jeans, coat
	// and boots (warmth 4-5) must stay out.
	target := TargetWarmth(forecast)
	for _, outfit := range result.Outfits {
		for _, item := range outfit.Items {
			if !FitsWeather(item, target) {
				t.Errorf("hot-day outfit included an item outside its warmth range (%d-%d, target %d): %+v", item.WarmthMin, item.WarmthMax, target, item)
			}
		}
	}
}

// TestSuggestRulesPreFilterIgnoresCustomTags confirms the deterministic
// pre-filter (Suggest, not SuggestAI) never uses custom_tags to include
// or exclude an item — it's free text, and the pre-filter is deliberately
// typed/arithmetic (docs/taxonomy.md). An item with a custom tag that
// looks nothing like a matching criterion must still match purely on its
// structured fields, same as one with no custom tags at all.
func TestSuggestRulesPreFilterIgnoresCustomTags(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")

	topID := createTaggedItem(t, s, userID, "top", "t-shirt", 2, 1, []string{"summer"}, []string{"casual"})
	if err := s.UpdateItemTags(ctx, userID, topID, store.ItemTags{
		Category: "top", Subcategory: "t-shirt", ColorPrimary: "blue",
		Pattern: "solid", Material: "cotton", Fit: "regular",
		FormalityMin: 2, FormalityMax: 2, WarmthMin: 1, WarmthMax: 5,
		Seasons: []string{"summer"}, Occasions: []string{"casual"},
		CustomTags: []string{"needs mending"},
	}); err != nil {
		t.Fatalf("UpdateItemTags: %v", err)
	}
	createTaggedItem(t, s, userID, "bottom", "shorts", 2, 1, []string{"summer"}, []string{"casual"})

	forecast := hotSummerForecast()
	result, err := Suggest(ctx, s, userID, taxonomy.OccasionCasual, &forecast, true, julyDate)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if len(result.Outfits) == 0 {
		t.Fatalf("expected an outfit including the custom-tagged item, got none (reason: %q)", result.NoMatchReason)
	}
	found := false
	for _, outfit := range result.Outfits {
		for _, item := range outfit.Items {
			if item.ID == topID {
				found = true
			}
		}
	}
	if !found {
		t.Error("item with a custom tag should match on its structured fields exactly like any other item")
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
			if item.FormalityMax < OccasionFormality(taxonomy.OccasionWork).Min {
				t.Errorf("work suggestion included a too-casual item (formality %d-%d): %+v", item.FormalityMin, item.FormalityMax, item)
			}
		}
	}
}

func TestSuggestNothingMatchesNamesFormality(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	buildCasualOnlyWardrobe(t, s, userID) // nothing formal enough

	forecast := hotSummerForecast()
	result, err := Suggest(ctx, s, userID, taxonomy.OccasionFormal, &forecast, true, julyDate)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if len(result.Outfits) != 0 {
		t.Fatalf("expected no outfits when nothing is formal enough, got %d", len(result.Outfits))
	}
	if result.NoMatchReason == "" {
		t.Fatal("expected a non-empty no-match reason")
	}
	if !strings.Contains(result.NoMatchReason, "formality") {
		t.Errorf("expected the reason to point at formality, got: %q", result.NoMatchReason)
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

	if _, err := s.LogWear(ctx, userID, nil, []int64{topID, bottomID}, time.Now(), "", ""); err != nil {
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

func TestSuggestAI_UsesAIRationaleWhenAvailable(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	topID := createTaggedItem(t, s, userID, "top", "t-shirt", 2, 1, []string{"summer"}, []string{"casual"})
	bottomID := createTaggedItem(t, s, userID, "bottom", "shorts", 2, 1, []string{"summer"}, []string{"casual"})

	provider := &fakeAIProvider{
		suggestions: []ai.AISuggestion{
			{ItemIDs: []int64{topID, bottomID}, Rationale: "Light and breathable for a hot day."},
		},
		usage: ai.Usage{Model: ai.SuggestionModelName, InputTokens: 100, OutputTokens: 20, EstCostUSD: 0.001},
	}

	forecast := hotSummerForecast()
	result, err := SuggestAI(ctx, s, provider, true, 5.00, DailySuggestionCap, userID, taxonomy.OccasionCasual, ai.SuggestContext{Mood: "easy", TimeOfDay: "morning"}, &forecast, true, julyDate)
	if err != nil {
		t.Fatalf("SuggestAI: %v", err)
	}
	if !provider.called {
		t.Fatal("expected the AI provider to be called")
	}
	if result.Source != SourceAI {
		t.Errorf("Source = %q, want %q", result.Source, SourceAI)
	}
	if len(result.Outfits) != 1 {
		t.Fatalf("got %d outfits, want 1", len(result.Outfits))
	}
	if result.Outfits[0].Note != "Light and breathable for a hot day." {
		t.Errorf("Note = %q, want the AI rationale", result.Outfits[0].Note)
	}
}

// TestSuggestAI_PassesRequestAndNames confirms the user's request reaches
// the model untouched alongside the mood, and that item names ride along
// in the candidates so "the green jacket" can be found.
func TestSuggestAI_PassesRequestAndNames(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	topID := createTaggedItem(t, s, userID, "top", "t-shirt", 2, 1, []string{"summer"}, []string{"casual"})
	bottomID := createTaggedItem(t, s, userID, "bottom", "shorts", 2, 1, []string{"summer"}, []string{"casual"})
	if err := s.UpdateItemName(ctx, userID, topID, "Lucky tee"); err != nil {
		t.Fatalf("UpdateItemName: %v", err)
	}

	provider := &fakeAIProvider{
		suggestions: []ai.AISuggestion{{ItemIDs: []int64{topID, bottomID}, Rationale: "Your lucky tee, as asked."}},
		usage:       ai.Usage{Model: ai.SuggestionModelName},
	}
	sc := ai.SuggestContext{Mood: "playful", Prompt: "wear my lucky tee"}
	forecast := hotSummerForecast()
	if _, err := SuggestAI(ctx, s, provider, true, 5.00, DailySuggestionCap, userID, taxonomy.OccasionCasual, sc, &forecast, true, julyDate); err != nil {
		t.Fatalf("SuggestAI: %v", err)
	}
	if provider.gotContext != sc {
		t.Errorf("provider got context %+v, want %+v", provider.gotContext, sc)
	}
	var named bool
	for _, c := range provider.gotCandidates {
		if c.ID == topID && c.Name == "Lucky tee" {
			named = true
		}
	}
	if !named {
		t.Errorf("candidates %+v do not carry the top's name", provider.gotCandidates)
	}
}

// TestSuggestAI_PassesCustomTagsAsContext confirms custom tags ride along
// in the compact candidate payload sent to the model — context for the
// rationale, per docs/ai.md's mood/time_of_day pattern, never a filter:
// the candidate set itself is already fixed by the rules pre-filter
// before SuggestOutfits is even called.
func TestSuggestAI_PassesCustomTagsAsContext(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	topID := createTaggedItem(t, s, userID, "top", "t-shirt", 2, 1, []string{"summer"}, []string{"casual"})
	if err := s.UpdateItemTags(ctx, userID, topID, store.ItemTags{
		Category: "top", Subcategory: "t-shirt", ColorPrimary: "blue",
		Pattern: "solid", Material: "cotton", Fit: "regular",
		FormalityMin: 2, FormalityMax: 2, WarmthMin: 1, WarmthMax: 5,
		Seasons: []string{"summer"}, Occasions: []string{"casual"},
		CustomTags: []string{"favorite"},
	}); err != nil {
		t.Fatalf("UpdateItemTags: %v", err)
	}
	bottomID := createTaggedItem(t, s, userID, "bottom", "shorts", 2, 1, []string{"summer"}, []string{"casual"})

	provider := &fakeAIProvider{
		suggestions: []ai.AISuggestion{{ItemIDs: []int64{topID, bottomID}, Rationale: "Your favorite tee, kept cool."}},
		usage:       ai.Usage{Model: ai.SuggestionModelName},
	}

	forecast := hotSummerForecast()
	_, err := SuggestAI(ctx, s, provider, true, 5.00, DailySuggestionCap, userID, taxonomy.OccasionCasual, ai.SuggestContext{}, &forecast, true, julyDate)
	if err != nil {
		t.Fatalf("SuggestAI: %v", err)
	}

	var topCandidate *ai.SuggestionCandidate
	for i := range provider.gotCandidates {
		if provider.gotCandidates[i].ID == topID {
			topCandidate = &provider.gotCandidates[i]
		}
	}
	if topCandidate == nil {
		t.Fatalf("top item not found in candidates sent to the model: %+v", provider.gotCandidates)
	}
	if len(topCandidate.CustomTags) != 1 || topCandidate.CustomTags[0] != "favorite" {
		t.Errorf("CustomTags = %+v, want [favorite]", topCandidate.CustomTags)
	}
}

func TestSuggestAI_DropsHallucinatedIDsButKeepsValidOutfits(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	topID := createTaggedItem(t, s, userID, "top", "t-shirt", 2, 1, []string{"summer"}, []string{"casual"})
	bottomID := createTaggedItem(t, s, userID, "bottom", "shorts", 2, 1, []string{"summer"}, []string{"casual"})

	const hallucinatedID = 999999

	provider := &fakeAIProvider{
		suggestions: []ai.AISuggestion{
			{ItemIDs: []int64{topID, bottomID}, Rationale: "A valid outfit."},
			{ItemIDs: []int64{hallucinatedID}, Rationale: "An outfit that doesn't exist."},
		},
		usage: ai.Usage{Model: ai.SuggestionModelName},
	}

	forecast := hotSummerForecast()
	result, err := SuggestAI(ctx, s, provider, true, 5.00, DailySuggestionCap, userID, taxonomy.OccasionCasual, ai.SuggestContext{}, &forecast, true, julyDate)
	if err != nil {
		t.Fatalf("SuggestAI: %v", err)
	}
	if result.Source != SourceAI {
		t.Errorf("Source = %q, want %q (one outfit still valid)", result.Source, SourceAI)
	}
	if len(result.Outfits) != 1 {
		t.Fatalf("got %d outfits, want exactly the 1 valid one (hallucinated outfit should be dropped)", len(result.Outfits))
	}
	if result.Outfits[0].Note != "A valid outfit." {
		t.Errorf("unexpected surviving outfit: %+v", result.Outfits[0])
	}
}

func TestSuggestAI_FallsBackWhenEveryOutfitIsHallucinated(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	buildFixtureWardrobe(t, s, userID)

	provider := &fakeAIProvider{
		suggestions: []ai.AISuggestion{{ItemIDs: []int64{999999}, Rationale: "invented"}},
		usage:       ai.Usage{Model: ai.SuggestionModelName},
	}

	forecast := hotSummerForecast()
	result, err := SuggestAI(ctx, s, provider, true, 5.00, DailySuggestionCap, userID, taxonomy.OccasionCasual, ai.SuggestContext{}, &forecast, true, julyDate)
	if err != nil {
		t.Fatalf("SuggestAI: %v", err)
	}
	if result.Source != SourceRulesFallback {
		t.Errorf("Source = %q, want %q when every AI outfit is invalid", result.Source, SourceRulesFallback)
	}
	if len(result.Outfits) == 0 {
		t.Fatalf("expected the rules fallback to still produce outfits, got none (reason: %q)", result.NoMatchReason)
	}
}

func TestSuggestAI_FallsBackWhenAIDisabled(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	buildFixtureWardrobe(t, s, userID)

	provider := &fakeAIProvider{}
	forecast := hotSummerForecast()

	// aiEnabled=false is the kill switch (AI_PROVIDER=off) — must fully
	// degrade to Suggest()'s own rules-only behavior.
	got, err := SuggestAI(ctx, s, provider, false, 5.00, DailySuggestionCap, userID, taxonomy.OccasionCasual, ai.SuggestContext{}, &forecast, true, julyDate)
	if err != nil {
		t.Fatalf("SuggestAI: %v", err)
	}
	if provider.called {
		t.Error("provider must not be called when AI is disabled")
	}
	if got.Source != SourceRulesFallback {
		t.Errorf("Source = %q, want %q", got.Source, SourceRulesFallback)
	}

	want, err := Suggest(ctx, s, userID, taxonomy.OccasionCasual, &forecast, true, julyDate)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if len(got.Outfits) != len(want.Outfits) {
		t.Errorf("SuggestAI with AI disabled produced %d outfits, Suggest produced %d — should match exactly", len(got.Outfits), len(want.Outfits))
	}
}

func TestSuggestAI_FallsBackOverMonthlyBudget(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	buildFixtureWardrobe(t, s, userID)

	// Already over a tiny budget before SuggestAI even looks at the request.
	if err := s.LogUsage(ctx, userID, "claude-haiku-4-5-20251001", 1000, 1000, 1.00); err != nil {
		t.Fatalf("LogUsage: %v", err)
	}

	provider := &fakeAIProvider{}
	forecast := hotSummerForecast()
	result, err := SuggestAI(ctx, s, provider, true, 0.01, DailySuggestionCap, userID, taxonomy.OccasionCasual, ai.SuggestContext{}, &forecast, true, julyDate)
	if err != nil {
		t.Fatalf("SuggestAI: %v", err)
	}
	if provider.called {
		t.Error("provider must not be called once the monthly budget is exceeded")
	}
	if result.Source != SourceRulesFallback {
		t.Errorf("Source = %q, want %q", result.Source, SourceRulesFallback)
	}
}

func TestSuggestAI_FallsBackOverDailyCap(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	buildFixtureWardrobe(t, s, userID)

	const cap = 3
	for i := 0; i < cap; i++ {
		if err := s.LogUsage(ctx, userID, ai.SuggestionModelName, 100, 20, 0.001); err != nil {
			t.Fatalf("LogUsage: %v", err)
		}
	}

	provider := &fakeAIProvider{}
	forecast := hotSummerForecast()
	result, err := SuggestAI(ctx, s, provider, true, 5.00, cap, userID, taxonomy.OccasionCasual, ai.SuggestContext{}, &forecast, true, julyDate)
	if err != nil {
		t.Fatalf("SuggestAI: %v", err)
	}
	if provider.called {
		t.Error("provider must not be called once the daily suggestion cap is exceeded")
	}
	if result.Source != SourceRulesFallback {
		t.Errorf("Source = %q, want %q", result.Source, SourceRulesFallback)
	}
}

func TestSuggestAI_FallsBackOnProviderError(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	buildFixtureWardrobe(t, s, userID)

	provider := &fakeAIProvider{err: errors.New("connection reset")}
	forecast := hotSummerForecast()
	result, err := SuggestAI(ctx, s, provider, true, 5.00, DailySuggestionCap, userID, taxonomy.OccasionCasual, ai.SuggestContext{}, &forecast, true, julyDate)
	if err != nil {
		t.Fatalf("SuggestAI: %v", err)
	}
	if result.Source != SourceRulesFallback {
		t.Errorf("Source = %q, want %q", result.Source, SourceRulesFallback)
	}
	if len(result.Outfits) == 0 {
		t.Fatalf("expected the rules fallback to still produce outfits, got none (reason: %q)", result.NoMatchReason)
	}
}

func TestSuggestAI_NothingMatchesNeverCallsProvider(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	buildCasualOnlyWardrobe(t, s, userID) // nothing formal enough

	provider := &fakeAIProvider{}
	forecast := hotSummerForecast()
	result, err := SuggestAI(ctx, s, provider, true, 5.00, DailySuggestionCap, userID, taxonomy.OccasionFormal, ai.SuggestContext{}, &forecast, true, julyDate)
	if err != nil {
		t.Fatalf("SuggestAI: %v", err)
	}
	if provider.called {
		t.Error("provider should not be called when the rules pass already found nothing to work with")
	}
	if len(result.Outfits) != 0 || result.NoMatchReason == "" {
		t.Errorf("expected the nothing-matches state, got outfits=%d reason=%q", len(result.Outfits), result.NoMatchReason)
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

// TestSuggestAI_RequestedPieceSurvivesTheCap: with more candidates than
// MaxCandidates, the round-robin sample keeps each category's first items.
// The one red top must still reach the model when the user asks for it.
func TestSuggestAI_RequestedPieceSurvivesTheCap(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	for range MaxCandidates {
		createTaggedItem(t, s, userID, "top", "t-shirt", 2, 1, []string{"summer"}, []string{"casual"})
	}
	bottomID := createTaggedItem(t, s, userID, "bottom", "shorts", 2, 1, []string{"summer"}, []string{"casual"})
	// Oldest of the tops and untagged, so it sorts last without the request.
	redID := createTaggedItem(t, s, userID, "top", "t-shirt", 2, 1, nil, nil)
	if err := s.UpdateItemTags(ctx, userID, redID, store.ItemTags{
		Category: "top", Subcategory: "t-shirt", ColorPrimary: "red", Pattern: "solid", Material: "cotton", Fit: "regular",
		FormalityMin: 2, FormalityMax: 2, WarmthMin: 1, WarmthMax: 5,
	}); err != nil {
		t.Fatalf("UpdateItemTags: %v", err)
	}

	has := func(cs []ai.SuggestionCandidate, id int64) bool {
		return slices.ContainsFunc(cs, func(c ai.SuggestionCandidate) bool { return c.ID == id })
	}
	forecast := hotSummerForecast()
	run := func(prompt string) []ai.SuggestionCandidate {
		provider := &fakeAIProvider{
			suggestions: []ai.AISuggestion{{ItemIDs: []int64{redID, bottomID}, Rationale: "Red, as asked."}},
			usage:       ai.Usage{Model: ai.SuggestionModelName},
		}
		if _, err := SuggestAI(ctx, s, provider, true, 0, 0, userID, taxonomy.OccasionCasual, ai.SuggestContext{Prompt: prompt}, &forecast, true, julyDate); err != nil {
			t.Fatalf("SuggestAI: %v", err)
		}
		return provider.gotCandidates
	}

	if got := run(""); has(got, redID) {
		t.Fatalf("fixture too weak: red top made the cut without a request")
	}
	got := run("I want something with a red top")
	if !has(got, redID) {
		t.Errorf("red top missing from %d candidates sent to the model", len(got))
	}
	if !has(got, bottomID) {
		t.Errorf("ranking dropped the only bottom")
	}
}
