package scene

import (
	"testing"

	"github.com/haiodo/gowt/gfx"
)

func TestLinkIndexes(t *testing.T) {
	tree, err := Load("../../docs", "Example.tenn")
	if err != nil {
		t.Fatal(err)
	}
	sc := gfx.NewScene()
	var s *Screen
	sc.Update(func(tx *gfx.Tx) { s, err = tree.Build(tx, nil, "Documentation.Basic_steps", Options{}) })
	if err != nil {
		t.Fatal(err)
	}
	var items []*RNode
	for _, k := range s.tree.Kids {
		if k.Kind == RItem && k.Name == "Item" {
			items = append(items, k)
		}
	}
	if len(items) < 2 {
		t.Fatalf("want several items named Item, got %d", len(items))
	}
	found := 0
	for _, k := range s.tree.Kids {
		if k.Kind != RLink || k.From != "Item" || k.To != "hold 'Ctrl'" {
			continue
		}
		found++
		pr := k.prop("source-index")
		if pr == nil {
			t.Fatal("fixture link lost its source-index")
		}
		idx, _ := pr.num(0)
		from := s.p.ends[k][0]
		if from != items[int(idx)] {
			t.Errorf("link with source-index %v joins %v, want that Item in file order", idx, from.Pos)
		}
		if idx == 1 && s.p.frames[from].RY != -51 {
			t.Errorf("second Item at y=%v, want -51 (pos y 51, Y up)", s.p.frames[from].RY)
		}
	}
	if found < 2 {
		t.Fatal("links not found")
	}
	for _, w := range s.Warnings {
		if w.Pos.Line > 248 {
			t.Errorf("warning outside the element: %v", w)
		}
	}
}
