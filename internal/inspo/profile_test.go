package inspo

import (
	"reflect"
	"strings"
	"testing"

	"github.com/gianlucanotaro/wardy/internal/store"
)

func fit(styles, colors, pieces []string, shape string, fmin, fmax int) store.InspoFit {
	return store.InspoFit{Styles: styles, Colors: colors, Pieces: pieces, Fit: shape, FormalityMin: fmin, FormalityMax: fmax}
}

func TestBuild(t *testing.T) {
	fits := []store.InspoFit{
		fit([]string{"minimal"}, []string{"black", "white"}, []string{"jeans", "t-shirt"}, "relaxed", 2, 2),
		fit([]string{"minimal", "workwear"}, []string{"green", "white"}, []string{"jacket", "jeans"}, "relaxed", 2, 4),
		fit([]string{"streetwear"}, []string{"black"}, []string{"hoodie"}, "oversized", 0, 0),
		{}, // not tagged yet: doesn't count
	}
	p := Build(fits)

	if p.Fits != 4 || p.Tagged != 3 {
		t.Errorf("Fits, Tagged = %d, %d, want 4, 3", p.Fits, p.Tagged)
	}
	wantStyles := []Count{{"minimal", 2}, {"streetwear", 1}, {"workwear", 1}}
	if !reflect.DeepEqual(p.Styles, wantStyles) {
		t.Errorf("Styles = %v, want %v", p.Styles, wantStyles)
	}
	wantColors := []Count{{"black", 2}, {"white", 2}, {"green", 1}}
	if !reflect.DeepEqual(p.Colors, wantColors) {
		t.Errorf("Colors = %v, want %v", p.Colors, wantColors)
	}
	if p.Pieces[0] != (Count{"jeans", 2}) {
		t.Errorf("top piece = %v, want jeans 2", p.Pieces[0])
	}
	if p.FitShapes[0] != (Count{"relaxed", 2}) {
		t.Errorf("top fit = %v, want relaxed 2", p.FitShapes[0])
	}
	// (2 + 3) / 2: the third fit has no formality.
	if p.Formality != 2.5 {
		t.Errorf("Formality = %v, want 2.5", p.Formality)
	}
}

func TestBuildKeepsOnlyTheTopStyles(t *testing.T) {
	var fits []store.InspoFit
	for _, s := range []string{"minimal", "minimal", "classic", "classic", "edgy", "preppy"} {
		fits = append(fits, fit([]string{s}, nil, nil, "", 0, 0))
	}
	p := Build(fits)
	want := []Count{{"classic", 2}, {"minimal", 2}, {"edgy", 1}}
	if !reflect.DeepEqual(p.Styles, want) {
		t.Errorf("Styles = %v, want %v", p.Styles, want)
	}
}

func TestUsableForSuggestions(t *testing.T) {
	var fits []store.InspoFit
	for range MinFitsForSuggestions - 1 {
		fits = append(fits, fit([]string{"minimal"}, nil, nil, "", 0, 0))
	}
	fits = append(fits, store.InspoFit{}) // untagged doesn't count
	if Build(fits).UsableForSuggestions() {
		t.Error("UsableForSuggestions with too few tagged fits = true, want false")
	}
	fits = append(fits, fit(nil, []string{"black"}, nil, "", 0, 0))
	if !Build(fits).UsableForSuggestions() {
		t.Error("UsableForSuggestions with enough tagged fits = false, want true")
	}
}

func TestKeyChangesOnlyWithTheProfile(t *testing.T) {
	a := []store.InspoFit{fit([]string{"minimal"}, []string{"black"}, []string{"jeans"}, "", 0, 0)}
	b := append(a, store.InspoFit{}) // one more untagged fit: same profile
	c := append(a, fit([]string{"edgy"}, nil, nil, "", 0, 0))

	if Build(a).Key() != Build(b).Key() {
		t.Error("Key changed though the profile didn't")
	}
	if Build(a).Key() == Build(c).Key() {
		t.Error("Key didn't change though the profile did")
	}
}

func TestDescribe(t *testing.T) {
	if got := Build(nil).Describe(); got != "" {
		t.Errorf("Describe of an empty board = %q, want empty", got)
	}
	p := Build([]store.InspoFit{
		fit([]string{"minimal"}, []string{"black"}, []string{"jeans"}, "relaxed", 2, 2),
		fit([]string{"minimal"}, []string{"white"}, nil, "", 0, 0),
	})
	got := p.Describe()
	for _, want := range []string{"From 2 outfit photos", "minimal 100%", "black 50%", "jeans 50%", "relaxed 50%", "Formality around 2.0"} {
		if !strings.Contains(got, want) {
			t.Errorf("Describe() = %q, missing %q", got, want)
		}
	}
}

func TestMatches(t *testing.T) {
	fits := []store.InspoFit{
		fit(nil, []string{"green", "cream"}, []string{"jacket", "knit"}, "", 0, 0),
		fit(nil, []string{"green"}, []string{"jacket"}, "", 0, 0),
		fit(nil, []string{"black"}, []string{"jacket"}, "", 0, 0), // wrong colour
		fit(nil, []string{"green"}, []string{"coat"}, "", 0, 0),   // wrong piece
	}
	item := store.Item{Subcategory: "jacket", ColorPrimary: "green"}
	if got := Matches(item, fits); got != 2 {
		t.Errorf("Matches = %d, want 2", got)
	}
}
