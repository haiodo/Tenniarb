package app

import (
	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/render"
)

// EdgeDots places the indicator dots (Swift drawRulers) for the rectangles of the gfx scene (y down)
// that lie outside the part of it a view of vw x vh points shows with cam. The dots are in the pixels of
// the view, y up, the origin at its lower-left corner: a dot keeps its size on screen at any zoom, as
// Swift draws it.
func EdgeDots(bounds []gfx.Rect, cam gfx.Camera, vw, vh float64) []editor.Dot {
	if cam.Zoom <= 0 || vw <= 0 || vh <= 0 || len(bounds) == 0 {
		return nil
	}
	k := cam.Zoom
	px := make([]render.Rect, len(bounds))
	for i, b := range bounds {
		// The gfx scene grows downwards, the dots' space upwards: the bottom of a box is its Y.
		px[i] = render.Rect{
			X:      (b.Min.X - cam.Origin.X) * k,
			Y:      vh - (b.Max.Y-cam.Origin.Y)*k,
			Width:  b.W() * k,
			Height: b.H() * k,
		}
	}
	return editor.IndicatorDots(px, render.Rect{Width: vw, Height: vh})
}
