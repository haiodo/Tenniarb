package app

import (
	"errors"
	"image"
	"image/draw"
	"math"
	"sync"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/offscreen"
	"github.com/haiodo/gowt/swt"

	"github.com/haiodo/tenniarb/go/exec"
	"github.com/haiodo/tenniarb/go/scene"
)

// componentPreview is a component as the browser and the picker draw it: the piece fitted into a w x h
// point canvas, centered, on a transparent background, and drawn again for every zoom the system asks
// for. Every preview has one size, and a picture of its own size would be stretched by the tree and
// blurry on a Retina screen.
type componentPreview struct {
	sc     *gfx.Scene
	root   *gfx.Node
	bounds gfx.Rect
	w, h   int
	grow   float64 // how far a small component may be blown up in the picture, 1 for a row
}

// componentPreviewImage draws a component of a library: a component that is an item is drawn as the piece
// it takes in its element, an element whole; scaled to fit w x h and blown up no more than grow times.
func (v *Viewer) componentPreviewImage(n *scene.Node, w, h int, grow float64) (*g.Image, error) {
	p, err := v.componentPreviewOf(n, w, h, grow)
	if err != nil {
		return nil, err
	}
	return v.app.ImageFromProvider(p), nil
}

// componentPreviewOf builds the scene of a component and the piece to draw from it.
func (v *Viewer) componentPreviewOf(n *scene.Node, w, h int, grow float64) (*componentPreview, error) {
	if v.stree == nil {
		return nil, errors.New("no scene to draw")
	}
	return previewOf(v.stree, n, w, h, grow)
}

// previewOf builds n of the tree and the piece it draws.
func previewOf(tree *scene.Tree, n *scene.Node, w, h int, grow float64) (*componentPreview, error) {
	if tree == nil || n == nil {
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
		screen, err = tree.BuildElement(tx, nil, build, scene.Options{Measurer: gfx.DefaultMeasurer, EvalVars: exec.SceneEvalVars})
	})
	if err != nil {
		return nil, err
	}
	if screen == nil {
		return nil, errors.New("the component draws nothing")
	}
	sc.Commit()
	p := &componentPreview{sc: sc, root: screen.Root, w: w, h: h, grow: max(grow, 1)}
	sc.Read(func() {
		if piece := placedOf(screen.Placed(), n); piece != nil && piece.Gfx != nil {
			p.root = piece.Gfx
		}
		p.bounds = p.root.SubtreeBounds()
	})
	if p.bounds.Empty() {
		return nil, errors.New("the component draws nothing")
	}
	return p, nil
}

// placeholderSource is the puzzle a node a use can take wears, as a component that draws nothing shows it:
// a component of slots is empty until it is filled.
const placeholderSource = `element Placeholder {
    item Puzzle {
        width 22; height 22; view-box 0 0 24 24; border-color #7c7c82; line-width 2
        path "M19.439 7.85c-.049.322.059.648.289.878l1.568 1.568c.47.47.706 1.087.706 1.704s-.235 1.233-.706 1.704l-1.611 1.611a.98.98 0 0 1-.837.276c-.47-.07-.802-.48-.968-.925a2.501 2.501 0 1 0-3.214 3.214c.446.166.855.497.925.968a.979.979 0 0 1-.276.837l-1.61 1.61a2.404 2.404 0 0 1-1.705.707 2.402 2.402 0 0 1-1.704-.706l-1.568-1.568a1.026 1.026 0 0 0-.877-.29c-.493.074-.84.504-1.02.968a2.5 2.5 0 1 1 3.237-3.237c.464-.18.894-.527.967-1.02a1.026 1.026 0 0 0-.289-.877l-1.568-1.568A2.402 2.402 0 0 1 1.998 12c0-.617.236-1.234.706-1.704L4.23 8.77c.24-.24.581-.353.917-.303.515.077.877.528 1.073 1.01a2.5 2.5 0 1 0 3.259-3.259c-.482-.196-.933-.558-1.01-1.073-.05-.336.062-.676.303-.917l1.525-1.525A2.402 2.402 0 0 1 12 1.998c.617 0 1.234.236 1.704.706l1.568 1.568c.23.23.556.338.877.29.493-.074.84-.504 1.02-.968a2.5 2.5 0 1 1 3.237 3.237c-.464.18-.894.527-.967 1.02Z"
    }
}`

var placeholder struct {
	once sync.Once
	tree *scene.Tree
}

// placeholderPreview is the picture of a component that draws nothing: the same canvas as a preview, so a
// tree of rows keeps one image size.
func placeholderPreview(w, h int, grow float64) *componentPreview {
	placeholder.once.Do(func() {
		tree, _ := scene.LoadSource("", "placeholder.tenn", placeholderSource)
		placeholder.tree = tree
	})
	f := placeholder.tree.File("placeholder.tenn")
	if f == nil || len(f.Elems) == 0 || len(f.Elems[0].Content) == 0 {
		return nil
	}
	p, _ := previewOf(placeholder.tree, f.Elems[0].Content[0], w, h, grow)
	return p
}

// GetImageData draws the preview with zoom device pixels per point: the content keeps its proportions on
// the canvas, whatever size the row or the pane has.
func (p *componentPreview) GetImageData(zoom int32) *swt.ImageData {
	return swt.NewImageDataFromNRGBA(p.pixels(float64(zoom) / 100))
}

// pixels is the preview at z device pixels per point: the content centered on a transparent w x h point
// canvas, so every row picture has one size and the tree draws none of them stretched.
func (p *componentPreview) pixels(z float64) *image.NRGBA {
	z = math.Max(z, 1)
	w, h := int(math.Round(float64(p.w)*z)), int(math.Round(float64(p.h)*z))
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	raster, err := p.raster(math.Min(math.Min(float64(w)/p.bounds.W(), float64(h)/p.bounds.H()), z*p.grow))
	if err != nil {
		return out // a blank point of the canvas, never a nil the image cannot take
	}
	x, y := (w-raster.W)/2, (h-raster.H)/2
	draw.Draw(out, image.Rect(x, y, x+raster.W, y+raster.H), raster.Image(), image.Point{}, draw.Src)
	return out
}

// raster draws the content at scale device pixels per scene unit; the renderer is made per call, as the
// preview outlives the one the browser was built with.
func (p *componentPreview) raster(scale float64) (*gfx.Raster, error) {
	r, err := offscreen.Default()
	if err != nil {
		return nil, err
	}
	return gfx.RenderSubtree(r, p.sc, p.root, p.bounds, scale, gfx.Color{})
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
