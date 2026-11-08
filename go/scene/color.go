package scene

import (
	"strconv"
	"strings"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/markdown"
)

// parseColor reads a name from the cmkdown table, hex digits, 6 (RGB) or 8 (RGBA), with an optional
// "#", or the channels of an array of numbers: the forms PreferencesController.parseColor takes.
func parseColor(s string) (gfx.Color, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "none" || s == "transparent" { // no fill, no border: what alpha 0 draws anyway
		return gfx.Color{}, true
	}
	if strings.Contains(s, ",") { // a colour array of an expression, as text
		return colorChannels(s)
	}
	// The cmkdown table maps "#00000000" itself to "": such an entry is not a name.
	if hex, ok := markdown.ColorNames[s]; ok && hex != "" {
		s = hex
	}
	s = strings.ReplaceAll(s, "#", "")
	if len(s) != 6 && len(s) != 8 {
		return gfx.Color{}, false
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return gfx.Color{}, false
	}
	if len(s) == 6 {
		v = v<<8 | 0xff
	}
	ch := func(shift uint) float64 { return float64(v>>shift&0xff) / 255 }
	return gfx.Color{R: ch(24), G: ch(16), B: ch(8), A: ch(0)}, true
}

// colorChannels reads "r,g,b" or "r,g,b,a" with channels 0..255: the text go/render gives for a colour
// array of an expression. r,g,b is opaque.
func colorChannels(s string) (gfx.Color, bool) {
	parts := strings.Split(s, ",")
	if len(parts) != 3 && len(parts) != 4 {
		return gfx.Color{}, false
	}
	var v [4]float64
	for i, part := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil {
			return gfx.Color{}, false
		}
		v[i] = f / 255
	}
	if len(parts) == 3 {
		v[3] = 1
	}
	return gfx.Color{R: v[0], G: v[1], B: v[2], A: v[3]}, true
}

// contrast is getTextColorBasedOn: black on a light fill, white on a dark one.
func contrast(c gfx.Color) gfx.Color {
	sum := (c.R*255*299 + c.G*255*587 + c.B*255*114) / 1000
	if sum+0.5 >= 129 {
		return gfx.Color{A: 1}
	}
	return gfx.Color{R: 1, G: 1, B: 1, A: 1}
}
