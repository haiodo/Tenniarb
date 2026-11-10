// tennlayout prints the geometry and the look of one element of a .tenn file as JSON.
//
//	tennlayout [-root DIR] FILE [ELEMENT]
//
// It is the Go side of the scene parity test: tennlayouts of the same files are the golden files of
// web/packages/render/test/scene-parity.test.ts. The output is stable: keys sorted, two space indent,
// numbers rounded to 3 decimals, colours as #rrggbb or #rrggbbaa.
//
// Every entry is one drawn item in paint order: its dotted path (spec 4.2), its rectangle in screen
// coordinates (y down, as go/scene/placed.go), its wrapped text lines and the colours, radius, clip,
// opacity, shadows and gradient the item is painted with. Links are listed apart, with the path of the
// item whose coordinates they are drawn in: they are not items and have no box of their own.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/tenniarb/go/classic"
	"github.com/haiodo/tenniarb/go/exec"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
	"github.com/haiodo/tenniarb/go/scene"
)

func main() {
	root := flag.String("root", ".", "workspace root: mounted files may not leave it")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: tennlayout [-root DIR] FILE [ELEMENT] > out.json")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() < 1 || flag.NArg() > 2 {
		flag.Usage()
		os.Exit(2)
	}
	path := ""
	if flag.NArg() == 2 && flag.Arg(1) != "." {
		path = flag.Arg(1)
	}
	out, err := layout(*root, flag.Arg(0), path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Stdout.Write(out)
}

func layout(root, file, path string) ([]byte, error) {
	tree, err := scene.Load(root, file)
	if err != nil {
		return nil, err
	}
	sc := gfx.NewScene()
	var screen *scene.Screen
	classic := classic.Hook(tree, root, file, path, classic.SceneContext(tree, path, gfx.DefaultMeasurer,
		func(text string, fontSize float64) model.DrawableSize {
			sz := render.MeasureTextSize(render.Options{Measurer: gfx.DefaultMeasurer}, text, fontSize)
			return model.DrawableSize{Width: sz.Width, Height: sz.Height}
		}))
	sc.Update(func(tx *gfx.Tx) {
		screen, err = tree.Build(tx, nil, path, scene.Options{FontName: "Helvetica, Arial, sans-serif", EvalVars: exec.SceneEvalVars, Classic: classic})
	})
	if err != nil {
		return nil, err
	}
	sc.Commit()
	for _, w := range screen.Warnings {
		fmt.Fprintln(os.Stderr, w)
	}
	out := dump(screen, screen.Classic())
	out["element"] = path
	return json.MarshalIndent(out, "", "  ")
}

// dump walks the placed items in paint order and reads each one's picture: what gfx draws is what a
// backend paints, so the golden is the render itself and not a second reading of the properties.
func dump(screen *scene.Screen, classic bool) map[string]any {
	boxes := screen.Boxes()
	var placed []*scene.Placed
	var walk func(p *scene.Placed)
	walk = func(p *scene.Placed) {
		placed = append(placed, p)
		for _, k := range p.Kids {
			walk(k)
		}
	}
	walk(screen.Placed())
	nodes := make([]any, 0, len(placed))
	paths := make(map[*gfx.Node]string, len(placed))
	for i, pl := range placed {
		path := ""
		if i < len(boxes) {
			path = boxes[i].Path
		}
		nodes = append(nodes, node(pl, path, classic))
		if pl.Gfx != nil {
			paths[pl.Gfx] = path
		}
	}
	return map[string]any{
		"width":  round(screen.W),
		"height": round(screen.H),
		"nodes":  nodes,
		"links":  links(screen.Root, paths),
	}
}

// links lists the link nodes: every gfx child that is not a placed item and paints a path without text.
// A link is drawn in the coordinates of the item that owns it, so that item's path goes with it.
func links(root *gfx.Node, paths map[*gfx.Node]string) []any {
	out := []any{}
	var walk func(n *gfx.Node, parent string)
	walk = func(n *gfx.Node, parent string) {
		for _, c := range n.Children() {
			if p, ok := paths[c]; ok {
				walk(c, p)
				continue
			}
			if l := linkOf(c, parent); l != nil {
				out = append(out, l)
			}
		}
	}
	walk(root, "")
	return out
}

// linkOf reads the stroke and the arrow head a link paints; nil for a node that paints text or nothing.
func linkOf(n *gfx.Node, parent string) map[string]any {
	pic := n.Picture()
	if pic == nil {
		return nil
	}
	var it gfx.Iter
	it.Reset(pic)
	var path []any
	var color *gfx.Color
	var width, phase float64
	var dash []float64
	arrow := false
	for it.Next() {
		op := it.Op()
		switch op.Kind {
		case gfx.OpStroke, gfx.OpStrokeShadow:
			c := op.Color
			color = &c
			width = op.Style.Width
			dash = op.Style.Dash
			phase = op.Style.DashPhase
			// The iter reuses the path it decodes into, so the points are copied now.
			path = pathPoints(op.Path)
		case gfx.OpFill:
			// The arrow head is the only fill of a link: a closed triangle at the end of the line.
			if op.Path != nil && len(op.Path.Verbs) == 4 && op.Path.Verbs[3] == gfx.ClosePath {
				arrow = true
			}
		case gfx.OpText, gfx.OpTextShadow:
			return nil
		}
	}
	if path == nil || color == nil {
		return nil
	}
	var line []float64
	if len(dash) > 0 {
		line = []float64{}
		for _, d := range dash {
			line = append(line, round(d))
		}
	}
	return map[string]any{
		"parent": parent,
		"path":   path,
		"color":  hex(*color),
		"width":  round(width),
		"dash":   line,
		"phase":  round(phase),
		"arrow":  arrow,
	}
}

// pathPoints is the path as verbs and points: ["m", x, y], ["l", x, y], ["q", cx, cy, x, y], ["z"].
// A rounded elbow has quadratic corners; keeping the verbs lets the web side draw the same curve.
func pathPoints(p *gfx.Path) []any {
	out := make([]any, 0, len(p.Verbs))
	i := 0
	for _, v := range p.Verbs {
		switch v {
		case gfx.MoveTo:
			out = append(out, []any{"m", round(p.Pts[i].X), round(p.Pts[i].Y)})
			i++
		case gfx.LineTo:
			out = append(out, []any{"l", round(p.Pts[i].X), round(p.Pts[i].Y)})
			i++
		case gfx.QuadTo:
			out = append(out, []any{"q", round(p.Pts[i].X), round(p.Pts[i].Y), round(p.Pts[i+1].X), round(p.Pts[i+1].Y)})
			i += 2
		case gfx.CubicTo:
			out = append(out, []any{"c", round(p.Pts[i].X), round(p.Pts[i].Y), round(p.Pts[i+1].X), round(p.Pts[i+1].Y), round(p.Pts[i+2].X), round(p.Pts[i+2].Y)})
			i += 3
		case gfx.ClosePath:
			out = append(out, []any{"z"})
		}
	}
	return out
}

func node(pl *scene.Placed, path string, classic bool) map[string]any {
	var fill, border, text *gfx.Color
	var borderW float64
	var lines []string
	var grad *gfx.Gradient
	var shadows []gfx.Shadow
	var radius [4]float64
	rotate := 0.0
	if pl.Gfx != nil {
		m := pl.Gfx.Transform()
		rotate = math.Atan2(m[1], m[0]) * 180 / math.Pi
	}
	clip := flagProp(pl, "clip")
	if pl.Gfx != nil {
		if pic := pl.Gfx.Picture(); pic != nil {
			var it gfx.Iter
			it.Reset(pic)
			for it.Next() {
				op := it.Op()
				switch op.Kind {
				case gfx.OpFill:
					c := op.Color
					fill = &c
					radius = mergeRadii(radius, pathRadius(op.Path))
				case gfx.OpFillGrad:
					grad = op.Grad
					g := *op.Grad
					c := gradientBase(&g)
					fill = &c
					radius = mergeRadii(radius, pathRadius(op.Path))
				case gfx.OpFillShadow:
					c := op.Color
					fill = &c
					shadows = addShadow(shadows, op.Shadow)
					radius = mergeRadii(radius, pathRadius(op.Path))
				case gfx.OpStroke:
					c, w := op.Color, op.Style.Width
					border, borderW = &c, w
					radius = mergeRadii(radius, pathRadius(op.Path))
				case gfx.OpStrokeShadow:
					c, w := op.Color, op.Style.Width
					border, borderW = &c, w
					shadows = addShadow(shadows, op.Shadow)
					radius = mergeRadii(radius, pathRadius(op.Path))
				case gfx.OpText, gfx.OpTextShadow:
					c := op.Color
					text = &c
					lines = append(lines, op.Text)
					if op.Kind == gfx.OpTextShadow {
						shadows = addShadow(shadows, op.Shadow)
					}
				}
			}
		}
	}
	if fill != nil || clip {
		// A clip or a fill draws the rounded rectangle of the item itself: the property is the radius
		// (a border-only item draws it inside by half the line width, which pathRadius has read).
		radius = propRadius(pl, pl.W, pl.H, classic)
	}
	out := map[string]any{
		"path":   path,
		"name":   placedName(pl),
		"rect":   map[string]any{"x": round(pl.X), "y": round(pl.Y), "w": round(pl.W), "h": round(pl.H)},
		"radius": []float64{round(radius[0]), round(radius[1]), round(radius[2]), round(radius[3])},
		"rotate": round(rotate),
		"clip":   clip,
		"opacity": round(func() float64 {
			if pl.Gfx != nil {
				return pl.Gfx.Opacity()
			}
			return 1
		}()),
	}
	if len(lines) > 0 {
		out["text"] = lines
	} else {
		out["text"] = nil
	}
	out["fill"] = colorOf(fill)
	out["border"] = colorOf(border)
	if border != nil {
		out["borderWidth"] = round(borderW)
	} else {
		out["borderWidth"] = nil
	}
	out["textColor"] = colorOf(text)
	out["shadows"] = shadowsOf(shadows)
	out["gradient"] = gradientOf(grad)
	return out
}

func placedName(pl *scene.Placed) string {
	if pl.Src == nil {
		return ""
	}
	return pl.Src.Name
}

// flagProp reads a yes/true flag of the item through the properties in effect (the overrides of the
// uses around applied), as go/scene's flag does.
func flagProp(pl *scene.Placed, name string) bool {
	pr := pl.Prop(name)
	if pr == nil {
		return false
	}
	v := pr.Values()
	return len(v) > 0 && (v[0] == "yes" || v[0] == "true")
}

// pathRadius is the corner radius of a rounded rectangle path: the first move, the three straight
// runs to the corners. A path that is not a rectangle (an icon) gives zeros, so the item's radius stays
// what the other paths say.
func pathRadius(p *gfx.Path) [4]float64 {
	var out [4]float64
	if p == nil || len(p.Verbs) < 5 {
		return out
	}
	var minX, minY, maxX, maxY float64
	first := true
	for _, pt := range p.Pts {
		if first {
			minX, maxX, minY, maxY = pt.X, pt.X, pt.Y, pt.Y
			first = false
			continue
		}
		minX, maxX = math.Min(minX, pt.X), math.Max(maxX, pt.X)
		minY, maxY = math.Min(minY, pt.Y), math.Max(maxY, pt.Y)
	}
	// Vertical and horizontal lines: the corner radius is the distance from the box edge to the end of
	// the straight run. RoundRect4 writes move, line, line, line in that order.
	var moves, lines []gfx.Point
	pi := 0
	for _, v := range p.Verbs {
		switch v {
		case gfx.MoveTo:
			moves = append(moves, p.Pts[pi])
			pi++
		case gfx.LineTo:
			lines = append(lines, p.Pts[pi])
			pi++
		case gfx.QuadTo:
			pi += 2
		case gfx.CubicTo:
			pi += 3
		}
	}
	if len(moves) == 0 || len(lines) < 3 {
		return out
	}
	out[0] = moves[0].X - minX
	out[1] = maxX - lines[0].X
	out[2] = maxY - lines[1].Y
	out[3] = lines[2].X - minX
	for i := range out {
		if out[i] < 0.001 {
			out[i] = 0
		}
	}
	return out
}

// propRadius is the corner-radius of the item, clamped as go/scene's RoundRect4 clamps it.
func propRadius(pl *scene.Placed, w, h float64, classic bool) [4]float64 {
	var out [4]float64
	pr := pl.Prop("corner-radius")
	// A classic diagram rounds every box by 8 points unless the file says otherwise; a circle and a
	// hairline are drawn as they are.
	switch {
	case pr == nil && (!classic || displayOf(pl) == "circle" || w < 2 || h < 2):
		return out
	case pr == nil:
		out = [4]float64{8, 8, 8, 8}
	default:
		first, _ := pr.Num(0)
		for i := range out {
			out[i] = first
		}
		if n := len(pr.Values()); n == 4 {
			for i := range out {
				if v, ok := pr.Num(i); ok {
					out[i] = v
				}
			}
		}
	}
	lim := math.Min(w, h) / 2
	for i := range out {
		out[i] = math.Min(out[i], lim)
	}
	return out
}

// displayOf is the display word of the item, "rect" when unset.
func displayOf(pl *scene.Placed) string {
	if pr := pl.Prop("display"); pr != nil {
		if v := pr.Values(); len(v) > 0 {
			return v[0]
		}
	}
	return "rect"
}

// mergeRadii keeps the largest radius of every corner across the paths of an item: the fill path has
// the radius itself, the border path the same radius less half the line width.
func mergeRadii(a, b [4]float64) [4]float64 {
	for i := range a {
		a[i] = math.Max(a[i], b[i])
	}
	return a
}

func colorOf(c *gfx.Color) any {
	if c == nil {
		return nil
	}
	return hex(*c)
}

func hex(c gfx.Color) string {
	q := func(v float64) int { return int(math.Round(math.Max(0, math.Min(1, v)) * 255)) }
	if q(c.A) >= 255 {
		return fmt.Sprintf("#%02x%02x%02x", q(c.R), q(c.G), q(c.B))
	}
	return fmt.Sprintf("#%02x%02x%02x%02x", q(c.R), q(c.G), q(c.B), q(c.A))
}

func gradientBase(g *gfx.Gradient) gfx.Color {
	for _, s := range g.Stops {
		if s.Color.A > 0 {
			return s.Color
		}
	}
	return g.Stops[0].Color
}

func gradientOf(g *gfx.Gradient) any {
	if g == nil {
		return nil
	}
	dx, dy := g.P1.X-g.P0.X, g.P1.Y-g.P0.Y
	angle := math.Atan2(dx, -dy) * 180 / math.Pi
	angle = math.Mod(math.Mod(angle, 360)+360, 360)
	stops := make([]any, 0, len(g.Stops))
	for _, s := range g.Stops {
		stops = append(stops, map[string]any{"offset": round(s.Offset), "color": hex(s.Color)})
	}
	return map[string]any{"angle": round(angle), "stops": stops}
}

// addShadow appends a shadow the picture paints, once: a fill and its border may share the same list.
func addShadow(list []gfx.Shadow, s gfx.Shadow) []gfx.Shadow {
	if n := len(list); n > 0 && list[n-1] == s {
		return list
	}
	return append(list, s)
}

func shadowsOf(list []gfx.Shadow) []any {
	out := make([]any, 0, len(list))
	for _, s := range list {
		out = append(out, map[string]any{
			"dx":    round(s.DX),
			"dy":    round(s.DY),
			"blur":  round(s.Blur),
			"color": hex(s.Color),
		})
	}
	return out
}

func round(v float64) float64 {
	return math.Round(v*1000) / 1000
}
