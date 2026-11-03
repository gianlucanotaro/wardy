// Package ai integrates with the vision tagging model (and later, outfit
// suggestions). It returns plain structs and knows nothing about HTTP or
// SQL.
package ai

import (
	"context"
	"errors"
	"fmt"
)

// TagResult is the raw output of vision tagging, before validation against
// internal/taxonomy — a model can hallucinate or invent values, so nothing
// here is trusted until it's been through Tags.Validate().
type TagResult struct {
	Category       string
	Subcategory    string
	ColorPrimary   string
	ColorSecondary string
	Pattern        string
	Material       string
	Fit            string
	FormalityMin   int
	FormalityMax   int
	WarmthMin      int
	WarmthMax      int
	Seasons        []string
	Occasions      []string
	// CustomTags are short, purely observational style/condition/era
	// descriptors the closed vocabulary doesn't capture (e.g. "vintage",
	// "distressed") — never anything sentimental or about origin, which
	// the model has no way to know. Empty is a normal answer.
	CustomTags []string
	// Styles uses the inspo style vocabulary (taxonomy.Styles), at most
	// taxonomy.MaxItemStyles.
	Styles []string
}

// InspoTagResult is the raw output of tagging an inspiration photo (#49)
// — someone else's whole outfit, not one garment. Untrusted until it has
// been through taxonomy.InspoTags.Validate().
type InspoTagResult struct {
	Styles       []string
	Colors       []string
	Pieces       []string
	Fit          string
	FormalityMin int
	FormalityMax int
	Seasons      []string
	Occasions    []string
}

// Usage records what a single model call cost, for the usage table.
type Usage struct {
	Model        string
	InputTokens  int
	OutputTokens int
	EstCostUSD   float64
}

// SuggestionCandidate is the compact, ids-plus-relevant-attributes view of
// an item sent to the suggestion model — no paths, no timestamps (see
// docs/ai.md).
type SuggestionCandidate struct {
	ID int64
	// Name is the user's own name for the item, when they gave one — so a
	// request like "the green jacket" or "my lucky shirt" can find it.
	Name         string
	Category     string
	Subcategory  string
	ColorPrimary string
	FormalityMin int
	FormalityMax int
	WarmthMin    int
	WarmthMax    int
	// CustomTags are the user's own personal tags (sentimental or
	// organizational — "favorite", "gift from mom") — prompt context
	// only, per docs/ai.md's mood/time_of_day pattern: they may shape
	// the model's rationale but never the candidate set itself.
	CustomTags []string
}

// AISuggestion is one outfit as returned by the suggestion model: item ids
// plus a short rationale. Nothing here is trusted until every id has been
// checked against the candidate set the model was given — models
// hallucinate ids (docs/ai.md).
type AISuggestion struct {
	ItemIDs   []int64
	Rationale string
}

// SuggestContext is what the user asked for beyond the occasion: prompt
// context for the suggestion model, never a filter on the candidates.
// Mood is one of suggest.Moods and Prompt is the user's own words (#41),
// both already validated by the caller; empty fields are left out.
//
// Style is the user's style profile from their inspiration board
// (inspo.Profile.Describe plus the cached summary, #49): same rule, it
// steers the picks but never overrides the weather or occasion.
type SuggestContext struct {
	Mood      string
	TimeOfDay string
	Prompt    string
	Style     string
}

// SuggestionModelName identifies the model SuggestOutfits calls, for the
// per-user daily suggestion cap (internal/suggest.SuggestAI queries usage
// logged under this name).
const SuggestionModelName = "claude-sonnet-5"

// MaxAISuggestions caps how many outfits SuggestOutfits asks for and
// returns — docs/ai.md: "ask for 2-3 outfits".
const MaxAISuggestions = 3

// ErrDisabled is returned by the "off" provider — tagging is turned off
// entirely, and callers should leave the item for manual tagging rather
// than retrying.
var ErrDisabled = errors.New("ai tagging is disabled")

// Provider tags item thumbnails and suggests outfits. Implementations:
// anthropic (real API calls), mock (fixture data, no network), off (always
// disabled). Selected by AI_PROVIDER.
type Provider interface {
	TagItem(ctx context.Context, thumbnail []byte) (TagResult, Usage, error)

	// TagItemStyles tags only an item's styles, for items tagged before
	// items had styles. Untrusted until taxonomy.ValidItemStyles.
	TagItemStyles(ctx context.Context, thumbnail []byte) ([]string, Usage, error)

	// SuggestOutfits asks for 2-3 outfits built only from candidates,
	// with sc as prompt context (not filters) shaping the picks and the
	// rationale — see docs/ai.md. Callers must validate every returned id
	// against the candidate set before trusting it.
	SuggestOutfits(ctx context.Context, candidates []SuggestionCandidate, occasion string, sc SuggestContext) ([]AISuggestion, Usage, error)

	// TagInspo tags a photo of a whole outfit for the inspiration board:
	// the clothes only, never the person wearing them.
	TagInspo(ctx context.Context, thumbnail []byte) (InspoTagResult, Usage, error)

	// SummarizeStyle writes a one-sentence description of a style
	// profile (inspo.Profile.Describe) — from the counted tags, never
	// the photos.
	SummarizeStyle(ctx context.Context, profile string) (string, Usage, error)
}

// New builds the Provider selected by name. apiKey is only required for
// "anthropic".
func New(provider, apiKey string) (Provider, error) {
	switch provider {
	case "anthropic":
		if apiKey == "" {
			return nil, errors.New("ANTHROPIC_API_KEY is required when AI_PROVIDER=anthropic")
		}
		return newAnthropicProvider(apiKey), nil
	case "mock":
		return mockProvider{}, nil
	case "off", "":
		return offProvider{}, nil
	default:
		return nil, fmt.Errorf("unknown AI_PROVIDER %q", provider)
	}
}
