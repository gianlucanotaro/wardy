package ai

import "context"

// Mock returns fixture tags and outfits with no network calls.
// Use AI_PROVIDER=mock in development to avoid burning API credits.
type Mock struct{}

func (Mock) TagItem(_ context.Context, _ []byte) (*Tags, error) {
	return &Tags{
		Category:     "top",
		Subcategory:  "t-shirt",
		ColorPrimary: "white",
		Pattern:      "solid",
		Material:     "cotton",
		Formality:    2,
		Warmth:       2,
		Fit:          "regular",
		Seasons:      []string{"spring", "summer"},
		Occasions:    []string{"casual"},
	}, nil
}

func (Mock) SuggestOutfits(_ context.Context, req SuggestionRequest) ([]Outfit, error) {
	if len(req.Candidates) == 0 {
		return nil, nil
	}
	var ids []int64
	for i, c := range req.Candidates {
		if i >= 3 {
			break
		}
		ids = append(ids, c.ID)
	}
	return []Outfit{{
		ItemIDs:   ids,
		Rationale: "A simple casual combination from your wardrobe.",
	}}, nil
}
