//go:build darwin || linux || windows

package app

import (
	"path/filepath"
	"slices"
	"strings"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/icons"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/model"
)

// rebuildOutline fills the outline from the model: a row per element, every one a canvas, the root
// first when it holds content itself, then the imported libraries. expanded is the set of rows to open
// by path key; nil opens the first ExpandLevel+1 levels when the settings have auto expand, as Swift does.
func (v *Viewer) rebuildOutline(expanded map[string]bool) {
	v.syncing = true
	defer func() { v.syncing = false }()
	var add func(parent func(string) *g.Node, els []*model.Element, depth int)
	add = func(parent func(string) *g.Node, els []*model.Element, depth int) {
		for _, e := range els {
			n := parent(e.Name)
			n.SetData(e)
			n.SetImage(v.rowIcon(e))
			add(n.Node, e.Elements, depth+1)
			if len(e.Elements) == 0 {
				continue
			}
			if expanded == nil {
				n.SetExpanded(v.sh.prefs.AutoExpand && depth <= v.sh.prefs.ExpandLevel)
			} else {
				n.SetExpanded(expanded[pathKey(e)])
			}
		}
	}
	v.tree.Batch(func() {
		v.tree.Clear()
		if m := v.state.Doc.Model; m != nil {
			if v.hasContent(&m.Element) {
				n := v.tree.Node(v.rootLabel())
				n.SetData(&m.Element)
				n.SetImage(v.rowIcon(&m.Element))
			}
			add(v.tree.Node, m.Elements, 0)
			v.libRows()
		}
	})
	v.outlineSig = v.outlineKey()
	v.syncOutline()
}

// rootLabel names the row of the root element: the file without .tenn.
func (v *Viewer) rootLabel() string {
	if v.state.Doc.Path == "" {
		return "Untitled"
	}
	return strings.TrimSuffix(filepath.Base(v.state.Doc.Path), filepath.Ext(v.state.Doc.Path))
}

// hasContent reports whether e draws something itself: classic items or scene nodes.
func (v *Viewer) hasContent(e *model.Element) bool {
	if len(e.Items) > 0 {
		return true
	}
	if !v.isScene(e) {
		return false
	}
	n := v.elementNode(e)
	return n != nil && len(sceneKids(n)) > 0
}

// rowIcon is a document for a canvas that holds content, a folder for one that only groups elements.
func (v *Viewer) rowIcon(e *model.Element) *g.Image {
	name := "folder"
	if v.hasContent(e) {
		name = "file"
	}
	return icons.Get(v.app, name, icons.Size(16))
}

// outlineKey changes when the outline would look different: OutlineSignature, the icons of scene
// elements, the root row and the libraries.
func (v *Viewer) outlineKey() string {
	m := v.state.Doc.Model
	if m == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(OutlineSignature(&m.Element))
	if v.stree != nil {
		var walk func(e *model.Element)
		walk = func(e *model.Element) {
			if v.hasContent(e) {
				b.WriteByte('+')
			} else {
				b.WriteByte('-')
			}
			for _, c := range e.Elements {
				walk(c)
			}
		}
		walk(&m.Element)
		for _, r := range v.libraries() {
			b.WriteString("|" + r.lib + "=" + r.file)
		}
	} else if v.hasContent(&m.Element) {
		b.WriteByte('+')
	}
	return b.String()
}

// refreshOutline follows the model: the rows are rebuilt when they would look different.
func (v *Viewer) refreshOutline() {
	if v.state.Doc.Model == nil {
		return
	}
	if v.outlineKey() != v.outlineSig {
		v.rebuildOutline(v.expandedRows())
		return
	}
	v.syncOutline()
}

// expandedRows is the path keys of the outline rows that are open now.
func (v *Viewer) expandedRows() map[string]bool {
	out := map[string]bool{}
	var walk func(ns []*g.Node)
	walk = func(ns []*g.Node) {
		for _, n := range ns {
			if d, ok := n.Data().(*model.Element); ok && n.Expanded() {
				out[pathKey(d)] = true
			}
			walk(n.Children())
		}
	}
	walk(v.tree.Roots())
	return out
}

// syncOutline selects the row of the canvas: the library on it, else the current element, opening its
// ancestors. The root has a row only when it holds content.
func (v *Viewer) syncOutline() {
	v.syncing = true
	defer func() { v.syncing = false }()
	cur := v.state.Current
	is := func(n *g.Node) bool {
		if v.lib != nil {
			r, ok := n.Data().(*libRow)
			return ok && r.file == v.lib.file
		}
		return cur != nil && n.Data() == cur
	}
	if sel := v.tree.Selected(); len(sel) == 1 && is(sel[0]) {
		return
	}
	var find func(ns []*g.Node) []*g.Node
	find = func(ns []*g.Node) []*g.Node {
		for _, n := range ns {
			if is(n) {
				return []*g.Node{n}
			}
			if f := find(n.Children()); f != nil {
				return append(f, n)
			}
		}
		return nil
	}
	chain := find(v.tree.Roots())
	if chain == nil {
		v.tree.Deselect()
		return
	}
	for _, a := range chain[1:] {
		a.SetExpanded(true)
	}
	v.tree.Select(chain[0])
}

// nodeOf is the row of e.
func (v *Viewer) nodeOf(e *model.Element) *g.Node {
	var find func(ns []*g.Node) *g.Node
	find = func(ns []*g.Node) *g.Node {
		for _, n := range ns {
			if n.Data() == e {
				return n
			}
			if f := find(n.Children()); f != nil {
				return f
			}
		}
		return nil
	}
	return find(v.tree.Roots())
}

// addElement adds a child to parent (the document when nil or the root) and puts it on the canvas.
func (v *Viewer) addElement(parent *model.Element) {
	if v.session == nil {
		return
	}
	if parent != nil && parent.Parent == nil {
		parent = nil
	}
	if el := v.session.AddElement(parent); el != nil {
		v.session.SetElement(el)
	}
}

func (v *Viewer) duplicateElement() {
	if v.session == nil || v.state.Current.Parent == nil {
		return
	}
	if cp := v.session.DuplicateElement(v.state.Current); cp != nil {
		v.session.SetElement(cp)
	}
}

func (v *Viewer) removeElement() {
	if v.session != nil {
		v.session.RemoveElement(v.state.Current)
	}
}

// moveElement moves the current element by delta places among its siblings.
func (v *Viewer) moveElement(delta int) {
	e := v.state.Current
	if v.session == nil || e == nil || e.Parent == nil {
		return
	}
	i := slices.Index(e.Parent.Elements, e)
	j := i + delta
	if i < 0 || j < 0 || j >= len(e.Parent.Elements) {
		return
	}
	// MoveElement counts the target place before the element leaves its slot.
	at := j
	if delta > 0 {
		at = j + 1
	}
	v.session.MoveElement(e, e.Parent, at)
}

// renameNode is the activation of a row: an element is renamed, a library opens in its own window.
func (v *Viewer) renameNode(n *g.Node) {
	switch d := n.Data().(type) {
	case *model.Element:
		v.renameElement(d)
	case *libRow:
		v.openLib(d)
	}
}

// renameElement asks for the new name in a small box over the row.
func (v *Viewer) renameElement(e *model.Element) {
	n := v.nodeOf(e)
	if v.session == nil || e == nil || e.Parent == nil || n == nil {
		return
	}
	v.closePopover()
	p := v.win.Popover(g.Gap(4))
	var field *g.TextView
	p.Add(g.Text(g.Focused(), g.Ref(&field), g.Init(func(t *g.TextView) {
		t.SetText(e.Name)
		t.OnActivate(func() {
			name := t.Text()
			p.Close()
			v.session.RenameElement(e, name)
		})
	})))
	v.openPopover(p)
	v.activeText = field.Unwrap()
	b := n.Unwrap().GetBounds()
	p.ShowAtPoint(v.tree, int(b.X), int(b.Y+b.Height), g.PopoverBelow)
}

// outlineEntries is the context menu of the outline (Swift's, plus rename and moves); a library row
// opens its file.
func (v *Viewer) outlineEntries() []editor.Entry {
	if v.session == nil {
		return nil
	}
	if sel := v.tree.Selected(); len(sel) == 1 {
		if r, ok := sel[0].Data().(*libRow); ok {
			return []editor.Entry{{Label: "Open Library", Run: func() { v.openLib(r) }}}
		}
	}
	cur := v.state.Current
	return []editor.Entry{
		{Label: "New element", Run: func() { v.addElement(cur) }},
		{Label: "New top-level element", Run: func() { v.addElement(nil) }},
		{Label: "Rename", Run: func() { v.renameElement(cur) }},
		{Label: "Duplicate", Run: v.duplicateElement},
		{Sep: true},
		{Label: "Copy", Run: v.cmdCopy},
		{Label: "Paste", Run: v.cmdPaste},
		{Sep: true},
		{Label: "Move up", Run: func() { v.moveElement(-1) }},
		{Label: "Move down", Run: func() { v.moveElement(1) }},
		{Sep: true},
		{Label: "Delete", Run: v.removeElement},
	}
}

// findNode is the first row with the text.
func (v *Viewer) findNode(text string) *g.Node {
	var find func(ns []*g.Node) *g.Node
	find = func(ns []*g.Node) *g.Node {
		for _, n := range ns {
			if n.Text() == text {
				return n
			}
			if f := find(n.Children()); f != nil {
				return f
			}
		}
		return nil
	}
	return find(v.tree.Roots())
}
