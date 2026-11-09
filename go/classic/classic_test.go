package classic_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/classic"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
	"github.com/haiodo/tenniarb/go/scene"
)

// parseDiagram parses tenn source and returns the first nested element.
func parseDiagram(t *testing.T, src string) *model.Element {
	t.Helper()
	m := model.ReadTenn(src)
	if m == nil {
		t.Fatal("source must parse cleanly")
	}
	return m.Elements[0]
}

// evalCtx is the evaluator of the old renderer with the measuring of the tests.
func evalCtx(el *model.Element) model.IExecutionContext {
	return render.NewExecContext(el, render.Options{Measurer: gfx.EstimateText{}})
}

// TestValues: the values a classic diagram reads, by item name and index, by a template string and by a
// colour array; a literal comes back as written, an element property too.
func TestValues(t *testing.T) {
	el := parseDiagram(t, `element Root {
    width ${10 + 20}
    item A {
        pos 0 0
        my_value 3
        value ${my_value + 1}
        title "${value}"
        color ${[my_value, 2, 3]}
    }
    item A {
        pos 100 0
        my_value 7
        value ${my_value * 2}
    }
}`)
	valu := classic.Values(el, evalCtx(el))

	at := func(nth int) scene.ClassicRef { return scene.ClassicRef{Kind: scene.RItem, Name: "A", Nth: nth} }
	if got, ok := valu(at(0), "title"); !ok || len(got) != 1 || got[0] != "4" {
		t.Errorf("first A title %v, %v; want 4", got, ok)
	}
	if got, ok := valu(at(0), "color"); !ok || len(got) != 1 || got[0] != "3,2,3" {
		t.Errorf("first A color %v, %v; want 3,2,3", got, ok)
	}
	if got, ok := valu(at(0), "my_value"); !ok || len(got) != 1 || got[0] != "3" {
		t.Errorf("first A my_value %v, %v; want the literal 3", got, ok)
	}
	if got, ok := valu(at(1), "value"); !ok || len(got) != 1 || got[0] != "14" {
		t.Errorf("second A value %v, %v; want 14", got, ok)
	}
	root := scene.ClassicRef{Kind: scene.RItem, Name: "Root", Root: true}
	if got, ok := valu(root, "width"); !ok || len(got) != 1 || got[0] != "30" {
		t.Errorf("element width %v, %v; want 30", got, ok)
	}
	if _, ok := valu(at(2), "value"); ok {
		t.Errorf("a third A was found")
	}
	if _, ok := valu(at(0), "unknown"); ok {
		t.Errorf("an unknown property was found")
	}
}

// TestValuesLink: a link is named by its ends, as the diagram writes them.
func TestValuesLink(t *testing.T) {
	el := parseDiagram(t, `element Root {
    item A { pos 0 0 }
    item B { pos 100 0 }
    link A B { line-width ${2 * 3} }
}`)
	valu := classic.Values(el, evalCtx(el))
	got, ok := valu(scene.ClassicRef{Kind: scene.RLink, From: "A", To: "B"}, "line-width")
	if !ok || len(got) != 1 || got[0] != "6" {
		t.Errorf("link line-width %v, %v; want 6", got, ok)
	}
}

// Hook reads a value from the element model of a classic diagram, and stays out of the way of a path that
// names no element of the file.
func TestHook(t *testing.T) {
	root := t.TempDir()
	const src = `element "E" {
  item A { width ${40 + 2} }
}
`
	if err := os.WriteFile(filepath.Join(root, "e.tenn"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tree, err := scene.Load(root, "e.tenn")
	if err != nil {
		t.Fatal(err)
	}
	hook := classic.Hook(tree, root, "e.tenn", "E", render.ClassicContext)
	if hook == nil {
		t.Fatal("no hook for the element E")
	}
	got, ok := hook(scene.ClassicRef{Kind: scene.RItem, Name: "A"}, "width")
	if !ok || len(got) != 1 || got[0] != "42" {
		t.Errorf("A|width = %v %v, want 42", got, ok)
	}
	for _, path := range []string{"", "Gone"} {
		if hook := classic.Hook(tree, root, "e.tenn", path, render.ClassicContext); hook != nil {
			t.Errorf("path %q has a hook", path)
		}
	}
	if hook := classic.Hook(tree, root, "e.tenn", "E", nil); hook != nil {
		t.Error("a hook without an evaluator")
	}
}

// TestValuesAgreeWithRender: the text of a value is the one go/render writes, so a picture of the scene and
// one of the old renderer show the same words: the two conversions are one conversion, twice written.
func TestValuesAgreeWithRender(t *testing.T) {
	// The conversion goes through an expression, the way a classic property is read: the value of the
	// property is what both renderers print.
	src := `element Root {
    item A {
        pos 0 0
        n_nil ${null}
        n_bool ${true}
        n_num ${-2.25}
        n_big ${1e21}
        n_small ${1e-7}
        n_list ${[1, 2.5]}
    }
}`
	el := parseDiagram(t, src)
	valu := classic.Values(el, evalCtx(el))
	at := scene.ClassicRef{Kind: scene.RItem, Name: "A"}
	for _, prop := range []string{"n_nil", "n_bool", "n_num", "n_big", "n_small", "n_list"} {
		got, ok := valu(at, prop)
		if !ok || len(got) != 1 {
			t.Fatalf("%s = %v %v", prop, got, ok)
		}
		if want := render.ValueString(evalExpr(t, el, prop)); got[0] != want {
			t.Errorf("%s: %q, go/render writes %q", prop, got[0], want)
		}
	}
}

// evalExpr is the value of property prop of item A as the element model computes it.
func evalExpr(t *testing.T, el *model.Element, prop string) any {
	t.Helper()
	var item *model.DiagramItem
	for _, it := range el.Items {
		if it.Name == "A" {
			item = it
		}
	}
	cmd := item.Properties.Get(prop)
	if cmd == nil || len(cmd.Children) < 2 {
		t.Fatalf("no %s", prop)
	}
	ev := evalCtx(el).GetEvaluated(item, nil, nil)
	v, ok := ev[cmd.Children[1].Token]
	if !ok {
		t.Fatalf("%s has no value", prop)
	}
	return v
}
