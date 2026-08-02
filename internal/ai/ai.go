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
	Formality      int
	Warmth         int
	Seasons        []string
	Occasions      []string
}

// Usage records what a single model call cost, for the usage table.
type Usage struct {
	Model        string
	InputTokens  int
	OutputTokens int
	EstCostUSD   float64
}

// ErrDisabled is returned by the "off" provider — tagging is turned off
// entirely, and callers should leave the item for manual tagging rather
// than retrying.
var ErrDisabled = errors.New("ai tagging is disabled")

// Provider tags a single item's thumbnail. Implementations: anthropic
// (real API calls), mock (fixture data, no network), off (always
// disabled). Selected by AI_PROVIDER.
type Provider interface {
	TagItem(ctx context.Context, thumbnail []byte) (TagResult, Usage, error)
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
