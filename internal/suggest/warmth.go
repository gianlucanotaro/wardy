package suggest

import "github.com/gianlucanotaro/wardy/internal/weather"

// TargetWarmth maps a forecast to the item warmth value (1 = hot weather
// only .. 5 = deep winter) that would feel comfortable today. A rough
// banding on the day's average temperature, not a precise model — see
// MatchWindow for how much slack candidates get around this target.
func TargetWarmth(f weather.Forecast) int {
	avgC := (f.TempHighC + f.TempLowC) / 2

	switch {
	case avgC >= 27:
		return 1
	case avgC >= 21:
		return 2
	case avgC >= 14:
		return 3
	case avgC >= 7:
		return 4
	default:
		return 5
	}
}

// MatchWindow is how far an item's formality/warmth may sit from the
// target and still count as a candidate — docs/taxonomy.md's "within ±1"
// convention, applied consistently to both scales.
const MatchWindow = 1
