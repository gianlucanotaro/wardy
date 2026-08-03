package suggest

import "github.com/gianlucanotaro/wardy/internal/taxonomy"

// occasionFormality maps an occasion to the item formality value (1
// loungewear .. 5 black tie) it calls for. docs/taxonomy.md defines
// occasions and the formality scale separately without tying them
// together — this mapping is a judgment call, not a spec. Easy to adjust;
// items within MatchWindow of this are still considered candidates.
var occasionFormality = map[taxonomy.Occasion]int{
	taxonomy.OccasionHome:    1,
	taxonomy.OccasionSport:   1,
	taxonomy.OccasionCasual:  2,
	taxonomy.OccasionOutdoor: 2,
	taxonomy.OccasionDate:    3,
	taxonomy.OccasionWork:    4,
	taxonomy.OccasionFormal:  5,
}

// TargetFormality returns occasion's target formality, defaulting to a
// middle value for any occasion not in the map (shouldn't happen for a
// validated taxonomy.Occasion, but never worth a panic over).
func TargetFormality(occasion taxonomy.Occasion) int {
	if f, ok := occasionFormality[occasion]; ok {
		return f
	}
	return 3
}
