package images

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"

	"golang.org/x/image/draw"
)

const (
	// MaskMaxDim is the longest edge, in pixels, of a stored mask. The
	// browser scales a mask to the photo's size, so it can be much smaller
	// than the photo: a mask is a few tens of KB where a cut-out PNG of the
	// photo itself would be megabytes.
	MaskMaxDim = 1000
	// MaxMaskBytes caps an uploaded mask.
	MaxMaskBytes = 3 << 20

	maskMinDim    = 16
	maskMaxInDim  = 4096 // longest edge accepted, before shrinking
	maskShapeSlop = 0.02 // how far the mask's shape may differ from the photo's
)

// ErrBadMask is returned for a mask that is not a usable cut-out of its photo.
var ErrBadMask = errors.New("invalid mask")

// ProcessMask checks a mask a phone made for a photo of photoW x photoH
// pixels and returns it as a PNG, at most MaskMaxDim on its longest edge.
//
// The mask is the PNG's alpha channel: opaque where the garment is,
// transparent where the background is, which is what a CSS mask reads. The
// colour is dropped (a stored mask is black), which also keeps it small.
//
// It is user input like the photo: only PNG is decoded, the size is checked
// before any pixels are, it must have the photo's shape or it would cut the
// wrong garment out, and a mask that is all one value is no mask.
func ProcessMask(data []byte, photoW, photoH int) ([]byte, error) {
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: not a PNG", ErrBadMask)
	}
	if cfg.Width < maskMinDim || cfg.Height < maskMinDim || max(cfg.Width, cfg.Height) > maskMaxInDim {
		return nil, fmt.Errorf("%w: %dx%d is out of range", ErrBadMask, cfg.Width, cfg.Height)
	}
	if photoW <= 0 || photoH <= 0 {
		return nil, fmt.Errorf("%w: photo has no size", ErrBadMask)
	}
	maskRatio := float64(cfg.Width) / float64(cfg.Height)
	photoRatio := float64(photoW) / float64(photoH)
	if math.Abs(maskRatio-photoRatio)/photoRatio > maskShapeSlop {
		return nil, fmt.Errorf("%w: shape %dx%d does not match the photo's %dx%d", ErrBadMask, cfg.Width, cfg.Height, photoW, photoH)
	}

	src, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadMask, err)
	}

	// the alpha channel on its own
	b := src.Bounds()
	alpha := image.NewAlpha(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			_, _, _, a := src.At(b.Min.X+x, b.Min.Y+y).RGBA()
			alpha.SetAlpha(x, y, color.Alpha{A: uint8(a >> 8)})
		}
	}

	if longest := max(b.Dx(), b.Dy()); longest > MaskMaxDim {
		scale := float64(MaskMaxDim) / float64(longest)
		small := image.NewAlpha(image.Rect(0, 0, max(1, int(float64(b.Dx())*scale)), max(1, int(float64(b.Dy())*scale))))
		draw.CatmullRom.Scale(small, small.Bounds(), alpha, alpha.Bounds(), draw.Src, nil)
		alpha = small
	}

	lo, hi := uint8(255), uint8(0)
	for _, a := range alpha.Pix {
		lo, hi = min(lo, a), max(hi, a)
	}
	if lo == hi {
		return nil, fmt.Errorf("%w: every pixel is the same", ErrBadMask)
	}

	out := image.NewNRGBA(alpha.Bounds())
	for i, a := range alpha.Pix {
		out.Pix[4*i+3] = a
	}
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, out); err != nil {
		return nil, fmt.Errorf("encode mask: %w", err)
	}
	return buf.Bytes(), nil
}
