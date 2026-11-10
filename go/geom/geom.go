// Package geom is the diagram as the editor sees it: where an item is drawn, what lies under a point, and
// how the boxes move while they are dragged. The interface lives here, apart from both sides, so the
// session does not import a renderer and the engine that draws does not import the session.
package geom

import (
	"github.com/haiodo/tenniarb/go/geometry"
	"github.com/haiodo/tenniarb/go/model"
)

// Geom is answered by whatever draws the diagram.
type Geom interface {
	// Items is every item of the diagram, in the order it is painted.
	Items() []*model.DiagramItem
	// Box is the drawn box of an item: the frame without its shadow, in scene coordinates. A click tests
	// it, and the frame of the selection follows it.
	Box(it *model.DiagramItem) (geometry.Rect, bool)
	// PaintBox is the box the picture covers, the shadow included.
	PaintBox(it *model.DiagramItem) (geometry.Rect, bool)
	// LabelBox is where the text of the item is edited: the label of a link, the text area of an item.
	LabelBox(it *model.DiagramItem) (geometry.Rect, bool)
	// FontSize is the size of the text of the item, in points.
	FontSize(it *model.DiagramItem) float64
	// Selectable reports whether the item takes part in a selection: a link is taken by its line, an item
	// by its box, and both are picked up by a band.
	Selectable(it *model.DiagramItem) bool
	// Find is the items under p, the boxes of an item before the lines of a link; over allows the items a
	// plain click leaves alone.
	Find(p geometry.Point, over bool) []*model.DiagramItem
	// HitLine reports whether p is on the line of a link: a link has no box to click.
	HitLine(it *model.DiagramItem, p geometry.Point) bool
	// Move puts the drawn boxes at the positions of a drag and answers the items that moved.
	Move(positions map[*model.DiagramItem]geometry.Point) []*model.DiagramItem
	// Grid is the step the canvas moves an item by.
	Grid() geometry.Point
	// Bounds is the extent of the diagram, for the layout that arranges the items in it.
	Bounds() geometry.Rect
	// ItemSize is the size the expression engine reads as width and height of the item: it stands in for
	// the drawable's bounds, and prev is a read-only view of the values of the round before.
	ItemSize(it *model.DiagramItem, prev model.IExecutionContext) *model.DrawableSize
	// MeasureText is the size of a plain text at a font size, for the expressions that ask for one.
	MeasureText(text string, fontSize float64) model.DrawableSize
}
