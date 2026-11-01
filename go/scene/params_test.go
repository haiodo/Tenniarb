package scene_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/tenniarb/go/exec"
	"github.com/haiodo/tenniarb/go/scene"
)

// componentDoc is a component with parameters (spec 5c): kind and size choose the styles, title is the text.
const componentDoc = `element "Button" kind Component {
  export let title = "Button"
  export let kind = secondary
  export let size
  styles {
    primary { color #6b3ad3; text-color #ffffff }
    secondary { color #ffffff; border-color #1725471a }
    large { height 48 }
    medium { height 40 }
    kind one-of primary secondary
    size one-of large medium
  }
  item Btn {
    auto-layout row
    use-style ${kind} ${size}
    item Label { title ${title}; font-size 14 }
  }
}

element "Screen" {
  item col {
    auto-layout column
    use Button as save { title "Save"; kind primary; size large }
    use Button as plain { size medium }
  }
}
`

// tenner loads src as file for a workspace of its own.
func tenner(t *testing.T, file, src string) (*scene.Tree, error) {
	t.Helper()
	return scene.LoadSource(t.TempDir(), file, src)
}

// paramsTree is tenner that must load.
func paramsTree(t *testing.T, file, src string) *scene.Tree {
	t.Helper()
	tree, err := tenner(t, file, src)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

// paramsBuild builds element path of tree with the parameters of the instances in scope, evaluated by the
// goja engine as the app does.
func paramsBuild(t *testing.T, tree *scene.Tree, path string) (*scene.Screen, string) {
	t.Helper()
	sc := gfx.NewScene()
	var screen *scene.Screen
	var err error
	sc.Update(func(tx *gfx.Tx) {
		screen, err = tree.Build(tx, nil, path, scene.Options{Measurer: frac{}, EvalVars: exec.SceneEvalVars})
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

// TestComponentParams: two parameters per instance, two uses with different values: the body of the
// component sees the values of its own instance in ${...}, and use-style ${kind} ${size} picks the styles
// before the styles are applied.
func TestComponentParams(t *testing.T) {
	screen, svg := paramsBuild(t, paramsTree(t, "params.tenn", componentDoc), "Screen")
	m := map[string]scene.Box{}
	for _, b := range screen.Boxes() {
		m[b.Path] = b
	}
	if h := m["col.save.Btn"].H; h != 48 {
		t.Errorf("save height %v, want the large style 48", h)
	}
	if h := m["col.plain.Btn"].H; h != 40 {
		t.Errorf("plain height %v, want the medium style 40 of the default size", h)
	}
	if l := m["col.save.Btn.Label"].Lines; len(l) != 1 || l[0] != "Save" {
		t.Errorf("save label %v, want Save", l)
	}
	// no title in the use: the default of the declaration
	if l := m["col.plain.Btn.Label"].Lines; len(l) != 1 || l[0] != "Button" {
		t.Errorf("plain label %v, want the default Button", l)
	}
	// save takes kind primary from the use, plain the default secondary
	for _, colour := range []string{"rgb(107,58,211)", "rgb(23,37,71)"} {
		if !strings.Contains(svg, colour) {
			t.Errorf("no %s in the styles chosen by ${kind}:\n%s", colour, svg)
		}
	}
	if len(screen.Warnings) != 0 {
		t.Errorf("warnings: %v", screen.Warnings)
	}
}

// TestParamExpression: a parameter is a variable of an expression, a value from the use site too.
func TestParamExpression(t *testing.T) {
	screen, _ := paramsBuild(t, paramsTree(t, "params.tenn", `element "Gauge" kind Component {
  export let n = 40
  item Box { width ${n + 12}; height 10 }
}
element "Screen" {
  item col {
    auto-layout column
    use Gauge as a
    use Gauge as b { n 20 }
  }
}`), "Screen")
	want := map[string]float64{"col.a.Box": 52, "col.b.Box": 32}
	for _, b := range screen.Boxes() {
		if w, ok := want[b.Path]; ok && b.W != w {
			t.Errorf("%s width %v, want %v", b.Path, b.W, w)
		}
	}
	if len(screen.Warnings) != 0 {
		t.Errorf("warnings: %v", screen.Warnings)
	}
}

// TestParamRequired: a parameter without a default and no value in the use is an error.
func TestParamRequired(t *testing.T) {
	_, err := tenner(t, "missing.tenn", `element "C" kind Component {
  export let size
  item Box { width 10; height 10 }
}
element "Screen" { item col { use C as x } }`)
	if err == nil || !strings.Contains(err.Error(), `element "C" needs "size"`) {
		t.Errorf("want an error about the missing size, got %v", err)
	}
}

// TestKind: kind marks a component, a library or an application; a file or element without one is a screen,
// and an unknown kind is a warning, not an error.
func TestKind(t *testing.T) {
	tree := paramsTree(t, "kinds.tenn", `kind Library
element "Button" kind Component { item Btn { width 10; height 10 } }
element "Screen" { use Button as b }
element "Other" { item Box { width 4; height 4 } }
`)
	if got := tree.File("kinds.tenn").Type; got != scene.TypeLibrary {
		t.Errorf("file kind %q, want %q", got, scene.TypeLibrary)
	}
	elems := tree.File("kinds.tenn").Elems
	if elems[0].Type != scene.TypeComponent || elems[1].Type != "" || elems[2].Type != "" {
		t.Errorf("element kinds %q, %q, %q; want component, empty, empty", elems[0].Type, elems[1].Type, elems[2].Type)
	}

	other := paramsTree(t, "gadget.tenn", `element "Loose" kind Gadget { item Box { width 4; height 4 } }`)
	if got := other.File("gadget.tenn").Elems[0].Type; got != "gadget" {
		t.Errorf("unknown kind %q, want it kept as written in lowercase", got)
	}
	if !strings.Contains(other.Diagnostics().Error(), `kind "Gadget" is not one of`) {
		t.Errorf("no warning about the unknown kind: %v", other.Diagnostics())
	}
}

// TestComponentOnItsOwn: a component opened as a screen, not through a use, shows its defaults: its own
// parameters are in scope with their default values.
func TestComponentOnItsOwn(t *testing.T) {
	screen, _ := paramsBuild(t, paramsTree(t, "own.tenn", `element "Button" kind Component {
  export let title = "Button"
  export let size = medium
  styles {
    medium { height 40 }
    large { height 48 }
  }
  item Btn {
    auto-layout row
    use-style ${size}
    item Label { title ${title} }
  }
}`), "Button")
	m := map[string]scene.Box{}
	for _, b := range screen.Boxes() {
		m[b.Path] = b
	}
	if h := m["Btn"].H; h != 40 {
		t.Errorf("height %v, want the default size medium 40", h)
	}
	if l := m["Btn.Label"].Lines; len(l) != 1 || l[0] != "Button" {
		t.Errorf("label %v, want the default Button", l)
	}
	if len(screen.Warnings) != 0 {
		t.Errorf("warnings: %v", screen.Warnings)
	}
}
