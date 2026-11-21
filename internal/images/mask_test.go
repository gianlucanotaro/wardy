package images

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func pngBytes(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// a mask: opaque block (the garment) on a transparent background, w x h
func maskFixture(w, h int) *image.NRGBA {
	m := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := h / 4; y < h*3/4; y++ {
		for x := w / 4; x < w*3/4; x++ {
			m.SetNRGBA(x, y, color.NRGBA{R: 200, G: 30, B: 90, A: 255})
		}
	}
	return m
}

func TestProcessMaskKeepsTheAlphaOfTheSameShape(t *testing.T) {
	out, err := ProcessMask(pngBytes(t, maskFixture(400, 300)), 1600, 1200)
	if err != nil {
		t.Fatalf("ProcessMask: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("result is not a PNG: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 400 || b.Dy() != 300 {
		t.Errorf("size changed to %dx%d, want 400x300 (smaller than the cap)", b.Dx(), b.Dy())
	}
	if _, _, _, a := img.At(200, 150).RGBA(); a>>8 != 255 {
		t.Errorf("the garment's alpha is %d in the middle, want 255", a>>8)
	}
	if _, _, _, a := img.At(5, 5).RGBA(); a != 0 {
		t.Errorf("the background's alpha is %d in the corner, want 0", a>>8)
	}
	if r, g, bl, _ := img.(*image.NRGBA).NRGBAAt(200, 150).RGBA(); r|g|bl != 0 {
		t.Errorf("colour is kept (%d,%d,%d); a stored mask is black", r, g, bl)
	}
}

func TestProcessMaskShrinksToTheCap(t *testing.T) {
	out, err := ProcessMask(pngBytes(t, maskFixture(3000, 2000)), 1500, 1000)
	if err != nil {
		t.Fatalf("ProcessMask: %v", err)
	}
	cfg, _ := png.DecodeConfig(bytes.NewReader(out))
	if cfg.Width != MaskMaxDim || cfg.Height != 666 {
		t.Errorf("got %dx%d, want %dx666", cfg.Width, cfg.Height, MaskMaxDim)
	}
}

func TestProcessMaskRejectsWhatIsNotAUsableMask(t *testing.T) {
	var jpg bytes.Buffer
	jpeg.Encode(&jpg, image.NewGray(image.Rect(0, 0, 100, 100)), nil)

	for name, tc := range map[string]struct {
		data           []byte
		photoW, photoH int
	}{
		"not a PNG":          {[]byte("not an image"), 100, 100},
		"a JPEG":             {jpg.Bytes(), 100, 100},
		"empty":              {nil, 100, 100},
		"wrong shape":        {pngBytes(t, maskFixture(100, 100)), 200, 100},
		"no cut-out at all":  {pngBytes(t, image.NewNRGBA(image.Rect(0, 0, 100, 100))), 100, 100},
		"opaque everywhere":  {pngBytes(t, image.NewGray(image.Rect(0, 0, 100, 100))), 100, 100},
		"too small":          {pngBytes(t, maskFixture(8, 8)), 8, 8},
		"too wide":           {pngBytes(t, image.NewNRGBA(image.Rect(0, 0, 5000, 20))), 5000, 20},
		"photo without size": {pngBytes(t, maskFixture(100, 100)), 0, 0},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ProcessMask(tc.data, tc.photoW, tc.photoH); !errors.Is(err, ErrBadMask) {
				t.Errorf("got %v, want ErrBadMask", err)
			}
		})
	}
}

func TestMeasureMaskFindsTheGarmentsBox(t *testing.T) {
	// maskFixture keeps the middle: a quarter in from every side
	out, err := ProcessMask(pngBytes(t, maskFixture(400, 300)), 400, 300)
	if err != nil {
		t.Fatalf("ProcessMask: %v", err)
	}
	box, err := MeasureMask(out)
	if err != nil {
		t.Fatalf("MeasureMask: %v", err)
	}
	near := func(name string, got, want float64) {
		if got < want-0.01 || got > want+0.01 {
			t.Errorf("%s = %.3f, want %.3f", name, got, want)
		}
	}
	near("x", box.X, 0.25)
	near("y", box.Y, 0.25)
	near("w", box.W, 0.5)
	near("h", box.H, 0.5)
	near("ratio", box.Ratio, 400.0/300.0)
}

func TestMeasureMaskRefusesWhatIsNotAMask(t *testing.T) {
	if _, err := MeasureMask([]byte("nope")); !errors.Is(err, ErrBadMask) {
		t.Errorf("garbage: got %v, want ErrBadMask", err)
	}
	if _, err := MeasureMask(pngBytes(t, image.NewNRGBA(image.Rect(0, 0, 50, 50)))); !errors.Is(err, ErrBadMask) {
		t.Errorf("nothing kept: got %v, want ErrBadMask", err)
	}
}
