package editor

import (
	"fmt"
	"math"
	"testing"
)

func TestHighlightRanges(t *testing.T) {
	text := "name \"A b\"\nfontSize $(1 + 2)\npos 10 2.5\ntext ${w}"
	var got [][2]string
	for _, r := range HighlightRanges(text) {
		got = append(got, [2]string{string(r.Class), text[r.From:r.To]})
	}
	eq(t, "ranges", got, [][2]string{
		{"symbol", "name"},
		{"string", `"A b"`},
		{"symbol", "fontSize"},
		{"expression", "$(1 + 2)"},
		{"symbol", "pos"},
		{"number", "10"},
		{"number", "2.5"},
		{"symbol", "text"},
		{"expression", "${w}"},
	})
}

func TestHighlightMultilineExpression(t *testing.T) {
	text := "a ${\n  1 +\n  2\n}\nb"
	for _, r := range HighlightRanges(text) {
		if r.Class == HighlightExpression {
			eq(t, "expression", text[r.From:r.To], "${\n  1 +\n  2\n}")
			return
		}
	}
	t.Error("no expression range")
}

func TestHighlightMarkdownAndUnicode(t *testing.T) {
	text := "title \"\"\"héllo\"\"\" \"ü\""
	for _, r := range HighlightRanges(text) {
		if r.From < 0 || r.To > len(text) || r.From >= r.To {
			t.Errorf("range %v out of the text", r)
		}
	}
}

func TestColorsAreSwiftTennColors(t *testing.T) {
	eq(t, "light symbol", Colors["light"][HighlightSymbol], "#815f03")
	eq(t, "dark expression", Colors["dark"][HighlightExpression], "#d4854f")
}

func TestParseDiagnostics(t *testing.T) {
	eq(t, "valid", len(ParseDiagnostics("a 1\nb 2")), 0)
	text := "a 1\nb {\n  c 2"
	d := ParseDiagnostics(text)
	if len(d) == 0 {
		t.Fatal("expected a diagnostic")
	}
	for _, x := range d {
		if x.From < 0 || x.From > x.To || x.To > len(text) {
			t.Errorf("diagnostic %v out of range", x)
		}
	}
	if d[0].From < 4 {
		t.Errorf("error placed on line 0: %v", d[0])
	}
}

func luminance(t *testing.T, hex string) float64 {
	t.Helper()
	var r, g, b uint8
	if _, err := fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b); err != nil {
		t.Fatalf("bad colour %q", hex)
	}
	lin := func(c uint8) float64 {
		f := float64(c) / 255
		if f <= 0.03928 {
			return f / 12.92
		}
		return math.Pow((f+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}

// Every palette colour reads (WCAG AA, 4.5:1) on the material behind the code: dark grey in the
// dark theme, light grey and white in the light one.
func TestColorsContrast(t *testing.T) {
	backgrounds := map[string][]string{"light": {"#ffffff", "#f0f0f0"}, "dark": {"#2b2b2b", "#1e1e1e"}}
	for mode, palette := range Colors {
		for class, c := range palette {
			for _, bg := range backgrounds[mode] {
				l1, l2 := luminance(t, c), luminance(t, bg)
				if l1 < l2 {
					l1, l2 = l2, l1
				}
				if ratio := (l1 + 0.05) / (l2 + 0.05); ratio < 4.5 {
					t.Errorf("%s %s %s on %s: contrast %.2f < 4.5", mode, class, c, bg, ratio)
				}
			}
		}
	}
}
