package palette_test

import (
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/palette"
)

// Parse is the app's colour reading, moved out of go/render: a background or a colour typed into the
// inspector reads the same as before, so the values are spelled out here rather than borrowed from it.
func TestParse(t *testing.T) {
	six := func(r, g, b float64) gfx.Color { return gfx.Color{R: r / 255, G: g / 255, B: b / 255, A: 1} }
	cases := []struct {
		in   string
		want gfx.Color
		ok   bool
	}{
		{"#ffffff", six(255, 255, 255), true},
		{"FFFFFF", six(255, 255, 255), true},
		{"  #ff0000  ", six(255, 0, 0), true},
		{"#123456", six(0x12, 0x34, 0x56), true},
		{"#abcdef", six(0xab, 0xcd, 0xef), true},
		{"#12345678", gfx.Color{R: 0x12 / 255.0, G: 0x34 / 255.0, B: 0x56 / 255.0, A: 0x78 / 255.0}, true},
		{"white", six(255, 255, 255), true},
		{"black", six(0, 0, 0), true},
		{"red", six(255, 0, 0), true},
		{"", gfx.Color{}, false},
		{"#", gfx.Color{}, false},
		{"zzz", gfx.Color{}, false},
		{"12345", gfx.Color{}, false},
		{"#1234567", gfx.Color{}, false},
	}
	for _, c := range cases {
		got, ok := palette.Parse(c.in)
		if ok != c.ok {
			t.Errorf("%q: ok %v, want %v", c.in, ok, c.ok)
			continue
		}
		if ok && got != c.want {
			t.Errorf("%q: %+v, want %+v", c.in, got, c.want)
		}
	}
}
