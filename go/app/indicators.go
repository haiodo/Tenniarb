package app

import (
	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/render"
)

// EdgeDots places the indicator dots (Swift drawRulers) for the rectangles of the gfx scene (y down)
// that lie outside the part of it a view of vw x vh points shows with cam. The dots are in the y-up
// space of editor.IndicatorDots: scene units, the origin at the lower-left corner of the view.
func EdgeDots(bounds []gfx.Rect, cam gfx.Camera, vw, vh float64) []editor.Dot {
	if cam.Zoom <= 0 || vw <= 0 || vh <= 0 || len(bounds) == 0 {
		return nil
	}
	flipped := make([]render.Rect, len(bounds))
	for i, b := range bounds {
		flipped[i] = render.Rect{X: b.Min.X, Y: -b.Max.Y, Width: b.W(), Height: b.H()}
	}
	visible := render.Rect{X: cam.Origin.X, Y: -(cam.Origin.Y + vh/cam.Zoom), Width: vw / cam.Zoom, Height: vh / cam.Zoom}
	return editor.IndicatorDots(flipped, visible)
}
