package classic_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/classic"
	"github.com/haiodo/tenniarb/go/exec"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
	"github.com/haiodo/tenniarb/go/scene"
)

// Sizes answer the size of every item the way go/render did: the evaluator asks for `width` and `height`,
// and moving it off the old renderer must not move a box.
func TestSizesMatchRender(t *testing.T) {
	const text = `element "E" {
    item "plain" { pos 0 0; title "plain" }
    item "tall" { pos 0 100; title "tall"; font-size 30 }
    item "wide" { pos 0 200; body %{a body
with two lines} }
}`
	dir := t.TempDir()
	file := filepath.Join(dir, "e.tenn")
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	tree, err := scene.Load(dir, "e.tenn")
	if err != nil {
		t.Fatal(err)
	}
	el := elementNamed(t, model.ReadTenn(text), "E")
	sizes := classic.Sizes(tree, "E", el, gfx.DefaultMeasurer)
	ctx := exec.NewExecutionContext(exec.Options{ItemSize: sizes})
	ctx.SetElement(el)

	n := 0
	for _, it := range el.Items {
		if it.Kind != model.ItemKindItem {
			continue
		}
		n++
		want, err := render.ItemSize(it, render.Options{Measurer: gfx.DefaultMeasurer})
		if err != nil {
			t.Fatalf("%s: %v", it.Name, err)
		}
		got := sizes(it, ctx)
		if got == nil {
			t.Fatalf("%s: no size", it.Name)
		}
		if got.Width != want.Width || got.Height != want.Height {
			t.Errorf("%s: %gx%g, go/render says %gx%g", it.Name, got.Width, got.Height, want.Width, want.Height)
		}
	}
	if n != 3 {
		t.Fatalf("%d items compared, want 3", n)
	}
}

// elementNamed is the element of the file that has this name.
func elementNamed(t *testing.T, m *model.ElementModel, name string) *model.Element {
	t.Helper()
	var found *model.Element
	var walk func(*model.Element)
	walk = func(e *model.Element) {
		if e.Name == name {
			found = e
		}
		for _, c := range e.Elements {
			walk(c)
		}
	}
	walk(&m.Element)
	if found == nil {
		t.Fatalf("no element %q", name)
	}
	return found
}

// ItemAt is the item a node of the element is written for, by its place among the items of the parent.
func TestItemAtByPlace(t *testing.T) {
	const text = `element "E" {
    item "A" { pos 0 0 }
    link A B
    item "B" { pos 10 0 }
}`
	tree, err := scene.LoadSource(t.TempDir(), "e.tenn", text)
	if err != nil {
		t.Fatal(err)
	}
	el := elementNamed(t, model.ReadTenn(text), "E")
	file := tree.File("e.tenn")
	var element *scene.Node
	for _, n := range append(append([]*scene.Node{}, file.Elems...), file.Content...) {
		if n.Kind == scene.KElement {
			element = n
		}
	}
	if element == nil {
		t.Fatal("no element")
	}
	var items []*scene.Node
	for _, c := range element.Content {
		if c.Kind == scene.KItem {
			items = append(items, c)
		}
	}
	if len(items) != 2 {
		t.Fatalf("%d items", len(items))
	}
	if got := classic.ItemAt(el, items[0]); got == nil || got.Name != "A" {
		t.Errorf("first node -> %v", got)
	}
	if got := classic.ItemAt(el, items[1]); got == nil || got.Name != "B" {
		t.Errorf("second node -> %v", got)
	}
}

// A link is matched by its ends and by its place among the links with the same ends, as the file resolves
// a repeated `link A B` by index. It is what a scene needs once it draws a link as an item with a box.
func TestLinkAtByEndsAndPlace(t *testing.T) {
	const text = `element "E" {
    item "A" { pos 0 0 }
    item "B" { pos 100 0 }
    link A B
    link A B
}`
	tree, err := scene.LoadSource(t.TempDir(), "e.tenn", text)
	if err != nil {
		t.Fatal(err)
	}
	el := elementNamed(t, model.ReadTenn(text), "E")
	file := tree.File("e.tenn")
	var element *scene.Node
	for _, n := range append(append([]*scene.Node{}, file.Elems...), file.Content...) {
		if n.Kind == scene.KElement {
			element = n
		}
	}
	if element == nil {
		t.Fatal("no element")
	}
	var links []*scene.Node
	for _, c := range element.Content {
		if c.Kind == scene.KLink {
			links = append(links, c)
		}
	}
	if len(links) != 2 {
		t.Fatalf("%d links", len(links))
	}
	first, second := classic.LinkAt(el, links[0]), classic.LinkAt(el, links[1])
	if first == nil || second == nil {
		t.Fatalf("LinkAt: %v, %v", first, second)
	}
	if first == second {
		t.Fatal("both links map to one item")
	}
	if first.Kind != model.ItemKindLink || second.Kind != model.ItemKindLink {
		t.Fatalf("not links: %v, %v", first.Kind, second.Kind)
	}
}
