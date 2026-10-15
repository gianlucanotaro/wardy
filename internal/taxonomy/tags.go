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
	// Ranges, not single values: a shirt can be formality 2-4 and warmth
	// 1-5 (worn layered in winter). Each bound is on the 1-5 scale and
	// min <= max.
	FormalityMin int
	FormalityMax int
	WarmthMin    int
	WarmthMax    int
	Seasons      []string
	Occasions    []string
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
	if err := validRange("formality", t.FormalityMin, t.FormalityMax); err != nil {
		return err
	}
	if err := validRange("warmth", t.WarmthMin, t.WarmthMax); err != nil {
		return err
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

// validRange checks one min/max pair of a 1-5 scale.
func validRange(field string, lo, hi int) error {
	if !ValidScale(lo) {
		return &ErrInvalid{Field: field + "_min", Value: fmt.Sprint(lo)}
	}
	if !ValidScale(hi) {
		return &ErrInvalid{Field: field + "_max", Value: fmt.Sprint(hi)}
	}
	if lo > hi {
		return &ErrInvalid{Field: field + "_max", Value: fmt.Sprintf("%d (below %s_min %d)", hi, field, lo)}
	}
	return nil
}
