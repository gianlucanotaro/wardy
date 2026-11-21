package jobs

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/gianlucanotaro/wardy/internal/store"
)

func TestBackfillMeasuresCutOutsThatHaveNoBox(t *testing.T) {
	ctx := context.Background()
	s, photosDir, userID, _ := newTestFixture(t)

	// a stored mask: the garment is the middle half of a 200x100 photo
	m := image.NewNRGBA(image.Rect(0, 0, 200, 100))
	for y := 25; y < 75; y++ {
		for x := 50; x < 150; x++ {
			m.SetNRGBA(x, y, color.NRGBA{A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, m); err != nil {
		t.Fatal(err)
	}
	good, _ := s.CreateItem(ctx, userID, "1/g.jpg", "1/gt.jpg")
	missing, _ := s.CreateItem(ctx, userID, "1/m.jpg", "1/mt.jpg")
	if err := os.WriteFile(filepath.Join(photosDir, "1", "g_mask.png"), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	s.SetItemMask(ctx, userID, good, "1/g_mask.png", store.MaybeBox{})
	s.SetItemMask(ctx, userID, missing, "1/not_there_mask.png", store.MaybeBox{})

	n, err := BackfillMaskBoxes(ctx, s, photosDir)
	if err != nil {
		t.Fatalf("BackfillMaskBoxes: %v (a lost mask file must not stop the app starting)", err)
	}
	if n != 1 {
		t.Errorf("measured %d, want 1 (the other mask file is gone)", n)
	}

	item, _ := s.ItemByID(ctx, userID, good)
	b := item.MaskBox.Box
	if !item.MaskBox.Valid || b.X != 0.25 || b.Y != 0.25 || b.W != 0.5 || b.H != 0.5 || b.Ratio != 2 {
		t.Errorf("got %+v, want x .25 y .25 w .5 h .5 ratio 2", item.MaskBox)
	}
	if lost, _ := s.ItemByID(ctx, userID, missing); lost.MaskBox.Valid {
		t.Errorf("a mask that cannot be read got a box: %+v", lost.MaskBox)
	}

	// done is done: a later start finds only the one it could not read
	if n, err := BackfillMaskBoxes(ctx, s, photosDir); err != nil || n != 0 {
		t.Errorf("second run measured %d (err %v), want 0", n, err)
	}
}
