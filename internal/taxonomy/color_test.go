package taxonomy

import "testing"

func TestValidColor(t *testing.T) {
	for _, c := range Colors {
		if !ValidColor(c) {
			t.Errorf("ValidColor(%q) = false, want true", c)
		}
	}

	invalid := []string{"", "BLACK", "sage mist", "dusty olive", "chartreuse"}
	for _, c := range invalid {
		if ValidColor(c) {
			t.Errorf("ValidColor(%q) = true, want false", c)
		}
	}
}
