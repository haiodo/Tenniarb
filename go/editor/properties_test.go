package editor

import (
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
	eq(t, "dark expression", Colors["dark"][HighlightExpression], "#c67c48")
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
