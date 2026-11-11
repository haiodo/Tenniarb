package app

import (
	"errors"
	"fmt"
	"github.com/haiodo/tenniarb/go/editor"
	"os"
	"path/filepath"
	"runtime/debug"

	tennlib "github.com/haiodo/tenniarb/go/library"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/scene"
)

// untitledName is the file name an untitled document has for the scene loader.
const untitledName = "Untitled.tenn"

// docCore is the document of a window without the window, so the API for the MCP service and the
// tests can use it without one.
type docCore struct {
	state   *State
	watch   Watcher
	session *editor.Session
	tracker *Tracker

	// text is what Save writes. A text edit splices it, so the rest of the file stays as written. A
	// classic edit prints the model (Swift printer) while printed, else only the elements it changed.
	text    string
	textSet bool // a text edit has set text; the session's change callback keeps it
	printed bool // a diagram document as read: the Swift printer owns its text
	version int
	kind    model.DocKind
	shape   shape // the element tree the text holds
	restore restore

	root     string      // workspace root: mounted files may not leave it
	stree    *scene.Tree // nil for a diagram document
	sceneErr string      // the errors go/scene finds in this file, for the banner
	entry    string      // the entry document the file is loaded in the context of, "" when alone
	libs     map[string]Stamp
	selNodes []string // selected scene nodes of the current element, by node path
	builtin  bool     // the document is a file of a library the app ships or has installed: read-only

	// Mounted files (mountdoc.go): the text held for each by relative path, the mounting elements, the
	// ones edited since the tree was read, and the error of the last write to a window that has the file.
	libDocs map[string]*libDoc
	mounts  map[*model.Element]*mountDoc
	keep    []*model.Element
	libErr  error
	libBusy bool // a window that has a mounted file is being edited by this document

	// pending is the tree loaded to check an edit, which loadScene takes when it would load the same.
	pending *pendingScene

	// peer gives the document another window holds for the mounted file rel; nil when none.
	peer func(self *docCore, rel string) *docCore

	// others gives the text of the other open documents under root, by slash path relative to it.
	others func(root string) map[string]string
}

// WorkspaceRoot is the root mounted files resolve in: the nearest folder up from the file that holds .git or
// a tenniarb.tenn, else the folder of the file.
func WorkspaceRoot(path string) string {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "."
		}
		return home
	}
	dir := filepath.Dir(path)
	for d := dir; ; {
		for _, mark := range []string{".git", "tenniarb.tenn"} {
			if _, err := os.Stat(filepath.Join(d, mark)); err == nil {
				return d
			}
		}
		up := filepath.Dir(d)
		if up == d {
			return dir
		}
		d = up
	}
}

// rel is the path of the document relative to the workspace root, with slashes.
func (d *docCore) rel() string {
	if d.state.Doc.Path == "" {
		return untitledName
	}
	r, err := filepath.Rel(d.root, d.state.Doc.Path)
	if err != nil {
		return filepath.Base(d.state.Doc.Path)
	}
	return filepath.ToSlash(r)
}

// libraryFile reports whether path is a file of a library the application ships or has installed: the
// text lives outside the workspace, so a write here lands nowhere the next load reads. The folder a
// scheme mounts under is not the workspace's either.
func (d *docCore) libraryFile(path string) bool {
	if path == "" {
		return false
	}
	if rel, err := filepath.Rel(d.root, path); err == nil && scene.IsLibraryMount(filepath.ToSlash(rel)) {
		return true
	}
	return tennlib.Default(d.root).Contains(path)
}

// errLibraryReadOnly is the refusal of an edit or a save of a built-in library.
func errLibraryReadOnly(file string) error {
	return fmt.Errorf("%s is a library of Tenniarb, not a file of the workspace: read-only", filepath.Base(file))
}

// adopt takes the text of a document that was just read into a new session. A diagram document keeps
// the text of the session, as before: the file is written by the Swift printer from the first edit on.
func (d *docCore) adopt(raw string) {
	d.root = WorkspaceRoot(d.state.Doc.Path)
	d.kind = model.KindOfModel(d.session.Root())
	d.text = raw
	d.printed = d.kind == model.DocDiagram
	if d.printed {
		d.text = d.session.Text()
	}
	d.shape = shapeOf(&d.session.Root().Element)
	d.restore = restore{}
	d.textSet = false
	d.selNodes = nil
	d.tracker = NewTracker(d.text)
	d.version++
	d.session.Store().MountWritable, d.session.Store().OnMountEdit = d.mountWritable, d.mountEdited
	d.builtin = d.libraryFile(d.state.Doc.Path)
	d.loadScene()
	d.session.SetReadonly(d.builtin)
	debug.FreeOSMemory() // the parse of a big document leaves garbage the runtime would hold for a while
}

// sessionChanged follows a change of the model: a classic edit, a text edit, undo or redo.
func (d *docCore) sessionChanged() {
	switch {
	case d.textSet:
		d.textSet = false
	case d.printed:
		d.text = d.session.Text()
	default:
		text, ok := d.spliceClassic()
		if !ok {
			// The text no longer pairs with the model: print it whole rather than lose a change.
			text = d.session.Text()
		}
		d.text = text
	}
	if d.tracker != nil {
		d.tracker.Changed(d.text)
	}
	d.version++
	d.kind = model.KindOfModel(d.session.Root())
	if d.kind != model.DocDiagram {
		d.printed = false
	}
	d.shape = shapeOf(&d.session.Root().Element)
	d.loadScene()
}

// drawsWithScene reports whether go/scene draws e on the canvas.
func (d *docCore) drawsWithScene(e *model.Element) bool {
	return d.stree != nil && e != nil
}

// libsChanged reports whether an imported file changed on disk since the tree was read.
func (d *docCore) libsChanged() bool {
	for abs, s := range d.libs {
		if now, err := StampOf(abs); err == nil && now != s {
			return true
		}
	}
	return false
}

// imports reports whether the tree holds the file abs (another open document).
func (d *docCore) imports(abs string) bool {
	_, ok := d.libs[abs]
	return ok
}

// isScene reports whether go/scene draws e.
func (d *docCore) isScene(e *model.Element) bool {
	return d.stree != nil && e != nil && model.SceneElement(e)
}

// file is the scene root of the document's own file.
func (d *docCore) file() *scene.Node {
	if d.stree == nil {
		return nil
	}
	return d.stree.File(d.rel())
}

// elementNode is the scene node of the element e: the file root for the root element.
func (d *docCore) elementNode(e *model.Element) *scene.Node {
	cur := d.file()
	if cur == nil || e == nil {
		return nil
	}
	for _, seg := range PathOf(e) {
		name, n := splitIndex(seg)
		var next *scene.Node
		for _, c := range cur.Elems {
			if c.Name == name {
				if n == 0 {
					next = c
					break
				}
				n--
			}
		}
		if next == nil {
			return nil
		}
		cur = next
	}
	return cur
}

// ErrStale is the error of an edit made against a version of the text that is not the current one.
var ErrStale = errors.New("the document changed since that version")
