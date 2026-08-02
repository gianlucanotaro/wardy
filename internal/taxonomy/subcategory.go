package taxonomy

// Subcategories lists the valid subcategory values per category. This is a
// starting v1 list (docs/taxonomy.md only gives examples) — adding a value
// is a one-line change here plus its mirror in web/src/taxonomy.ts, not a
// schema change, so don't hesitate to extend it as real wardrobes surface
// gaps.
var Subcategories = map[Category][]string{
	CategoryTop: {
		"t-shirt", "shirt", "button-down", "blouse", "knit", "sweater",
		"hoodie", "tank", "polo", "cardigan", "sweatshirt", "long-sleeve-tee",
		"turtleneck", "crop-top",
	},
	CategoryBottom: {
		"jeans", "chinos", "trousers", "shorts", "skirt", "leggings",
		"sweatpants", "cargo-pants", "overalls",
	},
	CategoryDress: {
		"casual-dress", "cocktail-dress", "maxi-dress", "jumpsuit",
		"midi-dress", "mini-dress", "shirt-dress", "sweater-dress", "romper",
	},
	CategoryOuterwear: {
		"jacket", "blazer", "coat", "parka", "puffer-jacket", "vest",
		"trench-coat", "raincoat", "bomber-jacket", "fleece-jacket",
	},
	CategoryShoes: {
		"sneakers", "boots", "sandals", "heels", "flats", "loafers",
		"dress-shoes", "slippers", "flip-flops", "rain-boots",
	},
	CategoryBag: {
		"backpack", "tote", "crossbody", "clutch", "handbag",
		"belt-bag", "duffel", "briefcase",
	},
	// "hat" means a brimmed hat specifically, distinct from "cap"
	// (structured, flat-brim/baseball style) and "beanie" (knit).
	CategoryAccessory: {
		"belt", "hat", "scarf", "jewelry", "sunglasses", "watch",
		"gloves", "tie", "socks", "beanie", "cap", "glasses",
	},
}

// ValidSubcategory reports whether sub is one of category's allowed
// subcategories. An unrecognized category is always invalid.
func ValidSubcategory(category Category, sub string) bool {
	for _, v := range Subcategories[category] {
		if v == sub {
			return true
		}
	}
	return false
}
