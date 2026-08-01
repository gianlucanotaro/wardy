package images

import (
	"bytes"
	"image"

	"github.com/rwcarlsen/goexif/exif"
)

// readOrientation returns the EXIF orientation tag (1-8), defaulting to 1
// (no transform needed) if the tag is missing or the file has no readable
// EXIF at all — PNGs and camera photos without EXIF are both common and
// not errors.
func readOrientation(data []byte) int {
	x, err := exif.Decode(bytes.NewReader(data))
	if err != nil {
		return 1
	}

	tag, err := x.Get(exif.Orientation)
	if err != nil {
		return 1
	}

	v, err := tag.Int(0)
	if err != nil || v < 1 || v > 8 {
		return 1
	}

	return v
}

// applyOrientation returns img rotated/flipped upright per the EXIF
// orientation spec. 1 is the identity case; 2-8 cover the eight standard
// combinations of rotation and mirroring that real cameras produce.
func applyOrientation(img image.Image, orientation int) image.Image {
	switch orientation {
	case 2:
		return flipH(img)
	case 3:
		return rotate180(img)
	case 4:
		return flipV(img)
	case 5:
		return flipH(rotate90(img))
	case 6:
		return rotate90(img)
	case 7:
		return flipH(rotate270(img))
	case 8:
		return rotate270(img)
	default:
		return img
	}
}
