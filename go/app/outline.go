//go:build darwin || linux || windows

package app

import (
	"slices"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/icons"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/model"
)

// rebuildOutline fills the outline from the model. expanded is the set of rows to open by path key;
// nil opens the first ExpandLevel+1 levels when the settings have auto expand, as Swift does.
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
			add(v.tree.Node, m.Elements, 0)
		}
	})
	v.outlineSig = ""
	if m := v.state.Doc.Model; m != nil {
		v.outlineSig = OutlineSignature(&m.Element)
	}
	v.syncOutline()
}

// rowIcon is a document for an element that holds items, a folder for one that only groups elements.
func (v *Viewer) rowIcon(e *model.Element) *g.Image {
	name := "folder"
	if len(e.Items) > 0 {
		name = "file"
	}
	return icons.Get(v.app, name, icons.Size(16))
}

// refreshOutline follows the model: the rows are rebuilt when an element was added, removed, renamed or moved.
func (v *Viewer) refreshOutline() {
	m := v.state.Doc.Model
	if m == nil {
		return
	}
	if sig := OutlineSignature(&m.Element); sig != v.outlineSig {
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
			if e, ok := n.Data().(*model.Element); ok && n.Expanded() {
				out[pathKey(e)] = true
			}
			walk(n.Children())
		}
	}
	walk(v.tree.Roots())
	return out
}

// syncOutline selects the row of the current element, opening its ancestors; the root has none.
func (v *Viewer) syncOutline() {
	v.syncing = true
	defer func() { v.syncing = false }()
	cur := v.state.Current
	if cur == nil || cur.Parent == nil {
		v.tree.Deselect()
		return
	}
	if sel := v.tree.Selected(); len(sel) == 1 && sel[0].Data() == cur {
		return
	}
	var find func(ns []*g.Node) []*g.Node
	find = func(ns []*g.Node) []*g.Node {
		for _, n := range ns {
			if n.Data() == cur {
				return []*g.Node{n}
			}
			if f := find(n.Children()); f != nil {
				return append(f, n)
			}
		}
		return nil
	}
	if chain := find(v.tree.Roots()); chain != nil {
		for _, a := range chain[1:] {
			a.SetExpanded(true)
		}
		v.tree.Select(chain[0])
	}
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

func (v *Viewer) renameNode(n *g.Node) {
	if e, ok := n.Data().(*model.Element); ok {
		v.renameElement(e)
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

// outlineEntries is the context menu of the outline (Swift's, plus rename and moves).
func (v *Viewer) outlineEntries() []editor.Entry {
	if v.session == nil {
		return nil
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
