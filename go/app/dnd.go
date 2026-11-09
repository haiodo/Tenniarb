//go:build darwin || linux || windows

package app

import (
	"strings"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/gfx"

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

// componentDragPrefix marks the text a component row carries: the canvas tells it from a text drop of a
// row of the outline, which pastes an element.
const componentDragPrefix = "\x00tenniarb-component:"

// dragComponents lets a row of the component browser of the Items pane be dragged onto the canvas: the
// drag carries the use path of the component, and the drop writes the use where it lands.
func (v *Viewer) dragComponents() {
	l := &v.itemsUI
	if l.tree == nil || v.view == nil {
		return
	}
	// The canvas takes the drop: x, y are its own points, and the camera turns them into the scene.
	var last [2]int
	g.DropOn(v.view.Canvas(), g.Drop{
		Ops:  g.OpCopy,
		Over: func(x, y int) bool { last = [2]int{x, y}; return true },
		Text: func(text string, _ g.Op) { v.takeComponentDrop(text, last[0], last[1]) },
	})
	g.DragFrom(l.tree, g.Drag{
		Ops: g.OpCopy,
		OnStart: func() bool {
			if !l.components {
				return false
			}
			sel := l.tree.Selected()
			if len(sel) != 1 {
				return false
			}
			r, _ := sel[0].Data().(*ItemRow)
			return r != nil && r.Component
		},
		Text: func() string {
			sel := l.tree.Selected()
			if len(sel) != 1 {
				return ""
			}
			r, _ := sel[0].Data().(*ItemRow)
			if r == nil || !r.Component {
				return ""
			}
			return componentDragPrefix + r.Path
		},
	})
}

// takeComponentDrop is the canvas accepting a text drop: a component of the browser puts the use where the
// pointer is, anything else is left to the other handlers.
func (v *Viewer) takeComponentDrop(text string, x, y int) bool {
	if !strings.HasPrefix(text, componentDragPrefix) {
		return false
	}
	v.dropComponent(strings.TrimPrefix(text, componentDragPrefix), x, y)
	return true
}

// dropComponent is the canvas accepting a component dragged out of the browser: the use goes where the
// pointer was, and the new instance is selected.
func (v *Viewer) dropComponent(path string, x, y int) {
	if v.componentOfPath(path) == nil || !v.ensureSceneElement() {
		return // a browser built for another library, or a document that cannot take a component
	}
	name := path
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		name = name[i+1:]
	}
	v.insertUseAt(nil, "", path, name, v.dropPoint(x, y))
}

// dropPoint is the scene point of a view point of the canvas: where a drop put the pointer.
func (v *Viewer) dropPoint(x, y int) *gfx.Point {
	if v.view == nil {
		return nil
	}
	p := ViewToScene(v.view.Compositor().Camera(), gfx.Point{X: float64(x), Y: float64(y)})
	return &p
}
