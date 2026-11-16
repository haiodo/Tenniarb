//go:build darwin || linux || windows

package app

import (
	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/model"
)

// rowEntries is the list menu of an item, component or group row; handled is false for a canvas node,
// whose commands itemsEntries builds itself.
func rowEntries(r *ItemRow, duplicate, remove, insert, link func()) (entries []editor.Entry, handled bool) {
	switch {
	case r.Item != nil:
		return []editor.Entry{
			{Label: "Duplicate", Run: duplicate},
			{Label: "Delete", Run: remove},
			{Sep: true},
			{Label: "Copy Link", Run: link},
		}, true
	case r.Component:
		// The row of a component is not a node of the canvas: the menu inserts it, as a double click does.
		return []editor.Entry{{Label: "Insert", Run: insert}}, true
	case r.Kind == "group":
		return nil, true
	}
	return nil, false
}

// itemsEntries is the context menu of the list: the commands of the selected node, and in a mounted
// element Insert Component.
func (v *Viewer) itemsEntries() []editor.Entry {
	sel := v.itemsUI.tree.Selected()
	if len(sel) != 1 || v.session == nil {
		return nil
	}
	r, ok := sel[0].Data().(*ItemRow)
	if !ok {
		return nil
	}
	if entries, handled := rowEntries(r, v.session.Duplicate, v.session.DeleteSelection,
		func() { v.activateItemRow(*r, nil) }, v.copyLink); handled {
		return entries
	}
	entries := []editor.Entry{
		{Label: "Add", Sub: []editor.Entry{
			{Label: "Frame", Run: func() { v.addSceneItem("Frame") }},
			{Label: "Row", Run: func() { v.addSceneItem("Row") }},
			{Label: "Text", Run: func() { v.addSceneItem("Text") }},
			{Label: "Box", Run: func() { v.addSceneItem("Box") }},
		}},
		{Sep: true},
	}
	if NodeAt(v.sv.node, r.Path) != nil {
		entries = append(entries,
			editor.Entry{Label: "Move", Sub: []editor.Entry{
				{Label: "Up", Run: func() { v.moveNodeBy(-1) }},
				{Label: "Down", Run: func() { v.moveNodeBy(1) }},
				{Sep: true},
				{Label: "Into Previous", Run: func() { v.moveNodeInto(-1) }},
				{Label: "Into Next", Run: func() { v.moveNodeInto(1) }},
			}},
			editor.Entry{Sep: true})
	}
	entries = append(entries,
		editor.Entry{Label: "Duplicate", Run: v.duplicateScene},
		editor.Entry{Label: "Zoom to Item", Run: v.zoomToSelection},
		editor.Entry{Sep: true},
		editor.Entry{Label: "Delete", Run: v.deleteSceneNode})
	if r.Kind == "use" {
		entries = append([]editor.Entry{{Label: "Go to Component", Run: func() {
			v.goToComponentNode(NodeAt(v.sv.node, r.Path))
		}}, {Sep: true}}, entries...)
	}
	if m := model.MountOf(v.state.Current, false); m != nil {
		entries = append([]editor.Entry{{Label: "Insert Component", Run: func() { v.insertFromMount(m, r) }}, {Sep: true}}, entries...)
	}
	return v.withLink(entries)
}
