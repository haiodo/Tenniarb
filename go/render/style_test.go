package render

import (
	"testing"

	"github.com/haiodo/tenniarb/go/tenn"
)

// Port of DrawableStyleTests.swift (non-AppKit parts).

func apply(t testing.TB, src string, s interface {
	ParseStyle(*tenn.Node, Evaluated)
}) {
	t.Helper()
	s.ParseStyle(styleTree(t, src), nil)
}

func TestStyleDefaultsDifferBetweenLightAndDark(t *testing.T) {
	light, dark := newDrawableStyle(false), newDrawableStyle(true)
	if light.Color == dark.Color || light.BorderColor == dark.BorderColor {
		t.Error("light and dark defaults must differ")
	}
	if light.FontSize != 18 || light.Width != nil || light.Height != nil {
		t.Errorf("defaults: %+v", light)
	}
}

func TestStyleColor(t *testing.T) {
	s := newDrawableStyle(false)
	apply(t, "color red", s)
	if s.Color != (Color{1, 0, 0, 1}) {
		t.Errorf("red = %+v", s.Color)
	}
	before := s.Color
	apply(t, "color notacoloratall", s)
	if s.Color != before {
		t.Error("unknown name must keep the colour")
	}

	apply(t, "color #336699", s)
	if s.Color != (Color{0.2, 0.4, 0.6, 1}) {
		t.Errorf("#336699 = %+v", s.Color)
	}
	apply(t, "color #33669980", s)
	if s.Color.A != 128.0/255 {
		t.Errorf("8 digits carry alpha: %v", s.Color.A)
	}
	before = s.Color
	apply(t, "color #123", s)
	if s.Color != before {
		t.Error("3 digits must be rejected")
	}
}

func TestStyleTextColorFollowsContrast(t *testing.T) {
	s := newDrawableStyle(false)
	if s.TextColorValue != nil || s.TextColor() != colorBlack {
		t.Error("white box, black text")
	}
	apply(t, "color black", s)
	if s.TextColor() != colorWhite {
		t.Error("dark box, white text")
	}
	apply(t, "text-color red", s)
	if s.TextColor() != (Color{1, 0, 0, 1}) {
		t.Error("explicit value wins")
	}
}

func TestStyleFontSizeClamped(t *testing.T) {
	for src, want := range map[string]float64{"font-size 99": 36, "font-size 1": 4, "font-size 20": 20} {
		s := newDrawableStyle(false)
		apply(t, src, s)
		if s.FontSize != want {
			t.Errorf("%s: %v, want %v", src, s.FontSize, want)
		}
	}
}

func TestStyleWidthHeightCapped(t *testing.T) {
	s := newDrawableStyle(false)
	apply(t, "width 99999\nheight 99999", s)
	if *s.Width != 10000 || *s.Height != 10000 {
		t.Errorf("cap: %v %v", *s.Width, *s.Height)
	}
	apply(t, "width 120\nheight 40", s)
	if *s.Width != 120 || *s.Height != 40 {
		t.Errorf("set: %v %v", *s.Width, *s.Height)
	}
}

func TestStyleDisplayLayerLineStyleVerbatim(t *testing.T) {
	s := newDrawableStyle(false)
	apply(t, "display text\nlayer background\nline-style dashed", s)
	if *s.Display != "text" || *s.Layer != "background" || *s.LineStyle != "dashed" {
		t.Errorf("%v %v %v", *s.Display, *s.Layer, *s.LineStyle)
	}
}

func TestStyleLayoutJoinsArguments(t *testing.T) {
	s := newDrawableStyle(false)
	apply(t, "layout auto center", s)
	if *s.Layout != "auto, center" {
		t.Errorf("layout = %q", *s.Layout)
	}
	apply(t, "layout", s)
	if *s.Layout != "" {
		t.Errorf("bare layout = %q", *s.Layout)
	}
}

func TestStyleShadow(t *testing.T) {
	s := newDrawableStyle(false)
	apply(t, "shadow 2 3 7 red", s)
	if *s.Shadow != (Size{2, 3}) || s.ShadowBlur != 7 || s.ShadowColor == nil {
		t.Errorf("shadow: %+v blur %v", s.Shadow, s.ShadowBlur)
	}
	p := newDrawableStyle(false)
	blur := p.ShadowBlur
	apply(t, "shadow 1 1", p)
	if *p.Shadow != (Size{1, 1}) || p.ShadowBlur != blur || p.ShadowColor != nil {
		t.Errorf("offset only: %+v", p)
	}
}

func TestStyleInheritReadsCommandNameAndIndex(t *testing.T) {
	s := newDrawableStyle(false)
	apply(t, "inherit 3", s)
	if *s.Inherit != "inherit" || s.InheritIndex != 3 {
		t.Errorf("inherit %v %v", *s.Inherit, s.InheritIndex)
	}
}

func TestStyleUnknownIgnoredAndReset(t *testing.T) {
	s := newDrawableStyle(false)
	apply(t, "totally-unknown 1 2 3", s)
	if s.FontSize != 18 {
		t.Error("unknown command changed the style")
	}
	apply(t, "font-size 30\nwidth 100\ndisplay text\nline-style dashed", s)
	s.Reset()
	if s.FontSize != 18 || s.Width != nil || s.Display != nil || s.LineStyle != nil {
		t.Errorf("reset: %+v", s)
	}
}

func TestStyleCopyIsIndependent(t *testing.T) {
	s := NewDrawableItemStyle(false)
	apply(t, "color red\nfont-size 22\nwidth 120\ndisplay text\nlayer hover\nline-width 3\nshadow 1 2 3 blue\ntitle T\ncorner-radius 4 5", s)
	c := s.Copy()
	if c.Color != s.Color || c.FontSize != 22 || *c.Width != 120 || *c.Display != "text" || *c.Layer != "hover" || c.LineWidth != 3 || *c.Shadow != (Size{1, 2}) || *c.Title != "T" {
		t.Errorf("copy lost a field: %+v", c)
	}
	apply(t, "font-size 30\ncorner-radius 9", s)
	if c.FontSize != 22 || len(c.CornerRadius) != 2 {
		t.Error("copy follows the original")
	}
}

func TestComponentValue(t *testing.T) {
	if componentValue(10) != 10 || componentValue(20.5) != 20.5 || componentValue("not a number") != 255 {
		t.Error("components")
	}
}

func TestLineStyleDefaults(t *testing.T) {
	l := NewDrawableLineStyle(false)
	if l.Color != (Color{0.2, 0.2, 0.2, 1}) || l.LineWidth != 1 {
		t.Errorf("%+v", l)
	}
	if NewDrawableLineStyle(true).Color != colorWhite {
		t.Error("dark line colour")
	}
	if l.Copy().LineWidth != 1 {
		t.Error("copy")
	}
}

// Evaluated values replace the written token (JSValue in Swift).
func exprNode(name, literal string) (*tenn.Node, *tenn.Token) {
	tok := tenn.NewToken(tenn.TokExpression, literal)
	return tenn.NewCommand(name, tenn.NewNode(tenn.KindExpression, tok)), tok
}

func TestExpressionSuppliesFloat(t *testing.T) {
	n, tok := exprNode("font-size", "10 + 5")
	s := newDrawableStyle(false)
	s.ParseStyle(tenn.NewBlockExpr(n), Evaluated{tok: 15.0})
	if s.FontSize != 15 {
		t.Errorf("font size %v", s.FontSize)
	}
}

func TestExpressionSuppliesRGBArray(t *testing.T) {
	n, tok := exprNode("color", "[255, 0, 0]")
	s := newDrawableStyle(false)
	s.ParseStyle(tenn.NewBlockExpr(n), Evaluated{tok: []any{255.0, 0.0, 0.0}})
	if s.Color != (Color{1, 0, 0, 1}) {
		t.Errorf("rgb %+v", s.Color)
	}
	n, tok = exprNode("color", "[255, 0, 0, 128]")
	s.ParseStyle(tenn.NewBlockExpr(n), Evaluated{tok: []any{255.0, 0.0, 0.0, 128.0}})
	if s.Color.A != 128.0/255 {
		t.Errorf("alpha %v", s.Color.A)
	}
}

func TestExpressionColorNameAndUnparsable(t *testing.T) {
	n, tok := exprNode("color", "'red'")
	s := newDrawableStyle(false)
	s.ParseStyle(tenn.NewBlockExpr(n), Evaluated{tok: "Red"})
	if s.Color != (Color{1, 0, 0, 1}) {
		t.Errorf("named %+v", s.Color)
	}
	n, tok = exprNode("color", "42")
	before := s.Color
	s.ParseStyle(tenn.NewBlockExpr(n), Evaluated{tok: 42.0})
	if s.Color != before {
		t.Error("an unparsable value must not fall back to the token text")
	}
}

func TestItemStyle(t *testing.T) {
	s := NewDrawableItemStyle(false)
	apply(t, "title Header\nmarker dot", s)
	if *s.Title != "Header" || *s.Marker != "dot" {
		t.Error("title/marker")
	}
	apply(t, "corner-radius 4 8 99 2", s)
	if len(s.CornerRadius) != 4 || s.CornerRadius[2] != 15 {
		t.Errorf("radius cap: %v", s.CornerRadius)
	}
	apply(t, "corner-radius 5", s)
	if len(s.CornerRadius) != 1 || s.CornerRadius[0] != 5 {
		t.Errorf("radius replace: %v", s.CornerRadius)
	}
	apply(t, "line-spacing 3\nfont-size 24\ndisplay text", s)
	if s.LineSpacing != 3 || s.FontSize != 24 || *s.Display != "text" {
		t.Error("spacing/size/display")
	}
}

func TestSceneStyle(t *testing.T) {
	if NewSceneStyle(false).GridSpan != (Point{5, 5}) || NewSceneStyle(false).DefaultLineStyle.FontSize != 12 {
		t.Error("scene defaults")
	}
	s := NewSceneStyle(false)
	apply(t, "grid 20 30", s)
	if s.GridSpan != (Point{20, 30}) {
		t.Error("grid")
	}
	apply(t, "styles {\n  item {\n    font-size 24\n  }\n  line {\n    line-width 4\n  }\n}", s)
	if s.DefaultItemStyle.FontSize != 24 || s.DefaultLineStyle.LineWidth != 4 || s.DefaultLineStyle.FontSize == 24 {
		t.Error("styles block")
	}
}

func TestPrepareBodyText(t *testing.T) {
	for in, want := range map[string]string{
		"\n    one\n    two\n": "one\ntwo",
		"\n  one\n    two\n":   "one\n  two",
		"one\\ntwo":            "one\ntwo",
		"\nbody\n":             "body",
		"plain":                "plain",
		"":                     "",
	} {
		if got := prepareBodyText(in); got != want {
			t.Errorf("prepareBodyText(%q) = %q, want %q", in, got, want)
		}
	}
	if got := prepareBodyText("\tone"); got != "one" && got != "one " {
		t.Errorf("tab: %q", got)
	}
}

func TestParseColorForms(t *testing.T) {
	if c, ok := parseColor("#ff8000", 1); !ok || c != (Color{1, 128.0 / 255, 0, 1}) {
		t.Errorf("hex: %+v %v", c, ok)
	}
	if _, ok := parseColor("12345", 1); ok {
		t.Error("5 digits")
	}
	if _, ok := parseColor("zzzzzz", 1); ok {
		t.Error("not hex")
	}
	if c, ok := parseColor("transparent", 1); !ok || c.A != 0 {
		t.Errorf("transparent: %+v %v", c, ok)
	}
}

func TestGenericRGBToSRGB(t *testing.T) {
	// CGColor(red:green:blue:) is Generic RGB; Swift's sRGB bitmap shows red as #FF2600 and 0.2 gray as 66.
	for _, tc := range []struct {
		c    Color
		want [3]int
	}{
		{Color{1, 0, 0, 1}, [3]int{255, 38, 0}},
		{Color{0.2, 0.2, 0.2, 1}, [3]int{66, 66, 66}},
	} {
		s := genericToSRGB(tc.c)
		for i := range s {
			if got := int(s[i]*255 + 0.5); got != tc.want[i] {
				t.Errorf("%+v channel %d = %d, want %d", tc.c, i, got, tc.want[i])
			}
		}
	}
	if c := (Color{1, 1, 1, 0.5}).gfxColor(); c.A != 0.5 || !near(c.R, 1, 1e-6) || !near(c.G, 1, 1e-6) {
		t.Errorf("white half alpha: %+v", c)
	}
}
