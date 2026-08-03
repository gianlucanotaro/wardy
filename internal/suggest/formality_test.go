package suggest

import (
	"testing"

	"github.com/gianlucanotaro/wardy/internal/taxonomy"
)

func TestTargetFormality(t *testing.T) {
	for _, occasion := range taxonomy.Occasions {
		got := TargetFormality(occasion)
		if got < taxonomy.MinScale || got > taxonomy.MaxScale {
			t.Errorf("TargetFormality(%s) = %d, out of the valid 1-5 range", occasion, got)
		}
	}
}

func TestTargetFormalityUnknownOccasionDoesNotPanic(t *testing.T) {
	got := TargetFormality(taxonomy.Occasion("bogus"))
	if got < taxonomy.MinScale || got > taxonomy.MaxScale {
		t.Errorf("TargetFormality(bogus) = %d, out of range", got)
	}
}
