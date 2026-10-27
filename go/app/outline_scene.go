//go:build darwin || linux || windows

package app

import (
	"path"
	"strings"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/icons"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/scene"
)

type rowKind int

const (
	rowNode rowKind = iota // a node of a scene element
	rowLib                 // an imported file
	rowComp                // a node of an imported file: a component
)

// outlineRow is a row that is not an element. Rows hold paths, not nodes: the tree is read anew after
// every change.
type outlineRow struct {
	kind rowKind
	el   *model.Element // rowNode: the element the node is in
	path string         // rowNode: node path in el; rowComp: names from the library root, dotted
	lib  string         // rowLib, rowComp: the import alias, the start of a use path
	file string         // rowLib, rowComp: the library, relative to the workspace root
}

func (r *outlineRow) key() string {
	if r.kind == rowNode {
		return pathKey(r.el) + "\x01" + r.path
	}
	return "\x02" + r.lib + "\x01" + r.path
}

// sceneRows adds the scene nodes of e under parent, and for the root the imported libraries.
func (v *Viewer) sceneRows(parent func(string) *g.Node, e *model.Element, expanded map[string]bool) {
	if !v.isScene(e) {
		return
	}
	el := v.elementNode(e)
	if el == nil {
		return
	}
	for _, k := range sceneKids(el) {
		v.addNodeRow(parent, &outlineRow{kind: rowNode, el: e, path: NodePath(el, k)}, k, expanded)
	}
}

func (v *Viewer) libRows(expanded map[string]bool) {
	f := v.file()
	if f == nil {
		return
	}
	var mounts func(n *scene.Node, prefix string)
	seen := map[*scene.Node]bool{}
	mounts = func(n *scene.Node, prefix string) {
		for _, m := range n.Mounts {
			alias := m.Name
			if prefix != "" {
				alias = prefix + "." + m.Name
			}
			if m.Kind == scene.KVirtual {
				mounts(m, alias)
				continue
			}
			if m.Target == nil || seen[m.Target] {
				continue
			}
			seen[m.Target] = true
			r := &outlineRow{kind: rowLib, lib: alias, file: m.Target.Pos.File}
			v.addNodeRow(v.tree.Node, r, m.Target, expanded)
		}
	}
	var walk func(n *scene.Node)
	walk = func(n *scene.Node) {
		mounts(n, "")
		for _, c := range n.Elems {
			walk(c)
		}
	}
	walk(f)
}

// rowKids are the nodes under the node of a row: scene kids, and the elements of a library.
func rowKids(r *outlineRow, n *scene.Node) []*scene.Node {
	if r.kind == rowNode {
		return sceneKids(n)
	}
	return n.Nodes()
}

func (v *Viewer) addNodeRow(parent func(string) *g.Node, r *outlineRow, n *scene.Node, expanded map[string]bool) {
	row := parent(rowLabel(r, n))
	row.SetData(r)
	row.SetImage(icons.Get(v.app, rowIcon(r, n), icons.Size(16)))
	if len(rowKids(r, n)) == 0 {
		return
	}
	if expanded[r.key()] {
		v.fillRow(row, r, n, expanded)
		row.SetExpanded(true)
		return
	}
	row.Node("") // the children are made when the row opens
}

func rowLabel(r *outlineRow, n *scene.Node) string {
	switch {
	case r.kind == rowLib:
		return r.lib + " - " + path.Base(r.file)
	case n.Kind == scene.KSlot && n.Name == "":
		return "slot"
	case n.Kind == scene.KSlot:
		return "slot " + n.Name
	case n.Name == "":
		return n.Kind.String()
	}
	return n.Name
}

func rowIcon(r *outlineRow, n *scene.Node) string {
	switch {
	case r.kind == rowLib:
		return "open"
	case n.Kind == scene.KUse:
		return "copy"
	case n.Kind == scene.KElement:
		return "folder"
	case r.kind == rowComp:
		return "star"
	case len(sceneKids(n)) > 0:
		return "menu"
	}
	return "file"
}

func (v *Viewer) fillRow(row *g.Node, r *outlineRow, n *scene.Node, expanded map[string]bool) {
	for _, c := range row.Children() {
		c.Remove()
	}
	kids := rowKids(r, n)
	for _, k := range kids {
		kr := &outlineRow{kind: r.kind, el: r.el, lib: r.lib, file: r.file}
		switch r.kind {
		case rowNode:
			kr.path = r.path + "." + nodeSeg(kids, k)
		default:
			kr.kind = rowComp
			kr.path = k.Key
			if r.path != "" {
				kr.path = r.path + "." + k.Key
			}
		}
		v.addNodeRow(row.Node, kr, k, expanded)
	}
}

// rowNode is the scene node of a row now; nil when it is gone.
func (v *Viewer) rowNodeOf(r *outlineRow) *scene.Node {
	switch r.kind {
	case rowNode:
		return NodeAt(v.elementNode(r.el), r.path)
	default:
		if v.stree == nil {
			return nil
		}
		n := v.stree.File(r.file)
		if r.path == "" || n == nil {
			return n
		}
		for _, seg := range strings.Split(r.path, ".") {
			var next *scene.Node
			for _, c := range n.Nodes() {
				if c.Key == seg {
					next = c
					break
				}
			}
			if next == nil {
				return nil
			}
			n = next
		}
		return n
	}
}

func (v *Viewer) onOutlineExpand(row *g.Node) {
	r, ok := row.Data().(*outlineRow)
	if !ok {
		return
	}
	if kids := row.Children(); len(kids) != 1 || kids[0].Data() != nil {
		return
	}
	if n := v.rowNodeOf(r); n != nil {
		v.fillRow(row, r, n, nil)
	}
}

// selectRow is a click on a row that is not an element: a node goes on the canvas selected, a
// component becomes what Insert Component adds.
func (v *Viewer) selectRow(r *outlineRow) {
	switch r.kind {
	case rowNode:
		if r.el != v.state.Current {
			v.selectElement(r.el)
		}
		if v.sceneMode() {
			v.selectScene([]string{r.path})
			v.revealScene()
		}
	case rowComp:
		v.picked = r
	}
}

// insertPicked inserts the component picked in the outline.
func (v *Viewer) insertPicked() {
	r := v.picked
	if r == nil || r.kind != rowComp {
		v.setBanner("Pick a component in a library of the sidebar first.")
		return
	}
	n := v.rowNodeOf(r)
	if n == nil {
		return
	}
	v.insertComponent(r.lib+"."+r.path, n.Name)
}

// syncSceneOutline selects the row of the selected node when it is made already.
func (v *Viewer) syncSceneOutline() {
	if len(v.selNodes) == 0 || !v.sceneMode() {
		return
	}
	want := pathKey(v.state.Current) + "\x01" + v.selNodes[0]
	var find func(ns []*g.Node) *g.Node
	find = func(ns []*g.Node) *g.Node {
		for _, n := range ns {
			if r, ok := n.Data().(*outlineRow); ok && r.kind == rowNode && r.key() == want {
				return n
			}
			if f := find(n.Children()); f != nil {
				return f
			}
		}
		return nil
	}
	if n := find(v.tree.Roots()); n != nil {
		v.syncing = true
		v.tree.Select(n)
		v.syncing = false
	}
}
