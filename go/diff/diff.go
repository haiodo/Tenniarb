// Package diff compares two texts of one .tenn file node by node: elements, items, links and uses.
// A node is matched between the versions by its path (the names from the file root down); a node that
// changed parent is matched by its name. It also reads a version of a file from git, for a comparison
// against the last commit.
package diff

import (
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/haiodo/tenn/go"
	"github.com/haiodo/tenniarb/go/scene"
)

// Status is what happened to a node between the two versions.
type Status string

const (
	Added   Status = "added"
	Removed Status = "removed"
	Changed Status = "changed"
	Moved   Status = "moved"
)

// Prop is one property that differs: its value in the old version and in the new one. An empty side
// means the property is not there.
type Prop struct{ Name, Old, New string }

// Node is one element, item, link or use of the comparison tree. A node with no status is the same in
// both versions, and is kept only to hold the nodes under it.
type Node struct {
	Path   string // elements joined by "/", then nodes by "."; "" for the file root
	Kind   string // file, element, item, link, use, slot
	Name   string // as written in the file, for the row
	Status Status // "" when unchanged
	Props  []Prop // what changed, for Changed
	Note   string // where a moved node came from, or why it moved
	// Element is the element that holds the node (its own path for an element row); Sel is the path
	// inside it as NodePath writes it, "" for an element.
	Element string
	Sel     string
	Kids    []*Node

	old, new *scene.Node // the node in each version; nil on the side it is not written
	parent   string      // the path of the parent
}

// Report is the comparison of two texts of one file.
type Report struct {
	Root     *Node
	Old, New string // the texts compared
	byPath   map[string]*Node
	oldEntry map[string]*entry
	newEntry map[string]*entry
	counts   map[Status]int
}

// Node finds a node of the report by its path.
func (r *Report) Node(path string) *Node { return r.byPath[path] }

// Count is how many nodes have status s.
func (r *Report) Count(s Status) int { return r.counts[s] }

// Changed reports whether the two versions differ in any node or element.
func (r *Report) Changed() bool {
	for _, s := range []Status{Added, Removed, Changed, Moved} {
		if r.counts[s] > 0 {
			return true
		}
	}
	return false
}

// Paths lists the paths of the nodes with status s, in file order.
func (r *Report) Paths(s Status) []string {
	var out []string
	var walk func(n *Node)
	walk = func(n *Node) {
		if n.Status == s {
			out = append(out, n.Path)
		}
		for _, k := range n.Kids {
			walk(k)
		}
	}
	walk(r.Root)
	return out
}

// Edit replaces Text[Start:End) by New; offsets are bytes of the new text. It is the edit that takes a
// node back to the old version.
type Edit struct {
	Start, End int
	New        string
}

// Compare reads oldText and newText as the file rel under the workspace root root and compares them.
func Compare(root, rel, oldText, newText string) (*Report, error) {
	oldRoot, err := load(root, rel, oldText)
	if err != nil {
		return nil, fmt.Errorf("the old version: %w", err)
	}
	newRoot, err := load(root, rel, newText)
	if err != nil {
		return nil, fmt.Errorf("the new version: %w", err)
	}
	r := &Report{Old: oldText, New: newText, byPath: map[string]*Node{},
		oldEntry: map[string]*entry{}, newEntry: map[string]*entry{}, counts: map[Status]int{}}
	oldE := &entry{path: "", kind: "file", name: baseName(rel), node: oldRoot}
	newE := &entry{path: "", kind: "file", name: baseName(rel), node: newRoot}
	oldE.kids = collect(oldRoot, "", "")
	newE.kids = collect(newRoot, "", "")
	r.index(r.oldEntry, oldE)
	r.index(r.newEntry, newE)
	b := &builder{r: r, movedFrom: matchMoved(oldE, newE)}
	r.Root = b.node(oldE, newE)
	r.Root.Kids = b.kids(oldE.kids, newE.kids)
	r.count(r.Root)
	return r, nil
}

func baseName(rel string) string {
	return strings.TrimSuffix(path.Base(filepath.ToSlash(rel)), path.Ext(rel))
}

func load(root, rel, text string) (*scene.Node, error) {
	tree, err := scene.LoadSource(root, rel, text)
	if tree == nil {
		return nil, err
	}
	files := tree.Files()
	if len(files) == 0 {
		return nil, fmt.Errorf("%s is not part of the tree", rel)
	}
	f := tree.File(files[0])
	if f == nil {
		return nil, fmt.Errorf("%s is not part of the tree", rel)
	}
	return f, nil
}

// entry is one node of one version: what the comparison needs about it.
type entry struct {
	path, parent, elem, sel, kind, name, key string
	node                                     *scene.Node
	kids                                     []*entry
	values                                   map[string]string
}

// collect lists what container holds in file order: its elements, then the items, uses and links.
// elemPath is the element that holds them ("" at the file root); nodePath is the path of container.
func collect(c *scene.Node, elemPath, nodePath string) []*entry {
	type at struct {
		pos int
		e   *entry
	}
	var all []at
	for _, el := range c.Elems {
		seg := segOf(c.Elems, el, false)
		p := seg
		if elemPath != "" {
			p = elemPath + "/" + seg
		}
		e := &entry{path: p, parent: nodePath, elem: p, kind: "element", name: el.Name, key: el.Name, node: el}
		e.kids = collect(el, p, p)
		e.values = values(el)
		all = append(all, at{el.Range.Start, e})
	}
	for _, k := range c.Content {
		e := nodeEntry(c, k, elemPath, nodePath)
		if e == nil {
			continue
		}
		all = append(all, at{k.Range.Start, e})
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].pos < all[j].pos })
	out := make([]*entry, 0, len(all))
	for _, a := range all {
		out = append(out, a.e)
	}
	return out
}

// nodeEntry is an item, use, link or slot of container as an entry; nil for a command that is no node.
func nodeEntry(c *scene.Node, k *scene.Node, elemPath, nodePath string) *entry {
	switch k.Kind {
	case scene.KItem, scene.KUse, scene.KSlot, scene.KLink:
	default:
		return nil
	}
	e := &entry{kind: k.Kind.String(), parent: nodePath, elem: elemPath, node: k, values: values(k)}
	switch k.Kind {
	case scene.KLink:
		e.name = k.From + " -> " + k.To
		e.key = "link:" + k.From + "->" + k.To
		e.sel = e.key
	case scene.KSlot:
		e.name = "slot " + k.Name
		e.key = "slot:" + k.Key
		e.sel = e.key
	default:
		e.name = k.Name
		e.key = k.Key
		e.sel = segOf(sceneKids(c), k, true)
	}
	e.path = e.sel
	if nodePath != "" {
		e.path = nodePath + "." + e.sel
	}
	e.kids = collect(k, elemPath, e.path)
	return e
}

// segOf is the segment of n among siblings: its name or key, with #n for a repeat.
func segOf(siblings []*scene.Node, n *scene.Node, key bool) string {
	base := n.Name
	if key {
		base = n.Key
	}
	k := 0
	for _, c := range siblings {
		if c == n {
			break
		}
		if key && c.Key == n.Key || !key && c.Name == n.Name {
			k++
		}
	}
	if k > 0 {
		return base + "#" + fmt.Sprint(k)
	}
	return base
}

// sceneKids are the nodes a path goes through under n: its items, uses and slots.
func sceneKids(n *scene.Node) []*scene.Node {
	var out []*scene.Node
	for _, c := range n.Content {
		switch c.Kind {
		case scene.KItem, scene.KUse, scene.KSlot:
			out = append(out, c)
		}
	}
	return out
}

// values are the properties of n as the file writes them, by name. A repeated property keeps every
// value; styles, the kind, parameters, the template and the link ends count as properties too.
func values(n *scene.Node) map[string]string {
	m := map[string]string{}
	add := func(name, v string) {
		if old, ok := m[name]; ok {
			v = old + " " + v
		}
		m[name] = v
	}
	switch n.Kind {
	case scene.KElement:
		if n.Type != "" {
			add("kind", n.Type)
		}
		if n.Mount != nil {
			add("mount", n.Mount.Pos.File)
		}
		for _, p := range n.Params {
			v := "(required)"
			if p.Default != nil {
				v = argsText(p.Default.Args)
			}
			add("param "+p.Name, v)
		}
		if n.Template != nil {
			add("template", n.Template.Text)
		}
	case scene.KUse:
		add("use", n.Path)
		for _, ov := range n.Block {
			switch ov.Kind {
			case scene.OvProp:
				add(ov.Prop.Name, argsText(ov.Prop.Args))
			case scene.OvAt, scene.OvReplace:
				add("at "+ov.Path, "")
			}
		}
	case scene.KLink:
		add("link", n.From+" -> "+n.To)
	}
	for _, p := range n.Props {
		add(p.Name, argsText(p.Args))
	}
	var names []string
	for name := range n.Styles {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		add("style "+name, styleText(n.Styles[name]))
	}
	return m
}

func argsText(args []*tenn.Node) string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		s, _ := a.GetIdentText()
		out = append(out, s)
	}
	return strings.Join(out, " ")
}

// styleText is one style as the file writes it, for a comparison.
func styleText(st *scene.Style) string {
	var b strings.Builder
	for _, p := range st.Props {
		b.WriteString(p.Name)
		if t := argsText(p.Args); t != "" {
			b.WriteString(" " + t)
		}
		b.WriteString("; ")
	}
	for _, k := range st.Kids {
		b.WriteString(k.Key + " { " + styleText(k.Style) + "} ")
	}
	return strings.TrimSpace(b.String())
}

// index records the nodes of one version by path.
func (r *Report) index(into map[string]*entry, e *entry) {
	into[e.path] = e
	for _, k := range e.kids {
		r.index(into, k)
	}
}

// matchMoved pairs the nodes that have no counterpart by path with one of the other version by name:
// a node that changed parent. It returns the old node of each new node so paired.
func matchMoved(oldRoot, newRoot *entry) map[*entry]*entry {
	oldByPath, newByPath := map[string]*entry{}, map[string]*entry{}
	walk(oldRoot, func(e *entry) { oldByPath[e.path] = e })
	walk(newRoot, func(e *entry) { newByPath[e.path] = e })
	byKey := func(root *entry, matched func(*entry) bool) map[string][]*entry {
		out := map[string][]*entry{}
		walk(root, func(e *entry) {
			if matched(e) || e.key == "" {
				return
			}
			k := e.kind + "\x00" + e.key
			out[k] = append(out[k], e)
		})
		return out
	}
	oldLeft := byKey(oldRoot, func(e *entry) bool { return newByPath[e.path] != nil })
	newLeft := byKey(newRoot, func(e *entry) bool { return oldByPath[e.path] != nil })
	out := map[*entry]*entry{}
	for k, ns := range newLeft {
		if os := oldLeft[k]; len(ns) == 1 && len(os) == 1 {
			out[ns[0]] = os[0]
		}
	}
	return out
}

func walk(e *entry, f func(*entry)) {
	f(e)
	for _, k := range e.kids {
		walk(k, f)
	}
}

// builder turns two entry trees into the nodes of the report.
type builder struct {
	r         *Report
	movedFrom map[*entry]*entry
}

// node is the report node of an entry pair; either side may be nil.
func (b *builder) node(oldE, newE *entry) *Node {
	var e *entry
	switch {
	case newE != nil:
		e = newE
	case oldE != nil:
		e = oldE
	default:
		return nil
	}
	n := &Node{Path: e.path, Kind: e.kind, Name: e.name, Element: e.elem, Sel: e.sel, parent: e.parent,
		old: nodeOf(oldE), new: nodeOf(newE)}
	if oldE == nil {
		n.Status = Added
	} else if newE == nil {
		n.Status = Removed
	} else {
		if props := propChanges(oldE.values, newE.values); len(props) > 0 {
			n.Status, n.Props = Changed, props
		}
		if oldE.parent != newE.parent {
			n.Status = Moved
			n.Note = "from " + displayPath(oldE.parent)
		}
	}
	return n
}

// count registers the nodes of n, its own included, by status and by path.
func (r *Report) count(n *Node) {
	r.counts[n.Status]++
	r.byPath[n.Path] = n
	for _, k := range n.Kids {
		r.count(k)
	}
}

func nodeOf(e *entry) *scene.Node {
	if e == nil {
		return nil
	}
	return e.node
}

// kidsOf is the children of an entry, none when the other version has no such node.
func kidsOf(e *entry) []*entry {
	if e == nil {
		return nil
	}
	return e.kids
}

func displayPath(p string) string {
	if p == "" {
		return "the file"
	}
	return strings.ReplaceAll(p, "/", " / ")
}

// propChanges lists the properties that differ, by name.
func propChanges(oldV, newV map[string]string) []Prop {
	names := make([]string, 0, len(oldV)+len(newV))
	for k := range oldV {
		names = append(names, k)
	}
	for k := range newV {
		if _, ok := oldV[k]; !ok {
			names = append(names, k)
		}
	}
	sort.Strings(names)
	var out []Prop
	for _, k := range names {
		if oldV[k] != newV[k] {
			out = append(out, Prop{Name: k, Old: oldV[k], New: newV[k]})
		}
	}
	return out
}

// kids builds the rows of the children of a paired node: matched by path, then by name when the node
// moved, and the nodes the new version no longer has put back where they were.
func (b *builder) kids(oldKids, newKids []*entry) []*Node {
	oldByPath := map[string]*entry{}
	for _, k := range oldKids {
		oldByPath[k.path] = k
	}
	newByPath := map[string]*entry{}
	for _, k := range newKids {
		newByPath[k.path] = k
	}
	out := make([]*Node, 0, len(newKids))
	pairedOld := map[*entry]*Node{}
	var newSeq []*Node
	for _, nk := range newKids {
		ok := oldByPath[nk.path]
		if ok == nil {
			ok = b.movedFrom[nk]
			if ok != nil && newByPath[ok.path] != nil {
				ok = nil // another node has its old path now: not the same node
			}
		}
		n := b.node(ok, nk)
		if ok != nil {
			pairedOld[ok] = n
			newSeq = append(newSeq, n)
		}
		n.Kids = b.kids(kidsOf(ok), nk.kids)
		out = append(out, n)
	}
	for i, ok := range oldKids {
		if newByPath[ok.path] != nil || b.isMoved(ok) {
			continue
		}
		at := len(out)
		for _, next := range oldKids[i+1:] {
			if j := indexOf(out, next.path); j >= 0 {
				at = j
				break
			}
		}
		out = append(out, nil)
		copy(out[at+1:], out[at:])
		n := b.node(ok, nil)
		n.Kids = b.kids(ok.kids, nil)
		out[at] = n
	}
	// A node whose place among the siblings both versions hold changed has moved; an added or a removed
	// node around it does not move it.
	markOrder(oldKids, newSeq, pairedOld)
	return out
}

// markOrder flags the paired nodes whose order among the siblings changed: every node outside the
// longest run that kept its order has moved.
func markOrder(oldKids []*entry, newSeq []*Node, pairedOld map[*entry]*Node) {
	pos := make(map[*Node]int, len(oldKids))
	for i, ok := range oldKids {
		if n := pairedOld[ok]; n != nil {
			pos[n] = i
		}
	}
	idx := make([]int, len(newSeq))
	for i, n := range newSeq {
		p, ok := pos[n]
		if !ok {
			return
		}
		idx[i] = p
	}
	keep := longestIncreasing(idx)
	for i, n := range newSeq {
		if !keep[i] && n.Status == "" {
			n.Status = Moved
			n.Note = "moved among siblings"
		}
	}
}

// longestIncreasing marks one longest strictly increasing subsequence of a, by index.
func longestIncreasing(a []int) []bool {
	keep := make([]bool, len(a))
	if len(a) == 0 {
		return keep
	}
	prev := make([]int, len(a))
	var tails []int // indices into a: the smallest tail of a run of each length
	for i, v := range a {
		lo, hi := 0, len(tails)
		for lo < hi {
			mid := (lo + hi) / 2
			if a[tails[mid]] < v {
				lo = mid + 1
			} else {
				hi = mid
			}
		}
		prev[i] = -1
		if lo > 0 {
			prev[i] = tails[lo-1]
		}
		if lo == len(tails) {
			tails = append(tails, i)
		} else {
			tails[lo] = i
		}
	}
	for i := tails[len(tails)-1]; i >= 0; i = prev[i] {
		keep[i] = true
	}
	return keep
}

// isMoved reports whether old entry e is written in the new version under another path.
func (b *builder) isMoved(e *entry) bool {
	for _, o := range b.movedFrom {
		if o == e {
			return true
		}
	}
	return false
}

func indexOf(ns []*Node, path string) int {
	for i, n := range ns {
		if n != nil && n.Path == path {
			return i
		}
	}
	return -1
}

// Revert is the edit that writes the node at path as the old version has it: it replaces the node,
// deletes one added since, or writes a removed one back where it was. One edit, one undo step.
func (r *Report) Revert(path string) (Edit, bool) {
	n := r.byPath[path]
	switch {
	case n == nil:
		return Edit{}, false
	case n.old == nil && n.new != nil:
		return deleteEdit(r.New, n.new), true
	case n.old != nil && n.new == nil:
		return r.insertOld(n)
	case n.old != nil && n.new != nil:
		return Edit{n.new.Range.Start, n.new.Range.End, r.Old[n.old.Range.Start:n.old.Range.End]}, true
	}
	return Edit{}, false
}

// insertOld writes a node the new version no longer has back into its old place.
func (r *Report) insertOld(n *Node) (Edit, bool) {
	parent := r.newEntry[n.parent]
	if parent == nil {
		return Edit{}, false // the parent is gone too: take it back first
	}
	src := r.Old[n.old.Range.Start:n.old.Range.End]
	oldParent := r.oldEntry[n.parent]
	if oldParent != nil {
		for i, k := range oldParent.kids {
			if k.path != n.Path {
				continue
			}
			for j := i - 1; j >= 0; j-- {
				if prev := r.newEntry[oldParent.kids[j].path]; prev != nil {
					at := prev.node.Range.End
					return Edit{at, at, "\n" + indent(r.New, prev.node.Range.Start) + src}, true
				}
			}
			break
		}
	}
	if parent.path == "" { // a top-level element: at the end of the file
		at := len(r.New)
		return Edit{at, at, "\n" + src}, true
	}
	if b := parent.node.Body; b.Start > 0 && b.Start < len(r.New) {
		return Edit{b.Start + 1, b.Start + 1, "\n" + childIndent(r.New, parent) + src}, true
	}
	return Edit{}, false
}

// deleteEdit cuts the line a node is written on.
func deleteEdit(text string, n *scene.Node) Edit {
	start, end := n.Range.Start, n.Range.End
	if line := strings.LastIndexByte(text[:start], '\n') + 1; strings.TrimSpace(text[line:start]) == "" {
		start = line
	}
	if i := strings.IndexByte(text[end:], '\n'); i >= 0 && strings.TrimSpace(text[end:end+i]) == "" {
		end += i + 1
	}
	return Edit{start, end, ""}
}

// indent is the whitespace before the command at pos, "" when the line has other text before it.
func indent(text string, pos int) string {
	line := strings.LastIndexByte(text[:pos], '\n') + 1
	if lead := text[line:pos]; strings.TrimSpace(lead) == "" {
		return lead
	}
	return ""
}

// childIndent is the indentation a node added to parent gets: that of its first node, else one level in.
func childIndent(text string, parent *entry) string {
	for _, k := range parent.kids {
		if in := indent(text, k.node.Range.Start); in != "" {
			return in
		}
	}
	return indent(text, parent.node.Range.Start) + "  "
}
