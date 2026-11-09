package scene_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/exec"
	"github.com/haiodo/tenniarb/go/scene"
)

// classicStub is scene.Options.Classic of the tests: the values of one property of one node, keyed by
// name and property, and the calls it got.
func classicStub(vals map[string][]string, calls *[]string) func(scene.ClassicRef, string) ([]string, bool) {
	return func(at scene.ClassicRef, prop string) ([]string, bool) {
		key := at.Name + "|" + prop
		if calls != nil {
			*calls = append(*calls, key)
		}
		v, ok := vals[key]
		return v, ok
	}
}

// buildClassic builds element "E" of src as a classic diagram, with valu as its model values.
func buildClassic(t *testing.T, src string, valu func(scene.ClassicRef, string) ([]string, bool)) (*scene.Screen, string) {
	t.Helper()
	tree, err := scene.LoadSource(t.TempDir(), "e.tenn", src)
	if err != nil {
		t.Fatal(err)
	}
	sc := gfx.NewScene()
	var screen *scene.Screen
	sc.Update(func(tx *gfx.Tx) {
		screen, err = tree.Build(tx, nil, "E", scene.Options{Measurer: frac{}, Classic: valu})
	})
	if err != nil {
		t.Fatal(err)
	}
	sc.Commit()
	var out bytes.Buffer
	if err := gfx.WriteSubtreeSVG(&out, sc, screen.Root, gfx.Rect{}, 1); err != nil {
		t.Fatal(err)
	}
	return screen, out.String()
}

// TestClassicValues: a classic diagram takes the text of an expression from Options.Classic, for an item
// and for its colour; a property the hook does not know is still an ignored expression.
func TestClassicValues(t *testing.T) {
	var calls []string
	screen, svg := buildClassic(t, `element "E" {
  item A { width ${w}; height ${h}; title "${t}"; color ${c} }
}`, classicStub(map[string][]string{
		"A|width":  {"120"},
		"A|height": {"20"},
		"A|title":  {"Hi"},
		"A|color":  {"11,234,0"},
	}, &calls))
	box := screen.Boxes()[1]
	if box.W != 120 || box.H != 20 {
		t.Errorf("box %vx%v, want 120x20", box.W, box.H)
	}
	if len(box.Lines) != 1 || box.Lines[0] != "Hi" {
		t.Errorf("title %v, want Hi", box.Lines)
	}
	if !strings.Contains(svg, "rgb(11,234,0)") {
		t.Errorf("no rgb(11,234,0) fill in the SVG")
	}
	if len(screen.Warnings) != 0 {
		t.Errorf("warnings: %v", screen.Warnings)
	}
	// one lookup per node and property: the value of the hook is cached with the property
	seen := map[string]bool{}
	for _, c := range calls {
		if seen[c] {
			t.Errorf("%s looked up twice", c)
		}
		seen[c] = true
	}
}

// TestClassicRoot: the element itself is the root ref, not the item of its name.
func TestClassicRoot(t *testing.T) {
	var got scene.ClassicRef
	screen, _ := buildClassic(t, `element "E" {
  width ${w}
  item A { title x }
}`, func(at scene.ClassicRef, prop string) ([]string, bool) {
		if prop == "width" {
			got = at
			return []string{"300"}, true
		}
		return nil, false
	})
	if !got.Root || got.Name != "E" {
		t.Errorf("root ref %+v, want the element E", got)
	}
	if w := screen.Boxes()[0].W; w != 300 {
		t.Errorf("element width %v, want 300", w)
	}
}

// TestClassicHookNotCalledInScene: a scene element evaluates its expressions itself; the classic hook is
// not asked, even when the expression names a parameter of an instance.
func TestClassicHookNotCalledInScene(t *testing.T) {
	tree := paramsTree(t, "s.tenn", `element "Card" kind Component {
  export let w = 40
  item Box { height 10; title ${w} }
}
element "Screen" {
  item col {
    auto-layout column
    use Card as a { w 60 }
  }
}`)
	sc := gfx.NewScene()
	var screen *scene.Screen
	var err error
	sc.Update(func(tx *gfx.Tx) {
		screen, err = tree.Build(tx, nil, "Screen", scene.Options{Measurer: frac{}, EvalVars: exec.SceneEvalVars,
			Classic: func(scene.ClassicRef, string) ([]string, bool) {
				t.Error("the classic hook was asked in a scene")
				return nil, false
			}})
	})
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]scene.Box{}
	for _, b := range screen.Boxes() {
		m[b.Path] = b
	}
	if l := m["col.a.Box"].Lines; len(l) != 1 || l[0] != "60" {
		t.Errorf("title %v, want the parameter 60", l)
	}
}

// TestClassicWithoutHook: no hook keeps the old behaviour, an expression of a classic diagram is ignored
// without a warning.
func TestClassicWithoutHook(t *testing.T) {
	screen, _ := buildClassic(t, `element "E" {
  item A { width ${w}; height 10; title x }
}`, nil)
	if w := screen.Boxes()[1].W; w == 0 {
		t.Errorf("width %v, want the size of the content", w)
	}
	if len(screen.Warnings) != 0 {
		t.Errorf("warnings: %v", screen.Warnings)
	}
}

// A classic diagram drops the arguments of a title after the first, as go/render always has, instead of
// failing the whole file: a markdown title followed by another token is common in the examples.
func TestClassicTitleKeepsItsFirstArgument(t *testing.T) {
	_, svg := buildClassic(t, `element "E" {
  item A { width 80; height 20; title %{Hello} !(red) }
}`, classicStub(nil, nil))
	if !strings.Contains(svg, "Hello") {
		t.Fatalf("the first argument of the title is not drawn:\n%s", svg)
	}
}

// classicBoxes are the boxes of element "E" of src built as a classic diagram.
func classicBoxes(t *testing.T, src string) map[string]scene.Box {
	t.Helper()
	screen, _ := buildClassic(t, src, classicStub(nil, nil))
	m := map[string]scene.Box{}
	for _, b := range screen.Boxes() {
		m[b.Path] = b
	}
	return m
}

// An item with no text and no size is a dot of eight points in go/render: old diagrams hold markers and
// anchors that way.
func TestClassicEmptyItemIsADot(t *testing.T) {
	m := classicBoxes(t, `element "E" { item A { pos 0 0; title "" } }`)
	if got := m["A"]; got.W != 8 || got.H != 8 {
		t.Errorf("empty item %gx%g, want 8x8", got.W, got.H)
	}
}

// A value go/render could not read was ignored, not fatal: a size below one falls back to the text, and
// an item that computes 0 still draws.
func TestClassicIgnoresBadValues(t *testing.T) {
	m := classicBoxes(t, `element "E" {
  item A { pos 0 0; width (1 + 2); title "" }
  item B { pos 20 0; height 0; display circle; title "" }
}`)
	for _, name := range []string{"A", "B"} {
		if got := m[name]; got.W <= 0 || got.H <= 0 {
			t.Errorf("%s %gx%g draws nothing", name, got.W, got.H)
		}
	}
}

// A link whose end the item list does not hold is a warning in a classic diagram: the persistence
// fixtures keep such links, and the old renderer drew the rest of the file.
func TestClassicBrokenLinkWarns(t *testing.T) {
	screen, _ := buildClassic(t, `element "E" {
  item A { pos 0 0 }
  link "A" "Gone"
}`, classicStub(nil, nil))
	found := false
	for _, w := range screen.Warnings {
		if strings.Contains(w.Msg, "both ends must be items") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no warning for the broken link: %v", screen.Warnings)
	}
}
