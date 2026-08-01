package taxonomy

// Colors is the fixed palette every color_primary/color_secondary value
// must map to. Per docs/taxonomy.md: never let the model (or a user)
// invent "sage mist" — two names for one color kills filtering.
var Colors = []string{
	"black", "white", "gray", "navy", "blue", "red", "burgundy", "pink",
	"orange", "yellow", "green", "olive", "brown", "tan", "beige", "cream",
	"purple", "teal", "gold", "silver", "multicolor",
}

func ValidColor(c string) bool {
	for _, v := range Colors {
		if v == c {
			return true
		}
	}
	return false
}
