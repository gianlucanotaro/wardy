package ai

import "context"

// offProvider is the kill switch: tagging never runs, and every item is
// left for manual tagging.
type offProvider struct{}

func (offProvider) TagItem(_ context.Context, _ []byte) (TagResult, Usage, error) {
	return TagResult{}, Usage{}, ErrDisabled
}

func (offProvider) SuggestOutfits(_ context.Context, _ []SuggestionCandidate, _, _, _ string) ([]AISuggestion, Usage, error) {
	return nil, Usage{}, ErrDisabled
}
