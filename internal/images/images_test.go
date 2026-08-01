package images

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"testing"

	"github.com/rwcarlsen/goexif/exif"
)

// cornerRect returns a small sampling rectangle inset from one corner of an
// image with the given bounds, expressed as fractions so it scales with
// whatever size the pipeline actually produced.
func cornerRect(b image.Rectangle, right, bottom bool) image.Rectangle {
	w, h := b.Dx(), b.Dy()
	x0, x1 := int(0.05*float64(w)), int(0.35*float64(w))
	y0, y1 := int(0.05*float64(h)), int(0.35*float64(h))
	if right {
		x0, x1 = w-x1, w-x0
	}
	if bottom {
		y0, y1 = h-y1, h-y0
	}
	return image.Rect(b.Min.X+x0, b.Min.Y+y0, b.Min.X+x1, b.Min.Y+y1)
}

func decodeJPEG(t *testing.T, data []byte) image.Image {
	t.Helper()
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode output jpeg: %v", err)
	}
	return img
}

func TestProcess_RejectsHEIC(t *testing.T) {
	heic := append([]byte{0, 0, 0, 0x20}, []byte("ftypheic")...)
	heic = append(heic, make([]byte, 16)...) // pad past the 12-byte sniff window

	_, _, err := Process(heic)
	if !errors.Is(err, ErrHEIC) {
		t.Fatalf("Process(HEIC) error = %v, want ErrHEIC", err)
	}
}

func TestProcess_StripsEXIF(t *testing.T) {
	base := encodeJPEG(t, newQuadrantImage(80, 120))
	tagged := withOrientation(base, 6)

	display, thumb, err := Process(tagged)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	for name, out := range map[string][]byte{"display": display, "thumb": thumb} {
		if bytes.Contains(out, []byte("Exif")) {
			t.Errorf("%s: output bytes still contain an Exif marker", name)
		}
		if _, err := exif.Decode(bytes.NewReader(out)); err == nil {
			t.Errorf("%s: exif.Decode succeeded on output, want no EXIF present", name)
		}
	}
}

func TestProcess_Sizes(t *testing.T) {
	t.Run("downscales a large image on its longest edge", func(t *testing.T) {
		base := encodeJPEG(t, newQuadrantImage(3000, 2000))
		display, thumb, err := Process(base)
		if err != nil {
			t.Fatalf("Process: %v", err)
		}

		d := decodeJPEG(t, display).Bounds()
		if d.Dx() != DisplayMaxDim {
			t.Errorf("display width = %d, want %d", d.Dx(), DisplayMaxDim)
		}
		wantH := 2000 * DisplayMaxDim / 3000
		if diff := d.Dy() - wantH; diff < -1 || diff > 1 {
			t.Errorf("display height = %d, want ~%d", d.Dy(), wantH)
		}

		th := decodeJPEG(t, thumb).Bounds()
		if th.Dx() != ThumbMaxDim {
			t.Errorf("thumb width = %d, want %d", th.Dx(), ThumbMaxDim)
		}
	})

	t.Run("never upscales a small image", func(t *testing.T) {
		base := encodeJPEG(t, newQuadrantImage(100, 80))
		display, thumb, err := Process(base)
		if err != nil {
			t.Fatalf("Process: %v", err)
		}

		if b := decodeJPEG(t, display).Bounds(); b.Dx() != 100 || b.Dy() != 80 {
			t.Errorf("display dims = %dx%d, want 100x80 (no upscale)", b.Dx(), b.Dy())
		}
		if b := decodeJPEG(t, thumb).Bounds(); b.Dx() != 100 || b.Dy() != 80 {
			t.Errorf("thumb dims = %dx%d, want 100x80 (no upscale)", b.Dx(), b.Dy())
		}
	})
}

func TestProcess_AppliesOrientation(t *testing.T) {
	// Source quadrants (80 wide x 120 tall): TL=red TR=green BL=blue BR=yellow.
	// Orientation 6 (rotate90 CW) should land: TL=blue TR=red BR=green BL=yellow.
	base := encodeJPEG(t, newQuadrantImage(80, 120))
	tagged := withOrientation(base, 6)

	display, _, err := Process(tagged)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	out := decodeJPEG(t, display)
	b := out.Bounds()

	const tolerance = 60
	check := func(name string, right, bottom bool, want color.NRGBA) {
		got := avgColor(out, cornerRect(b, right, bottom))
		if colorDist(got, want) > tolerance {
			t.Errorf("%s corner = %v, want ~%v", name, got, want)
		}
	}

	check("top-left", false, false, color.NRGBA{0, 0, 255, 255})     // blue
	check("top-right", true, false, color.NRGBA{255, 0, 0, 255})     // red
	check("bottom-left", false, true, color.NRGBA{255, 255, 0, 255}) // yellow
	check("bottom-right", true, true, color.NRGBA{0, 255, 0, 255})   // green
}
