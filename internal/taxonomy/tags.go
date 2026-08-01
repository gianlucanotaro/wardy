package taxonomy

import "fmt"

// Tags is the full set of taxonomy fields for one item, as submitted by
// either a manual tag edit or the AI vision pipeline (phase 5). Validate
// rejects the whole payload on the first invalid field — callers should
// never partially apply a tag edit.
type Tags struct {
	Category       Category
	Subcategory    string
	ColorPrimary   string
	ColorSecondary string // may be empty: no secondary color
	Pattern        Pattern
	Material       Material
	Fit            Fit
	Formality      int
	Warmth         int
	Seasons        []string
	Occasions      []string
}

func (t Tags) Validate() error {
	if !t.Category.Valid() {
		return &ErrInvalid{Field: "category", Value: string(t.Category)}
	}
	if !ValidSubcategory(t.Category, t.Subcategory) {
		return &ErrInvalid{Field: "subcategory", Value: t.Subcategory}
	}
	if !ValidColor(t.ColorPrimary) {
		return &ErrInvalid{Field: "color_primary", Value: t.ColorPrimary}
	}
	if t.ColorSecondary != "" && !ValidColor(t.ColorSecondary) {
		return &ErrInvalid{Field: "color_secondary", Value: t.ColorSecondary}
	}
	if !t.Pattern.Valid() {
		return &ErrInvalid{Field: "pattern", Value: string(t.Pattern)}
	}
	if !t.Material.Valid() {
		return &ErrInvalid{Field: "material", Value: string(t.Material)}
	}
	if !t.Fit.Valid() {
		return &ErrInvalid{Field: "fit", Value: string(t.Fit)}
	}
	if !ValidScale(t.Formality) {
		return &ErrInvalid{Field: "formality", Value: fmt.Sprint(t.Formality)}
	}
	if !ValidScale(t.Warmth) {
		return &ErrInvalid{Field: "warmth", Value: fmt.Sprint(t.Warmth)}
	}
	for _, s := range t.Seasons {
		if !Season(s).Valid() {
			return &ErrInvalid{Field: "seasons", Value: s}
		}
	}
	for _, o := range t.Occasions {
		if !Occasion(o).Valid() {
			return &ErrInvalid{Field: "occasions", Value: o}
		}
	}
	return nil
}
