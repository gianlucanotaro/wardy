// Package images turns an uploaded photo into two orientation-correct,
// EXIF-free JPEGs: a display size and a thumbnail. Both are freshly encoded
// from decoded pixels, never a copy of the original bytes, so stripping
// EXIF (including GPS) is a side effect of the pipeline rather than a
// separate step that could be forgotten.
package images

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
)

const (
	// DisplayMaxDim is the longest edge, in pixels, of the full-size
	// derivative shown on the item detail screen.
	DisplayMaxDim = 1600
	// ThumbMaxDim is the longest edge, in pixels, of the thumbnail used in
	// grid views.
	ThumbMaxDim = 400
	jpegQuality = 85
)

// Process decodes an uploaded photo, corrects its orientation per the EXIF
// tag, and returns a display-sized JPEG and a thumbnail JPEG.
func Process(data []byte) (display, thumb []byte, err error) {
	if IsHEIC(data) {
		return nil, nil, ErrHEIC
	}

	orientation := readOrientation(data)

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, nil, fmt.Errorf("decode image: %w", err)
	}
	img = applyOrientation(img, orientation)

	display, err = encodeResized(img, DisplayMaxDim)
	if err != nil {
		return nil, nil, fmt.Errorf("encode display: %w", err)
	}

	thumb, err = encodeResized(img, ThumbMaxDim)
	if err != nil {
		return nil, nil, fmt.Errorf("encode thumb: %w", err)
	}

	return display, thumb, nil
}
