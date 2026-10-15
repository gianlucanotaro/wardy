package store

import (
	"context"
	"testing"
)

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

func TestCandidateItemsFormalityRange(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")

	for formality := 1; formality <= 5; formality++ {
		tagItem(t, s, userID, formality, 3, []string{"summer"}, []string{"work"})
	}

	items, err := s.CandidateItems(ctx, userID, CandidateFilter{FormalityMin: 3, FormalityMax: 5})
	if err != nil {
		t.Fatalf("CandidateItems: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3 (formality 3,4,5 within range 3-5)", len(items))
	}
	for _, it := range items {
		if it.Formality < 3 {
			t.Errorf("item with formality %d is below the range 3-5", it.Formality)
		}
	}
}

// Occasion and season tags are preferences ranked in internal/suggest,
// never filters here: an item tagged only for another occasion and season
// still comes back.
func TestCandidateItemsIgnoresOccasionAndSeasonTags(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")

	tagItem(t, s, userID, 2, 2, []string{"winter"}, []string{"formal"})

	items, err := s.CandidateItems(ctx, userID, CandidateFilter{FormalityMin: 1, FormalityMax: 5})
	if err != nil {
		t.Fatalf("CandidateItems: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("got %d items, want the item regardless of its occasion/season tags", len(items))
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
		FormalityMin: 1, FormalityMax: 3,
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
		FormalityMin: 1, FormalityMax: 3,
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
		FormalityMin: 1, FormalityMax: 3,
	})
	if err != nil {
		t.Fatalf("CandidateItems: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("user B should not see user A's items, got %d", len(items))
	}
}
