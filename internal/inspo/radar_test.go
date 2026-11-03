package inspo

import (
	"reflect"
	"testing"
)

func TestBuildRadar(t *testing.T) {
	owned := [][]string{
		{"minimal"},
		{"classic", "preppy"}, // one axis: counts once
		{"streetwear"},
		{"romantic", "unknown-style"}, // an unknown style is ignored
		{},                            // no styles: not counted
	}
	r := BuildRadar(owned, nil, [][]string{{"minimal", "workwear"}})

	if len(r.Axes) != 9 || r.Axes[0] != "minimal" || r.Axes[1] != "classic" {
		t.Fatalf("Axes = %v", r.Axes)
	}
	// 4 tags over 4 items: minimal, classic, streetwear, romantic at 25% each
	want := []float64{25, 25, 0, 25, 0, 0, 0, 25, 0}
	if !reflect.DeepEqual(r.Owned.Shares, want) || r.Owned.Count != 4 {
		t.Errorf("Owned = %+v, want shares %v over 4 items", r.Owned, want)
	}
	if r.Worn.Count != 0 || r.Worn.Shares[0] != 0 {
		t.Errorf("Worn = %+v, want empty", r.Worn)
	}
	if r.Inspo.Shares[0] != 50 || r.Inspo.Shares[5] != 50 || r.Inspo.Count != 1 {
		t.Errorf("Inspo = %+v, want minimal and workwear at 50", r.Inspo)
	}
}

func TestBuildRadarCountsEveryWear(t *testing.T) {
	r := BuildRadar(nil, [][]string{{"sporty"}, {"sporty"}, {"vintage"}}, nil)
	if r.Worn.Shares[4] != 66.7 || r.Worn.Shares[8] != 33.3 || r.Worn.Count != 3 {
		t.Errorf("Worn = %+v, want sporty 66.7, vintage 33.3", r.Worn)
	}
}
