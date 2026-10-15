package suggest

import (
	"github.com/gianlucanotaro/wardy/internal/store"
	"github.com/gianlucanotaro/wardy/internal/taxonomy"
	"github.com/gianlucanotaro/wardy/internal/weather"
)

// TargetWarmth maps a forecast to the item warmth value (1 = hot weather
// only .. 5 = deep winter) that would feel comfortable today. A rough
// banding on the day's average temperature, not a precise model — see
// FitsWeather and assembleOutfits for how pieces are matched against it.
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

// FitsWeather reports whether a piece can be part of an outfit for the
// target warmth: the day has to fall inside the piece's own warmth range.
// The range already counts layering — a light shirt tagged 1-5 goes under
// a knit or coat in winter, which assembleOutfits then adds. Mirrored in
// web/src/outfitRules.ts (with TargetWarmth's bands) for the item view, so
// change both.
func FitsWeather(it store.Item, target int) bool {
	return it.WarmthMin <= target && target <= it.WarmthMax
}

// pieceWarmth is how warm a piece is on its own, for layering: one step
// above the hottest weather it suits. A shirt tagged 1-5 is a light 2 (it
// needs a layer over it in the cold); a coat tagged 4-5 is a 5.
func pieceWarmth(it store.Item) int {
	return min(it.WarmthMin+1, taxonomy.MaxScale)
}

// neededWarmth is the warmth an outfit's warmest upper-body layer must
// reach for the target — one step of slack, as layers add up.
func neededWarmth(target int) int {
	return target - 1
}

// OuterwearFromWarmth: at or above this target, outfits always get
// outerwear (when the wardrobe has any), however warm the top is.
const OuterwearFromWarmth = 4
