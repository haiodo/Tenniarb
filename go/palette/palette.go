// Package palette reads a colour the way the app stores one: a name from the markdown palette, or 6 or 8
// hex digits, as plain sRGB. A background is written so, and so is the colour of an item in the inspector.
package palette

import (
	"strings"
	"unicode/utf8"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/markdown"
)

// Parse reads a colour name or 6/8 hex digits, with or without the leading #. It is sRGB as written: a
// background takes no Generic RGB conversion, the one an item's fill takes.
func Parse(s string) (gfx.Color, bool) {
	hex := strings.ReplaceAll(strings.TrimSpace(s), "#", "")
	if h, ok := markdown.ColorNames[strings.ToLower(hex)]; ok {
		hex = strings.ReplaceAll(h, "#", "")
	}
	rgb, ok := scanHex(hex)
	if !ok {
		return gfx.Color{}, false
	}
	switch utf8.RuneCountInString(hex) {
	case 6:
		return gfx.Color{R: float64(rgb>>16&0xff) / 255, G: float64(rgb>>8&0xff) / 255, B: float64(rgb&0xff) / 255, A: 1}, true
	case 8:
		return gfx.Color{R: float64(rgb>>24&0xff) / 255, G: float64(rgb>>16&0xff) / 255, B: float64(rgb>>8&0xff) / 255, A: float64(rgb&0xff) / 255}, true
	}
	return gfx.Color{}, false
}

// scanHex is the digits of a hex number, false when one is not a hex digit or there is no digit at all.
func scanHex(s string) (uint64, bool) {
	if s == "" {
		return 0, false
	}
	var v uint64
	for i := 0; i < len(s); i++ {
		b := s[i]
		d := uint64(0)
		switch {
		case b >= '0' && b <= '9':
			d = uint64(b - '0')
		case b >= 'a' && b <= 'f':
			d = uint64(b-'a') + 10
		case b >= 'A' && b <= 'F':
			d = uint64(b-'A') + 10
		default:
			return 0, false
		}
		v = v<<4 | d
	}
	return v, true
}
