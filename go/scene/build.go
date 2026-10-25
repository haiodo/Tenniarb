package scene

import (
	"math"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/layout"
)

// Screen is one element built into the scene.
type Screen struct {
	Name string
	// W and H are the extent of the content from the origin of the element, as the layout sees it;
	// the scene bounds (Root.SubtreeBounds) also hold anything at a negative position.
	W, H float64
	Root *gfx.Node
	// Warnings are the warnings of this element and of what it pulls in through use and import.
	Warnings Errors

	tree *RNode
	p    *prep
}

// Box is the laid out rectangle of one item, relative to its parent.
type Box struct {
	Path, Name string // Path: normalized names from the element, joined by "."
	X, Y, W, H float64
	Depth      int
	Lines      []string // wrapped text lines of an item with a title
}

// Boxes lists the items of the screen in tree order, the element itself first.
func (s *Screen) Boxes() []Box {
	var out []Box
	var walk func(rn *RNode, path string, depth int)
	walk = func(rn *RNode, path string, depth int) {
		ln := s.p.frames[rn]
		b := Box{Path: path, Name: rn.Name, X: ln.RX, Y: ln.RY, W: ln.RW, H: ln.RH, Depth: depth}
		if ti := s.p.texts[rn]; ti != nil {
			b.Lines = ti.wrap(ln.RW - 2*ti.padX)
		}
		out = append(out, b)
		for _, k := range rn.Kids {
			if k.Kind != RItem || s.p.frames[k] == nil {
				continue
			}
			kp := k.Key
			if path != "" {
				kp = path + "." + k.Key
			}
			walk(k, kp, depth+1)
		}
	}
	walk(s.tree, "", 0)
	return out
}

// Build resolves the element at path ("" is the content of the file itself), lays it out and adds the
// scene nodes to tx under parent (the scene root when nil). The element's node sits at the origin.
// The error holds what is wrong with the element; the screen is nil then.
//
// A Tree is not safe for concurrent use.
func (t *Tree) Build(tx *gfx.Tx, parent *gfx.Node, path string, o Options) (*Screen, error) {
	var local Errors
	t.sink = &local
	defer func() { t.sink = nil }()
	rn := t.element(path)
	if rn == nil {
		return nil, local.asError()
	}
	p := newPrep(t, o)
	lay := p.root(rn)
	if local.Fatal() {
		return nil, local
	}
	lay.Layout()
	sc := &Screen{Name: rn.Name, W: lay.RW, H: lay.RH, tree: rn, p: p}
	b := &builder{p: p, tx: tx, next: p.opts.FirstID}
	sc.Root = b.emit(rn, parent, true)
	if local.Fatal() {
		return nil, local
	}
	for _, e := range local {
		if e.Warn {
			sc.Warnings = append(sc.Warnings, e)
		}
	}
	return sc, nil
}

// element expands the element at path into resolved nodes, slots filled.
func (t *Tree) element(path string) *RNode {
	target := t.main
	if path != "" {
		target = t.resolve(t.main, path, t.main.Pos)
		if target == nil {
			return nil
		}
	}
	if target.Kind == KMount {
		target = target.Target
	}
	if !target.isContainer() {
		t.fail(t.main.Pos, "path %s names a %s, not an element", path, "namespace")
		return nil
	}
	x := &expander{t: t}
	rn := x.container(target)
	if rn != nil {
		finalize(rn)
	}
	return rn
}

// validate expands and lays out every element of every loaded file, so problems show at Load.
func (t *Tree) validate() {
	var each func(n *Node)
	each = func(n *Node) {
		x := &expander{t: t}
		if rn := x.container(n); rn != nil {
			finalize(rn)
			newPrep(t, Options{}).root(rn)
		}
		for _, e := range n.Elems {
			each(e)
		}
	}
	for _, f := range t.order {
		each(f)
	}
}

type builder struct {
	p    *prep
	tx   *gfx.Tx
	next int
}

func (b *builder) node() *gfx.Node {
	n := gfx.NewNode(b.next)
	b.next++
	return n
}

// emit adds rn and its subtree under parent in file order, which is the z order.
func (b *builder) emit(rn *RNode, parent *gfx.Node, isRoot bool) *gfx.Node {
	ln := b.p.frames[rn]
	node := b.node()
	b.tx.Add(parent, node)
	b.tx.Move(node, ln.RX, ln.RY)
	if !isRoot {
		st := b.p.styles[rn]
		if pic := b.picture(rn, ln, st); pic != nil {
			b.tx.SetPicture(node, pic)
		}
		if st.opacity < 1 {
			b.tx.SetOpacity(node, st.opacity)
		}
		if st.clip {
			b.clip(node, ln, st.radius)
		}
	}
	for _, k := range rn.Kids {
		switch k.Kind {
		case RItem:
			if b.p.frames[k] != nil { // a hidden item has no frame
				b.emit(k, node, false)
			}
		case RLink:
			b.link(k, node)
		}
	}
	return node
}

func (b *builder) clip(node *gfx.Node, ln *layout.Node, radius [4]float64) {
	r := gfx.R(0, 0, ln.RW, ln.RH)
	switch {
	case radius[0] == radius[1] && radius[1] == radius[2] && radius[2] == radius[3]:
		if radius[0] > 0 {
			b.tx.ClipRRect(node, r, radius[0])
		} else {
			b.tx.ClipRect(node, r)
		}
	default:
		b.tx.ClipPath(node, gfx.RoundRectPath4(r, radius))
	}
}

func (b *builder) picture(rn *RNode, ln *layout.Node, st style) *gfx.Picture {
	w, h := ln.RW, ln.RH
	if w <= 0 || h <= 0 {
		return nil
	}
	r := gfx.NewRecorderWith(b.p.opts.Measurer)
	if ic := b.p.icons[rn]; ic != nil {
		ic.draw(r, w, h, st.lineW)
		if ti := b.p.texts[rn]; ti != nil {
			ti.draw(r, w, h, true, st.text)
		}
		return r.Finish()
	}
	if st.fill.A > 0 {
		shape := &gfx.Path{}
		shape.RoundRect4(0, 0, w, h, st.radius)
		r.FillPath(shape, st.fill)
	}
	if st.lineW > 0 && st.border.A > 0 {
		in := st.lineW / 2
		edge := &gfx.Path{}
		inner := st.radius
		for i := range inner {
			inner[i] = math.Max(inner[i]-in, 0)
		}
		edge.RoundRect4(in, in, w-st.lineW, h-st.lineW, inner)
		r.StrokePath(edge, gfx.StrokeStyle{Width: st.lineW, MiterLimit: 10}, st.border)
	}
	if ti := b.p.texts[rn]; ti != nil {
		leaf := len(ln.Kids) == 0
		ti.draw(r, w, h, leaf, st.text)
	}
	return r.Finish()
}

// draw writes the title: in a leaf centered unless layout says left or right, at the top left of an item that has children.
func (ti *textInfo) draw(r *gfx.Recorder, w, h float64, leaf bool, c gfx.Color) {
	lines := ti.wrap(w - 2*ti.padX)
	y0 := ti.padY
	if leaf {
		y0 = (h - float64(len(lines))*ti.lineH) / 2
	}
	for i, s := range lines {
		if s == "" {
			continue
		}
		x := ti.padX
		if leaf {
			switch ti.align {
			case "left":
			case "right":
				x = w - ti.padX - ti.width(s)
			default:
				x = (w - ti.width(s)) / 2
			}
		}
		base := y0 + float64(i)*ti.lineH + (ti.lineH-(ti.asc+ti.desc))/2 + ti.asc
		r.Text(s, x, base, ti.font, c)
	}
}

// link draws a line between the two items, from edge to edge of their boxes, in the parent's coordinates.
func (b *builder) link(lk *RNode, node *gfx.Node) {
	ends, ok := b.p.ends[lk]
	if !ok {
		return
	}
	a, c := b.p.frames[ends[0]], b.p.frames[ends[1]]
	pa := gfx.Point{X: a.RX + a.RW/2, Y: a.RY + a.RH/2}
	pc := gfx.Point{X: c.RX + c.RW/2, Y: c.RY + c.RH/2}
	if pa == pc {
		return
	}
	from, to := edgePoint(pa, a, pc), edgePoint(pc, c, pa)
	ln := b.node()
	b.tx.Add(node, ln)
	col, width := b.p.linkStyle(lk)
	r := gfx.NewRecorderWith(b.p.opts.Measurer)
	path := &gfx.Path{}
	path.MoveTo(from.X, from.Y)
	path.LineTo(to.X, to.Y)
	r.StrokePath(path, gfx.StrokeStyle{Width: width, MiterLimit: 10}, col)
	if pr := b.p.prop(lk, "display"); pr != nil {
		if s, _ := pr.text(0); s == "arrow" {
			r.FillPath(arrowHead(from, to, 6+width), col)
		}
	}
	b.tx.SetPicture(ln, r.Finish())
}

// edgePoint is where the line from the center of box n toward other leaves the box.
func edgePoint(center gfx.Point, n *layout.Node, other gfx.Point) gfx.Point {
	dx, dy := other.X-center.X, other.Y-center.Y
	t := math.Inf(1)
	if dx != 0 {
		t = math.Min(t, n.RW/2/math.Abs(dx))
	}
	if dy != 0 {
		t = math.Min(t, n.RH/2/math.Abs(dy))
	}
	t = math.Min(t, 1)
	return gfx.Point{X: center.X + dx*t, Y: center.Y + dy*t}
}

func arrowHead(from, to gfx.Point, size float64) *gfx.Path {
	d := to.Sub(from).Unit()
	n := gfx.Point{X: -d.Y, Y: d.X}
	base := to.Sub(d.Mul(size))
	p := &gfx.Path{}
	p.MoveTo(to.X, to.Y)
	p.LineTo(base.X+n.X*size/2, base.Y+n.Y*size/2)
	p.LineTo(base.X-n.X*size/2, base.Y-n.Y*size/2)
	p.Close()
	return p
}

type style struct {
	fill, border, text gfx.Color
	lineW              float64
	radius             [4]float64 // top left, top right, bottom right, bottom left
	clip               bool
	opacity            float64
}

var (
	white = gfx.Color{R: 1, G: 1, B: 1, A: 1}
	black = gfx.Color{A: 1}
)

func (p *prep) color(rn *RNode, name string, def gfx.Color) gfx.Color {
	pr := p.prop(rn, name)
	if pr == nil {
		return def
	}
	s, _ := pr.text(0)
	c, ok := parseColor(s)
	if !ok {
		p.t.warn(pr.Pos, "%s: %q is not a color", name, s)
		return def
	}
	return c
}

// style reads the look of an item: Tenniarb's defaults are a white fill and a black 1 point border;
// in a layout context there is no fill and no border until color or border-color is given.
func (p *prep) style(rn *RNode, inCtx bool) style {
	st := style{lineW: 1, opacity: 1}
	fill, border := white, black
	if inCtx {
		fill, border = gfx.Color{}, gfx.Color{}
	}
	st.fill = p.color(rn, "color", fill)
	st.border = p.color(rn, "border-color", border)
	text := black
	if st.fill.A > 0 {
		text = contrast(st.fill)
	}
	st.text = p.color(rn, "text-color", text)
	if v, ok := p.num(rn, "line-width"); ok {
		st.lineW = v
	}
	if pr := p.prop(rn, "corner-radius"); pr != nil {
		st.radius = p.radii(pr)
	}
	if v, ok := p.num(rn, "opacity"); ok {
		if v < 0 || v > 1 {
			p.t.warn(p.prop(rn, "opacity").Pos, "opacity takes 0..1, got %v", v)
		} else {
			st.opacity = v
		}
	}
	if inCtx {
		st.clip = p.flag(rn, "clip")
	}
	return st
}

func (p *prep) linkStyle(lk *RNode) (gfx.Color, float64) {
	w := 1.0
	if v, ok := p.num(lk, "line-width"); ok {
		w = v
	}
	return p.color(lk, "color", black), w
}

// radii reads corner-radius: one value for all corners or four in CSS order; anything else uses the first value.
func (p *prep) radii(pr *Prop) [4]float64 {
	var out [4]float64
	first, _ := pr.num(0)
	for i := range out {
		out[i] = first
	}
	switch len(pr.Args) {
	case 0, 1:
	case 4:
		for i := range out {
			v, ok := pr.num(i)
			if !ok {
				p.t.warn(pr.Pos, "corner-radius: radius %d is not a number, the first value is used for all corners", i+1)
				return [4]float64{first, first, first, first}
			}
			out[i] = v
		}
	default:
		p.t.warn(pr.Pos, "corner-radius: takes 1 or 4 radii (top left, top right, bottom right, bottom left), the first value is used")
	}
	return out
}
