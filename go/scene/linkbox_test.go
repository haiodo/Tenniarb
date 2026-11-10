package scene_test

import (
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/scene"
)

// A link has no box of its own in the layout, and the canvas needs one: where the line is drawn is the box
// of its two ends together, in the coordinates of the item that holds them.
func TestLinkBoxIsTheBoxOfItsEnds(t *testing.T) {
	const text = `element "Old" {
  item A { pos 0 0; width 80; height 40 }
  item B { pos 200 100; width 80; height 40 }
  link A B
}`
	tree, err := scene.LoadSource(t.TempDir(), "m.tenn", text)
	if err != nil {
		t.Fatal(err)
	}
	sc := gfx.NewScene()
	var screen *scene.Screen
	sc.Update(func(tx *gfx.Tx) {
		screen, err = tree.Build(tx, nil, "Old", scene.Options{Measurer: gfx.DefaultMeasurer})
	})
	if err != nil {
		t.Fatal(err)
	}
	file := tree.File("m.tenn")
	var element *scene.Node
	for _, n := range append(append([]*scene.Node{}, file.Elems...), file.Content...) {
		if n.Kind == scene.KElement {
			element = n
		}
	}
	if element == nil {
		t.Fatal("no element")
	}
	var link, item *scene.Node
	for _, c := range element.Content {
		switch c.Kind {
		case scene.KLink:
			link = c
		case scene.KItem:
			if item == nil {
				item = c
			}
		}
	}
	if link == nil || item == nil {
		t.Fatalf("link %v item %v", link, item)
	}
	box, ok := screen.LinkBox(link)
	if !ok {
		t.Fatal("no box for the link")
	}
	if box.W() != 280 || box.H() != 140 {
		t.Errorf("box %gx%g, want 280x140", box.W(), box.H())
	}
	if _, ok := screen.LinkBox(item); ok {
		t.Error("an item has a link box")
	}
}
