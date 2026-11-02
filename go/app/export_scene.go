package app

import (
	"bytes"
	"fmt"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/offscreen"

	"github.com/haiodo/tenniarb/go/exec"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/scene"
)

// The export of a scene goes through go/scene, not the classic renderer: only it knows auto-layout,
// styles, components and animations, and its SVG writer puts the animations into the document as CSS.

// exportScene builds the scene of e the way the canvas does, for the writers of go/scene.
func (v *Viewer) exportScene(e *model.Element) (*gfx.Scene, *scene.Screen, error) {
	if v.stree == nil {
		return nil, nil, fmt.Errorf("the scene of %s is not loaded", e.Name)
	}
	sc := gfx.NewScene()
	var screen *scene.Screen
	var err error
	sc.Update(func(tx *gfx.Tx) {
		screen, err = v.stree.BuildElement(tx, nil, v.elementNode(e), scene.Options{
			Measurer: gfx.DefaultMeasurer, EvalVars: exec.SceneEvalVars,
		})
	})
	if err != nil {
		return nil, nil, err
	}
	if screen == nil {
		return nil, nil, fmt.Errorf("the scene of %s is empty", e.Name)
	}
	sc.Commit()
	return sc, screen, nil
}

// exportSVGOf is ExportSVG for an element the viewer has: the scene writer for a scene, the classic
// renderer otherwise.
func (v *Viewer) exportSVGOf(e *model.Element, p Prefs) ([]byte, error) {
	if !v.isScene(e) {
		return ExportSVG(e, p)
	}
	sc, screen, err := v.exportScene(e)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := screen.WriteSVG(&buf, sc, gfx.Rect{}, 1); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// exportPNGOf is ExportPNG for an element the viewer has.
func (v *Viewer) exportPNGOf(e *model.Element, p Prefs, scale float64) ([]byte, error) {
	if !v.isScene(e) {
		return ExportPNG(e, p, scale)
	}
	sc, screen, err := v.exportScene(e)
	if err != nil {
		return nil, err
	}
	rend, err := offscreen.Default()
	if err != nil {
		return nil, err
	}
	raster, err := gfx.RenderSubtree(rend, sc, screen.Root, gfx.Rect{}, scale, p.exportBackground())
	if err != nil {
		return nil, err
	}
	return raster.PNG()
}
