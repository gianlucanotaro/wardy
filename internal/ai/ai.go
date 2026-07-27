package ai

import "context"

// Tags holds the structured attributes extracted from a garment photo.
type Tags struct {
	Category       string   `json:"category"`
	Subcategory    string   `json:"subcategory"`
	ColorPrimary   string   `json:"color_primary"`
	ColorSecondary string   `json:"color_secondary"`
	Pattern        string   `json:"pattern"`
	Material       string   `json:"material"`
	Formality      int      `json:"formality"`
	Warmth         int      `json:"warmth"`
	Fit            string   `json:"fit"`
	Seasons        []string `json:"seasons"`
	Occasions      []string `json:"occasions"`
}

// OutfitItem is a lightweight item representation sent to the LLM.
type OutfitItem struct {
	ID        int64    `json:"id"`
	Category  string   `json:"category"`
	Formality int      `json:"formality"`
	Warmth    int      `json:"warmth"`
	Seasons   []string `json:"seasons"`
	Occasions []string `json:"occasions"`
}

// Outfit is a suggested combination of items.
type Outfit struct {
	ItemIDs   []int64 `json:"item_ids"`
	Rationale string  `json:"rationale"`
}

// SuggestionRequest carries context for the outfit suggestion call.
type SuggestionRequest struct {
	Candidates  []OutfitItem
	Occasion    string
	Mood        string
	TimeOfDay   string
	WeatherNote string
}

// Provider is the interface all AI backends implement.
type Provider interface {
	// TagItem runs vision tagging on the given thumbnail bytes and returns structured tags.
	TagItem(ctx context.Context, thumbJPEG []byte) (*Tags, error)

	// SuggestOutfits returns 2-3 outfit suggestions from the pre-filtered candidate set.
	SuggestOutfits(ctx context.Context, req SuggestionRequest) ([]Outfit, error)
}
