package inspo

import (
	"math"

	"github.com/gianlucanotaro/wardy/internal/taxonomy"
)

// RadarWornDays is how far back the radar's "worn" layer looks.
const RadarWornDays = 90

// radarAxes groups the 12 styles into the radar's 9 axes: close neighbours
// share one, so a phone-sized chart stays readable.
var radarAxes = []struct {
	Name   string
	Styles []taxonomy.Style
}{
	{"minimal", []taxonomy.Style{taxonomy.StyleMinimal}},
	{"classic", []taxonomy.Style{taxonomy.StyleClassic, taxonomy.StylePreppy}},
	{"smart-casual", []taxonomy.Style{taxonomy.StyleSmartCasual}},
	{"streetwear", []taxonomy.Style{taxonomy.StyleStreetwear}},
	{"sporty", []taxonomy.Style{taxonomy.StyleSporty}},
	{"workwear", []taxonomy.Style{taxonomy.StyleWorkwear}},
	{"edgy", []taxonomy.Style{taxonomy.StyleEdgy, taxonomy.StyleAvantGarde}},
	{"romantic", []taxonomy.Style{taxonomy.StyleRomantic, taxonomy.StyleBohemian}},
	{"vintage", []taxonomy.Style{taxonomy.StyleVintage}},
}

// RadarLayer is one source's style mix: Shares (percent, one per axis,
// summing to 100 when Count > 0) and how many items, wears or fits with at
// least one style it was counted from.
type RadarLayer struct {
	Shares []float64 `json:"shares"`
	Count  int       `json:"count"`
}

// Radar compares three style mixes on one scale: what the user owns, what
// they wore lately, and what their inspo board likes. Each layer is a share
// of that source's style tags, so 80 items and 10 fits compare directly.
type Radar struct {
	Axes  []string   `json:"axes"`
	Owned RadarLayer `json:"owned"`
	Worn  RadarLayer `json:"worn"`
	Inspo RadarLayer `json:"inspo"`
}

// BuildRadar adds up style lists: one per owned item, one per item in each
// recent wear (an item worn twice counts twice) and one per inspo fit.
func BuildRadar(owned, worn, fits [][]string) Radar {
	axes := make([]string, len(radarAxes))
	for i, a := range radarAxes {
		axes[i] = a.Name
	}
	return Radar{Axes: axes, Owned: radarLayer(owned), Worn: radarLayer(worn), Inspo: radarLayer(fits)}
}

func radarLayer(sources [][]string) RadarLayer {
	counts := make([]int, len(radarAxes))
	total, n := 0, 0
	for _, styles := range sources {
		// An item that is both bohemian and romantic counts once on the
		// axis they share.
		hit := make([]bool, len(radarAxes))
		for _, s := range styles {
			if i := radarAxisOf(taxonomy.Style(s)); i >= 0 && !hit[i] {
				hit[i] = true
				counts[i]++
				total++
			}
		}
		if len(styles) > 0 {
			n++
		}
	}
	shares := make([]float64, len(radarAxes))
	for i, c := range counts {
		if total > 0 {
			shares[i] = math.Round(float64(c)/float64(total)*1000) / 10
		}
	}
	return RadarLayer{Shares: shares, Count: n}
}

func radarAxisOf(s taxonomy.Style) int {
	for i, a := range radarAxes {
		for _, v := range a.Styles {
			if v == s {
				return i
			}
		}
	}
	return -1
}
