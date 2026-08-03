package suggest

import (
	"time"

	"github.com/gianlucanotaro/wardy/internal/taxonomy"
)

// CurrentSeason returns the meteorological season for t, flipped for the
// southern hemisphere. Pass northernHemisphere = home latitude >= 0 (phase
// 6 stores the user's home_lat, so this comes for free once it's set).
func CurrentSeason(t time.Time, northernHemisphere bool) taxonomy.Season {
	var season taxonomy.Season
	switch t.Month() {
	case time.March, time.April, time.May:
		season = taxonomy.SeasonSpring
	case time.June, time.July, time.August:
		season = taxonomy.SeasonSummer
	case time.September, time.October, time.November:
		season = taxonomy.SeasonAutumn
	default: // December, January, February
		season = taxonomy.SeasonWinter
	}

	if northernHemisphere {
		return season
	}
	return oppositeSeason(season)
}

func oppositeSeason(s taxonomy.Season) taxonomy.Season {
	switch s {
	case taxonomy.SeasonSpring:
		return taxonomy.SeasonAutumn
	case taxonomy.SeasonAutumn:
		return taxonomy.SeasonSpring
	case taxonomy.SeasonSummer:
		return taxonomy.SeasonWinter
	case taxonomy.SeasonWinter:
		return taxonomy.SeasonSummer
	default:
		return s
	}
}
