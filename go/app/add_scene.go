//go:build darwin || linux || windows

package app

// SceneKinds are the nodes Add offers for a scene, with the lines each one starts as.
var SceneKinds = []struct{ Name, Text string }{
	{"Frame", "width 200; height 120; auto-layout column; gap 8; padding 8"},
	{"Row", "width 200; height 40; auto-layout row; gap 8"},
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
	if body == "" || !v.ensureSceneElement() || v.sv == nil || v.sv.node == nil {
		return
	}
	target := v.sv.node
	if n, _ := v.selectedScene(); n != nil {
		target = n
	}
	name := FreeName(target, kind)
	head := CommandText("item", name)
	v.selNodes = nil
	v.insertAndSelect(target, head+" { "+body+" }", head)
	if len(v.selNodes) == 1 {
		if row := v.itemRowOf(v.selNodes[0]); row != nil {
			v.activateItem(row)
		}
	}
}
