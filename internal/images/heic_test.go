package images

import "testing"

func TestIsHEIC(t *testing.T) {
	ftyp := func(brand string) []byte {
		return append([]byte{0, 0, 0, 0x20, 'f', 't', 'y', 'p'}, []byte(brand)...)
	}

	cases := []struct {
		name string
		data []byte
		want bool
	}{
		{"heic brand", ftyp("heic"), true},
		{"heix brand", ftyp("heix"), true},
		{"mif1 brand", ftyp("mif1"), true},
		{"plain mp4 brand", ftyp("mp42"), false},
		{"jpeg signature", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0, 0, 0, 0, 0, 0, 0}, false},
		{"png signature", []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0, 0, 0, 0}, false},
		{"too short", []byte{0, 0, 0, 0x20, 'f', 't'}, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsHEIC(c.data); got != c.want {
				t.Errorf("IsHEIC(%q) = %v, want %v", c.name, got, c.want)
			}
		})
	}
}
