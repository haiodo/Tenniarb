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

// TestExpressionSizeWord: width and height take the word an expression gives, as they take a number: a
// parametrized item with `width ${shape === "icon" ? 44 : "hug"}` is 44 wide for an icon and hugs its text
// otherwise (spec 6, 5c).
func TestExpressionSizeWord(t *testing.T) {
	screen, _ := paramsBuild(t, paramsTree(t, "size.tenn", `element "Button" kind Component {
  export let shape = text
  item Btn {
    auto-layout row
    width ${shape === "icon" ? 44 : "hug"}
    height 20
    title "Hi"
  }
}
element "Screen" {
  item col {
    auto-layout column
    use Button as icon { shape icon }
    use Button as text
  }
}`), "Screen")
	m := map[string]scene.Box{}
	for _, b := range screen.Boxes() {
		m[b.Path] = b
	}
	if w := m["col.icon.Btn"].W; w != 44 {
		t.Errorf("icon width %v, want the number 44 of the expression", w)
	}
	// hug is the title "Hi" of the frac measurer: 10 a letter
	if w := m["col.text.Btn"].W; w != 20 {
		t.Errorf("text width %v, want the hug size 20 of the expression", w)
	}
	if len(screen.Warnings) != 0 {
		t.Errorf("warnings: %v", screen.Warnings)
	}
}

// TestExpressionSizeInStyle: a property of a style row is evaluated too, in the scope of the instance whose
// node names the style.
func TestExpressionSizeInStyle(t *testing.T) {
	screen, _ := paramsBuild(t, paramsTree(t, "style.tenn", `element "Button" kind Component {
  export let shape = text
  styles {
    icon   { width ${shape === "icon" ? 44 : "hug"} }
    round  { corner-radius ${shape === "icon" ? 12 : 4} }
  }
  item Btn {
    auto-layout row
    height 20
    title "Hi"
    use-style icon round
  }
}
element "Screen" {
  item col {
    auto-layout column
    use Button as icon { shape icon }
    use Button as text
  }
}`), "Screen")
	m := map[string]scene.Box{}
	for _, b := range screen.Boxes() {
		m[b.Path] = b
	}
	if w := m["col.icon.Btn"].W; w != 44 {
		t.Errorf("icon width %v, want 44 from the style row", w)
	}
	if w := m["col.text.Btn"].W; w != 20 {
		t.Errorf("text width %v, want the hug size 20 from the style row", w)
	}
	if len(screen.Warnings) != 0 {
		t.Errorf("warnings: %v", screen.Warnings)
	}
}

// TestExpressionSizeInUseOverride: a property of the block of a use is evaluated in the scope of the
// instance root, and at reaches a node inside it with the same scope.
func TestExpressionSizeInUseOverride(t *testing.T) {
	screen, _ := paramsBuild(t, paramsTree(t, "override.tenn", `element "Card" kind Component {
  export let w = 40
  item Box { height 20 }
}
element "Screen" {
  item col {
    auto-layout column
    use Card as a { width ${w + 2}; at Box { width ${w * 2} } }
  }
}`), "Screen")
	m := map[string]scene.Box{}
	for _, b := range screen.Boxes() {
		m[b.Path] = b
	}
	if w := m["col.a"].W; w != 42 {
		t.Errorf("instance width %v, want 42 from ${w + 2}", w)
	}
	if w := m["col.a.Box"].W; w != 80 {
		t.Errorf("box width %v, want 80 from ${w * 2}", w)
	}
	if len(screen.Warnings) != 0 {
		t.Errorf("warnings: %v", screen.Warnings)
	}
}
