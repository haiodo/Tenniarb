package scene

import (
	"slices"
	"sort"

	"golang.org/x/text/unicode/norm"
)

// RefKind says where a reference to a node is written.
type RefKind uint8

const (
	RefUse     RefKind = iota // the path of a use
	RefAt                     // the path of an at override
	RefReplace                // the path of a replace override
	RefLink                   // an end of a link
)

// Ref is an argument of a command in the text of File that names a node. Text is its value, a dotted
// path or the name of a link end; Seg is the segment of the path that names the node.
type Ref struct {
	Kind RefKind
	File string
	Arg  Range
	Text string
	Seg  int // -1 for a link end
}

// References lists what names target in every file of the tree: a path counts when it resolves
// through target (spec 4.2), so the same name in another scope is not found. at and replace are
// followed into the instances in each mount context. blocked says why the links cannot follow a
// rename: several siblings share the name and the index of a link would shift.
// Ceiling: links in the block of a use or in a slot fill are not looked at.
func (t *Tree) References(target *Node) (refs []Ref, blocked string) {
	if target == nil || t.top == nil {
		return nil, ""
	}
	f := &refFinder{t: t, target: target, seen: map[refKey]bool{}}
	t.quiet = true
	defer func() { t.quiet = false }()
	f.expand(t.top, nil)
	blocked = f.links()
	sort.SliceStable(f.out, func(i, j int) bool {
		a, b := f.out[i], f.out[j]
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Arg.Start < b.Arg.Start
	})
	return f.out, blocked
}

type refKey struct {
	file     string
	start, i int
}

// refFinder collects the references to target while the files are expanded.
type refFinder struct {
	t      *Tree
	target *Node
	seen   map[refKey]bool
	out    []Ref
}

func (f *refFinder) add(kind RefKind, file string, arg Range, text string, seg int) {
	k := refKey{file, arg.Start, seg}
	if arg == (Range{}) || f.seen[k] {
		return
	}
	f.seen[k] = true
	f.out = append(f.out, Ref{kind, file, arg, text, seg})
}

// expand expands n and what is written below it as validate does, in the context of its mount.
func (f *refFinder) expand(n *Node, c *mountCtx) {
	x := &expander{t: f.t, ctx: c, rec: f}
	x.container(n)
	if n.Mount != nil {
		c = c.push(n)
	}
	for _, e := range n.Elems {
		f.expand(e, c)
	}
}

// use sees the path of a use that resolved from ctx.
func (f *refFinder) use(n *Node, ctx *mountCtx) {
	h, msg := lookupHit(n.Parent, ctx, n.Path, true)
	if msg != "" {
		return
	}
	if i := slices.Index(slices.Concat(h.via, []*Node{h.n}), f.target); i >= 0 {
		f.add(RefUse, n.Pos.File, n.PathArg, n.Path, i)
	}
}

// at sees the path of an at or replace that reached final from the root of the instance.
func (f *refFinder) at(ov *Override, root, final *RNode) {
	segs, ok := splitPath(ov.Path)
	if !ok {
		return
	}
	kind := RefAt
	if ov.Kind == OvReplace {
		kind = RefReplace
	}
	for i, r := range chainTo(root, segs, final) {
		if r.Src == f.target {
			f.add(kind, ov.Pos.File, ov.PathArg, ov.Path, i)
		}
	}
}

// chainTo is the nodes the segments name from root down to final.
func chainTo(root *RNode, segs []string, final *RNode) []*RNode {
	for _, k := range root.Kids {
		if !(k.Kind == RItem && k.Key == segs[0] || k.Kind == RSlot && slotSegment(k) == segs[0]) {
			continue
		}
		if len(segs) == 1 {
			if k == final {
				return []*RNode{k}
			}
			continue
		}
		if rest := chainTo(k, segs[1:], final); rest != nil {
			return append([]*RNode{k}, rest...)
		}
	}
	return nil
}

// links adds the ends of the links next to target that name it.
func (f *refFinder) links() (blocked string) {
	p := f.target.Parent
	if p == nil || f.target.Kind != KItem && f.target.Kind != KUse {
		return ""
	}
	name := norm.NFC.String(f.target.Name)
	var same []*Node
	for _, c := range p.Content {
		if (c.Kind == KItem || c.Kind == KUse) && norm.NFC.String(c.Name) == name {
			same = append(same, c)
		}
	}
	at := slices.Index(same, f.target)
	for _, l := range p.Content {
		if l.Kind != KLink {
			continue
		}
		ends := [2]struct {
			text string
			arg  Range
			prop string
		}{{l.From, l.FromArg, "source-index"}, {l.To, l.ToArg, "target-index"}}
		for _, e := range ends {
			if norm.NFC.String(e.text) != name {
				continue
			}
			if len(same) > 1 {
				return "several items are named " + f.target.Name + " and a link addresses them by index"
			}
			if linkIndex(l, e.prop) == at {
				f.add(RefLink, l.Pos.File, e.arg, e.text, -1)
			}
		}
	}
	return ""
}

// linkIndex is the source-index or target-index of a link; the last one written wins, 0 when none.
func linkIndex(l *Node, prop string) int {
	for i := len(l.Props) - 1; i >= 0; i-- {
		if p := l.Props[i]; p.Name == prop {
			v, _ := p.num(0)
			return int(v)
		}
	}
	return 0
}

// NodeNamedAt is the node whose name is written at offset off of file, nil when there is none.
func (t *Tree) NodeNamedAt(file string, off int) *Node {
	var find func(n *Node) *Node
	find = func(n *Node) *Node {
		if n.Pos.File == file && n.NameArg.Start == off && n.NameArg != (Range{}) {
			return n
		}
		for _, c := range slices.Concat(n.Elems, n.Content) {
			if f := find(c); f != nil {
				return f
			}
		}
		return nil
	}
	if root := t.File(file); root != nil {
		return find(root)
	}
	return nil
}
