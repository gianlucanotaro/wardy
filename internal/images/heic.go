package images

import (
	"bytes"
	"errors"
)

// ErrHEIC is returned when the uploaded file is HEIC/HEIF. Pure-Go HEIC
// decoding isn't viable (see docs/backend.md), so callers should surface a
// clear "convert to JPEG" error rather than attempt to process it.
var ErrHEIC = errors.New("HEIC/HEIF images are not supported; please convert to JPEG")

// heicBrands are the ISO base media file format "major brand" values used
// by HEIC/HEIF containers.
var heicBrands = [][]byte{
	[]byte("heic"), []byte("heix"), []byte("hevc"), []byte("hevx"),
	[]byte("heim"), []byte("heis"), []byte("hevm"), []byte("hevs"),
	[]byte("mif1"), []byte("msf1"),
}

// IsHEIC sniffs the ISOBMFF "ftyp" box for a HEIC/HEIF major brand,
// independent of whatever Content-Type the browser sent — library picks on
// iOS can hand over raw HEIC regardless of what the form field claims.
func IsHEIC(data []byte) bool {
	if len(data) < 12 || !bytes.Equal(data[4:8], []byte("ftyp")) {
		return false
	}
	brand := data[8:12]
	for _, b := range heicBrands {
		if bytes.Equal(brand, b) {
			return true
		}
	}
	return false
}
