//go:build darwin || linux || windows

package app

// moveNodeBy is Cmd+Alt+Up/Down on a scene node: the node changes places with the sibling before or
// after it, as one text edit and one undo step. The selection follows the node.
func (v *Viewer) moveNodeBy(delta int) {
	n, _ := v.selectedScene()
	if n == nil || n.Parent == nil {
		return
	}
	e, at, ok := MoveNodeEdit(v.fileText(v.fileOf(n)), n, delta)
	if !ok || !v.sceneEdit(n, e) {
		return
	}
	v.selectMoved(at)
}

// moveNodeInto is Cmd+Alt+Left/Right on a scene node: the node becomes the last kid of the sibling
// before it or the first kid of the one after it, as one text edit and one undo step.
func (v *Viewer) moveNodeInto(delta int) {
	n, _ := v.selectedScene()
	if n == nil || n.Parent == nil {
		return
	}
	e, at, ok := MoveNodeIntoEdit(v.fileText(v.fileOf(n)), n, delta)
	if !ok || !v.sceneEdit(n, e) {
		return
	}
	v.selectMoved(at)
}

// moveSelected is Element > Move Up/Down: the selected node of a scene, else the element.
func (v *Viewer) moveSelected(delta int) {
	if v.sceneMode() && len(v.selNodes) > 0 {
		v.moveNodeBy(delta)
		return
	}
	v.moveElement(delta)
}

// selectMoved selects the node that starts at at in the scene tree read after an edit.
func (v *Viewer) selectMoved(at int) {
	if at < 0 || v.sv == nil || v.sv.node == nil {
		return
	}
	if n := nodeStartingAt(v.sv.node, at); n != nil {
		v.selectScene([]string{NodePath(v.sv.node, n)})
	}
}
