package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/gianlucanotaro/wardy/internal/taxonomy"
)

const (
	taggingModel     = anthropic.ModelClaudeHaiku4_5_20251001
	taggingMaxTokens = 400

	// Haiku 4.5 pricing per docs/ai.md — used only to produce an estimated
	// cost for the usage table, not for billing.
	inputPricePerMTokens  = 1.0
	outputPricePerMTokens = 5.0
)

// taggingSystemPrompt is generated from internal/taxonomy rather than
// hand-typed, so the vocabulary the model is told about can never drift
// out of sync with what Tags.Validate() actually accepts — every field
// gets its real closed list, not a description of one. It's computed once
// at package init, so it's still byte-identical on every call (required
// for the prompt-caching breakpoint below).
var taggingSystemPrompt = buildTaggingSystemPrompt()

func buildTaggingSystemPrompt() string {
	var subcats strings.Builder
	for _, cat := range taxonomy.Categories {
		fmt.Fprintf(&subcats, "%s: %s\n", cat, strings.Join(taxonomy.Subcategories[cat], ", "))
	}

	return fmt.Sprintf(`You tag a single clothing item photo with structured attributes for a wardrobe app.
Respond with ONLY a JSON object — no markdown fences, no prose before or after. Every field must be
exactly one of its listed values below — never invent a new one, even if it seems more descriptive.

{
  "category": one of %s,
  "subcategory": one of the values listed below for the chosen category,
  "color_primary": one of %s,
  "color_secondary": one of the same list, or "" if there isn't one,
  "pattern": one of %s,
  "material": one of %s,
  "fit": one of %s,
  "formality": integer 1 (loungewear) to 5 (black tie),
  "warmth": integer 1 (hot weather only) to 5 (deep winter),
  "seasons": array, any of %s,
  "occasions": array, any of %s
}

Valid subcategory values per category:
%s
If you can't tell the material from the photo, use "unknown" rather than guessing.`,
		joinEnum(taxonomy.Categories, "|"),
		strings.Join(taxonomy.Colors, "|"),
		joinEnum(taxonomy.Patterns, "|"),
		joinEnum(taxonomy.Materials, "|"),
		joinEnum(taxonomy.Fits, "|"),
		joinEnum(taxonomy.Seasons, "|"),
		joinEnum(taxonomy.Occasions, "|"),
		subcats.String(),
	)
}

// joinEnum joins a slice of any taxonomy string-based enum type — Go can't
// pass []taxonomy.Category where []string is expected, even though the
// underlying type is string.
func joinEnum[T ~string](values []T, sep string) string {
	strs := make([]string, len(values))
	for i, v := range values {
		strs[i] = string(v)
	}
	return strings.Join(strs, sep)
}

// anthropicProvider calls claude-haiku-4-5 for vision tagging. Classification
// is its sweet spot and it's the cheapest current model — see docs/ai.md.
type anthropicProvider struct {
	client anthropic.Client
}

func newAnthropicProvider(apiKey string) *anthropicProvider {
	return &anthropicProvider{
		client: anthropic.NewClient(option.WithAPIKey(apiKey)),
	}
}

func (p *anthropicProvider) TagItem(ctx context.Context, thumbnail []byte) (TagResult, Usage, error) {
	encoded := base64.StdEncoding.EncodeToString(thumbnail)

	resp, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     taggingModel,
		MaxTokens: taggingMaxTokens,
		System: []anthropic.TextBlockParam{
			{
				// Identical on every call, so this is the prompt-caching
				// breakpoint docs/ai.md asks for.
				Text:         taggingSystemPrompt,
				CacheControl: anthropic.NewCacheControlEphemeralParam(),
			},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(
				anthropic.NewImageBlockBase64("image/jpeg", encoded),
				anthropic.NewTextBlock("Tag this clothing item."),
			),
		},
	})
	if err != nil {
		return TagResult{}, Usage{}, fmt.Errorf("anthropic: %w", err)
	}

	usage := Usage{
		Model:        string(taggingModel),
		InputTokens:  int(resp.Usage.InputTokens),
		OutputTokens: int(resp.Usage.OutputTokens),
		EstCostUSD:   estimateCost(resp.Usage.InputTokens, resp.Usage.OutputTokens),
	}

	var text strings.Builder
	for _, block := range resp.Content {
		text.WriteString(block.Text)
	}

	result, err := parseTagResponse(text.String())
	if err != nil {
		return TagResult{}, usage, err
	}
	return result, usage, nil
}

// parseTagResponse parses a model's raw text response into a TagResult.
// Kept separate from TagItem so it can be exercised against recorded
// fixtures in tests without any live API calls.
func parseTagResponse(text string) (TagResult, error) {
	text = stripMarkdownFences(text)

	var raw struct {
		Category       string   `json:"category"`
		Subcategory    string   `json:"subcategory"`
		ColorPrimary   string   `json:"color_primary"`
		ColorSecondary string   `json:"color_secondary"`
		Pattern        string   `json:"pattern"`
		Material       string   `json:"material"`
		Fit            string   `json:"fit"`
		Formality      int      `json:"formality"`
		Warmth         int      `json:"warmth"`
		Seasons        []string `json:"seasons"`
		Occasions      []string `json:"occasions"`
	}
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return TagResult{}, fmt.Errorf("parse model response: %w", err)
	}

	return TagResult{
		Category:       raw.Category,
		Subcategory:    raw.Subcategory,
		ColorPrimary:   raw.ColorPrimary,
		ColorSecondary: raw.ColorSecondary,
		Pattern:        raw.Pattern,
		Material:       raw.Material,
		Fit:            raw.Fit,
		Formality:      raw.Formality,
		Warmth:         raw.Warmth,
		Seasons:        raw.Seasons,
		Occasions:      raw.Occasions,
	}, nil
}

func stripMarkdownFences(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

func estimateCost(inputTokens, outputTokens int64) float64 {
	return float64(inputTokens)/1_000_000*inputPricePerMTokens +
		float64(outputTokens)/1_000_000*outputPricePerMTokens
}
