//go:build darwin || linux || windows

package app

import (
	"strings"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/scene"
)

// sceneCodeKey names what the Code section shows for a scene element: the element and the node.
func (v *Viewer) sceneCodeKey() string {
	if v.state.Current == nil {
		return ""
	}
	key := pathKey(v.state.Current) + "\x01"
	if len(v.selNodes) == 0 {
		return key
	}
	return key + v.selNodes[0]
}

// sceneCodeNode is the node the Code section shows: the selected one, or the element itself when nothing
// is selected, so its own properties are there to edit.
func (v *Viewer) sceneCodeNode() (*scene.Node, *scene.Placed) {
	if n, pl := v.selectedScene(); n != nil || pl != nil {
		return n, pl
	}
	return v.elementNode(v.state.Current), nil
}

// refreshSceneCode shows the source of the selected node in the Code section, its kids as one line each;
// with nothing selected, the element itself.
func (v *Viewer) refreshSceneCode() {
	n, pl := v.sceneCodeNode()
	key := v.sceneCodeKey()
	want := ""
	switch {
	case n != nil:
		want = OwnText(v.fileText(v.ownFileOf(n)), n)
	case pl != nil:
		want = EffectiveText(pl)
	}
	if propsKeepDraft(v.propsFocused(), key == v.props.sceneKey, v.props.applying, want, v.props.want) {
		return
	}
	if key == v.props.sceneKey && v.props.text.Text() == want {
		return
	}
	// What the user typed stays on screen until the code changes the text: a kid written in full keeps
	// its properties, the kids already there stay stubs.
	if key == v.props.sceneKey && want == v.props.want {
		return
	}
	v.setPropsText(want, editor.Target{})
	v.props.sceneKey = key
	v.props.values = nil
}

// sceneApply applies the edit of the Code pane with props.applying set: the session redraws inside it,
// and that redraw must not take the pane's draft for a change of the code.
func (v *Viewer) sceneApply(n *scene.Node, e TextEdit) bool {
	v.props.applying = true
	defer func() { v.props.applying = false }()
	return v.sceneEdit(n, e)
}

// applySceneCode puts the text of the Code section in place of the node it was opened for; the kids stay
// as they are written.
func (v *Viewer) applySceneCode(key, text string) {
	if v.props.text.Text() == text {
		v.styleProps(text)
	}
	if key == "" || key != v.sceneCodeKey() || len(editor.ParseDiagnostics(text)) > 0 {
		return
	}
	n, pl := v.sceneCodeNode()
	if n == nil && pl != nil { // a part of a component has no code of its own
		if text != EffectiveText(pl) {
			v.setBanner("The code of a part of an instance is read only: change it in the inspector")
			v.refreshSceneCode()
		}
		return
	}
	src := v.fileText(v.ownFileOf(n))
	if n == nil || OwnText(src, n) == text {
		return
	}
	// The kids are hidden behind stubs: an emptied pane would delete a subtree the user never saw.
	// No refresh here, the user may be retyping the node.
	if strings.TrimSpace(text) == "" {
		v.setBanner("Not applied: delete the item from the Items tree")
		return
	}
	e := ReplaceOwnEdit(src, n, text)
	if e.New == src[e.Start:e.End] { // only stubs changed: show the kids again
		v.refreshSceneCode()
		return
	}
	start := n.Range.Start
	at, length := v.props.text.Selection()
	if !v.sceneApply(n, e) {
		return
	}
	// A renamed node keeps the selection: it starts where the old one did.
	if len(v.selNodes) == 1 && NodeAt(v.sv.node, v.selNodes[0]) == nil {
		if m := nodeStartingAt(v.sv.node, start); m != nil {
			v.selNodes = []string{NodePath(v.sv.node, m)}
			v.props.sceneKey = v.sceneCodeKey()
			v.drawSceneOverlay()
		}
	}
	// The pane keeps what was typed, as the code now holds it: a kid written in full stays open for the
	// next lines, the kids already there stay stubs.
	if m, _ := v.sceneCodeNode(); m != nil {
		if v.props.text.Text() != text {
			v.setPropsText(text, editor.Target{})
			v.props.text.SetSelection(at, length)
		}
		v.props.want = OwnText(v.fileText(v.ownFileOf(m)), m)
	}
}

func nodeStartingAt(n *scene.Node, start int) *scene.Node {
	for _, k := range editor.SceneKids(n) {
		if k.Range.Start == start {
			return k
		}
		if k.Range.Start < start && start < k.Range.End {
			return nodeStartingAt(k, start)
		}
	}
	return nil
}
