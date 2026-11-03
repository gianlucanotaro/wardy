package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/gianlucanotaro/wardy/internal/taxonomy"
)

const itemStylesMaxTokens = 60

// itemStylesSystemPrompt asks only for an item's styles, for the backfill
// of items tagged before items had styles: a fraction of a full tagging
// call. Generated from internal/taxonomy like the other prompts, and
// identical on every call for prompt caching.
var itemStylesSystemPrompt = fmt.Sprintf(`You tag the style of a single clothing item photo for a wardrobe app.
Respond with ONLY a JSON object — no markdown fences, no prose before or after:

{"styles": array of 1 to %d, the styles this piece belongs to, most fitting first, from %s}

Use only the listed values — never invent a new one. A plain basic can belong to several styles; pick the ones it most clearly signals.`,
	taxonomy.MaxItemStyles, joinEnum(taxonomy.Styles, "|"))

// TagItemStyles tags only the styles of an item photo.
func (p *anthropicProvider) TagItemStyles(ctx context.Context, thumbnail []byte) ([]string, Usage, error) {
	resp, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     taggingModel,
		MaxTokens: itemStylesMaxTokens,
		System: []anthropic.TextBlockParam{
			{Text: itemStylesSystemPrompt, CacheControl: anthropic.NewCacheControlEphemeralParam()},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(
				anthropic.NewImageBlockBase64("image/jpeg", base64.StdEncoding.EncodeToString(thumbnail)),
				anthropic.NewTextBlock("Tag the style of this clothing item."),
			),
		},
	})
	if err != nil {
		return nil, Usage{}, fmt.Errorf("anthropic: %w", err)
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
	styles, err := parseItemStylesResponse(text.String())
	return styles, usage, err
}

// parseItemStylesResponse parses TagItemStyles' raw text. Kept separate so
// tests can run it against recorded fixtures without any API calls.
func parseItemStylesResponse(text string) ([]string, error) {
	var raw struct {
		Styles []string `json:"styles"`
	}
	if err := json.Unmarshal([]byte(stripMarkdownFences(text)), &raw); err != nil {
		return nil, fmt.Errorf("parse model response: %w", err)
	}
	return raw.Styles, nil
}
