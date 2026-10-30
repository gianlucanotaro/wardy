package suggest

import (
	"errors"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gianlucanotaro/wardy/internal/store"
)

// MaxPromptLength caps the free-text request sent with a suggestion, in
// characters. Mirrored in web/src/Build.tsx's PROMPT_MAX.
const MaxPromptLength = 280

// CleanPrompt tidies the user's free-text request before it goes into the
// AI prompt: control characters (newlines included) become spaces, runs of
// whitespace collapse to one, and the ends are trimmed. Empty means "no
// request". It rejects invalid UTF-8 and anything longer than
// MaxPromptLength rather than silently cutting the user's words short.
func CleanPrompt(raw string) (string, error) {
	if !utf8.ValidString(raw) {
		return "", errors.New("prompt is not valid text")
	}
	cleaned := strings.Join(strings.FieldsFunc(raw, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r)
	}), " ")
	if utf8.RuneCountInString(cleaned) > MaxPromptLength {
		return "", errors.New("prompt is too long")
	}
	return cleaned, nil
}

// preferRequested moves the items the user's request names to the front:
// by colour, category, subcategory or their own name. "A red top" ranks red
// tops first, then other red pieces and other tops. Like preferTagged it
// only ranks, it never filters (docs/ai.md) — but with more than
// MaxCandidates items, the round-robin sample keeps each category's first
// items, so without this the one piece the user asked for could be cut
// before the model ever sees it. Stable, so ties keep their order.
func preferRequested(items []store.Item, prompt string) {
	words := requestWords(prompt)
	if len(words) == 0 {
		return
	}
	score := func(it store.Item) int {
		n := 0
		for _, attr := range []string{it.ColorPrimary, it.ColorSecondary, it.Category, it.Subcategory} {
			if attr != "" && mentionsAll(words, attr) {
				n++
			}
		}
		for _, w := range splitWords(it.Name) {
			if len(w) >= 3 && !stopWords[w] && words[singular(w)] {
				n++
				break
			}
		}
		return n
	}
	sort.SliceStable(items, func(i, j int) bool { return score(items[i]) > score(items[j]) })
}

// stopWords are skipped when matching item names, so "the" in a request
// doesn't pull up "The good jeans".
var stopWords = map[string]bool{"the": true, "and": true, "for": true, "with": true, "my": true, "your": true}

// requestWords is the set of lowercase words in the request, singular.
func requestWords(prompt string) map[string]bool {
	words := map[string]bool{}
	for _, w := range splitWords(prompt) {
		words[singular(w)] = true
	}
	return words
}

// mentionsAll reports whether every word of a tag value is in the request,
// so "t-shirt" needs both "t" and "shirt" and "dress-shoes" doesn't match
// a request for a dress.
func mentionsAll(words map[string]bool, value string) bool {
	parts := splitWords(value)
	for _, p := range parts {
		if !words[singular(p)] {
			return false
		}
	}
	return len(parts) > 0
}

func splitWords(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// singular is a crude plural strip ("tops" → "top", "shoes" → "shoe"),
// applied to both sides so they meet in the middle.
func singular(w string) string {
	if len(w) > 3 && strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss") {
		return w[:len(w)-1]
	}
	return w
}
