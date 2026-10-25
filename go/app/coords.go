package app

import (
	"math"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/render"
)

// SceneMap converts between the gfx scene of the canvas (y down, the space of View pointer events)
// and the scene of go/render and go/editor (y up). It is the translation render.Realize gives the
// root node: gfx = (x + TX, TY - y).
type SceneMap struct{ TX, TY float64 }

// MapOf is the map of a scene whose content has the bounds b, realized with the default padding.
func MapOf(b render.Rect) SceneMap {
	return SceneMap{TX: render.DefaultPadding - b.X, TY: render.DefaultPadding + math.Max(b.Y, b.Y+b.Height)}
}

// ToScene turns a gfx scene point into a point of the editor scene.
func (m SceneMap) ToScene(p gfx.Point) render.Point {
	return render.Point{X: p.X - m.TX, Y: m.TY - p.Y}
}

// ToGfx is the inverse of ToScene.
func (m SceneMap) ToGfx(p render.Point) gfx.Point {
	return gfx.Point{X: p.X + m.TX, Y: m.TY - p.Y}
}

// RectToGfx turns a rectangle of the editor scene (Y is its bottom edge) into a gfx rectangle.
func (m SceneMap) RectToGfx(r render.Rect) gfx.Rect {
	return gfx.R(r.X+m.TX, m.TY-(r.Y+r.Height), r.Width, r.Height)
}

// Shift is how far the content moved on the gfx scene from old to m.
func (m SceneMap) Shift(old SceneMap) gfx.Point {
	return gfx.Point{X: m.TX - old.TX, Y: m.TY - old.TY}
}

// ViewToScene is the gfx scene point under the view point p with the camera cam.
func ViewToScene(cam gfx.Camera, p gfx.Point) gfx.Point {
	return gfx.Point{X: cam.Origin.X + p.X/cam.Zoom, Y: cam.Origin.Y + p.Y/cam.Zoom}
}

// Follow moves the camera by shift, so the content that moved by shift stays where it was on screen.
func Follow(cam gfx.Camera, shift gfx.Point) gfx.Camera {
	cam.Origin = gfx.Point{X: cam.Origin.X + shift.X, Y: cam.Origin.Y + shift.Y}
	return cam
}
