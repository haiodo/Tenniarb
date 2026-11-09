//go:build darwin || linux || windows

package app

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/exec"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/scene"
)

// exprStyleDoc is a component whose use-style is an expression over its parameters, used by a screen
// that sets one of them: the styles of a part of the instance are known only after the expression runs.
const exprStyleDoc = `element "Rel" {
  styles {
    right { width 10 }
    left { width 20 }
    down { width 30 }
    up { width 40 }
    left-dashed { width 50 }
    direction one-of right left down up
    stroke one-of solid dashed
  }
  export let direction = right
  export let stroke = solid
  item Flow {
    auto-layout column
    use-style ${stroke == "dashed" ? direction + "-dashed" : direction}
    item Label { title ${direction} }
  }
}

element "Screen" {
  auto-layout column
  item Box { use Rel as r2 { direction down } }
}
`

// selectDrawn shows the scene of element el of v and selects the node at path, the way the canvas does.
func selectDrawn(t *testing.T, v *Viewer, el *model.Element, path string) {
	t.Helper()
	node := v.elementNode(el)
	var screen *scene.Screen
	var err error
	gfx.NewScene().Update(func(tx *gfx.Tx) {
		screen, err = v.stree.BuildElement(tx, nil, node,
			scene.Options{Measurer: gfx.EstimateText{}, EvalVars: exec.SceneEvalVars})
	})
	if err != nil {
		t.Fatal(err)
	}
	v.sv = newSceneView(el, node)
	if screen != nil {
		v.sv.placed = screen.Placed()
	}
	v.sv.index(v.sv.placed)
	v.state.Current = el
	v.selNodes = []string{path}
}

// A style chosen on a part of an instance replaces the style its expression produces: the override
// names the evaluated style, not the words of the expression, and the inspector reads it back.
func TestPressStyleOnExpression(t *testing.T) {
	v := newDoc(t, &Shell{}, filepath.Join(t.TempDir(), "s.tenn"), exprStyleDoc)
	var screen *model.Element
	for _, e := range v.session.Root().Elements {
		if e.Name == "Screen" {
			screen = e
		}
	}
	if screen == nil {
		t.Fatal("no screen")
	}
	selectDrawn(t, v, screen, "Box.r2/Flow")
	if got := v.nodeStyles(); !slices.Equal(got, []string{"down"}) {
		t.Fatalf("styles before the edit %q", got)
	}
	if err := v.pressStyle("direction", "left"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(v.text, "at Flow { use-style -down left }") {
		t.Fatalf("the override is not the styles the node shows:\n%s", v.text)
	}
	selectDrawn(t, v, screen, "Box.r2/Flow")
	if got := v.nodeStyles(); !slices.Equal(got, []string{"left"}) {
		t.Fatalf("styles after the edit %q", got)
	}
}
