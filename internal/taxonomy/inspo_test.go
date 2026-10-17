package taxonomy

import (
	"errors"
	"testing"
)

func validInspoTags() InspoTags {
	return InspoTags{
		Styles:       []string{"minimal", "workwear"},
		Colors:       []string{"green", "cream"},
		Pieces:       []string{"jacket", "knit", "trousers", "boots"},
		Fit:          "relaxed",
		FormalityMin: 2,
		FormalityMax: 3,
		Seasons:      []string{"autumn"},
		Occasions:    []string{"casual"},
	}
}

func TestInspoTagsValidate_Valid(t *testing.T) {
	if err := validInspoTags().Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
	if err := (InspoTags{}).Validate(); err != nil {
		t.Errorf("Validate() on an untagged fit = %v, want nil", err)
	}
}

func TestInspoTagsValidate_RejectsEveryInvalidField(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*InspoTags)
		field  string
	}{
		{"unknown style", func(t *InspoTags) { t.Styles = []string{"cottagecore"} }, "styles"},
		{"too many styles", func(t *InspoTags) { t.Styles = []string{"minimal", "classic", "preppy", "edgy"} }, "styles"},
		{"repeated style", func(t *InspoTags) { t.Styles = []string{"minimal", "minimal"} }, "styles"},
		{"empty style", func(t *InspoTags) { t.Styles = []string{""} }, "styles"},
		{"off-palette colour", func(t *InspoTags) { t.Colors = []string{"sage mist"} }, "colors"},
		{"too many colours", func(t *InspoTags) {
			t.Colors = []string{"black", "white", "gray", "navy", "blue", "red"}
		}, "colors"},
		{"category as piece", func(t *InspoTags) { t.Pieces = []string{"outerwear"} }, "pieces"},
		{"made-up piece", func(t *InspoTags) { t.Pieces = []string{"chore coat"} }, "pieces"},
		{"too many pieces", func(t *InspoTags) {
			t.Pieces = []string{"jacket", "knit", "jeans", "boots", "belt", "hat", "scarf", "tote", "watch"}
		}, "pieces"},
		{"unknown fit", func(t *InspoTags) { t.Fit = "baggy" }, "fit"},
		{"half-set formality", func(t *InspoTags) { t.FormalityMin, t.FormalityMax = 0, 3 }, "formality_min"},
		{"formality off scale", func(t *InspoTags) { t.FormalityMax = 6 }, "formality_max"},
		{"formality backwards", func(t *InspoTags) { t.FormalityMin, t.FormalityMax = 4, 2 }, "formality_max"},
		{"unknown season", func(t *InspoTags) { t.Seasons = []string{"monsoon"} }, "seasons"},
		{"unknown occasion", func(t *InspoTags) { t.Occasions = []string{"party"} }, "occasions"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tags := validInspoTags()
			tt.mutate(&tags)
			err := tags.Validate()
			var inv *ErrInvalid
			if !errors.As(err, &inv) {
				t.Fatalf("Validate() = %v, want *ErrInvalid", err)
			}
			if inv.Field != tt.field {
				t.Errorf("Validate() field = %q, want %q", inv.Field, tt.field)
			}
		})
	}
}

func TestValidPiece(t *testing.T) {
	for _, p := range []string{"blazer", "jeans", "sneakers", "tote", "watch", "maxi-dress"} {
		if !ValidPiece(p) {
			t.Errorf("ValidPiece(%q) = false, want true", p)
		}
	}
	for _, p := range []string{"", "top", "Blazer", "chore jacket"} {
		if ValidPiece(p) {
			t.Errorf("ValidPiece(%q) = true, want false", p)
		}
	}
}
