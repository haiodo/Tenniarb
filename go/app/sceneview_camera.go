//go:build darwin || linux || windows

package app

import (
	"math"
	"strconv"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/view"

	"github.com/haiodo/tenniarb/go/scene"
)

// setCamera moves the camera; the selection frames are part of the scene and follow it.
func (v *Viewer) setCamera(cam gfx.Camera) {
	v.view.Compositor().SetCamera(cam)
	v.drawSceneOverlay()
}

// panView pans the canvas; the selection frames are part of the scene and follow it.
func (v *Viewer) panView(dx, dy float64) {
	v.view.Pan(dx, dy)
	v.drawSceneOverlay()
}

// onScroll pans the canvas sideways with Shift+wheel. GTK reports the wheel on one axis only, so
// without this the canvas moves up and down but never sideways.
func (v *Viewer) onScroll(s *view.Scroll) bool {
	if !s.Mods.Shift || s.DX != 0 {
		return false
	}
	dx := s.DY
	if !s.Precise {
		dx *= wheelLine
	}
	v.panView(dx, 0)
	return true
}

// drawSceneOverlay draws the selection frames on the HUD, so a zoom or a pan does not damage the
// tiles of the scene.
func (v *Viewer) drawSceneOverlay() {
	if v.view == nil {
		return
	}
	v.syncMarks() // the marks sit on the nodes, so they move with the camera
	pic := gfx.NewRecorder().Finish()
	if v.sceneDrawn() {
		pic = v.sv.selectionPicture(v.canvasSelection(), v.greenPath())
	}
	if v.selHUD != nil || pic.Len() > 0 {
		v.scene.Update(func(tx *gfx.Tx) {
			if v.selHUD == nil {
				v.selHUD = gfx.NewNode(3)
				tx.Add(nil, v.selHUD)
				tx.SetShape(v.selHUD, nil, gfx.HitNone) // hit-testing of the selection is done on the scene
				tx.SetZ(v.selHUD, overlayZ)
			}
			tx.SetPicture(v.selHUD, pic)
		})
	}
	v.syncCompareHUD()
}

// selectScene makes paths the selection of the scene element on the canvas.
func (v *Viewer) selectScene(paths []string) {
	v.selNodes = paths
	if v.state.Current == v.docElem {
		v.docSel = paths
	}
	v.drawSceneOverlay()
	v.refreshProps()
	v.syncItems()
	v.updateMenus()
	v.sh.publish(DocEvent{Path: v.state.Doc.Path, Kind: EventSelect, Version: v.version})
}

// revealScene centers the canvas on the first selected node when it is out of view.
func (v *Viewer) revealScene() {
	_, pl := v.selectedScene()
	if pl == nil {
		return
	}
	w, h := v.view.Canvas().Size()
	cam := v.view.Compositor().Camera()
	c := gfx.Point{X: pl.X + pl.W/2 + v.sv.off.X, Y: pl.Y + pl.H/2 + v.sv.off.Y}
	vx, vy := (c.X-cam.Origin.X)*cam.Zoom, (c.Y-cam.Origin.Y)*cam.Zoom
	if vx < 0 || vy < 0 || vx > float64(w) || vy > float64(h) {
		v.setCamera(CenterOn(cam, c, float64(w), float64(h)))
	}
}

func parseNum(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// zoomToSelection fits the selected scene node into the canvas.
func (v *Viewer) zoomToSelection() {
	_, pl := v.selectedScene()
	if pl == nil || pl.W <= 0 || pl.H <= 0 {
		return
	}
	w, h := v.view.Canvas().Size()
	z := ClampZoom(0.9 * math.Min(float64(w)/pl.W, float64(h)/pl.H))
	c := gfx.Point{X: pl.X + pl.W/2 + v.sv.off.X, Y: pl.Y + pl.H/2 + v.sv.off.Y}
	v.setCamera(CenterOn(gfx.Camera{Zoom: z}, c, float64(w), float64(h)))
	v.drawSceneOverlay()
}

// plCentre is the middle of an item, in content coordinates.
func plCentre(pl *scene.Placed) gfx.Point {
	return gfx.Point{X: pl.X + pl.W/2, Y: pl.Y + pl.H/2}
}

// greenPath is the frame drawn green: the item of the text being edited, or the item a link is dropped on.
func (v *Viewer) greenPath() string {
	if v.sv != nil && v.sv.lineTarget != nil {
		return v.sv.pathOf[v.sv.lineTarget]
	}
	return v.editingPath()
}
