package render

import (
	"fmt"
	"io"
	"math"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/offscreen"

	"github.com/haiodo/tenniarb/go/exec"
	"github.com/haiodo/tenniarb/go/model"
)

// Picture records one drawable (a top-level box or line with its text) into a gfx.Picture in y-down
// coordinates of the scene, without padding. A window can re-record and replace just one such picture.
func (s *DrawableScene) Picture(d Drawable) *gfx.Picture {
	rec := gfx.NewRecorderWith(s.env.measurer)
	d.Draw(rec, Point{})
	return rec.Finish()
}

// Rendered is a scene realized as gfx nodes: Root holds one node per top-level drawable, in draw order.
type Rendered struct {
	Root *gfx.Node
	// Width and Height are the size of the image, padding included; the content starts at (padding, padding).
	Width, Height float64
	Nodes         map[Drawable]*gfx.Node
	// NextID is the first node id not used.
	NextID int
}

// Area is the image area in scene units, to pass to gfx.RenderSubtree and gfx.WriteSubtreeSVG.
func (r *Rendered) Area() gfx.Rect { return gfx.R(0, 0, r.Width, r.Height) }

// Realize adds the scene to tx under parent (the gfx scene root when nil). Layout must have run; Build does it.
func (s *DrawableScene) Realize(tx *gfx.Tx, parent *gfx.Node) *Rendered {
	bounds := s.Bounds()
	padding := s.opts.padding()
	id := s.opts.FirstID
	if id == 0 {
		id = 1
	}
	res := &Rendered{
		Width:  bounds.Width + padding*2,
		Height: bounds.Height + padding*2,
		Nodes:  map[Drawable]*gfx.Node{},
	}

	res.Root = gfx.NewNode(id)
	id++
	tx.Add(parent, res.Root)
	// The scene is y-up with its top at maxY; the image is y-down with the content padded.
	tx.Move(res.Root, padding-bounds.X, padding+maxY(bounds))

	for i, d := range s.elements.Ordered() {
		pic := s.Picture(d)
		if pic.Len() == 0 {
			continue
		}
		n := gfx.NewNode(id)
		id++
		tx.Add(res.Root, n)
		tx.SetPicture(n, pic)
		tx.SetZ(n, i)
		res.Nodes[d] = n
	}
	res.NextID = id
	return res
}

// Build evaluates, builds and lays out the scene of element. Without o.Exec it makes an expression context
// (NewExecContext) unless o.NoEvaluate is set.
func Build(element *model.Element, o Options) *DrawableScene {
	if o.Exec == nil && !o.NoEvaluate {
		o.Exec = NewExecContext(element, o)
	}
	s := NewDrawableScene(element, o)
	b := s.Bounds()
	s.Layout(b, b)
	return s
}

// SceneSize is the size of the image Render produces for element.
func SceneSize(element *model.Element, o Options) Size {
	b := Build(element, o).Bounds()
	p := o.padding()
	return Size{b.Width + p*2, b.Height + p*2}
}

func (s *DrawableScene) realizeNew() (*gfx.Scene, *Rendered) {
	gs := gfx.NewScene()
	var r *Rendered
	gs.Update(func(tx *gfx.Tx) { r = s.Realize(tx, nil) })
	gs.Commit()
	return gs, r
}

// PNG draws element at scale device pixels per unit on bg (transparent for the zero Color).
func PNG(element *model.Element, o Options, scale float64, bg gfx.Color) ([]byte, error) {
	gs, r := Build(element, o).realizeNew()
	rend, err := offscreen.Default()
	if err != nil {
		return nil, err
	}
	raster, err := gfx.RenderSubtree(rend, gs, r.Root, r.Area(), scale, bg)
	if err != nil {
		return nil, err
	}
	return raster.PNG()
}

// SVG writes element as an SVG document of the image size times scale.
func SVG(w io.Writer, element *model.Element, o Options, scale float64) error {
	return SVGBackground(w, element, o, scale, gfx.Color{})
}

// SVGBackground is SVG on a bg rectangle behind the content (none for the zero Color).
func SVGBackground(w io.Writer, element *model.Element, o Options, scale float64, bg gfx.Color) error {
	gs, r := Build(element, o).realizeNew()
	return gfx.WriteSubtreeSVGWith(w, gs, r.Root, r.Area(), scale, gfx.SVGOptions{Background: bg})
}

// MeasureTextSize is utils.textSize of the expression engine: a frame at most 1000 wide in the default
// font (CTFramesetterSuggestFrameSizeWithConstraints).
func MeasureTextSize(o Options, text string, fontSize float64) Size {
	env := newTextEnv(o.Measurer, o.FontFamily)
	maxW := 1000.0
	lines := env.layoutLines(PlainText(text, systemFont(fontSize)), &maxW)
	var size Size
	for _, l := range lines {
		size.Width = math.Max(size.Width, l.width)
		size.Height += l.textAscent + l.textDescent
	}
	return Size{math.Ceil(size.Width), math.Ceil(size.Height)}
}

// ItemSize is the size of the drawable of item, for the expression engine (`width` and `height` in
// expressions). Swift builds a throwaway scene per evaluation round and reads the drawable's size.
//
// The scene reads evaluated values from o.Exec. The engine calls ItemSize with its lock held, so there o.Exec
// must be the read-only view the engine passes in (ExecHooks does that), never the engine itself.
func ItemSize(item *model.DiagramItem, o Options) (model.DrawableSize, error) {
	if item.Parent == nil {
		return model.DrawableSize{}, fmt.Errorf("item %q has no parent element", item.Name)
	}
	o.SkipChildren = true
	o.Items = nil
	s := NewDrawableScene(item.Parent, o)
	s.BuildItemDrawable(item, NewDrawableContainer())
	d, ok := s.Drawables[item]
	if !ok {
		return model.DrawableSize{}, fmt.Errorf("item %q has no drawable", item.Name)
	}
	b := d.SelectorBounds()
	return model.DrawableSize{Width: b.Width, Height: b.Height}, nil
}

// ExecHooks are the two callbacks exec.Options takes (ItemSize and MeasureText), measuring with o. ItemSize
// sizes the item from the evaluated values of the previous round, as Swift and TS do; o.Exec is replaced by
// the engine's view for that.
func ExecHooks(o Options) (itemSize func(*model.DiagramItem, model.IExecutionContext) *model.DrawableSize, measureText func(string, float64) model.DrawableSize) {
	itemSize = func(item *model.DiagramItem, prev model.IExecutionContext) *model.DrawableSize {
		oo := o
		oo.Exec = prev
		sz, err := ItemSize(item, oo)
		if err != nil {
			return nil
		}
		return &sz
	}
	measureText = func(text string, fontSize float64) model.DrawableSize {
		sz := MeasureTextSize(o, text, fontSize)
		return model.DrawableSize{Width: sz.Width, Height: sz.Height}
	}
	return itemSize, measureText
}

// NewExecContext is an expression context for element that measures with o and has evaluated it once.
// With o.NoEvaluate it only returns tokens as written.
func NewExecContext(element *model.Element, o Options) *exec.ExecutionContext {
	itemSize, measureText := ExecHooks(o)
	ctx := exec.NewExecutionContext(exec.Options{NoEvaluate: o.NoEvaluate, RoundLimit: o.ExecLimit, ItemSize: itemSize, MeasureText: measureText})
	if o.ScaleFactor != 0 {
		ctx.SetScaleFactor(o.ScaleFactor)
	}
	ctx.SetElement(element)
	return ctx
}
