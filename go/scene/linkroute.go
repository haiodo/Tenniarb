package scene

import (
	"math"
	"sort"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/layout"
)

// Sides of a box in layout coordinates: X grows to the right, Y down.
const (
	sideRight = iota
	sideLeft
	sideTop
	sideBottom
)

// elbowRadius is the bend of an elbow link; corner-radius on the link overrides it.
const elbowRadius = 12

// routeWord is route as written on a link, without a warning: the elbow layout reads it many times.
func (p *prep) routeWord(lk *RNode) string {
	if pr := p.prop(lk, "route"); pr != nil {
		s, _ := pr.text(0)
		return s
	}
	return ""
}

// isElbow reports whether the link asks for the elbow route.
func (p *prep) isElbow(lk *RNode) bool { return p.routeWord(lk) == "elbow" }

// sideWord is a side as the route names it.
func sideWord(s string) (int, bool) {
	switch s {
	case "right":
		return sideRight, true
	case "left":
		return sideLeft, true
	case "top":
		return sideTop, true
	case "bottom":
		return sideBottom, true
	}
	return 0, false
}

// routeSides are the sides the elbow leaves and enters: the ones the route names, else the sides of the
// boxes that look at each other.
func (p *prep) routeSides(lk *RNode, a, c *layout.Node) (from, to int) {
	from, to = facingSides(a, c)
	pr := p.prop(lk, "route")
	if pr == nil {
		return from, to
	}
	for i, want := range []*int{&from, &to} {
		if i+1 >= len(pr.Args) {
			break
		}
		word, _ := pr.text(i + 1)
		side, ok := sideWord(word)
		if !ok {
			p.t.warn(pr.Pos, "a route side is left, right, top or bottom, got %q", word)
			continue
		}
		*want = side
	}
	return from, to
}

// checkRoute warns about a route the format does not know; a link without one is straight.
func (p *prep) checkRoute(lk *RNode) {
	switch s := p.routeWord(lk); s {
	case "", "straight", "elbow":
	default:
		p.t.warn(p.prop(lk, "route").Pos, "route takes straight or elbow, got %q", s)
	}
}

// elbowCorner is where one link leaves or enters a box: the side and the place along it, 0..1.
type elbowCorner struct {
	side int
	f    float64
}

// elbow returns the corners of an elbow link: it leaves and enters the sides of its boxes that face each
// other, and the links that share a side spread along it, in the order of the boxes they lead to.
func (b *builder) elbow(parent, lk *RNode) []gfx.Point {
	ends, ok := b.p.ends[lk]
	if !ok {
		return nil
	}
	a, c := b.p.frames[ends[0]], b.p.frames[ends[1]]
	if a == nil || c == nil {
		return nil
	}
	starts, finishes := b.elbowSides(parent)
	sa, sc := starts[lk], finishes[lk]
	if _, ok := starts[lk]; !ok {
		// The link was not in the groups: sides still have to be known for the drawing.
		sa.side, sc.side = b.p.routeSides(lk, a, c)
		sa.f, sc.f = .5, .5
	}
	pa, pc := sideAnchor(a, sa.side, sa.f), sideAnchor(c, sc.side, sc.f)
	// A side that is a horizontal edge makes the line leave upwards or downwards, and the other way round.
	leaves, enters := !sideVertical(sa.side), !sideVertical(sc.side)
	switch {
	case leaves && !enters:
		return []gfx.Point{pa, {X: pa.X, Y: pc.Y}, pc}
	case !leaves && enters:
		return []gfx.Point{pa, {X: pc.X, Y: pa.Y}, pc}
	case leaves:
		my := (pa.Y + pc.Y) / 2
		return []gfx.Point{pa, {X: pa.X, Y: my}, {X: pc.X, Y: my}, pc}
	default:
		mx := (pa.X + pc.X) / 2
		return []gfx.Point{pa, {X: mx, Y: pa.Y}, {X: mx, Y: pc.Y}, pc}
	}
}

// elbowSides places every elbow link of a parent on the sides of both its ends.
func (b *builder) elbowSides(parent *RNode) (starts, finishes map[*RNode]elbowCorner) {
	type key struct {
		rn   *RNode
		side int
	}
	type member struct {
		lk    *RNode
		order float64
	}
	startGroups, endGroups := map[key][]member{}, map[key][]member{}
	sides := map[*RNode][2]int{}
	for _, k := range parent.Kids {
		if k.Kind != RLink || !b.p.isElbow(k) {
			continue
		}
		ends, ok := b.p.ends[k]
		if !ok {
			continue
		}
		a, c := b.p.frames[ends[0]], b.p.frames[ends[1]]
		if a == nil || c == nil {
			continue
		}
		from, to := b.p.routeSides(k, a, c)
		sides[k] = [2]int{from, to}
		startGroups[key{ends[0], from}] = append(startGroups[key{ends[0], from}], member{k, along(c, to)})
		endGroups[key{ends[1], to}] = append(endGroups[key{ends[1], to}], member{k, along(a, from)})
	}
	place := func(groups map[key][]member, which int) map[*RNode]elbowCorner {
		out := map[*RNode]elbowCorner{}
		for _, list := range groups {
			sort.SliceStable(list, func(i, j int) bool { return list[i].order < list[j].order })
			for i, m := range list {
				out[m.lk] = elbowCorner{side: sides[m.lk][which], f: float64(i+1) / float64(len(list)+1)}
			}
		}
		return out
	}
	return place(startGroups, 0), place(endGroups, 1)
}

// facingSides are the sides of a and of c that look at each other.
func facingSides(a, c *layout.Node) (from, to int) {
	dx := (c.RX + c.RW/2) - (a.RX + a.RW/2)
	dy := (c.RY + c.RH/2) - (a.RY + a.RH/2)
	if math.Abs(dx) >= math.Abs(dy) {
		if dx >= 0 {
			return sideRight, sideLeft
		}
		return sideLeft, sideRight
	}
	if dy >= 0 {
		return sideBottom, sideTop
	}
	return sideTop, sideBottom
}

// sideVertical reports whether the side is a vertical edge, where anchors vary in Y.
func sideVertical(side int) bool { return side == sideLeft || side == sideRight }

// sideAnchor is the point at f of a side of n.
func sideAnchor(n *layout.Node, side int, f float64) gfx.Point {
	switch side {
	case sideRight:
		return gfx.Point{X: n.RX + n.RW, Y: n.RY + n.RH*f}
	case sideLeft:
		return gfx.Point{X: n.RX, Y: n.RY + n.RH*f}
	case sideTop:
		return gfx.Point{X: n.RX + n.RW*f, Y: n.RY}
	}
	return gfx.Point{X: n.RX + n.RW*f, Y: n.RY + n.RH}
}

// along is the centre of n on the axis a side of the other box runs along.
func along(n *layout.Node, side int) float64 {
	if sideVertical(side) {
		return n.RY + n.RH/2
	}
	return n.RX + n.RW/2
}

// elbowPath is the polyline with the interior corners rounded by r.
func elbowPath(points []gfx.Point, r float64) *gfx.Path {
	p := &gfx.Path{}
	if len(points) < 2 {
		return p
	}
	p.MoveTo(points[0].X, points[0].Y)
	for i := 1; i < len(points)-1; i++ {
		prev, cur, next := points[i-1], points[i], points[i+1]
		rr := math.Min(r, math.Min(cur.Dist(prev)/2, cur.Dist(next)/2))
		if rr <= 0 {
			p.LineTo(cur.X, cur.Y)
			continue
		}
		before := cur.Sub(cur.Sub(prev).Unit().Mul(rr))
		after := cur.Add(next.Sub(cur).Unit().Mul(rr))
		p.LineTo(before.X, before.Y)
		p.QuadTo(cur.X, cur.Y, after.X, after.Y)
	}
	last := points[len(points)-1]
	p.LineTo(last.X, last.Y)
	return p
}
