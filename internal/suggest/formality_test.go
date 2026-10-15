package suggest

import (
	"testing"

	"github.com/gianlucanotaro/wardy/internal/taxonomy"
)

func TestOccasionFormalityRangesAreValid(t *testing.T) {
	for _, occasion := range taxonomy.Occasions {
		r := OccasionFormality(occasion)
		if r.Min < taxonomy.MinScale || r.Max > taxonomy.MaxScale || r.Min > r.Max {
			t.Errorf("OccasionFormality(%s) = %+v, not a valid range within 1-5", occasion, r)
		}
	}
}

func TestOccasionFormalityUnknownOccasionDoesNotPanic(t *testing.T) {
	r := OccasionFormality(taxonomy.Occasion("bogus"))
	if r.Min != taxonomy.MinScale || r.Max != taxonomy.MaxScale {
		t.Errorf("OccasionFormality(bogus) = %+v, want the whole scale", r)
	}
}

// The issue that started this: a business piece (4) must be usable for
// casual, and loungewear (1) must not be usable for work.
func TestOccasionFormalityDressUpNotDown(t *testing.T) {
	casual := OccasionFormality(taxonomy.OccasionCasual)
	if 4 < casual.Min || 4 > casual.Max {
		t.Errorf("casual range %+v should accept a business piece (4)", casual)
	}
	work := OccasionFormality(taxonomy.OccasionWork)
	if 1 >= work.Min {
		t.Errorf("work range %+v should reject loungewear (1)", work)
	}
}
