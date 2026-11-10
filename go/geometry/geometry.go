// Package geometry is the flat geometry the editor and the old renderer share: a point and a rectangle in
// the coordinates of a diagram, y up. It is its own package so the editor stops importing a renderer for
// two types, so a package that places a point does not import a renderer for it.
package geometry

// Padding is the room the canvas leaves around the diagram: the classic one had fifteen points.
const Padding = 15.0

// Point is a point of the diagram.
type Point struct{ X, Y float64 }

// Rect is a rectangle: X, Y is its bottom left corner, and Y grows up.
type Rect struct{ X, Y, Width, Height float64 }

// Contains reports whether p is inside: the min edges are included, the max edges are not.
func (r Rect) Contains(p Point) bool {
	return p.X >= r.X && p.X < r.X+r.Width && p.Y >= r.Y && p.Y < r.Y+r.Height
}

// Intersects reports whether the two rectangles share any point.
func (r Rect) Intersects(o Rect) bool {
	return r.X < o.X+o.Width && o.X < r.X+r.Width && r.Y < o.Y+o.Height && o.Y < r.Y+r.Height
}
