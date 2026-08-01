package taxonomy

// Subcategories lists the valid subcategory values per category. This is a
// starting v1 list (docs/taxonomy.md only gives examples) — adding a value
// is a one-line change here plus its mirror in web/src/taxonomy.ts, not a
// schema change, so don't hesitate to extend it as real wardrobes surface
// gaps.
var Subcategories = map[Category][]string{
	CategoryTop: {
		"t-shirt", "shirt", "button-down", "blouse", "knit", "sweater",
		"hoodie", "tank", "polo",
	},
	CategoryBottom: {
		"jeans", "chinos", "trousers", "shorts", "skirt", "leggings",
	},
	CategoryDress: {
		"casual-dress", "cocktail-dress", "maxi-dress", "jumpsuit",
	},
	CategoryOuterwear: {
		"jacket", "blazer", "coat", "parka", "vest",
	},
	CategoryShoes: {
		"sneakers", "boots", "sandals", "heels", "flats", "loafers",
	},
	CategoryBag: {
		"backpack", "tote", "crossbody", "clutch", "handbag",
	},
	CategoryAccessory: {
		"belt", "hat", "scarf", "jewelry", "sunglasses", "watch",
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
