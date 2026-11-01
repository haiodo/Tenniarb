package scene_test

import (
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/scene"
)

// stubEval is the scene.Options.Eval of the tests: the code of an expression to its value.
func stubEval(values map[string]string) func(string) (string, bool) {
	return func(code string) (string, bool) {
		v, ok := values[code]
		return v, ok
	}
}

// buildEval builds element "E" of src with the expressions evaluated by eval.
func buildEval(t *testing.T, src string, eval func(string) (string, bool)) (*scene.Screen, []scene.Error) {
	t.Helper()
	tree, err := scene.LoadSource(t.TempDir(), "e.tenn", src)
	if err != nil {
		t.Fatal(err)
	}
	sc := gfx.NewScene()
	var screen *scene.Screen
	var berr error
	sc.Update(func(tx *gfx.Tx) {
		screen, berr = tree.Build(tx, nil, "E", scene.Options{Measurer: frac{}, Eval: eval})
	})
	if berr != nil {
		t.Fatal(berr)
	}
	return screen, tree.Diagnostics()
}

// TestSceneExpressions: a ${...} in a scene is evaluated, a string with one inside is a template.
func TestSceneExpressions(t *testing.T) {
	screen, diags := buildEval(t, `element "E" {
  item Box { width ${100 + 20}; height ${10}; title "Hi ${who}" }
}`, stubEval(map[string]string{
		"100 + 20":    "120",
		"10":          "10",
		"`Hi ${who}`": "Hi Bob",
	}))
	if len(diags) != 0 {
		t.Errorf("diagnostics: %v", diags)
	}
	box := screen.Boxes()[1]
	if box.W != 120 || box.H != 10 {
		t.Errorf("box %vx%v, want 120x10", box.W, box.H)
	}
	if len(box.Lines) != 1 || box.Lines[0] != "Hi Bob" {
		t.Errorf("title lines %v, want Hi Bob", box.Lines)
	}
}

// TestSceneExpressionFails: without an evaluator an expression is a warning and no value at all.
func TestSceneExpressionFails(t *testing.T) {
	tree, err := scene.LoadSource(t.TempDir(), "e.tenn", `element "E" {
  item Box { auto-layout column; width ${unknown}; height 10 }
}`)
	if err != nil {
		t.Fatal(err)
	}
	sc := gfx.NewScene()
	var screen *scene.Screen
	sc.Update(func(tx *gfx.Tx) { screen, _ = tree.Build(tx, nil, "E", scene.Options{Measurer: frac{}}) })
	var said []string
	for _, d := range screen.Warnings {
		said = append(said, d.Msg)
	}
	if len(said) == 0 || !strings.Contains(said[0], "expressions are not evaluated") {
		t.Fatalf("warnings %v, want one about expressions", said)
	}

	// an evaluator that fails leaves no value: the width falls back to the content
	screen, _ = buildEval(t, `element "E" {
  item Box { width ${unknown}; height 10; title x }
}`, stubEval(nil))
	if w := screen.Boxes()[1].W; w == 0 {
		t.Errorf("the failed expression gave a width")
	}
}
