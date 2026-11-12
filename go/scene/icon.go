package scene

import (
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/gowt/gfx/layout"
	"github.com/haiodo/tenn/go"
)

// pathCache keeps the parsed icon paths by their text: every load validates every element again.
// Cleared when full, which only costs a parse.
var pathCache = struct {
	sync.Mutex
	m map[string]*gfx.Path
}{m: map[string]*gfx.Path{}}

const pathCacheMax = 4096

// parsePath is gfx.ParseSVGPath of d; the result is a copy of the cached one, so a caller may set EvenOdd.
func parsePath(d string) (*gfx.Path, error) {
	pathCache.Lock()
	g, ok := pathCache.m[d]
	pathCache.Unlock()
	if !ok {
		var err error
		if g, err = gfx.ParseSVGPath(d); err != nil {
			return nil, err
		}
		pathCache.Lock()
		if len(pathCache.m) >= pathCacheMax {
			clear(pathCache.m)
		}
		pathCache.m[d] = g
		pathCache.Unlock()
	}
	cp := *g // Verbs and Pts are shared and never written
	return &cp, nil
}

// icon is a vector shape that a leaf item draws in place of its box.
type icon struct {
	path           *gfx.Path
	vx, vy, vw, vh float64
	fill, stroke   gfx.Color
	width          float64   // line-width of the command, else of the item
	grad           *gradient // set by color linear ...; fill is then its first visible stop
}

// hidden reports an item that is out of the layout and the drawing.
func (p *prep) hidden(rn *RNode) bool { return p.flag(rn, "hidden") }

// icon reads every path of a leaf, in file order, and the view-box of the item: a shape may name its own
// ink - color fills it, border-color and line-width stroke it - and takes the one of the item otherwise,
// as a shadow without a colour takes the alpha of the fill. The first path is drawn first, so it lies
// under the rest. Without view-box the bounds of the first path stand in.
func (p *prep) icon(rn *RNode, n *layout.Node) {
	list := p.allProps(rn, "path")
	if len(list) == 0 {
		return
	}
	vx, vy, vw, vh := 0.0, 0.0, 0.0, 0.0
	box := false
	if vb := p.prop(rn, "view-box"); vb != nil {
		var v [4]float64
		ok := len(vb.Args) == 4
		for i := 0; ok && i < 4; i++ {
			v[i], ok = vb.num(i)
		}
		if ok && v[2] > 0 && v[3] > 0 {
			vx, vy, vw, vh = v[0], v[1], v[2], v[3]
			box = true
		} else {
			p.t.warn(vb.Pos, "view-box takes 4 numbers with width and height above 0, the path bounds are used")
		}
	}
	hasFill, hasStroke := p.prop(rn, "color") != nil, p.prop(rn, "border-color") != nil
	itemFill, itemGrad := p.fill(rn, gfx.Color{})
	itemStroke := p.color(rn, "border-color", gfx.Color{})
	var out []*icon
	for _, pr := range list {
		d, _ := pr.text(0)
		g, err := parsePath(d)
		if err != nil || g.Empty() {
			p.t.warn(pr.Pos, "path: not an SVG path, the item is drawn as a box")
			continue
		}
		ic := &icon{path: g, vx: vx, vy: vy, vw: vw, vh: vh}
		if !box {
			ic.vx, ic.vy, ic.vw, ic.vh = pointBounds(g)
		}
		if ic.vw <= 0 || ic.vh <= 0 {
			p.t.warn(pr.Pos, "path: empty bounds, give a view-box")
			continue
		}
		// The ink of the command is the whole ink of its shape: a shape that names a fill is filled and not
		// stroked, one that names a stroke is stroked and not filled. Only a shape that names no ink at all
		// takes the one of the item, as a shadow without a colour takes the alpha of the fill.
		fill, hasOwnFill := pr.colorOf("color")
		stroke, hasOwnStroke := pr.colorOf("border-color")
		switch {
		case hasOwnFill || hasOwnStroke:
			ic.fill, ic.stroke = fill, stroke
		case !hasFill && !hasStroke && len(out) == 0:
			ic.fill = p.styles[rn].text
		default:
			ic.fill, ic.grad = itemFill, itemGrad
			ic.stroke = itemStroke
		}
		if v, ok := pr.numOf("line-width"); ok && v >= 0 {
			ic.width = v
		} else {
			ic.width = p.styles[rn].lineW
		}
		if v, ok := pr.wordOf("fill-rule", "evenodd", "nonzero"); ok {
			g.EvenOdd = v == "evenodd"
		} else if v, ok := p.word(rn, "fill-rule", "evenodd", "nonzero"); ok {
			g.EvenOdd = v == "evenodd"
		}
		out = append(out, ic)
	}
	if len(out) == 0 {
		return
	}
	p.icons[rn] = out
	n.LeafW = func() float64 { return out[0].vw }
	n.LeafH = func(float64) float64 { return out[0].vh }
}

// identText is the text of a node as a word: the argument of a command is a bare word or a string.
func identText(a *tenn.Node) string {
	if a == nil {
		return ""
	}
	if s, ok := a.GetIdentText(); ok {
		return s
	}
	return ""
}

// argAt is the argument that follows the word name in a command: `path "M…" color #e11` names its ink
// that way. It is nil when the command does not name it.
func (pr *Prop) argAt(name string) *tenn.Node {
	for i, a := range pr.Args {
		if i+1 < len(pr.Args) && a != nil && identText(a) == name {
			return pr.Args[i+1]
		}
	}
	return nil
}

// colorOf reads a colour written as an argument of the command, ok is false when it writes none.
func (pr *Prop) colorOf(name string) (gfx.Color, bool) {
	a := pr.argAt(name)
	if a == nil {
		return gfx.Color{}, false
	}
	c, ok := parseColor(identText(a))
	return c, ok
}

// numOf reads a number written as an argument of the command.
func (pr *Prop) numOf(name string) (float64, bool) {
	a := pr.argAt(name)
	if a == nil {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(identText(a)), 64)
	return v, err == nil
}

// wordOf reads a word written as an argument of the command, one of the given.
func (pr *Prop) wordOf(name string, words ...string) (string, bool) {
	a := pr.argAt(name)
	if a == nil {
		return "", false
	}
	v := identText(a)
	return v, slices.Contains(words, v)
}

func pointBounds(g *gfx.Path) (x, y, w, h float64) {
	if len(g.Pts) == 0 {
		return
	}
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, q := range g.Pts {
		x0, x1 = math.Min(x0, q.X), math.Max(x1, q.X)
		y0, y1 = math.Min(y0, q.Y), math.Max(y1, q.Y)
	}
	return x0, y0, x1 - x0, y1 - y0
}

// draw fits the view box into w x h keeping the aspect ratio, centered. The path is scaled, not the
// canvas, so the stroke width stays in item units.
func (ic *icon) draw(r *gfx.Recorder, w, h, lineW float64, cap gfx.Cap, join gfx.Join) {
	s := math.Min(w/ic.vw, h/ic.vh)
	m := gfx.Affine{s, 0, 0, s, (w-ic.vw*s)/2 - ic.vx*s, (h-ic.vh*s)/2 - ic.vy*s}
	g := ic.path.Transform(m)
	if ic.fill.A > 0 {
		if ic.grad != nil {
			r.FillGradient(g, ic.grad.at(w, h))
		} else {
			r.FillPath(g, ic.fill)
		}
	}
	if lineW > 0 && ic.stroke.A > 0 {
		r.StrokePath(g, gfx.StrokeStyle{Width: lineW, Cap: cap, Join: join, MiterLimit: 10}, ic.stroke)
	}
}
