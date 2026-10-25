// Package render draws classic .tenn diagrams (elements with items and links) into a gfx scene.
// It is a port of web/packages/render, itself a 1:1 port of the Swift renderer (ElementScene.swift).
//
// Geometry stays in Swift's y-up space (an item at pos x y has its top-left corner at (x, y - height));
// the y-flip happens once, when a drawable is recorded into a gfx.Picture.
package render

import "math"

type Point struct{ X, Y float64 }

type Size struct{ Width, Height float64 }

type Rect struct{ X, Y, Width, Height float64 }

func minX(r Rect) float64 { return math.Min(r.X, r.X+r.Width) }
func maxX(r Rect) float64 { return math.Max(r.X, r.X+r.Width) }
func minY(r Rect) float64 { return math.Min(r.Y, r.Y+r.Height) }
func maxY(r Rect) float64 { return math.Max(r.Y, r.Y+r.Height) }
func midX(r Rect) float64 { return (minX(r) + maxX(r)) / 2 }
func midY(r Rect) float64 { return (minY(r) + maxY(r)) / 2 }

func union(a, b Rect) Rect {
	x0 := math.Min(minX(a), minX(b))
	y0 := math.Min(minY(a), minY(b))
	return Rect{x0, y0, math.Max(maxX(a), maxX(b)) - x0, math.Max(maxY(a), maxY(b)) - y0}
}

func intersects(a, b Rect) bool {
	return minX(a) < maxX(b) && minX(b) < maxX(a) && minY(a) < maxY(b) && minY(b) < maxY(a)
}

func insetBy(r Rect, dx, dy float64) Rect {
	return Rect{r.X + dx, r.Y + dy, r.Width - 2*dx, r.Height - 2*dy}
}

func containsRect(outer, inner Rect) bool {
	return minX(outer) <= minX(inner) && maxX(outer) >= maxX(inner) && minY(outer) <= minY(inner) && maxY(outer) >= maxY(inner)
}

// crossLine is the crossing point of segments p1-p2 and p3-p4.
func crossLine(p1, p2, p3, p4 Point) (Point, bool) {
	d := (p1.X-p2.X)*(p4.Y-p3.Y) - (p1.Y-p2.Y)*(p4.X-p3.X)
	da := (p1.X-p3.X)*(p4.Y-p3.Y) - (p1.Y-p3.Y)*(p4.X-p3.X)
	db := (p1.X-p2.X)*(p1.Y-p3.Y) - (p1.Y-p2.Y)*(p1.X-p3.X)

	ta := da / d
	tb := db / d
	// NaN (parallel or zero-length segments) fails every comparison, as in the Swift and TS code.
	if ta >= 0 && ta <= 1 && tb >= 0 && tb <= 1 {
		return Point{p1.X + ta*(p2.X-p1.X), p1.Y + ta*(p2.Y-p1.Y)}, true
	}
	return Point{}, false
}

// crossBox is where segment p1-p2 crosses the border of rect; the sides are tried top, left, bottom, right.
func crossBox(p1, p2 Point, rect Rect) (Point, bool) {
	ox, oy := rect.X, rect.Y
	if p, ok := crossLine(p1, p2, Point{ox, oy}, Point{ox + rect.Width, oy}); ok {
		return p, true
	}
	if p, ok := crossLine(p1, p2, Point{ox, oy}, Point{ox, oy + rect.Height}); ok {
		return p, true
	}
	if p, ok := crossLine(p1, p2, Point{ox, oy + rect.Height}, Point{ox + rect.Width, oy + rect.Height}); ok {
		return p, true
	}
	return crossLine(p1, p2, Point{ox + rect.Width, oy}, Point{ox + rect.Width, oy + rect.Height})
}

// containsPoint is CGRect.contains(point): min edges inclusive, max edges exclusive.
func containsPoint(r Rect, p Point) bool {
	return p.X >= minX(r) && p.X < maxX(r) && p.Y >= minY(r) && p.Y < maxY(r)
}

// crossPointLine is SceneMath.crossPointLine: whether p is on segment p1-p2, with a 7 unit tolerance.
func crossPointLine(p1, p2, p Point) bool {
	if math.Hypot(p1.X-p.X, p1.Y-p.Y) < 7 || math.Hypot(p2.X-p.X, p2.Y-p.Y) < 7 {
		return true
	}
	if p1.X == p2.X && p1.Y == p2.Y {
		return false
	}
	a := p2.Y - p1.Y
	b := p1.X - p2.X
	c := -(p1.X*a + p1.Y*b)
	if math.Abs(a*p.X+b*p.Y+c)/math.Hypot(a, b) >= 7 {
		return false
	}
	// The foot of the perpendicular must fall inside the segment's box, padded to 5 for flat lines.
	r := Rect{math.Min(p1.X, p2.X), math.Min(p1.Y, p2.Y), math.Abs(p1.X - p2.X), math.Abs(p1.Y - p2.Y)}
	if r.Height < 5 {
		r = Rect{r.X, r.Y - 2.5, r.Width, r.Height + 5}
	}
	if r.Width < 5 {
		r = Rect{r.X - 2.5, r.Y, r.Width + 5, r.Height}
	}
	d := a*a + b*b
	return containsPoint(r, Point{(b*(b*p.X-a*p.Y) - a*c) / d, (a*(-b*p.X+a*p.Y) - b*c) / d})
}

// Contains is CGRect.contains(point): min edges inclusive, max edges exclusive.
func (r Rect) Contains(p Point) bool { return containsPoint(r, p) }

// Intersects is CGRect.intersects.
func (r Rect) Intersects(o Rect) bool { return intersects(r, o) }
