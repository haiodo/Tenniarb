//go:build darwin || linux || windows

package app

import (
	"path"
	"path/filepath"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/icons"

	"github.com/haiodo/tenniarb/go/scene"
)

// libRow is an imported file in the outline: a canvas of its components.
type libRow struct {
	lib  string // the import alias, the start of a use path
	file string // relative to the workspace root
}

// libraries are the files the document imports, each once, in the order of the imports.
func (v *Viewer) libraries() []*libRow {
	f := v.file()
	if f == nil {
		return nil
	}
	var out []*libRow
	seen := map[*scene.Node]bool{}
	var mounts func(n *scene.Node, prefix string)
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
			out = append(out, &libRow{lib: alias, file: m.Target.Pos.File})
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
	return out
}

// libRows puts the libraries under a Libraries row after the elements of the document.
func (v *Viewer) libRows() {
	libs := v.libraries()
	if len(libs) == 0 {
		return
	}
	head := v.tree.Node("Libraries")
	for _, r := range libs {
		row := head.Node(r.lib + " - " + path.Base(r.file))
		row.SetData(r)
		row.SetImage(icons.Get(v.app, "open", icons.Size(16)))
	}
	head.SetExpanded(true)
}

// libNode is the root of the library on the canvas; nil when none is or it is gone.
func (v *Viewer) libNode() *scene.Node {
	if v.lib == nil || v.stree == nil {
		return nil
	}
	return v.stree.File(v.lib.file)
}

// showLib puts a library on the canvas, read only. The selection of the document is kept for the
// return, and for the insert of a component taken from it.
func (v *Viewer) showLib(r *libRow) {
	if v.lib != nil && v.lib.file == r.file {
		return
	}
	if v.lib == nil {
		v.saveCamera()
		v.libSaved = v.selNodes
	}
	v.endInline(false)
	v.lib = r
	v.selNodes = nil
	v.shownElem = nil
	v.realizeScene()
	v.refreshProps()
}

// leaveLib gives the canvas back to the document's element with its selection; the caller redraws.
func (v *Viewer) leaveLib() bool {
	if v.lib == nil {
		return false
	}
	v.lib = nil
	v.selNodes, v.libSaved = v.libSaved, nil
	// the camera on the canvas is the library's: it must not be remembered for the element
	v.shownElem, v.pendingFit = nil, true
	return true
}

// openLib opens the library in its own window, where it can be edited.
func (v *Viewer) openLib(r *libRow) {
	if v.stree != nil {
		v.sh.openPath(filepath.Join(v.stree.Root(), filepath.FromSlash(r.file)))
	}
}

// insertFromLib is Insert Component on a row of the library canvas: the document comes back on the
// canvas and gets a use of the node.
func (v *Viewer) insertFromLib(r *ItemRow) {
	lib := v.lib
	if lib == nil {
		return
	}
	p, ok := UsePath(lib.lib, NodeAt(v.libNode(), r.Path))
	if !ok {
		v.setBanner("Only an item of the library, not inside an instance or a slot, can be inserted.")
		return
	}
	name := NodeAt(v.libNode(), r.Path).Name
	v.leaveLib()
	v.realize()
	v.syncOutline()
	v.insertComponent(p, name)
}

// components are what the imported libraries offer for the query.
func (v *Viewer) components(q string, limit int) []Component {
	var out []Component
	for _, r := range v.libraries() {
		if n := v.stree.File(r.file); n != nil {
			out = append(out, Components(r.lib, n, q, limit-len(out))...)
		}
	}
	return out
}

// showComponentPicker is Insert Component...: a filtered list of the components of the libraries;
// Return or a double click inserts the one chosen.
func (v *Viewer) showComponentPicker() {
	if v.leaveLib() {
		v.realize()
		v.syncOutline()
	}
	if len(v.libraries()) == 0 {
		v.setBanner("The document imports no library.")
		return
	}
	v.closePopover()
	p := v.win.Popover(g.Gap(4), g.Padding(6))
	var field *g.TextView
	var list *g.ListView
	var shown []Component
	fill := func(q string) {
		shown = v.components(q, 200)
		labels := make([]string, len(shown))
		for i, c := range shown {
			labels[i] = c.Label
		}
		list.SetItems(labels)
		if len(labels) > 0 {
			list.Select(0)
		}
	}
	pick := func(i int) {
		if i < 0 || i >= len(shown) {
			return
		}
		c := shown[i]
		p.Close()
		v.insertComponent(c.Path, c.Name)
	}
	p.Add(
		g.Text(g.Search(), g.Focused(), g.Hint("Component"), g.ID("component-filter"), g.Ref(&field), g.Init(func(t *g.TextView) {
			t.OnChange(fill)
			t.OnActivate(func() { pick(list.Index()) })
		})),
		g.List(nil, g.Size(340, 280), g.ID("component-list"), g.Ref(&list), g.Init(func(l *g.ListView) {
			l.OnActivate(pick)
		})))
	fill("")
	v.openPopover(p)
	v.activeText = field.Unwrap()
	w, _ := v.view.Canvas().Size()
	p.ShowAtPoint(v.view.Canvas(), w/2, 0, g.PopoverBelow)
}
