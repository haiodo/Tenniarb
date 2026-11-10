//go:build darwin || linux || windows

package app

import (
	"math"
	"strconv"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/geometry"
	"github.com/haiodo/tenniarb/go/model"
)

// canvasGeom is editor.Geom over the scene the canvas drew: the session asks where an item is and the
// canvas answers from its own picture, so the edit of a classic diagram no longer needs a scene built by
// go/render.
type canvasGeom struct{ v *Viewer }

// Items is every item of the shown element in the order the scene paints it.
func (g canvasGeom) Items() []*model.DiagramItem {
	v := g.v
	if v.sv == nil || v.state.Current == nil {
		return nil
	}
	seen := map[*model.DiagramItem]bool{}
	var out []*model.DiagramItem
	for _, pl := range v.sv.byPath {
		it := classicItemAt(v.state.Current, pl.Src)
		if it == nil || seen[it] {
			continue
		}
		seen[it] = true
		out = append(out, it)
	}
	for _, it := range v.state.Current.Items {
		if !seen[it] {
			out = append(out, it) // a link, or an item the scene drew no box for
		}
	}
	return out
}

// Box is the drawn frame of the item in the coordinates of the editor scene, the shadow left out.
func (g canvasGeom) Box(it *model.DiagramItem) (geometry.Rect, bool) {
	b, ok := g.v.drawnRect(it)
	if !ok {
		return geometry.Rect{}, false
	}
	return g.v.smap.RectToScene(b), true
}

// PaintBox is the same box: the scene draws the frame, and the shadow of an item is not a separate box.
func (g canvasGeom) PaintBox(it *model.DiagramItem) (geometry.Rect, bool) { return g.Box(it) }

// LabelBox is where the text of the item lies: the label of a link sits in the same box.
func (g canvasGeom) LabelBox(it *model.DiagramItem) (geometry.Rect, bool) { return g.Box(it) }

// FontSize is the size the item draws its text with.
func (g canvasGeom) FontSize(it *model.DiagramItem) float64 {
	if it == nil {
		return 15
	}
	if v, ok := propNum(it, "font-size"); ok && v > 0 {
		return v
	}
	return 15
}

// Selectable: a band takes every item it meets, links included.
func (g canvasGeom) Selectable(it *model.DiagramItem) bool { return it != nil }

// Find is the items under p, the boxes of an item before the lines of a link.
func (g canvasGeom) Find(p geometry.Point, over bool) []*model.DiagramItem {
	var boxes, lines []*model.DiagramItem
	for _, it := range g.Items() {
		if it.IsLink() {
			if g.HitLine(it, p) {
				lines = append(lines, it)
			}
			continue
		}
		if b, ok := g.Box(it); ok && b.Contains(p) {
			boxes = append(boxes, it)
		}
	}
	if len(boxes) > 0 || over {
		return boxes
	}
	return lines
}

// HitLine reports whether p is on the line of a link: the segment between the middles of its two ends.
func (g canvasGeom) HitLine(it *model.DiagramItem, p geometry.Point) bool {
	if it == nil || !it.IsLink() || it.Source == nil || it.Target == nil {
		return false
	}
	a, ok1 := g.centre(it.Source)
	b, ok2 := g.centre(it.Target)
	if !ok1 || !ok2 {
		return false
	}
	return distanceToSegment(p, a, b) <= 7
}

// centre is the middle of the drawn box of an item, in the coordinates of the editor scene.
func (g canvasGeom) centre(it *model.DiagramItem) (geometry.Point, bool) {
	b, ok := g.Box(it)
	if !ok {
		return geometry.Point{}, false
	}
	return geometry.Point{X: b.X + b.Width/2, Y: b.Y + b.Height/2}, true
}

// distanceToSegment is how far p lies from the segment a-b.
func distanceToSegment(p, a, b geometry.Point) float64 {
	dx, dy := b.X-a.X, b.Y-a.Y
	if dx == 0 && dy == 0 {
		return math.Hypot(p.X-a.X, p.Y-a.Y)
	}
	t := ((p.X-a.X)*dx + (p.Y-a.Y)*dy) / (dx*dx + dy*dy)
	t = math.Max(0, math.Min(1, t))
	return math.Hypot(p.X-(a.X+t*dx), p.Y-(a.Y+t*dy))
}

// Move puts the drawn boxes of the items where a drag takes them, in the coordinates of the editor scene,
// and answers the items that moved.
func (g canvasGeom) Move(positions map[*model.DiagramItem]geometry.Point) []*model.DiagramItem {
	v := g.v
	if v.sv == nil {
		return nil
	}
	var moved []*model.DiagramItem
	v.scene.Update(func(tx *gfx.Tx) {
		for it, pos := range positions {
			pl := v.classicPlaced(it)
			if pl == nil {
				continue
			}
			b, ok := g.Box(it)
			if !ok {
				continue
			}
			// Scene coordinates are y up and the drawn box is the top left in y down: the frame of the
			// layout takes the corner the box keeps, without the offset of the canvas.
			gfxBox := v.smap.RectToGfx(geometry.Rect{X: pos.X, Y: pos.Y, Width: b.Width, Height: b.Height})
			if v.sv.screen.MoveItem(tx, pl, gfxBox.Min.X-v.sv.off.X, gfxBox.Min.Y-v.sv.off.Y) {
				moved = append(moved, it)
			}
		}
	})
	v.drawSceneOverlay()
	return moved
}

// Grid is the step the canvas snaps to: the grid of the element, or five points as the classic one had.
func (g canvasGeom) Grid() geometry.Point {
	if g.v != nil && g.v.state.Current != nil {
		if n, ok := propNum(g.v.state.Current, "grid"); ok && n > 0 {
			return geometry.Point{X: n, Y: n}
		}
	}
	return geometry.Point{X: 5, Y: 5}
}

// propNum is the first argument of a numeric property of an item or of an element.
func propNum(t any, name string) (float64, bool) {
	var props *model.ModelProperties
	switch v := t.(type) {
	case *model.DiagramItem:
		if v == nil {
			return 0, false
		}
		props = v.Properties
	case *model.Element:
		if v == nil {
			return 0, false
		}
		props = v.Properties
	}
	if props == nil {
		return 0, false
	}
	p := props.Get(name)
	if p == nil || len(p.Children) < 2 {
		return 0, false
	}
	text, ok := p.Children[1].GetIdentText()
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

// Bounds is the extent of the diagram the canvas draws, in the coordinates of the editor scene.
func (g canvasGeom) Bounds() geometry.Rect {
	if g.v == nil || g.v.sv == nil {
		return geometry.Rect{}
	}
	g.v.sv.bounds()
	return geometry.Rect{Width: g.v.sv.w, Height: g.v.sv.h}
}
