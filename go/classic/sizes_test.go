package classic_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/classic"
	"github.com/haiodo/tenniarb/go/exec"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/scene"
)

// Sizes answer the size of every item of the element: the evaluator reads `width` and `height` through it,
// and the box it gives must follow the text - a bigger font makes an item taller, more words make it
// wider. Absolute numbers are the font's, so only the relations are held.
func TestSizesFollowTheText(t *testing.T) {
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

	got := map[string]*model.DrawableSize{}
	for _, it := range el.Items {
		if it.Kind != model.ItemKindItem {
			continue
		}
		sz := sizes(it, ctx)
		if sz == nil {
			t.Fatalf("%s: no size", it.Name)
		}
		if sz.Width <= 0 || sz.Height <= 0 {
			t.Fatalf("%s: %gx%g", it.Name, sz.Width, sz.Height)
		}
		got[it.Name] = sz
	}
	if len(got) != 3 {
		t.Fatalf("%d items sized, want 3", len(got))
	}
	if got["tall"].Height <= got["plain"].Height {
		t.Errorf("a bigger font must make a taller item: %g, %g", got["tall"].Height, got["plain"].Height)
	}
	if got["wide"].Width <= got["plain"].Width || got["wide"].Height <= got["plain"].Height {
		t.Errorf("a body of two lines must make a bigger item: %gx%g, %gx%g",
			got["wide"].Width, got["wide"].Height, got["plain"].Width, got["plain"].Height)
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
