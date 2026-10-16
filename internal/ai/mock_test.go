package ai

import (
	"context"
	"strings"
	"testing"
)

// The dev mock puts garments named in the request first, so the request
// can be tried without the real model.
func TestMockSuggestOutfitsFollowsRequest(t *testing.T) {
	candidates := []SuggestionCandidate{
		{ID: 1, Category: "top", Name: "White tee", Subcategory: "t-shirt", ColorPrimary: "white"},
		{ID: 2, Category: "top", Name: "Blue oxford", Subcategory: "shirt", ColorPrimary: "blue"},
		{ID: 3, Category: "bottom", Name: "Jeans", Subcategory: "jeans", ColorPrimary: "blue"},
		{ID: 4, Category: "outerwear", Name: "Denim jacket", Subcategory: "jacket", ColorPrimary: "blue"},
		{ID: 5, Category: "outerwear", Name: "Navy blazer", Subcategory: "blazer", ColorPrimary: "navy"},
	}

	got, _, err := mockProvider{}.SuggestOutfits(context.Background(), candidates, "date", SuggestContext{Prompt: "The NAVY blazer, and an oxford"})
	if err != nil {
		t.Fatalf("SuggestOutfits: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("no outfits")
	}
	first := got[0].ItemIDs
	for _, want := range []int64{2, 5} {
		if !containsID(first, want) {
			t.Errorf("first outfit %v lacks requested item %d", first, want)
		}
	}
	if !strings.Contains(got[0].Rationale, "navy blazer") || !strings.Contains(got[0].Rationale, "blue oxford") {
		t.Errorf("rationale doesn't name what it found: %q", got[0].Rationale)
	}

	plain, _, err := mockProvider{}.SuggestOutfits(context.Background(), candidates, "date", SuggestContext{})
	if err != nil {
		t.Fatalf("SuggestOutfits: %v", err)
	}
	if plain[0].ItemIDs[0] != 1 {
		t.Errorf("without a request the order should be untouched, got %v", plain[0].ItemIDs)
	}
}

func containsID(ids []int64, id int64) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}
