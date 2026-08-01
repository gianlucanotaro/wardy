package images

import (
	"image"
	"testing"
)

func TestReadOrientation(t *testing.T) {
	base := encodeJPEG(t, newQuadrantImage(8, 8))

	for tag := 1; tag <= 8; tag++ {
		data := withOrientation(base, tag)
		if got := readOrientation(data); got != tag {
			t.Errorf("tag %d: readOrientation = %d", tag, got)
		}
	}
}

func TestReadOrientation_NoEXIF(t *testing.T) {
	base := encodeJPEG(t, newQuadrantImage(8, 8))
	if got := readOrientation(base); got != 1 {
		t.Errorf("no EXIF: readOrientation = %d, want 1 (default)", got)
	}
}

func TestApplyOrientation_Identity(t *testing.T) {
	src := newLabeledFixture()
	got := applyOrientation(src, 1)
	if got != image.Image(src) {
		t.Errorf("orientation 1 should return the image unchanged")
	}
}

func TestApplyOrientation_UnknownFallsBackToIdentity(t *testing.T) {
	src := newLabeledFixture()
	got := applyOrientation(src, 0)
	if got != image.Image(src) {
		t.Errorf("unknown orientation should fall back to unchanged")
	}
}

// Cases 5 and 7 combine a flip with a rotation; verify the dispatch wires
// them up as flipH(rotate90(x)) and flipH(rotate270(x)) respectively, since
// the individual primitives are already covered in transform_test.go.
func TestApplyOrientation_Combined(t *testing.T) {
	src := newLabeledFixture()

	got5 := applyOrientation(src, 5)
	want5 := flipH(rotate90(src))
	if !imagesEqual(got5, want5) {
		t.Errorf("orientation 5 did not match flipH(rotate90(x))")
	}

	got7 := applyOrientation(src, 7)
	want7 := flipH(rotate270(src))
	if !imagesEqual(got7, want7) {
		t.Errorf("orientation 7 did not match flipH(rotate270(x))")
	}
}

func imagesEqual(a, b image.Image) bool {
	if a.Bounds() != b.Bounds() {
		return false
	}
	r := a.Bounds()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if at(a, x, y) != at(b, x, y) {
				return false
			}
		}
	}
	return true
}
