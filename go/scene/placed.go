package scene

import (
	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/layout"
	"github.com/haiodo/tenniarb/go/tenn"
)

// Placed is an item of a built screen: where it is drawn and the command written for it.
type Placed struct {
	// Src is the item or the slot content written in a file, the use for the root of an instance, the
	// element or file for the screen itself.
	Src *Node
	// Inst marks the root of an instance: the items below it come from the component, except those
	// the use passes in.
	Inst bool
	// Free: the position comes from pos (the parent has no flow layout, or the item is absolute).
	Free       bool
	Parent     *Placed
	Kids       []*Placed // in z order
	X, Y, W, H float64   // in the coordinates of the screen
	Gfx        *gfx.Node

	r *RNode
}

// Prop returns the property in effect, the overrides of the uses around applied; nil when unset.
func (p *Placed) Prop(name string) *Prop { return p.r.prop(name) }

// Placed is the tree of the drawn items, the screen itself at the root. Hidden items are not in it.
func (s *Screen) Placed() *Placed {
	var walk func(rn *RNode, parent *Placed, ox, oy float64, parentDir layout.Dir) *Placed
	walk = func(rn *RNode, parent *Placed, ox, oy float64, parentDir layout.Dir) *Placed {
		ln := s.p.frames[rn]
		pl := &Placed{Src: rn.Src, Inst: rn.Inst, Parent: parent, X: ox + ln.RX, Y: oy + ln.RY, W: ln.RW, H: ln.RH,
			Gfx: s.gfx[rn], r: rn, Free: parentDir == layout.Abs || ln.Absolute}
		for _, k := range rn.Kids {
			if k.Kind == RItem && s.p.frames[k] != nil {
				pl.Kids = append(pl.Kids, walk(k, pl, pl.X, pl.Y, ln.Dir))
			}
		}
		return pl
	}
	return walk(s.tree, nil, 0, 0, layout.Abs)
}

// Values are the arguments as text: the content of a string, the digits of a number.
func (p *Prop) Values() []string {
	out := make([]string, 0, len(p.Args))
	for _, a := range p.Args {
		s, _ := a.GetIdentText()
		out = append(out, s)
	}
	return out
}

// IsString reports whether argument i is a string literal.
func (p *Prop) IsString(i int) bool {
	return i < len(p.Args) && p.Args[i].Kind == tenn.KindStringLit
}

// Files lists the files of the tree, the main one first, as slash paths relative to the root.
func (t *Tree) Files() []string {
	out := []string{t.main.Pos.File}
	for _, f := range t.order {
		if f != t.main {
			out = append(out, f.Pos.File)
		}
	}
	return out
}

// Root is the workspace root, absolute with symlinks resolved.
func (t *Tree) Root() string { return t.root }
