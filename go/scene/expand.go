package scene

import (
	"fmt"
	"strings"
)

// RKind is the kind of a resolved node.
type RKind uint8

const (
	RItem RKind = iota
	RLink
	RSlot // a slot of a component not yet filled; gone after finalize
)

// RNode is a node after use and slots are expanded: what is drawn.
type RNode struct {
	Kind     RKind
	Name     string
	Key      string
	Pos      Pos // where it is written; for an instance, the use
	Props    []*Prop
	Kids     []*RNode // in file order, links and items mixed (z order)
	Slot     string   // RSlot: normalized name, "" for the default slot; Kids are the fallback
	Inst     bool     // the root of an instance: slots below it belong to it, not to the use around
	touched  bool     // RSlot: at or replace went through it into the fallback
	From, To string   // RLink
}

func (r *RNode) prop(name string) *Prop {
	for i := len(r.Props) - 1; i >= 0; i-- {
		if r.Props[i].Name == name {
			return r.Props[i]
		}
	}
	return nil
}

type frame struct {
	n     *Node
	label string
}

type expander struct {
	t     *Tree
	stack []frame
	depth int // uses being expanded; slots stay open until the outermost one is done
}

func label(n *Node) string {
	switch n.Kind {
	case KUse:
		return fmt.Sprintf("use %s as %s (%s)", n.Path, n.Name, n.Pos)
	case KItem:
		return fmt.Sprintf("item %q (%s)", n.Name, n.Pos)
	case KElement:
		return fmt.Sprintf("element %q (%s)", n.Name, n.Pos)
	}
	return fmt.Sprintf("file (%s)", n.Pos)
}

// enter pushes n on the stack of nodes being expanded; a node already there is a cycle.
func (x *expander) enter(n *Node, pos Pos) bool {
	for i, f := range x.stack {
		if f.n != n {
			continue
		}
		chain := make([]string, 0, len(x.stack)-i+1)
		for _, g := range x.stack[i:] {
			chain = append(chain, g.label)
		}
		x.t.fail(pos, "cycle: %s -> %s", strings.Join(chain, " -> "), label(n))
		return false
	}
	x.stack = append(x.stack, frame{n, label(n)})
	return true
}

func (x *expander) leave() { x.stack = x.stack[:len(x.stack)-1] }

func newR(kind RKind, n *Node) *RNode {
	return &RNode{Kind: kind, Name: n.Name, Key: n.Key, Pos: n.Pos, Props: append([]*Prop(nil), n.Props...)}
}

// container expands an element or a file: its content is the scene.
func (x *expander) container(n *Node) *RNode {
	if !x.enter(n, n.Pos) {
		return nil
	}
	defer x.leave()
	r := newR(RItem, n)
	r.Kids = x.contents(n.Content)
	return r
}

func (x *expander) contents(nodes []*Node) []*RNode {
	var out []*RNode
	for _, c := range nodes {
		if r := x.content(c); r != nil {
			out = append(out, r)
		}
	}
	return out
}

func (x *expander) content(n *Node) *RNode {
	switch n.Kind {
	case KItem:
		return x.item(n)
	case KUse:
		return x.use(n, false)
	case KLink:
		r := newR(RLink, n)
		r.Name, r.Key, r.From, r.To = "", "", n.From, n.To
		return r
	case KSlot:
		r := newR(RSlot, n)
		r.Slot = n.Key
		r.Kids = x.contents(n.Content)
		return r
	}
	return nil
}

func (x *expander) item(n *Node) *RNode {
	if !x.enter(n, n.Pos) {
		return nil
	}
	defer x.leave()
	r := newR(RItem, n)
	r.Kids = x.contents(n.Content)
	return r
}

// use expands an instance. As a target of another use (asTarget) its slots stay open for that use.
func (x *expander) use(n *Node, asTarget bool) *RNode {
	if !x.enter(n, n.Pos) {
		return nil
	}
	defer x.leave()
	x.depth++
	defer func() { x.depth-- }()
	target := x.t.resolve(n.Parent, n.Path, n.Pos)
	if target == nil {
		return nil
	}
	inst := x.instance(target, n)
	if inst == nil {
		return nil
	}
	inst.Name, inst.Key, inst.Pos, inst.Inst = n.Name, n.Key, n.Pos, true
	x.apply(inst, n.Block)
	if !asTarget && x.depth == 1 {
		finalize(inst)
	}
	return inst
}

func (x *expander) instance(target, use *Node) *RNode {
	switch target.Kind {
	case KItem, KLink:
		return x.content(target)
	case KUse:
		return x.use(target, true)
	case KElement, KFile:
		return x.container(target)
	case KMount:
		return x.container(target.Target)
	}
	x.t.fail(use.Pos, "path %s names a namespace, not a node", use.Path)
	return nil
}

// finalize replaces the slots still open by their fallback content.
func finalize(r *RNode) {
	kids := make([]*RNode, 0, len(r.Kids))
	for _, k := range r.Kids {
		finalize(k)
		if k.Kind == RSlot {
			kids = append(kids, k.Kids...)
		} else {
			kids = append(kids, k)
		}
	}
	r.Kids = kids
}

type slotFill struct {
	name  string
	pos   Pos
	nodes []*RNode
}

// apply puts the block of a use onto the instance root: properties override, nodes go to the default
// slot, at and replace reach into the instance, slot fills a named slot.
func (x *expander) apply(root *RNode, block []*Override) {
	var fills []*slotFill
	add := func(name string, pos Pos, nodes []*RNode) {
		for _, f := range fills {
			if f.name == name {
				f.nodes = append(f.nodes, nodes...)
				return
			}
		}
		fills = append(fills, &slotFill{name, pos, nodes})
	}
	for _, ov := range block {
		switch ov.Kind {
		case OvProp:
			setProp(root, ov.Prop)
		case OvNode:
			add("", ov.Pos, x.contents([]*Node{ov.Node}))
		case OvSlot:
			add(NormalizeName(ov.Slot), ov.Pos, x.nodes(ov.Block))
		case OvAt:
			if at, ok := x.find(root, ov.Path, ov.Pos); ok {
				x.apply(at.node(), ov.Block)
			}
		case OvReplace:
			if at, ok := x.find(root, ov.Path, ov.Pos); ok {
				at.parent.Kids = splice(at.parent.Kids, at.i, x.nodes(ov.Block))
			}
		}
	}
	for _, f := range fills {
		x.fillSlot(root, f)
	}
}

// nodes expands the block of a slot or replace, which takes nodes only.
func (x *expander) nodes(block []*Override) []*RNode {
	var out []*RNode
	for _, ov := range block {
		if ov.Kind != OvNode {
			x.t.fail(ov.Pos, "only item, link and use are allowed here")
			continue
		}
		if r := x.content(ov.Node); r != nil {
			out = append(out, r)
		}
	}
	return out
}

func splice(kids []*RNode, i int, with []*RNode) []*RNode {
	out := make([]*RNode, 0, len(kids)-1+len(with))
	out = append(out, kids[:i]...)
	out = append(out, with...)
	return append(out, kids[i+1:]...)
}

// setProp replaces the properties of that name, in the place of the first one, or appends.
func setProp(r *RNode, p *Prop) {
	out := make([]*Prop, 0, len(r.Props)+1)
	done := false
	for _, q := range r.Props {
		switch {
		case q.Name != p.Name:
			out = append(out, q)
		case !done:
			out, done = append(out, p), true
		}
	}
	if !done {
		out = append(out, p)
	}
	r.Props = out
}

type loc struct {
	parent *RNode
	i      int
}

func (l loc) node() *RNode { return l.parent.Kids[l.i] }

// find follows a path of names from the instance root: items, instances and slots, no links. A
// slot segment is the slot name ("default" for the unnamed one) and leads into its fallback; the
// last segment must be an item or an instance.
func (x *expander) find(root *RNode, path string, pos Pos) (loc, bool) {
	segs, ok := splitPath(path)
	if !ok {
		x.t.fail(pos, "bad path %q", path)
		return loc{}, false
	}
	nodes := []*RNode{root}
	var locs []loc
	for _, seg := range segs {
		locs = locs[:0:0]
		for _, p := range nodes {
			for i, k := range p.Kids {
				if k.Kind == RItem && k.Key == seg || k.Kind == RSlot && slotSegment(k) == seg {
					locs = append(locs, loc{p, i})
				}
			}
		}
		if len(locs) == 0 {
			x.t.fail(pos, "path %s: nothing named %q inside the instance", path, seg)
			return loc{}, false
		}
		nodes = nodes[:0:0]
		for _, l := range locs {
			n := l.node()
			if n.Kind == RSlot {
				n.touched = true
			}
			nodes = append(nodes, n)
		}
	}
	if len(nodes) > 1 {
		parts := make([]string, len(nodes))
		for i, n := range nodes {
			parts[i] = n.Pos.String()
		}
		x.t.fail(pos, "ambiguous path %s: candidates %s", path, strings.Join(parts, ", "))
		return loc{}, false
	}
	if nodes[0].Kind == RSlot {
		x.t.fail(pos, "path %s names a slot, not a node; fill it with slot %s { ... }", path, segs[len(segs)-1])
		return loc{}, false
	}
	return locs[0], true
}

// slotSegment is the path segment of a slot.
func slotSegment(r *RNode) string {
	if r.Slot == "" {
		return "default"
	}
	return r.Slot
}

func findSlot(r *RNode, name string) (loc, bool) {
	for i, k := range r.Kids {
		if k.Kind == RSlot && k.Slot == name {
			return loc{r, i}, true
		}
		if k.Inst {
			continue
		}
		if l, ok := findSlot(k, name); ok {
			return l, true
		}
	}
	return loc{}, false
}

func slotNames(r *RNode, out []string) []string {
	for _, k := range r.Kids {
		if k.Inst {
			continue
		}
		if k.Kind == RSlot {
			n := k.Slot
			if n == "" {
				n = "(default)"
			}
			out = append(out, n)
		}
		out = slotNames(k, out)
	}
	return out
}

func (x *expander) fillSlot(root *RNode, f *slotFill) {
	if f.name == "" && len(f.nodes) == 0 {
		return
	}
	at, ok := findSlot(root, f.name)
	if ok {
		if at.node().touched {
			x.t.warn(f.pos, "slot %s is filled here, the at/replace changes to its fallback are dropped", slotSegment(at.node()))
		}
		at.parent.Kids = splice(at.parent.Kids, at.i, f.nodes)
		return
	}
	have := strings.Join(slotNames(root, nil), ", ")
	if have == "" {
		have = "none"
	}
	if f.name == "" {
		x.t.fail(f.pos, "nodes passed, but the component has no default slot (slots: %s)", have)
		return
	}
	x.t.fail(f.pos, "unknown slot %q (slots: %s)", f.name, have)
}
