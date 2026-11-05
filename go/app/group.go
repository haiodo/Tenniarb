package app

import (
	"sort"
	"strings"

	"github.com/haiodo/tenniarb/go/scene"
)

// GroupItem is one node to wrap in a group: its pos in the parent (y up) and the size it draws.
type GroupItem struct {
	Node *scene.Node
	X, Y float64
	W, H float64
}

// GroupEdit wraps items, kids of one parent, in a new item named name, as one edit: the group takes the
// place of the first of them and holds them at their pos inside it, the rest of the span stays outside.
// at is where the group starts in the new text.
func GroupEdit(text string, items []GroupItem, name string) (e TextEdit, at int, ok bool) {
	if len(items) < 2 {
		return TextEdit{}, 0, false
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Node.Range.Start < items[j].Node.Range.Start })
	parent := items[0].Node.Parent
	for _, it := range items {
		if it.Node.Parent != parent || it.Node.Kind != scene.KItem && it.Node.Kind != scene.KUse {
			return TextEdit{}, 0, false
		}
	}
	minX, maxX := items[0].X, items[0].X+items[0].W
	top, bottom := items[0].Y, items[0].Y-items[0].H
	for _, it := range items[1:] {
		minX, maxX = min(minX, it.X), max(maxX, it.X+it.W)
		top, bottom = max(top, it.Y), min(bottom, it.Y-it.H)
	}
	first, last := items[0].Node, items[len(items)-1].Node
	ind, step := indentAt(text, first.Range.Start), indentUnit(text)
	head := CommandText("item", name) + " { " + CommandText("auto-layout", "abs") + "; " +
		CommandText("pos", numText(minX), numText(top)) + "; " +
		CommandText("width", numText(maxX-minX)) + "; " + CommandText("height", numText(top-bottom))
	var b strings.Builder
	b.WriteString(head)
	inside := map[*scene.Node]bool{}
	for _, it := range items {
		inside[it.Node] = true
		pos := []string{numText(it.X - minX), numText(it.Y - top)}
		src, good := copyText(text, it.Node, "", pos)
		if !good {
			return TextEdit{}, 0, false
		}
		b.WriteString("\n" + ind + step + reindent(src, step, false))
	}
	b.WriteString("\n" + ind + "}")
	// What lies between the wrapped items stays in the parent, after the group.
	for _, k := range sceneKids(parent) {
		if k.Range.Start < first.Range.Start || k.Range.End > last.Range.End || inside[k] {
			continue
		}
		b.WriteString("\n" + indentAt(text, k.Range.Start) + text[k.Range.Start:k.Range.End])
	}
	return TextEdit{first.Range.Start, last.Range.End, b.String()}, first.Range.Start, true
}

// UngroupEdit takes the kids of the group n out of it, as one edit, the group gone: each kid keeps its
// place, and the first of them starts at at in the new text.
func UngroupEdit(text string, n *scene.Node) (e TextEdit, at int, ok bool) {
	if n == nil || n.Kind != scene.KItem {
		return TextEdit{}, 0, false
	}
	kids := sceneKids(n)
	if len(kids) == 0 {
		return TextEdit{}, 0, false
	}
	ind, step := indentAt(text, n.Range.Start), indentUnit(text)
	dx, dy := 0.0, 0.0
	if pr := ownProp(n, "pos"); pr != nil {
		if v := pr.Values(); len(v) == 2 {
			dx, dy = parseNum(v[0]), parseNum(v[1])
		}
	}
	var b strings.Builder
	for i, k := range kids {
		src := text[k.Range.Start:k.Range.End]
		if k.Kind == scene.KItem {
			moved, good := copyText(text, k, "", []string{numText(posOf(k).X + dx), numText(posOf(k).Y + dy)})
			if good {
				src = moved
			}
		}
		if i > 0 {
			b.WriteString("\n" + ind)
		}
		b.WriteString(dedent(src, step))
	}
	return TextEdit{n.Range.Start, n.Range.End, b.String()}, n.Range.Start, true
}

// posOf is the pos a node sets, or zero: a node of a flow layout has none.
func posOf(n *scene.Node) struct{ X, Y float64 } {
	if pr := ownProp(n, "pos"); pr != nil {
		if v := pr.Values(); len(v) == 2 {
			return struct{ X, Y float64 }{parseNum(v[0]), parseNum(v[1])}
		}
	}
	return struct{ X, Y float64 }{}
}

// groupSelection is Cmd+G: the selected items of a scene become one item, which the canvas then moves,
// copies and deletes as a whole.
func (v *Viewer) groupSelection() {
	if !v.sceneMode() || v.sv == nil || v.sv.node == nil {
		v.setBanner("Grouping works in a screen element, not in a diagram.")
		return
	}
	items := v.groupTargets()
	if len(items) < 2 {
		v.setBanner("Select two or more items placed at pos to group them.")
		return
	}
	parent := items[0].Node.Parent
	e, at, ok := GroupEdit(v.fileText(v.fileOf(parent)), items, FreeName(parent, "Group"))
	if !ok || !v.sceneEdit(parent, e) {
		return
	}
	if n := nodeStartingAt(v.sv.node, at); n != nil {
		v.selectScene([]string{NodePath(v.sv.node, n)})
	}
}

// ungroupSelection is Cmd+Shift+G: the kids of the selected item take its place, one undo step.
func (v *Viewer) ungroupSelection() {
	if !v.sceneMode() || v.sv == nil || v.sv.node == nil {
		v.setBanner("Ungrouping works in a screen element, not in a diagram.")
		return
	}
	n, pl := v.selectedScene()
	if n == nil || pl == nil || n.Kind != scene.KItem || len(sceneKids(n)) == 0 || pl.Inst || v.inInstance() {
		v.setBanner("Select an item with items inside it to ungroup.")
		return
	}
	e, at, ok := UngroupEdit(v.fileText(v.fileOf(n)), n)
	if !ok || !v.sceneEdit(n, e) {
		return
	}
	if k := nodeStartingAt(v.sv.node, at); k != nil {
		v.selectScene([]string{NodePath(v.sv.node, k)})
	}
}

// inInstance reports whether the selection reaches into an instance: those nodes are not written in this
// file, so an edit cannot move them.
func (v *Viewer) inInstance() bool {
	for _, p := range v.selNodes {
		if strings.Contains(p, "/") {
			return true
		}
	}
	return false
}

// groupTargets are the selected items a group can take: written in the file, kids of one parent, placed
// at pos by that parent, with the box and the pos a group needs.
func (v *Viewer) groupTargets() []GroupItem {
	var out []GroupItem
	var parent *scene.Node
	for _, p := range v.selNodes {
		n := NodeAt(v.sv.node, p)
		pl := v.sv.byNode[n]
		if n == nil || pl == nil || !pl.Free || v.inInstance() {
			return nil
		}
		if n.Kind != scene.KItem && n.Kind != scene.KUse {
			return nil
		}
		if parent != nil && n.Parent != parent {
			return nil
		}
		parent = n.Parent
		pos := posOf(n)
		out = append(out, GroupItem{Node: n, X: pos.X, Y: pos.Y, W: pl.W, H: pl.H})
	}
	if len(out) > 0 && flowLayout(parent) {
		return nil
	}
	return out
}
