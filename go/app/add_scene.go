//go:build darwin || linux || windows

package app

import "github.com/haiodo/tenniarb/go/editor"

// SceneKinds are the nodes Add offers for a scene, with the lines each one starts as. A container
// takes a colour: an empty one with no fill draws nothing, and nothing on the canvas takes a drag.
var SceneKinds = []struct{ Name, Text string }{
	{"Frame", "width 200; height 120; auto-layout column; gap 8; padding 8; color #ffffff"},
	{"Row", "width 200; height 40; auto-layout row; gap 8; color #ffffff"},
	{"Text", `title "Text"; font-size 14`},
	{"Box", "width 100; height 100; color #ffffff"},
}

// sceneBody is the body a new node of kind starts with.
func sceneBody(kind string) string {
	for _, k := range SceneKinds {
		if k.Name == kind {
			return k.Text
		}
	}
	return ""
}

// addSceneItem writes a new kid of the named kind of the selected node, or of the element when nothing
// is selected, and starts renaming it in the item list.
func (v *Viewer) addSceneItem(kind string) {
	body := sceneBody(kind)
	switch {
	case body == "":
		uilog("addSceneItem %s: no such kind", kind)
		return
	case !v.ensureSceneElement():
		uilog("addSceneItem %s: the canvas is not a scene element", kind)
		return
	case v.sv == nil || v.sv.node == nil:
		uilog("addSceneItem %s: no scene on the canvas", kind)
		return
	}
	target := v.sv.node
	if n, _ := v.selectedScene(); n != nil {
		target = n
	}
	name := FreeName(target, kind)
	head := CommandText("item", name)
	if target == v.sv.node {
		// The kids of the element take their place from pos: without one the node lands at 0,0, over
		// whatever is there, and a node with no place of its own does not drag.
		x, y := v.freeSpot()
		body = CommandText("pos", numText(x), numText(y)) + "; " + body
	}
	v.selNodes = nil
	before := v.text
	v.insertAndSelect(target, head+" { "+body+" }", head)
	uilog("addSceneItem %s into %q: selected=%v edit=%v", kind, PathOf(v.state.Current), v.selNodes, v.text != before)
	if len(v.selNodes) == 1 {
		if row := v.itemRowOf(v.selNodes[0]); row != nil {
			v.activateItem(row)
		}
	}
}

// addEntries are the entries of the Add menu: the node kinds of a scene, and the classic items of a
// diagram. The "+" of the Items pane and Edit > Add offer them.
func (v *Viewer) addEntries() []editor.Entry {
	out := make([]editor.Entry, 0, len(SceneKinds)+4)
	if v.sceneMode() {
		for _, k := range SceneKinds {
			kind := k.Name
			out = append(out, editor.Entry{Label: kind, Run: func() { uilog("add entry %q", kind); v.addSceneItem(kind) }})
		}
		return out
	}
	itemRun := func(label string, add func(*editor.Session)) func() {
		return func() {
			if v.session == nil {
				uilog("add entry %q: no session", label)
				return
			}
			before := len(v.session.Element().Items)
			add(v.session)
			uilog("add entry %q: items %d -> %d", label, before, len(v.session.Element().Items))
		}
	}
	return append(out,
		editor.Entry{Label: "New Item", Run: itemRun("New Item", (*editor.Session).AddTopItem)},
		editor.Entry{Label: "New Linked Item", Run: itemRun("New Linked Item", func(s *editor.Session) { s.AddNewItem(false) })},
		editor.Entry{Label: "Linked Styled Item", Run: itemRun("Linked Styled Item", func(s *editor.Session) { s.AddNewItem(true) })})
}
