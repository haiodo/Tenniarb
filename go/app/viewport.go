package app

import (
	"math"

	"github.com/haiodo/gowt/gfx"
)

// Zoom limits and steps. The gfx view enforces its own limits on gestures; these match its defaults.
const (
	MinZoom  = 0.02
	MaxZoom  = 64.0
	ZoomStep = 1 / 0.75 // Swift zoomIn divides the zoom by 0.75 and zoomOut multiplies it
)

// ClampZoom keeps z within [MinZoom, MaxZoom].
func ClampZoom(z float64) float64 { return math.Min(MaxZoom, math.Max(MinZoom, z)) }

// CenterCamera shows a content of cw x ch scene units in the middle of a view of vw x vh points at zoom.
func CenterCamera(zoom, cw, ch, vw, vh float64) gfx.Camera {
	zoom = ClampZoom(zoom)
	return gfx.Camera{
		Origin: gfx.Point{X: cw/2 - vw/(2*zoom), Y: ch/2 - vh/(2*zoom)},
		Zoom:   zoom,
	}
}

// FitCamera is CenterCamera at the zoom that shows all of the content, but never above 1.
func FitCamera(cw, ch, vw, vh float64) gfx.Camera {
	zoom := 1.0
	if cw > 0 && ch > 0 && vw > 0 && vh > 0 {
		zoom = math.Min(1, math.Min(vw/cw, vh/ch))
	}
	return CenterCamera(zoom, cw, ch, vw, vh)
}

// ZoomCamera multiplies the zoom of cam by f, keeping the scene point under the view point (px, py) in place.
func ZoomCamera(cam gfx.Camera, f, px, py float64) gfx.Camera {
	z := ClampZoom(cam.Zoom * f)
	sx, sy := cam.Origin.X+px/cam.Zoom, cam.Origin.Y+py/cam.Zoom
	return gfx.Camera{Origin: gfx.Point{X: sx - px/z, Y: sy - py/z}, Zoom: z}
}

// CenterOn puts the scene point c in the middle of a view of vw x vh points, keeping the zoom.
func CenterOn(cam gfx.Camera, c gfx.Point, vw, vh float64) gfx.Camera {
	cam.Origin = gfx.Point{X: c.X - vw/(2*cam.Zoom), Y: c.Y - vh/(2*cam.Zoom)}
	return cam
}
