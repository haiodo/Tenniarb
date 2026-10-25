package penimport

// enode mirrors the expansion go/scene makes of use: the tree of an instance with its overrides and
// slots, so that an override addressed by a pen id can be turned into a path of names.
type enode struct {
	o      *Node
	kids   []*enode
	parent *enode
	open   bool // a slot nobody has filled yet
}

func (e *enode) add(k *enode) {
	k.parent = e
	e.kids = append(e.kids, k)
}

func expContent(o *Node) *enode {
	switch o.Kind {
	case KItem, KSlot:
		e := &enode{o: o, open: o.Kind == KSlot}
		for _, k := range o.Kids {
			if ke := expContent(k); ke != nil {
				e.add(ke)
			}
		}
		return e
	case KUse:
		return expUse(o)
	}
	return nil
}

// expUse is the expansion of an instance. Its slots stay open, like in go/scene, where they are
// replaced by their fallback only when the outermost use is done: overrides reach into them.
func expUse(u *Node) *enode {
	t := u.Target
	if t == nil {
		return nil
	}
	var inst *enode
	if t.Kind == KUse {
		inst = expUse(t)
	} else {
		inst = expContent(t)
	}
	if inst == nil {
		return nil
	}
	inst.o = u
	applyBlock(inst, u.Block)
	return inst
}

func applyBlock(root *enode, block []*Ovr) {
	var fills []*Ovr
	for _, ov := range block {
		switch ov.Kind {
		case OvReplace:
			if parent, i := findChain(root, ov.Chain); parent != nil {
				parent.splice(i, expNodes(ov.Nodes))
			}
		case OvSlot:
			fills = append(fills, ov)
		}
	}
	for _, ov := range fills {
		if parent, i := findSlot(root, ov.Slot); parent != nil {
			parent.splice(i, expNodes(ov.Nodes))
		}
	}
}

func expNodes(nodes []*Node) []*enode {
	var out []*enode
	for _, n := range nodes {
		if e := expContent(n); e != nil {
			out = append(out, e)
		}
	}
	return out
}

func (e *enode) splice(i int, with []*enode) {
	out := make([]*enode, 0, len(e.kids)-1+len(with))
	out = append(out, e.kids[:i]...)
	for _, w := range with {
		w.parent = e
		out = append(out, w)
	}
	e.kids = append(out, e.kids[i+1:]...)
}

func findSlot(e *enode, slot *Node) (*enode, int) {
	for i, k := range e.kids {
		if k.open && k.o == slot {
			return e, i
		}
		if p, j := findSlot(k, slot); p != nil {
			return p, j
		}
	}
	return nil, 0
}

// findChain follows origin nodes down from the instance root; it returns the parent of the target and
// the index of the target.
func findChain(root *enode, chain []*Node) (*enode, int) {
	cur := root
	var parent *enode
	idx := 0
	for _, n := range chain {
		parent, idx = nil, 0
		for i, k := range cur.kids {
			if k.o == n {
				parent, idx = cur, i
				break
			}
		}
		if parent == nil {
			return nil, 0
		}
		cur = cur.kids[idx]
	}
	return parent, idx
}

// locate lists the nodes below scope that come from the pen node (f, id).
func locate(scope *enode, f *File, id string) []*enode {
	var out []*enode
	var walk func(e *enode)
	walk = func(e *enode) {
		for _, k := range e.kids {
			if k.o.res == f && k.o.ID == id && k.o.Kind != KSlot {
				out = append(out, k)
			}
			walk(k)
		}
	}
	walk(scope)
	return out
}

// chainOf is the origin nodes from just below root down to e.
func chainOf(e, root *enode) []*Node {
	var rev []*Node
	for ; e != nil && e != root; e = e.parent {
		rev = append(rev, e.o)
	}
	out := make([]*Node, len(rev))
	for i, n := range rev {
		out[len(rev)-1-i] = n
	}
	return out
}
