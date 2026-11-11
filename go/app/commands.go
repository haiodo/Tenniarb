//go:build darwin || linux || windows

package app

import (
	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/model"
)

// selectKind selects the items of the element of one kind; "-" selects none.
func (v *Viewer) selectKind(kind model.ItemKind) {
	if v.session == nil || v.activeText != nil {
		return
	}
	if kind == "-" {
		v.session.Select(nil)
		return
	}
	v.session.SelectAll(kind)
}

// pasteAs is Paste as Item (set: an item per line) from the clipboard text.
func (v *Viewer) pasteAs(set bool) {
	if v.session == nil || v.activeText != nil {
		return
	}
	text, ok := v.app.Clipboard().Text()
	switch {
	case !ok:
	case set:
		v.session.PasteAsItemSet(text)
	default:
		v.session.PasteAsItem(text)
	}
}

// editKey is Edit title (no modifier), Edit body (Shift) or Edit value (Alt): the Enter of the canvas.
func (v *Viewer) editKey(m editor.Modifiers) {
	if v.session != nil && v.activeText == nil {
		v.session.Key(editor.KeyEvent{Key: "Enter", Mods: m})
	}
}

func (v *Viewer) cmdUndo() {
	if v.session != nil && v.activeText == nil {
		v.session.Undo()
	}
}

func (v *Viewer) cmdRedo() {
	if v.session != nil && v.activeText == nil {
		v.session.Redo()
	}
}

func (v *Viewer) setClipboard(text string) {
	if text != "" {
		v.app.Clipboard().SetText(text)
	}
}

func (v *Viewer) cmdCopy() {
	switch {
	case v.activeText != nil:
		v.activeText.Copy()
	case v.propsFocused():
		v.props.text.Copy()
	case v.session == nil:
	case v.treeFocused():
		v.setClipboard(v.session.CopyElement(v.state.Current))
	default:
		v.setClipboard(v.session.CopyText())
	}
}

func (v *Viewer) cmdCut() {
	switch {
	case v.activeText != nil:
		v.activeText.Cut()
	case v.propsFocused():
		v.props.text.Cut()
	case v.session == nil:
	case v.treeFocused():
		if v.state.Current.Parent != nil {
			v.setClipboard(v.session.CutElement(v.state.Current))
		}
	default:
		v.setClipboard(v.session.Cut())
	}
}

func (v *Viewer) cmdPaste() {
	if v.activeText != nil {
		v.activeText.Paste()
		return
	}
	if v.propsFocused() {
		v.props.text.Paste()
		return
	}
	if v.session == nil {
		return
	}
	text, ok := v.app.Clipboard().Text()
	if !ok {
		return
	}
	if v.treeFocused() {
		v.session.PasteElements(v.state.Current, text)
		return
	}
	if !v.session.Paste(text) {
		v.session.PasteAsItem(text)
	}
}

func (v *Viewer) cmdDelete() {
	switch {
	case v.activeText != nil, v.propsFocused(), v.session == nil:
	case v.sceneMode() && len(v.selNodes) > 0:
		v.deleteSceneNode()
	case v.treeFocused():
		v.removeElement()
	default:
		v.session.DeleteSelection()
	}
}

func (v *Viewer) cmdDuplicate() {
	switch {
	case v.activeText != nil, v.propsFocused(), v.session == nil:
	case v.treeFocused():
		v.duplicateElement()
	case v.sceneMode():
		v.duplicateScene()
	default:
		v.session.Duplicate()
	}
}

func (v *Viewer) cmdSelectAll() {
	switch {
	case v.activeText != nil:
		v.activeText.SelectAll()
	case v.propsFocused():
		v.props.text.SetSelection(0, len([]rune(v.props.text.Text())))
	case v.session != nil:
		v.session.SelectAll("")
	}
}

// cmdOrder runs a command of the drawing order of a classic diagram: the items take their place in the
// list, which is the order they are drawn in.
func (v *Viewer) cmdOrder(run func()) {
	if v.session == nil || v.activeText != nil || v.propsFocused() {
		return
	}
	run()
	v.sessionChanged()
}
