package taxonomy

import "fmt"

// Style is the overall look of an inspiration fit (#49) — the one closed
// vocabulary inspo adds; everything else on a fit reuses the item enums.
type Style string

const (
	StyleMinimal     Style = "minimal"
	StyleClassic     Style = "classic"
	StylePreppy      Style = "preppy"
	StyleWorkwear    Style = "workwear"
	StyleStreetwear  Style = "streetwear"
	StyleSporty      Style = "sporty"
	StyleSmartCasual Style = "smart-casual"
	StyleBohemian    Style = "bohemian"
	StyleEdgy        Style = "edgy"
	StyleRomantic    Style = "romantic"
	StyleAvantGarde  Style = "avant-garde"
	StyleVintage     Style = "vintage"
)

var Styles = []Style{
	StyleMinimal, StyleClassic, StylePreppy, StyleWorkwear, StyleStreetwear, StyleSporty,
	StyleSmartCasual, StyleBohemian, StyleEdgy, StyleRomantic, StyleAvantGarde, StyleVintage,
}

func (s Style) Valid() bool {
	for _, v := range Styles {
		if s == v {
			return true
		}
	}
	return false
}

// Caps on an inspo fit's lists: enough to describe one outfit, small
// enough that the style profile stays about the dominant pieces.
const (
	MaxInspoStyles = 3
	MaxInspoColors = 5
	MaxInspoPieces = 8
)

// ValidPiece reports whether piece is a subcategory of any category —
// an inspo fit's pieces are named with the item subcategories ("blazer",
// "jeans") so they can be matched against the user's own wardrobe.
func ValidPiece(piece string) bool {
	for _, cat := range Categories {
		if ValidSubcategory(cat, piece) {
			return true
		}
	}
	return false
}

// InspoTags is the full set of tags on an inspiration fit, from a manual
// edit or AI tagging. Unlike an item, every field may be empty: an
// untagged fit just doesn't count toward that part of the profile.
// Formality 0-0 means unset.
type InspoTags struct {
	Styles       []string
	Colors       []string
	Pieces       []string
	Fit          string
	FormalityMin int
	FormalityMax int
	Seasons      []string
	Occasions    []string
}

func (t InspoTags) Validate() error {
	if err := validList("styles", t.Styles, MaxInspoStyles, func(s string) bool { return Style(s).Valid() }); err != nil {
		return err
	}
	if err := validList("colors", t.Colors, MaxInspoColors, ValidColor); err != nil {
		return err
	}
	if err := validList("pieces", t.Pieces, MaxInspoPieces, ValidPiece); err != nil {
		return err
	}
	if t.Fit != "" && !Fit(t.Fit).Valid() {
		return &ErrInvalid{Field: "fit", Value: t.Fit}
	}
	if t.FormalityMin != 0 || t.FormalityMax != 0 {
		if err := validRange("formality", t.FormalityMin, t.FormalityMax); err != nil {
			return err
		}
	}
	if err := validList("seasons", t.Seasons, len(Seasons), func(s string) bool { return Season(s).Valid() }); err != nil {
		return err
	}
	return validList("occasions", t.Occasions, len(Occasions), func(s string) bool { return Occasion(s).Valid() })
}

// validList checks every value of a multi-value field, that none repeats,
// and that there are at most limit of them.
func validList(field string, values []string, limit int, valid func(string) bool) error {
	if len(values) > limit {
		return &ErrInvalid{Field: field, Value: fmt.Sprintf("%d values (max %d)", len(values), limit)}
	}
	seen := make(map[string]bool, len(values))
	for _, v := range values {
		if !valid(v) || seen[v] {
			return &ErrInvalid{Field: field, Value: v}
		}
		seen[v] = true
	}
	return nil
}
