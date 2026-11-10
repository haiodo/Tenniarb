// Package scenegeom is geom.Geom over a scene tree: the diagram the engine draws, asked where its items
// are. It is what a session uses when no window draws the diagram - the editor's own tests, a batch that
// arranges items without a window.
package scenegeom

import (
	"math"
	"strconv"
	"strings"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/classic"
	"github.com/haiodo/tenniarb/go/geometry"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/scene"
)

// Geom is the diagram of an element as the scene builds it.
type Geom struct {
	tree    *scene.Tree
	el      func() *model.Element
	measure gfx.TextMeasurer
	// path is where the element sits in the tree; "" is worked out from the element's name chain.
	path string
	// built is the element the sizes were made for.
	built *model.Element
	// preview is where a drag has taken an item to, before the model is written: the picture follows the
	// pointer, and the model changes once, when the drag is committed.
	preview map[*model.DiagramItem]geometry.Point
	size    func(*model.DiagramItem, model.IExecutionContext) *model.DrawableSize
}

// New is the geometry of el in tree. measure sizes the text, and it is also what the expressions get when
// they ask for the size of a text.
func New(tree *scene.Tree, el *model.Element, measure gfx.TextMeasurer) *Geom {
	return For(tree, func() *model.Element { return el }, measure)
}

// For is New where the element may change: the geometry follows the element a session works on.
func For(tree *scene.Tree, el func() *model.Element, measure gfx.TextMeasurer) *Geom {
	if measure == nil {
		measure = gfx.DefaultMeasurer
	}
	g := &Geom{tree: tree, el: el, measure: measure}
	cur := el()
	if cur != nil {
		g.path = strings.Join(model.PathOf(cur), ".")
		g.size = classic.Sizes(tree, g.path, cur, measure)
	}
	return g
}

// Items is every item of the element.
func (g *Geom) Items() []*model.DiagramItem {
	el := g.element()
	if el == nil {
		return nil
	}
	return el.Items
}

// Box is the drawn box of an item: its place from the model and the size the scene gives it.
func (g *Geom) Box(it *model.DiagramItem) (geometry.Rect, bool) {
	if it.Kind == model.ItemKindLink {
		return g.linkBox(it)
	}
	if it == nil || !g.present(it) {
		return geometry.Rect{}, false
	}
	sz := g.ItemSize(it, nil)
	if sz == nil {
		// The scene drew no box for it: the place and size the model names, and the size the session gives
		// an item it knows nothing more about.
		w, h := 100.0, 50.0
		if v, ok := numberProp(it, "width"); ok && v > 0 {
			w = v
		}
		if v, ok := numberProp(it, "height"); ok && v > 0 {
			h = v
		}
		sz = &model.DrawableSize{Width: w, Height: h}
	}
	// The model keeps pos as the bottom left with y up in the classic layout; the box a click tests is the
	// frame the renderer had: the same left edge and the top, so the height comes off the y.
	r := geometry.Rect{X: it.X, Y: it.Y - sz.Height, Width: sz.Width, Height: sz.Height}
	if p, ok := g.preview[it]; ok {
		r.X, r.Y = p.X, p.Y
	}
	return r, true
}

// element is the element the geometry is of, nil when there is none.
func (g *Geom) element() *model.Element {
	if g.el == nil {
		return nil
	}
	el := g.el()
	if el != g.built {
		g.rebuild(el)
	}
	return el
}

// rebuild follows the element to a new one: the sizes are of that element.
func (g *Geom) rebuild(el *model.Element) {
	g.built = el
	g.size = nil
	g.preview = nil
	if el == nil {
		g.path = ""
		return
	}
	g.path = strings.Join(model.PathOf(el), ".")
	if g.tree != nil {
		g.size = classic.Sizes(g.tree, g.path, el, g.measure)
	}
}

// present reports whether the item is still part of the diagram: a deleted item is not, and the selection
// must let go of it.
func (g *Geom) present(it *model.DiagramItem) bool {
	for _, x := range g.Items() {
		if x == it {
			return true
		}
	}
	return false
}

// numberProp is a numeric property of an item.
func numberProp(it *model.DiagramItem, name string) (float64, bool) {
	if it == nil {
		return 0, false
	}
	p := it.Properties.Get(name)
	if p == nil || len(p.Children) < 2 {
		return 0, false
	}
	text, ok := p.Children[1].GetIdentText()
	if !ok {
		return 0, false
	}
	v, err := strconv.ParseFloat(text, 64)
	return v, err == nil
}

// PaintBox is the same box: a classic item draws its frame where it is placed.
func (g *Geom) PaintBox(it *model.DiagramItem) (geometry.Rect, bool) { return g.Box(it) }

// LabelBox is where the text of the item lies.
func (g *Geom) LabelBox(it *model.DiagramItem) (geometry.Rect, bool) { return g.Box(it) }

// FontSize is the size of the text of the item.
func (g *Geom) FontSize(it *model.DiagramItem) float64 { return fontSize(it) }

// Selectable: a band takes every item it meets.
func (g *Geom) Selectable(it *model.DiagramItem) bool { return it != nil }

// Find is the items under p, boxes before the lines of a link.
func (g *Geom) Find(p geometry.Point, over bool) []*model.DiagramItem {
	var boxes, lines []*model.DiagramItem
	for _, it := range g.Items() {
		if it.Kind == model.ItemKindLink {
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

// HitLine reports whether p is on the line of a link: the segment between the middles of its ends.
func (g *Geom) HitLine(it *model.DiagramItem, p geometry.Point) bool {
	if it == nil || it.Source == nil || it.Target == nil {
		return false
	}
	a, ok1 := g.centre(it.Source)
	b, ok2 := g.centre(it.Target)
	if !ok1 || !ok2 {
		return false
	}
	return distance(p, a, b) <= 7
}

// centre is the middle of the drawn box of an item.
func (g *Geom) centre(it *model.DiagramItem) (geometry.Point, bool) {
	b, ok := g.Box(it)
	if !ok {
		return geometry.Point{}, false
	}
	return geometry.Point{X: b.X + b.Width/2, Y: b.Y + b.Height/2}, true
}

// distance is how far p lies from the segment a-b.
func distance(p, a, b geometry.Point) float64 {
	dx, dy := b.X-a.X, b.Y-a.Y
	if dx == 0 && dy == 0 {
		return math.Hypot(p.X-a.X, p.Y-a.Y)
	}
	t := ((p.X-a.X)*dx + (p.Y-a.Y)*dy) / (dx*dx + dy*dy)
	t = math.Max(0, math.Min(1, t))
	return math.Hypot(p.X-(a.X+t*dx), p.Y-(a.Y+t*dy))
}

// Move takes the boxes where a drag asks, without touching the model: the commit writes the model once, so
// one drag stays one undo step.
func (g *Geom) Move(positions map[*model.DiagramItem]geometry.Point) []*model.DiagramItem {
	if g.preview == nil {
		g.preview = map[*model.DiagramItem]geometry.Point{}
	}
	moved := make([]*model.DiagramItem, 0, len(positions))
	for it, p := range positions {
		g.preview[it] = p
		moved = append(moved, it)
	}
	return moved
}

// Grid is the step an item moves by: five points, as the classic canvas had.
func (g *Geom) Grid() geometry.Point { return geometry.Point{X: 5, Y: 5} }

// Bounds is the extent of the placed items.
func (g *Geom) Bounds() geometry.Rect {
	var r geometry.Rect
	first := true
	for _, it := range g.Items() {
		b, ok := g.Box(it)
		if !ok {
			continue
		}
		if first {
			r, first = b, false
			continue
		}
		r = union(r, b)
	}
	return r
}

// union is the smallest rectangle holding both.
func union(a, b geometry.Rect) geometry.Rect {
	x, y := math.Min(a.X, b.X), math.Min(a.Y, b.Y)
	maxX, maxY := math.Max(a.X+a.Width, b.X+b.Width), math.Max(a.Y+a.Height, b.Y+b.Height)
	return geometry.Rect{X: x, Y: y, Width: maxX - x, Height: maxY - y}
}

// ItemSize is the size the expression engine reads as width and height of the item.
func (g *Geom) ItemSize(it *model.DiagramItem, prev model.IExecutionContext) *model.DrawableSize {
	if g.size == nil {
		return nil
	}
	return g.size(it, prev)
}

// MeasureText is the size of a plain text at a font size.
func (g *Geom) MeasureText(text string, fontSize float64) model.DrawableSize {
	w, h := scene.MeasureText(g.measure, text, fontSize)
	return model.DrawableSize{Width: w, Height: h}
}

// fontSize is the size the item draws its text with.
func fontSize(it *model.DiagramItem) float64 {
	if it == nil {
		return 15
	}
	if p := it.Properties.Get("font-size"); p != nil && len(p.Children) >= 2 {
		if text, ok := p.Children[1].GetIdentText(); ok {
			if v, err := strconv.ParseFloat(text, 64); err == nil && v > 0 {
				return v
			}
		}
	}
	return 15
}

// linkBox is the box of a link: the room its line takes between the middles of its ends.
func (g *Geom) linkBox(it *model.DiagramItem) (geometry.Rect, bool) {
	if it.Source == nil || it.Target == nil {
		// A link whose ends were not resolved still has a place, a point of it at the pos of the model.
		return geometry.Rect{X: it.X, Y: it.Y}, true
	}
	a, ok1 := g.centre(it.Source)
	b, ok2 := g.centre(it.Target)
	if !ok1 || !ok2 {
		return geometry.Rect{}, false
	}
	x, y := math.Min(a.X, b.X), math.Min(a.Y, b.Y)
	return geometry.Rect{X: x, Y: y, Width: math.Abs(a.X - b.X), Height: math.Abs(a.Y - b.Y)}, true
}
