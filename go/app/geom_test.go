//go:build darwin || linux || windows

package app

import (
	"testing"

	"github.com/haiodo/tenniarb/go/model"
)

// A box of the canvas is the box of an item the element still holds: a deleted one answers none, so the
// session lets go of it. A deleted link that still answers a box stays in the selection, and the next
// press with the line modifier picks it up as the item under the pointer instead of drawing a line.
func TestDeletedItemHasNoBox(t *testing.T) {
	const text = "element \"D\" {\n  item A { pos 0 0 }\n  item B { pos 200 0 }\n  link \"A\" \"B\" { display arrow }\n}\n"
	v, _, el := classicView(t, text)
	var link *model.DiagramItem
	for _, it := range el.Items {
		if it.Kind == model.ItemKindLink {
			link = it
		}
	}
	if link == nil {
		t.Fatal("the fixture has no link")
	}
	if _, ok := v.session.Geometry().Box(link); !ok {
		t.Fatal("the link drawn between A and B has no box")
	}
	v.session.Select([]*model.DiagramItem{link})
	v.session.DeleteSelection()
	if _, ok := v.session.Geometry().Box(link); ok {
		t.Error("a deleted link still answers a box")
	}
	if sel := v.session.Selection(); len(sel) != 0 {
		t.Errorf("the selection kept %d deleted items", len(sel))
	}
}
