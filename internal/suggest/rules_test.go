package suggest

import (
	"context"
	"strings"
	"testing"

	"github.com/gianlucanotaro/wardy/internal/ai"
	"github.com/gianlucanotaro/wardy/internal/store"
	"github.com/gianlucanotaro/wardy/internal/taxonomy"
)

// Outfit-level rules (#29): warmth is reached by layering, formality is a
// per-occasion range plus coherence within one outfit, and occasion/season
// tags are preferences.

func outfitHas(o Outfit, id int64) bool {
	for _, it := range o.Items {
		if it.ID == id {
			return true
		}
	}
	return false
}

func anyOutfitHas(outfits []Outfit, id int64) bool {
	for _, o := range outfits {
		if outfitHas(o, id) {
			return true
		}
	}
	return false
}

func TestSuggestLightShirtLayeredInWinter(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")

	// Tagged summer-only, warmth 2: the old ±1 window kept it out of
	// winter entirely.
	shirt := createTaggedItem(t, s, userID, "top", "button-down", 3, 2, []string{"summer"}, []string{"casual"})
	knit := createTaggedItem(t, s, userID, "top", "knit", 3, 5, []string{"winter"}, []string{"casual"})
	createTaggedItem(t, s, userID, "bottom", "jeans", 2, 4, []string{"winter"}, []string{"casual"})
	createTaggedItem(t, s, userID, "outerwear", "coat", 3, 5, []string{"winter"}, []string{"casual"})
	createTaggedItem(t, s, userID, "shoes", "boots", 3, 4, []string{"winter"}, []string{"casual"})

	forecast := freezingForecast()
	result, err := Suggest(ctx, s, userID, taxonomy.OccasionCasual, &forecast, true, januaryDate)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}

	var withShirt *Outfit
	for i := range result.Outfits {
		if outfitHas(result.Outfits[i], shirt) {
			withShirt = &result.Outfits[i]
		}
	}
	if withShirt == nil {
		t.Fatalf("expected the light shirt in a winter outfit, got %+v (reason %q)", result.Outfits, result.NoMatchReason)
	}
	if !outfitHas(*withShirt, knit) {
		t.Error("the shirt should be layered under the knit on a freezing day")
	}
	hasCoat := false
	for _, it := range withShirt.Items {
		hasCoat = hasCoat || it.Category == "outerwear"
	}
	if !hasCoat {
		t.Error("a freezing-day outfit should include outerwear")
	}
	if !strings.Contains(withShirt.Note, "layered") {
		t.Errorf("note should say it's layered, got %q", withShirt.Note)
	}
}

func TestSuggestBusinessShirtWorksForCasual(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")

	// Tagged for work only, formality 4: the old filter required both the
	// casual tag and formality within ±1 of 2.
	shirt := createTaggedItem(t, s, userID, "top", "button-down", 4, 2, []string{"summer"}, []string{"work"})
	createTaggedItem(t, s, userID, "bottom", "jeans", 2, 2, []string{"summer"}, []string{"casual"})
	createTaggedItem(t, s, userID, "shoes", "sneakers", 2, 2, []string{"summer"}, []string{"casual"})

	forecast := hotSummerForecast()
	result, err := Suggest(ctx, s, userID, taxonomy.OccasionCasual, &forecast, true, julyDate)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if !anyOutfitHas(result.Outfits, shirt) {
		t.Errorf("expected the business shirt with jeans for casual, got %+v (reason %q)", result.Outfits, result.NoMatchReason)
	}
}

func TestSuggestKeepsOutfitFormalityCoherent(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")

	// Both fit casual's 1-4 range on their own, but 4 with 1 is three
	// steps apart — a blazer-level shirt with joggers.
	createTaggedItem(t, s, userID, "top", "button-down", 4, 2, []string{"summer"}, []string{"casual"})
	createTaggedItem(t, s, userID, "bottom", "joggers", 1, 2, []string{"summer"}, []string{"casual"})

	forecast := hotSummerForecast()
	result, err := Suggest(ctx, s, userID, taxonomy.OccasionCasual, &forecast, true, julyDate)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if len(result.Outfits) != 0 {
		t.Errorf("formality 4 with 1 should never be paired, got %+v", result.Outfits)
	}
}

func TestSuggestNoShortsInWinter(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")

	createTaggedItem(t, s, userID, "top", "knit", 2, 5, []string{"winter"}, []string{"casual"})
	shorts := createTaggedItem(t, s, userID, "bottom", "shorts", 2, 1, []string{"summer"}, []string{"casual"})
	createTaggedItem(t, s, userID, "bottom", "jeans", 2, 4, []string{"winter"}, []string{"casual"})

	forecast := freezingForecast()
	result, err := Suggest(ctx, s, userID, taxonomy.OccasionCasual, &forecast, true, januaryDate)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if len(result.Outfits) == 0 {
		t.Fatalf("expected a winter outfit, got none (reason %q)", result.NoMatchReason)
	}
	if anyOutfitHas(result.Outfits, shorts) {
		t.Error("shorts can't be layered and must not appear on a freezing day")
	}
}

func TestSuggestSkipsOutfitThatCantBeLayeredWarmEnough(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")

	// A tee and jeans, but nothing warm to put over the tee.
	createTaggedItem(t, s, userID, "top", "t-shirt", 2, 1, []string{"summer"}, []string{"casual"})
	createTaggedItem(t, s, userID, "bottom", "jeans", 2, 4, []string{"winter"}, []string{"casual"})

	forecast := freezingForecast()
	result, err := Suggest(ctx, s, userID, taxonomy.OccasionCasual, &forecast, true, januaryDate)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if len(result.Outfits) != 0 {
		t.Fatalf("a lone tee isn't warm enough for freezing, got %+v", result.Outfits)
	}
	if !strings.Contains(result.NoMatchReason, "weather") {
		t.Errorf("expected the reason to point at the weather, got %q", result.NoMatchReason)
	}
}

func TestSuggestPrefersItemsTaggedForTheOccasion(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")

	// Candidates come back newest first, so without the preference the
	// untagged (newer) top would lead.
	tagged := createTaggedItem(t, s, userID, "top", "t-shirt", 2, 2, []string{"summer"}, []string{"casual"})
	createTaggedItem(t, s, userID, "top", "polo", 2, 2, []string{"summer"}, []string{"work"})
	createTaggedItem(t, s, userID, "bottom", "jeans", 2, 2, []string{"summer"}, []string{"casual"})

	forecast := hotSummerForecast()
	result, err := Suggest(ctx, s, userID, taxonomy.OccasionCasual, &forecast, true, julyDate)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if len(result.Outfits) < 2 {
		t.Fatalf("expected both tops to be used, got %d outfits (reason %q)", len(result.Outfits), result.NoMatchReason)
	}
	if !outfitHas(result.Outfits[0], tagged) {
		t.Errorf("first outfit should use the casual-tagged top, got %+v", result.Outfits[0].Items)
	}
}

func TestSuggestAI_DropsOutfitsThatBreakOutfitRules(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")

	tee := createTaggedItem(t, s, userID, "top", "t-shirt", 2, 1, []string{"summer"}, []string{"casual"})
	jeans := createTaggedItem(t, s, userID, "bottom", "jeans", 2, 4, []string{"winter"}, []string{"casual"})
	createTaggedItem(t, s, userID, "outerwear", "coat", 2, 5, []string{"winter"}, []string{"casual"})

	// Every id is a real candidate, but a tee and jeans alone isn't warm
	// enough for freezing — the model doesn't see the weather.
	provider := &fakeAIProvider{suggestions: []ai.AISuggestion{
		{ItemIDs: []int64{tee, jeans}, Rationale: "Simple and clean."},
	}}
	forecast := freezingForecast()
	result, err := SuggestAI(ctx, s, provider, true, 5.00, DailySuggestionCap, userID, taxonomy.OccasionCasual, "", "", &forecast, true, januaryDate)
	if err != nil {
		t.Fatalf("SuggestAI: %v", err)
	}
	if result.Source != SourceRulesFallback {
		t.Errorf("source = %q, want the rules fallback once the only AI outfit is dropped", result.Source)
	}
	target := TargetWarmth(forecast)
	for _, o := range result.Outfits {
		if !outfitWorks(o.Items, &target) {
			t.Errorf("returned an outfit that breaks the rules: %+v", o.Items)
		}
	}
}

// Layers worn together are as warm as the warmest one; bottoms don't
// count towards upper-body warmth.
func TestUpperWarmthCountsLayers(t *testing.T) {
	items := []store.Item{
		{Category: "top", Warmth: 1},
		{Category: "bottom", Warmth: 5},
		{Category: "outerwear", Warmth: 4},
	}
	if got := upperWarmth(items); got != 4 {
		t.Errorf("upperWarmth = %d, want 4 (warmest upper layer; bottoms don't count)", got)
	}
}
