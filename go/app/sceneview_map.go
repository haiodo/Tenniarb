//go:build darwin || linux || windows

package app

import (
	"slices"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/classic"
	"github.com/haiodo/tenniarb/go/geometry"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/scene"
)

// sceneDrawn reports whether the canvas draws the element through go/scene: a scene, or a classic
// diagram whose picture the scene draws while the session keeps the input.
func (v *Viewer) sceneDrawn() bool { return v.sv != nil }

// mapOf is the map of a picture the scene drew: the model coordinates of the session go through the
// same offset as the drawn nodes, so a click and the frame it selects cannot drift apart.
func (sv *sceneView) mapOf() SceneMap { return SceneMap{TX: sv.off.X, TY: sv.off.Y} }

// syncMap makes the model coordinates of the session follow the picture of a classic diagram: the map
// is the offset the scene drew it with, not the bounds the old renderer would have given it.
func (v *Viewer) syncMap(sv *sceneView) bool {
	if !sv.classic || v.session == nil {
		return false
	}
	v.smap = sv.mapOf()
	v.selNodes = nil // the paths of the selected scene nodes name nothing here
	return true
}

// drawnRect is where the canvas draws a classic item of the element on it; false when the scene does
// not draw that item, a hidden one or a link.
func (v *Viewer) drawnRect(it *model.DiagramItem) (gfx.Rect, bool) {
	if pl := v.classicPlaced(it); pl != nil {
		return gfx.R(pl.X+v.sv.off.X, pl.Y+v.sv.off.Y, pl.W, pl.H), true
	}
	if b, ok := v.drawnLinkRect(it); ok {
		return b, true
	}
	return gfx.Rect{}, false
}

// drawnLinkRect is where the line of a classic link is drawn: the box of its two ends, which the scene
// reports for the node that writes the link.
func (v *Viewer) drawnLinkRect(it *model.DiagramItem) (gfx.Rect, bool) {
	if v.sv == nil || v.sv.screen == nil || v.state.Current == nil || it == nil || it.Kind != model.ItemKindLink {
		return gfx.Rect{}, false
	}
	for _, c := range v.sv.node.Content {
		if c.Kind != scene.KLink || classic.LinkAt(v.state.Current, c) != it {
			continue
		}
		if b, ok := v.sv.screen.LinkBox(c); ok {
			return gfx.R(b.Min.X+v.sv.off.X, b.Min.Y+v.sv.off.Y, b.W(), b.H()), true
		}
	}
	return gfx.Rect{}, false
}

// drawnCovers is editor.Options.Covers of a classic diagram: the boxes of the session and the picture of
// the canvas come from the same scene, and a press where nothing is drawn still must not grab the item.
func (v *Viewer) drawnCovers(r geometry.Rect, it *model.DiagramItem) bool {
	if v.sv == nil || !v.sv.classic {
		return true
	}
	b, ok := v.drawnRect(it)
	if !ok {
		return true // nothing drawn for it: a link or a hidden item, the picture has no say
	}
	return b.Intersects(v.smap.RectToGfx(r))
}

// sceneMode reports whether the canvas shows a scene element: the pointer, the keys and the edits are
// the scene's. A classic diagram drawn by the scene stays out: it has no scene input yet.
func (v *Viewer) sceneMode() bool { return v.sv != nil && !v.sv.classic }

// canvasSelection is what the scene picture outlines: the selected nodes of a scene, the selected items
// of a classic diagram drawn by the scene.
func (v *Viewer) canvasSelection() []string {
	if v.sceneMode() {
		if t := v.sv.lineTarget; t != nil {
			if p, ok := v.sv.pathOf[t]; ok && !slices.Contains(v.selNodes, p) {
				return append(slices.Clone(v.selNodes), p)
			}
		}
		return v.selNodes
	}
	sel := map[*model.DiagramItem]bool{}
	for _, it := range v.sessionItems() {
		sel[it] = true
	}
	var out []string
	for _, pl := range v.sv.byPath {
		if pl.Src != nil && sel[classicItemAt(v.state.Current, pl.Src)] {
			out = append(out, v.sv.pathOf[pl])
		}
	}
	// The item under the pointer while a link is drawn is outlined too, so the drop target is visible.
	if v.session != nil {
		if t := v.session.LineTarget(); t != nil {
			if pl := v.classicPlaced(t); pl != nil {
				out = append(out, v.sv.pathOf[pl])
			}
		}
	}
	return out
}
