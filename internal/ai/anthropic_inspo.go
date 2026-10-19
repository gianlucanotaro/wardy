package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/gianlucanotaro/wardy/internal/taxonomy"
)

const (
	inspoTaggingMaxTokens = 400
	styleSummaryMaxTokens = 120
)

// inspoTaggingSystemPrompt, like taggingSystemPrompt, is generated from
// internal/taxonomy so the vocabulary can't drift from what
// InspoTags.Validate accepts, and is byte-identical on every call for
// prompt caching.
var inspoTaggingSystemPrompt = buildInspoTaggingSystemPrompt()

func buildInspoTaggingSystemPrompt() string {
	var pieces []string
	for _, cat := range taxonomy.Categories {
		pieces = append(pieces, taxonomy.Subcategories[cat]...)
	}

	return fmt.Sprintf(`You tag a photo of an outfit someone is wearing, for a wardrobe app where the user collects outfits they like as inspiration.
Describe ONLY the clothes and how they are put together. Never describe, identify, guess anything about, or comment on the person — not their body, face, age, gender, ethnicity, or who they might be.

Respond with ONLY a JSON object — no markdown fences, no prose before or after. Every value must be exactly one of its listed values — never invent a new one.

{
  "styles": array of 1 to %d, the overall style, from %s,
  "colors": array of up to %d, the main colours of the outfit, most prominent first, from %s,
  "pieces": array of up to %d, the visible garments, shoes, bags and accessories, from %s,
  "fit": the overall silhouette, one of %s,
  "formality_min", "formality_max": integers 1 (loungewear) to 5 (black tie) — the range of settings this outfit suits,
  "seasons": array, any of %s,
  "occasions": array, any of %s
}

If the photo shows no outfit you can see, respond with empty arrays, "fit": "" and both formality values 0.`,
		taxonomy.MaxInspoStyles, joinEnum(taxonomy.Styles, "|"),
		taxonomy.MaxInspoColors, strings.Join(taxonomy.Colors, "|"),
		taxonomy.MaxInspoPieces, strings.Join(pieces, "|"),
		joinEnum(taxonomy.Fits, "|"),
		joinEnum(taxonomy.Seasons, "|"),
		joinEnum(taxonomy.Occasions, "|"),
	)
}

const styleSummarySystemPrompt = `You describe a person's clothing style for a wardrobe app, from counts of tags on outfit photos they saved as inspiration.

Write ONE plain sentence of at most 20 words describing the style — its feel, colours, shapes and typical pieces — addressed to nobody, like "Clean, muted and relaxed — neutral bases, one earthy layer, wide trousers, minimal shoes." No quotes, no preamble, no percentages, no mention of photos or tags. Never describe the people in the photos.`

// TagInspo tags an inspiration photo with claude-haiku-4-5, as TagItem
// does an item.
func (p *anthropicProvider) TagInspo(ctx context.Context, thumbnail []byte) (InspoTagResult, Usage, error) {
	resp, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     taggingModel,
		MaxTokens: inspoTaggingMaxTokens,
		System: []anthropic.TextBlockParam{
			{Text: inspoTaggingSystemPrompt, CacheControl: anthropic.NewCacheControlEphemeralParam()},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(
				anthropic.NewImageBlockBase64("image/jpeg", base64.StdEncoding.EncodeToString(thumbnail)),
				anthropic.NewTextBlock("Tag this outfit."),
			),
		},
	})
	if err != nil {
		return InspoTagResult{}, Usage{}, fmt.Errorf("anthropic: %w", err)
	}

	usage := Usage{
		Model:        string(taggingModel),
		InputTokens:  int(resp.Usage.InputTokens),
		OutputTokens: int(resp.Usage.OutputTokens),
		EstCostUSD:   estimateCost(resp.Usage.InputTokens, resp.Usage.OutputTokens),
	}
	result, err := parseInspoTagResponse(responseText(resp))
	return result, usage, err
}

// SummarizeStyle writes the one-line style summary with claude-haiku-4-5
// — a short text task, no reason for a bigger model.
func (p *anthropicProvider) SummarizeStyle(ctx context.Context, profile string) (string, Usage, error) {
	resp, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     taggingModel,
		MaxTokens: styleSummaryMaxTokens,
		System:    []anthropic.TextBlockParam{{Text: styleSummarySystemPrompt}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(profile)),
		},
	})
	if err != nil {
		return "", Usage{}, fmt.Errorf("anthropic: %w", err)
	}

	usage := Usage{
		Model:        string(taggingModel),
		InputTokens:  int(resp.Usage.InputTokens),
		OutputTokens: int(resp.Usage.OutputTokens),
		EstCostUSD:   estimateCost(resp.Usage.InputTokens, resp.Usage.OutputTokens),
	}
	summary, err := cleanStyleSummary(responseText(resp))
	return summary, usage, err
}

func responseText(resp *anthropic.Message) string {
	var text strings.Builder
	for _, block := range resp.Content {
		text.WriteString(block.Text)
	}
	return text.String()
}

// maxStyleSummaryLength caps what's stored and shown, whatever the model
// writes.
const maxStyleSummaryLength = 200

// cleanStyleSummary trims the model's sentence to one line within
// maxStyleSummaryLength, and rejects an empty answer.
func cleanStyleSummary(text string) (string, error) {
	text = strings.Join(strings.Fields(strings.Trim(strings.TrimSpace(text), `"`)), " ")
	if text == "" {
		return "", fmt.Errorf("parse model response: empty style summary")
	}
	if r := []rune(text); len(r) > maxStyleSummaryLength {
		text = strings.TrimSpace(string(r[:maxStyleSummaryLength-1])) + "…"
	}
	return text, nil
}

// parseInspoTagResponse parses a model's raw text into an InspoTagResult,
// separate from TagInspo so it can be tested against recorded fixtures.
func parseInspoTagResponse(text string) (InspoTagResult, error) {
	var raw struct {
		Styles       []string `json:"styles"`
		Colors       []string `json:"colors"`
		Pieces       []string `json:"pieces"`
		Fit          string   `json:"fit"`
		FormalityMin int      `json:"formality_min"`
		FormalityMax int      `json:"formality_max"`
		Seasons      []string `json:"seasons"`
		Occasions    []string `json:"occasions"`
	}
	if err := json.Unmarshal([]byte(stripMarkdownFences(text)), &raw); err != nil {
		return InspoTagResult{}, fmt.Errorf("parse model response: %w", err)
	}
	// Too many values is still a usable answer: keep the first ones (the
	// prompt asks for most prominent first). Out-of-vocabulary values are
	// left for Validate to reject.
	return InspoTagResult{
		Styles:       firstUnique(raw.Styles, taxonomy.MaxInspoStyles),
		Colors:       firstUnique(raw.Colors, taxonomy.MaxInspoColors),
		Pieces:       firstUnique(raw.Pieces, taxonomy.MaxInspoPieces),
		Fit:          raw.Fit,
		FormalityMin: min(raw.FormalityMin, raw.FormalityMax),
		FormalityMax: max(raw.FormalityMin, raw.FormalityMax),
		Seasons:      firstUnique(raw.Seasons, len(taxonomy.Seasons)),
		Occasions:    firstUnique(raw.Occasions, len(taxonomy.Occasions)),
	}, nil
}

// firstUnique drops repeats and keeps at most n values, in order.
func firstUnique(values []string, n int) []string {
	out := make([]string, 0, min(len(values), n))
	for _, v := range values {
		if len(out) == n {
			break
		}
		if !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}
