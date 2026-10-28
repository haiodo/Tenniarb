package scene

import (
	"math"
	"strings"
	"unicode"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/layout"
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
}

// Defaults of a text leaf, as in the Tenniarb app (font-size 18) plus a padding around the text.
const (
	defaultFontSize = 18.0
	textPadX        = 10.0
	textPadY        = 6.0
)

// prep turns resolved nodes into a layout tree and keeps what the drawing needs.
type prep struct {
	t      *Tree
	opts   Options
	frames map[*RNode]*layout.Node
	texts  map[*RNode]*textInfo
	styles map[*RNode]style
	ends   map[*RNode][2]*RNode // link -> its two items
	// classic: the element being laid out is a classic diagram (sceneRoot is false).
	classic bool
	icons   map[*RNode]*icon
}

type textInfo struct {
	font       gfx.Font
	padX, padY float64
	lineH      float64
	align      string // layout of a leaf's lines: "left", "middle" (the default) or "right"
	asc, desc  float64
	text       string
	m          gfx.TextMeasurer
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
		styles: map[*RNode]style{}, ends: map[*RNode][2]*RNode{}, icons: map[*RNode]*icon{}}
}

// root is the layout of an element: its content at their own positions.
func (p *prep) root(rn *RNode) *layout.Node {
	p.classic = !sceneRoot(rn)
	n := &layout.Node{Dir: layout.Abs, Tag: rn}
	p.frames[rn] = n
	for _, k := range rn.Kids {
		if k.Kind == RItem && !p.hidden(k) {
			n.Kids = append(n.Kids, p.item(k, false))
		}
	}
	p.links(rn)
	return n
}

// sceneRoot is model.SceneElement on the expanded tree: an instance, an item with child nodes or
// auto-layout. Other elements are classic diagrams, which go/render draws with their expressions.
func sceneRoot(rn *RNode) bool {
	for _, k := range rn.Kids {
		if k.Kind == RItem && (k.Inst || len(k.Kids) > 0 || k.prop("auto-layout") != nil) {
			return true
		}
	}
	return false
}

// prop is rn.prop, except that a property with an expression ($(...), ${...}) is treated as absent:
// expressions are not evaluated here. A scene element reports it once; in a classic diagram an
// expression is normal and go/render evaluates it.
func (p *prep) prop(rn *RNode, name string) *Prop {
	pr := rn.prop(name)
	if pr == nil {
		return nil
	}
	for _, a := range pr.Args {
		if a.Kind == tenn.KindExpression || a.Kind == tenn.KindExpressionBlock {
			if !p.classic {
				p.t.warn(pr.Pos, "%s: expressions are not evaluated", name)
			}
			return nil
		}
	}
	return pr
}

// A link end is the n-th sibling with that NFC name (source-index/target-index, default 0),
// the same rule as PrepareItemRefs in go/model.
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
			p.t.fail(k.Pos, "link %q %q: both ends must be items of the same parent", k.From, k.To)
			continue
		}
		if p.frames[a] == nil || p.frames[b] == nil { // an end is hidden: the link goes with it
			continue
		}
		p.ends[k] = [2]*RNode{a, b}
		p.linkStyle(k)
	}
}

func (p *prep) item(rn *RNode, parentAuto bool) *layout.Node {
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
	n.W, n.H = p.size(rn, "width", inCtx), p.size(rn, "height", inCtx)
	if inCtx {
		p.limits(rn, n)
		n.Absolute = p.flag(rn, "absolute")
	}
	if auto {
		p.container(rn, n)
	} else {
		n.Dir = layout.Abs
	}
	for _, k := range rn.Kids {
		if k.Kind == RItem && !p.hidden(k) {
			n.Kids = append(n.Kids, p.item(k, auto))
		}
	}
	p.styles[rn] = p.style(rn, inCtx)
	if len(n.Kids) == 0 {
		p.leaf(rn, n, inCtx)
	} else {
		p.texts[rn] = p.textOf(rn, inCtx)
		p.links(rn)
	}
	return n
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
	n.Pad = p.padding(rn)
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
			p.t.fail(pr.Pos, "padding takes numbers")
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
	p.t.fail(pr.Pos, "padding takes 1, 2 or 4 numbers, got %d", len(v))
	return [4]float64{}
}

func (p *prep) num(rn *RNode, name string) (float64, bool) {
	pr := p.prop(rn, name)
	if pr == nil {
		return 0, false
	}
	v, ok := pr.num(0)
	if !ok {
		p.t.fail(pr.Pos, "%s takes a number", name)
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
	p.t.fail(pr.Pos, "%s takes yes or no", name)
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
		p.t.fail(pr.Pos, "%s takes one of %s, got %q", name, strings.Join(allowed, ", "), s)
	}
	return "", false
}

// size reads width or height: a number, or hug and fill where an auto-layout is at work.
func (p *prep) size(rn *RNode, name string, inCtx bool) layout.Size {
	pr := p.prop(rn, name)
	if pr == nil || len(pr.Args) == 0 {
		return layout.Size{}
	}
	if v, ok := pr.num(0); ok {
		return layout.Fixed(v)
	}
	a := pr.Args[0]
	s, _ := a.GetIdentText()
	switch {
	case s == "hug" || s == "fill":
		return p.sizeWord(pr, s, inCtx)
	case strings.HasSuffix(s, "%"):
		p.t.warn(pr.Pos, "%s: percentages are not supported, the size is hug", name)
	default:
		p.t.fail(pr.Pos, "%s takes a number, hug or fill, got %q", name, s)
	}
	return layout.Size{}
}

func (p *prep) sizeWord(pr *Prop, word string, inCtx bool) layout.Size {
	if !inCtx {
		p.t.fail(pr.Pos, "%s %s needs auto-layout on the item or on its parent", pr.Name, word)
		return layout.Size{}
	}
	if len(pr.Args) > 1 {
		p.t.warn(pr.Pos, "%s fill with a fallback value is not supported", pr.Name)
	}
	if word == "fill" {
		return layout.Fill
	}
	return layout.Size{}
}

// leaf gives an item without children the size of its title.
func (p *prep) leaf(rn *RNode, n *layout.Node, inCtx bool) {
	p.icon(rn, n)
	// An icon draws no name as text, as in a layout context.
	ti := p.textOf(rn, inCtx || p.icons[rn] != nil)
	if ti == nil {
		return
	}
	p.texts[rn] = ti
	if pr := p.prop(rn, "padding"); pr != nil && p.prop(rn, "auto-layout") != nil {
		ti.padY, ti.padX = n.Pad[0], n.Pad[3]
	}
	n.LeafW = func() float64 { return math.Ceil(ti.natural()) + 2*ti.padX }
	n.LeafH = func(w float64) float64 { return float64(len(ti.wrap(w-2*ti.padX)))*ti.lineH + 2*ti.padY }
}

func (p *prep) textOf(rn *RNode, inCtx bool) *textInfo {
	// As in scene.ts the name is the text and a title overrides it; in a layout context only a title is text.
	title := rn.Name
	if inCtx {
		title = ""
	}
	if pr := p.prop(rn, "title"); pr != nil {
		title, _ = pr.text(0)
	}
	if title == "" {
		return nil
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
	if inCtx {
		padX, padY = 0, 0
	}
	ti := &textInfo{font: p.fontOf(rn, size), padX: padX, padY: padY, text: title, m: p.opts.Measurer}
	m := ti.m.MeasureText("Hg", ti.font)
	ti.asc, ti.desc = m.Ascent, m.Descent
	ti.lineH = math.Round(m.Ascent + m.Descent)
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
	w := 0.0
	for _, ln := range strings.Split(ti.text, "\n") {
		w = math.Max(w, ti.width(ln))
	}
	return w
}

// wrap breaks the text greedily at spaces and after a hyphen inside a word to fit w; a piece wider
// than w stays on its line.
func (ti *textInfo) wrap(w float64) []string {
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
