package penimport

import (
	"strings"
)

type conv struct {
	o     Options
	mode  string
	files map[string]*File
	order []*File // libraries before the files that import them
}

// bctx is what a node needs to know about its place.
type bctx struct {
	mode      string
	parentAbs bool // the parent has layout none: x and y matter
	top       bool
	noIndex   bool // node of a block, not of the file tree
	layout    bool // the parent is a frame: go/scene draws no name there
	res       *File
}

func newConv(o Options) *conv {
	return &conv{o: o, mode: "light", files: map[string]*File{}}
}

func off(m map[string]any) bool {
	en, ok := m["enabled"].(bool)
	return ok && !en
}

// hiddenProp is the hidden property for a pen enabled key; an override writes both values, a node only yes.
func hiddenProp(m map[string]any, explicit bool) []Prop {
	en, ok := m["enabled"].(bool)
	switch {
	case ok && !en:
		return []Prop{prop("hidden", wordArg("yes"))}
	case ok && explicit:
		return []Prop{prop("hidden", wordArg("no"))}
	}
	return nil
}

func penName(m map[string]any) string {
	typ, _ := m["type"].(string)
	id, _ := m["id"].(string)
	s, _ := m["name"].(string)
	s = strings.TrimSpace(strings.NewReplacer("\n", " ", "\r", " ").Replace(s))
	if s == "" {
		return typ + " " + id
	}
	return s
}

func mapMode(m map[string]any, mode string) string {
	th, _ := m["theme"].(map[string]any)
	for k, v := range th {
		if s, ok := v.(string); ok && strings.HasSuffix(k, "mode") {
			mode = s
		}
	}
	return mode
}

func posProps(m map[string]any, b bctx) []Prop {
	x, _ := m["x"].(float64)
	y, _ := m["y"].(float64)
	abs := !b.top && !b.parentAbs && m["layoutPosition"] == "absolute"
	var ps []Prop
	if abs {
		ps = append(ps, prop("absolute", wordArg("yes")))
	}
	if b.top || b.parentAbs || abs {
		if x != 0 || y != 0 {
			ps = append(ps, prop("pos", numArg(x), numArg(-y)))
		}
	}
	return ps
}

// buildNode converts one pen node; nil when the node is not part of the scene.
func (c *conv) buildNode(f *File, m map[string]any, parent *Node, b bctx) *Node {
	id, _ := m["id"].(string)
	if off(m) && (b.noIndex || b.top) {
		// nothing can switch it on again: a block node has no id to address, a screen is not drawn
		f.dropped[id] = true
		return nil
	}
	b.mode = mapMode(m, b.mode)
	if r, ok := m[resKey].(*File); ok {
		b.res = r
	}
	if b.res == nil {
		b.res = f
	}
	typ, _ := m["type"].(string)
	switch typ {
	case "frame", "rectangle", "ellipse", "path", "text":
		return c.buildItem(f, m, typ, parent, b)
	case "ref":
		n := &Node{Kind: KUse, File: f, res: b.res, ID: id, Typ: b.res.rootType(m), Pen: m, Parent: parent, Name: penName(m), b: b}
		n.ref, _ = m["ref"].(string)
		if !b.noIndex {
			f.index[id] = n
		}
		return n
	}
	f.unsup("type " + typ)
	return nil
}

func (c *conv) slotName(f *File, name string) string {
	base := normalize(name)
	if base == "" {
		base = "slot"
	}
	s := base
	for i := 2; f.slots[s]; i++ {
		s = base + "_" + strings.TrimSpace(fnum(float64(i)))
	}
	f.slots[s] = true
	return s
}

func (c *conv) buildItem(f *File, m map[string]any, typ string, parent *Node, b bctx) *Node {
	id, _ := m["id"].(string)
	n := &Node{Kind: KItem, File: f, res: b.res, ID: id, Typ: typ, Pen: m, Parent: parent, Name: penName(m), b: b}
	if !b.noIndex {
		f.index[id] = n
	}
	n.Props = append(hiddenProp(m, false), c.props(f, m, typ, pctx{mode: b.mode})...)
	n.Props = append(n.Props, posProps(m, b)...)
	if th := m["theme"]; th != nil && typ != "frame" {
		f.unsup("theme on a leaf")
	}
	cb := bctx{mode: b.mode, parentAbs: m["layout"] == "none", noIndex: b.noIndex, res: b.res, layout: typ == "frame"}
	holder := n
	if _, isSlot := m["slot"].([]any); isSlot && typ == "frame" {
		holder = &Node{Kind: KSlot, File: f, res: b.res, Parent: n, Slot: c.slotName(f, n.Name)}
		n.Kids = append(n.Kids, holder)
	}
	for _, km := range kids(m) {
		if k := c.buildNode(f, km, holder, cb); k != nil {
			holder.Kids = append(holder.Kids, k)
		}
	}
	return n
}

// buildFile makes the item tree of the file; refs are resolved later by finish.
func (c *conv) buildFile(f *File) {
	for i, m := range kids(f.raw) {
		if n := c.buildNode(f, m, f.Root, bctx{mode: c.mode, top: true}); n != nil {
			f.Root.Kids = append(f.Root.Kids, n)
			f.Tops = append(f.Tops, Top{Index: i, Node: n})
		}
	}
}
