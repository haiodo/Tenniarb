//go:build darwin || linux || windows

package app

import (
	"math"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/view"
)

// startActions are the rows of the start hint, in the order a click takes them.
var startActions = []string{"New screen", "New brain map item", "Insert component"}

// startFont is the face of the hint rows.
var startFont = gfx.Font{Size: 13}

// hintRow is one clickable row of the start hint, its box in view points.
type hintRow struct {
	label string
	box   gfx.Rect
}

// hintUI is the start hint over an empty element: one HUD node with a picture and the boxes of its
// rows, so a click can find the row under the pointer.
type hintUI struct {
	node *gfx.Node
	key  hintKey
	rows []hintRow
}

// hintKey is what the hint picture depends on: whether it is shown, the view size and the theme.
type hintKey struct {
	show bool
	w, h float64
	dark bool
}

// emptyElement reports whether the element on the canvas writes nothing itself: no classic items and
// no scene nodes. The start hint is for that.
func (v *Viewer) emptyElement() bool {
	e := v.state.Current
	if e == nil || len(e.Items) > 0 || v.state.Error != "" {
		return false
	}
	if v.sv != nil && v.sv.err != nil {
		return false
	}
	n := v.elementNode(e)
	return n == nil || len(sceneKids(n)) == 0
}

// syncHint draws or drops the start hint after the element, the view size or the theme changed.
func (v *Viewer) syncHint() {
	if v.view == nil {
		return
	}
	comp := v.view.Compositor()
	w, h, _ := comp.Viewport()
	key := hintKey{show: v.emptyElement(), w: w, h: h, dark: v.app.Dark()}
	hint := &v.hint
	if key == hint.key {
		return
	}
	hint.key = key
	if !key.show {
		hint.rows = nil
		if hint.node != nil {
			node := hint.node
			hint.node = nil
			comp.HUD().Update(func(tx *gfx.Tx) { tx.Remove(node) })
		}
		return
	}
	pic, rows, card := startHintPicture(w, h, key.dark)
	if pic == nil {
		hint.rows = nil
		if hint.node != nil {
			node := hint.node
			hint.node = nil
			comp.HUD().Update(func(tx *gfx.Tx) { tx.Remove(node) })
		}
		return
	}
	hint.rows = rows
	comp.HUD().Update(func(tx *gfx.Tx) {
		if hint.node == nil {
			hint.node = gfx.NewNode(4)
			tx.Add(nil, hint.node)
		}
		tx.SetPicture(hint.node, pic)
		shape := &gfx.Path{}
		shape.Rect(card.Min.X, card.Min.Y, card.W(), card.H())
		tx.SetShape(hint.node, shape, gfx.HitFill)
	})
}

// startHintPicture draws the hint card in the middle of the view, a row per action, and returns the
// picture, the boxes of the rows and the card, in view points. nil when the view is too small.
func startHintPicture(vw, vh float64, dark bool) (*gfx.Picture, []hintRow, gfx.Rect) {
	const pad, rowH, gap, margin = 18.0, 24.0, 4.0, 20.0
	text := 0.0
	for _, s := range startActions {
		text = math.Max(text, gfx.DefaultMeasurer.MeasureText(s, startFont).Width)
	}
	bw := text + 2*pad
	bh := 2*pad + float64(len(startActions))*rowH + float64(len(startActions)-1)*gap
	if vw < bw+2*margin || vh < bh+2*margin {
		return nil, nil, gfx.Rect{}
	}
	card := gfx.R(math.Round((vw-bw)/2), math.Round((vh-bh)/2), bw, bh)
	bg, fg, line := gfx.RGBA(255, 255, 255, 0.9), gfx.RGB(60, 60, 60), gfx.RGBA(0, 0, 0, 0.1)
	if dark {
		bg, fg, line = gfx.RGBA(45, 45, 48, 0.9), gfx.RGB(225, 225, 227), gfx.RGBA(255, 255, 255, 0.1)
	}
	rec := gfx.NewRecorder()
	rec.FillRRect(card, 10, bg)
	rec.StrokeRRect(card, 10, gfx.StrokeStyle{Width: 1}, line)
	rows := make([]hintRow, len(startActions))
	for i, s := range startActions {
		y := card.Min.Y + pad + float64(i)*(rowH+gap)
		m := gfx.DefaultMeasurer.MeasureText(s, startFont)
		rec.Text(s, card.Min.X+pad, y+(rowH-(m.Ascent+m.Descent))/2+m.Ascent, startFont, fg)
		rows[i] = hintRow{label: s, box: gfx.R(card.Min.X+pad, y, bw-2*pad, rowH)}
	}
	return rec.Finish(), rows, card
}

// hintPointer handles a press on the start hint, and the drag and release that belong to it. A press
// that began elsewhere is left to the canvas.
func (v *Viewer) hintPointer(p *view.Pointer) bool {
	hint := &v.hint
	if hint.node == nil {
		return false
	}
	if p.Kind == view.PointerDown {
		if p.HUD != hint.node || p.Button != 1 {
			return false
		}
		v.press = press{active: true, hint: true, pan: true, cam: v.view.Compositor().Camera(), pos: p.Pos, from: p.Pos}
		v.view.SetCursor(view.CursorGrabbing)
		return true
	}
	if !v.press.hint {
		return false
	}
	switch p.Kind {
	case view.PointerMove:
		if !p.Drag {
			return true
		}
		v.panView(p.Pos.X-v.press.pos.X, p.Pos.Y-v.press.pos.Y)
		v.press.pos = p.Pos
	case view.PointerUp, view.PointerCancel:
		pr := v.press
		v.press = press{}
		v.view.SetCursor(view.CursorArrow)
		if p.Kind == view.PointerUp && math.Hypot(p.Pos.X-pr.from.X, p.Pos.Y-pr.from.Y) <= 3 {
			v.hintClick(p.Pos)
		}
	}
	return true
}

// hintClick runs what the hint row under p stands for.
func (v *Viewer) hintClick(p gfx.Point) {
	for i, r := range v.hint.rows {
		if r.box.Contains(p) {
			v.startAction(i)
			return
		}
	}
}

// startAction is row i of the start hint: the first of the empty element.
func (v *Viewer) startAction(i int) {
	switch i {
	case 0: // New screen: the sizes, as Element > New Screen offers them
		v.sizeMenu = true
		v.app.Async(func() { v.canvasMenu.Show() })
	case 1: // New brain map item, as Edit > Add > New Item
		if v.session != nil {
			v.session.AddTopItem()
		}
	case 2: // Insert component: with no library it says so
		v.showComponentPicker()
	}
}
