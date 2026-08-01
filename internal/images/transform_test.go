package images

import (
	"image"
	"image/color"
	"testing"
)

// A 2x3 fixture with a distinct color per pixel, labeled by position:
//
//	A B
//	C D
//	E F
func newLabeledFixture() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 3))
	img.Set(0, 0, color.NRGBA{10, 0, 0, 255}) // A
	img.Set(1, 0, color.NRGBA{20, 0, 0, 255}) // B
	img.Set(0, 1, color.NRGBA{30, 0, 0, 255}) // C
	img.Set(1, 1, color.NRGBA{40, 0, 0, 255}) // D
	img.Set(0, 2, color.NRGBA{50, 0, 0, 255}) // E
	img.Set(1, 2, color.NRGBA{60, 0, 0, 255}) // F
	return img
}

func at(img image.Image, x, y int) color.NRGBA {
	return color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
}

func TestFlipH(t *testing.T) {
	src := newLabeledFixture()
	got := flipH(src)

	want := [][2]color.NRGBA{
		{at(src, 1, 0), at(got, 0, 0)}, // B -> (0,0)
		{at(src, 0, 0), at(got, 1, 0)}, // A -> (1,0)
		{at(src, 1, 2), at(got, 0, 2)}, // F -> (0,2)
	}
	for i, w := range want {
		if w[0] != w[1] {
			t.Errorf("flipH case %d: got %v, want %v", i, w[1], w[0])
		}
	}
}

func TestFlipV(t *testing.T) {
	src := newLabeledFixture()
	got := flipV(src)

	if at(got, 0, 0) != at(src, 0, 2) { // row0 <- row2 (E)
		t.Errorf("flipV: (0,0) = %v, want E %v", at(got, 0, 0), at(src, 0, 2))
	}
	if at(got, 0, 2) != at(src, 0, 0) { // row2 <- row0 (A)
		t.Errorf("flipV: (0,2) = %v, want A %v", at(got, 0, 2), at(src, 0, 0))
	}
	if at(got, 1, 1) != at(src, 1, 1) { // middle row unchanged
		t.Errorf("flipV: (1,1) = %v, want D %v", at(got, 1, 1), at(src, 1, 1))
	}
}

func TestRotate180(t *testing.T) {
	src := newLabeledFixture()
	got := rotate180(src)

	if at(got, 0, 0) != at(src, 1, 2) { // F -> (0,0)
		t.Errorf("rotate180: (0,0) = %v, want F %v", at(got, 0, 0), at(src, 1, 2))
	}
	if at(got, 1, 2) != at(src, 0, 0) { // A -> (1,2)
		t.Errorf("rotate180: (1,2) = %v, want A %v", at(got, 1, 2), at(src, 0, 0))
	}
}

func TestRotate90(t *testing.T) {
	src := newLabeledFixture() // 2 wide x 3 tall
	got := rotate90(src)       // expect 3 wide x 2 tall

	b := got.Bounds()
	if b.Dx() != 3 || b.Dy() != 2 {
		t.Fatalf("rotate90: got dims %dx%d, want 3x2", b.Dx(), b.Dy())
	}

	// Expected grid:
	// row0: E C A
	// row1: F D B
	expect := map[[2]int]color.NRGBA{
		{0, 0}: at(src, 0, 2), // E
		{1, 0}: at(src, 0, 1), // C
		{2, 0}: at(src, 0, 0), // A
		{0, 1}: at(src, 1, 2), // F
		{1, 1}: at(src, 1, 1), // D
		{2, 1}: at(src, 1, 0), // B
	}
	for pos, want := range expect {
		if got := at(got, pos[0], pos[1]); got != want {
			t.Errorf("rotate90: (%d,%d) = %v, want %v", pos[0], pos[1], got, want)
		}
	}
}

func TestRotate270(t *testing.T) {
	src := newLabeledFixture() // 2 wide x 3 tall
	got := rotate270(src)      // expect 3 wide x 2 tall

	b := got.Bounds()
	if b.Dx() != 3 || b.Dy() != 2 {
		t.Fatalf("rotate270: got dims %dx%d, want 3x2", b.Dx(), b.Dy())
	}

	// Expected grid:
	// row0: B D F
	// row1: A C E
	expect := map[[2]int]color.NRGBA{
		{0, 0}: at(src, 1, 0), // B
		{1, 0}: at(src, 1, 1), // D
		{2, 0}: at(src, 1, 2), // F
		{0, 1}: at(src, 0, 0), // A
		{1, 1}: at(src, 0, 1), // C
		{2, 1}: at(src, 0, 2), // E
	}
	for pos, want := range expect {
		if got := at(got, pos[0], pos[1]); got != want {
			t.Errorf("rotate270: (%d,%d) = %v, want %v", pos[0], pos[1], got, want)
		}
	}
}
