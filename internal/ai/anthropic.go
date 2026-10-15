package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

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

const (
	suggestionModel     = anthropic.ModelClaudeSonnet5
	suggestionMaxTokens = 600

	// Sonnet 5 list pricing — used only to produce an estimated cost for
	// the usage table, not for billing.
	suggestionInputPricePerMTokens  = 3.0
	suggestionOutputPricePerMTokens = 15.0

	// maxSuggestAttempts mirrors the tagging job's retry policy (docs/ai.md:
	// "3 retries max"). Unlike tagging, a suggestion request runs inline in
	// an HTTP request rather than through the job queue, so the retries
	// happen in a tight loop here rather than across ProcessOne polls.
	maxSuggestAttempts  = 3
	suggestRetryBackoff = 250 * time.Millisecond
)

// suggestionSystemPrompt is generated once at package init and reused
// byte-for-byte on every call.
var suggestionSystemPrompt = `You suggest outfits for a wardrobe app from a fixed list of candidate items that have already been matched to today's weather and occasion.

Respond with ONLY a JSON array — no markdown fences, no prose before or after:

[
  {"item_ids": [1, 2, 3], "rationale": "one sentence explaining why this outfit works"},
  ...
]

Return 2 to 3 outfits. Use ONLY item ids from the candidate list you are given — never invent an id, and never include an item not in the list. Each outfit should combine items that work together (e.g. a top and bottom, or a dress, plus shoes and outerwear where appropriate), keeping its pieces within about two formality steps of each other. Mood and time of day, when given, should shape the rationale and which outfits you pick — they are not additional filters. Some candidates carry custom_tags: the user's own personal tags (sentimental or organizational, like "favorite" or "needs mending") — use them as context for the rationale when relevant, never as a reason to include or exclude an item, since the candidate list is already final.`

type suggestionCandidateJSON struct {
	ID           int64    `json:"id"`
	Category     string   `json:"category"`
	Subcategory  string   `json:"subcategory"`
	ColorPrimary string   `json:"color_primary"`
	FormalityMin int      `json:"formality_min"`
	FormalityMax int      `json:"formality_max"`
	WarmthMin    int      `json:"warmth_min"`
	WarmthMax    int      `json:"warmth_max"`
	CustomTags   []string `json:"custom_tags,omitempty"`
}

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
  "formality_min", "formality_max": integers 1 (loungewear) to 5 (black tie) — the least and
    most formal settings the piece can be worn in, e.g. a plain white shirt 2 to 4, a tuxedo
    jacket 5 to 5,
  "warmth_min", "warmth_max": integers 1 (hot weather) to 5 (deep winter) — the weather the
    piece can be worn in, counting layering: a light shirt that goes under a knit in winter is
    1 to 5, a wool coat 4 to 5, shorts 1 to 2,
  "seasons": array, any of %s,
  "occasions": array, any of %s,
  "custom_tags": array, 0 to 3 short style/condition/era descriptors not covered by the fields
    above — things you can actually see, like "vintage", "distressed", "logo-print". Never guess
    at sentiment, ownership, or origin (never something like "gift" or "college era") — you have
    no way to know that from a photo. An empty array is a completely normal answer.
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

// SuggestOutfits asks claude-sonnet-5 for 2-3 outfits built from
// candidates. Retries up to maxSuggestAttempts times with exponential
// backoff, never retrying a non-retryable (4xx) error — see IsRetryable.
func (p *anthropicProvider) SuggestOutfits(ctx context.Context, candidates []SuggestionCandidate, occasion, mood, timeOfDay string) ([]AISuggestion, Usage, error) {
	userText, err := buildSuggestionPrompt(candidates, occasion, mood, timeOfDay)
	if err != nil {
		return nil, Usage{}, err
	}

	params := anthropic.MessageNewParams{
		Model:     suggestionModel,
		MaxTokens: suggestionMaxTokens,
		System: []anthropic.TextBlockParam{
			{Text: suggestionSystemPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userText)),
		},
	}

	var resp *anthropic.Message
	var lastErr error
	for attempt := 1; attempt <= maxSuggestAttempts; attempt++ {
		resp, lastErr = p.client.Messages.New(ctx, params)
		if lastErr == nil {
			break
		}
		if !IsRetryable(lastErr) || attempt == maxSuggestAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return nil, Usage{}, ctx.Err()
		case <-time.After(time.Duration(attempt) * suggestRetryBackoff):
		}
	}
	if lastErr != nil {
		return nil, Usage{}, fmt.Errorf("anthropic: %w", lastErr)
	}

	usage := Usage{
		Model:        string(suggestionModel),
		InputTokens:  int(resp.Usage.InputTokens),
		OutputTokens: int(resp.Usage.OutputTokens),
		EstCostUSD:   estimateCostAt(resp.Usage.InputTokens, resp.Usage.OutputTokens, suggestionInputPricePerMTokens, suggestionOutputPricePerMTokens),
	}

	var text strings.Builder
	for _, block := range resp.Content {
		text.WriteString(block.Text)
	}

	suggestions, err := parseSuggestResponse(text.String())
	if err != nil {
		return nil, usage, err
	}
	return suggestions, usage, nil
}

func buildSuggestionPrompt(candidates []SuggestionCandidate, occasion, mood, timeOfDay string) (string, error) {
	payload := make([]suggestionCandidateJSON, len(candidates))
	for i, c := range candidates {
		payload[i] = suggestionCandidateJSON{
			ID:           c.ID,
			Category:     c.Category,
			Subcategory:  c.Subcategory,
			ColorPrimary: c.ColorPrimary,
			FormalityMin: c.FormalityMin,
			FormalityMax: c.FormalityMax,
			WarmthMin:    c.WarmthMin,
			WarmthMax:    c.WarmthMax,
			CustomTags:   c.CustomTags,
		}
	}
	candidateJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal candidates: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Occasion: %s\n", occasion)
	if mood != "" {
		fmt.Fprintf(&b, "Mood: %s\n", mood)
	}
	if timeOfDay != "" {
		fmt.Fprintf(&b, "Time of day: %s\n", timeOfDay)
	}
	fmt.Fprintf(&b, "Candidates: %s", candidateJSON)
	return b.String(), nil
}

// parseSuggestResponse parses a model's raw text response into outfit
// suggestions. Kept separate from SuggestOutfits so it can be exercised
// against recorded fixtures in tests without any live API calls.
func parseSuggestResponse(text string) ([]AISuggestion, error) {
	text = stripMarkdownFences(text)

	var raw []struct {
		ItemIDs   []int64 `json:"item_ids"`
		Rationale string  `json:"rationale"`
	}
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return nil, fmt.Errorf("parse model response: %w", err)
	}

	suggestions := make([]AISuggestion, 0, len(raw))
	for _, r := range raw {
		suggestions = append(suggestions, AISuggestion{ItemIDs: r.ItemIDs, Rationale: r.Rationale})
	}
	return suggestions, nil
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
		FormalityMin   int      `json:"formality_min"`
		FormalityMax   int      `json:"formality_max"`
		WarmthMin      int      `json:"warmth_min"`
		WarmthMax      int      `json:"warmth_max"`
		Seasons        []string `json:"seasons"`
		Occasions      []string `json:"occasions"`
		// Optional — an older or malformed response omitting it is not a
		// parse error, it just yields no custom tags.
		CustomTags []string `json:"custom_tags"`
	}
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return TagResult{}, fmt.Errorf("parse model response: %w", err)
	}

	// A model that lists a range high-to-low still meant that range, so
	// order it rather than failing the whole tag; tags.Validate still
	// checks both bounds are on the 1-5 scale.
	return TagResult{
		Category:       raw.Category,
		Subcategory:    raw.Subcategory,
		ColorPrimary:   raw.ColorPrimary,
		ColorSecondary: raw.ColorSecondary,
		Pattern:        raw.Pattern,
		Material:       raw.Material,
		Fit:            raw.Fit,
		FormalityMin:   min(raw.FormalityMin, raw.FormalityMax),
		FormalityMax:   max(raw.FormalityMin, raw.FormalityMax),
		WarmthMin:      min(raw.WarmthMin, raw.WarmthMax),
		WarmthMax:      max(raw.WarmthMin, raw.WarmthMax),
		Seasons:        raw.Seasons,
		Occasions:      raw.Occasions,
		CustomTags:     raw.CustomTags,
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
	return estimateCostAt(inputTokens, outputTokens, inputPricePerMTokens, outputPricePerMTokens)
}

func estimateCostAt(inputTokens, outputTokens int64, inputPrice, outputPrice float64) float64 {
	return float64(inputTokens)/1_000_000*inputPrice +
		float64(outputTokens)/1_000_000*outputPrice
}
