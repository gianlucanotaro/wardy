package ai

import "context"

// mockProvider returns fixture tags instantly, with no network calls — the
// default in development so iterating on the tagging UI never burns API
// credits.
type mockProvider struct{}

// SuggestOutfits pairs up consecutive candidates into fake outfits — no
// network, no fashion logic, just deterministic ids drawn from the
// candidate set so the pipeline (validation, fallback, UI) can be
// exercised in development without burning API credits.
func (mockProvider) SuggestOutfits(_ context.Context, candidates []SuggestionCandidate, occasion, _, _ string) ([]AISuggestion, Usage, error) {
	var suggestions []AISuggestion
	for i := 0; i < len(candidates) && len(suggestions) < MaxAISuggestions; i += 2 {
		ids := []int64{candidates[i].ID}
		if i+1 < len(candidates) {
			ids = append(ids, candidates[i+1].ID)
		}
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
		Formality:    2,
		Warmth:       3,
		Seasons:      []string{"summer", "spring"},
		Occasions:    []string{"casual"},
		CustomTags:   []string{"wardrobe-basic"},
	}, Usage{Model: "mock"}, nil
}
