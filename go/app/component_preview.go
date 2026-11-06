package app

import (
	"bytes"
	"errors"
	"math"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/offscreen"

	"github.com/haiodo/tenniarb/go/exec"
	"github.com/haiodo/tenniarb/go/scene"
)

// componentImage draws a component of a library as the picker previews it: a component that is an item
// is drawn as the piece it takes in its element, an element whole; scaled to fit w x h, never blown up,
// on a transparent background.
func (v *Viewer) componentImage(n *scene.Node, w, h int) (*g.Image, error) {
	if v.stree == nil || n == nil {
		return nil, errors.New("no scene to draw")
	}
	// The canvas builds elements, so an item is drawn as its part of the element it sits in.
	build := n
	if n.Kind != scene.KElement && n.Kind != scene.KFile {
		for p := n.Parent; p != nil; p = p.Parent {
			if p.Kind == scene.KElement || p.Kind == scene.KFile {
				build = p
				break
			}
		}
	}
	sc := gfx.NewScene()
	var screen *scene.Screen
	var err error
	sc.Update(func(tx *gfx.Tx) {
		screen, err = v.stree.BuildElement(tx, nil, build, scene.Options{Measurer: gfx.DefaultMeasurer, EvalVars: exec.SceneEvalVars})
	})
	if err != nil {
		return nil, err
	}
	if screen == nil {
		return nil, errors.New("the component draws nothing")
	}
	sc.Commit()
	root := screen.Root
	var bounds gfx.Rect
	sc.Read(func() {
		if piece := placedOf(screen.Placed(), n); piece != nil && piece.Gfx != nil {
			root = piece.Gfx
		}
		bounds = root.SubtreeBounds()
	})
	if bounds.Empty() {
		return nil, errors.New("the component draws nothing")
	}
	scale := math.Min(math.Min(float64(w)/bounds.W(), float64(h)/bounds.H()), 1)
	r, err := offscreen.Default()
	if err != nil {
		return nil, err
	}
	raster, err := gfx.RenderSubtree(r, sc, root, bounds, scale, gfx.Color{})
	if err != nil {
		return nil, err
	}
	png, err := raster.PNG()
	if err != nil {
		return nil, err
	}
	return v.app.ImageFrom(bytes.NewReader(png))
}

// placedOf is where n is drawn in the tree of placed nodes, nil when it is not drawn.
func placedOf(p *scene.Placed, n *scene.Node) *scene.Placed {
	if p == nil {
		return nil
	}
	if p.Src == n {
		return p
	}
	for _, k := range p.Kids {
		if got := placedOf(k, n); got != nil {
			return got
		}
	}
	return nil
}
