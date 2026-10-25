package render

import (
	"math"

	"github.com/haiodo/gowt/gfx"
)

// path is CGMutablePath: recorded in y-up space so it can be measured like CGPath.boundingBox (control
// points included) and replayed into a gfx.Path, which is y-down.
type path struct {
	ops []pathOp
	box struct{ x0, y0, x1, y1 float64 }
}

type pathOp struct {
	kind   byte // 'M', 'L', 'A' (arcTo), 'Q', 'Z'
	a, b   Point
	radius float64
}

func newPath() *path {
	p := &path{}
	p.box.x0, p.box.y0, p.box.x1, p.box.y1 = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	return p
}

func (p *path) grow(pts ...Point) {
	for _, q := range pts {
		p.box.x0 = math.Min(p.box.x0, q.X)
		p.box.x1 = math.Max(p.box.x1, q.X)
		p.box.y0 = math.Min(p.box.y0, q.Y)
		p.box.y1 = math.Max(p.box.y1, q.Y)
	}
}

func (p *path) move(to Point) { p.ops = append(p.ops, pathOp{kind: 'M', a: to}); p.grow(to) }
func (p *path) line(to Point) { p.ops = append(p.ops, pathOp{kind: 'L', a: to}); p.grow(to) }

// arc is addArc(tangent1End:tangent2End:radius:).
func (p *path) arc(t1, t2 Point, radius float64) {
	p.ops = append(p.ops, pathOp{kind: 'A', a: t1, b: t2, radius: radius})
	p.grow(t1, t2)
}

// quad is addQuadCurve(to:control:).
func (p *path) quad(to, control Point) {
	p.ops = append(p.ops, pathOp{kind: 'Q', a: control, b: to})
	p.grow(control, to)
}

func (p *path) close() { p.ops = append(p.ops, pathOp{kind: 'Z'}) }

// boundingBox is CGPath.boundingBox: all points including control points.
func (p *path) boundingBox() Rect {
	return Rect{p.box.x0, p.box.y0, p.box.x1 - p.box.x0, p.box.y1 - p.box.y0}
}

// toGfx replays the path into a gfx.Path moved by (dx, dy) in y-up space and flipped to y-down.
func (p *path) toGfx(dx, dy float64) *gfx.Path {
	out := &gfx.Path{}
	pt := func(q Point) (float64, float64) { return q.X + dx, -(q.Y + dy) }
	have := false
	for _, op := range p.ops {
		switch op.kind {
		case 'M':
			out.MoveTo(pt(op.a))
			have = true
		case 'L':
			if !have {
				out.MoveTo(pt(op.a))
				have = true
			} else {
				out.LineTo(pt(op.a))
			}
		case 'Q':
			cx, cy := pt(op.a)
			x, y := pt(op.b)
			out.QuadTo(cx, cy, x, y)
		case 'A':
			x1, y1 := pt(op.a)
			x2, y2 := pt(op.b)
			out.ArcTo(x1, y1, x2, y2, op.radius)
			have = true
		case 'Z':
			out.Close()
		}
	}
	return out
}
