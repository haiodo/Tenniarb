//go:build darwin || linux || windows

package app

import (
	"slices"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/editor"
)

// indicators is the HUD node that holds the dots for the items outside the view.
type indicators struct {
	node *gfx.Node
	seen gfx.Camera // the camera of the last frame report
	dots []editor.Dot
	key  indicatorKey
}

// indicatorKey is what the picture depends on besides the dots.
type indicatorKey struct {
	vh, zoom float64
	dark     bool
}

// itemBounds are the rectangles of the items on the gfx scene: every drawable of a classic diagram,
// the top-level items of a scene element.
func (v *Viewer) itemBounds() []gfx.Rect {
	var out []gfx.Rect
	switch {
	case v.sceneMode():
		if v.sv.placed == nil {
			return nil
		}
		for _, k := range v.sv.placed.Kids {
			out = append(out, gfx.R(k.X+v.sv.off.X, k.Y+v.sv.off.Y, k.W, k.H))
		}
	case v.session != nil && v.rendered != nil:
		ds := v.session.Scene()
		for _, i := range ds.Items() {
			if d := ds.Drawables[i]; d != nil {
				out = append(out, v.smap.RectToGfx(d.Bounds()))
			}
		}
	}
	return out
}

// syncIndicators redraws the dots of the HUD after the camera, the view size, the theme or the items
// changed. The HUD scene is touched only when the picture would differ.
func (v *Viewer) syncIndicators() {
	if v.view == nil {
		return
	}
	defer v.syncPins()
	comp := v.view.Compositor()
	w, h, _ := comp.Viewport()
	cam := comp.Camera()
	dots := EdgeDots(v.itemBounds(), cam, w, h)
	key := indicatorKey{h, cam.Zoom, v.app.Dark()}
	ind := &v.ind
	if ind.node != nil && key == ind.key && slices.Equal(dots, ind.dots) {
		return
	}
	ind.dots, ind.key = dots, key
	pic := editor.IndicatorsPicture(dots, h, cam.Zoom, key.dark)
	hud := comp.HUD()
	hud.Update(func(tx *gfx.Tx) {
		if ind.node == nil {
			ind.node = gfx.NewNode(1)
			tx.Add(nil, ind.node)
			tx.SetShape(ind.node, nil, gfx.HitNone) // the dots never take the pointer
		}
		tx.SetPicture(ind.node, pic)
	})
}
