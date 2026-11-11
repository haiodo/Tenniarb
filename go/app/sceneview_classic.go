//go:build darwin || linux || windows

package app

import (
	"math"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/classic"
	"github.com/haiodo/tenniarb/go/geometry"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/scene"
)

// previewClassic draws what the session previews in a classic diagram: the dragged items where the drop
// will write them, the rubber band, and the line of a link being drawn.
func (v *Viewer) previewClassic() {
	sv := v.sv
	sv.band, sv.line = nil, nil
	if v.session == nil {
		return
	}
	if b := v.session.Band(); b != nil {
		r := v.smap.RectToGfx(*b)
		sv.band = &r
		return
	}
	if v.session.DrawingLine() {
		if a, b, ok := v.linePreview(); ok {
			sv.line = &[2]gfx.Point{a, b}
		}
		return
	}
	if v.dragAt == v.press.scene {
		return
	}
	v.scene.Update(func(tx *gfx.Tx) { v.dragPreview(v.press.scene, v.dragAt, tx) })
}

// linePreview is the line a link is drawn along, in gfx scene coordinates: from the edge of the source box
// to the edge of the item under the pointer, or to the pointer itself.
func (v *Viewer) linePreview() (gfx.Point, gfx.Point, bool) {
	sel := v.session.Selection()
	if len(sel) == 0 {
		return gfx.Point{}, gfx.Point{}, false
	}
	from, ok := v.drawnRect(sel[0])
	if !ok {
		return gfx.Point{}, gfx.Point{}, false
	}
	mid := gfx.Point{X: from.Min.X + from.W()/2, Y: from.Min.Y + from.H()/2}
	end := v.smap.ToGfx(geometry.Point{X: v.dragAt.X, Y: v.dragAt.Y})
	var target gfx.Rect
	if t := v.session.LineTarget(); t != nil {
		if r, ok := v.drawnRect(t); ok {
			target = r
			end = gfx.Point{X: r.Min.X + r.W()/2, Y: r.Min.Y + r.H()/2}
		}
	}
	if p, ok := crossBoxPoint(mid, end, from); ok {
		mid = p
	}
	if target.W() > 0 {
		if p, ok := crossBoxPoint(mid, end, target); ok {
			end = p
		}
	}
	return mid, end, true
}

// crossBoxPoint is where the segment mid-end leaves the box: the nearest edge crossing, and false when the
// segment does not meet it.
func crossBoxPoint(mid, end gfx.Point, r gfx.Rect) (gfx.Point, bool) {
	edges := [4][2]gfx.Point{
		{{X: r.Min.X, Y: r.Min.Y}, {X: r.Max.X, Y: r.Min.Y}},
		{{X: r.Min.X, Y: r.Min.Y}, {X: r.Min.X, Y: r.Max.Y}},
		{{X: r.Min.X, Y: r.Max.Y}, {X: r.Max.X, Y: r.Max.Y}},
		{{X: r.Max.X, Y: r.Min.Y}, {X: r.Max.X, Y: r.Max.Y}},
	}
	for _, e := range edges {
		if p, ok := crossSegments(mid, end, e[0], e[1]); ok {
			return p, true
		}
	}
	return gfx.Point{}, false
}

// crossSegments is the point where two segments meet, false when they do not.
func crossSegments(a, b, c, d gfx.Point) (gfx.Point, bool) {
	rx, ry := b.X-a.X, b.Y-a.Y
	sx, sy := d.X-c.X, d.Y-c.Y
	den := rx*sy - ry*sx
	if den == 0 {
		return gfx.Point{}, false
	}
	t := ((c.X-a.X)*sy - (c.Y-a.Y)*sx) / den
	u := ((c.X-a.X)*ry - (c.Y-a.Y)*rx) / den
	if t < 0 || t > 1 || u < 0 || u > 1 {
		return gfx.Point{}, false
	}
	return gfx.Point{X: a.X + t*rx, Y: a.Y + t*ry}, true
}

// dragPreview moves the drawn items of a classic diagram by the pointer from to to, both in session
// coordinates: every item lands where the drop rounds its position, so the picture under the pointer is
// where the commit writes it.
func (v *Viewer) dragPreview(from, to geometry.Point, tx *gfx.Tx) {
	sv := v.sv
	if sv.dragBase == nil {
		// The drawn place of an item moves with the pointer, so the start is kept once per press.
		sv.dragBase = map[*scene.Placed]gfx.Point{}
		for _, it := range v.sessionItems() {
			if pl := v.classicPlaced(it); pl != nil {
				sv.dragBase[pl] = gfx.Point{X: pl.X, Y: pl.Y}
			}
		}
	}
	dx, dy := to.X-from.X, to.Y-from.Y
	for _, it := range v.sessionItems() {
		if it.Kind != model.ItemKindItem {
			continue
		}
		pl := v.classicPlaced(it)
		if pl == nil || pl.Gfx == nil {
			continue
		}
		base := sv.dragBase[pl]
		x, y := dragRound(it.X+dx), dragRound(it.Y+dy)
		if sv.screen.MoveItem(tx, pl, base.X+x-it.X, base.Y-(y-it.Y)) {
			sv.moved = true
		}
	}
}

// dragRound is the editor's roundCoord: the preview must land on the position the drop commits.
func dragRound(v float64) float64 { return math.Round(v*100) / 100 }

// classicPlaced is where the scene draws the classic item it of the element on the canvas.
func (v *Viewer) classicPlaced(it *model.DiagramItem) *scene.Placed {
	if v.sv == nil || v.state.Current == nil {
		return nil
	}
	for _, pl := range v.sv.byPath {
		if pl.Src != nil && classicItemAt(v.state.Current, pl.Src) == it {
			return pl
		}
	}
	return nil
}

// classicItemAt is the model item of a drawn classic item: both keep file order, so the n-th drawn item
// of its parent is the n-th item of the element, a mounted element's file included.
func classicItemAt(e *model.Element, n *scene.Node) *model.DiagramItem { return classic.ItemAt(e, n) }

// classicNamed is where the scene draws the classic item with the model name name, the way a comment or a
// mark names an item.
func (v *Viewer) classicNamed(name string) *scene.Placed {
	if v.state.Current == nil {
		return nil
	}
	for _, it := range v.state.Current.Items {
		if it.Name == name && !it.IsLink() {
			return v.classicPlaced(it)
		}
	}
	return nil
}

// selectedScene is the first selected scene node and where it is drawn (nil when not drawn). A part of
// a component has no node of its own: nil and where it is drawn, see selectedInst.
func (v *Viewer) selectedScene() (*scene.Node, *scene.Placed) {
	if !v.sceneMode() || len(v.selNodes) == 0 {
		return nil, nil
	}
	if _, ok := v.selectedInst(); ok {
		return nil, v.sv.byPath[v.selNodes[0]]
	}
	n := NodeAt(v.sv.node, v.selNodes[0])
	if n == nil {
		return nil, nil
	}
	return n, v.sv.byNode[n]
}

// selectedInst is the first selected node when it is a part of a component.
func (v *Viewer) selectedInst() (instRef, bool) {
	if !v.sceneMode() || len(v.selNodes) == 0 {
		return instRef{}, false
	}
	pl := v.sv.byPath[v.selNodes[0]]
	ref, ok := v.sv.inst[pl]
	return ref, pl != nil && ok
}

// shownNode is the node the inspector describes: the selected one, for a part of a component the node it
// is drawn from.
func (v *Viewer) shownNode() *scene.Node {
	n, pl := v.selectedScene()
	if n == nil && pl != nil {
		return pl.Src
	}
	return n
}

// goToComponent shows the element that defines the component the selected use takes: the library is a
// mount of this document, so the window moves to it instead of opening the file on its own.
func (v *Viewer) goToComponent() { v.goToComponentNode(v.shownNode()) }

// goToComponentNode is goToComponent for a node taken from a row of the Items tree, which the canvas
// selection need not follow.
func (v *Viewer) goToComponentNode(n *scene.Node) {
	if n == nil || n.Kind != scene.KUse || v.stree == nil {
		return
	}
	comp, err := v.stree.Resolve(n.Parent, n.Path)
	if err != nil {
		v.setBanner(err.Error())
		return
	}
	if comp == nil || comp.Pos.File == "" {
		v.setBanner("the component " + n.Path + " has no file of its own")
		return
	}
	if !v.showMount(comp.Pos.File) {
		v.setBanner("the component is in " + comp.Pos.File + ", which this document does not mount")
		return
	}
	v.selectComponent(comp)
}

// selectComponent selects the node a component is defined in, once the window shows the element that
// mounts its file, and scrolls it into view.
func (v *Viewer) selectComponent(comp *scene.Node) {
	if v.sv == nil || comp == nil {
		return
	}
	for path, pl := range v.sv.byPath {
		if pl.Src == comp {
			v.selectPaths([]string{path}, true)
			return
		}
	}
}

// editingPath is the path of the item whose text is being edited, so its frame turns green as Swift's
// SelectorBox does.
func (v *Viewer) editingPath() string {
	if v.session == nil || v.sv == nil {
		return ""
	}
	it := v.session.Editing()
	if it == nil {
		return ""
	}
	for p, pl := range v.sv.byPath {
		if pl.Src != nil && classicItemAt(v.state.Current, pl.Src) == it {
			return p
		}
	}
	return ""
}

// dropStaleSelection forgets the selected paths that name nothing after a change.
func (v *Viewer) dropStaleSelection() {
	kept := v.selNodes[:0:0]
	for _, p := range v.selNodes {
		if NodeAt(v.sv.node, p) != nil || v.sv.byPath[p] != nil {
			kept = append(kept, p)
		}
	}
	v.selNodes = kept
}

// The selector frame of the Swift app (SelectorBox): the box of the item grown by 5 points, rounded by 9,
// stroked by a dash of 5 on and 5 off with a glow of its own colour. Blue, or green over the item that is
// being edited.
const (
	selectorGrow  = 5
	selectorRound = 9
	selectorBlur  = 5
)

var (
	selectorBlue  = gfx.RGB(0, 0, 255)
	selectorGreen = gfx.RGB(0, 255, 0)
)

// selectorPath is the rounded rectangle of Swift's SelectorBox, its arcs the addArc of the corners.
func selectorPath(r gfx.Rect, radius float64) *gfx.Path {
	p := &gfx.Path{}
	p.MoveTo((r.Min.X+r.Max.X)/2, r.Min.Y)
	p.ArcTo(r.Max.X, r.Min.Y, r.Max.X, r.Max.Y, radius)
	p.ArcTo(r.Max.X, r.Max.Y, r.Min.X, r.Max.Y, radius)
	p.ArcTo(r.Min.X, r.Max.Y, r.Min.X, r.Min.Y, radius)
	p.ArcTo(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y, radius)
	p.Close()
	return p
}

// selectorStroke is the dashed glow of the selector frame, the one of Swift's SelectorBox.draw.
func selectorStroke(r *gfx.Recorder, path *gfx.Path, color gfx.Color) {
	r.StrokeShadowPath(path, gfx.StrokeStyle{Width: 1, MiterLimit: 10, Dash: []float64{5, 5}, DashPhase: 5},
		color, gfx.Shadow{Blur: selectorBlur, Color: color, Screen: true})
}

// selectionPicture records the frames of the selected paths in the coordinates of the gfx scene, so a
// pan or a zoom moves them with the content instead of leaving them a frame behind. green is the path of
// the item being edited, if any. The classic preview - the rubber band, the line of a link being drawn -
// comes after them, in the same coordinates.
func (sv *sceneView) selectionPicture(sel []string, green string) *gfx.Picture {
	r := gfx.NewRecorder()
	for _, p := range sel {
		pl := sv.byPath[p]
		if pl == nil {
			continue
		}
		c := selectorBlue
		if p == green {
			c = selectorGreen
		}
		box := gfx.R(pl.X+sv.off.X-selectorGrow, pl.Y+sv.off.Y-selectorGrow,
			pl.W+2*selectorGrow, pl.H+2*selectorGrow)
		selectorStroke(r, selectorPath(box, selectorRound), c)
	}
	if sv.band != nil {
		selectorStroke(r, selectorPath(*sv.band, 0), selectorBlue)
	}
	if sv.line != nil {
		p := &gfx.Path{}
		p.MoveTo(sv.line[0].X, sv.line[0].Y)
		p.LineTo(sv.line[1].X, sv.line[1].Y)
		r.StrokeShadowPath(p, gfx.StrokeStyle{Width: 1, MiterLimit: 10}, selectorBlue,
			gfx.Shadow{Blur: selectorBlur, Color: selectorBlue, Screen: true})
	}
	return r.Finish()
}
