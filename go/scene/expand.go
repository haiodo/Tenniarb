package scene

import (
	"fmt"
	"strings"

	"golang.org/x/text/unicode/norm"
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
	replaced bool     // RSlot: declared by the nodes of a replace, which win over a slot of the same name
	From, To string   // RLink
	// Vars are the variables of the expressions of the node: the parameters of the component instance it
	// is in, by name, and the context in scope as ctx.* keys (spec 5c, 5f). Nil outside a component and
	// without context.
	Vars map[string]string
	// Def is the definition the root of an instance stands for: the item or element the use names, nil
	// elsewhere. The styles of the root come from there, not from the place the use is written in.
	Def *Node
	// Src is the command written for the node: the item, link or slot, the use for the root of an
	// instance, the element or file for the root of a screen.
	Src *Node
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
	depth int               // uses being expanded; slots stay open until the outermost one is done
	ctx   *mountCtx         // mount context of the file the node being expanded lives in
	rec   *refFinder        // set by References: sees the paths that resolve
	eval  evalFunc          // Options.EvalVars or Eval, for the values of parameters (spec 5c); nil: no expression runs
	vars  map[string]string // parameters of the instance being expanded
	env   map[string]string // context declarations in scope, key -> value (spec 5f)
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

func (x *expander) newR(kind RKind, n *Node) *RNode {
	return &RNode{Kind: kind, Name: n.Name, Key: n.Key, Pos: n.Pos, Props: append([]*Prop(nil), n.Props...), Src: n, Vars: x.scope()}
}

// container expands an element or a file: its content is the scene.
func (x *expander) container(n *Node, env map[string]string) *RNode {
	if !x.enter(n, n.Pos) {
		return nil
	}
	defer x.leave()
	if n.Mount != nil { // its content is the mounted file, which looks outward from this element
		saved := x.ctx
		x.ctx = saved.push(n)
		defer func() { x.ctx = saved }()
	}
	savedEnv := x.env
	if env != nil { // an instance: the declarations of the definition root are already in env
		x.env = env
	} else {
		x.env = x.context(n.Props, x.env)
	}
	defer func() { x.env = savedEnv }()
	r := x.newR(RItem, n)
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
		return x.use(n, false, nil, nil)
	case KLink:
		r := x.newR(RLink, n)
		r.Name, r.Key, r.From, r.To = "", "", n.From, n.To
		return r
	case KSlot:
		r := x.newR(RSlot, n)
		r.Slot = n.Key
		r.Kids = x.contents(n.Content)
		return r
	}
	return nil
}

// item expands an item written in place: a component opened by itself is an instance that sets nothing, so
// its body sees the defaults of its parameters (spec 5c).
func (x *expander) item(n *Node) *RNode {
	if len(n.Params) > 0 {
		caller := x.vars
		x.vars = x.defaults(n)
		defer func() { x.vars = caller }()
	}
	return x.itemBody(n, nil)
}

// itemBody expands the content of an item in the scope x.vars already holds: the use of a component item
// has set the values there, and they must not fall back to the defaults. env is the context of an
// instance, nil for an item written in place: then its own context commands declare it.
func (x *expander) itemBody(n *Node, env map[string]string) *RNode {
	if !x.enter(n, n.Pos) {
		return nil
	}
	defer x.leave()
	saved := x.env
	if env != nil {
		x.env = env
	} else {
		x.env = x.context(n.Props, x.env)
	}
	defer func() { x.env = saved }()
	r := x.newR(RItem, n)
	r.Kids = x.contents(n.Content)
	return r
}

// use expands an instance. As a target of another use (asTarget) its slots stay open for that use. env is
// the context of the use site (nil: x.env), overlay the block declarations of the uses whose target it
// is, weakest first (spec 5f).
func (x *expander) use(n *Node, asTarget bool, env map[string]string, overlay []ctxDecl) *RNode {
	if !x.enter(n, n.Pos) {
		return nil
	}
	defer x.leave()
	x.depth++
	defer func() { x.depth-- }()
	ctx := x.ctx
	target, tctx := x.t.resolve(n.Parent, ctx, n.Path, n.Pos)
	if target == nil {
		return nil
	}
	if x.rec != nil {
		x.rec.use(n, ctx)
	}
	// The values of the parameters are the scope of the block of the use, a default the scope of the
	// declaration; without parameters of its own the target reads the scope it is written in.
	caller, callerEnv := x.vars, x.env
	if env != nil {
		callerEnv = env
	}
	scope := x.scopeOf(caller, callerEnv)
	values, used := x.params(target, n, scope)
	if values != nil {
		scope = x.scopeOf(values, callerEnv) // the block reads the parameters of the instance it makes
	}
	// The order of the context is: the place of the use, the definition root, the block of the use
	// (spec 5f). A use as target keeps its own block below the block that inserts it, so the nested use
	// applies the two itself.
	block := x.ctxDecls(blockProps(n.Block), scope)
	var inst *RNode
	var instEnv map[string]string
	x.ctx = tctx // the component's names resolve in its own file, the block's in this one
	if values != nil {
		x.vars = values
	}
	if target.Kind == KUse {
		passed := make([]ctxDecl, 0, len(block)+len(overlay))
		passed = append(append(passed, block...), overlay...)
		inst = x.instance(target, n, callerEnv, passed)
	} else {
		instEnv = applyCtx(callerEnv, x.ctxDecls(target.Props, scope))
		instEnv = applyCtx(instEnv, block)
		instEnv = applyCtx(instEnv, overlay)
		instEnv = applyCtx(instEnv, x.fileDefaults(target, callerEnv))
		inst = x.instance(target, n, instEnv, nil)
	}
	x.ctx, x.vars = ctx, caller
	if inst == nil {
		return nil
	}
	inst.Def = target
	inst.Name, inst.Key, inst.Pos, inst.Inst, inst.Src = n.Name, n.Key, n.Pos, true, n
	inst.Props = withoutProp(inst.Props, "pos") // the component's pos is its place on the library canvas
	if instEnv == nil {
		instEnv = ctxEnv(inst.Vars)
	}
	savedEnv := x.env // the content of the slots and at reads the context of the instance it lands in
	x.env = instEnv
	x.apply(inst, n.Block, used)
	x.env = savedEnv
	if !asTarget && x.depth == 1 {
		finalize(inst)
	}
	return inst
}

func (x *expander) instance(target, use *Node, env map[string]string, overlay []ctxDecl) *RNode {
	switch target.Kind {
	case KItem:
		return x.itemBody(target, env)
	case KLink:
		return x.content(target)
	case KUse:
		return x.use(target, true, env, overlay)
	case KElement, KFile:
		return x.container(target, env)
	}
	x.t.fail(use.Pos, "path %s names a %s, not a node", use.Path, target.Kind)
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
	name     string
	pos      Pos
	nodes    []*RNode
	explicit bool // written as slot [name] { ... }: an empty block still replaces the fallback
}

// apply puts the block of a use onto the instance root: properties override, nodes go to the default
// slot, at and replace reach into the instance, slot fills a named slot. A command named as a parameter of
// the component is no property: used holds those overrides (spec 5c); an undeclared name is a property of
// the root, as before.
func (x *expander) apply(root *RNode, block []*Override, used map[*Override]bool) {
	var fills []*slotFill
	add := func(name string, pos Pos, nodes []*RNode, explicit bool) {
		for _, f := range fills {
			if f.name == name {
				f.nodes = append(f.nodes, nodes...)
				f.explicit = f.explicit || explicit
				return
			}
		}
		fills = append(fills, &slotFill{name, pos, nodes, explicit})
	}
	for _, ov := range block {
		switch ov.Kind {
		case OvProp:
			if !used[ov] {
				setProp(root, ov.Prop)
			}
		case OvNode:
			add("", ov.Pos, x.contents([]*Node{ov.Node}), false)
		case OvSlot:
			add(NormalizeName(ov.Slot), ov.Pos, x.nodes(ov.Block, false), true)
		case OvAt:
			if at, ok := x.find(root, ov.Path, ov.Pos); ok {
				if x.rec != nil {
					x.rec.at(ov, root, at.node())
				}
				x.apply(at.node(), ov.Block, nil)
			}
		case OvReplace:
			if at, ok := x.find(root, ov.Path, ov.Pos); ok {
				if x.rec != nil {
					x.rec.at(ov, root, at.node())
				}
				nodes := x.nodes(ov.Block, true)
				markReplaced(nodes)
				at.parent.Kids = splice(at.parent.Kids, at.i, nodes)
			}
		}
	}
	for _, f := range fills {
		x.fillSlot(root, f)
	}
}

// nodes expands the block of a slot or replace, which takes nodes only; slots too in a replace block,
// because a slot the replacement declares is filled from the outside as any other (spec 4.7a).
func (x *expander) nodes(block []*Override, slots bool) []*RNode {
	var out []*RNode
	for _, ov := range block {
		switch {
		case ov.Kind == OvNode:
			if r := x.content(ov.Node); r != nil {
				out = append(out, r)
			}
		case ov.Kind == OvSlot && slots:
			out = append(out, &RNode{Kind: RSlot, Name: ov.Slot, Pos: ov.Pos, Slot: NormalizeName(ov.Slot), Kids: x.nodes(ov.Block, false)})
		default:
			x.t.fail(ov.Pos, "only item, link and use are allowed here")
		}
	}
	return out
}

// markReplaced marks the slots the nodes of a replace declare: a fill reaches them before a slot of the
// same name the component declares elsewhere (spec 4.7a).
func markReplaced(nodes []*RNode) {
	for _, n := range nodes {
		if n.Kind == RSlot {
			n.replaced = true
		}
		markReplaced(n.Kids)
	}
}

func splice(kids []*RNode, i int, with []*RNode) []*RNode {
	out := make([]*RNode, 0, len(kids)-1+len(with))
	out = append(out, kids[:i]...)
	out = append(out, with...)
	return append(out, kids[i+1:]...)
}

func withoutProp(props []*Prop, name string) []*Prop {
	out := make([]*Prop, 0, len(props))
	for _, p := range props {
		if p.Name != name {
			out = append(out, p)
		}
	}
	return out
}

// setProp replaces the properties of that name, in the place of the first one, or appends.
// setProp puts p after the properties of the node: what the use site writes wins over the definition and
// over the styles of the component, which sit where their use-style is written (spec 5b).
func setProp(r *RNode, p *Prop) {
	r.Props = append(r.Props, p)
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
				if matchesSeg(k, seg) {
					locs = append(locs, loc{p, i})
				}
			}
		}
		if len(locs) == 0 {
			x.t.fail(pos, "path %s: nothing named %q inside the instance", path, seg.Name)
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
			parts[i] = QuoteName(rName(n)) + " (" + n.Pos.String() + ")"
		}
		x.t.fail(pos, "ambiguous path %s: candidates %s", path, strings.Join(parts, ", "))
		return loc{}, false
	}
	if nodes[0].Kind == RSlot {
		x.t.fail(pos, "path %s names a slot, not a node; fill it with slot %s { ... }", path, segs[len(segs)-1].Text())
		return loc{}, false
	}
	return locs[0], true
}

// matchesSeg reports whether a segment of an at or replace path names the drawn node k. A slot is
// named "default" when it has no name.
func matchesSeg(k *RNode, seg Segment) bool {
	switch k.Kind {
	case RItem:
		if seg.Quoted {
			return norm.NFC.String(seg.Name) == norm.NFC.String(k.Name)
		}
		return k.Key == seg.Key()
	case RSlot:
		if seg.Quoted {
			return NormalizeName(seg.Name) == k.Slot
		}
		return slotSegment(k) == seg.Key()
	}
	return false
}

// rName is the name a path or a message gives a drawn node; the unnamed slot is "default".
func rName(n *RNode) string {
	if n.Kind == RSlot && n.Slot == "" {
		return "default"
	}
	return n.Name
}

// slotSegment is the path segment of a slot.
func slotSegment(r *RNode) string {
	if r.Slot == "" {
		return "default"
	}
	return r.Slot
}

// findSlot is the slot a fill of name goes into: one the nodes of a replace declared wins over one of
// the component, then the first in file order.
func findSlot(r *RNode, name string) (loc, bool) {
	if l, ok := findSlotIn(r, name, true); ok {
		return l, true
	}
	return findSlotIn(r, name, false)
}

func findSlotIn(r *RNode, name string, replaced bool) (loc, bool) {
	for i, k := range r.Kids {
		if k.Kind == RSlot && k.Slot == name && k.replaced == replaced {
			return loc{r, i}, true
		}
		if k.Inst {
			continue
		}
		if l, ok := findSlotIn(k, name, replaced); ok {
			return l, true
		}
	}
	return loc{}, false
}

// forwardedSlot is a slot of a component nested in a wrapper, with the path a reader writes it with.
type forwardedSlot struct {
	loc
	path string
}

// forwardedSlots lists the slots named name that live in the components r uses, in file order: a wrapper
// passes a slot of a component it uses through by its name (spec 4.7a). The slots of r itself are not
// here, findSlot looks at them first; a slot of a replace wins over a plain one, as fillSlot prefers.
func forwardedSlots(r *RNode, name string) []forwardedSlot {
	if out := collectSlots(r, name, "", false, true, nil); len(out) > 0 {
		return out
	}
	return collectSlots(r, name, "", false, false, nil)
}

// collectSlots walks the children of r; inInstance tells that they belong to an instance, whose slots a
// fill of the wrapper may reach.
func collectSlots(r *RNode, name, prefix string, inInstance, replaced bool, out []forwardedSlot) []forwardedSlot {
	for i, k := range r.Kids {
		p := joinPath(prefix, nodeSegment(k))
		switch {
		case k.Kind == RSlot:
			if inInstance && k.Slot == name && k.replaced == replaced {
				out = append(out, forwardedSlot{loc{r, i}, joinPath(p, slotSegment(k))})
			}
		case k.Inst:
			out = collectSlots(k, name, p, true, replaced, out)
		default:
			out = collectSlots(k, name, p, inInstance, replaced, out)
		}
	}
	return out
}

// nodeSegment is the path segment of a node: its name, else the key the format writes.
func nodeSegment(n *RNode) string {
	if n.Name != "" {
		return n.Name
	}
	return n.Key
}

func joinPath(prefix, seg string) string {
	if prefix == "" {
		return seg
	}
	if seg == "" {
		return prefix
	}
	return prefix + "." + seg
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
	// Nodes of the block that failed leave no fill; only slot {} says the fallback goes.
	if f.name == "" && len(f.nodes) == 0 && !f.explicit {
		return
	}
	at, ok := findSlot(root, f.name)
	if !ok && f.name != "" {
		// A wrapper passes a slot of a component it uses through by its name (spec 4.7a).
		switch hits := forwardedSlots(root, f.name); len(hits) {
		case 1:
			at, ok = hits[0].loc, true
		case 0:
		default:
			paths := make([]string, len(hits))
			for i, h := range hits {
				paths[i] = h.path
			}
			x.t.fail(f.pos, "slot %q is in %d nested components: %s; give the wrapper a slot of its own",
				f.name, len(hits), strings.Join(paths, ", "))
			return
		}
	}
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
