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
)

// inspectScene reports the geometry the engine computed for a rendered scene: the size of the scene, the
// box of every item a few levels deep, the items that leave the box of their parent, and the pairs of
// siblings that overlap. Numbers are scene units, the origin at the top left of the scene.
func inspectScene(screen *scene.Screen) string {
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
		if over, ok := overflow(p); ok {
			fmt.Fprintf(&b, "%s! %s leaves its parent: %s\n", strings.Repeat("  ", depth), name, over)
			lines++
		}
		if depth >= inspectDepth {
			return
		}
		for _, k := range p.Kids {
			walk(k, depth+1, k.Src.Key)
		}
	}
	for _, k := range root.Kids {
		walk(k, 0, k.Src.Key)
	}
	for _, line := range sticksOut(root) {
		b.WriteString(line + "\n")
	}
	for _, pair := range overlaps(root.Kids) {
		fmt.Fprintf(&b, "overlap: %s and %s share %.0fx%.0f\n", pair[0], pair[1], pair[2], pair[3])
	}
	if lines >= inspectLines {
		b.WriteString("… more items left out\n")
	}
	return b.String()
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
