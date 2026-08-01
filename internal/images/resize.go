package images

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"

	"golang.org/x/image/draw"
)

// encodeResized scales img so its longest edge is maxDim — never
// upscaling smaller originals — and encodes the result as JPEG.
func encodeResized(img image.Image, maxDim int) ([]byte, error) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()

	dstW, dstH := w, h
	if longest := max(w, h); longest > maxDim {
		scale := float64(maxDim) / float64(longest)
		dstW = int(float64(w) * scale)
		dstH = int(float64(h) * scale)
	}

	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, fmt.Errorf("encode jpeg: %w", err)
	}
	return buf.Bytes(), nil
}
