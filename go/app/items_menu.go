//go:build darwin || linux || windows

package app

import (
	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/model"
)

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
	if r.Item != nil {
		return []editor.Entry{{Label: "Duplicate", Run: v.session.Duplicate}, {Label: "Delete", Run: v.session.DeleteSelection}}
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
	return entries
}
