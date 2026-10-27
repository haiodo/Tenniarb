//go:build darwin || linux || windows

package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/offscreen"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
	"github.com/haiodo/tenniarb/go/scene"
)

// The document API of the shell, for the MCP service: every method may be called from any goroutine
// and runs on the UI thread. A path names an open document; "" is the active one.

// EventKind says what happened to a document.
type EventKind string

const (
	EventOpen   EventKind = "open"
	EventChange EventKind = "change" // the text changed: an edit, undo or redo
	EventReload EventKind = "reload" // the file was read again from disk
	EventSave   EventKind = "save"
	EventClose  EventKind = "close"
	EventSelect EventKind = "select" // the scene selection changed
)

// DocEvent is a change of an open document. Version is the version of its text after it.
type DocEvent struct {
	Path    string
	Kind    EventKind
	Version int
}

// DocInfo describes an open document.
type DocInfo struct {
	Path    string // "" for an untitled document
	Dirty   bool
	Kind    model.DocKind
	Version int // grows with every change of the text
	Active  bool
}

// Selection is what a document shows: the element on the canvas (PathOf), the selected scene nodes
// of it (NodePath) and the selected classic items (by name).
type Selection struct {
	Element []string
	Nodes   []string
	Items   []string
}

// ErrNoDocument: no open document has that path.
var ErrNoDocument = errors.New("no open document with that path")

// ui runs f on the UI thread and waits.
func (sh *Shell) ui(f func()) {
	if sh.app == nil {
		f()
		return
	}
	sh.app.Sync(f)
}

func (sh *Shell) viewer(path string) (*Viewer, error) {
	if path == "" {
		if sh.active != nil {
			return sh.active, nil
		}
		return nil, ErrNoDocument
	}
	want := realPath(path)
	for _, v := range sh.viewers {
		if v.state.Doc.Path != "" && realPath(v.state.Doc.Path) == want {
			return v, nil
		}
	}
	return nil, ErrNoDocument
}

// realPath is p absolute with symlinks resolved, as go/scene names files.
func realPath(p string) string {
	if a, err := filepath.Abs(p); err == nil {
		p = a
	}
	if r, err := filepath.EvalSymlinks(p); err == nil {
		p = r
	}
	return p
}

// Documents lists the open documents in the order their windows opened.
func (sh *Shell) Documents() []DocInfo {
	var out []DocInfo
	sh.ui(func() {
		for _, v := range sh.viewers {
			out = append(out, DocInfo{Path: v.state.Doc.Path, Dirty: v.isDirty(), Kind: v.kind, Version: v.version, Active: v == sh.active})
		}
	})
	return out
}

// Text is the text of a document as Save would write it, and its version.
func (sh *Shell) Text(path string) (text string, version int, err error) {
	sh.ui(func() {
		var v *Viewer
		if v, err = sh.viewer(path); err == nil {
			text, version = v.text, v.version
		}
	})
	return
}

// Selection is the element on the canvas and what is selected in it.
func (sh *Shell) Selection(path string) (sel Selection, err error) {
	sh.ui(func() {
		var v *Viewer
		if v, err = sh.viewer(path); err != nil || v.state.Current == nil {
			return
		}
		sel.Element = PathOf(v.state.Current)
		sel.Nodes = slices.Clone(v.selNodes)
		if v.session != nil {
			for _, it := range v.session.Selection() {
				sel.Items = append(sel.Items, it.Name)
			}
		}
	})
	return
}

// Select puts the element of sel on the canvas, selects its nodes or items and brings them into view.
func (sh *Shell) Select(path string, sel Selection) (err error) {
	sh.ui(func() {
		var v *Viewer
		if v, err = sh.viewer(path); err != nil {
			return
		}
		err = v.selectAPI(sel)
	})
	return
}

func (v *Viewer) selectAPI(sel Selection) error {
	if v.session == nil {
		return errors.New("the document has parse errors")
	}
	e := Resolve(&v.session.Root().Element, sel.Element)
	if e == nil {
		return fmt.Errorf("no element %q", sel.Element)
	}
	if len(sel.Nodes) > 0 {
		el := v.elementNode(e)
		for _, p := range sel.Nodes {
			if NodeAt(el, p) == nil {
				return fmt.Errorf("no node %q in the element", p)
			}
		}
	}
	var items []*model.DiagramItem
	for _, name := range sel.Items {
		i := slices.IndexFunc(e.Items, func(it *model.DiagramItem) bool { return it.Name == name })
		if i < 0 {
			return fmt.Errorf("no item %q in the element", name)
		}
		items = append(items, e.Items[i])
	}
	v.showSelection(e, slices.Clone(sel.Nodes), items, true)
	return nil
}

// showSelection puts e on the canvas with nodes (scene, by NodePath) or items (classic) selected.
func (v *Viewer) showSelection(e *model.Element, nodes []string, items []*model.DiagramItem, reveal bool) {
	if e != v.state.Current {
		v.session.SetElement(e)
	}
	if len(items) > 0 || len(v.session.Selection()) > 0 {
		v.session.Select(items)
	}
	if v.win == nil {
		v.selNodes = nodes
		return
	}
	if v.sceneMode() {
		v.selectScene(nodes)
		if reveal {
			v.revealScene()
		}
	}
}

// ApplyEdit replaces [e.Start, e.End) of the text by e.New as one undo step of the document and
// returns the new text and version. version 0 skips the check that the edit was made against the
// current text; another version gives ErrStale. A text that does not parse is refused.
func (sh *Shell) ApplyEdit(path string, version int, e TextEdit) (text string, ver int, err error) {
	sh.ui(func() {
		var v *Viewer
		if v, err = sh.viewer(path); err != nil {
			return
		}
		if version != 0 && version != v.version {
			err = ErrStale
			return
		}
		if err = v.applyText(e.Start, e.End, e.New); err == nil {
			text, ver = v.text, v.version
		}
	})
	return
}

// RenderPNG draws what the canvas of the document shows at scale pixels per point, the selection
// frame included. A document without a window gives its element on the canvas whole.
func (sh *Shell) RenderPNG(path string, scale float64) (png []byte, err error) {
	if scale <= 0 {
		scale = 1
	}
	sh.ui(func() {
		var v *Viewer
		if v, err = sh.viewer(path); err != nil {
			return
		}
		png, err = v.renderPNG(scale)
	})
	return
}

func (v *Viewer) renderPNG(scale float64) ([]byte, error) {
	bg, _ := render.ParseColor(v.sh.prefs.Background)
	if v.view != nil {
		r, err := offscreen.Default()
		if err != nil {
			return nil, err
		}
		w, h := v.view.Canvas().Size()
		cam := v.view.Compositor().Camera()
		area := gfx.R(cam.Origin.X, cam.Origin.Y, float64(w)/cam.Zoom, float64(h)/cam.Zoom)
		raster, err := gfx.RenderSubtree(r, v.scene, nil, area, scale*cam.Zoom, bg)
		if err != nil {
			return nil, err
		}
		return raster.PNG()
	}
	e := v.state.Current
	if e == nil {
		return nil, errors.New("the document has parse errors")
	}
	if !v.isScene(e) {
		return render.PNG(e, exportOptions(), scale, bg)
	}
	s := gfx.NewScene()
	var screen *scene.Screen
	var err error
	s.Update(func(tx *gfx.Tx) {
		screen, err = v.stree.BuildElement(tx, nil, v.elementNode(e), scene.Options{Measurer: gfx.DefaultMeasurer})
	})
	if err != nil {
		return nil, err
	}
	return offscreen.PNG(s, screen.Root, scale, bg)
}

// Subscribe calls f on the UI thread after every change of an open document, until cancel. f must
// not block: hand the event to another goroutine.
func (sh *Shell) Subscribe(f func(DocEvent)) (cancel func()) {
	var id int
	sh.ui(func() {
		sh.nextSub++
		id = sh.nextSub
		if sh.subs == nil {
			sh.subs = map[int]func(DocEvent){}
		}
		sh.subs[id] = f
	})
	return func() { sh.ui(func() { delete(sh.subs, id) }) }
}

func (sh *Shell) publish(e DocEvent) {
	for _, id := range sortedKeys(sh.subs) {
		sh.subs[id](e)
	}
}

func sortedKeys(m map[int]func(DocEvent)) []int {
	ids := make([]int, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// othersText is the text of the open documents under root, by slash path relative to it: go/scene
// reads them instead of the files, so a screen shows a library as it is being edited.
func (sh *Shell) othersText(root string) map[string]string {
	out := map[string]string{}
	base := realPath(root)
	for _, v := range sh.viewers {
		if rel, ok := v.relIn(base); ok {
			out[rel] = v.text
		}
	}
	return out
}

// relIn is the slash path of the document relative to base (a real path); false for a document without
// a file or outside base.
func (v *Viewer) relIn(base string) (string, bool) {
	if v.session == nil || v.state.Doc.Path == "" {
		return "", false
	}
	rel, err := filepath.Rel(base, realPath(v.state.Doc.Path))
	if err != nil || rel == ".." || len(rel) > 2 && rel[:3] == ".."+string(filepath.Separator) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// peerDoc is the document of another window that has the file rel under the root of self.
func (sh *Shell) peerDoc(self *docCore, rel string) *docCore {
	base := realPath(self.root)
	for _, v := range sh.viewers {
		if r, ok := v.relIn(base); ok && r == rel && v.docCore != self {
			return v.docCore
		}
	}
	return nil
}

// docChanged reads the scene tree again in the documents that import the one that changed.
func (sh *Shell) docChanged(src *Viewer) {
	if src.state.Doc.Path == "" {
		return
	}
	abs := realPath(src.state.Doc.Path)
	for _, v := range slices.Clone(sh.viewers) {
		if v != src && v.stree != nil && !v.libBusy && v.imports(abs) {
			v.sceneReload()
		}
	}
}

// sceneReload reads the scene tree again after an imported file changed, and redraws.
func (v *Viewer) sceneReload() {
	v.loadScene()
	if v.win == nil {
		return
	}
	if v.session != nil && v.isScene(v.state.Current) {
		v.realizeScene()
	}
	v.rebuildOutline(v.expandedRows())
}
