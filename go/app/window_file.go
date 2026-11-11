//go:build darwin || linux || windows

package app

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/check"
	"github.com/haiodo/tenniarb/go/editor"
)

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
	if v.win != nil && len(doc.Notes) > 0 {
		v.win.MessageBox(g.Message{Title: "Opened with notes", Icon: g.IconInfo, Buttons: g.ButtonsOK,
			Text: fmt.Sprintf("%d notes:\n\n%s", len(doc.Notes), strings.Join(doc.Notes, "\n"))})
	}
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
	paths := v.win.FileDialog(g.FileDialog{Title: "Open", Dir: dir, Multi: true, Filters: fileFilters})
	for _, p := range paths {
		v.sh.openPath(p)
	}
}

func (v *Viewer) poll() {
	v.agentTick()
	v.syncComments()
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

func (v *Viewer) updateTitle() {
	v.setTitle()
	v.updateMenus()
}

// setTitle shows the entry document when the file is loaded in its context: "part.tenn - in Design".
func (v *Viewer) setTitle() {
	title := Title(v.state.Doc.Path, v.edited())
	if v.entry != "" {
		title += " - in " + strings.TrimSuffix(filepath.Base(v.entry), ".tenn")
	}
	v.win.SetTitle(title)
}

func (v *Viewer) isDirty() bool { return v.tracker != nil && v.tracker.Dirty() }

// edited reports whether the document or a file it shows has changes that are not on disk: an edit
// inside a mounted element dirties the mounted file, not the document.
func (v *Viewer) edited() bool { return v.isDirty() || v.libsDirty() }

// startSession makes a new editing session on the current element of the document, with its own undo history.
func (v *Viewer) startSession() {
	v.endInline(false)
	v.session, v.tracker, v.shownElem, v.docElem = nil, nil, nil, nil
	v.cmp = nil // the comparison was of the document that was open
	v.agentEdits = false
	v.syncItemsAdd()
	if v.state.Doc.Model == nil || v.state.Current == nil {
		v.clearCanvas()
		v.updateTitle()
		return
	}
	s, err := editor.New(v.state.Current, editor.Options{
		PCKeys:        runtime.GOOS != "darwin",
		OnModelChange: v.onChange,
		OnRedraw:      v.onRedraw,
		OnElement:     v.onElement,
		OnEdit:        v.onEdit,
		OnOperation:   v.showOperation,
		OnError:       v.showStoreError,
		// The picture has the last word on what a press finds: the canvas knows what it drew there.
		Covers: v.drawnCovers,
		// The session asks the canvas where the items are: one engine draws and answers for the diagram.
		Geom: canvasGeom{v: v},
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
	if old := v.sv; old != nil {
		v.scene.Update(func(tx *gfx.Tx) { tx.Remove(old.root) })
	}
	v.sv = nil
	v.drawSceneOverlay()
	v.refreshProps()
}

// setBanner shows text in the banner in the colour of an error.
func (v *Viewer) setBanner(text string) {
	v.note = ""            // an error takes the banner over from a note and stays
	v.banner.SetText(text) // the text decides whether Fix is offered, so it goes first
	v.showBanner(text != "")
	if text != "" {
		v.banner.SetForeground(g.RGB{R: 200, G: 40, B: 40})
	}
	v.win.Unwrap().Layout()
}

// showBanner shows or hides the banner with its Copy button, and its Fix button when the message names a
// diagnostic the fixer repairs: the text of an error is selectable, and a reader without a terminal has the
// fix here.
func (v *Viewer) showBanner(on bool) {
	v.banner.SetVisible(on)
	v.bannerCopy.SetVisible(on)
	v.bannerFix.SetVisible(on && check.Fixable(v.banner.Text()))
}

// noteTime is how long a note about a finished action stays in the banner.
const noteTime = 6 * time.Second

// setNote shows a line of information in the banner, in the text colour, until something else replaces it.
func (v *Viewer) setNote(text string) {
	v.note = text
	c := g.RGB{R: 80, G: 80, B: 80}
	if v.app.Dark() {
		c = g.RGB{R: 200, G: 200, B: 200}
	}
	v.banner.SetText(text) // the text decides whether Fix is offered, so it goes first
	v.showBanner(text != "")
	if text != "" {
		v.banner.SetForeground(c)
	}
	v.win.Unwrap().Layout()
}

// noteFor shows a note that goes away by itself: what a finished action did stays long enough to read.
func (v *Viewer) noteFor(text string) {
	v.setNote(text)
	v.app.After(noteTime, func() {
		if v.win.Unwrap().IsDisposed() || v.note != text {
			return
		}
		v.setBanner(v.bannerText())
	})
}
