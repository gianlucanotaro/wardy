package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// TestItemOwnership verifies user A's items are completely invisible to
// user B's requests — the failure mode CLAUDE.md calls out as silent (a
// leaked wardrobe throws no error), so every access path gets checked.
func TestItemOwnership(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	userA, err := s.CreateUser(ctx, "a@example.com", "hash", "User A")
	if err != nil {
		t.Fatalf("create user A: %v", err)
	}
	userB, err := s.CreateUser(ctx, "b@example.com", "hash", "User B")
	if err != nil {
		t.Fatalf("create user B: %v", err)
	}

	itemID, err := s.CreateItem(ctx, userA, "photo.jpg", "thumb.jpg")
	if err != nil {
		t.Fatalf("create item: %v", err)
	}

	t.Run("read", func(t *testing.T) {
		if _, err := s.ItemByID(ctx, userB, itemID); !errors.Is(err, ErrNotFound) {
			t.Errorf("ItemByID as user B = %v, want ErrNotFound", err)
		}
		if _, err := s.ItemByID(ctx, userA, itemID); err != nil {
			t.Errorf("ItemByID as owner = %v, want no error", err)
		}
	})

	t.Run("list", func(t *testing.T) {
		items, err := s.ItemsByUser(ctx, userB, false)
		if err != nil {
			t.Fatalf("ItemsByUser: %v", err)
		}
		if len(items) != 0 {
			t.Errorf("ItemsByUser(userB) = %d items, want 0", len(items))
		}
	})

	t.Run("update", func(t *testing.T) {
		if err := s.UpdateItemName(ctx, userB, itemID, "hacked"); !errors.Is(err, ErrNotFound) {
			t.Errorf("UpdateItemName as user B = %v, want ErrNotFound", err)
		}
		item, err := s.ItemByID(ctx, userA, itemID)
		if err != nil {
			t.Fatalf("ItemByID as owner: %v", err)
		}
		if item.Name == "hacked" {
			t.Error("user B's update was applied despite not owning the item")
		}
	})

	t.Run("delete", func(t *testing.T) {
		if _, _, err := s.DeleteItem(ctx, userB, itemID); !errors.Is(err, ErrNotFound) {
			t.Errorf("DeleteItem as user B = %v, want ErrNotFound", err)
		}
		if _, err := s.ItemByID(ctx, userA, itemID); err != nil {
			t.Errorf("item should still exist for owner after user B's failed delete: %v", err)
		}
	})

	t.Run("archive", func(t *testing.T) {
		if err := s.ArchiveItem(ctx, userB, itemID, true); !errors.Is(err, ErrNotFound) {
			t.Errorf("ArchiveItem as user B = %v, want ErrNotFound", err)
		}
		item, err := s.ItemByID(ctx, userA, itemID)
		if err != nil {
			t.Fatalf("ItemByID as owner: %v", err)
		}
		if item.Archived {
			t.Error("user B's archive was applied despite not owning the item")
		}
	})
}

// TestArchiveItemFiltersList verifies an archived item drops out of the
// default (non-archived) list and only shows up when archived is
// explicitly requested — the whole point of ItemsByUser taking that
// parameter.
func TestArchiveItemFiltersList(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	userID, err := s.CreateUser(ctx, "a@example.com", "hash", "User A")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	itemID, err := s.CreateItem(ctx, userID, "photo.jpg", "thumb.jpg")
	if err != nil {
		t.Fatalf("create item: %v", err)
	}

	if err := s.ArchiveItem(ctx, userID, itemID, true); err != nil {
		t.Fatalf("archive item: %v", err)
	}

	active, err := s.ItemsByUser(ctx, userID, false)
	if err != nil {
		t.Fatalf("ItemsByUser(false): %v", err)
	}
	if len(active) != 0 {
		t.Errorf("ItemsByUser(false) = %d items, want 0 (item was archived)", len(active))
	}

	archived, err := s.ItemsByUser(ctx, userID, true)
	if err != nil {
		t.Fatalf("ItemsByUser(true): %v", err)
	}
	if len(archived) != 1 {
		t.Fatalf("ItemsByUser(true) = %d items, want 1", len(archived))
	}
	if archived[0].ID != itemID {
		t.Errorf("ItemsByUser(true)[0].ID = %d, want %d", archived[0].ID, itemID)
	}

	if err := s.ArchiveItem(ctx, userID, itemID, false); err != nil {
		t.Fatalf("unarchive item: %v", err)
	}
	active, err = s.ItemsByUser(ctx, userID, false)
	if err != nil {
		t.Fatalf("ItemsByUser(false) after unarchive: %v", err)
	}
	if len(active) != 1 {
		t.Errorf("ItemsByUser(false) after unarchive = %d items, want 1", len(active))
	}
}
