package app

import (
	"sort"
	"strconv"
	"strings"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/scene"
	"github.com/haiodo/tenniarb/go/tenn"
)

// ItemRow is a row of the Items list: a node of a scene canvas (Path, as NodePath writes it) or an
// item of a diagram (Item).
type ItemRow struct {
	Label string
	Kind  string // item, use, slot; item or link on a diagram
	Path  string
	Item  *model.DiagramItem
	Kids  bool // the row opens; its rows are read when it does
}

// nodeLabel is how a node is named in a list: its name, the kind of an unnamed one.
func nodeLabel(n *scene.Node) string {
	switch {
	case n.Kind == scene.KSlot && n.Name == "":
		return "slot"
	case n.Kind == scene.KSlot:
		return "slot " + n.Name
	case n.Name == "":
		return n.Kind.String()
	}
	return n.Name
}

// SceneRows are the rows of the nodes under n; prefix is the path of n, "" for the canvas itself.
func SceneRows(n *scene.Node, prefix string) []ItemRow {
	rows, _ := sceneRows(n, prefix)
	return rows
}

func sceneRows(n *scene.Node, prefix string) ([]ItemRow, []*scene.Node) {
	kids := sceneKids(n)
	out := make([]ItemRow, 0, len(kids))
	seen := map[string]int{}
	for _, k := range kids {
		seg := k.Key
		if i := seen[k.Key]; i > 0 {
			seg += "#" + strconv.Itoa(i)
		}
		seen[k.Key]++
		if prefix != "" {
			seg = prefix + "." + seg
		}
		out = append(out, ItemRow{Label: nodeLabel(k), Kind: k.Kind.String(), Path: seg, Kids: len(sceneKids(k)) > 0})
	}
	return out, kids
}

// DiagramRows are the items of a diagram element and then its links.
func DiagramRows(e *model.Element) []ItemRow {
	var items, links []ItemRow
	for _, it := range e.Items {
		r := ItemRow{Label: it.Name, Kind: "item", Item: it}
		if it.IsLink() {
			r.Kind = "link"
			r.Label = linkEnd(it.Source) + " -> " + linkEnd(it.Target)
		} else if r.Label == "" {
			r.Label = "(no name)"
		}
		if r.Kind == "link" {
			links = append(links, r)
		} else {
			items = append(items, r)
		}
	}
	return append(items, links...)
}

func linkEnd(it *model.DiagramItem) string {
	if it == nil {
		return "?"
	}
	return it.Name
}

// Box is where a node is drawn, in the coordinates of the screen (y down).
type Box struct{ X, Y, W, H float64 }

func (a Box) overlaps(b Box) bool {
	return a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H
}

// FreeRight is the x of a copy of at placed right of it with gap, moved further right past every box
// of others it would come closer to than gap.
func FreeRight(at Box, others []Box, gap float64) float64 {
	c := at
	c.X = at.X + at.W + gap
	for moved := true; moved; {
		moved = false
		for _, o := range others {
			if c.overlaps(Box{o.X - gap, o.Y - gap, o.W + 2*gap, o.H + 2*gap}) {
				c.X = o.X + o.W + gap
				moved = true
			}
		}
	}
	return c.X
}

// DupName is the name of a copy: "Screen" gives "Screen 2", "Screen 2" gives "Screen 3", the first
// whose key is not taken. An unnamed node stays unnamed.
func DupName(name string, taken map[string]bool) string {
	if name == "" {
		return ""
	}
	base, n := name, 2
	if i := strings.LastIndexByte(name, ' '); i > 0 {
		if k, err := strconv.Atoi(name[i+1:]); err == nil && k > 0 {
			base, n = name[:i], k+1
		}
	}
	for ; ; n++ {
		if c := base + " " + strconv.Itoa(n); !taken[scene.NormalizeName(c)] {
			return c
		}
	}
}

// takenKeys are the keys of what is written in n, as FreeName counts them.
func takenKeys(n *scene.Node) map[string]bool {
	taken := map[string]bool{}
	for _, c := range n.Nodes() {
		taken[c.Key] = true
	}
	for _, c := range n.Content {
		taken[c.Key] = true
	}
	return taken
}

// DupSource is a node to duplicate and where it is drawn (At nil: not drawn). Free: the place comes
// from pos, Pos is the pos in effect. Others are the drawn boxes of its siblings.
type DupSource struct {
	Node   *scene.Node
	At     *Box
	Free   bool
	Pos    [2]float64
	Others []Box
}

// Gaps between a copy and its original: screens on a canvas, items inside a frame.
const (
	topGap    = 100
	nestedGap = 16
)

// DuplicateEdit writes a renamed copy after every source as one edit, a free one moved right clear of
// its siblings; starts are where the copies begin in the new text.
func DuplicateEdit(text string, srcs []DupSource) (e TextEdit, starts []int, ok bool) {
	srcs = outermost(srcs)
	if len(srcs) == 0 {
		return TextEdit{}, nil, false
	}
	sort.Slice(srcs, func(i, j int) bool { return srcs[i].Node.Range.Start < srcs[j].Node.Range.Start })
	taken := map[*scene.Node]map[string]bool{}
	placed := map[*scene.Node][]Box{}
	from, to := srcs[0].Node.Range.End, srcs[len(srcs)-1].Node.Range.End
	var b strings.Builder
	at := from
	for _, s := range srcs {
		n := s.Node
		parent := n.Parent
		if taken[parent] == nil {
			taken[parent] = takenKeys(parent)
		}
		name := DupName(n.Name, taken[parent])
		if name != "" {
			taken[parent][scene.NormalizeName(name)] = true
		}
		pos := []string(nil)
		if s.At != nil && s.Free {
			gap := float64(nestedGap)
			if parent.Kind == scene.KFile || parent.Kind == scene.KElement {
				gap = topGap
			}
			x := FreeRight(*s.At, append(s.Others, placed[parent]...), gap)
			c := *s.At
			c.X = x
			placed[parent] = append(placed[parent], c)
			pos = []string{numText(s.Pos[0] + x - s.At.X), numText(s.Pos[1])}
		}
		cp, good := copyText(text, n, name, pos)
		if !good {
			return TextEdit{}, nil, false
		}
		b.WriteString(text[at:n.Range.End])
		ins := "\n" + indentAt(text, n.Range.Start)
		starts = append(starts, from+b.Len()+len(ins))
		b.WriteString(ins + cp)
		at = n.Range.End
	}
	b.WriteString(text[at:to])
	return TextEdit{from, to, b.String()}, starts, true
}

// outermost drops the sources that lie inside another source.
func outermost(srcs []DupSource) []DupSource {
	var out []DupSource
	for _, s := range srcs {
		inside := false
		for _, o := range srcs {
			if o.Node != s.Node && o.Node.Range.Start <= s.Node.Range.Start && s.Node.Range.End <= o.Node.Range.End {
				inside = true
			}
		}
		if !inside {
			out = append(out, s)
		}
	}
	return out
}

// copyText is the source of n with its name replaced by name and, with pos, its pos set to it.
func copyText(text string, n *scene.Node, name string, pos []string) (string, bool) {
	start := n.Range.Start
	src := text[start:n.Range.End]
	if pos != nil {
		e, ok := SetPropEdit(text, n, "pos", pos...)
		if !ok || e.Start < start || e.End > n.Range.End {
			return "", false
		}
		src = src[:e.Start-start] + e.New + src[e.End-start:]
	}
	if name == "" {
		return src, true
	}
	a, b, ok := nameSpan(src, n.Kind)
	if !ok {
		return "", false
	}
	return src[:a] + strings.TrimPrefix(CommandText("x", name), "x ") + src[b:], true
}

// nameSpan is where the name is written in the command src: after item, after the as of a use.
func nameSpan(src string, kind scene.Kind) (int, int, bool) {
	ast, p := tenn.Parse(src)
	if p.Errors.HasErrors() || len(ast.Children) == 0 {
		return 0, 0, false
	}
	args := ast.Children[0].Children
	i := 1
	if kind == scene.KUse {
		i = 3
		if len(args) < 4 || args[2].Token == nil || args[2].Token.Literal != "as" {
			return 0, 0, false
		}
	}
	if i >= len(args) || args[i].Kind == tenn.KindBlockExpr || args[i].Token == nil {
		return 0, 0, false
	}
	tok := args[i].Token
	a := tok.Pos
	if args[i].Kind == tenn.KindStringLit && a > 0 && src[a-1] == '"' {
		a--
	}
	return a, tok.EndPos(), true
}

// InsertTarget is where Insert Component puts a component while sel is selected: an item, an instance
// (slots to choose from, "" the default one), or nil for a free spot on the canvas.
func InsertTarget(t *scene.Tree, sel *scene.Node) (into *scene.Node, slots []string) {
	if sel == nil || t == nil {
		return nil, nil
	}
	switch sel.Kind {
	case scene.KItem:
		return sel, nil
	case scene.KUse:
		comp, err := t.Resolve(sel.Parent, sel.Path)
		if err != nil {
			return nil, nil
		}
		def, named := componentSlots(comp)
		switch {
		case len(named) > 0 && def:
			return sel, append([]string{""}, named...)
		case len(named) > 0:
			return sel, named
		case def:
			return sel, nil
		}
	}
	return nil, nil
}

// componentSlots are the slots a component defines, not those of the instances inside it. Ceiling: a
// component that is itself a use reports none; its open slots need the expanded instance.
func componentSlots(comp *scene.Node) (def bool, named []string) {
	var walk func(n *scene.Node)
	walk = func(n *scene.Node) {
		for _, c := range n.Content {
			switch c.Kind {
			case scene.KSlot:
				if c.Name == "" {
					def = true
				} else {
					named = append(named, c.Name)
				}
				walk(c)
			case scene.KItem:
				walk(c)
			}
		}
	}
	if comp.Kind == scene.KItem {
		walk(comp)
	}
	return def, named
}

// SlotCommand wraps cmd into the fill of a named slot; the default slot takes cmd as it is.
func SlotCommand(slot, cmd string) string {
	if slot == "" {
		return cmd
	}
	return CommandText("slot", slot) + " { " + cmd + " }"
}

// UsePath is the path a use of n writes from the file that mounts n's file as the element alias. False when n
// cannot be used: it is not an item or a use, or it lies inside an instance or a slot.
func UsePath(alias string, n *scene.Node) (string, bool) {
	if n == nil || n.Kind != scene.KItem && n.Kind != scene.KUse {
		return "", false
	}
	segs := []string{n.Key}
	for p := n.Parent; p != nil && p.Kind != scene.KFile; p = p.Parent {
		if p.Kind != scene.KItem && p.Kind != scene.KElement {
			return "", false
		}
		segs = append(segs, p.Key)
	}
	for i, j := 0, len(segs)-1; i < j; i, j = i+1, j-1 {
		segs[i], segs[j] = segs[j], segs[i]
	}
	return alias + "." + strings.Join(segs, "."), true
}

// Component is a node of a library one can insert: the label the picker shows and the use path.
type Component struct {
	Label, Path, Name string
}

// Components lists what a library file offers, depth first in file order, those whose name or key
// holds q (case folded): at most limit.
func Components(alias string, root *scene.Node, q string, limit int) []Component {
	q = strings.ToLower(q)
	var out []Component
	var walk func(n *scene.Node, where string)
	walk = func(n *scene.Node, where string) {
		for _, k := range n.Nodes() {
			if len(out) >= limit {
				return
			}
			if k.Kind == scene.KElement {
				walk(k, k.Name)
				continue
			}
			hit := strings.Contains(strings.ToLower(k.Name), q) || strings.Contains(strings.ToLower(k.Key), q)
			if p, ok := UsePath(alias, k); ok && k.Name != "" && hit {
				label := k.Name
				if where != "" {
					label += " - " + where
				}
				out = append(out, Component{Label: label, Path: p, Name: k.Name})
			}
			if k.Kind == scene.KItem {
				w := where
				if w == "" {
					w = k.Name
				}
				walk(k, w)
			}
		}
	}
	walk(root, "")
	return out
}
