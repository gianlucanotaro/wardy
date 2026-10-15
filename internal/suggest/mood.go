package suggest

import "slices"

// Moods are the moods a suggestion can be asked for. They go into the AI
// prompt as context — never a filter on the candidate set — so the list is
// closed: a mood is one of these words, not free text (free text is #41's
// prompt, which gets its own caps). Mirrored in web/src/api.ts's MOODS.
var Moods = []string{"sharp", "easy", "bold", "quiet", "cosy", "playful"}

// ValidMood reports whether mood is one of Moods. Empty means "no mood".
func ValidMood(mood string) bool {
	return mood == "" || slices.Contains(Moods, mood)
}
