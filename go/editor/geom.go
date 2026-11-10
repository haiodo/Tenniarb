package editor

import (
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
)

// Geom is the diagram as the session sees it: where an item is drawn, what lies under a point, and how the
// boxes move while they are dragged. The canvas answers it from go/scene, which draws the diagram; a
// session without one falls back to the boxes of go/render, which is what the session built for itself
// until the canvas could answer.
type Geom interface {
	// Items is every item of the diagram, drawn or not, in the order it is painted.
	Items() []*model.DiagramItem
	// Box is the drawn box of an item: the frame without its shadow, in scene coordinates. It is what a
	// click tests and what the frame of the selection follows.
	Box(it *model.DiagramItem) (render.Rect, bool)
	// PaintBox is the box the picture covers, the shadow of an item included.
	PaintBox(it *model.DiagramItem) (render.Rect, bool)
	// LabelBox is where the text of the item is edited: the label of a link, the text area of an item.
	LabelBox(it *model.DiagramItem) (render.Rect, bool)
	// FontSize is the size of the text of the item, in points.
	FontSize(it *model.DiagramItem) float64
	// Selectable reports whether the item takes part in a selection: a link is taken by its line, an item
	// by its box, and both are picked up by a band.
	Selectable(it *model.DiagramItem) bool
	// Find is the items under p, the boxes of an item before the lines of a link; over allows the items
	// that a plain click leaves alone.
	Find(p render.Point, over bool) []*model.DiagramItem
	// Move puts the drawn boxes at the positions of a drag and answers the items that moved.
	Move(positions map[*model.DiagramItem]render.Point) []*model.DiagramItem
	// HitLine reports whether p is on the line of a link: a link has no box to click.
	HitLine(it *model.DiagramItem, p render.Point) bool
	// Grid is the step the canvas moves an item by.
	Grid() render.Point
	// Bounds is the extent of the diagram, for the layout that arranges the items in it.
	Bounds() render.Rect
}

// sceneGeom is Geom over the drawables of go/render: what the session built for itself before the canvas
// answered.
type sceneGeom struct{ scene *render.DrawableScene }

func (g sceneGeom) Items() []*model.DiagramItem { return g.scene.Items() }

func (g sceneGeom) Box(it *model.DiagramItem) (render.Rect, bool) {
	d, ok := g.scene.Drawables[it]
	if !ok {
		return render.Rect{}, false
	}
	return d.Bounds(), true
}

func (g sceneGeom) PaintBox(it *model.DiagramItem) (render.Rect, bool) {
	d, ok := g.scene.Drawables[it]
	if !ok {
		return render.Rect{}, false
	}
	return d.Bounds(), true
}

func (g sceneGeom) LabelBox(it *model.DiagramItem) (render.Rect, bool) {
	d, ok := g.scene.Drawables[it]
	if !ok {
		return render.Rect{}, false
	}
	if l, isLine := d.(*render.DrawableLine); isLine {
		return l.LabelBounds(), true
	}
	return d.SelectorBounds(), true
}

func (g sceneGeom) FontSize(it *model.DiagramItem) float64 {
	d, ok := g.scene.Drawables[it]
	if !ok {
		return 0
	}
	return fontSizeOf(d)
}

func (g sceneGeom) Selectable(it *model.DiagramItem) bool {
	d, ok := g.scene.Drawables[it]
	if !ok {
		return false
	}
	return d.Selectable()
}

func (g sceneGeom) Find(p render.Point, over bool) []*model.DiagramItem {
	return g.scene.Find(p, over)
}

func (g sceneGeom) Move(positions map[*model.DiagramItem]render.Point) []*model.DiagramItem {
	moved := g.scene.UpdateLayout(positions)
	out := make([]*model.DiagramItem, 0, len(moved))
	for _, d := range moved {
		if it := render.DrawableOf(d); it != nil {
			out = append(out, it)
		}
	}
	return out
}

func (g sceneGeom) HitLine(it *model.DiagramItem, p render.Point) bool {
	d, ok := g.scene.Drawables[it]
	if !ok {
		return false
	}
	l, isLine := d.(*render.DrawableLine)
	return isLine && l.Find(p)
}

func (g sceneGeom) Grid() render.Point { return g.scene.Style.GridSpan }

func (g sceneGeom) Bounds() render.Rect { return g.scene.Bounds() }

// geom is the geometry the session works with: the one its caller gave it, else the boxes of the scene it
// built for itself.
func (s *Session) geom() Geom {
	if s.opts.Geom != nil {
		return s.opts.Geom
	}
	return sceneGeom{scene: s.scene}
}
