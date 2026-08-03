package suggest

import (
	"testing"
	"time"

	"github.com/gianlucanotaro/wardy/internal/taxonomy"
)

func TestCurrentSeasonNorthernHemisphere(t *testing.T) {
	cases := []struct {
		date time.Time
		want taxonomy.Season
	}{
		{time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC), taxonomy.SeasonWinter},
		{time.Date(2026, time.April, 15, 0, 0, 0, 0, time.UTC), taxonomy.SeasonSpring},
		{time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC), taxonomy.SeasonSummer},
		{time.Date(2026, time.October, 15, 0, 0, 0, 0, time.UTC), taxonomy.SeasonAutumn},
		{time.Date(2026, time.December, 15, 0, 0, 0, 0, time.UTC), taxonomy.SeasonWinter},
	}
	for _, c := range cases {
		if got := CurrentSeason(c.date, true); got != c.want {
			t.Errorf("CurrentSeason(%s, north) = %s, want %s", c.date.Month(), got, c.want)
		}
	}
}

func TestCurrentSeasonSouthernHemisphereIsFlipped(t *testing.T) {
	july := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	if got := CurrentSeason(july, false); got != taxonomy.SeasonWinter {
		t.Errorf("CurrentSeason(July, south) = %s, want winter", got)
	}

	january := time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC)
	if got := CurrentSeason(january, false); got != taxonomy.SeasonSummer {
		t.Errorf("CurrentSeason(January, south) = %s, want summer", got)
	}
}
