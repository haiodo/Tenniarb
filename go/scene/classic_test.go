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
