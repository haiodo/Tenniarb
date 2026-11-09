package scene

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/haiodo/gowt/gfx"
)

// MoveItem is the drag preview: the box moves to the new place and the links attached to it are drawn
// again, so the line follows the box before the model is written.
func TestMoveItemFollowsLinks(t *testing.T) {
	root := t.TempDir()
	text := `element "E" {
  item A { pos 0 0; size 40 }
  item B { pos 200 0; size 40 }
  link A B { display arrow }
}
`
	if err := os.WriteFile(filepath.Join(root, "e.tenn"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	tree, err := Load(root, "e.tenn")
	if err != nil {
		t.Fatal(err)
	}
	var sc *Screen
	gs := gfx.NewScene()
	gs.Update(func(tx *gfx.Tx) {
		sc, err = tree.Build(tx, nil, "E", Options{Measurer: gfx.DefaultMeasurer})
	})
	if err != nil || sc == nil {
		t.Fatalf("build: %v", err)
	}
	gs.Commit()
	var a, link *RNode
	var walk func(rn *RNode)
	walk = func(rn *RNode) {
		switch rn.Kind {
		case RItem:
			if rn.Name == "A" {
				a = rn
			}
		case RLink:
			link = rn
		}
		for _, k := range rn.Kids {
			walk(k)
		}
	}
	walk(sc.tree)
	if a == nil || link == nil {
		t.Fatal("no item A or no link")
	}
	var pl *Placed
	var find func(p *Placed)
	find = func(p *Placed) {
		if p.r == a {
			pl = p
		}
		for _, k := range p.Kids {
			find(k)
		}
	}
	find(sc.Placed())
	if pl == nil {
		t.Fatal("item A is not placed")
	}
	linkNode := sc.gfx[link]
	if linkNode == nil || linkNode.Picture() == nil {
		t.Fatal("the link is not drawn")
	}
	before := linkNode.Picture().Bounds
	gs.Update(func(tx *gfx.Tx) { sc.MoveItem(tx, pl, 0, 120) })
	gs.Commit()
	if tr := pl.Gfx.Transform(); tr[4] != 0 || tr[5] != 120 {
		t.Errorf("item A at %v,%v, want 0,120", tr[4], tr[5])
	}
	after := linkNode.Picture().Bounds
	if after == before {
		t.Fatalf("the link still starts where it did: %v", after)
	}
	if after.Min.Y < before.Min.Y {
		t.Errorf("the link went up to %v from %v, the box moved down", after.Min.Y, before.Min.Y)
	}
}

// An item of a flow is placed by its parent: moving it needs the layout again, so MoveItem refuses.
func TestMoveItemRefusesFlowItems(t *testing.T) {
	root := t.TempDir()
	text := `element "E" {
  item Row { auto-layout column; item A { size 10 } }
}
`
	if err := os.WriteFile(filepath.Join(root, "e.tenn"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	tree, err := Load(root, "e.tenn")
	if err != nil {
		t.Fatal(err)
	}
	var sc *Screen
	gs := gfx.NewScene()
	gs.Update(func(tx *gfx.Tx) {
		sc, err = tree.Build(tx, nil, "E", Options{Measurer: gfx.DefaultMeasurer})
	})
	if err != nil || sc == nil {
		t.Fatalf("build: %v", err)
	}
	var pl *Placed
	var find func(p *Placed)
	find = func(p *Placed) {
		if p.Src != nil && p.Src.Name == "A" {
			pl = p
		}
		for _, k := range p.Kids {
			find(k)
		}
	}
	find(sc.Placed())
	if pl == nil {
		t.Fatal("no item A")
	}
	ok := false
	gs.Update(func(tx *gfx.Tx) { ok = sc.MoveItem(tx, pl, 50, 50) })
	if ok {
		t.Error("MoveItem moved an item of a flow")
	}
}
