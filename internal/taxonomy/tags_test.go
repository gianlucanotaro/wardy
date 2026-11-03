package taxonomy

import (
	"errors"
	"testing"
)

func validTags() Tags {
	return Tags{
		Category:       CategoryTop,
		Subcategory:    "t-shirt",
		ColorPrimary:   "blue",
		ColorSecondary: "",
		Pattern:        PatternSolid,
		Material:       MaterialCotton,
		Fit:            FitRegular,
		FormalityMin:   2,
		FormalityMax:   3,
		WarmthMin:      1,
		WarmthMax:      3,
		Seasons:        []string{"summer", "spring"},
		Occasions:      []string{"casual"},
	}
}

func TestTagsValidate_Valid(t *testing.T) {
	if err := validTags().Validate(); err != nil {
		t.Errorf("Validate() on a fully valid Tags = %v, want nil", err)
	}
}

func TestTagsValidate_OptionalSecondaryColor(t *testing.T) {
	tags := validTags()
	tags.ColorSecondary = "white"
	if err := tags.Validate(); err != nil {
		t.Errorf("Validate() with a valid secondary color = %v, want nil", err)
	}
}

func TestTagsValidate_RejectsEveryInvalidField(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*Tags)
		wantErr string
	}{
		{"category", func(tg *Tags) { tg.Category = "sock" }, "category"},
		{"subcategory wrong for category", func(tg *Tags) { tg.Subcategory = "jeans" }, "subcategory"},
		{"color_primary", func(tg *Tags) { tg.ColorPrimary = "sage mist" }, "color_primary"},
		{"color_secondary", func(tg *Tags) { tg.ColorSecondary = "sage mist" }, "color_secondary"},
		{"pattern", func(tg *Tags) { tg.Pattern = "plaid" }, "pattern"},
		{"material", func(tg *Tags) { tg.Material = "acrylic" }, "material"},
		{"fit", func(tg *Tags) { tg.Fit = "baggy" }, "fit"},
		{"formality_min too low", func(tg *Tags) { tg.FormalityMin = 0 }, "formality_min"},
		{"formality_max too high", func(tg *Tags) { tg.FormalityMax = 6 }, "formality_max"},
		{"formality min above max", func(tg *Tags) { tg.FormalityMin, tg.FormalityMax = 4, 2 }, "formality_max"},
		{"warmth_min too low", func(tg *Tags) { tg.WarmthMin = 0 }, "warmth_min"},
		{"warmth_max too high", func(tg *Tags) { tg.WarmthMax = 6 }, "warmth_max"},
		{"warmth min above max", func(tg *Tags) { tg.WarmthMin, tg.WarmthMax = 5, 1 }, "warmth_max"},
		{"seasons", func(tg *Tags) { tg.Seasons = []string{"monsoon"} }, "seasons"},
		{"occasions", func(tg *Tags) { tg.Occasions = []string{"vacation"} }, "occasions"},
		{"styles out of vocabulary", func(tg *Tags) { tg.Styles = []string{"cottagecore"} }, "styles"},
		{"styles repeated", func(tg *Tags) { tg.Styles = []string{"minimal", "minimal"} }, "styles"},
		{"styles over the cap", func(tg *Tags) { tg.Styles = []string{"minimal", "classic", "edgy", "vintage"} }, "styles"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tags := validTags()
			c.mutate(&tags)

			err := tags.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want an error for %s", c.name)
			}

			var invalid *ErrInvalid
			if !errors.As(err, &invalid) {
				t.Fatalf("Validate() error = %v, want *ErrInvalid", err)
			}
			if invalid.Field != c.wantErr {
				t.Errorf("Validate() field = %q, want %q", invalid.Field, c.wantErr)
			}
		})
	}
}

func TestTagsValidate_Styles(t *testing.T) {
	tags := validTags()
	tags.Styles = []string{"minimal", "smart-casual", "workwear"}
	if err := tags.Validate(); err != nil {
		t.Errorf("Validate() with 3 valid styles = %v, want nil", err)
	}
	tags.Styles = nil
	if err := tags.Validate(); err != nil {
		t.Errorf("Validate() with no styles = %v, want nil (items start without)", err)
	}
}

func TestTagsValidate_EmptySecondaryColorIsFine(t *testing.T) {
	tags := validTags() // ColorSecondary already ""
	if err := tags.Validate(); err != nil {
		t.Errorf("Validate() with empty color_secondary = %v, want nil", err)
	}
}
