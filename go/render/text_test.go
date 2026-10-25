package render

import (
	"math"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/markdown"
)

// Regressions from the tenniarb.tenn Swift-vs-TS comparison, and the SceneTextTests.

func itemHeight(t *testing.T, src string) float64 {
	t.Helper()
	el := parseDiagram(t, src)
	return NewDrawableScene(el, testOpts()).Drawables[el.Items[0]].SelectorBounds().Height
}

// SF 10pt: CTLineGetTypographicBounds 9.668 + 2.109 -> floor 11, +1 per line in calculateSize; plus 4+4 padding.
func TestLineHeightFollowsSFMetrics(t *testing.T) {
	if h := itemHeight(t, `element R { item "a\nb\nc\nd\ne\nf" { pos 0 0
 font-size 10 } }`); h != 6*12+8 {
		t.Errorf("height %v", h)
	}
}

// CoreText counts the "\n" glyph in the line it ends, so a big-font body lifts the title line to its height.
func TestNewlineAfterTitleTakesBodyFont(t *testing.T) {
	// SF 37pt: floor(37 * 2412 / 2048) + 1 = 44 per line, three lines (T, B, r), 4+4 padding.
	h := itemHeight(t, `element R { item "T" { pos 0 0
 body { text "B\nr"
 font-size 37 } } }`)
	if h != 3*44+8 {
		t.Errorf("height %v", h)
	}
}

func plain(s string) *AttributedString { return PlainText(s, systemFont(12)) }

func TestCalculateSize(t *testing.T) {
	env := testEnv()
	if !(env.calculateSize(plain("a much longer piece of text")).Width > env.calculateSize(plain("a")).Width) {
		t.Error("width grows with the text")
	}
	if got := env.calculateSize(plain("")); got != (Size{}) {
		t.Errorf("empty text: %+v", got)
	}
	if !(env.calculateSize(plain("one\ntwo")).Height > env.calculateSize(plain("one")).Height) {
		t.Error("multiline text is taller")
	}
	if env.calculateSize(plain("one\n")).Height != env.calculateSize(plain("one")).Height {
		t.Error("a trailing newline adds no line")
	}
}

func lineTexts(lines []*textLine) string {
	var parts []string
	for _, l := range lines {
		var b strings.Builder
		for _, s := range l.segments {
			b.WriteString(s.text)
		}
		parts = append(parts, b.String())
	}
	return strings.Join(parts, " ")
}

func TestWrappingHappensAtFrameWidthOnly(t *testing.T) {
	env := testEnv()
	text := plain("alpha beta gamma delta epsilon")
	if n := len(env.layoutLines(text, nil)); n != 1 {
		t.Fatalf("calculateSize must not wrap: %d lines", n)
	}
	width := env.calculateSize(text).Width
	half := width / 2
	lines := env.layoutLines(text, &half)
	if len(lines) < 2 {
		t.Fatalf("%d lines", len(lines))
	}
	for _, l := range lines {
		if l.width > half+1e-3 {
			t.Errorf("line %v wider than %v", l.width, half)
		}
	}
	if got := strings.Join(strings.Fields(lineTexts(lines)), " "); got != "alpha beta gamma delta epsilon" {
		t.Errorf("text lost: %q", got)
	}
}

func TestLongWordBreaksBetweenCharacters(t *testing.T) {
	env := testEnv()
	limit := 40.0
	lines := env.layoutLines(plain("Supercalifragilistic"), &limit)
	if len(lines) <= 1 {
		t.Fatal("not broken")
	}
	for _, l := range lines {
		if l.width > limit+1e-3 {
			t.Errorf("line %v wider than %v", l.width, limit)
		}
	}
}

func TestLongWordKeepsGraphemeClusters(t *testing.T) {
	env := testEnv()
	limit := 20.0
	// A flag is two regional indicators: one character for NSLayoutManager.
	lines := env.layoutLines(plain(strings.Repeat("\U0001F1E9\U0001F1EA", 4)), &limit)
	for _, l := range lines {
		for _, s := range l.segments {
			if n := len([]rune(s.text)); n%2 != 0 {
				t.Errorf("a flag was split: %q", s.text)
			}
		}
	}
}

func TestBulletsIndentContinuationLines(t *testing.T) {
	shift := Point{}
	attr := toAttributedString(markdown.GetTokens("* item text that wraps"), systemFont(12), colorBlack, &shift, NewElementImageProvider(host(), 1, nil), nil, 0)
	if shift.X != 5 {
		t.Errorf("shift %v", shift.X)
	}
	w := 60.0
	lines := testEnv().layoutLines(attr, &w)
	if len(lines) < 2 || lines[1].paragraph.HeadIndent != 5 {
		t.Errorf("lines %d", len(lines))
	}
}

func TestMarkdownRuns(t *testing.T) {
	attr := toAttributedString(markdown.GetTokens("*bold* text _it_ ~gone~ __under__"), systemFont(12), colorBlack, &Point{}, NewElementImageProvider(host(), 1, nil), nil, 0)
	if !strings.Contains(attr.String(), "bold") || !strings.Contains(attr.String(), "text") {
		t.Errorf("text %q", attr.String())
	}
	var bold, italic, strike bool
	for _, r := range attr.Runs {
		bold = bold || r.Font.Bold && r.Text == "bold"
		italic = italic || r.Font.Italic
		strike = strike || r.Strikethrough
	}
	if !bold || !italic || !strike {
		t.Errorf("bold %v italic %v strike %v", bold, italic, strike)
	}
}

func TestColorAndFontTokens(t *testing.T) {
	attr := toAttributedString(markdown.GetTokens("!(red|r) &(30|big) plain"), systemFont(12), colorBlack, &Point{}, NewElementImageProvider(host(), 1, nil), nil, 0)
	var red, big bool
	for _, r := range attr.Runs {
		if r.Text == "r" && r.Color != nil && *r.Color == (Color{1, 0, 0, 1}) {
			red = true
		}
		if r.Text == "big" && r.Font.Size == 30 {
			big = true
		}
	}
	if !red || !big {
		t.Errorf("red %v big %v", red, big)
	}
}

func TestTextBoxHTML(t *testing.T) {
	el := parseDiagram(t, "element R { item \"**x**\" { pos 0 0 } }")
	s := NewDrawableScene(el, testOpts())
	tb := s.Drawables[el.Items[0]].(*RoundBox).Children[0].(*TextBox)
	if !strings.Contains(tb.HTML(), "<div>") {
		t.Errorf("html %q", tb.HTML())
	}
}

func TestFontSpecMetricsAreLinear(t *testing.T) {
	f := systemFont(10)
	if math.Abs(f.ascent()-9.668) > 1e-3 || math.Abs(f.descent()-2.109) > 1e-3 {
		t.Errorf("SF 10pt: %v %v", f.ascent(), f.descent())
	}
}
