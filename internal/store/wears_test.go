package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

var wearDate = time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)

func TestLogWearAdHocItems(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	itemID, _ := s.CreateItem(ctx, userID, "photo.jpg", "thumb.jpg")

	wearID, err := s.LogWear(ctx, userID, nil, []int64{itemID}, wearDate, "casual", "comfy")
	if err != nil {
		t.Fatalf("LogWear: %v", err)
	}

	wear, err := s.WearByID(ctx, userID, wearID)
	if err != nil {
		t.Fatalf("WearByID: %v", err)
	}
	if wear.WornOn != "2026-07-15" || wear.Occasion != "casual" || wear.Notes != "comfy" {
		t.Errorf("unexpected wear: %+v", wear)
	}
	if wear.LookID.Valid {
		t.Errorf("LookID = %+v, want invalid (ad-hoc wear)", wear.LookID)
	}
	if len(wear.Items) != 1 || wear.Items[0].ID != itemID {
		t.Errorf("Items = %+v, want just %d", wear.Items, itemID)
	}
}

func TestLogWearFromLook(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	itemID, _ := s.CreateItem(ctx, userID, "photo.jpg", "thumb.jpg")
	lookID, _ := s.CreateLook(ctx, userID, "Look", "", []int64{itemID})

	wearID, err := s.LogWear(ctx, userID, &lookID, []int64{itemID}, wearDate, "", "")
	if err != nil {
		t.Fatalf("LogWear: %v", err)
	}

	wear, err := s.WearByID(ctx, userID, wearID)
	if err != nil {
		t.Fatalf("WearByID: %v", err)
	}
	if !wear.LookID.Valid || wear.LookID.Int64 != lookID {
		t.Errorf("LookID = %+v, want %d", wear.LookID, lookID)
	}
}

func TestLogWearRejectsItemFromAnotherUser(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userA, _ := s.CreateUser(ctx, "a@example.com", "hash", "A")
	userB, _ := s.CreateUser(ctx, "b@example.com", "hash", "B")
	itemA, _ := s.CreateItem(ctx, userA, "photo.jpg", "thumb.jpg")

	if _, err := s.LogWear(ctx, userB, nil, []int64{itemA}, wearDate, "", ""); !errors.Is(err, ErrNotFound) {
		t.Errorf("LogWear with another user's item = %v, want ErrNotFound", err)
	}

	wears, err := s.WearsByUser(ctx, userB)
	if err != nil {
		t.Fatalf("WearsByUser: %v", err)
	}
	if len(wears) != 0 {
		t.Errorf("expected the failed LogWear to leave no wear behind, got %d", len(wears))
	}
}

func TestLogWearRejectsLookFromAnotherUser(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userA, _ := s.CreateUser(ctx, "a@example.com", "hash", "A")
	userB, _ := s.CreateUser(ctx, "b@example.com", "hash", "B")
	itemA, _ := s.CreateItem(ctx, userA, "a.jpg", "a_thumb.jpg")
	itemB, _ := s.CreateItem(ctx, userB, "b.jpg", "b_thumb.jpg")
	lookA, _ := s.CreateLook(ctx, userA, "A's look", "", []int64{itemA})

	if _, err := s.LogWear(ctx, userB, &lookA, []int64{itemB}, wearDate, "", ""); !errors.Is(err, ErrNotFound) {
		t.Errorf("LogWear with another user's look = %v, want ErrNotFound", err)
	}
}

func TestLogWearRejectsNonexistentLook(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	itemID, _ := s.CreateItem(ctx, userID, "photo.jpg", "thumb.jpg")

	bogusLook := int64(999999)
	if _, err := s.LogWear(ctx, userID, &bogusLook, []int64{itemID}, wearDate, "", ""); !errors.Is(err, ErrNotFound) {
		t.Errorf("LogWear with nonexistent look id = %v, want ErrNotFound (not a raw FK error)", err)
	}
}

func TestWearOwnership(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userA, _ := s.CreateUser(ctx, "a@example.com", "hash", "A")
	userB, _ := s.CreateUser(ctx, "b@example.com", "hash", "B")
	itemID, _ := s.CreateItem(ctx, userA, "photo.jpg", "thumb.jpg")
	wearID, err := s.LogWear(ctx, userA, nil, []int64{itemID}, wearDate, "", "")
	if err != nil {
		t.Fatalf("LogWear: %v", err)
	}

	t.Run("read", func(t *testing.T) {
		if _, err := s.WearByID(ctx, userB, wearID); !errors.Is(err, ErrNotFound) {
			t.Errorf("WearByID as user B = %v, want ErrNotFound", err)
		}
	})

	t.Run("list", func(t *testing.T) {
		wears, err := s.WearsByUser(ctx, userB)
		if err != nil {
			t.Fatalf("WearsByUser: %v", err)
		}
		if len(wears) != 0 {
			t.Errorf("WearsByUser(userB) = %d wears, want 0", len(wears))
		}
	})

	t.Run("delete", func(t *testing.T) {
		if err := s.DeleteWear(ctx, userB, wearID); !errors.Is(err, ErrNotFound) {
			t.Errorf("DeleteWear as user B = %v, want ErrNotFound", err)
		}
	})
}

func TestDeleteWearCascadesWearItemsButNotLook(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	itemID, _ := s.CreateItem(ctx, userID, "photo.jpg", "thumb.jpg")
	lookID, _ := s.CreateLook(ctx, userID, "Look", "", []int64{itemID})
	wearID, _ := s.LogWear(ctx, userID, &lookID, []int64{itemID}, wearDate, "", "")

	if err := s.DeleteWear(ctx, userID, wearID); err != nil {
		t.Fatalf("DeleteWear: %v", err)
	}

	if _, err := s.WearByID(ctx, userID, wearID); !errors.Is(err, ErrNotFound) {
		t.Errorf("WearByID after delete = %v, want ErrNotFound", err)
	}
	// The look and item must both survive — deleting a wear log entry
	// must not touch the look it referenced or the items themselves.
	if _, err := s.LookByID(ctx, userID, lookID); err != nil {
		t.Errorf("look should survive its wear being deleted, got: %v", err)
	}
	if _, err := s.ItemByID(ctx, userID, itemID); err != nil {
		t.Errorf("item should survive its wear being deleted, got: %v", err)
	}
}

func TestWearsByUserOrdersNewestWornFirst(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	itemID, _ := s.CreateItem(ctx, userID, "photo.jpg", "thumb.jpg")

	earlier := wearDate
	later := wearDate.AddDate(0, 0, 5)
	earlierID, _ := s.LogWear(ctx, userID, nil, []int64{itemID}, earlier, "", "")
	laterID, _ := s.LogWear(ctx, userID, nil, []int64{itemID}, later, "", "")

	wears, err := s.WearsByUser(ctx, userID)
	if err != nil {
		t.Fatalf("WearsByUser: %v", err)
	}
	if len(wears) != 2 {
		t.Fatalf("got %d wears, want 2", len(wears))
	}
	if wears[0].ID != laterID || wears[1].ID != earlierID {
		t.Errorf("wrong order: got ids %d, %d, want most-recently-worn (%d) first", wears[0].ID, wears[1].ID, laterID)
	}
}

func TestItemsByLastWorn(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")

	wornRecently, _ := s.CreateItem(ctx, userID, "recent.jpg", "recent_thumb.jpg")
	wornLongAgo, _ := s.CreateItem(ctx, userID, "old.jpg", "old_thumb.jpg")
	neverWorn, _ := s.CreateItem(ctx, userID, "new.jpg", "new_thumb.jpg")

	if _, err := s.LogWear(ctx, userID, nil, []int64{wornLongAgo}, wearDate, "", ""); err != nil {
		t.Fatalf("LogWear: %v", err)
	}
	if _, err := s.LogWear(ctx, userID, nil, []int64{wornRecently}, wearDate.AddDate(0, 0, 10), "", ""); err != nil {
		t.Fatalf("LogWear: %v", err)
	}

	items, err := s.ItemsByLastWorn(ctx, userID)
	if err != nil {
		t.Fatalf("ItemsByLastWorn: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}

	// Never-worn first, then oldest-worn, then most-recently-worn last.
	if items[0].ID != neverWorn || items[0].LastWornOn.Valid {
		t.Errorf("items[0] = %+v, want the never-worn item with an invalid LastWornOn", items[0])
	}
	if items[1].ID != wornLongAgo || items[1].LastWornOn.String != "2026-07-15" {
		t.Errorf("items[1] = %+v, want the item worn 2026-07-15", items[1])
	}
	if items[2].ID != wornRecently || items[2].LastWornOn.String != "2026-07-25" {
		t.Errorf("items[2] = %+v, want the item worn 2026-07-25", items[2])
	}
}
