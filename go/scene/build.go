package scene

import (
	"math"
	"slices"

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
	// NextID is the id after the last scene node made.
	NextID int
	// Warnings are the warnings of this element and of what it pulls in through use and import.
	Warnings Errors
	// Animations are the animations of the items, by the scene node they play on (spec 13): WriteSVG
	// puts them into the document as CSS, and a viewer plays them with Animated.
	Animations map[*gfx.Node]*Animation
	// animated is Animations with what playing one needs: the box of the node in its parent and the
	// opacity the style gives it.
	animated []Animated
	// links and anchors are the links and the headings of the bodies, where they are drawn.
	links   []BodyLink
	anchors []BodyAnchor

	tree *RNode
	p    *prep
	gfx  map[*RNode]*gfx.Node
}

// Animated is one animation a viewer plays on its node: the node, the box of the item in its parent (a
// rotation turns about its center) and the opacity the style gives it.
type Animated struct {
	Node    *gfx.Node
	Box     Box
	Anim    *Animation
	Opacity float64
}

// AnimatedNodes lists the animations of the screen for a viewer that plays them.
func (s *Screen) AnimatedNodes() []Animated { return s.animated }

// AnimationOf is the animation playing on node, if any: the inspector offers it for the selected item.
func (s *Screen) AnimationOf(node *gfx.Node) *Animation {
	for _, a := range s.animated {
		if a.Node == node {
			return a.Anim
		}
	}
	return nil
}

// Box is the laid out rectangle of one item, relative to its parent.
type Box struct {
	Path, Name string // Path: normalized names from the element, joined by "."
	X, Y, W, H float64
	Depth      int
	Lines      []string // wrapped text lines of an item with a title
}

// Boxes lists the items of the screen in paint order, the element itself first.
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
		if s.p.styles[rn].display == "none" { // display none is not drawn, its children with it
			return
		}
		for _, k := range s.p.paintOrder(rn) {
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

// paintOrder lists the kids in draw order: background items under the rest, hover items over them, the
// file order within each pass. links keep the place of an item of no layer.
func (p *prep) paintOrder(rn *RNode) []*RNode {
	if len(rn.Kids) < 2 {
		return rn.Kids
	}
	out := slices.Clone(rn.Kids)
	slices.SortStableFunc(out, func(a, b *RNode) int { return p.layerKey(a) - p.layerKey(b) })
	return out
}

// layerKey is the position of a node among its siblings; the zero layer is the middle pass.
func (p *prep) layerKey(rn *RNode) int {
	if rn.Kind != RItem {
		return 1
	}
	switch p.styles[rn].layer {
	case layerBackground:
		return 0
	case layerHover:
		return 2
	}
	return 1
}

// Build resolves the element at path ("" is the content of the file itself), lays it out and adds the
// scene nodes to tx under parent (the scene root when nil). The element's node sits at the origin.
// The error holds what is wrong with the element; the screen is nil then.
//
// A Tree is not safe for concurrent use.
func (t *Tree) Build(tx *gfx.Tx, parent *gfx.Node, path string, o Options) (*Screen, error) {
	return t.build(tx, parent, o, func() *RNode { return t.element(path, evaluator(o)) })
}

// BuildElement is Build of an element or file root of the tree, for a caller that holds the node:
// a path of names cannot tell elements with one name apart.
func (t *Tree) BuildElement(tx *gfx.Tx, parent *gfx.Node, el *Node, o Options) (*Screen, error) {
	return t.build(tx, parent, o, func() *RNode {
		if !el.isContainer() {
			t.fail(el.Pos, "a %s is not an element", el.Kind)
			return nil
		}
		return t.expandElement(el, t.ctxOf(el), evaluator(o))
	})
}

func (t *Tree) build(tx *gfx.Tx, parent *gfx.Node, o Options, expand func() *RNode) (*Screen, error) {
	var local Errors
	t.sink = &local
	defer func() { t.sink = nil }()
	rn := expand()
	if rn == nil {
		return nil, local.asError()
	}
	// The expressions of the screen come before the styles, in the scope of their instance: use-style
	// ${kind} picks a style by its value (spec 5c, 6).
	p := newPrep(t, o)
	p.classic = !sceneRoot(rn)
	if p.classic {
		classicDefaults(rn)
	}
	if p.classic && p.opts.Classic != nil {
		p.classicRefs(rn)
	}
	if p.canEval() {
		p.evalTree(rn)
	}
	applyStyles(rn)
	lay := p.root(rn)
	if local.Fatal() {
		return nil, local
	}
	lay.Layout()
	p.warnOutside(rn)
	p.warnBodyFit(rn)
	sc := &Screen{Name: rn.Name, W: lay.RW, H: lay.RH, tree: rn, p: p}
	sc.collectBodies()
	b := &builder{p: p, tx: tx, next: p.opts.FirstID, made: map[*RNode]*gfx.Node{}}
	sc.Root = b.emit(rn, parent, true)
	sc.gfx, sc.NextID = b.made, b.next
	if len(p.anim) > 0 {
		sc.Animations = make(map[*gfx.Node]*Animation, len(p.anim))
		for rn, a := range p.anim {
			n := sc.gfx[rn]
			if n == nil {
				continue
			}
			sc.Animations[n] = a
			an := Animated{Node: n, Anim: a, Opacity: p.styles[rn].opacity}
			if ln := p.frames[rn]; ln != nil {
				an.Box = Box{X: ln.RX, Y: ln.RY, W: ln.RW, H: ln.RH}
			}
			sc.animated = append(sc.animated, an)
		}
	}
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
func (t *Tree) element(path string, eval evalFunc) *RNode {
	target, ctx := t.main, t.ctxOf(t.main)
	if path != "" {
		target, ctx = t.resolve(t.main, ctx, path, t.main.Pos)
		if target == nil {
			return nil
		}
	}
	if !target.isContainer() {
		// An item is drawn on its own with the defaults of its parameters, which item takes (spec 5c).
		if target.Kind != KItem {
			t.fail(t.main.Pos, "path %s names a %s, not an element or an item", path, target.Kind)
			return nil
		}
		return (&expander{t: t, ctx: ctx, eval: eval}).item(target)
	}
	return t.expandElement(target, ctx, eval)
}

func (t *Tree) expandElement(target *Node, ctx *mountCtx, eval evalFunc) *RNode {
	x := &expander{t: t, ctx: ctx, eval: eval}
	x.vars = x.defaults(target) // a component on its own shows its defaults (spec 5c)
	rn := x.container(target)
	if rn != nil {
		finalize(rn)
	}
	return rn
}

// validate expands and lays out every element of the main file and of the files mounted below it,
// each in the context of its mount, so problems show at Load.
func (t *Tree) validate() {
	var each func(n *Node, c *mountCtx)
	each = func(n *Node, c *mountCtx) {
		x := &expander{t: t, ctx: c}
		if rn := x.container(n); rn != nil {
			finalize(rn)
			// validate lays out without an evaluator: an expression is a value check cannot know, not a
			// problem of the file (the build with Options.Eval and check report it).
			p := newPrep(t, Options{noExprWarn: true})
			p.root(rn).Layout()
			p.warnOutside(rn)
			p.warnBodyFit(rn)
		}
		if n.Mount != nil {
			c = c.push(n)
		}
		for _, e := range n.Elems {
			each(e, c)
		}
	}
	each(t.main, t.ctxOf(t.main))
}

type builder struct {
	p    *prep
	tx   *gfx.Tx
	next int
	made map[*RNode]*gfx.Node
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
	b.made[rn] = node
	b.tx.Add(parent, node)
	b.tx.Move(node, ln.RX, ln.RY)
	if deg := b.p.rotation(rn); deg != 0 {
		b.tx.SetTransform(node, turn(ln.RX, ln.RY, ln.RW, ln.RH, deg))
	}
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
	} else if im := b.p.images[rn]; im != nil {
		// The root of a screen is otherwise a bare container; a picture there is the whole screen.
		r := gfx.NewRecorderWith(b.p.opts.Measurer)
		b.image(r, im, ln.RW, ln.RH, b.p.radiiOf(rn))
		b.tx.SetPicture(node, r.Finish())
	}
	if !isRoot && b.p.styles[rn].display == "none" {
		// display none takes room and gives bounds but is never drawn, its children with it.
		return node
	}
	for _, k := range b.p.paintOrder(rn) {
		switch k.Kind {
		case RItem:
			if b.p.frames[k] != nil { // a hidden item has no frame
				b.emit(k, node, false)
			}
		case RLink:
			b.link(rn, k, node)
		}
	}
	if bn := b.p.bodyAt[rn]; bn != nil {
		body := b.node()
		b.tx.Add(node, body)
		b.tx.Move(body, bn.RX, bn.RY)
		r := gfx.NewRecorderWith(b.p.opts.Measurer)
		b.p.bodies[rn].draw(r, 0, 0, bn.RW)
		b.tx.SetPicture(body, r.Finish())
	}
	return node
}

// image draws the picture of an item. The file is read here, at the build of a screen that is drawn,
// not at the layout of every element of the workspace.
func (b *builder) image(r *gfx.Recorder, im *itemImage, w, h float64, radius [4]float64) {
	if img := im.raster(b.p); img != nil {
		drawImage(r, img, im.fit, w, h, radius)
	}
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
	if w <= 0 || h <= 0 || st.display == "none" {
		return nil
	}
	r := gfx.NewRecorderWith(b.p.opts.Measurer)
	if ic := b.p.icons[rn]; ic != nil {
		ic.draw(r, w, h, st.lineW)
		if ti := b.p.texts[rn]; ti != nil {
			ti.draw(r, w, h, true, st.text, nil)
		}
		return r.Finish()
	}
	shadows := st.shadows
	if st.display == "stack" {
		// Three sheets behind the box, stepped 2 up and right, as the classic renderer draws a stack.
		for i := 1; i <= 3; i++ {
			d := 2 * float64(3-i)
			sheet := &gfx.Path{}
			sheet.RoundRect4(d, -d, w, h, st.radius)
			r.FillShadowPath(sheet, st.fill, stackShadow())
		}
	}
	if st.display != "text" && st.display != "no-fill" && st.fill.A > 0 {
		shape := boxPath(st, w, h)
		// A shadow takes the alpha of the fill; a gradient is drawn over an opaque base, so a gradient
		// with transparent stops shows the base. Ceiling: gfx has no gradient with a shadow.
		base := st.fill
		base.A = 1
		for _, sh := range shadows {
			r.FillShadowPath(shape, base, sh)
		}
		if st.grad != nil {
			r.FillGradient(shape, st.grad.at(w, h))
		} else if len(shadows) == 0 {
			r.FillPath(shape, st.fill)
		}
		shadows = nil
	}
	if im := b.p.images[rn]; im != nil && (st.display == "" || st.display == "rect") {
		b.image(r, im, w, h, st.radius)
	}
	if st.display != "text" && st.lineW > 0 && st.border.A > 0 {
		in := st.lineW / 2
		inner := st.radius
		for i := range inner {
			inner[i] = math.Max(inner[i]-in, 0)
		}
		var edge *gfx.Path
		if st.display == "circle" {
			edge = gfx.EllipsePath(gfx.R(in, in, w-st.lineW, h-st.lineW))
		} else {
			edge = &gfx.Path{}
			edge.RoundRect4(in, in, w-st.lineW, h-st.lineW, inner)
		}
		line := gfx.StrokeStyle{Width: st.lineW, MiterLimit: 10, Dash: st.dash, DashPhase: st.dashPhase}
		if len(shadows) > 0 { // no fill: the border casts the shadow
			for _, sh := range shadows {
				r.StrokeShadowPath(edge, line, st.border, sh)
			}
		} else {
			r.StrokePath(edge, line, st.border)
		}
	}
	if ti := b.p.texts[rn]; ti != nil {
		// With a body the title goes to the top left and the body follows it, as in classic diagrams.
		leaf := len(ln.Kids) == 0 && b.p.bodies[rn] == nil
		var sh *gfx.Shadow
		if st.display == "text" && len(st.shadows) > 0 {
			// display text has no box, so the shadow of the item falls on the glyphs.
			sh = &st.shadows[0]
		}
		ti.draw(r, w, h, leaf, st.text, sh)
	}
	if body := b.p.bodies[rn]; body != nil && b.p.bodyAt[rn] == nil {
		y := body.padY
		if ti := b.p.texts[rn]; ti != nil {
			y = ti.padY + ti.height(w-2*ti.padX)
		}
		body.draw(r, body.padX, y, w-2*body.padX)
	}
	return r.Finish()
}

// boxPath is the outline of the box of an item: an ellipse for display circle, else its rounded rectangle.
func boxPath(st style, w, h float64) *gfx.Path {
	if st.display == "circle" {
		return gfx.EllipsePath(gfx.R(0, 0, w, h))
	}
	p := &gfx.Path{}
	p.RoundRect4(0, 0, w, h, st.radius)
	return p
}

// stackShadow is the shadow of one sheet of display stack, as the classic renderer gives it.
func stackShadow() gfx.Shadow {
	return gfx.Shadow{DX: 1, DY: -1, Blur: defaultShadowBlur, Color: defaultShadowColor}
}

// draw writes the title: in a leaf centered unless layout says left or right, at the top left of an item that has children.
// With sh set the glyphs cast it (display text), else they are plain text.
func (ti *textInfo) draw(r *gfx.Recorder, w, h float64, leaf bool, c gfx.Color, sh *gfx.Shadow) {
	if ti.parts != nil {
		ti.drawParts(r, w, h, leaf, c, sh)
		return
	}
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
		if sh != nil {
			r.TextShadow(s, x, base, ti.font, c, *sh)
		} else {
			r.Text(s, x, base, ti.font, c)
		}
	}
}

// drawParts writes a title that holds inline pictures: each line is centered, left or right as a whole,
// the text on its baseline and the picture centered in the line.
func (ti *textInfo) drawParts(r *gfx.Recorder, w, h float64, leaf bool, c gfx.Color, sh *gfx.Shadow) {
	lines := ti.layout(w - 2*ti.padX)
	total := 0.0
	for _, ln := range lines {
		total += ln.h
	}
	y := ti.padY
	if leaf {
		y = (h - total) / 2
	}
	for _, ln := range lines {
		x := ti.padX
		if leaf {
			switch ti.align {
			case "left":
			case "right":
				x = w - ti.padX - ln.w
			default:
				x = (w - ln.w) / 2
			}
		}
		base := y + (ln.h-(ln.asc+ln.desc))/2 + ln.asc
		for _, part := range ln.parts {
			if part.img != nil {
				// The picture hangs from the cap height of the line, as go/render draws it, not centred
				// in the line box.
				if img := part.img.raster(ti.p); img != nil {
					r.DrawImage(img, x, base-ti.partFont(part).Size*capHeightRatio/2-part.img.h/2, part.img.w, part.img.h)
				}
				x += part.img.w
				continue
			}
			col := c
			if part.hasColor {
				col = part.color
			}
			if sh != nil {
				r.TextShadow(part.text, x, base, ti.partFont(part), col, *sh)
			} else {
				r.Text(part.text, x, base, ti.partFont(part), col)
			}
			x += ti.partWidth(part)
		}
		y += ln.h
	}
}

// MoveItem puts an absolute item of the screen at x, y in its coordinates and redraws the links attached
// to it, so the editor previews a drag before the model is written. Only an item that is placed by its
// own pos moves this way: one in a flow is placed by its parent and needs the layout again.
func (s *Screen) MoveItem(tx *gfx.Tx, pl *Placed, x, y float64) bool {
	if pl == nil || pl.r == nil || pl.Gfx == nil || pl.Parent == nil || pl.Parent.r == nil {
		return false
	}
	fr, parent := s.p.frames[pl.r], s.p.frames[pl.Parent.r]
	if fr == nil || parent == nil || parent.Dir != layout.Abs && !fr.Absolute {
		return false
	}
	dx, dy := x-pl.X, y-pl.Y
	if dx == 0 && dy == 0 {
		return true
	}
	fr.X += dx
	fr.Y += dy
	fr.RX += dx
	fr.RY += dy
	pl.X, pl.Y = x, y
	if deg := s.p.rotation(pl.r); deg != 0 {
		tx.SetTransform(pl.Gfx, turn(fr.RX, fr.RY, fr.RW, fr.RH, deg))
	} else {
		tx.Move(pl.Gfx, fr.RX, fr.RY)
	}
	b := &builder{p: s.p, tx: tx}
	for _, k := range pl.Parent.r.Kids {
		if k.Kind != RLink {
			continue
		}
		if ends, ok := s.p.ends[k]; !ok || ends[0] != pl.r && ends[1] != pl.r {
			continue
		}
		if ln := s.gfx[k]; ln != nil {
			if pic, ok := b.linkPicture(pl.Parent.r, k); ok {
				tx.SetPicture(ln, pic)
			}
		}
	}
	return true
}

// link draws the line between the two items in the parent's coordinates: straight from edge to edge, as
// the elbow the link asks for, or through the control point its pos gives.
func (b *builder) link(parent, lk *RNode, node *gfx.Node) {
	b.p.checkRoute(lk)
	pic, ok := b.linkPicture(parent, lk)
	if !ok {
		return
	}
	ln := b.node()
	b.made[lk] = ln
	b.tx.Add(node, ln)
	b.tx.SetPicture(ln, pic)
}

// linkPicture is the stroke and the arrow of lk where the boxes of its ends are now, so a link is redrawn
// without being built again when a dragged item moves.
func (b *builder) linkPicture(parent, lk *RNode) (*gfx.Picture, bool) {
	ends, ok := b.p.ends[lk]
	if !ok {
		return nil, false
	}
	a, c := b.p.frames[ends[0]], b.p.frames[ends[1]]
	pa := gfx.Point{X: a.RX + a.RW/2, Y: a.RY + a.RH/2}
	pc := gfx.Point{X: c.RX + c.RW/2, Y: c.RY + c.RH/2}
	if pa == pc {
		return nil, false
	}
	var (
		path *gfx.Path
		from gfx.Point
		to   gfx.Point
	)
	ctrl, hasCtrl := linkControl(b.p.prop(lk, "pos"), pa, pc)
	switch {
	case b.p.isElbow(lk):
		points := b.elbow(parent, lk)
		if len(points) < 2 {
			return nil, false
		}
		radius := float64(elbowRadius)
		if pr := b.p.prop(lk, "corner-radius"); pr != nil {
			if v, ok := pr.num(0); ok {
				radius = v
			}
		}
		path = elbowPath(points, radius)
		from, to = points[len(points)-2], points[len(points)-1]
	case hasCtrl:
		// The ends leave the boxes toward the control point, not toward each other.
		from, to = edgePoint(pa, a, ctrl), edgePoint(pc, c, ctrl)
		path = &gfx.Path{}
		path.MoveTo(from.X, from.Y)
		if b.p.isQuad(lk) {
			path.QuadTo(ctrl.X, ctrl.Y, to.X, to.Y)
		} else {
			path.LineTo(ctrl.X, ctrl.Y)
			path.LineTo(to.X, to.Y)
		}
		// The arrow keeps the direction of the last stretch: the control point into the target.
		from = ctrl
	default:
		from, to = edgePoint(pa, a, pc), edgePoint(pc, c, pa)
		path = &gfx.Path{}
		path.MoveTo(from.X, from.Y)
		path.LineTo(to.X, to.Y)
	}
	col, stroke := b.p.linkStyle(lk)
	r := gfx.NewRecorderWith(b.p.opts.Measurer)
	r.StrokePath(path, stroke, col)
	if pr := b.p.prop(lk, "display"); pr != nil {
		if s, _ := pr.text(0); s == "arrow" {
			r.FillPath(arrowHead(from, to, 6+stroke.Width), col)
		}
	}
	return r.Finish(), true
}

// linkControl is the control point a link's pos gives: the middle of its two boxes plus the offset. The
// offset is y up as the file writes it and the scene y down, so its vertical part is negated.
func linkControl(pr *Prop, pa, pc gfx.Point) (gfx.Point, bool) {
	if pr == nil {
		return gfx.Point{}, false
	}
	x, okx := pr.num(0)
	y, oky := pr.num(1)
	if !okx || !oky || x == 0 && y == 0 {
		return gfx.Point{}, false
	}
	return gfx.Point{X: (pa.X+pc.X)/2 + x, Y: (pa.Y+pc.Y)/2 - y}, true
}

// isQuad reports whether the link asks for the quad curve with layout quad.
func (p *prep) isQuad(lk *RNode) bool {
	pr := p.prop(lk, "layout")
	if pr == nil {
		return false
	}
	s, _ := pr.text(0)
	return s == "quad"
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
	grad               *gradient    // set by color linear ...; fill is then its first visible stop
	shadows            []gfx.Shadow // in file order, the last one on top
	lineW              float64
	dash               []float64 // line-style dashed or dotted; nil is a solid line
	dashPhase          float64
	radius             [4]float64 // top left, top right, bottom right, bottom left
	display            string     // rect when unset; no-fill, circle, stack, text or none
	layer              int        // draw order among the siblings
	clip               bool
	opacity            float64
}

// The layer of an item among its siblings as style.layer: background then hover, the zero layer (unset)
// between them, so an item that names no layer draws as it always did.
const (
	layerBackground = iota + 1
	layerHover
)

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
func (p *prep) style(rn *RNode, inCtx bool, up gfx.Color) style {
	st := style{lineW: 1, opacity: 1}
	fill, border := white, black
	if inCtx || fileImage(rn.prop("image")) {
		// A picture is the content of the box: the white default would show through a transparent PNG,
		// and the black frame around every photo is not what a box of content draws.
		fill, border = gfx.Color{}, gfx.Color{}
	}
	st.fill, st.grad = p.fill(rn, fill)
	st.shadows = p.shadows(rn)
	st.border = p.color(rn, "border-color", border)
	text := up // inherited from the parent, black at the root
	if st.fill.A > 0 {
		text = contrast(st.fill)
	}
	st.text = p.color(rn, "text-color", text)
	if v, ok := p.num(rn, "line-width"); ok {
		st.lineW = v
	}
	st.dash, st.dashPhase = p.dash(rn)
	if pr := p.prop(rn, "corner-radius"); pr != nil {
		st.radius = p.radii(pr)
	}
	if pr := p.prop(rn, "display"); pr != nil {
		st.display, _ = pr.text(0)
	}
	if pr := p.prop(rn, "layer"); pr != nil {
		switch s, _ := pr.text(0); s {
		case "background":
			st.layer = layerBackground
		case "hover":
			st.layer = layerHover
		}
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

func (p *prep) linkStyle(lk *RNode) (gfx.Color, gfx.StrokeStyle) {
	st := gfx.StrokeStyle{Width: 1, MiterLimit: 10}
	if v, ok := p.num(lk, "line-width"); ok {
		st.Width = v
	}
	st.Dash, st.DashPhase = p.dash(lk)
	return p.color(lk, "color", black), st
}

// dash reads line-style: the dashes the renderer of a classic diagram uses, so a border and a link look
// the same in both.
func (p *prep) dash(rn *RNode) ([]float64, float64) {
	pr := p.prop(rn, "line-style")
	if pr == nil {
		return nil, 0
	}
	s, _ := pr.text(0)
	switch s {
	case "dashed":
		return []float64{5, 5}, 5
	case "dotted":
		return []float64{1, 4}, 1
	case "solid":
		return nil, 0
	}
	p.t.warn(pr.Pos, "line-style takes solid, dashed or dotted, got %q", s)
	return nil, 0
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
