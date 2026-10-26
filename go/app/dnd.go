//go:build darwin || linux || windows

package app

import (
	g "github.com/haiodo/gowt"

	"github.com/haiodo/tenniarb/go/model"
)

func elementOf(n *g.Node) *model.Element {
	if n == nil {
		return nil
	}
	e, _ := n.Data().(*model.Element)
	return e
}

// dragAndDrop lets the outline rows be dragged: onto a row to move the element inside it, to the top
// or bottom edge of a row to put it next to it (Swift outlineView acceptDrop).
func (v *Viewer) dragAndDrop() {
	g.DragFrom(v.tree, g.Drag{
		Ops: g.OpMove | g.OpCopy,
		OnStart: func() bool {
			// the press that starts a drag has selected the row
			if sel := v.tree.Selected(); len(sel) == 1 {
				v.dragging = elementOf(sel[0])
			}
			return v.session != nil && v.dragging != nil && v.dragging.Parent != nil
		},
		Text: func() string { return v.session.CopyElement(v.dragging) },
	})
	g.DropOn(v.tree, g.Drop{
		Ops:  g.OpMove | g.OpCopy,
		Over: func(x, y int) bool { v.dropAt = [2]int{x, y}; return v.dragging != nil },
		Text: func(string, g.Op) { v.dropElement() },
	})
}

// dropElement moves the dragged element to where the drag was last over the outline.
func (v *Viewer) dropElement() {
	dragged := v.dragging
	v.dragging = nil
	if v.session == nil || dragged == nil || v.state.Doc.Model == nil {
		return
	}
	x, y := v.dropAt[0], v.dropAt[1]
	n, _ := v.tree.CellAt(x, y)
	zone := DropInto
	if n != nil {
		b := n.Unwrap().GetBounds()
		zone = ZoneAt(y, int(b.Y), int(b.Height))
	}
	root := &v.state.Doc.Model.Element
	if parent, index, ok := DropPlace(root, dragged, elementOf(n), zone); ok {
		v.session.MoveElement(dragged, parent, index)
	}
}
