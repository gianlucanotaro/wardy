package store

import (
	"context"
	"testing"
)

func strPtr(s string) *string { return &s }

// tagItem is a test helper: creates an item and immediately tags it with
// the given attributes via UpdateItemTags, so candidate-query tests can
// build a fixture wardrobe without going through the HTTP/taxonomy layers.
func tagItem(t *testing.T, s *Store, userID int64, formality, warmth int, seasons, occasions []string) int64 {
	t.Helper()
	ctx := context.Background()

	itemID, err := s.CreateItem(ctx, userID, "photo.jpg", "thumb.jpg")
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	if err := s.UpdateItemTags(ctx, userID, itemID, ItemTags{
		Category: "top", Subcategory: "t-shirt", ColorPrimary: "blue",
		Pattern: "solid", Material: "cotton", Fit: "regular",
		Formality: formality, Warmth: warmth, Seasons: seasons, Occasions: occasions,
	}); err != nil {
		t.Fatalf("UpdateItemTags: %v", err)
	}

	return itemID
}

func TestCandidateItemsMatchesOccasionAndSeason(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")

	casualSummer := tagItem(t, s, userID, 2, 2, []string{"summer"}, []string{"casual"})
	tagItem(t, s, userID, 2, 2, []string{"winter"}, []string{"casual"}) // wrong season
	tagItem(t, s, userID, 2, 2, []string{"summer"}, []string{"formal"}) // wrong occasion

	items, err := s.CandidateItems(ctx, userID, CandidateFilter{
		Occasion: strPtr("casual"), Season: strPtr("summer"),
		TargetFormality: 2, FormalityWindow: 1,
	})
	if err != nil {
		t.Fatalf("CandidateItems: %v", err)
	}
	if len(items) != 1 || items[0].ID != casualSummer {
		t.Errorf("got %d items, want exactly the casual+summer item; got %+v", len(items), items)
	}
}

func TestCandidateItemsFormalityWindow(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")

	for formality := 1; formality <= 5; formality++ {
		tagItem(t, s, userID, formality, 3, []string{"summer"}, []string{"work"})
	}

	items, err := s.CandidateItems(ctx, userID, CandidateFilter{
		Occasion: strPtr("work"), Season: strPtr("summer"),
		TargetFormality: 3, FormalityWindow: 1,
	})
	if err != nil {
		t.Fatalf("CandidateItems: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3 (formality 2,3,4 within window of target 3)", len(items))
	}
	for _, it := range items {
		if it.Formality < 2 || it.Formality > 4 {
			t.Errorf("item with formality %d should be outside the ±1 window of 3", it.Formality)
		}
	}
}

func TestCandidateItemsWarmthNilSkipsFilter(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")

	for warmth := 1; warmth <= 5; warmth++ {
		tagItem(t, s, userID, 2, warmth, []string{"summer"}, []string{"casual"})
	}

	items, err := s.CandidateItems(ctx, userID, CandidateFilter{
		Occasion: strPtr("casual"), Season: strPtr("summer"),
		TargetFormality: 2, FormalityWindow: 1,
		TargetWarmth: nil, // weather unavailable — degrade gracefully, don't filter
	})
	if err != nil {
		t.Fatalf("CandidateItems: %v", err)
	}
	if len(items) != 5 {
		t.Errorf("got %d items, want all 5 (warmth filter should be skipped when nil)", len(items))
	}
}

func TestCandidateItemsWarmthWindow(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")

	for warmth := 1; warmth <= 5; warmth++ {
		tagItem(t, s, userID, 2, warmth, []string{"summer"}, []string{"casual"})
	}

	target := 3
	items, err := s.CandidateItems(ctx, userID, CandidateFilter{
		Occasion: strPtr("casual"), Season: strPtr("summer"),
		TargetFormality: 2, FormalityWindow: 1,
		TargetWarmth: &target, WarmthWindow: 1,
	})
	if err != nil {
		t.Fatalf("CandidateItems: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3 (warmth 2,3,4 within window of target 3)", len(items))
	}
}

func TestCandidateItemsExcludesArchived(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")

	itemID := tagItem(t, s, userID, 2, 2, []string{"summer"}, []string{"casual"})
	if _, err := s.db.ExecContext(ctx, `UPDATE items SET archived = 1 WHERE id = ?`, itemID); err != nil {
		t.Fatalf("archive item: %v", err)
	}

	items, err := s.CandidateItems(ctx, userID, CandidateFilter{
		Occasion: strPtr("casual"), Season: strPtr("summer"),
		TargetFormality: 2, FormalityWindow: 1,
	})
	if err != nil {
		t.Fatalf("CandidateItems: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("archived item should never be a candidate, got %d items", len(items))
	}
}

func TestCandidateItemsExcludesRecentlyWorn(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")

	wornRecently := tagItem(t, s, userID, 2, 2, []string{"summer"}, []string{"casual"})
	neverWorn := tagItem(t, s, userID, 2, 2, []string{"summer"}, []string{"casual"})

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO wears (user_id, worn_on) VALUES (?, date('now'))`, userID)
	if err != nil {
		t.Fatalf("insert wear: %v", err)
	}
	wearID, _ := res.LastInsertId()
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO wear_items (wear_id, item_id) VALUES (?, ?)`, wearID, wornRecently); err != nil {
		t.Fatalf("insert wear_items: %v", err)
	}

	items, err := s.CandidateItems(ctx, userID, CandidateFilter{
		Occasion: strPtr("casual"), Season: strPtr("summer"),
		TargetFormality: 2, FormalityWindow: 1,
		ExcludeWornSinceDays: 7,
	})
	if err != nil {
		t.Fatalf("CandidateItems: %v", err)
	}
	if len(items) != 1 || items[0].ID != neverWorn {
		t.Errorf("expected only the never-worn item, got %+v", items)
	}
}

func TestCandidateItemsOwnershipScoped(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userA, _ := s.CreateUser(ctx, "a@example.com", "hash", "A")
	userB, _ := s.CreateUser(ctx, "b@example.com", "hash", "B")

	tagItem(t, s, userA, 2, 2, []string{"summer"}, []string{"casual"})

	items, err := s.CandidateItems(ctx, userB, CandidateFilter{
		Occasion: strPtr("casual"), Season: strPtr("summer"),
		TargetFormality: 2, FormalityWindow: 1,
	})
	if err != nil {
		t.Fatalf("CandidateItems: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("user B should not see user A's items, got %d", len(items))
	}
}
