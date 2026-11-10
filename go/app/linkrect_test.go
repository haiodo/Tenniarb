//go:build darwin || linux || windows

package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/scene"
)

// The canvas answers where a link of a classic diagram is: the scene reports the box of its two ends. A
// link has no Placed of its own, so without this the canvas had no box for one and asked go/render.
func TestDrawnRectOfALink(t *testing.T) {
	const text = `element "Old" {
  item A { pos 0 0; width 80; height 40; title "A" }
  item B { pos 200 100; width 80; height 40; title "B" }
  link A B
}`
	dir := t.TempDir()
	file := filepath.Join(dir, "m.tenn")
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	v := newDoc(t, &Shell{}, file, text)
	el := Resolve(&v.session.Root().Element, []string{"Old"})
	if el == nil {
		t.Fatal("no element Old")
	}
	var link *model.DiagramItem
	for _, it := range el.Items {
		if it.Kind == model.ItemKindLink {
			link = it
		}
	}
	if link == nil {
		t.Fatal("no link")
	}
	v.state.Current = el
	v.sv = sceneViewOf(t, dir, el)
	b, ok := v.drawnRect(link)
	if !ok {
		t.Fatal("no box for the link")
	}
	if b.W() != 280 || b.H() != 140 {
		t.Errorf("box %gx%g at %g,%g, want 280x140", b.W(), b.H(), b.Min.X, b.Min.Y)
	}
	// the size row of the inspector reads the same answer: a link is 280x140, not nothing
	v.session.Select([]*model.DiagramItem{link})
	if w, h := v.itemSize(); w != 280 || h != 140 {
		t.Errorf("item size of the link %gx%g, want 280x140", w, h)
	}

	// an item still answers with its own box, not with the union of a link
	var item *model.DiagramItem
	for _, it := range el.Items {
		if it.Kind == model.ItemKindItem {
			item = it
			break
		}
	}
	if b, ok := v.drawnRect(item); !ok || b.W() != 80 || b.H() != 40 {
		t.Errorf("item box %v %v", b, ok)
	}
}

// sceneViewOf builds the view the canvas keeps for el, without a window: the same build, indexed the same.
func sceneViewOf(t *testing.T, dir string, el *model.Element) *sceneView {
	t.Helper()
	tree, err := scene.Load(dir, "m.tenn")
	if err != nil {
		t.Fatal(err)
	}
	file := tree.File("m.tenn")
	var node *scene.Node
	for _, n := range append(append([]*scene.Node{}, file.Elems...), file.Content...) {
		if n.Kind == scene.KElement && n.Key == el.Name {
			node = n
		}
	}
	if node == nil {
		t.Fatal("no node for the element")
	}
	sv := newSceneView(el, node)
	sc := gfx.NewScene()
	sc.Update(func(tx *gfx.Tx) {
		screen, err := tree.Build(tx, nil, el.Name, scene.Options{Measurer: gfx.DefaultMeasurer})
		if err != nil {
			t.Fatal(err)
		}
		sv.screen = screen
		sv.placed = screen.Placed()
		sv.index(screen.Placed())
	})
	return sv
}

// The comment mode asks the same boxes: the smallest item under the pointer, from the picture the canvas
// draws.
func TestItemAtUsesTheDrawnBoxes(t *testing.T) {
	const text = `element "Old" {
  item A { pos 0 0; width 80; height 40; title "A" }
  item B { pos 20 10; width 40; height 20; title "B" }
  link A B
}`
	dir := t.TempDir()
	file := filepath.Join(dir, "m.tenn")
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	v := newDoc(t, &Shell{}, file, text)
	el := Resolve(&v.session.Root().Element, []string{"Old"})
	if el == nil {
		t.Fatal("no element Old")
	}
	v.state.Current = el
	v.sv = sceneViewOf(t, dir, el)

	// a classic pos is the bottom left and y goes up: A covers y 0..40, B covers y -10..10
	if name, _ := v.itemAt(gfx.Point{X: 40, Y: 0}); name != "B" {
		t.Errorf("inside the small item: %q, want B", name)
	}
	if name, _ := v.itemAt(gfx.Point{X: 70, Y: 30}); name != "A" {
		t.Errorf("inside the big item only: %q, want A", name)
	}
	if name, _ := v.itemAt(gfx.Point{X: 500, Y: 500}); name != "" {
		t.Errorf("outside everything: %q", name)
	}
}
