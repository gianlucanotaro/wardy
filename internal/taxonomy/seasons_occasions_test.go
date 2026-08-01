package taxonomy

import "testing"

func TestSeasonValid(t *testing.T) {
	for _, s := range Seasons {
		if !s.Valid() {
			t.Errorf("Season(%q).Valid() = false, want true", s)
		}
	}

	invalid := []Season{"", "SUMMER", "monsoon", "fall", "wet-season"}
	for _, s := range invalid {
		if s.Valid() {
			t.Errorf("Season(%q).Valid() = true, want false", s)
		}
	}
}

func TestOccasionValid(t *testing.T) {
	for _, o := range Occasions {
		if !o.Valid() {
			t.Errorf("Occasion(%q).Valid() = false, want true", o)
		}
	}

	invalid := []Occasion{"", "WORK", "party", "vacation", "gym"}
	for _, o := range invalid {
		if o.Valid() {
			t.Errorf("Occasion(%q).Valid() = true, want false", o)
		}
	}
}
