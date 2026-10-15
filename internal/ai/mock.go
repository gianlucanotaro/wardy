package ai

import "context"

// mockProvider returns fixture tags instantly, with no network calls — the
// default in development so iterating on the tagging UI never burns API
// credits.
type mockProvider struct{}

// SuggestOutfits builds fake outfits slot by slot — the i-th top with the
// i-th bottom, plus shoes and outerwear when there are any — so the
// outfits look like outfits on the dev instance. No network, no fashion
// logic beyond that: the rules pass's own checks (internal/suggest) still
// decide whether each one survives, exactly as they would for a real
// model's picks.
func (mockProvider) SuggestOutfits(_ context.Context, candidates []SuggestionCandidate, occasion, _, _ string) ([]AISuggestion, Usage, error) {
	byCategory := make(map[string][]int64)
	for _, c := range candidates {
		byCategory[c.Category] = append(byCategory[c.Category], c.ID)
	}
	nth := func(category string, i int) []int64 {
		ids := byCategory[category]
		if len(ids) == 0 {
			return nil
		}
		return []int64{ids[i%len(ids)]}
	}

	var suggestions []AISuggestion
	for i := 0; i < len(byCategory["top"]) && i < len(byCategory["bottom"]) && len(suggestions) < MaxAISuggestions; i++ {
		ids := append(nth("top", i), nth("bottom", i)...)
		ids = append(ids, nth("shoes", i)...)
		ids = append(ids, nth("outerwear", i)...)
		suggestions = append(suggestions, AISuggestion{
			ItemIDs:   ids,
			Rationale: "Mock pick for " + occasion + " — set AI_PROVIDER=anthropic for real rationale.",
		})
	}
	return suggestions, Usage{Model: "mock"}, nil
}

func (mockProvider) TagItem(_ context.Context, _ []byte) (TagResult, Usage, error) {
	return TagResult{
		Category:     "top",
		Subcategory:  "t-shirt",
		ColorPrimary: "blue",
		Pattern:      "solid",
		Material:     "cotton",
		Fit:          "regular",
		FormalityMin: 1,
		FormalityMax: 3,
		WarmthMin:    1,
		WarmthMax:    5,
		Seasons:      []string{"summer", "spring"},
		Occasions:    []string{"casual"},
		CustomTags:   []string{"wardrobe-basic"},
	}, Usage{Model: "mock"}, nil
}
