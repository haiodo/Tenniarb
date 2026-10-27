//go:build darwin || linux || windows

package app

import (
	"fmt"
	"math"
	"path/filepath"
	"runtime"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/view"
	"github.com/haiodo/gowt/look"
	"github.com/haiodo/gowt/rx"
	"github.com/haiodo/gowt/swt"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
)

// Viewer is the window: the outline as the sidebar, the diagram of the selected element, and the
// inspector on the right, editable through an editor.Session.
type Viewer struct {
	app    *g.App
	sh     *Shell
	win    *g.Window
	tree   *g.TreeView
	banner *g.LabelView
	view   *view.View
	scene  *gfx.Scene

	*docCore
	// conflict: the file changed on disk while the document had unsaved edits and the edits were kept.
	conflict bool
	asking   bool // a modal question is open; the poll waits

	rendered    *render.Rendered
	overlay     *gfx.Node
	sv          *sceneView // the element on the canvas is drawn by go/scene
	itemsUI     itemsList
	sceneBuilds int // how many times go/scene drew the canvas, for the tests
	smap        SceneMap
	shownElem   *model.Element // the element the camera was placed for
	nextID      int
	pendingFit  bool // the canvas had no size yet when the element was shown
	syncing     bool // the outline selection is being set by code
	zoomSeen    float64
	zoomText    *rx.State[string] // the toolbar zoom item, like Swift zoomLabel

	press      press
	inline     *view.Editor
	inlineMode editor.EditMode
	activeText *swt.Text // a text field of the window that has the keyboard: clipboard commands go to it
	popover    *g.Popover
	outlineSig string
	props      props
	insp       *inspector
	search     search
	docElem    *model.Element // the last element on the canvas that is not in a mounted file
	docSel     []string       // its selected scene nodes, for the return from a mounted file
	dragging   *model.Element // the outline row a drag started on
	dropAt     [2]int         // where the drag was last over the outline

	undoItem, redoItem *g.MenuItem
	canvasMenu         *g.Menu
	menuOverride       []editor.Entry // entries for the next opening of the canvas menu
	rightClick         render.Point   // where the canvas menu was asked for
	recentMenu         *g.Menu
	autoQuick          bool // show the quick style panel after a click on an item

	id        string // names the stash of an untitled document
	autosave  *Debounce
	frameSave *Debounce
	force     bool // the window closes without questions (QA)
}

// viewerInit says what a new window shows: a file, or an untitled document with the text.
type viewerInit struct {
	path    string
	id      string
	text    string
	stashed bool // the text is a stash of the last launch: it has unsaved edits
}

// newViewer opens a window for a file or an untitled document.
func (sh *Shell) newViewer(in viewerInit) *Viewer {
	v := &Viewer{app: sh.app, sh: sh, scene: gfx.NewScene(), zoomText: rx.New("100%"), nextID: 1, docCore: &docCore{state: NewState(&Document{}), others: sh.othersText, peer: sh.peerDoc}, id: in.id}
	v.autosave = &Debounce{Delay: AutosaveDelay, After: sh.app.After, Fn: v.autosaveNow}
	v.frameSave = &Debounce{Delay: FrameDelay, After: sh.app.After, Fn: v.saveFrame}
	prev := sh.active
	sh.viewers = append(sh.viewers, v)
	sh.active = v
	v.build()
	v.placeWindow(in.path, prev)
	v.win.Show()
	if in.path != "" {
		v.openPath(in.path)
	} else {
		v.openText(in.text, in.stashed)
	}
	v.trackWindow()
	return v
}

// placeWindow puts the window where the document was last closed, else a step down and right of the
// window it was opened from.
func (v *Viewer) placeWindow(path string, prev *Viewer) {
	sw, sh := v.app.ScreenSize()
	if f, ok := LoadFrame(v.sh.dir, path); ok && path != "" && f.OnScreen(sw, sh) {
		v.win.SetSize(int(f.Width), int(f.Height))
		v.win.Unwrap().SetLocation(int32(f.X), int32(f.Y))
		return
	}
	if prev != nil && !prev.win.Unwrap().IsDisposed() {
		b := prev.win.Unwrap().GetBounds()
		v.win.Unwrap().SetLocation(b.X+26, b.Y+26)
	}
}

// trackWindow follows the window: it is the active one when it gets the focus, its frame is remembered
// after a move or resize, and it leaves the shell when it is gone.
func (v *Viewer) trackWindow() {
	s := v.win.Unwrap()
	s.AddShellListener(swt.ShellListenerShellActivatedAdapter(func(*swt.ShellEvent) { v.sh.active = v }))
	frame := func(*swt.ControlEvent) {
		if v.state.Doc.Path != "" {
			v.frameSave.Touch()
		}
	}
	s.AddControlListener(swt.ControlListenerControlMovedAdapter(frame))
	s.AddControlListener(swt.ControlListenerControlResizedAdapter(frame))
	s.AddListener(swt.Dispose, listenerFunc(func(*swt.Event) {
		v.autosave.Cancel()
		v.frameSave.Cancel()
		v.sh.remove(v)
	}))
}

func (v *Viewer) saveFrame() {
	if v.state.Doc.Path == "" || v.win.Unwrap().IsDisposed() {
		return
	}
	b := v.win.Unwrap().GetBounds()
	_ = SaveFrame(v.sh.dir, v.state.Doc.Path, Frame{float64(b.X), float64(b.Y), float64(b.Width), float64(b.Height)})
}

// canvasColors are the light and dark background of the canvas.
func canvasColors(p Prefs) (light, dark g.RGB) {
	l, _ := render.ParseColor(p.Background)
	d, _ := render.ParseColor(p.BackgroundDark)
	to8 := func(f float64) uint8 { return uint8(f*255 + 0.5) }
	return g.RGB{R: to8(l.R), G: to8(l.G), B: to8(l.B)}, g.RGB{R: to8(d.R), G: to8(d.G), B: to8(d.B)}
}

// applyBackground sets the canvas colour for the current appearance, after the settings changed.
func (v *Viewer) applyBackground() {
	light, dark := canvasColors(v.sh.prefs)
	c := light
	if v.app.Dark() {
		c = dark
	}
	v.view.Canvas().Unwrap().SetBackgroundWithColor(swt.NewColorRedGreenBlue(int32(c.R), int32(c.G), int32(c.B)))
}

func (v *Viewer) themeChanged() {
	if v.session != nil {
		v.session.SetDarkMode(v.app.Dark())
	}
	v.applyBackground()
}

// prefsChanged applies new settings: the canvas colour and the quick panel at once, the outline
// expansion the next time the outline is built.
func (v *Viewer) prefsChanged() {
	v.autoQuick = v.sh.prefs.QuickPanel
	v.applyBackground()
}

func (v *Viewer) build() {
	light, dark := canvasColors(v.sh.prefs)
	v.autoQuick = v.sh.prefs.QuickPanel
	v.win = v.app.Window("Tenniarb", g.Padding(0), g.Gap(0))
	var sidebar, content *g.Panel
	v.win.Add(
		g.VBox(g.Ref(&sidebar), g.Padding(0), g.Gap(0),
			g.Tree(g.NoBorder(), g.Grow(), g.ID("outline"), g.Init(func(t *g.TreeView) {
				v.tree = t
				t.Column("", g.Scaled(220))
				t.ShowHeader(false)
				t.OnSelect(v.onOutlineSelect)
				t.OnActivate(v.activateNode)
				v.outlineKeys(t)
			})),
			v.itemsPane()),
		g.VBox(g.Ref(&content), g.Padding(0), g.Gap(0),
			g.Label("", g.Wrap(), g.GrowX(), g.Ref(&v.banner)),
			g.Split(g.Vertical(), g.Grow(), g.Init(func(s *g.SplitView) { s.SetWeights(78, 22) }),
				view.New(v.scene, v.initView, g.Grow(), g.ID("canvas"),
					g.Background(g.Adaptive{Light: light, Dark: dark})),
				v.codePane())),
		v.inspectorPane())
	look.SetSidebar(v.win, sidebar, content, g.Spacing.SidebarWidth)
	look.SetInspector(v.win, content, v.insp.panel, g.Spacing.InspectorWidth)
	v.toolbar()
	v.win.SetSize(1180, 760)
	v.win.SetMinSize(820, 440)
	v.banner.SetVisible(false)
	v.itemsUI.box.SetVisible(false)
	v.win.OnClose(v.onClose)
	v.menus()
	v.contextMenus()
	v.dragAndDrop()
	v.updateTitle()
}

// toolbar is the unified window toolbar: element, item navigator and sidebar buttons, zoom, new item and quick style, inspector, and the
// Goto Item search.
func (v *Viewer) toolbar() {
	look.SetToolbar(v.win,
		look.ToolbarItem{Title: "Element", Sidebar: true, Small: true, Group: []look.ToolbarItem{
			{Title: "New Element", Symbol: "plus", OnClick: func() { v.addElement(v.state.Current) }},
			{Title: "Delete Element", Symbol: "minus", OnClick: v.removeElement},
		}},
		look.ToolbarItem{Title: "Items", Symbol: "list.bullet", Sidebar: true, Small: true, OnClick: v.toggleItems},
		look.ToolbarItem{Title: "Hide Sidebar", Symbol: "sidebar.left", Sidebar: true, Small: true, OnClick: v.toggleSidebar},
		look.ToolbarItem{Title: "Zoom", Small: true, Group: []look.ToolbarItem{
			{Title: "Zoom Out", Symbol: "minus.magnifyingglass", OnClick: func() { v.zoomBy(1 / ZoomStep) }},
			{Title: "Zoom In", Symbol: "plus.magnifyingglass", OnClick: func() { v.zoomBy(ZoomStep) }},
			{Title: "Fit to Window", Symbol: "arrow.up.left.and.arrow.down.right", OnClick: v.fitCommand},
		}},
		look.ToolbarItem{Title: "Reset Zoom", Text: v.zoomText, Small: true, OnClick: v.resetZoom},
		look.ToolbarItem{Space: true},
		look.ToolbarItem{Title: "Item", Inspector: true, Small: true, Group: []look.ToolbarItem{
			{Title: "New Item", Symbol: "plus", OnClick: v.newItem},
			{Title: "Delete Item", Symbol: "minus", OnClick: v.deleteItems},
		}},
		look.ToolbarItem{Title: "Quick Style", Symbol: "paintpalette", Inspector: true, Small: true, OnClick: v.showQuickPanel},
		look.ToolbarItem{Title: "Goto Item", ID: "goto-field", Inspector: true, Small: true, OnSearch: v.searchTo},
		look.ToolbarItem{Title: "Hide Inspector", Symbol: "sidebar.right", Inspector: true, Small: true, OnClick: v.toggleInspector})
}

func (v *Viewer) toggleSidebar() { look.SetSidebarVisible(v.win, !look.SidebarVisible(v.win)) }

func (v *Viewer) toggleInspector() { look.SetInspectorVisible(v.win, !look.InspectorVisible(v.win)) }

func (v *Viewer) fitCommand() {
	if _, _, ok := v.contentSize(); ok {
		v.fitView()
	}
}

// contentSize is the size of what the canvas shows, padding included: a diagram or a scene element.
func (v *Viewer) contentSize() (w, h float64, ok bool) {
	switch {
	case v.rendered != nil:
		return v.rendered.Width, v.rendered.Height, true
	case v.sv != nil:
		return v.sv.w, v.sv.h, true
	}
	return 0, 0, false
}

func (v *Viewer) deleteItems() {
	if v.session != nil {
		v.session.DeleteSelection()
	}
}

func (v *Viewer) newItem() {
	if v.session != nil {
		v.session.AddNewItem(false)
	}
}

// initView sets up navigation (scroll pans, pinch and Cmd+scroll zoom are the view's own) and routes
// pointer and keys to the editor.
func (v *Viewer) initView(vw *view.View) {
	v.view = vw
	vw.SetZoomLimits(MinZoom, MaxZoom)
	// Tab adds an item (Swift keyDown); a traversal would take the focus away first.
	vw.Canvas().Unwrap().AddListener(swt.Traverse, listenerFunc(func(e *swt.Event) {
		if e.Detail == swt.TRAVERSE_TAB_NEXT || e.Detail == swt.TRAVERSE_TAB_PREVIOUS {
			e.Doit = false
		}
	}))
	vw.OnPointer(v.onPointer)
	vw.OnKey(v.onKey)
	vw.OnResize(func(int, int) {
		if v.pendingFit {
			v.applyInitialCamera()
		}
	})
	vw.OnFrameReport(func(r view.FrameReport) {
		if r.Zoom != v.zoomSeen {
			v.zoomSeen = r.Zoom
			v.zoomText.Set(fmt.Sprintf("%d%%", int(math.Round(r.Zoom*100))))
			if v.session != nil {
				v.session.SetZoom(r.Zoom)
				v.drawOverlay()
				v.drawSceneOverlay()
			}
		}
	})
}

func (v *Viewer) isDirty() bool { return v.tracker != nil && v.tracker.Dirty() }

func (v *Viewer) updateTitle() {
	v.win.SetTitle(Title(v.state.Doc.Path, v.isDirty()))
	v.updateMenus()
}

// openPath makes path the document of this window: selection and viewports start over. Unsaved edits
// are asked about first. A file that cannot be read leaves an untitled document and the reason.
func (v *Viewer) openPath(path string) {
	if !v.confirmDiscard() {
		return
	}
	doc, err := Load(path)
	if err != nil {
		if v.session == nil && v.state.Doc.Path == "" {
			v.openText(NewDocumentText, false)
		}
		v.setBanner("Could not open " + filepath.Base(path) + ": " + err.Error())
		return
	}
	v.state.Open(doc)
	v.watch = Watcher{Path: path, Last: doc.Stamp}
	v.conflict = false
	v.id = ""
	v.autosave.Cancel()
	v.startSession()
	v.rebuildOutline(nil)
	v.setBanner(v.bannerText())
	v.sh.addRecent(path)
	v.sh.publish(DocEvent{Path: path, Kind: EventOpen, Version: v.version})
}

// openText makes an untitled document of text. stashed: the text is what the last launch left, so it
// counts as edited.
func (v *Viewer) openText(text string, stashed bool) {
	doc := ParseText(text)
	failed := ""
	if doc.Model == nil {
		failed = "The saved untitled document could not be read: " + errorText(doc)
		doc, stashed = ParseText(NewDocumentText), false
	}
	v.state.Open(doc)
	v.watch = Watcher{}
	v.conflict = false
	v.startSession()
	v.rebuildOutline(nil)
	if stashed && v.tracker != nil {
		v.tracker.saved = "" // never equal to the text, so the document stays edited until it is saved
		v.updateTitle()
	}
	v.setBanner(failed)
}

func (v *Viewer) openDialog() {
	dir := ""
	if v.state.Doc.Path != "" {
		dir = filepath.Dir(v.state.Doc.Path)
	}
	paths := v.win.FileDialog(g.FileDialog{Title: "Open", Dir: dir, Multi: true, Filters: tennFilters})
	for _, p := range paths {
		v.sh.openPath(p)
	}
}

func (v *Viewer) poll() {
	if !v.asking && v.libsChanged() {
		v.sceneReload()
	}
	if v.asking || v.watch.Path == "" || !v.watch.Poll() {
		return
	}
	if DecideReload(v.isDirty()) == ReloadAsk {
		v.asking = true
		discard := v.win.Confirm("File changed on disk",
			"\""+filepath.Base(v.watch.Path)+"\" was changed by another program. Discard your unsaved changes and reload it?")
		v.asking = false
		if !discard {
			v.conflict = true
			return
		}
	}
	v.reload()
}

// reload reads the file again, keeping the selected element, the expanded outline rows and the viewport.
// Unsaved edits are dropped: the callers have asked.
func (v *Viewer) reload() {
	path := v.watch.Path
	if path == "" {
		return
	}
	doc, err := Load(path)
	if err != nil {
		v.setBanner(err.Error())
		return
	}
	v.watch.Last = doc.Stamp
	v.saveCamera()
	prev := v.state.Doc
	expanded := v.expandedRows()
	v.state.Reload(doc)
	v.setBanner(v.bannerText())
	if v.state.Doc == prev {
		return
	}
	v.conflict = false
	v.startSession()
	v.rebuildOutline(expanded)
	v.sh.docChanged(v)
	v.sh.publish(DocEvent{Path: path, Kind: EventReload, Version: v.version})
}

// reloadCommand is File > Reload: it asks before it drops edits.
func (v *Viewer) reloadCommand() {
	if v.watch.Path == "" {
		return
	}
	if DecideReload(v.isDirty()) == ReloadAsk && !v.win.Confirm("Reload", "Discard your unsaved changes and reload the file?") {
		return
	}
	v.reload()
}

func (v *Viewer) setBanner(text string) {
	v.banner.SetText(text)
	v.banner.SetVisible(text != "")
	if text != "" {
		v.banner.SetForeground(g.RGB{R: 200, G: 40, B: 40})
	}
	v.win.Unwrap().Layout()
}

// startSession makes a new editing session on the current element of the document, with its own undo history.
func (v *Viewer) startSession() {
	v.endInline(false)
	v.session, v.tracker, v.shownElem, v.docElem = nil, nil, nil, nil
	if v.state.Doc.Model == nil || v.state.Current == nil {
		v.clearCanvas()
		v.updateTitle()
		return
	}
	s, err := editor.New(v.state.Current, editor.Options{
		// Shadows stay constant in screen points while zooming, as Swift setShadow ignores the zoom CTM.
		Render:        render.Options{DarkMode: v.app.Dark(), ScreenShadows: true},
		PCKeys:        runtime.GOOS != "darwin",
		OnModelChange: v.onChange,
		OnRedraw:      v.onRedraw,
		OnElement:     v.onElement,
		OnEdit:        v.onEdit,
		OnOperation:   v.showOperation,
		OnError:       func(err error) { v.setBanner(err.Error()) },
	})
	if err != nil {
		v.setBanner(err.Error())
		return
	}
	v.session = s
	v.trackDocElem()
	v.adopt(v.state.Doc.Text)
	v.zoomSeen = 0
	v.realize()
	v.refreshProps()
	v.updateTitle()
}

func (v *Viewer) clearCanvas() {
	if old := v.rendered; old != nil {
		v.scene.Update(func(tx *gfx.Tx) { tx.Remove(old.Root) })
	}
	if old := v.sv; old != nil {
		v.scene.Update(func(tx *gfx.Tx) { tx.Remove(old.root) })
	}
	v.rendered, v.overlay, v.sv = nil, nil, nil
	v.refreshProps()
}

// selectElement puts the element on the canvas; the camera of the one left is remembered.
func (v *Viewer) selectElement(e *model.Element) {
	if v.session != nil && e != nil && e != v.state.Current {
		v.session.SetElement(e)
	}
}

func (v *Viewer) onOutlineSelect(n *g.Node) {
	if v.syncing || n == nil {
		return
	}
	switch d := n.Data().(type) {
	case *model.Element:
		v.selectElement(d)
	}
}

// applyInitialCamera places the view for the element on the canvas once the canvas has a size.
func (v *Viewer) applyInitialCamera() {
	w, h := v.view.Canvas().Size()
	if _, _, ok := v.contentSize(); !ok {
		v.pendingFit = false
		return
	}
	if cam, ok := v.state.Camera(v.state.Current); ok {
		v.view.Compositor().SetCamera(cam)
		v.pendingFit = false
		return
	}
	if w <= 0 || h <= 0 {
		return
	}
	v.fitView()
	v.pendingFit = false
}

func (v *Viewer) fitView() {
	w, h := v.view.Canvas().Size()
	cw, ch, _ := v.contentSize()
	v.view.Compositor().SetCamera(FitCamera(cw, ch, float64(w), float64(h)))
}

// saveCamera remembers the viewport of the element on the canvas, unless it was never placed.
func (v *Viewer) saveCamera() {
	if _, _, ok := v.contentSize(); v.state.Current != nil && ok && !v.pendingFit {
		v.state.Remember(v.state.Current, v.view.Compositor().Camera())
	}
}

func (v *Viewer) zoomBy(f float64) {
	w, h := v.view.Canvas().Size()
	comp := v.view.Compositor()
	comp.SetCamera(ZoomCamera(comp.Camera(), f, float64(w)/2, float64(h)/2))
}

// resetZoom is Swift's Reset Zoom: zoom 1, the diagram centered.
func (v *Viewer) resetZoom() {
	cw, ch, ok := v.contentSize()
	if !ok {
		return
	}
	w, h := v.view.Canvas().Size()
	v.view.Compositor().SetCamera(CenterCamera(1, cw, ch, float64(w), float64(h)))
}

// visibleScene is the part of the editor scene the canvas shows now.
func (v *Viewer) visibleScene() render.Rect {
	w, h := v.view.Canvas().Size()
	cam := v.view.Compositor().Camera()
	a := v.smap.ToScene(ViewToScene(cam, gfx.Point{}))
	b := v.smap.ToScene(ViewToScene(cam, gfx.Point{X: float64(w), Y: float64(h)}))
	return render.Rect{X: a.X, Y: b.Y, Width: b.X - a.X, Height: a.Y - b.Y}
}

// outlineKeys is Swift's keyDown of the outline: Tab adds an item to the canvas and moves the focus there.
func (v *Viewer) outlineKeys(t *g.TreeView) {
	tree := t.Unwrap()
	// The traversal would move the focus before the key reaches the key listener.
	tree.AddListener(swt.Traverse, listenerFunc(func(e *swt.Event) {
		if e.Detail == swt.TRAVERSE_TAB_NEXT {
			e.Doit = false
		}
	}))
	tree.AddKeyListener(swt.KeyListenerKeyPressedAdapter(func(e *swt.KeyEvent) {
		if e.KeyCode != int32(swt.TAB) || v.session == nil || v.session.Readonly() {
			return
		}
		e.Doit = false
		v.session.AddTopItem()
		v.view.Canvas().Unwrap().SetFocus()
	}))
}
