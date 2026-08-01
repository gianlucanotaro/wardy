package images

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

// newQuadrantImage builds a w x h image split into four solid-color
// quadrants (top-left red, top-right green, bottom-left blue, bottom-right
// yellow), so orientation transforms can be checked by sampling corners.
func newQuadrantImage(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	colors := map[[2]bool]color.NRGBA{
		{false, false}: {255, 0, 0, 255},   // top-left: red
		{true, false}:  {0, 255, 0, 255},   // top-right: green
		{false, true}:  {0, 0, 255, 255},   // bottom-left: blue
		{true, true}:   {255, 255, 0, 255}, // bottom-right: yellow
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, colors[[2]bool{x >= w/2, y >= h/2}])
		}
	}
	return img
}

func encodeJPEG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatalf("encode fixture jpeg: %v", err)
	}
	return buf.Bytes()
}

// buildEXIFApp1 returns a complete JPEG APP1 marker segment (0xFFE1 + length
// + "Exif\0\0" + a minimal little-endian TIFF blob) declaring the given
// orientation tag, ready to be spliced right after a JPEG's SOI marker.
func buildEXIFApp1(orientation int) []byte {
	var tiff bytes.Buffer
	tiff.WriteString("II")                                   // little-endian
	binary.Write(&tiff, binary.LittleEndian, uint16(0x002A)) // TIFF magic
	binary.Write(&tiff, binary.LittleEndian, uint32(8))      // offset to IFD0

	binary.Write(&tiff, binary.LittleEndian, uint16(1))      // 1 IFD entry
	binary.Write(&tiff, binary.LittleEndian, uint16(0x0112)) // tag: Orientation
	binary.Write(&tiff, binary.LittleEndian, uint16(3))      // type: SHORT
	binary.Write(&tiff, binary.LittleEndian, uint32(1))      // count: 1
	binary.Write(&tiff, binary.LittleEndian, uint16(orientation))
	binary.Write(&tiff, binary.LittleEndian, uint16(0)) // padding to 4 bytes
	binary.Write(&tiff, binary.LittleEndian, uint32(0)) // next IFD: none

	payload := append([]byte("Exif\x00\x00"), tiff.Bytes()...)

	var seg bytes.Buffer
	seg.Write([]byte{0xFF, 0xE1})
	binary.Write(&seg, binary.BigEndian, uint16(len(payload)+2))
	seg.Write(payload)
	return seg.Bytes()
}

// withOrientation returns baseJPEG with an EXIF APP1 segment declaring
// orientation spliced in right after the SOI marker.
func withOrientation(baseJPEG []byte, orientation int) []byte {
	out := make([]byte, 0, len(baseJPEG)+64)
	out = append(out, baseJPEG[:2]...)
	out = append(out, buildEXIFApp1(orientation)...)
	out = append(out, baseJPEG[2:]...)
	return out
}

func avgColor(img image.Image, r image.Rectangle) color.NRGBA {
	var rs, gs, bs, n int
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			rs += int(c.R)
			gs += int(c.G)
			bs += int(c.B)
			n++
		}
	}
	if n == 0 {
		return color.NRGBA{}
	}
	return color.NRGBA{R: uint8(rs / n), G: uint8(gs / n), B: uint8(bs / n), A: 255}
}

func colorDist(a, b color.NRGBA) int {
	d := func(x, y uint8) int {
		if x > y {
			return int(x - y)
		}
		return int(y - x)
	}
	return d(a.R, b.R) + d(a.G, b.G) + d(a.B, b.B)
}
