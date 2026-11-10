//go:build darwin || linux || windows

package app

import (
	"fmt"
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
	file.Item("Open Lin&k...", v.showOpenLink, g.Shortcut("Cmd+Shift+O"))
	file.Item("Import Pen &File...", v.importPenDialog)
	file.Item("&Reload", v.reloadCommand, g.Shortcut("Cmd+R"))
	file.Separator()
	file.Item("&Save", func() { v.save() }, g.Shortcut("Cmd+S"))
	file.Item("Save &As...", func() { v.saveAs() }, g.Shortcut("Cmd+Shift+S"))
	exp := file.Submenu("&Export")
	v.exportMenu(exp)
	v.selectionExportMenu(exp)
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
	edit.Item("Copy as P&NG", v.copySelectionPNG, g.Shortcut("Cmd+Shift+C"))
	edit.Item("&Paste", v.cmdPaste, g.Shortcut("Cmd+V"))
	edit.Item("&Delete", v.cmdDelete, g.Shortcut("Cmd+Backspace"))
	edit.Item("D&uplicate", v.cmdDuplicate, g.Shortcut("Cmd+D"))
	edit.Item("&Group", v.groupSelection, g.Shortcut("Cmd+G"))
	edit.Item("&Ungroup", v.ungroupSelection, g.Shortcut("Cmd+Shift+G"))
	edit.Item("Copy Lin&k", v.copyLink, g.Shortcut("Cmd+Alt+C"))
	edit.Separator()
	edit.Item("Select &All", v.cmdSelectAll, g.Shortcut("Cmd+A"))
	edit.Item("Select All &Items", func() { v.selectKind(model.ItemKindItem) }, g.Shortcut("Cmd+Shift+A"))
	edit.Item("Select All &Links", func() { v.selectKind(model.ItemKindLink) })
	edit.Item("Select &None", func() { v.selectKind("-") })
	edit.Separator()
	arrange := edit.Submenu("&Arrange")
	arrange.Item("Raise", func() { v.cmdOrder(v.session.Raise) }, g.Shortcut("Cmd+]"))
	arrange.Item("Lower", func() { v.cmdOrder(v.session.Lower) }, g.Shortcut("Cmd+["))
	arrange.Item("Bring to Front", func() { v.cmdOrder(v.session.ToFront) }, g.Shortcut("Cmd+Shift+]"))
	arrange.Item("Send to Back", func() { v.cmdOrder(v.session.ToBack) }, g.Shortcut("Cmd+Shift+["))
	arrange.Item("Layer Up", func() { v.cmdOrder(v.session.LayerUp) }, g.Shortcut("Cmd+Alt+]"))
	arrange.Item("Layer Down", func() { v.cmdOrder(v.session.LayerDown) }, g.Shortcut("Cmd+Alt+["))
	edit.Separator()
	edit.Item("Paste as I&tem", func() { v.pasteAs(false) })
	edit.Item("Paste as Item &Set", func() { v.pasteAs(true) })
	edit.Item("I&nherit", func() {
		if v.session != nil {
			v.session.Inherit()
		}
	})
	edit.Separator()
	edit.Item("Show Edit &Box", func() {
		if v.session != nil && len(v.session.Selection()) > 0 {
			v.showOperation()
		}
	})
	edit.Item("Edit Tit&le", func() { v.editKey(0) })
	edit.Item("Edit Bod&y", func() { v.editKey(editor.ModShift) })
	edit.Item("Edit &Value", func() { v.editKey(editor.ModAlt) })
	add := edit.Submenu("A&dd")
	for _, k := range SceneKinds {
		kind := k.Name
		add.Item(k.Name, func() { v.addSceneItem(kind) })
	}
	add.Separator()
	v.addClassic = []*g.MenuItem{
		add.Item("&New Item", func() {
			if v.session != nil {
				v.session.AddTopItem()
			}
		}),
		add.Item("New &Linked Item", func() {
			if v.session != nil {
				v.session.AddNewItem(false)
			}
		}),
		add.Item("Linked &Styled Item", func() {
			if v.session != nil {
				v.session.AddNewItem(true)
			}
		}),
	}
	edit.OnShow(v.updateMenus)

	el := bar.Submenu("E&lement")
	el.Item("&New Element", func() { v.addElement(v.state.Current) }, g.Shortcut("Cmd+Shift+N"))
	el.Item("New &Top-Level Element", func() { v.addElement(nil) })
	el.Item("&Rename", func() { v.renameElement(v.state.Current) }, g.Shortcut("Cmd+Shift+R"))
	el.Item("D&uplicate Element", v.duplicateElement)
	el.Item("&Delete Element", v.removeElement, g.Shortcut("Cmd+Shift+Backspace"))
	el.Separator()
	el.Item("Move &Up", func() { v.moveSelected(-1) }, g.Shortcut("Cmd+Alt+Up"))
	el.Item("Move D&own", func() { v.moveSelected(1) }, g.Shortcut("Cmd+Alt+Down"))
	v.moveItems = []*g.MenuItem{
		el.Item("Move &Left", func() { v.moveNodeInto(-1) }, g.Shortcut("Cmd+Alt+Left")),
		el.Item("Move &Right", func() { v.moveNodeInto(1) }, g.Shortcut("Cmd+Alt+Right")),
	}
	el.Separator()
	pages := el.Submenu("Export &Pages")
	pages.Item("PNG...", v.exportPagesPNG)
	pages.Item("PDF...", v.exportPagesPDF)
	el.Separator()
	v.suggestItem = el.Item("S&uggest Improvements...", v.showSuggest, g.Shortcut("Cmd+Alt+U"))
	el.OnShow(v.updateMenus)
	el.Separator()
	screens := el.Submenu("New &Screen")
	fillMenu(screens, v.screenSizeEntries())
	el.Item("&Insert Component...", v.showComponentPicker, g.Shortcut("Cmd+Shift+I"))
	el.Item("Add &Library...", v.addLibrary)

	if !v.sh.sysMenu {
		edit.Separator()
		edit.Item("Pre&ferences...", v.sh.showPrefs, g.Shortcut("Cmd+,"))
	}

	nav := bar.Submenu("&Navigate")
	nav.Item("&Goto Item...", v.showSearch, g.Shortcut("Cmd+F"))
	nav.Separator()
	v.nav.backItem = nav.Item("&Back", func() { v.navGo(-1) }, g.Shortcut("Cmd+["))
	v.nav.fwdItem = nav.Item("&Forward", func() { v.navGo(1) }, g.Shortcut("Cmd+]"))
	v.navSync()

	viewMenu := bar.Submenu("&View")
	viewMenu.Item("Zoom In", func() { v.zoomBy(ZoomStep) }, g.Shortcut("Cmd+="))
	viewMenu.Item("Zoom Out", func() { v.zoomBy(1 / ZoomStep) }, g.Shortcut("Cmd+-"))
	viewMenu.Item("Reset Zoom", v.resetZoom, g.Shortcut("Cmd+0"))
	viewMenu.Item("Zoom to &Fit", v.fitCommand, g.Shortcut("Cmd+9"))
	viewMenu.Item("Zoom to Se&lection", v.zoomToSelection)
	viewMenu.Separator()
	viewMenu.Item("&Quick Style Panel", v.showQuickPanel, g.Shortcut("Cmd+E"))
	viewMenu.Separator()
	viewMenu.Item("Play &Animations", v.toggleAnimations)
	viewMenu.Item("Comment &Mode", v.toggleCommentMode)
	viewMenu.Item("Comments...", v.showCommentList)
	viewMenu.Separator()
	viewMenu.Item("Toggle &Sidebar", v.toggleSidebar, g.Shortcut("Cmd+Alt+S"))
	viewMenu.Item("Toggle &Inspector", v.toggleInspector, g.Shortcut("Cmd+Alt+I"))
	viewMenu.Item("Toggle &Code", v.toggleCode, g.Shortcut("Cmd+Shift+Y"))
	viewMenu.Separator()
	v.compareItem = viewMenu.Item("Compare with &Git...", v.compareToggle, g.Shortcut("Cmd+Shift+G"))
	viewMenu.OnShow(v.updateMenus)

	help := bar.Submenu("&Help")
	help.Item("Tenniarb &Help", v.sh.showHelp, g.Shortcut("Cmd+Shift+/"))
	help.Item("Check for &Updates...", v.checkUpdatesCommand)
	help.Item("Install Agent &Skill", v.installSkills)
	help.Item("Install MCP for &Agents", v.installMCP)
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
	// A scene takes frames and rows; a diagram takes items.
	for _, it := range v.addClassic {
		it.SetEnabled(!v.sceneMode())
	}
	// Move Left/Right puts a scene node into its neighbouring sibling: no element takes that.
	for _, it := range v.moveItems {
		it.SetEnabled(v.sceneMode() && len(v.selNodes) > 0)
	}
	// A suggestion is a scene node or a style: a classic diagram has neither.
	if v.suggestItem != nil {
		v.suggestItem.SetEnabled(v.sceneMode())
	}
	// A comparison needs a version in a repository: the item is off for a file outside one.
	if v.compareItem != nil {
		_, repo := v.gitRepo()
		v.compareItem.SetEnabled(v.cmp != nil || repo)
	}
}

func (v *Viewer) treeFocused() bool { return v.tree.Unwrap().IsFocusControl() }

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

// contextMenus attaches the menus that open on a right click: the canvas, outline, Items and code.
func (v *Viewer) contextMenus() {
	canvas := g.PopupMenu(v.view.Canvas())
	canvas.OnShow(func() {
		entries := v.canvasEntries()
		if v.menuOverride == nil { // the quick style menu is not about the item
			entries = v.withLink(entries)
		}
		fillMenu(canvas, entries)
		v.menuOverride = nil
	})
	v.canvasMenu = canvas

	outline := g.PopupMenu(v.tree)
	outline.OnShow(func() { fillMenu(outline, v.withLink(v.outlineEntries())) })

	items := g.PopupMenu(v.itemsUI.tree)
	items.OnShow(func() { fillMenu(items, v.withLink(v.itemsEntries())) })

	code := g.PopupMenu(v.props.text)
	action := func(f func()) func() {
		return func() {
			v.props.text.Focus()
			if sel := v.props.menuSelection; sel != nil {
				v.props.text.SetSelection(int(sel.X), int(sel.Y))
			}
			f()
		}
	}
	code.Item("Cu&t", action(v.props.text.Cut), g.Shortcut("Cmd+X"))
	code.Item("&Copy", action(v.props.text.Copy), g.Shortcut("Cmd+C"))
	code.Item("&Paste", action(v.props.text.Paste), g.Shortcut("Cmd+V"))
	code.Item("Select &All", action(func() { v.props.text.SetSelection(0, len([]rune(v.props.text.Text()))) }), g.Shortcut("Cmd+A"))
}

// canvasEntries are the entries of the menu about to open on the canvas.
func (v *Viewer) canvasEntries() []editor.Entry {
	if v.menuOverride != nil {
		return v.menuOverride
	}
	if v.session == nil {
		return nil
	}
	if v.sceneMode() {
		return append(v.sceneEntries(), v.exportEntries()...)
	}
	v.session.Pick(v.rightClick)
	out := v.session.CanvasEntries(v.rightClick, editor.MenuHost{Layout: v.testLayout, AttachImage: v.attachImage, ExportHTML: v.copyItemHTML})
	// The same command the Element menu offers: a diagram element turns into a scene when it takes one.
	out = append(out, editor.Entry{Sep: true}, editor.Entry{Label: "Insert Component...", Run: v.showComponentPicker})
	return append(out, v.exportEntries()...)
}

// screenSizeEntries are the frames New Screen offers, the entries of the Element submenu and of the
// menu the start hint opens.
func (v *Viewer) screenSizeEntries() []editor.Entry {
	out := make([]editor.Entry, 0, len(ScreenSizes))
	for _, sz := range ScreenSizes {
		sz := sz
		out = append(out, editor.Entry{Label: fmt.Sprintf("%s %d x %d", sz.Name, sz.W, sz.H), Run: func() {
			v.newScreen(sz.W, sz.H)
		}})
	}
	return out
}

// sceneEntries are the canvas menu of a scene: the commands of the selected node, the screens.
func (v *Viewer) sceneEntries() []editor.Entry {
	out := []editor.Entry{{Label: "Attach image...", Run: v.attachSceneImage}, {Sep: true}}
	if len(v.selNodes) > 0 {
		out = append(out,
			editor.Entry{Label: "Duplicate", Run: v.duplicateScene},
			editor.Entry{Label: "Group", Run: v.groupSelection},
			editor.Entry{Label: "Ungroup", Run: v.ungroupSelection},
			editor.Entry{Label: "Move", Sub: []editor.Entry{
				{Label: "Up", Run: func() { v.moveNodeBy(-1) }},
				{Label: "Down", Run: func() { v.moveNodeBy(1) }},
				{Sep: true},
				{Label: "Into Previous", Run: func() { v.moveNodeInto(-1) }},
				{Label: "Into Next", Run: func() { v.moveNodeInto(1) }},
			}},
			editor.Entry{Label: "Zoom to Item", Run: v.zoomToSelection},
			editor.Entry{Label: "Delete", Run: v.deleteSceneNode},
			editor.Entry{Sep: true})
	}
	return append(out, editor.Entry{Label: "Insert Component...", Run: v.showComponentPicker})
}

// copyItemHTML is "Export text as html": the text of the selected item on the clipboard as HTML.
func (v *Viewer) copyItemHTML() {
	sel := v.session.Selection()
	if len(sel) != 1 {
		return
	}
	html := v.itemHTML(sel[0])
	if html != "" {
		_ = v.app.Clipboard().SetHTML(html, html)
	}
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

// itemHTML is the text of an item as HTML for the clipboard: the title and the body as the canvas lays
// them out. The old renderer built it from its own text boxes; the scene draws the text and knows it.
func (v *Viewer) itemHTML(item *model.DiagramItem) string {
	if v.sv == nil || v.sv.screen == nil || item == nil {
		return ""
	}
	pl := v.classicPlaced(item)
	if pl == nil || pl.Src == nil {
		return ""
	}
	return v.sv.screen.ItemHTML(pl.Src)
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
