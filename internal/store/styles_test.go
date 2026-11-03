package store

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func tagStyles(t *testing.T, s *Store, userID, itemID int64, styles []string) {
	t.Helper()
	if err := s.UpdateItemTags(context.Background(), userID, itemID, ItemTags{
		Category: "top", Subcategory: "t-shirt", ColorPrimary: "blue",
		Pattern: "solid", Material: "cotton", Fit: "regular",
		FormalityMin: 2, FormalityMax: 2, WarmthMin: 3, WarmthMax: 3,
		Styles: styles,
	}); err != nil {
		t.Fatalf("UpdateItemTags: %v", err)
	}
}

func TestUpdateItemAIStylesFillsOnlyEmptyStyles(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	plain, _ := s.CreateItem(ctx, userID, "a.jpg", "a_t.jpg")
	edited, _ := s.CreateItem(ctx, userID, "b.jpg", "b_t.jpg")
	tagStyles(t, s, userID, edited, []string{"classic"})

	if applied, err := s.UpdateItemAIStyles(ctx, plain, []string{"minimal", "sporty"}); err != nil || !applied {
		t.Fatalf("UpdateItemAIStyles(plain) = %v, %v; want applied", applied, err)
	}
	// The user's own styles survive the backfill.
	if applied, err := s.UpdateItemAIStyles(ctx, edited, []string{"edgy"}); err != nil || applied {
		t.Fatalf("UpdateItemAIStyles(edited) = %v, %v; want skipped", applied, err)
	}

	got, _ := s.ItemByID(ctx, userID, plain)
	if !reflect.DeepEqual([]string(got.Styles), []string{"minimal", "sporty"}) {
		t.Errorf("plain styles = %v", got.Styles)
	}
	got, _ = s.ItemByID(ctx, userID, edited)
	if !reflect.DeepEqual([]string(got.Styles), []string{"classic"}) {
		t.Errorf("edited styles = %v, want [classic]", got.Styles)
	}
}

func TestItemsNeedingStylesOnlyListsUnlookedEmptyItems(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	waiting, _ := s.CreateItem(ctx, userID, "a.jpg", "a_t.jpg")
	backfilled, _ := s.CreateItem(ctx, userID, "b.jpg", "b_t.jpg")
	handTagged, _ := s.CreateItem(ctx, userID, "c.jpg", "c_t.jpg")
	tagStyles(t, s, userID, handTagged, []string{"classic"})
	if _, err := s.UpdateItemAIStyles(ctx, backfilled, []string{"minimal"}); err != nil {
		t.Fatal(err)
	}

	refs, err := s.ItemsNeedingStyles(ctx)
	if err != nil {
		t.Fatalf("ItemsNeedingStyles: %v", err)
	}
	if len(refs) != 1 || refs[0] != (ItemRef{ID: waiting, UserID: userID}) {
		t.Errorf("refs = %+v, want only item %d", refs, waiting)
	}
}

func TestItemStyleLists(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	me, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	other, _ := s.CreateUser(ctx, "you@example.com", "hash", "You")

	tee, _ := s.CreateItem(ctx, me, "a.jpg", "a_t.jpg")
	tagStyles(t, s, me, tee, []string{"minimal"})
	hoodie, _ := s.CreateItem(ctx, me, "b.jpg", "b_t.jpg")
	tagStyles(t, s, me, hoodie, []string{"streetwear", "sporty"})
	archived, _ := s.CreateItem(ctx, me, "c.jpg", "c_t.jpg")
	tagStyles(t, s, me, archived, []string{"vintage"})
	if err := s.ArchiveItem(ctx, me, archived, true); err != nil {
		t.Fatal(err)
	}
	theirs, _ := s.CreateItem(ctx, other, "d.jpg", "d_t.jpg")
	tagStyles(t, s, other, theirs, []string{"edgy"})

	now := time.Now()
	// The hoodie twice in the window, the tee only long ago.
	for _, w := range []struct {
		items []int64
		on    time.Time
	}{
		{[]int64{hoodie}, now.AddDate(0, 0, -2)},
		{[]int64{hoodie}, now.AddDate(0, 0, -10)},
		{[]int64{tee}, now.AddDate(0, 0, -200)},
	} {
		if _, err := s.LogWear(ctx, me, nil, w.items, w.on, "", ""); err != nil {
			t.Fatalf("LogWear: %v", err)
		}
	}
	if _, err := s.LogWear(ctx, other, nil, []int64{theirs}, now, "", ""); err != nil {
		t.Fatalf("LogWear: %v", err)
	}

	owned, worn, err := s.ItemStyleLists(ctx, me, now.AddDate(0, 0, -90).Format(time.DateOnly))
	if err != nil {
		t.Fatalf("ItemStyleLists: %v", err)
	}
	if len(owned) != 2 {
		t.Errorf("owned = %v, want the tee and hoodie only (not archived, not another user's)", owned)
	}
	want := [][]string{{"streetwear", "sporty"}, {"streetwear", "sporty"}}
	if !reflect.DeepEqual(worn, want) {
		t.Errorf("worn = %v, want the hoodie twice", worn)
	}
}
