//go:build darwin || linux || windows

package app

import (
	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/swt"

	"github.com/haiodo/tenniarb/go/diff"
	"github.com/haiodo/tenniarb/go/geometry"
	"github.com/haiodo/tenniarb/go/model"
)

func (v *Viewer) deleteItems() {
	if v.session != nil {
		v.session.DeleteSelection()
	}
}

func (v *Viewer) newItem() {
	if v.sceneMode() {
		v.addSceneItem("Frame")
		return
	}
	if v.session != nil {
		v.session.AddNewItem(false)
	}
}

// selectElement puts the element on the canvas; the camera of the one left is remembered.
func (v *Viewer) selectElement(e *model.Element) {
	if v.session != nil && e != nil && e != v.state.Current {
		v.session.SetElement(e)
	}
}

func (v *Viewer) onOutlineSelect(n *g.Node) {
	if v.syncing || n == nil {
		return
	}
	if v.outlineAuto {
		v.outlineAuto = false
		// GTK puts the cursor on the first row when the tree gains focus with nothing selected. The
		// canvas then shows an element the outline has no row for, so that is not a click.
		if roots := v.tree.Roots(); len(roots) > 0 && n.Data() != nil && n.Data() == roots[0].Data() {
			v.syncing = true
			defer func() { v.syncing = false }()
			v.tree.Deselect()
			return
		}
	}
	switch d := n.Data().(type) {
	case *model.Element:
		v.selectElement(d)
	case *diff.Node:
		v.selectCompare(d)
	}
}

// applyInitialCamera places the view for the element on the canvas once the canvas has a size.
func (v *Viewer) applyInitialCamera() { v.placeCamera(v.pendingFit) }

// placeCamera puts the element on the canvas: the viewport it had when the outline switched away from
// it, else its fit for the first element of a document, else the element centred at the zoom of the one
// left — a switch then moves the canvas without jumping in scale between elements of different size.
func (v *Viewer) placeCamera(fit bool) {
	w, h := v.view.Canvas().Size()
	cw, ch, ok := v.contentSize()
	if !ok {
		v.pendingFit = false
		return
	}
	if cam, ok := v.state.Camera(v.state.Current); ok {
		v.setCamera(cam)
		v.pendingFit = false
		return
	}
	if w <= 0 || h <= 0 {
		return
	}
	if fit {
		v.setCamera(FitCamera(cw, ch, float64(w), float64(h)))
	} else {
		v.setCamera(CenterCamera(v.view.Compositor().Camera().Zoom, cw, ch, float64(w), float64(h)))
	}
	v.pendingFit = false
}

func (v *Viewer) fitView() {
	w, h := v.view.Canvas().Size()
	cw, ch, _ := v.contentSize()
	v.setCamera(FitCamera(cw, ch, float64(w), float64(h)))
}

// saveCamera remembers the viewport of the element on the canvas, unless it was never placed.
func (v *Viewer) saveCamera() {
	if _, _, ok := v.contentSize(); v.state.Current != nil && ok && !v.pendingFit {
		v.state.Remember(v.state.Current, v.view.Compositor().Camera())
	}
}

func (v *Viewer) zoomBy(f float64) {
	w, h := v.view.Canvas().Size()
	comp := v.view.Compositor()
	v.setCamera(ZoomCamera(comp.Camera(), f, float64(w)/2, float64(h)/2))
}

// resetZoom is Swift's Reset Zoom: zoom 1, the diagram centered.
func (v *Viewer) resetZoom() {
	cw, ch, ok := v.contentSize()
	if !ok {
		return
	}
	w, h := v.view.Canvas().Size()
	v.setCamera(CenterCamera(1, cw, ch, float64(w), float64(h)))
}

// visibleScene is the part of the editor scene the canvas shows now.
func (v *Viewer) visibleScene() geometry.Rect {
	w, h := v.view.Canvas().Size()
	cam := v.view.Compositor().Camera()
	a := v.smap.ToScene(ViewToScene(cam, gfx.Point{}))
	b := v.smap.ToScene(ViewToScene(cam, gfx.Point{X: float64(w), Y: float64(h)}))
	return geometry.Rect{X: a.X, Y: b.Y, Width: b.X - a.X, Height: a.Y - b.Y}
}

// outlineKeys is Swift's keyDown of the outline: Tab adds an item to the canvas and moves the focus there.
func (v *Viewer) outlineKeys(t *g.TreeView) {
	tree := t.Unwrap()
	// The traversal would move the focus before the key reaches the key listener.
	tree.AddListener(swt.Traverse, listenerFunc(func(e *swt.Event) {
		if e.Detail == swt.TRAVERSE_TAB_NEXT {
			e.Doit = false
		}
	}))
	tree.AddKeyListener(swt.KeyListenerKeyPressedAdapter(func(e *swt.KeyEvent) {
		if e.KeyCode != int32(swt.TAB) || v.session == nil || v.session.Readonly() {
			return
		}
		e.Doit = false
		if v.sceneMode() {
			v.addSceneItem("Frame")
		} else {
			v.session.AddTopItem()
		}
		v.view.Canvas().Unwrap().SetFocus()
	}))
}
