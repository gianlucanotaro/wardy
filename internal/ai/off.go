package ai

import "context"

// offProvider is the kill switch: tagging never runs, and every item is
// left for manual tagging.
type offProvider struct{}

func (offProvider) TagItem(_ context.Context, _ []byte) (TagResult, Usage, error) {
	return TagResult{}, Usage{}, ErrDisabled
}

func (offProvider) SuggestOutfits(_ context.Context, _ []SuggestionCandidate, _ string, _ SuggestContext) ([]AISuggestion, Usage, error) {
	return nil, Usage{}, ErrDisabled
}

func (offProvider) TagInspo(_ context.Context, _ []byte) (InspoTagResult, Usage, error) {
	return InspoTagResult{}, Usage{}, ErrDisabled
}

func (offProvider) SummarizeStyle(_ context.Context, _ string) (string, Usage, error) {
	return "", Usage{}, ErrDisabled
}
