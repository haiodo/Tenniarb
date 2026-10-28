//go:build darwin || linux || windows

package app

import (
	"fmt"
	"slices"
	"strings"

	g "github.com/haiodo/gowt"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/scene"
)

// afterElementRename is s once the element at path old has the path now.
func (s Selection) afterElementRename(old, now []string) Selection {
	if len(s.Element) >= len(old) && slices.Equal(s.Element[:len(old)], old) {
		s.Element = slices.Concat(now, s.Element[len(old):])
	}
	return s
}

// afterNodeRename is s once the node at path old of the element has the path now.
func (s Selection) afterNodeRename(elem []string, old, now string) Selection {
	if !slices.Equal(s.Element, elem) {
		return s
	}
	s.Nodes = slices.Clone(s.Nodes)
	for i, p := range s.Nodes {
		s.Nodes[i], _ = movedPath(p, old, now)
	}
	return s
}

// afterItemRename is s once the item named old of the diagram element has the name now.
func (s Selection) afterItemRename(elem []string, old, now string) Selection {
	if slices.Equal(s.Element, elem) {
		s.Items = slices.Clone(s.Items)
		for i, name := range s.Items {
			if name == old {
				s.Items[i] = now
			}
		}
	}
	return s
}

// each rewrites the entries: a rename changes the names they hold.
func (h *navHistory) each(f func(Selection) Selection) {
	for i := range h.list {
		h.list[i] = f(h.list[i])
	}
}

// editName shows a box under row n of tree. Enter or a click outside commits a changed name, Esc drops it.
// Ceiling: gowt 0.13.5 has no editor in a tree row, so the box is a popover.
func (v *Viewer) editName(tree *g.TreeView, n *g.Node, name string, commit func(string)) {
	v.closePopover()
	p := v.win.Popover(g.Gap(4))
	var field *g.TextView
	done := false
	finish := func(text string) {
		if done {
			return
		}
		done = true
		if text = strings.TrimSpace(text); text != name {
			v.app.Async(func() { commit(text) })
		}
	}
	b := n.Unwrap().GetBounds()
	p.Add(g.Text(g.Focused(), g.Ref(&field), g.Size(max(int(b.Width), 200), 0), g.ID("rename-field"), g.Init(func(t *g.TextView) {
		t.SetText(name)
		t.OnActivate(func() {
			finish(t.Text())
			p.Close()
		})
	})))
	p.OnEscape(func() bool {
		done = true
		return false
	})
	p.OnClose(func() { finish(field.Text()) })
	v.openPopover(p)
	v.activeText = field.Unwrap()
	field.Unwrap().SelectAll()
	p.ShowAtPoint(tree, int(b.X), int(b.Y+b.Height), g.PopoverBelow)
}

// renameElement asks for the new name of the element in a box under its row.
func (v *Viewer) renameElement(e *model.Element) {
	n := v.nodeOf(e)
	if v.session == nil || e == nil || e.Parent == nil || n == nil {
		return
	}
	v.editName(v.tree, n, e.Name, func(name string) { v.renameElementTo(e, name) })
}

// renameElementTo renames e; the references to it in all files follow when the user agrees, as one
// undo step.
func (v *Viewer) renameElementTo(e *model.Element, name string) {
	plan, err := v.planElementRename(e, name)
	if err != nil {
		v.setBanner(err.Error())
		return
	}
	update, ok := v.confirmRename(plan)
	if !ok {
		return
	}
	old := PathOf(e)
	v.nav.restoring = true
	err = v.applyRename(plan, update)
	v.nav.restoring = false
	if err != nil {
		v.setBanner(err.Error())
		return
	}
	now := PathOf(e)
	v.nav.hist.each(func(s Selection) Selection { return s.afterElementRename(old, now) })
	v.navSync()
}

// confirmRename asks about the references of the plan; update: they follow, ok: false when cancelled.
// Without references it does not ask.
func (v *Viewer) confirmRename(p *renamePlan) (update, ok bool) {
	if len(p.refs) == 0 {
		return true, true
	}
	text := fmt.Sprintf("%d references name it:\n\n%s\n\nYes: rename and update %d references.\n"+
		"No: rename only; the references keep the old name and stop working.\nCancel: keep the old name.",
		len(p.refs), v.refLines(p.refs, 10), len(p.refs))
	switch v.win.MessageBox(g.Message{Title: fmt.Sprintf("Rename to %q", p.name), Text: text, Icon: g.IconQuestion, Buttons: g.ButtonsYesNoCancel}) {
	case g.AnswerYes:
		return true, true
	case g.AnswerNo:
		return false, true
	}
	return false, false
}

// activateItem is a double click or Enter on a row of the item list: the name is edited. A slot, a
// part of a component and a link have no name of their own.
func (v *Viewer) activateItem(n *g.Node) {
	r, ok := n.Data().(*ItemRow)
	if !ok || v.session == nil {
		return
	}
	switch {
	case r.Item != nil && !r.Item.IsLink():
		it := r.Item
		v.editName(v.itemsUI.tree, n, it.Name, func(name string) { v.renameDiagramItem(it, name) })
	case r.Item == nil && v.sceneMode() && (r.Kind == "item" || r.Kind == "use") && !strings.Contains(r.Path, "/"):
		if node := NodeAt(v.sv.node, r.Path); node != nil {
			path := r.Path
			v.editName(v.itemsUI.tree, n, node.Name, func(name string) { v.renameNodeTo(path, name) })
		}
	}
}

// renameNodeTo renames the scene node at path. The paths that name it follow: the selection, the open
// rows, the history of the Code panel and, when the user agrees, the references in the files.
func (v *Viewer) renameNodeTo(path, name string) {
	if !v.sceneMode() {
		return
	}
	node := NodeAt(v.sv.node, path)
	if node == nil {
		return
	}
	plan, err := v.planNodeRename(node, name)
	if err != nil {
		v.setBanner(err.Error())
		return
	}
	update, ok := v.confirmRename(plan)
	if !ok {
		return
	}
	elem := PathOf(v.state.Current)
	now := renamedNodePath(path, scene.NormalizeName(plan.name))
	v.nav.restoring = true
	err = v.applyRename(plan, update)
	v.nav.restoring = false
	if err != nil {
		v.setBanner(err.Error())
		return
	}
	v.nav.hist.each(func(s Selection) Selection { return s.afterNodeRename(elem, path, now) })
	l := &v.itemsUI
	for p := range l.open {
		if moved, ok := movedPath(p, path, now); ok {
			delete(l.open, p)
			l.open[moved] = true
		}
	}
	v.rebuildItems()
	v.selectScene([]string{now})
}

// renameDiagramItem renames an item of a diagram element. Link ends follow by themselves, a link holds
// the item; the expressions and inherits that use the name are listed and not changed.
func (v *Viewer) renameDiagramItem(it *model.DiagramItem, name string) {
	if err := v.checkRename(); err != nil {
		v.setBanner(err.Error())
		return
	}
	if name == "" {
		v.setBanner("The name cannot be empty.")
		return
	}
	if refs := ClassicItemRefs(it.Parent, it); len(refs) > 0 {
		text := fmt.Sprintf("%d places use the name and are not changed:\n\n%s\n\nRename anyway?", len(refs), classicRefLines(refs, 10))
		if v.win.MessageBox(g.Message{Title: fmt.Sprintf("Rename to %q", name), Text: text, Icon: g.IconWarning, Buttons: g.ButtonsOKCancel}) != g.AnswerOK {
			return
		}
	}
	old, elem := it.Name, PathOf(it.Parent)
	v.nav.restoring = true
	v.session.CommitEdit(it, editor.EditName, name)
	v.nav.restoring = false
	v.nav.hist.each(func(s Selection) Selection { return s.afterItemRename(elem, old, name) })
	v.navSync()
}
