//go:build darwin

package mcpserver

import (
	"sync"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/cg"
)

var (
	cgOnce sync.Once
	cgRend *cg.Renderer
)

// cgRasterizer draws with CoreGraphics and CoreText into an offscreen bitmap; it needs no window.
type cgRasterizer struct{ r *cg.Renderer }

func newRasterizer() (rasterizer, error) {
	cgOnce.Do(func() { cgRend = cg.NewRenderer(cg.Options{TextCache: true}) })
	return cgRasterizer{cgRend}, nil
}

func (c cgRasterizer) Measurer() gfx.TextMeasurer { return c.r }

func (c cgRasterizer) PNG(sc *gfx.Scene, root *gfx.Node, scale float64) ([]byte, error) {
	raster, err := gfx.RenderSubtree(c.r, sc, root, gfx.Rect{}, scale, gfx.Color{R: 1, G: 1, B: 1, A: 1})
	if err != nil {
		return nil, err
	}
	return raster.PNG()
}
