//go:build darwin || linux || windows

package app

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"time"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/view"
	"github.com/haiodo/gowt/look"
	"github.com/haiodo/gowt/qa"
	"github.com/haiodo/gowt/rx"
	"github.com/haiodo/gowt/swt"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/geometry"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/palette"
	"github.com/haiodo/tenniarb/go/scene"
)

// Viewer is the window: the outline as the sidebar, the diagram of the selected element, and the
// inspector on the right, editable through an editor.Session.
type Viewer struct {
	app *g.App
	sh  *Shell
	// geomSize is the item sizes of the classic diagram for the expression engine, and geomSizeEl the
	// element they were built for.
	geomSize   func(*model.DiagramItem, model.IExecutionContext) *model.DrawableSize
	geomSizeEl *model.Element
	// previewBoxes is where a drag has taken the boxes, before the model is written.
	previewBoxes map[*model.DiagramItem]geometry.Point
	win          *g.Window
	tree         *g.TreeView
	banner       *g.TextView
	bannerCopy   *g.ButtonView // the Copy button next to the banner
	bannerFix    *g.ButtonView // the Fix button, shown when the banner names a diagnostic the fixer repairs
	note         string        // the note in the banner, cleared by the next action
	view         *view.View
	scene        *gfx.Scene

	*docCore
	// conflict: the file changed on disk while the document had unsaved edits and the edits were kept.
	conflict bool
	asking   bool // a modal question is open; the poll waits

	selHUD      *gfx.Node    // the selection frames of a scene, in the content scene
	diffHUD     *gfx.Node    // the frames of compare mode, on the HUD
	sv          *sceneView   // the element on the canvas is drawn by go/scene
	svCache     []*sceneView // views taken off the canvas, the last one first
	cmp         *compare     // compare mode: the file against a version from git
	repoDir     string       // the folder the repository of the document was looked up in
	repoRoot    string       // its repository, when it has one
	repoOK      bool
	compareItem *g.MenuItem // View > Compare with Git, off outside a repository
	itemsUI     itemsList
	addClassic  []*g.MenuItem // the diagram commands of Edit > Add, off in a scene
	moveItems   []*g.MenuItem // Element > Move Left/Right, on for a selected scene node
	suggestItem *g.MenuItem   // Element > Suggest Improvements, on for a scene
	sug         suggestUI
	sceneBuilds int // how many times go/scene drew the canvas, for the tests
	smap        SceneMap
	shownElem   *model.Element // the element the camera was placed for
	nextID      int
	pendingFit  bool // the canvas had no size yet when the element was shown
	syncing     bool // the outline selection is being set by code
	outlineAuto bool // code cleared the outline selection: the toolkit may select the first row itself
	zoomSeen    float64
	zoomText    *rx.State[string] // the toolbar zoom item, like Swift zoomLabel
	exporting   exportProgress    // the pages export running in the background, a toolbar item until done
	ind         indicators
	agent       agentUI
	marks       markUI
	cm          commentUI

	animFrom   time.Time     // the clock of the animations of the shown scene
	animScreen *scene.Screen // the screen whose animations play
	animCancel func()        // stops the frame timer of the animations
	animTicks  int           // frames played, for the QA state
	animMS     float64       // the average cost of a frame, ms
	animOff    bool          // the playback is paused by the scrub of the inspector

	press      press
	dragAt     geometry.Point // the pointer of the move in flight, in session coordinates, for the classic drag preview
	inline     *view.Editor
	inlineMode editor.EditMode
	activeText *swt.Text // a text field of the window that has the keyboard: clipboard commands go to it
	popover    *g.Popover
	outlineSig string
	props      props
	nav        navUI
	insp       *inspector
	search     search
	docElem    *model.Element // the last element on the canvas that is not in a mounted file
	docSel     []string       // its selected scene nodes, for the return from a mounted file
	dragging   *model.Element // the outline row a drag started on
	dropAt     [2]int         // where the drag was last over the outline

	recentColors []string // colours chosen in the picker, newest first

	undoItem, redoItem *g.MenuItem
	canvasMenu         *g.Menu
	menuOverride       []editor.Entry // entries for the next opening of the canvas menu
	rightClick         geometry.Point // where the canvas menu was asked for
	recentMenu         *g.Menu
	autoQuick          bool // show the quick style panel after a click on an item

	id         string // names the stash of an untitled document
	autosave   *Debounce
	agentEdits bool // the last change came from an agent: written by the every-tick of the autosave
	frameSave  *Debounce
	force      bool // the window closes without questions (QA)
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
	v.raise()
	return v
}

// raise brings the window and the application to the front, as AppKit does when a document is opened
// from Finder or with `open`; a QA run leaves the focus where the test put it.
func (v *Viewer) raise() {
	if os.Getenv(qa.EnvSocket) != "" {
		return
	}
	v.win.Unwrap().ForceActive()
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
	l, _ := palette.Parse(p.Background)
	d, _ := palette.Parse(p.BackgroundDark)
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
	v.refreshOutline()
	v.sizeOutlineColumn() // a denser theme moves the rows and the scrollbar
	v.itemsUI.sig = ""    // the grey rows take the colour of the new theme
	v.rebuildItems()
	v.sizeItemsAdd()
	v.syncIndicators()
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
	if runtime.GOOS == "windows" {
		look.SetBackdrop(v.win, look.BackdropGlass)
	}
	var sidebar, content, codePane *g.Panel
	v.win.Add(
		g.VBox(g.Ref(&sidebar), g.Padding(0), g.Gap(0),
			g.Split(g.Vertical(), g.Divider(), g.Grow(), g.Init(func(s *g.SplitView) { s.SetWeights(40, 60) }),
				g.Tree(g.NoBorder(), g.Grow(), g.ID("outline"), g.Init(func(t *g.TreeView) {
					v.tree = t
					t.Column("", g.Scaled(220))
					t.ShowHeader(false)
					t.Unwrap().AddControlListener(swt.ControlListenerControlResizedAdapter(func(*swt.ControlEvent) { v.sizeOutlineColumn() }))
					v.sizeOutlineColumn()
					t.OnSelect(v.onOutlineSelect)
					t.OnActivate(v.activateNode)
					v.outlineKeys(t)
				})),
				v.itemsPane())),
		g.VBox(g.Ref(&content), g.Padding(0), g.Gap(0),
			g.HBox(g.GrowX(), g.Padding(0), g.Gap(g.Scaled(6)),
				g.Text(g.ID("banner"), g.Multiline(), g.ReadOnly(), g.NoBorder(), g.GrowX(),
					g.Size(0, g.Scaled(36)), g.Ref(&v.banner)),
				g.Button("Fix", g.Small(), g.Ref(&v.bannerFix), g.ID("banner-fix"), g.Init(func(b *g.ButtonView) {
					b.SetTooltip("Quote the titles this diagnostic names, as one undo step")
					b.OnClick(func() { v.fixDiagnostics() })
				})),
				g.Button("Copy", g.Small(), g.Ref(&v.bannerCopy), g.Init(func(b *g.ButtonView) {
					b.OnClick(func() { v.setClipboard(v.banner.Text()) })
				}))),
			view.New(v.scene, v.initView, g.Grow(), g.ID("canvas"),
				g.Background(g.Adaptive{Light: light, Dark: dark}))),
		v.codePane(&codePane),
		v.inspectorPane())
	look.SetSidebar(v.win, sidebar, content, g.Spacing.SidebarWidth)
	look.SetSidebarFullHeight(v.win, true) // the outline scrolls under the toolbar, as Finder's sidebar does
	look.SetBottomPanel(v.win, content, codePane, g.Scaled(200))
	look.SetInspector(v.win, content, v.insp.panel, inspectorWidth())
	look.SetInspectorFixed(v.win, true)
	v.toolbar()
	v.win.SetSize(1180, 760)
	v.win.SetMinSize(820, 440)
	v.showBanner(false)
	v.win.OnClose(v.onClose)
	v.menus()
	v.contextMenus()
	v.dragAndDrop()
	v.dragComponents()
	v.updateTitle()
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
	vw.OnScroll(v.onScroll)
	vw.OnKey(v.onKey)
	vw.OnResize(func(int, int) {
		if v.pendingFit {
			v.applyInitialCamera()
		}
	})
	vw.Compositor().OnViewport(func(_, _, _ float64) { v.syncIndicators() })
	vw.OnFrameReport(func(r view.FrameReport) {
		if cam := (gfx.Camera{Origin: r.Origin, Zoom: r.Zoom}); cam != v.ind.seen {
			v.ind.seen = cam
			v.syncIndicators()
			v.drawSceneOverlay()
		}
		if r.Zoom != v.zoomSeen {
			v.zoomSeen = r.Zoom
			v.zoomText.Set(fmt.Sprintf("%d%%", int(math.Round(r.Zoom*100))))
			if v.session != nil {
				// The window draws the glow in screen points, so the picture does not depend on the zoom.
				v.session.SetZoom(r.Zoom)
			}
		}
	})
}
