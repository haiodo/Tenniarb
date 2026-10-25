package penimport

import "sort"

var rootSkip = map[string]bool{"id": true, "type": true, "ref": true, "name": true, "descendants": true, "children": true,
	"x": true, "y": true, "reusable": true, "theme": true, "enabled": true, "layoutPosition": true}

func mergeMaps(lo, hi map[string]any) map[string]any {
	out := make(map[string]any, len(lo)+len(hi))
	for k, v := range lo {
		out[k] = v
	}
	for k, v := range hi {
		out[k] = v
	}
	return out
}

func without(m map[string]any, skip map[string]bool) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		if !skip[k] {
			out[k] = v
		}
	}
	return out
}

// effPen is the pen map of a node with the root overrides of the instances on the way applied.
func effPen(n *Node) map[string]any {
	if n.Kind != KUse || n.Target == nil {
		return n.Pen
	}
	return mergeMaps(effPen(n.Target), without(n.Pen, rootSkip))
}

// finishTree resolves every use below n.
func (c *conv) finishTree(n *Node) {
	if n.Kind == KUse {
		c.finish(n)
		return
	}
	for _, k := range n.Kids {
		c.finishTree(k)
	}
}

// finishAll resolves the uses of a file; nodes made inside blocks are resolved when their block is.
func (c *conv) finishAll(f *File) {
	for _, n := range f.Root.Kids {
		c.finishTree(n)
	}
}

func (c *conv) placeholder(u *Node, why string) {
	f := u.File
	f.unsup("unresolved ref")
	f.Placeholders = append(f.Placeholders, u.ref)
	f.warn("%s: %s (ref %s), a zero size placeholder is written", u.Name, why, u.ref)
	u.Kind, u.Target, u.Block = KItem, nil, nil
	u.Name = "missing: " + u.Name
	u.missing = true
	u.Typ = "frame"
	// absolute keeps the placeholder out of the gaps of a flow, as the node that gfx/pen leaves out
	u.Props = append([]Prop{prop("width", numArg(0)), prop("height", numArg(0)), prop("absolute", wordArg("yes"))},
		posProps(u.Pen, u.b, false)...)
}

func (c *conv) finish(u *Node) {
	switch u.state {
	case 2:
		return
	case 1:
		c.placeholder(u, "instance of itself")
		u.state = 2
		return
	}
	u.state = 1
	defer func() { u.state = 2 }()
	t := u.res.resolve(u.ref)
	if t == nil {
		c.placeholder(u, "the component is not there")
		return
	}
	u.Target = t
	c.finishTree(t)
	if u.Kind != KUse || t.missing {
		if t.missing {
			c.placeholder(u, "the component is a placeholder")
		}
		return
	}
	u.Props = append(hiddenProp(u.Pen, false), c.props(u.res, without(u.Pen, rootSkip), u.Typ, pctx{mode: u.b.mode, explicit: true, eff: effPen(u)})...)
	u.Props = append(u.Props, posProps(u.Pen, u.b, false)...)
	if th := u.Pen["theme"]; th != nil {
		u.File.unsup("theme on an instance (colors stay light)")
	}
	c.buildBlock(u)
}

// ovAcc collects what the descendants of one instance say, per expanded node.
type ovAcc struct {
	props map[*enode]map[string]any
	repl  map[*enode]map[string]any
}

func (c *conv) buildBlock(u *Node) {
	base := expUse(u)
	if base == nil {
		return
	}
	acc := &ovAcc{props: map[*enode]map[string]any{}, repl: map[*enode]map[string]any{}}
	if kids, ok := u.Pen["children"].([]any); ok {
		acc.props[base] = map[string]any{"children": kids}
	}
	desc, _ := u.Pen["descendants"].(map[string]any)
	c.collect(u, base, desc, acc)
	u.Block = c.emitOps(u, base, acc)
}

func sortedKeys(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func isReplacement(o *Node, val map[string]any) bool { return replaces(o.Pen, o.ID, val) }

// replaces tells whether an override replaces the node pen instead of changing its properties.
func replaces(pen map[string]any, id string, val map[string]any) bool {
	t, has := val["type"]
	if !has {
		return false
	}
	oid, _ := val["id"].(string)
	return t != pen["type"] || val["ref"] != pen["ref"] || oid != "" && oid != id
}

// keyTarget resolves a key of a descendants map written in f to the file and the id of the pen node.
func keyTarget(f *File, key string) (*File, string) {
	alias, id := splitRef(key)
	if alias != "" {
		return f.Imports[alias], id
	}
	return f, id
}

// collect reads a descendants map of u. The nested maps of an instance come first, because the
// outer ones win where both name a node.
func (c *conv) collect(u *Node, scope *enode, desc map[string]any, acc *ovAcc) {
	keys := sortedKeys(desc)
	for pass := 0; pass < 2; pass++ {
		for _, key := range keys {
			val, ok := desc[key].(map[string]any)
			if !ok {
				continue
			}
			ms := c.matches(u, scope, key)
			for _, e := range ms {
				c.collectOne(u, e, val, pass, acc)
			}
			if len(ms) == 0 && pass == 0 {
				c.missingTarget(u, key, val)
			}
		}
	}
}

func (c *conv) collectOne(u *Node, e *enode, val map[string]any, pass int, acc *ovAcc) {
	if isReplacement(e.o, val) {
		if pass == 1 {
			acc.repl[e] = val
		}
		return
	}
	if nd, ok := val["descendants"].(map[string]any); ok && pass == 0 {
		c.collect(u, e, nd, acc)
	}
	if pass == 1 {
		acc.props[e] = mergeMaps(acc.props[e], without(val, map[string]bool{"descendants": true, "id": true, "type": true, "ref": true}))
	}
}

func (c *conv) matches(u *Node, scope *enode, key string) []*enode {
	g, id := keyTarget(u.res, key)
	if g == nil {
		return nil
	}
	return locate(scope, g, id)
}

func (c *conv) missingTarget(u *Node, key string, val map[string]any) {
	g, id := keyTarget(u.res, key)
	if g != nil && g.dropped[id] {
		if val["enabled"] == true {
			u.File.unsup("enabling a disabled node inside a nested instance (not reachable)")
		}
		return
	}
	if g == nil || g.rawIdx[id] == nil {
		u.File.unsup("override of an id that is not in the file or library")
		return
	}
	u.File.unsup("override target not found in the instance")
}

// emitOps turns the collected overrides into the commands of the block, in tree order.
func (c *conv) emitOps(u *Node, base *enode, acc *ovAcc) []*Ovr {
	var ops []*Ovr
	var visit func(e *enode)
	visit = func(e *enode) {
		if p := acc.props[e]; p != nil {
			ops = append(ops, c.propOps(u, base, e, p)...)
		}
		for _, k := range e.kids {
			if val := acc.repl[k]; val != nil {
				ops = append(ops, c.replaceOp(u, base, k, val))
				continue
			}
			// go/scene drops the changes to the fallback of a slot that the block fills
			if k.open && slotFilled(ops, k.o) {
				continue
			}
			visit(k)
		}
	}
	visit(base)
	return ops
}

func slotFilled(ops []*Ovr, slot *Node) bool {
	for _, op := range ops {
		if op.Kind == OvSlot && op.Slot == slot {
			return true
		}
	}
	return false
}

func (c *conv) replaceOp(u *Node, base, e *enode, val map[string]any) *Ovr {
	m := val
	for _, k := range [...]string{"x", "y"} {
		if _, has := m[k]; !has {
			if v, ok := e.o.Pen[k]; ok {
				m = mergeMaps(m, map[string]any{k: v})
			}
		}
	}
	b := bctx{mode: c.mode, parentAbs: parentAbs(e), noIndex: true, res: u.res, layout: layoutParent(e)}
	op := &Ovr{Kind: OvReplace, Chain: chainOf(e, base)}
	if n := c.buildNode(u.File, m, u, b); n != nil {
		c.finishTree(n)
		u.Content = append(u.Content, n)
		op.Nodes = []*Node{n}
	}
	return op
}

// layoutParent tells whether the node e sits in a frame, slots not counted.
func layoutParent(e *enode) bool {
	p := e.parent
	for p != nil && p.o.Kind == KSlot {
		p = p.parent
	}
	return p != nil && p.o.Typ == "frame"
}

func parentAbs(e *enode) bool {
	return e.parent != nil && effPen(e.parent.o)["layout"] == "none"
}

func (c *conv) propOps(u *Node, base, e *enode, p map[string]any) []*Ovr {
	var ops []*Ovr
	if kids, ok := p["children"].([]any); ok {
		ops = append(ops, c.fillOps(u, base, e, kids)...)
	}
	rest := without(p, map[string]bool{"children": true, "enabled": true, "x": true, "y": true, "layoutPosition": true})
	eff := mergeMaps(effPen(e.o), p)
	typ := e.o.Typ
	if typ == "" {
		typ = "frame"
	}
	props := append(hiddenProp(p, true), c.props(u.res, rest, typ, pctx{mode: c.mode, explicit: true, eff: eff})...)
	_, hx := p["x"]
	_, hy := p["y"]
	if hx || hy {
		x, _ := eff["x"].(float64)
		y, _ := eff["y"].(float64)
		props = append(props, prop("pos", numArg(x), numArg(-y)))
	}
	if len(props) > 0 && e != base {
		ops = append(ops, &Ovr{Kind: OvAt, Chain: chainOf(e, base), Props: props})
	} else if len(props) > 0 {
		u.Props = setProps(u.Props, props)
	}
	return ops
}

// setProps replaces the properties of the same name, or appends.
func setProps(old, with []Prop) []Prop {
	out := append([]Prop(nil), old...)
	for _, w := range with {
		done := false
		for i := range out {
			if out[i].Name == w.Name {
				out[i], done = w, true
			}
		}
		if !done {
			out = append(out, w)
		}
	}
	return out
}

// rootItem is the item an instance chain ends in.
func rootItem(n *Node) *Node {
	for n != nil && n.Kind == KUse {
		n = n.Target
	}
	return n
}

// fillOps puts new children into e: a slot when it has an open one, else in the place of the
// children it has. A frame with none gets a slot for them.
func (c *conv) fillOps(u *Node, base, e *enode, kids []any) []*Ovr {
	b := bctx{mode: c.mode, parentAbs: effPen(e.o)["layout"] == "none", noIndex: true, res: u.res, layout: e.o.Typ == "frame"}
	var nodes []*Node
	for _, k := range kids {
		km, ok := k.(map[string]any)
		if !ok {
			continue
		}
		if n := c.buildNode(u.File, km, u, b); n != nil {
			c.finishTree(n)
			u.Content = append(u.Content, n)
			nodes = append(nodes, n)
		}
	}
	for _, k := range e.kids {
		if k.open {
			return []*Ovr{c.slotFill(base, k, nodes)}
		}
	}
	if len(e.kids) == 0 {
		return c.slotFor(u, base, e, nodes)
	}
	if insideUse(e, base) {
		u.File.unsup("children replace the inlined content of a nested instance")
	}
	var ops []*Ovr
	for i, k := range e.kids {
		op := &Ovr{Kind: OvReplace, Chain: chainOf(k, base)}
		if i == 0 {
			op.Nodes = nodes
		}
		ops = append(ops, op)
	}
	return ops
}

// slotFill fills a slot. go/scene fills the slots of a nested instance only from inside it, so the
// fill goes into an at of that instance.
func (c *conv) slotFill(base, slot *enode, nodes []*Node) *Ovr {
	owner := slot.parent
	for owner != base && owner.o.Kind != KUse {
		owner = owner.parent
	}
	op := &Ovr{Kind: OvSlot, Slot: slot.o, Nodes: nodes}
	if owner != base {
		op.Chain = chainOf(owner, base)
	}
	return op
}

// slotFor adds a slot to the component frame that e comes from and fills it.
func (c *conv) slotFor(u *Node, base, e *enode, nodes []*Node) []*Ovr {
	item := e.o
	if e == base {
		item = rootItem(u.Target)
	}
	if len(nodes) == 0 {
		return nil
	}
	if item == nil || item.Kind != KItem || insideUse(e, base) {
		u.File.unsup("children given to an empty frame of a nested instance")
		return nil
	}
	s := &Node{Kind: KSlot, File: item.File, res: item.res, Parent: item}
	s.Slot = c.slotName(item.File, item.Name)
	item.Kids = append(item.Kids, s)
	slot := &enode{o: s, open: true}
	e.add(slot)
	return []*Ovr{c.slotFill(base, slot, nodes)}
}

func insideUse(e, base *enode) bool {
	for p := e; p != nil && p != base; p = p.parent {
		if p.o.Kind == KUse {
			return true
		}
	}
	return false
}

// tagRes marks the children an instance brings, so that their refs are read in the file that wrote them
// even when they end up inside a copy of a node of a library.
func tagRes(v any, f *File) any {
	list, _ := v.([]any)
	out := make([]any, len(list))
	for i, e := range list {
		if m, ok := e.(map[string]any); ok {
			out[i] = mergeMaps(m, map[string]any{resKey: f})
		}
	}
	return out
}

const resKey = "$res"
