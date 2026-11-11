package scene_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/exec"
	"github.com/haiodo/tenniarb/go/scene"
)

// ctxDoc is one screen with the context declared where the test puts it: on the element or on the root of
// the file, with the child reading both a colour and a title (spec 5f).
const ctxDoc = `element "Screen" {
  item Box {
    width 10; height 10
    color ${ctx.tone === "dark" ? "#000000" : "#ffffff"}
    title ${ctx.tone}
  }
}`

// TestContextOnElement: a provider on the element itself is inherited by its items, in a colour and in a
// title. The element is a scene here: the expressions are the scene's, not the element model's.
func TestContextOnElement(t *testing.T) {
	screen, svg := paramsBuild(t, paramsTree(t, "elem.tenn", `context tone ?= light
element "Screen" {
  context tone dark
  item Box {
    width 10; height 10
    color ${ctx.tone === "dark" ? "#000000" : "#ffffff"}
    title ${ctx.tone}
  }
}`), "Screen")
	if l := boxMap(screen)["Box"].Lines; len(l) != 1 || l[0] != "dark" {
		t.Errorf("Box title %v, want dark from the element", l)
	}
	if !strings.Contains(svg, "rgb(0,0,0)") {
		t.Errorf("Box is not painted dark:\n%s", svg)
	}
	if len(screen.Warnings) != 0 {
		t.Errorf("warnings: %v", screen.Warnings)
	}
}

// TestContextOnFileRoot: the root of the file is a node above every element in it, so an element built on
// its own sees the context declared there.
func TestContextOnFileRoot(t *testing.T) {
	screen, svg := paramsBuild(t, paramsTree(t, "file.tenn", `context tone dark
`+ctxDoc), "Screen")
	if l := boxMap(screen)["Box"].Lines; len(l) != 1 || l[0] != "dark" {
		t.Errorf("Box title %v, want dark from the file root", l)
	}
	if !strings.Contains(svg, "rgb(0,0,0)") {
		t.Errorf("Box is not painted dark:\n%s", svg)
	}
	if len(screen.Warnings) != 0 {
		t.Errorf("warnings: %v", screen.Warnings)
	}
}

// TestContextSoftKeepsInherited: ?= fills only a key nothing above has, on the element as on an item.
func TestContextSoftKeepsInherited(t *testing.T) {
	screen, _ := paramsBuild(t, paramsTree(t, "soft.tenn", `context tone dark
element "Screen" {
  context tone ?= light
  item Box { context tone ?= navy; width 10; height 10; title ${ctx.tone} }
}`), "Screen")
	if l := boxMap(screen)["Box"].Lines; len(l) != 1 || l[0] != "dark" {
		t.Errorf("Box title %v, want the inherited dark: ?= does not overrule it", l)
	}
	if len(screen.Warnings) != 0 {
		t.Errorf("warnings: %v", screen.Warnings)
	}
}

// TestContextClassicModel: a classic diagram reads its expressions from the element model, which has no
// ctx; a property that names ctx is the scene's to evaluate, or the provider on the element and on the
// file root would never reach the items (spec 5f).
func TestContextClassicModel(t *testing.T) {
	screen, svg := contextClassicBuild(t, `context tone dark
element "E" {
  context tone ?= light
  item Box {
    width 10; height 10
    color ${ctx.tone === "dark" ? "#000000" : "#ffffff"}
    title ${ctx.tone}
  }
}`, classicStub(map[string][]string{
		// What the element model answers for an expression it cannot evaluate: its scope has no ctx.
		"Box|color": {"ReferenceError: ctx is not defined"},
		"Box|title": {"ReferenceError: ctx is not defined"},
	}, nil))
	if l := boxMap(screen)["Box"].Lines; len(l) != 1 || l[0] != "dark" {
		t.Errorf("Box title %v, want dark, not the model's text", l)
	}
	if !strings.Contains(svg, "rgb(0,0,0)") {
		t.Errorf("Box is not painted dark:\n%s", svg)
	}
	if len(screen.Warnings) != 0 {
		t.Errorf("warnings: %v", screen.Warnings)
	}
}

// contextClassicBuild builds element "E" of src as a classic diagram, with the model values of valu and the
// scene evaluator in scope, as the app and tennlayout do.
func contextClassicBuild(t *testing.T, src string, valu func(scene.ClassicRef, string) ([]string, bool)) (*scene.Screen, string) {
	t.Helper()
	tree, err := scene.LoadSource(t.TempDir(), "e.tenn", src)
	if err != nil {
		t.Fatal(err)
	}
	sc := gfx.NewScene()
	var screen *scene.Screen
	sc.Update(func(tx *gfx.Tx) {
		screen, err = tree.Build(tx, nil, "E", scene.Options{Measurer: frac{}, EvalVars: exec.SceneEvalVars, Classic: valu})
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

// TestContextFileRootDefault: a `?=` on the root of the file a component is written in travels with it to
// another file, and the place of the insertion still wins (spec 5f).
func TestContextFileRootDefault(t *testing.T) {
	tree := contextTwoFiles(t, `context tone ?= light
item Chip { width 20; height 10; title ${ctx.tone} }
`, `element "lib" --from-file "lib.tenn"
element "Screen" {
  item Plain { use lib.Chip as plain }
  item Dark { context tone dark; use lib.Chip as dark }
}`)
	screen, _ := paramsBuild(t, tree, "Screen")
	m := boxMap(screen)
	if l := m["Plain.plain"].Lines; len(l) != 1 || l[0] != "light" {
		t.Errorf("Plain title %v, want the ?= default of the component file", l)
	}
	if l := m["Dark.dark"].Lines; len(l) != 1 || l[0] != "dark" {
		t.Errorf("Dark title %v, want the place of the insertion", l)
	}
	if len(screen.Warnings) != 0 {
		t.Errorf("warnings: %v", screen.Warnings)
	}
}

// TestContextFileRootHardStaysInFile: a declaration without `?=` on the root of a file binds the content of
// that file, not the component used in another one: the key is missing, which is a diagnostic (spec 5f).
func TestContextFileRootHardStaysInFile(t *testing.T) {
	tree := contextTwoFiles(t, `context tone dark
item Chip { width 20; height 10; title ${ctx.tone} }
`, `element "lib" --from-file "lib.tenn"
element "Screen" { item Plain { use lib.Chip as plain } }`)
	screen, _ := paramsBuild(t, tree, "Screen")
	if l := boxMap(screen)["Plain.plain"].Lines; len(l) == 1 && l[0] == "dark" {
		t.Errorf("Plain title %v: a hard declaration on the file root crossed into another file", l)
	}
	reported := false
	for _, w := range screen.Warnings {
		if strings.Contains(w.Msg, "is not evaluated") {
			reported = true
		}
	}
	if !reported {
		t.Errorf("warnings %v, want the missing ctx.tone reported", screen.Warnings)
	}
}

// contextTwoFiles writes lib and main into one folder and loads main, which mounts lib as an element.
func contextTwoFiles(t *testing.T, lib, main string) *scene.Tree {
	t.Helper()
	root := t.TempDir()
	for name, text := range map[string]string{"lib.tenn": lib, "main.tenn": main} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tree, err := scene.Load(root, "main.tenn")
	if err != nil {
		t.Fatal(err)
	}
	return tree
}
