package suggest

import "github.com/gianlucanotaro/wardy/internal/taxonomy"

// FormalityRange is the span of item formality (1 loungewear .. 5 black
// tie) an occasion accepts. Ranges rather than a single target because
// dressing one piece up is normal (a blazer with jeans is still casual)
// while dressing down below the floor isn't (a hoodie isn't work wear).
type FormalityRange struct {
	Min, Max int
}

// occasionFormality maps each occasion to the formality it accepts.
// docs/taxonomy.md defines occasions and the formality scale separately
// without tying them together — this mapping is a judgment call, not a
// spec. Easy to adjust — and mirrored in web/src/outfitRules.ts for the
// item view, so change both.
var occasionFormality = map[taxonomy.Occasion]FormalityRange{
	taxonomy.OccasionHome:    {1, 2},
	taxonomy.OccasionSport:   {1, 2},
	taxonomy.OccasionOutdoor: {1, 3},
	taxonomy.OccasionCasual:  {1, 4},
	taxonomy.OccasionDate:    {2, 4},
	taxonomy.OccasionWork:    {3, 5},
	taxonomy.OccasionFormal:  {4, 5},
}

// MaxFormalitySpread is how far apart the most and least formal pieces of
// one outfit may be: blazer (4) with jeans (2) works, blazer with joggers
// (1) doesn't.
const MaxFormalitySpread = 2

// OccasionFormality returns occasion's accepted formality range,
// defaulting to the whole scale for any occasion not in the map
// (shouldn't happen for a validated taxonomy.Occasion, but never worth a
// panic over).
func OccasionFormality(occasion taxonomy.Occasion) FormalityRange {
	if r, ok := occasionFormality[occasion]; ok {
		return r
	}
	return FormalityRange{taxonomy.MinScale, taxonomy.MaxScale}
}
