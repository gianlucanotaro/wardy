package store

import (
	"context"
	"errors"
	"testing"
)

func TestCreateAndGetLook(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")

	topID, _ := s.CreateItem(ctx, userID, "top.jpg", "top_thumb.jpg")
	bottomID, _ := s.CreateItem(ctx, userID, "bottom.jpg", "bottom_thumb.jpg")

	lookID, err := s.CreateLook(ctx, userID, "Casual look", "notes here", []int64{topID, bottomID})
	if err != nil {
		t.Fatalf("CreateLook: %v", err)
	}

	look, err := s.LookByID(ctx, userID, lookID)
	if err != nil {
		t.Fatalf("LookByID: %v", err)
	}
	if look.Name != "Casual look" || look.Notes != "notes here" {
		t.Errorf("unexpected look: %+v", look)
	}
	if !look.CoverItemID.Valid || look.CoverItemID.Int64 != topID {
		t.Errorf("CoverItemID = %+v, want the first item (%d)", look.CoverItemID, topID)
	}
	if len(look.Items) != 2 || look.Items[0].ID != topID || look.Items[1].ID != bottomID {
		t.Errorf("Items in wrong order or count: %+v", look.Items)
	}
}

func TestCreateLookRejectsItemFromAnotherUser(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userA, _ := s.CreateUser(ctx, "a@example.com", "hash", "A")
	userB, _ := s.CreateUser(ctx, "b@example.com", "hash", "B")

	itemA, _ := s.CreateItem(ctx, userA, "photo.jpg", "thumb.jpg")

	if _, err := s.CreateLook(ctx, userB, "Borrowed", "", []int64{itemA}); !errors.Is(err, ErrNotFound) {
		t.Errorf("CreateLook with another user's item = %v, want ErrNotFound", err)
	}

	// The rejected insert must not leave a half-created look behind.
	looks, err := s.LooksByUser(ctx, userB)
	if err != nil {
		t.Fatalf("LooksByUser: %v", err)
	}
	if len(looks) != 0 {
		t.Errorf("expected the failed CreateLook to leave no look behind, got %d", len(looks))
	}
}

func TestLookOwnership(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userA, _ := s.CreateUser(ctx, "a@example.com", "hash", "A")
	userB, _ := s.CreateUser(ctx, "b@example.com", "hash", "B")

	itemID, _ := s.CreateItem(ctx, userA, "photo.jpg", "thumb.jpg")
	lookID, err := s.CreateLook(ctx, userA, "Mine", "", []int64{itemID})
	if err != nil {
		t.Fatalf("CreateLook: %v", err)
	}

	t.Run("read", func(t *testing.T) {
		if _, err := s.LookByID(ctx, userB, lookID); !errors.Is(err, ErrNotFound) {
			t.Errorf("LookByID as user B = %v, want ErrNotFound", err)
		}
	})

	t.Run("list", func(t *testing.T) {
		looks, err := s.LooksByUser(ctx, userB)
		if err != nil {
			t.Fatalf("LooksByUser: %v", err)
		}
		if len(looks) != 0 {
			t.Errorf("LooksByUser(userB) = %d looks, want 0", len(looks))
		}
	})

	t.Run("update", func(t *testing.T) {
		if err := s.UpdateLook(ctx, userB, lookID, "Stolen", ""); !errors.Is(err, ErrNotFound) {
			t.Errorf("UpdateLook as user B = %v, want ErrNotFound", err)
		}
	})

	t.Run("replace items", func(t *testing.T) {
		if err := s.ReplaceLookItems(ctx, userB, lookID, []int64{itemID}); !errors.Is(err, ErrNotFound) {
			t.Errorf("ReplaceLookItems as user B = %v, want ErrNotFound", err)
		}
	})

	t.Run("delete", func(t *testing.T) {
		if err := s.DeleteLook(ctx, userB, lookID); !errors.Is(err, ErrNotFound) {
			t.Errorf("DeleteLook as user B = %v, want ErrNotFound", err)
		}
	})
}

func TestUpdateLookRenames(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	itemID, _ := s.CreateItem(ctx, userID, "photo.jpg", "thumb.jpg")
	lookID, _ := s.CreateLook(ctx, userID, "Old name", "old notes", []int64{itemID})

	if err := s.UpdateLook(ctx, userID, lookID, "New name", "new notes"); err != nil {
		t.Fatalf("UpdateLook: %v", err)
	}

	look, err := s.LookByID(ctx, userID, lookID)
	if err != nil {
		t.Fatalf("LookByID: %v", err)
	}
	if look.Name != "New name" || look.Notes != "new notes" {
		t.Errorf("rename didn't apply: %+v", look)
	}
	// Renaming must not touch the items.
	if len(look.Items) != 1 || look.Items[0].ID != itemID {
		t.Errorf("items changed after rename: %+v", look.Items)
	}
}

func TestReplaceLookItemsRecomputesCover(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	itemA, _ := s.CreateItem(ctx, userID, "a.jpg", "a_thumb.jpg")
	itemB, _ := s.CreateItem(ctx, userID, "b.jpg", "b_thumb.jpg")
	itemC, _ := s.CreateItem(ctx, userID, "c.jpg", "c_thumb.jpg")

	lookID, _ := s.CreateLook(ctx, userID, "Look", "", []int64{itemA, itemB})

	if err := s.ReplaceLookItems(ctx, userID, lookID, []int64{itemC}); err != nil {
		t.Fatalf("ReplaceLookItems: %v", err)
	}

	look, err := s.LookByID(ctx, userID, lookID)
	if err != nil {
		t.Fatalf("LookByID: %v", err)
	}
	if len(look.Items) != 1 || look.Items[0].ID != itemC {
		t.Errorf("items not replaced: %+v", look.Items)
	}
	if !look.CoverItemID.Valid || look.CoverItemID.Int64 != itemC {
		t.Errorf("cover not recomputed: %+v", look.CoverItemID)
	}
}

func TestReplaceLookItemsRejectsItemFromAnotherUser(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userA, _ := s.CreateUser(ctx, "a@example.com", "hash", "A")
	userB, _ := s.CreateUser(ctx, "b@example.com", "hash", "B")

	itemA1, _ := s.CreateItem(ctx, userA, "a1.jpg", "a1_thumb.jpg")
	itemA2, _ := s.CreateItem(ctx, userA, "a2.jpg", "a2_thumb.jpg")
	itemB, _ := s.CreateItem(ctx, userB, "b.jpg", "b_thumb.jpg")

	lookID, _ := s.CreateLook(ctx, userA, "Look", "", []int64{itemA1})

	if err := s.ReplaceLookItems(ctx, userA, lookID, []int64{itemB}); !errors.Is(err, ErrNotFound) {
		t.Errorf("ReplaceLookItems with another user's item = %v, want ErrNotFound", err)
	}

	// The rejected replace must not have cleared the original items.
	look, err := s.LookByID(ctx, userA, lookID)
	if err != nil {
		t.Fatalf("LookByID: %v", err)
	}
	if len(look.Items) != 1 || look.Items[0].ID != itemA1 {
		t.Errorf("items changed despite the rejected replace: %+v", look.Items)
	}
	_ = itemA2
}

func TestDeleteLookCascadesLookItems(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	itemID, _ := s.CreateItem(ctx, userID, "photo.jpg", "thumb.jpg")
	lookID, _ := s.CreateLook(ctx, userID, "Look", "", []int64{itemID})

	if err := s.DeleteLook(ctx, userID, lookID); err != nil {
		t.Fatalf("DeleteLook: %v", err)
	}

	if _, err := s.LookByID(ctx, userID, lookID); !errors.Is(err, ErrNotFound) {
		t.Errorf("LookByID after delete = %v, want ErrNotFound", err)
	}

	// The item itself must survive — deleting a look must not delete its
	// items.
	if _, err := s.ItemByID(ctx, userID, itemID); err != nil {
		t.Errorf("item should survive its look being deleted, got: %v", err)
	}
}

func TestLooksByUserOrdersNewestFirst(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	userID, _ := s.CreateUser(ctx, "me@example.com", "hash", "Me")
	itemID, _ := s.CreateItem(ctx, userID, "photo.jpg", "thumb.jpg")

	firstID, _ := s.CreateLook(ctx, userID, "First", "", []int64{itemID})
	secondID, _ := s.CreateLook(ctx, userID, "Second", "", []int64{itemID})

	looks, err := s.LooksByUser(ctx, userID)
	if err != nil {
		t.Fatalf("LooksByUser: %v", err)
	}
	if len(looks) != 2 {
		t.Fatalf("got %d looks, want 2", len(looks))
	}
	if looks[0].ID != secondID || looks[1].ID != firstID {
		t.Errorf("wrong order: got ids %d, %d, want newest (%d) first", looks[0].ID, looks[1].ID, secondID)
	}
}
