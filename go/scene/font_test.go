package scene_test

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/tenniarb/go/scene"
)

func buildWith(t *testing.T, element string, o scene.Options) (*scene.Screen, *gfx.Scene) {
	t.Helper()
	sc := gfx.NewScene()
	var screen *scene.Screen
	var err error
	sc.Update(func(tx *gfx.Tx) { screen, err = load(t, "font.tenn").Build(tx, nil, element, o) })
	if err != nil {
		t.Fatal(err)
	}
	sc.Commit()
	return screen, sc
}

func TestLetterSpacingWidensBox(t *testing.T) {
	screen, _ := buildWith(t, "Font", scene.Options{})
	m := map[string]scene.Box{}
	for _, b := range screen.Boxes() {
		m[b.Path] = b
	}
	// EstimateText: 11 runes, each 18*0.6 wide, plus 2 per rune with letter-spacing 2
	wantBox(t, m, "Plain", 0, 0, 139, 31)
	wantBox(t, m, "Spaced", 0, -40, 161, 31)
	for _, w := range []string{"font-weight: takes", "font-style:", "letter-spacing:"} {
		if !warned(screen, w) {
			t.Errorf("no warning %q: %v", w, screen.Warnings)
		}
	}
}

func TestFontNameKeptWithoutFontProps(t *testing.T) {
	screen, sc := buildWith(t, "Font", scene.Options{FontName: "Menlo"})
	var out bytes.Buffer
	if err := gfx.WriteSubtreeSVG(&out, sc, screen.Root, gfx.Rect{}, 1); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `font-family="Menlo"`) {
		t.Errorf("Plain lost Options.FontName:\n%s", out.String())
	}
}

func TestCornerRadii(t *testing.T) {
	r := gfx.R(0, 0, 100, 50)
	s, screen := radiusSVG(t)
	if want := gfx.RoundRectPath4(r, [4]float64{10, 0, 0, 20}).SVG(); strings.Count(s, want) < 2 {
		t.Errorf("fill and clip should use the four-radius path %q:\n%s", want, s)
	}
	// the border is inset by half the line width, so each radius shrinks by 0.5
	in := gfx.RoundRectPath4(gfx.R(0.5, 0.5, 99, 49), [4]float64{9.5, 0, 0, 19.5}).SVG()
	if !strings.Contains(s, in) {
		t.Errorf("no inset border path %q:\n%s", in, s)
	}
	if !warned(screen, "corner-radius: takes 1 or 4") {
		t.Errorf("no warning for two radii: %v", screen.Warnings)
	}
	if one := gfx.RoundRectPath4(gfx.R(0, 0, 100, 50), [4]float64{8, 8, 8, 8}).SVG(); !strings.Contains(s, one) {
		t.Errorf("single radius path %q missing", one)
	}
}

func radiusSVG(t *testing.T) (string, *scene.Screen) {
	t.Helper()
	screen, sc := buildWith(t, "Radius", scene.Options{})
	var out bytes.Buffer
	if err := gfx.WriteSubtreeSVG(&out, sc, screen.Root, gfx.Rect{}, 1); err != nil {
		t.Fatal(err)
	}
	return out.String(), screen
}

type fontLog struct {
	gfx.EstimateText
	fonts []gfx.Font
}

func (l *fontLog) MeasureText(s string, f gfx.Font) gfx.TextMetrics {
	l.fonts = append(l.fonts, f)
	return l.EstimateText.MeasureText(s, f)
}

func TestFontNameListGivesOneFamily(t *testing.T) {
	log := &fontLog{}
	buildWith(t, "Font", scene.Options{FontName: "Helvetica, Arial, sans-serif", Measurer: log})
	var bold *gfx.Font
	for _, f := range log.fonts {
		if f.Weight == 700 {
			bold = &f
		}
	}
	if bold == nil || bold.Family != "Helvetica" || bold.Name != "" {
		t.Errorf("a weight-only item should take the first name of the FontName list as its family: %+v", bold)
	}
}

// kern measures a string narrower than the sum of its parts, as kerning does.
type kern struct{}

func (kern) MeasureText(s string, f gfx.Font) gfx.TextMetrics {
	n := float64(len([]rune(s)))
	return gfx.TextMetrics{Width: math.Max(0, 10*n-2*(n-1)), Ascent: 8, Descent: 2}
}

func TestWrapKeepsTextThatFitsWhole(t *testing.T) {
	screen, _ := buildWith(t, "Kern", scene.Options{Measurer: kern{}})
	m := map[string]scene.Box{}
	for _, b := range screen.Boxes() {
		m[b.Path] = b
	}
	if b, ok := m["Row.T"]; !ok || b.H != 10 {
		t.Errorf("a hug text of natural width wrapped or is missing: %+v in %v", b, m)
	}
}
