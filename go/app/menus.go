//go:build darwin || linux || windows

package app

import (
	"math"
	"path/filepath"
	"runtime"
	"strings"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/model"
)

// cmdIsCtrl: the menu shortcut key is Ctrl where there is no Command key.
var cmdIsCtrl = runtime.GOOS != "darwin"

func (v *Viewer) menus() {
	bar := v.win.MenuBar()
	file := bar.Submenu("&File")
	file.Item("&New", func() { v.sh.newDocument() }, g.Shortcut("Cmd+N"))
	file.Item("&Open...", v.openDialog, g.Shortcut("Cmd+O"))
	v.recentMenu = file.Submenu("Open &Recent")
	v.fillRecent()
	file.Item("&Reload", v.reloadCommand, g.Shortcut("Cmd+R"))
	file.Separator()
	file.Item("&Save", func() { v.save() }, g.Shortcut("Cmd+S"))
	file.Item("Save &As...", func() { v.saveAs() }, g.Shortcut("Cmd+Shift+S"))
	v.exportMenu(file.Submenu("&Export"))
	file.Separator()
	file.Item("&Close Window", v.win.Close, g.Shortcut("Cmd+W"))
	if runtime.GOOS != "darwin" {
		file.Item("&Quit", v.app.Unwrap().Close, g.Shortcut("Cmd+Q")) // asks every window; app.Quit would not
	}

	edit := bar.Submenu("&Edit")
	v.undoItem = edit.Item("&Undo", v.cmdUndo, g.Shortcut("Cmd+Z"))
	v.redoItem = edit.Item("&Redo", v.cmdRedo, g.Shortcut("Cmd+Shift+Z"))
	edit.Separator()
	edit.Item("Cu&t", v.cmdCut, g.Shortcut("Cmd+X"))
	edit.Item("&Copy", v.cmdCopy, g.Shortcut("Cmd+C"))
	edit.Item("&Paste", v.cmdPaste, g.Shortcut("Cmd+V"))
	edit.Item("&Delete", v.cmdDelete, g.Shortcut("Cmd+Backspace"))
	edit.Item("D&uplicate", v.cmdDuplicate, g.Shortcut("Cmd+D"))
	edit.Separator()
	edit.Item("Select &All", v.cmdSelectAll, g.Shortcut("Cmd+A"))
	edit.Item("Select All &Items", func() {
		if v.session != nil {
			v.session.SelectAll(model.ItemKindItem)
		}
	}, g.Shortcut("Cmd+Shift+A"))
	edit.OnShow(v.updateMenus)

	el := bar.Submenu("E&lement")
	el.Item("&New Element", func() { v.addElement(v.state.Current) }, g.Shortcut("Cmd+Shift+N"))
	el.Item("New &Top-Level Element", func() { v.addElement(nil) })
	el.Item("&Rename", func() { v.renameElement(v.state.Current) }, g.Shortcut("Cmd+Shift+R"))
	el.Item("D&uplicate Element", v.duplicateElement)
	el.Item("&Delete Element", v.removeElement, g.Shortcut("Cmd+Shift+Backspace"))
	el.Separator()
	el.Item("Move &Up", func() { v.moveElement(-1) }, g.Shortcut("Cmd+Alt+Up"))
	el.Item("Move D&own", func() { v.moveElement(1) }, g.Shortcut("Cmd+Alt+Down"))

	if !v.sh.sysMenu {
		edit.Separator()
		edit.Item("Pre&ferences...", v.sh.showPrefs, g.Shortcut("Cmd+,"))
	}

	nav := bar.Submenu("&Navigate")
	nav.Item("&Goto Item...", v.showSearch, g.Shortcut("Cmd+F"))

	viewMenu := bar.Submenu("&View")
	viewMenu.Item("Zoom In", func() { v.zoomBy(ZoomStep) }, g.Shortcut("Cmd+="))
	viewMenu.Item("Zoom Out", func() { v.zoomBy(1 / ZoomStep) }, g.Shortcut("Cmd+-"))
	viewMenu.Item("Reset Zoom", v.resetZoom, g.Shortcut("Cmd+0"))
	viewMenu.Item("Zoom to &Fit", v.fitCommand, g.Shortcut("Cmd+9"))
	viewMenu.Separator()
	viewMenu.Item("&Quick Style Panel", v.showQuickPanel, g.Shortcut("Cmd+E"))
	viewMenu.Separator()
	viewMenu.Item("Toggle &Sidebar", v.toggleSidebar, g.Shortcut("Cmd+Alt+S"))
	viewMenu.Item("Toggle &Inspector", v.toggleInspector, g.Shortcut("Cmd+Alt+I"))

	help := bar.Submenu("&Help")
	help.Item("Tenniarb &Help", v.sh.showHelp, g.Shortcut("Cmd+Shift+/"))
	if !v.sh.sysMenu {
		help.Item("&About Tenniarb", v.sh.showAbout)
	}
}

// fillRecent rebuilds Open Recent from the list of the shell.
func (v *Viewer) fillRecent() {
	if v.recentMenu == nil {
		return
	}
	var entries []editor.Entry
	for _, p := range v.sh.recent {
		entries = append(entries, editor.Entry{Label: filepath.Base(p), Run: func() { v.sh.openPath(p) }})
	}
	entries = append(entries, editor.Entry{Sep: true}, editor.Entry{Label: "Clear Menu", Run: v.sh.clearRecent})
	fillMenu(v.recentMenu, entries)
}

// updateMenus sets what depends on the document: Undo and Redo are enabled while the history has a step.
func (v *Viewer) updateMenus() {
	if v.undoItem == nil {
		return
	}
	um := (*model.UndoManager)(nil)
	if v.session != nil {
		um = v.session.UndoManager()
	}
	v.undoItem.SetEnabled(um != nil && um.CanUndo() && v.activeText == nil)
	v.redoItem.SetEnabled(um != nil && um.CanRedo() && v.activeText == nil)
}

func (v *Viewer) treeFocused() bool { return v.tree.Unwrap().IsFocusControl() }

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

// fillMenu replaces the items of m by entries.
func fillMenu(m *g.Menu, entries []editor.Entry) {
	for _, it := range m.Unwrap().GetItems() {
		it.Dispose()
	}
	addEntries(m, entries)
}

func addEntries(m *g.Menu, entries []editor.Entry) {
	for _, e := range entries {
		label := strings.ReplaceAll(e.Label, "&", "&&")
		switch {
		case e.Sep:
			m.Separator()
		case e.Sub != nil:
			addEntries(m.Submenu(label), e.Sub)
		case e.Run != nil:
			run := e.Run
			m.Item(label, func() { run() })
		}
	}
}

// contextMenus attaches the menus that open on a right click: the canvas and the outline.
func (v *Viewer) contextMenus() {
	canvas := g.PopupMenu(v.view.Canvas())
	canvas.OnShow(func() {
		fillMenu(canvas, v.canvasEntries())
		v.menuOverride = nil
	})
	v.canvasMenu = canvas

	outline := g.PopupMenu(v.tree)
	outline.OnShow(func() { fillMenu(outline, v.outlineEntries()) })
}

// canvasEntries are the entries of the menu about to open on the canvas.
func (v *Viewer) canvasEntries() []editor.Entry {
	if v.menuOverride != nil {
		return v.menuOverride
	}
	if v.session == nil {
		return nil
	}
	v.session.Pick(v.rightClick)
	return v.session.CanvasEntries(v.rightClick, editor.MenuHost{Layout: v.testLayout, AttachImage: v.attachImage})
}

func (v *Viewer) testLayout() { v.session.TestLayout(v.visibleScene()) }

// showQuickPanel opens the panel of quick styles over the selected item (Swift showPopup).
func (v *Viewer) showQuickPanel() {
	if v.session == nil || len(v.session.Selection()) != 1 {
		return
	}
	item := v.session.Selection()[0]
	v.closePopover()
	p := v.win.Popover(g.Gap(2), g.Padding(4))
	var buttons []g.Option
	for _, seg := range editor.QuickPanel(item.Kind) {
		buttons = append(buttons, g.Button(seg.Label, g.Tooltip(seg.Style.Label), g.Init(func(b *g.ButtonView) {
			b.OnClick(func() { v.quickMenu(p, seg.Style) })
		})))
	}
	if len(buttons) == 0 {
		p.Dispose()
		return
	}
	p.Add(g.HBox(append([]g.Option{g.Gap(2), g.Padding(0)}, buttons...)...))
	v.openPopover(p)
	v.popoverAt(p, item)
}

// quickMenu closes the panel and opens the options of one style as a menu at the pointer: a menu
// opened inside the panel would close it, and the menu with it.
func (v *Viewer) quickMenu(p *g.Popover, q editor.QuickStyle) {
	entries := v.session.QuickEntries(q)
	p.Close()
	v.app.Async(func() {
		v.menuOverride = entries
		v.canvasMenu.Show()
	})
}

// maybeQuickPanel shows the panel after a click that selected one item.
func (v *Viewer) maybeQuickPanel(pr press, pos gfx.Point) {
	if !v.autoQuick {
		return
	}
	if math.Hypot(pos.X-pr.from.X, pos.Y-pr.from.Y) > 3 || len(v.session.Selection()) != 1 {
		v.closePopover()
		return
	}
	v.showQuickPanel()
}

// attachImage asks for a picture and adds it to the selected item.
func (v *Viewer) attachImage() {
	paths := v.win.FileDialog(g.FileDialog{
		Title:   "Attach image",
		Filters: []g.FileFilter{{Name: "Images", Pattern: "*.png;*.jpg;*.jpeg;*.gif"}},
	})
	if len(paths) != 1 || v.session == nil {
		return
	}
	data, err := PNGBase64(paths[0])
	if err != nil {
		v.setBanner(err.Error())
		return
	}
	v.session.AttachImage(filepath.Base(paths[0]), data)
}
