// Package taxonomy defines the closed vocabularies every wardrobe item is
// tagged against (see docs/taxonomy.md). Free-text AI tags make filtering
// useless, so every attribute here is a fixed enum, validated on every
// write regardless of whether it came from a human or the AI vision
// pipeline (phase 5).
package taxonomy

import "fmt"

type Category string

const (
	CategoryTop       Category = "top"
	CategoryBottom    Category = "bottom"
	CategoryDress     Category = "dress"
	CategoryOuterwear Category = "outerwear"
	CategoryShoes     Category = "shoes"
	CategoryBag       Category = "bag"
	CategoryAccessory Category = "accessory"
)

// Categories lists every valid category, in the order the UI should
// present them.
var Categories = []Category{
	CategoryTop, CategoryBottom, CategoryDress, CategoryOuterwear,
	CategoryShoes, CategoryBag, CategoryAccessory,
}

func (c Category) Valid() bool {
	for _, v := range Categories {
		if c == v {
			return true
		}
	}
	return false
}

type Pattern string

const (
	PatternSolid    Pattern = "solid"
	PatternStriped  Pattern = "striped"
	PatternChecked  Pattern = "checked"
	PatternFloral   Pattern = "floral"
	PatternGraphic  Pattern = "graphic"
	PatternTextured Pattern = "textured"
	PatternOther    Pattern = "other"
)

var Patterns = []Pattern{
	PatternSolid, PatternStriped, PatternChecked, PatternFloral,
	PatternGraphic, PatternTextured, PatternOther,
}

func (p Pattern) Valid() bool {
	for _, v := range Patterns {
		if p == v {
			return true
		}
	}
	return false
}

type Material string

const (
	MaterialCotton    Material = "cotton"
	MaterialLinen     Material = "linen"
	MaterialWool      Material = "wool"
	MaterialCashmere  Material = "cashmere"
	MaterialDenim     Material = "denim"
	MaterialLeather   Material = "leather"
	MaterialSilk      Material = "silk"
	MaterialSynthetic Material = "synthetic"
	MaterialBlend     Material = "blend"
	MaterialUnknown   Material = "unknown"
)

var Materials = []Material{
	MaterialCotton, MaterialLinen, MaterialWool, MaterialCashmere,
	MaterialDenim, MaterialLeather, MaterialSilk, MaterialSynthetic,
	MaterialBlend, MaterialUnknown,
}

func (m Material) Valid() bool {
	for _, v := range Materials {
		if m == v {
			return true
		}
	}
	return false
}

type Fit string

const (
	FitSlim      Fit = "slim"
	FitRegular   Fit = "regular"
	FitRelaxed   Fit = "relaxed"
	FitOversized Fit = "oversized"
)

var Fits = []Fit{FitSlim, FitRegular, FitRelaxed, FitOversized}

func (f Fit) Valid() bool {
	for _, v := range Fits {
		if f == v {
			return true
		}
	}
	return false
}

// MinScale and MaxScale bound both formality and warmth: 1 (loungewear /
// hot weather only) to 5 (black tie / deep winter). See docs/taxonomy.md
// for why these numeric scales carry the weight of weather and occasion
// matching instead of free-text.
const (
	MinScale = 1
	MaxScale = 5
)

// ValidScale reports whether v is a valid formality or warmth value.
func ValidScale(v int) bool {
	return v >= MinScale && v <= MaxScale
}

// ErrInvalid is returned (wrapped with the failing field and value) by
// Tags.Validate.
type ErrInvalid struct {
	Field string
	Value string
}

func (e *ErrInvalid) Error() string {
	return fmt.Sprintf("invalid %s: %q", e.Field, e.Value)
}
