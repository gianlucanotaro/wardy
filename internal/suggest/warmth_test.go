package suggest

import (
	"testing"

	"github.com/gianlucanotaro/wardy/internal/weather"
)

func TestTargetWarmth(t *testing.T) {
	cases := []struct {
		name string
		f    weather.Forecast
		want int
	}{
		{"hot summer day", weather.Forecast{TempHighC: 32, TempLowC: 24}, 1},
		{"warm day", weather.Forecast{TempHighC: 25, TempLowC: 19}, 2},
		{"mild day", weather.Forecast{TempHighC: 18, TempLowC: 12}, 3},
		{"cool day", weather.Forecast{TempHighC: 11, TempLowC: 5}, 4},
		{"freezing day", weather.Forecast{TempHighC: 2, TempLowC: -5}, 5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := TargetWarmth(c.f); got != c.want {
				t.Errorf("TargetWarmth(%+v) = %d, want %d", c.f, got, c.want)
			}
		})
	}
}
