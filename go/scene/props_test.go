package scene_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/tenniarb/go/scene"
)

func svgOf(t *testing.T, element string) (string, *scene.Screen) {
	t.Helper()
	screen, sc := build(t, load(t, "props.tenn"), element)
	var out bytes.Buffer
	if err := gfx.WriteSubtreeSVG(&out, sc, screen.Root, gfx.Rect{}, 1); err != nil {
		t.Fatal(err)
	}
	return out.String(), screen
}

func warned(s *scene.Screen, part string) bool {
	for _, w := range s.Warnings {
		if strings.Contains(w.Error(), part) {
			return true
		}
	}
	return false
}

func TestNameIsTextOnlyOutsideLayout(t *testing.T) {
	m := boxes(t, "props.tenn", "NameText")
	wantBox(t, m, "Brain", 0, 0, 74, 31) // name "Brain": ceil(5 * 10.8) + 2 * 10, padded 10x6
	if len(m["Ctx.NoTitle"].Lines) != 0 || len(m["Ctx.Empty"].Lines) != 0 {
		t.Errorf("name drawn as text in a layout context: %v", m)
	}
	wantBox(t, m, "Ctx.NoTitle", 0, 0, 50, 10)
	wantBox(t, m, "Ctx.Titled", 0, 10, 22, 19)
}

func TestFontSizeNoClamp(t *testing.T) {
	m := boxes(t, "props.tenn", "Font")
	if got := m["Big"].H; got != 59+12 { // ceil(56 * 1.05) + 2 * 6
		t.Errorf("Big.H = %v", got)
	}
	s, _ := build(t, load(t, "props.tenn"), "Font")
	if !warned(s, "font-size: 0 is not above 0") {
		t.Errorf("no warning for font-size 0: %v", s.Warnings)
	}
	if got := m["Bad"].H; got != 19+12 {
		t.Errorf("Bad.H = %v, want the default size", got)
	}
}

func TestHyphenWrap(t *testing.T) {
	m := boxes(t, "props.tenn", "Hyphen")
	if got := strings.Join(m["Box.H"].Lines, "|"); got != "aaaa-|bbbb" {
		t.Errorf("lines = %q", got)
	}
	// a hyphen with a space around it or at the start of a word is not a break point
	if got := strings.Join(m["Box.Dash"].Lines, "|"); got != "a - b -5" {
		t.Errorf("lines = %q", got)
	}
}

func TestOpacity(t *testing.T) {
	s, screen := svgOf(t, "Opacity")
	if !strings.Contains(s, "opacity") {
		t.Errorf("no opacity in:\n%s", s)
	}
	if !warned(screen, "opacity takes 0..1") {
		t.Errorf("no warning for opacity 2: %v", screen.Warnings)
	}
	if n := strings.Count(s, "opacity"); n != 1 {
		t.Errorf("%d opacity attributes, want 1", n)
	}
}

func TestHidden(t *testing.T) {
	screen, _ := build(t, load(t, "props.tenn"), "Hidden")
	if got, want := strings.Join(paths(screen), " "), " Row Row.A Row.C Kept"; got != want {
		t.Errorf("boxes = %q, want %q", got, want)
	}
	m := map[string]scene.Box{}
	for _, b := range screen.Boxes() {
		m[b.Path] = b
	}
	wantBox(t, m, "Row.C", 30, 0, 20, 10) // B takes no place and no gap
}

func TestIcon(t *testing.T) {
	s, screen := svgOf(t, "Icon")
	m := map[string]scene.Box{}
	for _, b := range screen.Boxes() {
		m[b.Path] = b
	}
	wantBox(t, m, "Sized.I", 0, 0, 20, 20)
	wantBox(t, m, "Sized.Hug", 20, 0, 10, 10) // the view box is the natural size
	if len(m["Sized.I"].Lines) != 0 {
		t.Errorf("an icon draws no name")
	}
	for _, part := range []string{"rgb(255,0,0)", "rgb(0,0,255)", "stroke-width=\"2\""} {
		if !strings.Contains(s, part) {
			t.Errorf("missing %q in:\n%s", part, s)
		}
	}
	// scaled by 2: the point (10,0) is at x=20
	if !strings.Contains(s, "20") {
		t.Errorf("no scaled coordinates:\n%s", s)
	}
	if !warned(screen, "path: not an SVG path") {
		t.Errorf("no warning for a bad path: %v", screen.Warnings)
	}
}

func TestFillRule(t *testing.T) {
	s, _ := svgOf(t, "FillRule")
	if n := strings.Count(s, `fill-rule="evenodd"`); n != 1 {
		t.Errorf("%d evenodd paths, want 1:\n%s", n, s)
	}
	_, err := scene.LoadSource(t.TempDir(), "m.tenn", `element "E" { item "I" { path "M0 0H1V1Z"; fill-rule diag } }`)
	if err == nil || !strings.Contains(err.Error(), "fill-rule takes one of") {
		t.Errorf("fill-rule diag: err = %v", err)
	}
}

func TestLayoutContextDefaults(t *testing.T) {
	s, _ := svgOf(t, "Defaults")
	// outside a layout context: white fill and black border; inside: only what is asked for
	if n := strings.Count(s, `fill="rgb(255,255,255)"/>`); n != 1 {
		t.Errorf("%d white fills, want 1 (the brain item):\n%s", n, s)
	}
	for _, part := range []string{"rgb(17,34,51)", "rgb(68,85,102)"} {
		if !strings.Contains(s, part) {
			t.Errorf("missing %q in:\n%s", part, s)
		}
	}
	if strings.Contains(s, `stroke-width="4"`) {
		t.Errorf("line-width alone drew a border:\n%s", s)
	}
	// black text on no fill, white text on a black fill
	if !strings.Contains(s, `rgb(255,255,255)">x<`) || !strings.Contains(s, `rgb(0,0,0)">y<`) {
		t.Errorf("text colors:\n%s", s)
	}
}

// An expression is normal in a classic diagram (go/render evaluates it); only a scene element warns.
func TestExpressionWarningOnlyInScenes(t *testing.T) {
	warns := func(src string) int {
		tr, err := scene.LoadSource(t.TempDir(), "a.tenn", src)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, d := range tr.Diagnostics() {
			if strings.Contains(d.Msg, "expressions are not evaluated") {
				n++
			}
		}
		return n
	}
	classic := "element Map {\n\titem A { pos 0 0; width ${10 * 2} }\n}\n"
	if n := warns(classic); n != 0 {
		t.Errorf("classic diagram: %d warnings", n)
	}
	screen := "element Screen {\n\titem Page { auto-layout column; width ${10 * 2} }\n}\n"
	if n := warns(screen); n != 1 {
		t.Errorf("scene element: %d warnings, want 1", n)
	}
}
