package taxonomy

import "testing"

func TestValidSubcategory(t *testing.T) {
	// Every declared subcategory must validate against its own category.
	for category, subs := range Subcategories {
		for _, sub := range subs {
			if !ValidSubcategory(category, sub) {
				t.Errorf("ValidSubcategory(%q, %q) = false, want true", category, sub)
			}
		}
	}

	cases := []struct {
		category Category
		sub      string
	}{
		{CategoryTop, "jeans"},       // valid sub, wrong category
		{CategoryBottom, "t-shirt"},  // valid sub, wrong category
		{CategoryTop, "spaceship"},   // not a real subcategory anywhere
		{CategoryTop, ""},            // empty
		{Category("bogus"), "jeans"}, // unknown category entirely
		{CategoryShoes, "T-Shirt"},   // case mismatch
	}
	for _, c := range cases {
		if ValidSubcategory(c.category, c.sub) {
			t.Errorf("ValidSubcategory(%q, %q) = true, want false", c.category, c.sub)
		}
	}
}
