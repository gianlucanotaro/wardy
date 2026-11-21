package store

import (
	"context"
	"testing"

	"github.com/gianlucanotaro/wardy/internal/images"
)

func TestMaskBoxIsKeptWithTheMaskAndClearedWithIt(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	user, err := s.CreateUser(ctx, "a@example.com", "hash", "A")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	id, err := s.CreateItem(ctx, user, "photo.jpg", "thumb.jpg")
	if err != nil {
		t.Fatalf("create item: %v", err)
	}

	item, _ := s.ItemByID(ctx, user, id)
	if item.MaskBox.Valid {
		t.Fatalf("a new item has a box: %+v", item.MaskBox)
	}

	want := images.Box{X: 0.1, Y: 0.2, W: 0.5, H: 0.6, Ratio: 0.75}
	if _, err := s.SetItemMask(ctx, user, id, "1/a_mask.png", MaybeBox{Box: want, Valid: true}); err != nil {
		t.Fatalf("set mask: %v", err)
	}
	item, _ = s.ItemByID(ctx, user, id)
	if !item.MaskBox.Valid || item.MaskBox.Box != want {
		t.Errorf("got %+v, want %+v", item.MaskBox, want)
	}
	all, err := s.ItemsByUser(ctx, user, false)
	if err != nil || len(all) != 1 || all[0].MaskBox.Box != want {
		t.Errorf("the list does not carry the box: %+v, %v", all, err)
	}

	if _, err := s.SetItemMask(ctx, user, id, "", MaybeBox{}); err != nil {
		t.Fatalf("remove mask: %v", err)
	}
	item, _ = s.ItemByID(ctx, user, id)
	if item.MaskPath != "" || item.MaskBox.Valid {
		t.Errorf("removing the cut-out left %q / %+v", item.MaskPath, item.MaskBox)
	}
}

func TestItemsNeedingMaskBoxAndTheGuardOnTheirMask(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	user, _ := s.CreateUser(ctx, "a@example.com", "hash", "A")
	plain, _ := s.CreateItem(ctx, user, "p.jpg", "pt.jpg")
	old, _ := s.CreateItem(ctx, user, "o.jpg", "ot.jpg")
	done, _ := s.CreateItem(ctx, user, "d.jpg", "dt.jpg")
	s.SetItemMask(ctx, user, old, "1/old_mask.png", MaybeBox{})
	s.SetItemMask(ctx, user, done, "1/done_mask.png", MaybeBox{Box: images.Box{W: 1, H: 1, Ratio: 1}, Valid: true})

	refs, err := s.ItemsNeedingMaskBox(ctx)
	if err != nil {
		t.Fatalf("ItemsNeedingMaskBox: %v", err)
	}
	if len(refs) != 1 || refs[0].ID != old || refs[0].MaskPath != "1/old_mask.png" {
		t.Fatalf("got %+v, want only the cut-out without a box (item %d; %d has no cut-out)", refs, old, plain)
	}

	// the mask was replaced while the old one was being measured: the old
	// one's box must not land on the new one
	s.SetItemMask(ctx, user, old, "1/new_mask.png", MaybeBox{})
	if err := s.SetItemMaskBox(ctx, old, "1/old_mask.png", MaybeBox{Box: images.Box{W: 0.5, H: 0.5, Ratio: 1}, Valid: true}); err != nil {
		t.Fatalf("SetItemMaskBox: %v", err)
	}
	if item, _ := s.ItemByID(ctx, user, old); item.MaskBox.Valid {
		t.Errorf("a stale measurement was written to the new mask: %+v", item.MaskBox)
	}
	if err := s.SetItemMaskBox(ctx, old, "1/new_mask.png", MaybeBox{Box: images.Box{W: 0.5, H: 0.5, Ratio: 1}, Valid: true}); err != nil {
		t.Fatalf("SetItemMaskBox: %v", err)
	}
	if item, _ := s.ItemByID(ctx, user, old); !item.MaskBox.Valid {
		t.Errorf("the box of the current mask was not written")
	}
}
