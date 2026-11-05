package mcpserver

import (
	"fmt"
	"strings"

	"github.com/haiodo/tenniarb/go/scene"
)

// inspect limits: the report is a tool for the next decision, not a dump of the tree.
const (
	inspectDepth = 3
	inspectLines = 220
	// inspectProblems bounds the problem lines of a summary; the full list is one depth 3 away.
	inspectProblems = 12
)

// inspectScene reports the geometry a rendered scene computed, levels deep: the size of the scene, the
// box of the items, the items that leave the box of their parent, and the overlapping siblings. Numbers
// are scene units, the origin at the top left of the scene. Zero levels is the summary: one line per
// top-level item, then only what is wrong.
func inspectScene(screen *scene.Screen, levels int) string {
	root := screen.Placed()
	if root == nil {
		return "geometry: nothing drawn"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "geometry: %.0fx%.0f units, %d top-level items\n", root.W, root.H, len(root.Kids))
	lines := 1
	var walk func(p *scene.Placed, depth int, path string)
	walk = func(p *scene.Placed, depth int, path string) {
		if lines >= inspectLines {
			return
		}
		name := path
		if name == "" {
			name = p.Src.Key
		}
		fmt.Fprintf(&b, "%s%s  %.0f %.0f %.0fx%.0f\n", strings.Repeat("  ", depth), name, p.X, p.Y, p.W, p.H)
		lines++
		if depth == levels && len(p.Kids) > 0 {
			fmt.Fprintf(&b, "%s  … %d below\n", strings.Repeat("  ", depth), descendants(p))
			lines++
			return
		}
		for _, k := range p.Kids {
			walk(k, depth+1, k.Src.Key)
		}
	}
	for _, k := range root.Kids {
		walk(k, 0, k.Src.Key)
	}
	// The items that leave a parent are why the numbers are asked for at all, so they are reported for
	// the whole tree even in the summary, once per kind: a repeated 1-unit overshoot is one line.
	seen := map[string]int{}
	var order []string
	var scan func(p *scene.Placed)
	scan = func(p *scene.Placed) {
		for _, k := range p.Kids {
			if over, ok := overflow(k); ok {
				key := k.Src.Key + " leaves its parent: " + over
				if seen[key] == 0 {
					order = append(order, key)
				}
				seen[key]++
			}
			scan(k)
		}
	}
	scan(root)
	for i, key := range order {
		if i == inspectProblems {
			fmt.Fprintf(&b, "… %d more problems, ask for depth 3\n", len(order)-i)
			break
		}
		if seen[key] > 1 {
			fmt.Fprintf(&b, "! %s (%d items)\n", key, seen[key])
			continue
		}
		fmt.Fprintf(&b, "! %s\n", key)
	}
	out := sticksOut(root)
	for _, line := range out {
		b.WriteString(line + "\n")
	}
	// A report with no problem line is the answer to "is anything wrong here?", and it should say so:
	// silence reads as "not checked" and costs another call.
	if len(order) == 0 && len(out) == 0 {
		b.WriteString("ok: nothing leaves its parent or its plate\n")
	}
	for _, pair := range overlaps(root.Kids) {
		fmt.Fprintf(&b, "overlap: %s and %s share %.0fx%.0f\n", pair[0], pair[1], pair[2], pair[3])
	}
	if lines >= inspectLines {
		b.WriteString("… more items left out\n")
	}
	return b.String()
}

// descendants counts the items below p.
func descendants(p *scene.Placed) int {
	n := 0
	for _, k := range p.Kids {
		n += 1 + descendants(k)
	}
	return n
}

// overflow is how far p leaves the box of its parent: right, bottom, left and top, the ones above 0.
func overflow(p *scene.Placed) (string, bool) {
	parent := p.Parent
	if parent == nil || parent.Src == nil || parent.Src.Kind == scene.KFile {
		return "", false
	}
	right := p.X + p.W - (parent.X + parent.W)
	bottom := p.Y + p.H - (parent.Y + parent.H)
	left := parent.X - p.X
	top := parent.Y - p.Y
	var out []string
	for _, s := range []struct {
		name string
		v    float64
	}{{"right", right}, {"bottom", bottom}, {"left", left}, {"top", top}} {
		if s.v > 0.5 {
			out = append(out, fmt.Sprintf("%s %.0f", s.name, s.v))
		}
	}
	return strings.Join(out, ", "), len(out) > 0
}

// sticksOut finds the items that sit on a bigger sibling and leave it, as a card on its plate: the
// smaller box overlapping the bigger one by most of its area, and not inside it.
func sticksOut(root *scene.Placed) []string {
	var out []string
	var walk func(p *scene.Placed)
	walk = func(p *scene.Placed) {
		kids := p.Kids
		for _, a := range kids {
			for _, b := range kids {
				if a == b || a.W*a.H >= b.W*b.H {
					continue
				}
				iw := min(a.X+a.W, b.X+b.W) - max(a.X, b.X)
				ih := min(a.Y+a.H, b.Y+b.H) - max(a.Y, b.Y)
				if iw <= 0.5 || ih <= 0.5 {
					continue
				}
				area := a.W * a.H
				if area <= 0 || iw*ih < 0.6*area {
					continue
				}
				if over, ok := leaves(a, b); ok {
					out = append(out, fmt.Sprintf("! %s sits on %s and leaves it: %s", a.Src.Key, b.Src.Key, over))
				}
			}
		}
		for _, k := range kids {
			walk(k)
		}
	}
	walk(root)
	return out
}

// leaves is how far a goes outside the box of b.
func leaves(a, b *scene.Placed) (string, bool) {
	var parts []string
	for _, s := range []struct {
		name string
		v    float64
	}{
		{"right", a.X + a.W - (b.X + b.W)},
		{"bottom", a.Y + a.H - (b.Y + b.H)},
		{"left", b.X - a.X},
		{"top", b.Y - a.Y},
	} {
		if s.v > 0.5 {
			parts = append(parts, fmt.Sprintf("%s %.0f", s.name, s.v))
		}
	}
	return strings.Join(parts, ", "), len(parts) > 0
}

// overlaps lists the pairs of siblings whose boxes share an area, with the size of that area.
func overlaps(kids []*scene.Placed) [][4]any {
	var out [][4]any
	for i, a := range kids {
		for _, b := range kids[i+1:] {
			w := min(a.X+a.W, b.X+b.W) - max(a.X, b.X)
			h := min(a.Y+a.H, b.Y+b.H) - max(a.Y, b.Y)
			if w > 0.5 && h > 0.5 {
				out = append(out, [4]any{a.Src.Key, b.Src.Key, w, h})
			}
		}
	}
	return out
}
