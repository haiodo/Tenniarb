//go:build darwin || linux || windows

package app

import (
	"os"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/classic"
	"github.com/haiodo/tenniarb/go/exec"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/scene"
)

// Every element of docs/Example.tenn builds a scene, and every item it places has a box: the diagram the
// window shows is the one engine's, so an element that fails to build would be a hole on the canvas.
func TestEveryElementOfTheExampleBuilds(t *testing.T) {
	const file = "../../docs/Example.tenn" // from the go directory, as the other tests read it
	const rel = "docs/Example.tenn"        // from the workspace root, as scene.Load wants it
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	m := model.ReadTenn(string(data))
	tree, err := scene.Load("../..", rel)
	if err != nil {
		t.Fatal(err)
	}
	var all []*model.Element
	var walk func(*model.Element)
	walk = func(e *model.Element) {
		all = append(all, e)
		for _, c := range e.Elements {
			walk(c)
		}
	}
	walk(&m.Element)

	built, placed, items := 0, 0, 0
	for _, el := range all {
		if len(el.Items) == 0 {
			continue
		}
		keys := pathKeys(&m.Element, el)
		if len(keys) < 2 {
			continue // the root itself is not a path of the scene
		}
		ctx := execContextOf(el)
		sc := gfx.NewScene()
		var screen *scene.Screen
		var buildErr error
		sc.Update(func(tx *gfx.Tx) {
			screen, buildErr = tree.Build(tx, nil, strings.Join(keys[1:], "."), scene.Options{
				Measurer: gfx.DefaultMeasurer, Classic: classic.Values(el, ctx)})
		})
		if buildErr != nil || screen == nil {
			t.Errorf("%s: %v", el.Name, buildErr)
			continue
		}
		built++
		var find func(p *scene.Placed)
		find = func(p *scene.Placed) {
			for _, k := range p.Kids {
				if k.Src != nil && k.Src.Kind == scene.KItem {
					placed++
					if k.W <= 0 || k.H <= 0 {
						t.Errorf("%s: item %q has the box %gx%g", el.Name, k.Src.Name, k.W, k.H)
					}
				}
				find(k)
			}
		}
		find(screen.Placed())
		items += len(el.Items)
	}
	// A top-level element of a file is a scene, not a classic diagram, so the walk leaves it out: thirty
	// elements of this file are diagrams, and they place the eleven hundred items the canvas draws.
	if built < 25 || placed < 1000 {
		t.Fatalf("%d elements built, %d items placed of %d: the walk or the file changed", built, placed, items)
	}
}

// execContextOf is the evaluator of an element, measuring as the window does.
func execContextOf(el *model.Element) model.IExecutionContext {
	ctx := exec.NewExecutionContext(exec.Options{MeasureText: func(text string, fontSize float64) model.DrawableSize {
		w, h := scene.MeasureText(gfx.DefaultMeasurer, text, fontSize)
		return model.DrawableSize{Width: w, Height: h}
	}})
	ctx.SetElement(el)
	return ctx
}

// pathKeys is the dotted key path of el from the root of the model.
func pathKeys(root, el *model.Element) []string {
	if root == el {
		return []string{root.Name}
	}
	for _, c := range root.Elements {
		if p := pathKeys(c, el); p != nil {
			return append([]string{root.Name}, p...)
		}
	}
	return nil
}
