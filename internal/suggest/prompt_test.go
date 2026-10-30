package suggest

import (
	"slices"
	"strings"
	"testing"

	"github.com/gianlucanotaro/wardy/internal/store"
)

func TestCleanPrompt(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "empty", raw: "", want: ""},
		{name: "only whitespace", raw: " \n\t ", want: ""},
		{name: "plain", raw: "something comfy for a train ride", want: "something comfy for a train ride"},
		{name: "trims and collapses", raw: "  smart   but\nnot stiff  ", want: "smart but not stiff"},
		{name: "control characters become spaces", raw: "green\x00jacket\x1b[2J please", want: "green jacket [2J please"},
		{name: "keeps accents and emoji", raw: "élégant 🎉", want: "élégant 🎉"},
		{name: "at the cap", raw: strings.Repeat("é", MaxPromptLength), want: strings.Repeat("é", MaxPromptLength)},
		{name: "cap counts after collapsing", raw: strings.Repeat("a", MaxPromptLength) + "\n\n\n", want: strings.Repeat("a", MaxPromptLength)},
		{name: "over the cap", raw: strings.Repeat("a", MaxPromptLength+1), wantErr: true},
		{name: "invalid utf-8", raw: "abc\xff", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CleanPrompt(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("CleanPrompt(%q) = %q, want an error", tt.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("CleanPrompt(%q): %v", tt.raw, err)
			}
			if got != tt.want {
				t.Errorf("CleanPrompt(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestPreferRequested_RanksNamedPiecesFirst(t *testing.T) {
	items := []store.Item{
		{ID: 1, Category: "top", Subcategory: "t-shirt", ColorPrimary: "blue"},
		{ID: 2, Category: "bottom", Subcategory: "skirt", ColorPrimary: "red"},
		{ID: 3, Category: "top", Subcategory: "sweater", ColorPrimary: "red"},
		{ID: 4, Category: "shoes", Subcategory: "dress-shoes", ColorPrimary: "black"},
		{ID: 5, Category: "top", Subcategory: "blouse", ColorPrimary: "white", Name: "Lucky blouse"},
	}
	preferRequested(items, "Something with a RED top, maybe my lucky one")
	var got []int64
	for _, it := range items {
		got = append(got, it.ID)
	}
	// Red top (colour + category) and lucky blouse (category + name) score
	// 2, blue tee and red skirt 1, shoes 0; ties keep their order.
	want := []int64{3, 5, 1, 2, 4}
	if !slices.Equal(got, want) {
		t.Errorf("order %v, want %v", got, want)
	}
}

func TestPreferRequested_EmptyPromptKeepsOrder(t *testing.T) {
	items := []store.Item{{ID: 1, ColorPrimary: "red"}, {ID: 2, ColorPrimary: "blue"}}
	preferRequested(items, "")
	if items[0].ID != 1 || items[1].ID != 2 {
		t.Errorf("order changed: %+v", items)
	}
}

func TestPreferRequested_WholeTagValues(t *testing.T) {
	// "dress" must not match "dress-shoes"; "shoes" matches "shoe" requests.
	items := []store.Item{
		{ID: 1, Category: "shoes", Subcategory: "dress-shoes"},
		{ID: 2, Category: "dress", Subcategory: "casual-dress"},
	}
	preferRequested(items, "a dress")
	if items[0].ID != 2 {
		t.Errorf("dress request ranked %+v first", items[0])
	}
	preferRequested(items, "comfy shoe")
	if items[0].ID != 1 {
		t.Errorf("shoe request ranked %+v first", items[0])
	}
}
