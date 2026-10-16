package ai

import (
	"context"
	"slices"
	"strings"
	"unicode"
)

// mockProvider returns fixture tags instantly, with no network calls — the
// default in development so iterating on the tagging UI never burns API
// credits.
type mockProvider struct{}

// SuggestOutfits builds fake outfits slot by slot — the i-th top with the
// i-th bottom, plus shoes, outerwear and an accessory when there are any — so the
// outfits look like outfits on the dev instance. A request (sc.Prompt) is
// honoured crudely so it can be tried on dev: candidates whose name, type
// or colour share a word with it go first in their slot, and the rationale
// says which ones it found. No network, no fashion logic beyond that: the
// rules pass's own checks (internal/suggest) still decide whether each one
// survives, exactly as they would for a real model's picks.
func (mockProvider) SuggestOutfits(_ context.Context, candidates []SuggestionCandidate, occasion string, sc SuggestContext) ([]AISuggestion, Usage, error) {
	words := promptWords(sc.Prompt)
	matches := func(c SuggestionCandidate) bool {
		for w := range promptWords(c.Name + " " + c.Subcategory + " " + c.ColorPrimary) {
			if words[w] {
				return true
			}
		}
		return false
	}

	byCategory := make(map[string][]SuggestionCandidate)
	for _, c := range candidates {
		byCategory[c.Category] = append(byCategory[c.Category], c)
	}
	var found []string
	for category, cs := range byCategory {
		slices.SortStableFunc(cs, func(a, b SuggestionCandidate) int {
			ma, mb := matches(a), matches(b)
			switch {
			case ma && !mb:
				return -1
			case mb && !ma:
				return 1
			}
			return 0
		})
		if len(cs) > 0 && matches(cs[0]) && slices.Contains(mockSlots, category) {
			found = append(found, mockLabel(cs[0]))
		}
		byCategory[category] = cs
	}
	slices.Sort(found)
	nth := func(category string, i int) []int64 {
		cs := byCategory[category]
		if len(cs) == 0 {
			return nil
		}
		return []int64{cs[i%len(cs)].ID}
	}

	rationale := "Mock pick for " + occasion + " — set AI_PROVIDER=anthropic for real rationale."
	switch {
	case len(found) > 0:
		rationale = "Mock pick for " + occasion + ": went with the " + strings.Join(found, ", ") + " from your request (\"" + sc.Prompt + "\")."
	case sc.Prompt != "":
		rationale = "Mock pick for " + occasion + ": nothing matched the words in \"" + sc.Prompt + "\" — the real AI reads it properly."
	}

	var suggestions []AISuggestion
	for i := 0; i < len(byCategory["top"]) && i < len(byCategory["bottom"]) && len(suggestions) < MaxAISuggestions; i++ {
		var ids []int64
		for _, category := range mockSlots {
			ids = append(ids, nth(category, i)...)
		}
		suggestions = append(suggestions, AISuggestion{
			ItemIDs:   ids,
			Rationale: rationale,
		})
	}
	return suggestions, Usage{Model: "mock"}, nil
}

// mockSlots are the categories the mock dresses an outfit from, in order.
var mockSlots = []string{"top", "bottom", "shoes", "outerwear", "accessory"}

// promptWords lower-cases text and splits it into words of three or more
// letters, so "a", "in" and "my" don't match everything.
func promptWords(text string) map[string]bool {
	words := make(map[string]bool)
	for _, w := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) }) {
		if len([]rune(w)) >= 3 {
			words[w] = true
		}
	}
	return words
}

func mockLabel(c SuggestionCandidate) string {
	if c.Name != "" {
		return strings.ToLower(c.Name)
	}
	return strings.TrimSpace(c.ColorPrimary + " " + c.Subcategory)
}

func (mockProvider) TagItem(_ context.Context, _ []byte) (TagResult, Usage, error) {
	return TagResult{
		Category:     "top",
		Subcategory:  "t-shirt",
		ColorPrimary: "blue",
		Pattern:      "solid",
		Material:     "cotton",
		Fit:          "regular",
		FormalityMin: 1,
		FormalityMax: 3,
		WarmthMin:    1,
		WarmthMax:    5,
		Seasons:      []string{"summer", "spring"},
		Occasions:    []string{"casual"},
		CustomTags:   []string{"wardrobe-basic"},
	}, Usage{Model: "mock"}, nil
}
