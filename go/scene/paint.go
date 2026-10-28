package scene

import (
	"math"

	"github.com/haiodo/gowt/gfx"
)

// gradient is a linear gradient of an item fill, as in CSS: the line runs at angle degrees
// (0 up, 90 right, 180 down) through the center of the box and reaches the corner colors.
type gradient struct {
	angle float64
	stops []gfx.Stop
}

func (g *gradient) at(w, h float64) *gfx.Gradient {
	a := g.angle * math.Pi / 180
	dx, dy := math.Sin(a), -math.Cos(a)
	half := (math.Abs(w*dx) + math.Abs(h*dy)) / 2
	cx, cy := w/2, h/2
	return gfx.NewLinearGradient(gfx.Point{X: cx - dx*half, Y: cy - dy*half}, gfx.Point{X: cx + dx*half, Y: cy + dy*half}, g.stops...)
}

// base is the color that stands for the gradient where one color is needed: the first stop that shows.
func (g *gradient) base() gfx.Color {
	for _, s := range g.stops {
		if s.Color.A > 0 {
			return s.Color
		}
	}
	return g.stops[0].Color
}

// fill reads color: one color, or `linear <angle> <color> [pos] <color> [pos] ...` with positions
// 0..1 for all stops or none (even spread). A bad gradient warns and gives def.
func (p *prep) fill(rn *RNode, def gfx.Color) (gfx.Color, *gradient) {
	pr := p.prop(rn, "color")
	if pr == nil {
		return def, nil
	}
	if w, _ := pr.text(0); w != "linear" {
		return p.color(rn, "color", def), nil
	}
	g := p.gradient(pr)
	if g == nil {
		return def, nil
	}
	return g.base(), g
}

func (p *prep) gradient(pr *Prop) *gradient {
	bad := func(format string, a ...any) *gradient {
		p.t.warn(pr.Pos, "color linear: "+format+", the default is used", a...)
		return nil
	}
	angle, ok := pr.num(1)
	if !ok {
		return bad("the angle is not a number")
	}
	g := &gradient{angle: math.Mod(math.Mod(angle, 360)+360, 360)}
	placed := 0
	for i := 2; i < len(pr.Args); {
		s, _ := pr.text(i)
		c, ok := parseColor(s)
		if !ok {
			return bad("%q is not a color", s)
		}
		i++
		off, has := pr.num(i)
		if has {
			i++
			placed++
		}
		g.stops = append(g.stops, gfx.Stop{Offset: off, Color: c})
	}
	n := len(g.stops)
	switch {
	case n < 2:
		return bad("needs at least 2 stops")
	case placed == 0:
		for i := range g.stops {
			g.stops[i].Offset = float64(i) / float64(n-1)
		}
	case placed != n:
		return bad("give a position to every stop or to none")
	}
	for i, s := range g.stops {
		if s.Offset < 0 || s.Offset > 1 || i > 0 && s.Offset < g.stops[i-1].Offset {
			return bad("positions must grow within 0..1")
		}
	}
	return g
}

// Defaults of shadow as in go/render: blur 5, black at 1/3 alpha.
const defaultShadowBlur = 5.0

var defaultShadowColor = gfx.Color{A: 1.0 / 3}

// shadow reads `shadow dx dy [blur [color]]` of the box. Y is up as in pos, the screen offset is -dy.
// A bad offset drops the shadow, a bad blur or color takes the default.
func (p *prep) shadow(rn *RNode) *gfx.Shadow {
	pr := p.prop(rn, "shadow")
	if pr == nil {
		return nil
	}
	dx, okx := pr.num(0)
	dy, oky := pr.num(1)
	if !okx || !oky {
		p.t.warn(pr.Pos, "shadow takes dx dy [blur [color]], numbers for the offset; ignored")
		return nil
	}
	sh := &gfx.Shadow{DX: dx, DY: -dy, Blur: defaultShadowBlur, Color: defaultShadowColor}
	if len(pr.Args) > 2 {
		if v, ok := pr.num(2); ok && v >= 0 {
			sh.Blur = v
		} else {
			p.t.warn(pr.Pos, "shadow: the blur is not a number from 0, %v is used", defaultShadowBlur)
		}
	}
	if len(pr.Args) > 3 {
		s, _ := pr.text(3)
		if c, ok := parseColor(s); ok {
			sh.Color = c
		} else {
			p.t.warn(pr.Pos, "shadow: %q is not a color, the default is used", s)
		}
	}
	if len(pr.Args) > 4 {
		p.t.warn(pr.Pos, "shadow takes dx dy [blur [color]], the extra values are ignored")
	}
	return sh
}
