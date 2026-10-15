package suggest

import (
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

// FitsWeather reports whether a single piece can be part of an outfit for
// the target warmth. Warmth is judged per outfit, not per piece: a light
// top is fine on a cold day because assembleOutfits layers a knit or
// outerwear over it, so tops only have a ceiling (no heavy knit in a
// heatwave). Legs and feet don't layer, so bottoms, dresses and shoes
// keep a floor too (no shorts or sandals in winter). Bags and accessories
// are never ruled out by weather.
func FitsWeather(category string, warmth, target int) bool {
	switch taxonomy.Category(category) {
	case taxonomy.CategoryTop:
		return warmth <= target+1
	case taxonomy.CategoryOuterwear:
		return warmth >= target-1 && warmth <= target+1
	case taxonomy.CategoryBottom, taxonomy.CategoryDress, taxonomy.CategoryShoes:
		return warmth >= target-2 && warmth <= target+1
	default:
		return true
	}
}

// neededWarmth is the warmth an outfit's warmest upper-body layer must
// reach for the target — one step of slack, as layers add up.
func neededWarmth(target int) int {
	return target - 1
}

// OuterwearFromWarmth: at or above this target, outfits always get
// outerwear (when the wardrobe has any), however warm the top is.
const OuterwearFromWarmth = 4
