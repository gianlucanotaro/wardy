package taxonomy

import "testing"

func TestCategoryValid(t *testing.T) {
	for _, c := range Categories {
		if !c.Valid() {
			t.Errorf("Category(%q).Valid() = false, want true", c)
		}
	}

	invalid := []Category{"", "TOP", "tops", "sock", "hat", "jacket"}
	for _, c := range invalid {
		if c.Valid() {
			t.Errorf("Category(%q).Valid() = true, want false", c)
		}
	}
}

func TestPatternValid(t *testing.T) {
	for _, p := range Patterns {
		if !p.Valid() {
			t.Errorf("Pattern(%q).Valid() = false, want true", p)
		}
	}

	invalid := []Pattern{"", "STRIPED", "plaid", "paisley", "camo"}
	for _, p := range invalid {
		if p.Valid() {
			t.Errorf("Pattern(%q).Valid() = true, want false", p)
		}
	}
}

func TestMaterialValid(t *testing.T) {
	for _, m := range Materials {
		if !m.Valid() {
			t.Errorf("Material(%q).Valid() = false, want true", m)
		}
	}

	invalid := []Material{"", "COTTON", "polyester", "acrylic", "nylon"}
	for _, m := range invalid {
		if m.Valid() {
			t.Errorf("Material(%q).Valid() = true, want false", m)
		}
	}
}

func TestFitValid(t *testing.T) {
	for _, f := range Fits {
		if !f.Valid() {
			t.Errorf("Fit(%q).Valid() = false, want true", f)
		}
	}

	invalid := []Fit{"", "SLIM", "baggy", "tailored", "athletic"}
	for _, f := range invalid {
		if f.Valid() {
			t.Errorf("Fit(%q).Valid() = true, want false", f)
		}
	}
}

func TestValidScale(t *testing.T) {
	cases := []struct {
		v    int
		want bool
	}{
		{0, false}, {1, true}, {2, true}, {3, true}, {4, true}, {5, true},
		{6, false}, {-1, false}, {100, false},
	}
	for _, c := range cases {
		if got := ValidScale(c.v); got != c.want {
			t.Errorf("ValidScale(%d) = %v, want %v", c.v, got, c.want)
		}
	}
}
