package scene

import (
	"math"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/layout"
)

// icon is a vector shape that a leaf item draws in place of its box.
type icon struct {
	path           *gfx.Path
	vx, vy, vw, vh float64
	fill, stroke   gfx.Color
}

// hidden reports an item that is out of the layout and the drawing.
func (p *prep) hidden(rn *RNode) bool { return p.flag(rn, "hidden") }

// icon reads path and view-box of a leaf; without view-box the bounds of the path points stand in.
func (p *prep) icon(rn *RNode, n *layout.Node) {
	pr := p.prop(rn, "path")
	if pr == nil {
		return
	}
	d, _ := pr.text(0)
	g, err := gfx.ParseSVGPath(d)
	if err != nil || g.Empty() {
		p.t.warn(pr.Pos, "path: not an SVG path, the item is drawn as a box")
		return
	}
	ic := &icon{path: g}
	ic.vx, ic.vy, ic.vw, ic.vh = pointBounds(g)
	if vb := p.prop(rn, "view-box"); vb != nil {
		var v [4]float64
		ok := len(vb.Args) == 4
		for i := 0; ok && i < 4; i++ {
			v[i], ok = vb.num(i)
		}
		if ok && v[2] > 0 && v[3] > 0 {
			ic.vx, ic.vy, ic.vw, ic.vh = v[0], v[1], v[2], v[3]
		} else {
			p.t.warn(vb.Pos, "view-box takes 4 numbers with width and height above 0, the path bounds are used")
		}
	}
	if ic.vw <= 0 || ic.vh <= 0 {
		p.t.warn(pr.Pos, "path: empty bounds, give a view-box")
		return
	}
	hasFill, hasStroke := p.prop(rn, "color") != nil, p.prop(rn, "border-color") != nil
	if !hasFill && !hasStroke {
		ic.fill = black
	} else {
		ic.fill = p.color(rn, "color", gfx.Color{})
		ic.stroke = p.color(rn, "border-color", gfx.Color{})
	}
	if v, ok := p.word(rn, "fill-rule", "evenodd", "nonzero"); ok {
		g.EvenOdd = v == "evenodd"
	}
	p.icons[rn] = ic
	n.LeafW = func() float64 { return ic.vw }
	n.LeafH = func(float64) float64 { return ic.vh }
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
func (ic *icon) draw(r *gfx.Recorder, w, h, lineW float64) {
	s := math.Min(w/ic.vw, h/ic.vh)
	m := gfx.Affine{s, 0, 0, s, (w-ic.vw*s)/2 - ic.vx*s, (h-ic.vh*s)/2 - ic.vy*s}
	g := ic.path.Transform(m)
	if ic.fill.A > 0 {
		r.FillPath(g, ic.fill)
	}
	if lineW > 0 && ic.stroke.A > 0 {
		r.StrokePath(g, gfx.StrokeStyle{Width: lineW, MiterLimit: 10}, ic.stroke)
	}
}
