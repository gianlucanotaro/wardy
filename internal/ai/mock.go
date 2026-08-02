package ai

import "context"

// mockProvider returns fixture tags instantly, with no network calls — the
// default in development so iterating on the tagging UI never burns API
// credits.
type mockProvider struct{}

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
	}, Usage{Model: "mock"}, nil
}
