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
		Formality:      2,
		Warmth:         3,
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
		{"formality too low", func(tg *Tags) { tg.Formality = 0 }, "formality"},
		{"formality too high", func(tg *Tags) { tg.Formality = 6 }, "formality"},
		{"warmth too low", func(tg *Tags) { tg.Warmth = 0 }, "warmth"},
		{"warmth too high", func(tg *Tags) { tg.Warmth = 6 }, "warmth"},
		{"seasons", func(tg *Tags) { tg.Seasons = []string{"monsoon"} }, "seasons"},
		{"occasions", func(tg *Tags) { tg.Occasions = []string{"vacation"} }, "occasions"},
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

func TestTagsValidate_EmptySecondaryColorIsFine(t *testing.T) {
	tags := validTags() // ColorSecondary already ""
	if err := tags.Validate(); err != nil {
		t.Errorf("Validate() with empty color_secondary = %v, want nil", err)
	}
}
