package scene_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/scene"
)

// buildSource draws element "E" of a source written in a temporary root, and returns the SVG.
func buildSource(t *testing.T, src string) string {
	t.Helper()
	tree, err := scene.LoadSource(t.TempDir(), "e.tenn", src)
	if err != nil {
		t.Fatal(err)
	}
	sc := gfx.NewScene()
	var screen *scene.Screen
	sc.Update(func(tx *gfx.Tx) {
		screen, err = tree.Build(tx, nil, "E", scene.Options{Measurer: frac{}})
	})
	if err != nil {
		t.Fatal(err)
	}
	sc.Commit()
	var out bytes.Buffer
	if err := gfx.WriteSubtreeSVG(&out, sc, screen.Root, gfx.Rect{}, 1); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

// countLines is how many text runs of the SVG carry the word.
func countLines(svg, word string) int {
	return strings.Count(svg, ">"+word+"<")
}

// A line break of a classic body is a hard break, as go/render draws it: every line of the text stands
// on its own, and a `\n` written in a string is a newline.
func TestClassicBodyBreaksLines(t *testing.T) {
	cases := map[string]string{
		"block":    "element \"E\" { item A { body %{\nline one\nline two\n} } }\n",
		"markdown": "element \"E\" { item A { body \"line one\\nline two\" } }\n",
	}
	for name, src := range cases {
		svg := buildSource(t, src)
		for _, want := range []string{"line one", "line two"} {
			if !strings.Contains(svg, ">"+want+"<") {
				t.Fatalf("%s: no run %q in:\n%s", name, want, svg)
			}
		}
		if n := strings.Count(svg, "<text"); n < 3 {
			t.Errorf("%s: %d text runs, want one per line:\n%s", name, n, svg)
		}
		// The two lines stand apart: a line that swallowed the break would draw one run of both words.
		if strings.Contains(svg, "line one line two") {
			t.Errorf("%s: the lines are joined:\n%s", name, svg)
		}
	}
}

// A scene body keeps markdown: one newline of a paragraph is a soft break, so it draws as a space.
func TestSceneBodyKeepsSoftBreaks(t *testing.T) {
	svg := buildSource(t, "element \"E\" { item A { auto-layout column; body %{\nline one\nline two\n} } }\n")
	if !strings.Contains(svg, "line one line two") && !strings.Contains(svg, "line") {
		t.Fatalf("no text at all:\n%s", svg)
	}
	if strings.Count(svg, "<text") > 2 {
		t.Errorf("a scene body broke the soft line:\n%s", svg)
	}
}

// A word wider than the line is broken by characters in a classic body, as go/render did: the box grows
// down instead of running over its width. A scene body keeps the word whole, as markdown does.
func TestClassicBodyBreaksALongWord(t *testing.T) {
	long := strings.Repeat("abcdefghij", 20)
	box := func(src string) scene.Box {
		tree, err := scene.LoadSource(t.TempDir(), "e.tenn", src)
		if err != nil {
			t.Fatal(err)
		}
		sc := gfx.NewScene()
		var screen *scene.Screen
		sc.Update(func(tx *gfx.Tx) {
			screen, err = tree.Build(tx, nil, "E", scene.Options{Measurer: frac{}})
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, b := range screen.Boxes() {
			if b.Path == "A" {
				return b
			}
		}
		t.Fatal("no box A")
		return scene.Box{}
	}
	if b := box("element \"E\" { item A { width 100; body \"" + long + "\" } }\n"); b.W != 100 || b.H < 100 {
		t.Errorf("classic body %vx%v, want 100 wide and broken down the box", b.W, b.H)
	}
	words := 0
	for _, line := range box("element \"E\" { item A { auto-layout column; width 100; body \"" + long + "\" } }\n").Lines {
		words += len(strings.Fields(line))
	}
	if words > 1 {
		t.Errorf("a scene body broke the word into %d lines", words)
	}
}

// A list keeps the five points go/render reserves beside a bullet: the frame of an item with a list is five
// points wider than the words in it, because the paragraph of a bullet carries a head indent of five.
func TestClassicListKeepsTheBulletIndent(t *testing.T) {
	lw := bodyBoxWidth(t, `element "E" {
  item A { body %{
* first item of the list
* second item
} }
}`)
	pw := bodyBoxWidth(t, `element "E" {
  item A { body %{
first item of the list
second item
} }
}`)
	// The frac measurer of the tests gives ten points a character: the marker "• " is twenty, and the head
	// indent of a bullet adds the five the old renderer reserved.
	if d := lw - pw; d < 24.9 || d > 25.1 {
		t.Errorf("the list box is %v wide, the same words without bullets %v: the difference is %v, want 25 (20 of the marker and 5 of the indent)", lw, pw, d)
	}
}

// bodyBoxWidth is the width of the box of item A of a source.
func bodyBoxWidth(t *testing.T, src string) float64 {
	t.Helper()
	tree, err := scene.LoadSource(t.TempDir(), "e.tenn", src)
	if err != nil {
		t.Fatal(err)
	}
	sc := gfx.NewScene()
	var screen *scene.Screen
	sc.Update(func(tx *gfx.Tx) {
		screen, err = tree.Build(tx, nil, "E", scene.Options{Measurer: frac{}})
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range screen.Boxes() {
		if b.Path == "A" {
			return b.W
		}
	}
	t.Fatal("no box of A")
	return 0
}
