// Package render draws classic .tenn diagrams (elements with items and links) into a gfx scene.
// It is a port of web/packages/render, itself a 1:1 port of the Swift renderer (ElementScene.swift).
//
// Geometry stays in Swift's y-up space (an item at pos x y has its top-left corner at (x, y - height));
// the y-flip happens once, when a drawable is recorded into a gfx.Picture.
package render

import (
	"math"

	"github.com/haiodo/tenniarb/go/geometry"
)

// Point and Rect are the geometry of the editor and of this renderer: one type each, kept in go/geometry so
// the editor does not import a renderer for them.
type Point = geometry.Point

type Rect = geometry.Rect

type Size struct{ Width, Height float64 }

func minX(r Rect) float64 { return math.Min(r.X, r.X+r.Width) }
func maxX(r Rect) float64 { return math.Max(r.X, r.X+r.Width) }
func minY(r Rect) float64 { return math.Min(r.Y, r.Y+r.Height) }
func maxY(r Rect) float64 { return math.Max(r.Y, r.Y+r.Height) }
func midX(r Rect) float64 { return (minX(r) + maxX(r)) / 2 }
func midY(r Rect) float64 { return (minY(r) + maxY(r)) / 2 }

func union(a, b Rect) Rect {
	x0 := math.Min(minX(a), minX(b))
	y0 := math.Min(minY(a), minY(b))
	return Rect{X: x0, Y: y0, Width: math.Max(maxX(a), maxX(b)) - x0, Height: math.Max(maxY(a), maxY(b)) - y0}
}

func intersects(a, b Rect) bool {
	return minX(a) < maxX(b) && minX(b) < maxX(a) && minY(a) < maxY(b) && minY(b) < maxY(a)
}

func insetBy(r Rect, dx, dy float64) Rect {
	return Rect{X: r.X + dx, Y: r.Y + dy, Width: r.Width - 2*dx, Height: r.Height - 2*dy}
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
		return Point{X: p1.X + ta*(p2.X-p1.X), Y: p1.Y + ta*(p2.Y-p1.Y)}, true
	}
	return Point{}, false
}

// crossBox is where segment p1-p2 crosses the border of rect; the sides are tried top, left, bottom, right.
func crossBox(p1, p2 Point, rect Rect) (Point, bool) {
	ox, oy := rect.X, rect.Y
	if p, ok := crossLine(p1, p2, Point{X: ox, Y: oy}, Point{X: ox + rect.Width, Y: oy}); ok {
		return p, true
	}
	if p, ok := crossLine(p1, p2, Point{X: ox, Y: oy}, Point{X: ox, Y: oy + rect.Height}); ok {
		return p, true
	}
	if p, ok := crossLine(p1, p2, Point{X: ox, Y: oy + rect.Height}, Point{X: ox + rect.Width, Y: oy + rect.Height}); ok {
		return p, true
	}
	return crossLine(p1, p2, Point{X: ox + rect.Width, Y: oy}, Point{X: ox + rect.Width, Y: oy + rect.Height})
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
	r := Rect{X: math.Min(p1.X, p2.X), Y: math.Min(p1.Y, p2.Y), Width: math.Abs(p1.X - p2.X), Height: math.Abs(p1.Y - p2.Y)}
	if r.Height < 5 {
		r = Rect{X: r.X, Y: r.Y - 2.5, Width: r.Width, Height: r.Height + 5}
	}
	if r.Width < 5 {
		r = Rect{X: r.X - 2.5, Y: r.Y, Width: r.Width + 5, Height: r.Height}
	}
	d := a*a + b*b
	return containsPoint(r, Point{X: (b*(b*p.X-a*p.Y) - a*c) / d, Y: (a*(-b*p.X+a*p.Y) - b*c) / d})
}
