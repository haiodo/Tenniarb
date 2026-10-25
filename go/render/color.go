package render

import (
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/markdown"
)

// Color is CGColor as RGBA in 0...1. Unlike markdown.ParseColor, parseColor here is the app's
// PreferencesController.parseColor: it can fail and takes only 6 or 8 hex digits.
type Color struct{ R, G, B, A float64 }

var (
	colorBlack = Color{0, 0, 0, 1}
	colorWhite = Color{1, 1, 1, 1}
)

func fromMarkdown(c markdown.RGBA) Color { return Color{c.R, c.G, c.B, c.A} }

// CGColor(red:green:blue:alpha:) is Generic RGB (gamma 1.8, own primaries); Swift draws it into an sRGB
// context, which converts. Rows are Generic RGB primaries in linear sRGB, measured with
// CGColor.converted(to: extendedLinearSRGB).
var genericToLinearSRGB = [3][3]float64{
	{1.025241732597351, -0.026545243337750435, 0.0013035525334998965},
	{0.019400764256715775, 0.9480088353157043, 0.032590351998806},
	{-0.0017621302977204323, -0.001434493693523109, 1.0031965970993042},
}

func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }

func srgbEncode(v float64) float64 {
	if v <= 0.0031308 {
		return 12.92 * v
	}
	return 1.055*math.Pow(v, 1/2.4) - 0.055
}

// genericToSRGB gives the sRGB channels (0...1) Swift ends up with for a Generic RGB colour.
func genericToSRGB(c Color) [3]float64 {
	lin := [3]float64{math.Pow(clamp01(c.R), 1.8), math.Pow(clamp01(c.G), 1.8), math.Pow(clamp01(c.B), 1.8)}
	var out [3]float64
	for i, row := range genericToLinearSRGB {
		out[i] = srgbEncode(clamp01(row[0]*lin[0] + row[1]*lin[1] + row[2]*lin[2]))
	}
	return out
}

// gfxColor is the colour as the Swift bitmap shows it. gfx colours are quantised to 8 bits per channel.
func (c Color) gfxColor() gfx.Color {
	s := genericToSRGB(c)
	return gfx.Color{R: s[0], G: s[1], B: s[2], A: clamp01(c.A)}
}

func withAlpha(c Color, a float64) Color { c.A = a; return c }

// swiftRound is Swift round(): half away from zero.
func swiftRound(x float64) float64 { return math.Round(x) }

func textColorBasedOn(c Color) Color {
	sum := swiftRound((c.R*255*299 + c.G*255*587 + c.B*255*114) / 1000)
	if sum > 128 {
		return colorBlack
	}
	return colorWhite
}

func isHexDigit(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F'
}

// scanHexInt64 is Foundation Scanner.scanHexInt64: skips leading whitespace, an optional 0x, then takes
// the leading run of hex digits (saturating on overflow).
func scanHexInt64(s string) (uint64, bool) {
	s = strings.TrimLeft(s, "\t\n\v\f\r ")
	if len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		s = s[2:]
	}
	end := 0
	for end < len(s) && isHexDigit(s[end]) {
		end++
	}
	if end == 0 {
		return 0, false
	}
	v, _ := strconv.ParseUint(s[:end], 16, 64)
	return v, true
}

// parseColor is PreferencesController.parseColor: a name from the cmkdown table or 6/8 hex digits with
// optional "#". Eight digits carry their own alpha.
func parseColor(hex string, alpha float64) (Color, bool) {
	sanitized := strings.ReplaceAll(strings.TrimSpace(hex), "#", "")
	if h, ok := markdown.ColorNames[strings.ToLower(sanitized)]; ok {
		sanitized = strings.ReplaceAll(h, "#", "")
	}
	rgb, ok := scanHexInt64(sanitized)
	if !ok {
		return Color{}, false
	}
	switch utf8.RuneCountInString(sanitized) {
	case 6:
		return Color{float64(rgb>>16&0xff) / 255, float64(rgb>>8&0xff) / 255, float64(rgb&0xff) / 255, alpha}, true
	case 8:
		return Color{float64(rgb>>24&0xff) / 255, float64(rgb>>16&0xff) / 255, float64(rgb>>8&0xff) / 255, float64(rgb&0xff) / 255}, true
	}
	return Color{}, false
}

// ParseColor reads a colour name or 6/8 hex digits as plain sRGB, for backgrounds (no Generic RGB conversion).
func ParseColor(s string) (gfx.Color, bool) {
	c, ok := parseColor(strings.ToLower(s), 1)
	return gfx.Color{R: c.R, G: c.G, B: c.B, A: c.A}, ok
}
