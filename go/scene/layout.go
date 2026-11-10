package scene

import (
	"math"
	"strings"
	"unicode"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/layout"
	"github.com/haiodo/tenniarb/go/markdown"
	"github.com/haiodo/tenniarb/go/tenn"
	"golang.org/x/text/unicode/norm"
)

// Options configure Build.
type Options struct {
	// Measurer measures text; gfx.EstimateText{} when nil. It is not gfx.DefaultMeasurer on purpose:
	// a backend replaces that one, and the layout must not depend on which backend ran first.
	Measurer gfx.TextMeasurer
	// FontName is the exact face of the text when an item sets no font-family, font-weight or font-style,
	// and the default family when it sets weight or style only; an item font-family replaces it.
	// "Helvetica" when empty.
	FontName string
	// FirstID is the id of the first scene node made; ids grow from there. 1 when zero.
	FirstID int
	// ScreenShadows keeps the offsets and the blur of a shadow in screen points while the view zooms, as
	// CGContext.setShadow does in the window of the classic editor; an export scales them with the content,
	// so it leaves this false.
	ScreenShadows bool
	// noExprWarn: expressions are a normal value here, not worth a warning (validate lays out without an
	// evaluator, so every ${...} of a screen would be reported).
	noExprWarn bool
	// Eval evaluates the code of a ${...} or $(...) argument, or a string with ${...} inside, and returns
	// the value as text (spec 6). nil: an expression is a warning and the property is ignored.
	Eval func(code string) (string, bool)
	// EvalVars is Eval with the parameters of the component instance in scope, by name and as text
	// (spec 5c): so ${title} and ${size + 12} read what the use sets. It wins over Eval; nil: only Eval
	// runs, and a parameter is an unknown name to it.
	EvalVars func(code string, vars map[string]string) (string, bool)
	// Classic gives the evaluated text of the arguments of prop on the node at, as go/render computed it
	// from the element model: the scope of a classic diagram (spec 6). It wins over Eval there. nil: a
	// classic diagram has no values, and its expressions are ignored.
	Classic func(at ClassicRef, prop string) ([]string, bool)
}

// Defaults of a text leaf, as in the Tenniarb app (font-size 18) plus a padding around the text.
const (
	defaultFontSize = 18.0
	textPadX        = 10.0
	textPadY        = 6.0
	// classicTextPad is the inset go/render gives a text leaf of a classic diagram when `padding` is not
	// written: the picture of an old file must not move when the canvas draws it through go/scene.
	classicTextPad = 4.0
	// capHeightRatio is the cap height of SF over the font size, as go/render places an inline picture.
	capHeightRatio = 1490.0 / 2048
	// classicRadius is the corner radius go/render gives a box of a classic diagram.
	classicRadius = 8.0
	// classicAscentRatio and classicDescentRatio are the parts of the font size go/render takes as the
	// ascent and the descent of a line (render/text.go, the SF metrics the Swift app measured with), so
	// a classic diagram keeps the line spacing and the baseline of its old picture.
	classicAscentRatio  = 1980.0 / 2048
	classicDescentRatio = 432.0 / 2048
)

// classicBorder is the border colour of a box of a classic diagram: the generic grey 0.2 of go/render,
// #424242.
var classicBorder = gfx.RGBA(66, 66, 66, 1)

// fitText is the longest prefix of s that fits avail, in whole characters; "" when not even one does.
func (p *prep) fitText(s string, f gfx.Font, avail float64) string {
	if avail <= 0 {
		return ""
	}
	m := p.opts.Measurer
	cut := ""
	for _, g := range tenn.Graphemes(s) {
		if m.MeasureText(cut+g, f).Width > avail+1e-9 {
			break
		}
		cut += g
	}
	return cut
}

// firstText is the first character of s, so a line always makes progress.
func (p *prep) firstText(s string) string {
	for _, g := range tenn.Graphemes(s) {
		return g
	}
	return s
}

// lineBox is the ascent, the descent and the height of one line of f: a scene takes them from the
// measurer, a classic diagram from the ratios and the rounding of go/render (floor(ascent+descent)+1),
// so an old file keeps the line spacing, the box and the baseline of its Swift picture.
func (p *prep) lineBox(f gfx.Font) (asc, desc, h float64) {
	if p != nil && p.classic {
		asc, desc = f.Size*classicAscentRatio, f.Size*classicDescentRatio
		return asc, desc, math.Floor(asc+desc) + 1
	}
	m := p.opts.Measurer.MeasureText("Hg", f)
	return m.Ascent, m.Descent, math.Round(m.Ascent + m.Descent)
}

// prep turns resolved nodes into a layout tree and keeps what the drawing needs.
type prep struct {
	t      *Tree
	opts   Options
	frames map[*RNode]*layout.Node
	texts  map[*RNode]*textInfo
	bodies map[*RNode]*bodyText
	bodyAt map[*RNode]*layout.Node // a body laid out as a child of its item
	styles map[*RNode]style
	ends   map[*RNode][2]*RNode // link -> its two items
	// classic: the element being laid out is a classic diagram (sceneRoot is false).
	classic bool
	icons   map[*RNode]*icon
	images  map[*RNode]*itemImage            // an item or a root that draws a picture
	inlines map[*RNode]map[string]*itemImage // the classic images its text names with @(name)
	anim    map[*RNode]*Animation            // the animation a node plays (spec 13)
	eval    evalFunc                         // Options.Eval or EvalVars; nil: no expression runs
	evals   map[evalKey]*Prop                // a property with an expression -> its evaluated copy, nil if it failed
	warned  map[string]bool                  // colours of the markdown markup already reported
	// classicAt names every node of a classic diagram in the terms of the element model, for
	// Options.Classic; empty in a scene.
	classicAt map[*RNode]ClassicRef
}

// evalKey is one property of one node: the Prop of a scene is shared between instances, so its
// evaluation is not.
type evalKey struct {
	rn *RNode
	pr *Prop
}

type textInfo struct {
	font       gfx.Font
	padX, padY float64
	lineH      float64
	align      string // layout of a leaf's lines: "left", "middle" (the default) or "right"
	asc, desc  float64
	text       string
	m          gfx.TextMeasurer
	parts      []titlePart // set when the text names an inline image
	p          *prep
}

// titlePart is a span of a title: text in the item font or in a size of its own, or an inline picture of
// a classic diagram. hasColor marks a text run the !(...) markup coloured; without it the run takes the
// text colour of the item, which is only known when the title is drawn.
type titlePart struct {
	text     string
	font     gfx.Font // zero: the font of the title
	color    gfx.Color
	hasColor bool
	img      *textImage
}

// titleLine is one laid out line of a title; h grows above lineH for a larger run or an inline picture,
// and asc/desc follow the largest run so the baseline stays on it.
type titleLine struct {
	parts     []titlePart
	w, h      float64
	asc, desc float64
}

func (l titleLine) text() string {
	var s strings.Builder
	for _, part := range l.parts {
		s.WriteString(part.text)
	}
	return s.String()
}

func newPrep(t *Tree, o Options) *prep {
	if o.Measurer == nil {
		o.Measurer = gfx.EstimateText{}
	}
	if o.FontName == "" {
		o.FontName = "Helvetica"
	}
	if o.FirstID == 0 {
		o.FirstID = 1
	}
	return &prep{t: t, opts: o, frames: map[*RNode]*layout.Node{}, texts: map[*RNode]*textInfo{},
		bodies: map[*RNode]*bodyText{}, bodyAt: map[*RNode]*layout.Node{}, styles: map[*RNode]style{}, ends: map[*RNode][2]*RNode{}, icons: map[*RNode]*icon{},
		images: map[*RNode]*itemImage{}, inlines: map[*RNode]map[string]*itemImage{},
		anim: map[*RNode]*Animation{}, eval: evaluator(o), evals: map[evalKey]*Prop{}}
}

// root is the layout of an element: its content at their own positions.
func (p *prep) root(rn *RNode) *layout.Node {
	p.classic = !sceneRoot(rn)
	p.animations(rn, nil) // before the layout: the first frame of an animation is part of the picture
	n := &layout.Node{Dir: layout.Abs, Tag: rn}
	p.frames[rn] = n
	// The root sizes the screen: a number is its box, and a picture with no number written gives its own
	// size, so a screen can be one image.
	im := p.loadImage(rn)
	if im != nil {
		p.images[rn] = im
	}
	if pr := p.prop(rn, "width"); pr != nil {
		if v, ok := pr.num(0); ok {
			n.W = layout.Fixed(v)
		}
	} else if im != nil {
		n.W = layout.Fixed(float64(im.w))
	}
	if pr := p.prop(rn, "height"); pr != nil {
		if v, ok := pr.num(0); ok {
			n.H = layout.Fixed(v)
		}
	} else if im != nil {
		n.H = layout.Fixed(float64(im.h))
	}
	for _, k := range rn.Kids {
		if k.Kind == RItem && !p.hidden(k) {
			n.Kids = append(n.Kids, p.item(k, false, black))
		}
	}
	p.links(rn)
	return n
}

// sceneRoot is model.SceneElement on the expanded tree: an instance, an item with child nodes or
// auto-layout. Other elements are classic diagrams, which go/render draws with their expressions.
// holdsItems reports whether the node has an item kid, so it draws no box of its own unless it asks.
func holdsItems(rn *RNode) bool {
	for _, k := range rn.Kids {
		if k.Kind == RItem {
			return true
		}
	}
	return false
}

func sceneRoot(rn *RNode) bool {
	for _, k := range rn.Kids {
		if k.Kind == RItem && (k.Inst || len(k.Kids) > 0 || k.prop("auto-layout") != nil) {
			return true
		}
	}
	return false
}

// invalid reports a value the scene cannot use: an error in a scene, and only a warning in a classic
// diagram, where go/render drew what it understood and ignored the rest of the file.
func (p *prep) invalid(pos Pos, format string, args ...any) {
	if p.classic {
		p.t.warn(pos, format, args...)
		return
	}
	p.t.fail(pos, format, args...)
}

// prop is rn.prop with the expressions of the arguments evaluated (spec 6), in the scope of the instance
// of the node (spec 5c). Without an evaluator an expression is a warning and the property is ignored; in a
// classic diagram it is normal and go/render evaluates it.
func (p *prep) prop(rn *RNode, name string) *Prop {
	pr := rn.prop(name)
	if pr == nil || !hasExpr(pr) {
		return pr
	}
	if !p.canEval() {
		if !p.classic && !p.opts.noExprWarn {
			p.t.warn(pr.Pos, "%s: expressions are not evaluated", name)
		}
		return nil
	}
	return p.evalProp(rn, pr)
}

// evalProp is pr evaluated in the scope of rn, once per build; nil when an expression failed, which leaves
// the property unset.
func (p *prep) evalProp(rn *RNode, pr *Prop) *Prop {
	k := evalKey{rn, pr}
	if out, ok := p.evals[k]; ok {
		return out
	}
	out := p.evaluated(rn, pr)
	p.evals[k] = out
	return out
}

// evalTree evaluates the expressions of every property of the screen before the styles are applied: the
// arguments of "use-style ${kind}" have to be values by then, or the style cannot be chosen (spec 5c). Only
// the nodes change, never the shared Prop: a node of an instance is its own, its Prop is not.
func (p *prep) evalTree(rn *RNode) {
	for i, pr := range rn.Props {
		if !hasExpr(pr) {
			continue
		}
		if out := p.evalProp(rn, pr); out != nil {
			rn.Props[i] = out
		}
	}
	for _, k := range rn.Kids {
		p.evalTree(k)
	}
}

// hasExpr reports whether an argument is an expression, or a string with one inside.
func hasExpr(pr *Prop) bool {
	if pr.Val != nil { // already evaluated
		return false
	}
	for _, a := range pr.Args {
		if a.Kind == tenn.KindExpression || a.Kind == tenn.KindExpressionBlock {
			return true
		}
		if a.Kind == tenn.KindStringLit {
			if s, _ := a.GetIdentText(); strings.Contains(s, "${") {
				return true
			}
		}
	}
	return false
}

// evaluated is a copy of pr with every argument evaluated, the plain ones copied as they are; nil when an
// expression failed. The source property is shared between instances, so it is never changed.
func (p *prep) evaluated(rn *RNode, pr *Prop) *Prop {
	out := &Prop{Name: pr.Name, Args: pr.Args, Block: pr.Block, Pos: pr.Pos, Range: pr.Range,
		Val: make([]string, len(pr.Args))}
	if vals, ok := p.modelVals(rn, pr); ok {
		copy(out.Val, vals)
		return out
	}
	for i, a := range pr.Args {
		v, ok := evalArg(p.eval, a, rn.Vars)
		if !ok {
			code, _ := a.GetIdentText()
			p.t.warn(pr.Pos, "%s: the expression %q is not evaluated", pr.Name, code)
			return nil
		}
		out.Val[i] = v
	}
	return out
}

// A link end is the n-th sibling with that NFC name (source-index/target-index, default 0),
// the same rule as PrepareItemRefs in go/model.
// linkBoxes is the box of every link, by the node that writes it: its two ends together.
func (p *prep) linkBoxes() map[*Node]gfx.Rect {
	out := make(map[*Node]gfx.Rect, len(p.ends))
	for k, ends := range p.ends {
		if k.Src == nil || ends[0] == nil || ends[1] == nil {
			continue
		}
		a, b := p.frames[ends[0]], p.frames[ends[1]]
		if a == nil || b == nil {
			continue
		}
		out[k.Src] = gfx.R(a.RX, a.RY, a.RW, a.RH).Union(gfx.R(b.RX, b.RY, b.RW, b.RH))
	}
	return out
}

func (p *prep) links(parent *RNode) {
	byName := map[string][]*RNode{}
	for _, k := range parent.Kids {
		if k.Kind == RItem {
			n := norm.NFC.String(k.Name)
			byName[n] = append(byName[n], k)
		}
	}
	end := func(lk *RNode, name, idxProp string) *RNode {
		idx := 0
		if pr := p.prop(lk, idxProp); pr != nil {
			if v, ok := pr.num(0); ok {
				idx = int(v)
			}
		}
		list := byName[norm.NFC.String(name)]
		if idx < 0 || idx >= len(list) {
			return nil
		}
		return list[idx]
	}
	for _, k := range parent.Kids {
		if k.Kind != RLink {
			continue
		}
		a, b := end(k, k.From, "source-index"), end(k, k.To, "target-index")
		if a == nil || b == nil {
			p.invalid(k.Pos, "link %q %q: both ends must be items of the same parent", k.From, k.To)
			continue
		}
		if p.frames[a] == nil || p.frames[b] == nil { // an end is hidden: the link goes with it
			continue
		}
		p.ends[k] = [2]*RNode{a, b}
		_, _ = p.linkStyle(k) // a bad line-style or colour is reported here, not while drawing
	}
}

// item lays out one node; up is the text colour of its parent, which it takes when it sets none and has
// no fill of its own (spec 5b).
func (p *prep) item(rn *RNode, parentAuto bool, up gfx.Color) *layout.Node {
	n := &layout.Node{Tag: rn}
	p.frames[rn] = n
	auto := p.prop(rn, "auto-layout") != nil
	inCtx := auto || parentAuto
	if pr := p.prop(rn, "pos"); pr != nil {
		// Decided: Y is up everywhere in .tenn (scene.ts: bounds.y = e.y - height), pos is the top left
		// corner, so the screen y is -pos.y; the same for children and absolute children.
		n.X, _ = pr.num(0)
		y, _ := pr.num(1)
		n.Y = 0 - y
	}
	n.W, n.H = p.size(rn, "width", inCtx, parentAuto), p.size(rn, "height", inCtx, parentAuto)
	if im := p.loadImage(rn); im != nil {
		p.images[rn] = im
		// A picture with no size written takes its own pixels as the size.
		iw, ih := float64(im.w), float64(im.h)
		if p.prop(rn, "width") == nil {
			n.W = layout.Fixed(iw)
		}
		if p.prop(rn, "height") == nil {
			n.H = layout.Fixed(ih)
		}
	}
	if inCtx {
		p.limits(rn, n)
		n.Absolute = p.flag(rn, "absolute")
	}
	n.Pad = p.padding(rn) // the text inset of a leaf, the children inset of a container
	if auto {
		p.container(rn, n)
	} else {
		n.Dir = layout.Abs
	}
	// The root of an instance that holds items is a container for what the element draws, not a box of
	// its own: a white fill there would cover the screen behind it.
	p.styles[rn] = p.style(rn, inCtx || (rn.Inst && holdsItems(rn)), up)
	// A body is the content of a leaf; an item that holds items shows them instead and is a backdrop.
	if !holdsItems(rn) {
		p.bodies[rn] = p.bodyInfo(rn, p.styles[rn], inCtx)
	}
	for _, k := range rn.Kids {
		if k.Kind == RItem && !p.hidden(k) {
			n.Kids = append(n.Kids, p.item(k, auto, p.styles[rn].text))
		}
	}
	// The body of an auto-layout item is a text block of its own after the children, so the flow
	// puts it below them and the item grows with it; a leaf paints it under its title instead.
	if body := p.bodies[rn]; body != nil && auto && len(n.Kids) > 0 {
		bn := &layout.Node{W: layout.Fill}
		bn.LeafW = func() float64 { return body.natural() }
		bn.LeafH = func(w float64) float64 { return body.height(w) }
		body.padX, body.padY = 0, 0
		n.Kids = append(n.Kids, bn)
		p.bodyAt[rn] = bn
	}
	if len(n.Kids) == 0 {
		p.leaf(rn, n, inCtx)
	} else {
		if !rn.Inst { // the name of an instance is an alias, not its content
			p.texts[rn] = p.textOf(rn, inCtx)
		}
		p.links(rn)
	}
	return n
}

// warnOutside reports an absolutely placed item that lies wholly outside its clipping parent: it is
// laid out but never drawn. Items of a flow that overflow are cut by design and not reported.
func (p *prep) warnOutside(rn *RNode) {
	ln := p.frames[rn]
	clip := p.styles[rn].clip
	for _, k := range rn.Kids {
		kn := p.frames[k]
		if k.Kind != RItem || kn == nil {
			continue
		}
		if clip && (ln.Dir == layout.Abs || kn.Absolute) && kn.RW > 0 && kn.RH > 0 &&
			(kn.RX >= ln.RW || kn.RY >= ln.RH || kn.RX+kn.RW <= 0 || kn.RY+kn.RH <= 0) {
			p.t.warn(k.Pos, "item %q is at %g, %g (size %g x %g) outside its clipping parent %q (%g x %g) and is not drawn",
				k.Name, kn.RX, kn.RY, kn.RW, kn.RH, rn.Name, ln.RW, ln.RH)
		}
		p.warnOutside(k)
	}
}

func (p *prep) limits(rn *RNode, n *layout.Node) {
	n.MinW, _ = p.num(rn, "min-width")
	n.MaxW, _ = p.num(rn, "max-width")
	n.MinH, _ = p.num(rn, "min-height")
	n.MaxH, _ = p.num(rn, "max-height")
}

// container reads the properties that only an item with auto-layout has.
func (p *prep) container(rn *RNode, n *layout.Node) {
	if v, ok := p.word(rn, "auto-layout", "row", "column", "abs"); ok {
		n.Dir = map[string]layout.Dir{"row": layout.Row, "column": layout.Column, "abs": layout.Abs}[v]
	}
	n.Gap, _ = p.num(rn, "gap")
	n.LineGap, _ = p.num(rn, "line-gap")
	n.Wrap = p.flag(rn, "wrap")
	aligns := map[string]layout.Align{"start": layout.Start, "center": layout.Center, "end": layout.End, "space-between": layout.SpaceBetween}
	if v, ok := p.word(rn, "align", "start", "center", "end"); ok {
		n.Align = aligns[v]
	}
	if v, ok := p.word(rn, "justify", "start", "center", "end", "space-between"); ok {
		n.Justify = aligns[v]
	}
}

func (p *prep) padding(rn *RNode) [4]float64 {
	pr := p.prop(rn, "padding")
	if pr == nil {
		return [4]float64{}
	}
	var v []float64
	for i := range pr.Args {
		f, ok := pr.num(i)
		if !ok {
			p.invalid(pr.Pos, "padding takes numbers")
			return [4]float64{}
		}
		v = append(v, f)
	}
	switch len(v) {
	case 1:
		return [4]float64{v[0], v[0], v[0], v[0]}
	case 2: // vertical, horizontal
		return [4]float64{v[0], v[1], v[0], v[1]}
	case 4:
		return [4]float64{v[0], v[1], v[2], v[3]}
	}
	p.invalid(pr.Pos, "padding takes 1, 2 or 4 numbers, got %d", len(v))
	return [4]float64{}
}

func (p *prep) num(rn *RNode, name string) (float64, bool) {
	pr := p.prop(rn, name)
	if pr == nil {
		return 0, false
	}
	v, ok := pr.num(0)
	if !ok {
		p.invalid(pr.Pos, "%s takes a number", name)
	}
	return v, ok
}

func (p *prep) flag(rn *RNode, name string) bool {
	pr := p.prop(rn, name)
	if pr == nil {
		return false
	}
	switch s, _ := pr.text(0); s {
	case "yes", "true":
		return true
	case "no", "false":
		return false
	}
	p.invalid(pr.Pos, "%s takes yes or no", name)
	return false
}

func (p *prep) word(rn *RNode, name string, allowed ...string) (string, bool) {
	pr := p.prop(rn, name)
	if pr == nil {
		return "", false
	}
	s, _ := pr.text(0)
	for _, a := range allowed {
		if s == a {
			return s, true
		}
	}
	if s == "space-around" {
		p.t.warn(pr.Pos, "%s space-around is not supported by the layout", name)
	} else if name != "auto-layout" || s != "" {
		p.invalid(pr.Pos, "%s takes one of %s, got %q", name, strings.Join(allowed, ", "), s)
	}
	return "", false
}

// size reads width or height: a number, or hug and fill where an auto-layout is at work.
func (p *prep) size(rn *RNode, name string, inCtx, parentAuto bool) layout.Size {
	pr := p.prop(rn, name)
	if pr == nil || len(pr.Args) == 0 {
		return layout.Size{}
	}
	if v, ok := pr.num(0); ok {
		// go/render takes a size below one as not written, so an item with a zero measure falls back
		// to its text: an old file that computes 0 draws a dot, not an invisible box.
		if p.classic && v < 1 {
			return layout.Size{}
		}
		return layout.Fixed(v)
	}
	// text, not the token: an expression may give the word as well as a number (spec 6).
	s, _ := pr.text(0)
	switch {
	case s == "hug" || s == "fill":
		return p.sizeWord(pr, s, inCtx, parentAuto)
	case strings.HasSuffix(s, "%"):
		p.t.warn(pr.Pos, "%s: percentages are not supported, the size is hug", name)
	default:
		p.invalid(pr.Pos, "%s takes a number, hug or fill, got %q", name, s)
	}
	return layout.Size{}
}

func (p *prep) sizeWord(pr *Prop, word string, inCtx, parentAuto bool) layout.Size {
	if !inCtx {
		p.invalid(pr.Pos, "%s %s needs auto-layout on the item or on its parent", pr.Name, word)
		return layout.Size{}
	}
	if len(pr.Args) > 1 {
		p.t.warn(pr.Pos, "%s fill with a fallback value is not supported", pr.Name)
	}
	if word == "fill" {
		// There is nothing to fill in a plain parent: the item takes its own size, as in pen.
		if !parentAuto {
			p.t.warn(pr.Pos, "%s fill needs auto-layout on the parent, the size is hug", pr.Name)
			return layout.Size{}
		}
		return layout.Fill
	}
	return layout.Size{}
}

// leaf gives an item without children the size of its title and body.
func (p *prep) leaf(rn *RNode, n *layout.Node, inCtx bool) {
	p.icon(rn, n)
	// An icon draws no name as text, as in a layout context.
	ti := p.textOf(rn, inCtx || p.icons[rn] != nil)
	body := p.bodies[rn]
	if p.icons[rn] != nil {
		body = nil // an icon has no text body: the path is the content
	}
	padX, padY := textPadX, textPadY
	if p.classic {
		padX, padY = classicTextPad, classicTextPad
	}
	if inCtx {
		padX, padY = 0, 0
	}
	// padding insets the text of an item whether or not it has auto-layout: a leaf with a written size
	// reads it as the space between its border and the text.
	if pr := p.prop(rn, "padding"); pr != nil {
		padX, padY = n.Pad[3], n.Pad[0]
	}
	if ti == nil && body == nil {
		// go/render keeps the inset around an item with no text: a dot of eight points, which old
		// diagrams use as markers and anchors.
		if p.classic && !inCtx {
			n.LeafW = func() float64 { return 2 * padX }
			n.LeafH = func(float64) float64 { return 2 * padY }
		}
		return
	}
	if ti != nil {
		p.texts[rn] = ti
	}
	if ti != nil {
		ti.padX, ti.padY = padX, padY
	}
	if body != nil {
		body.padX, body.padY = padX, padY
	}
	n.LeafW = func() float64 {
		w := 0.0
		if ti != nil {
			w = math.Max(w, ti.natural())
		}
		if body != nil {
			w = math.Max(w, body.natural())
		}
		if p.classic {
			// go/render kept the fraction of a measured line; only a scene rounds its leaf to whole points.
			return w + 2*padX
		}
		return math.Ceil(w) + 2*padX
	}
	n.LeafH = func(w float64) float64 {
		inner := w - 2*padX
		h := 0.0
		if ti != nil {
			h += ti.height(inner)
		}
		if body != nil {
			h += body.height(inner)
		}
		return h + 2*padY
	}
}

// quotedTitle is the title as the file should write it: every argument in one quoted text.
func quotedTitle(pr *Prop) string {
	words := make([]string, 0, len(pr.Args))
	for i := range pr.Args {
		if t, ok := pr.text(i); ok && t != "" {
			words = append(words, t)
		}
	}
	return `title "` + strings.Join(words, " ") + `"`
}

// evalText is the text of a title with each ${...} replaced by its value: the element model hands the
// whole title of a classic item, a scene asks the engine about each piece.
func (p *prep) evalText(rn *RNode, pr *Prop, text string) string {
	if !strings.Contains(text, "${") {
		return text
	}
	if vals, ok := p.modelVals(rn, pr); ok && len(vals) > 0 {
		return vals[0]
	}
	if p.eval == nil {
		return text
	}
	var b strings.Builder
	for i := 0; i < len(text); {
		if strings.HasPrefix(text[i:], "${") {
			if j := strings.IndexByte(text[i+2:], '}'); j >= 0 {
				if v, ok := p.eval(text[i+2:i+2+j], rn.Vars); ok {
					b.WriteString(v)
					i += len("${") + j + 1
					continue
				}
			}
		}
		b.WriteByte(text[i])
		i++
	}
	return b.String()
}

func (p *prep) textOf(rn *RNode, inCtx bool) *textInfo {
	// The name of a leaf is its text, as in a classic diagram, and a title overrides it. A node that holds
	// items is a box for them: its name is only a path segment, the text is a title if one is written.
	title := rn.Name
	if inCtx || holdsItems(rn) {
		title = ""
	}
	if pr := p.prop(rn, "title"); pr != nil {
		title, _ = pr.text(0)
		title = p.evalText(rn, pr, title)
		// A bare text with spaces is several arguments, and only the first would be drawn: name the
		// mistake instead of dropping the words. A classic diagram drops them, as go/render always has.
		if len(pr.Args) > 1 && !p.classic {
			p.t.fail(pr.Pos, "title takes one value: quote the words, %s (tenniarb check -fix does it)", quotedTitle(pr))
		}
	}
	if m := p.styles[rn].marker; m != "" {
		// A marker is drawn in front of the text, with a space, as go/render does it.
		title = m + " " + title
	}
	if title == "" {
		return nil
	}
	if p.classic {
		// go/render reads a text `\n` as a newline, in a title and in the name a leaf draws. The block of a
		// title keeps the newlines and the indentation of its edges when the model hands it over, and
		// go/render drew neither: they became an empty line above the text and one below it.
		title = strings.Trim(strings.ReplaceAll(title, `\n`, "\n"), " \t\r\n")
	}
	size := defaultFontSize
	if fs := p.prop(rn, "font-size"); fs != nil {
		if v, ok := fs.num(0); ok && v > 0 {
			size = v
		} else if ok {
			p.t.warn(fs.Pos, "font-size: %v is not above 0, the default is used", v)
		}
	}
	padX, padY := textPadX, textPadY
	if p.classic {
		padX, padY = classicTextPad, classicTextPad
	}
	if inCtx {
		padX, padY = 0, 0
	}
	ti := &textInfo{font: p.fontOf(rn, size), padX: padX, padY: padY, text: title, m: p.opts.Measurer, p: p}
	if p.classic {
		ti.parts = p.titleMarkupParts(rn, ti)
		if ti.parts == nil {
			ti.parts, _ = p.splitTitleImages(title, p.inlineImages(rn))
		}
	}
	ti.asc, ti.desc, ti.lineH = p.lineBox(ti.font)
	if lh := p.prop(rn, "line-height"); lh != nil {
		if v, ok := lh.num(0); ok && v > 0 {
			ti.lineH = v
		} else {
			p.t.warn(lh.Pos, "line-height: not a number above 0, ignored")
		}
	}
	if pr := p.prop(rn, "layout"); pr != nil {
		// Other words are the layout algorithms of diagram elements.
		if w, _ := pr.text(0); w == "left" || w == "right" {
			ti.align = w
		}
	}
	if ls := p.prop(rn, "line-spacing"); ls != nil {
		v, _ := ls.num(0)
		ti.lineH += v
	}
	return ti
}

// titleMarkupParts cuts a classic title at the !(colour), &(size) and @(picture) of the markdown markup;
// nil when the title writes none of them, so its text stays as written. The colours and sizes are the ones
// go/render gives the runs of a title, and everything the markup does not cover keeps its characters.
func (p *prep) titleMarkupParts(rn *RNode, ti *textInfo) []titlePart {
	title := ti.text
	if parts := p.titleEmphasisParts(ti); parts != nil {
		return parts
	}
	if !strings.Contains(title, "!(") && !strings.Contains(title, "&(") {
		return nil
	}
	imgs := p.inlineImages(rn)
	prov := inlineProvider(imgs)
	color, hasColor := gfx.Color{}, false
	font := ti.font
	parts := []titlePart{}
	var plain strings.Builder
	flush := func() {
		if plain.Len() == 0 {
			return
		}
		parts = append(parts, titlePart{text: plain.String(), font: font, color: color, hasColor: hasColor})
		plain.Reset()
	}
	for i := 0; i < len(title); {
		if title[i] == '\\' && i+1 < len(title) && strings.ContainsRune("@$*_#<~!", rune(title[i+1])) {
			plain.WriteByte(title[i+1])
			i += 2
			continue
		}
		mark, arg, next, ok := parenAt(title, i)
		if !ok {
			plain.WriteByte(title[i])
			i++
			continue
		}
		switch mark {
		case "!":
			flush()
			before, text, inline := strings.Cut(arg, "|")
			if inline {
				c, ok := p.markupColor(rn.Pos, before)
				parts = append(parts, titlePart{text: text, font: font, color: c, hasColor: ok})
			} else if strings.TrimSpace(arg) == "" {
				color, hasColor = gfx.Color{}, false
			} else if c, ok := p.markupColor(rn.Pos, arg); ok {
				color, hasColor = c, true
			} else {
				color, hasColor = gfx.Color{}, false
			}
		case "&":
			flush()
			size, text, inline := strings.Cut(arg, "|")
			if inline {
				if v, ok := markupSize(size); ok {
					parts = append(parts, titlePart{text: text, font: sizedFont(font, v), color: color, hasColor: hasColor})
				}
			} else if v, ok := markupSize(arg); ok {
				font = sizedFont(ti.font, v)
			} else {
				font = ti.font
			}
		case "@":
			if img := resolveInline(prov, imgs, arg); img != nil {
				flush()
				parts = append(parts, titlePart{img: img, font: font})
			} else {
				plain.WriteString(title[i:next])
			}
		}
		i = next
	}
	flush()
	return parts
}

// titleEmphasisParts is the runs of a title with the emphasis go/render reads in it: **bold**, *italic*,
// `code`. nil when the title writes none, so a plain title keeps its characters.
func (p *prep) titleEmphasisParts(ti *textInfo) []titlePart {
	text := ti.text
	if !strings.ContainsAny(text, "*_`") {
		return nil
	}
	// The raw text, as go/render lexes it: **bold** is bold and *italic* is italic, which the normalised
	// form of a body would swap.
	toks := markdown.GetTokens(text)
	if len(toks) == 0 {
		return nil
	}
	styled := func(weight int, italic bool) gfx.Font {
		f := ti.font
		if f.Name != "" {
			fam, _, _ := strings.Cut(f.Name, ",")
			f.Family, f.Name = strings.TrimSpace(fam), ""
		}
		if weight != 0 {
			f.Weight = weight
		}
		f.Italic = italic
		return f
	}
	parts := []titlePart{}
	plain := ""
	flush := func() {
		if plain == "" {
			return
		}
		parts = append(parts, titlePart{text: plain, font: ti.font})
		plain = ""
	}
	marked := false
	for _, t := range toks {
		switch t.Type {
		case markdown.TokBold:
			flush()
			parts = append(parts, titlePart{text: t.Literal, font: styled(700, false)})
			marked = true
		case markdown.TokItalic:
			flush()
			parts = append(parts, titlePart{text: t.Literal, font: styled(0, true)})
			marked = true
		case markdown.TokCode:
			flush()
			f := ti.font
			f.Family, f.Name = bodyMonoFace, ""
			parts = append(parts, titlePart{text: t.Literal, font: f})
			marked = true
		case markdown.TokEOF:
		default:
			plain += t.Literal
		}
	}
	flush()
	if !marked {
		return nil
	}
	return parts
}

// parenAt reads a marker of the markup at i: "!", "&" or "@" followed by "(...)", the content balanced
// the way the lexer reads it, and the index after the closing ")".
func parenAt(s string, i int) (mark, arg string, next int, ok bool) {
	if i+1 >= len(s) || s[i+1] != '(' || s[i] != '!' && s[i] != '&' && s[i] != '@' {
		return "", "", i, false
	}
	depth := 1
	for j := i + 2; j < len(s); j++ {
		switch s[j] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return s[i : i+1], s[i+2 : j], j + 1, true
			}
		}
	}
	return "", "", i, false
}

// partFont is the font of a part, the title font for a part that names none.
func (ti *textInfo) partFont(part titlePart) gfx.Font {
	if part.font.Size == 0 {
		return ti.font
	}
	return part.font
}

// partWidth is the advance of a part, its picture or its text in its own font.
func (ti *textInfo) partWidth(part titlePart) float64 {
	if part.img != nil {
		return part.img.w
	}
	return ti.m.MeasureText(part.text, ti.partFont(part)).Width
}

// fontOf reads font-family, font-weight, font-style and letter-spacing. gfx.Font.Name wins over Family,
// so Name is set only when no property needs the family lookup: that keeps the old font and SVG output.
func (p *prep) fontOf(rn *RNode, size float64) gfx.Font {
	f := gfx.Font{Size: size}
	if ls := p.prop(rn, "letter-spacing"); ls != nil {
		if v, ok := ls.num(0); ok {
			f.Spacing = v
		} else {
			p.t.warn(ls.Pos, "letter-spacing: not a number, ignored")
		}
	}
	if fam := p.prop(rn, "font-family"); fam != nil {
		f.Family, _ = fam.text(0)
	}
	if pr := p.prop(rn, "font-weight"); pr != nil {
		f.Weight = p.weight(pr)
	}
	if pr := p.prop(rn, "font-style"); pr != nil {
		s, _ := pr.text(0)
		switch s {
		case "italic", "oblique":
			f.Italic = true
		case "normal":
		default:
			p.t.warn(pr.Pos, "font-style: %q is not italic or normal, ignored", s)
		}
	}
	if f.Family == "" && f.Weight == 0 && !f.Italic {
		f.Name = p.opts.FontName
	} else if f.Family == "" {
		// FontName may be a CSS list for SVG (tenn-render); a family is one name.
		first, _, _ := strings.Cut(p.opts.FontName, ",")
		f.Family = strings.TrimSpace(first)
	}
	return f
}

// weight reads font-weight as a CSS number 100..900 or the words normal and bold; 0 when it is none of them.
func (p *prep) weight(pr *Prop) int {
	switch s, _ := pr.text(0); s {
	case "normal":
		return 400
	case "bold":
		return 700
	}
	v, ok := pr.num(0)
	if !ok || v < 100 || v > 900 {
		p.t.warn(pr.Pos, "font-weight: takes 100..900, normal or bold, ignored")
		return 0
	}
	return int(v)
}

func (ti *textInfo) width(s string) float64 { return ti.m.MeasureText(s, ti.font).Width }

// natural is the width of the widest line when only "\n" breaks lines.
func (ti *textInfo) natural() float64 {
	if ti.parts != nil {
		w := 0.0
		for _, ln := range ti.layout(math.Inf(1)) {
			w = math.Max(w, ln.w)
		}
		return w
	}
	w := 0.0
	for _, ln := range strings.Split(ti.text, "\n") {
		w = math.Max(w, ti.width(ln))
	}
	return w
}

// height is the height of the text at width w; an inline picture is taller than its line of text.
func (ti *textInfo) height(w float64) float64 {
	if ti.parts == nil {
		return float64(len(ti.wrap(w))) * ti.lineH
	}
	h := 0.0
	for _, ln := range ti.layout(w) {
		h += ln.h
	}
	return h
}

// layout breaks the title at width w, inline pictures counting as one unbreakable box.
func (ti *textInfo) layout(w float64) []titleLine {
	var lines []titleLine
	line := titleLine{h: ti.lineH, asc: ti.asc, desc: ti.desc}
	started := false
	flush := func() {
		lines = append(lines, line)
		line = titleLine{h: ti.lineH, asc: ti.asc, desc: ti.desc}
		started = false
	}
	add := func(part titlePart) {
		line.parts = append(line.parts, part)
		if part.img != nil {
			line.w += part.img.w
			line.h = math.Max(line.h, part.img.h)
			started = true
			return
		}
		f := ti.partFont(part)
		line.w += ti.m.MeasureText(part.text, f).Width
		if f != ti.font {
			asc, desc, h := ti.p.lineBox(f)
			line.asc = math.Max(line.asc, asc)
			line.desc = math.Max(line.desc, desc)
			line.h = math.Max(line.h, h)
		}
		started = started || part.text != ""
	}
	for _, part := range ti.parts {
		if part.img != nil {
			if started && w > 0 && line.w+part.img.w > w+1e-9 {
				flush()
			}
			add(part)
			continue
		}
		partFont := ti.partFont(part)
		for pi, para := range strings.Split(part.text, "\n") {
			if pi > 0 {
				flush()
			}
			for i, word := range strings.Split(para, " ") {
				for j, piece := range hyphenPieces(word) {
					text := piece
					if j == 0 && i > 0 {
						text = " " + piece
					}
					if text == "" || !started && strings.TrimSpace(text) == "" {
						continue
					}
					if started && strings.TrimSpace(text) != "" && w > 0 && line.w+ti.m.MeasureText(text, partFont).Width > w+1e-9 {
						flush()
						text = piece
					}
					// A word wider than the line is broken by characters in a classic diagram, as go/render
					// drew the title of an item: the box grows down instead of running over its width.
					if ti.p != nil && ti.p.classic && w > 0 && strings.TrimSpace(text) != "" {
						for text != "" {
							if ti.m.MeasureText(text, partFont).Width <= w-line.w+1e-9 {
								add(titlePart{text: text, font: part.font, color: part.color, hasColor: part.hasColor})
								break
							}
							cut := ti.p.fitText(text, partFont, w-line.w)
							if cut == "" {
								if !started {
									cut = ti.p.firstText(text)
								} else {
									flush()
									continue
								}
							}
							add(titlePart{text: cut, font: part.font, color: part.color, hasColor: part.hasColor})
							text = text[len(cut):]
							flush()
						}
						continue
					}
					if text != "" {
						add(titlePart{text: text, font: part.font, color: part.color, hasColor: part.hasColor})
					}
				}
			}
		}
	}
	if started || len(lines) == 0 {
		flush()
	}
	return lines
}

// wrap breaks the text greedily at spaces and after a hyphen inside a word to fit w; a piece wider
// than w stays on its line.
func (ti *textInfo) wrap(w float64) []string {
	if ti.parts != nil {
		var out []string
		for _, ln := range ti.layout(w) {
			out = append(out, ln.text())
		}
		return out
	}
	var out []string
	for _, para := range strings.Split(ti.text, "\n") {
		line := ""
		for i, word := range strings.Split(para, " ") {
			for j, piece := range hyphenPieces(word) {
				// The joined line is measured whole: a sum of word widths misses kerning and
				// wraps text that fits as one line, as its natural width says.
				cand := line + piece
				if j == 0 && i > 0 {
					cand = line + " " + piece
				}
				switch {
				case i == 0 && j == 0:
					line = piece
				case ti.width(cand) <= w+1e-9:
					line = cand
				default:
					out = append(out, line)
					line = piece
				}
			}
		}
		out = append(out, line)
	}
	return out
}

// hyphenPieces splits a word after each "-" that sits between a letter or digit and another character.
func hyphenPieces(word string) []string {
	if !strings.Contains(word, "-") {
		return []string{word}
	}
	var out []string
	rs := []rune(word)
	start := 0
	for i := 1; i+1 < len(rs); i++ {
		prev := rs[i-1]
		if rs[i] == '-' && (unicode.IsLetter(prev) || unicode.IsDigit(prev)) && rs[i+1] != '-' {
			out = append(out, string(rs[start:i+1]))
			start = i + 1
		}
	}
	return append(out, string(rs[start:]))
}

// TitleLine is one line of a title as the layout breaks it, for a test or a parity stand: how wide and
// tall the line is, the words on it, and whether a picture stands on it (written as "@").
type TitleLine struct {
	W, H  float64
	Parts []string
}

// TitleLines is how the title of the node is broken at w: one line when w is infinite or not above zero.
// It is the same layout the canvas draws, so a stand can compare the lines and not only the box.
func (s *Screen) TitleLines(n *Node, w float64) []TitleLine {
	if s == nil || s.p == nil || n == nil {
		return nil
	}
	rn := s.rnodeOf(n)
	if rn == nil {
		return nil
	}
	ti := s.p.texts[rn]
	if ti == nil {
		return nil
	}
	if w <= 0 {
		w = math.Inf(1)
	}
	out := make([]TitleLine, 0, 4)
	if ti.parts == nil {
		for _, text := range ti.wrap(w) {
			out = append(out, TitleLine{W: ti.width(text), H: ti.lineH, Parts: []string{text}})
		}
		return out
	}
	for _, ln := range ti.layout(w) {
		line := TitleLine{W: ln.w, H: ln.h}
		for _, part := range ln.parts {
			if part.img != nil {
				line.Parts = append(line.Parts, "@")
				continue
			}
			line.Parts = append(line.Parts, part.text)
		}
		out = append(out, line)
	}
	return out
}

// rnodeOf is the node of the layout that was written for n.
func (s *Screen) rnodeOf(n *Node) *RNode {
	for rn := range s.p.texts {
		if rn.Src == n {
			return rn
		}
	}
	return nil
}
