//go:build darwin || linux || windows

package app

import (
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/classic"
	"github.com/haiodo/tenniarb/go/geometry"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/scene"
)

// canvasGeom is geom.Geom over the scene the canvas drew: the session asks where an item is and the canvas
// answers from its own picture.
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

// Box is the drawn frame of the item in the coordinates of the editor scene, the shadow left out. An item
// the canvas drew no frame for - the session holds a classic diagram the canvas shows as a scene - is
// placed by the size the scene gives it, so the session still works on it.
func (g canvasGeom) Box(it *model.DiagramItem) (geometry.Rect, bool) {
	// An item the element no longer holds has no box, however recently it was drawn: the session leans on
	// this to let go of a deleted item, and a stale one under the pointer would grab the next press.
	if it == nil || !g.present(it) {
		return geometry.Rect{}, false
	}
	if b, ok := g.v.drawnRect(it); ok {
		r := g.v.smap.RectToScene(b)
		if p, moved := g.v.previewBoxes[it]; moved {
			r.X, r.Y = p.X, p.Y
		}
		return r, true
	}
	if it.Kind == model.ItemKindLink {
		if a, ok := g.centre(it.Source); ok {
			if b, ok2 := g.centre(it.Target); ok2 {
				x, y := math.Min(a.X, b.X), math.Min(a.Y, b.Y)
				return geometry.Rect{X: x, Y: y, Width: math.Abs(a.X - b.X), Height: math.Abs(a.Y - b.Y)}, true
			}
		}
		return geometry.Rect{}, false
	}
	// Nothing drawn for it: the place and the size the model names, and the size the session gives an item
	// it knows no more about.
	w, h := 100.0, 50.0
	if v, ok := propNum(it, "width"); ok && v > 0 {
		w = v
	}
	if v, ok := propNum(it, "height"); ok && v > 0 {
		h = v
	}
	return geometry.Rect{X: it.X, Y: it.Y - h, Width: w, Height: h}, true
}

// present reports whether the item is still part of the element on the canvas: a deleted item is not,
// and the selection must let go of it.
func (g canvasGeom) present(it *model.DiagramItem) bool {
	v := g.v
	if v == nil || v.state.Current == nil {
		return false
	}
	return slices.Contains(v.state.Current.Items, it)
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
	if it == nil || it.Source == nil || it.Target == nil {
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

// Move takes the boxes where a drag asks, without moving the scene: the session commits the model once,
// and the canvas draws the preview of the drag from the same positions.
func (g canvasGeom) Move(positions map[*model.DiagramItem]geometry.Point) []*model.DiagramItem {
	if g.v.previewBoxes == nil {
		g.v.previewBoxes = map[*model.DiagramItem]geometry.Point{}
	}
	moved := make([]*model.DiagramItem, 0, len(positions))
	for it, p := range positions {
		g.v.previewBoxes[it] = p
		moved = append(moved, it)
	}
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

// ItemSize is the size the expression engine reads as width and height of the item: the scene is asked for
// it once per evaluation round, as the classic renderer's throwaway scene was.
func (g canvasGeom) ItemSize(it *model.DiagramItem, prev model.IExecutionContext) *model.DrawableSize {
	v := g.v
	if v == nil || v.stree == nil || v.state.Current == nil {
		return nil
	}
	if v.geomSize == nil || v.geomSizeEl != v.state.Current {
		v.geomSize = classic.Sizes(v.stree, strings.Join(PathOf(v.state.Current), "."), v.state.Current, gfx.DefaultMeasurer)
		v.geomSizeEl = v.state.Current
	}
	return v.geomSize(it, prev)
}

// MeasureText is the size of a plain text at a font size, measured by the engine that draws it.
func (g canvasGeom) MeasureText(text string, fontSize float64) model.DrawableSize {
	w, h := scene.MeasureText(gfx.DefaultMeasurer, text, fontSize)
	return model.DrawableSize{Width: w, Height: h}
}
