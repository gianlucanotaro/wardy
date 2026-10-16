package suggest

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
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
