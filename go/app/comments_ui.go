//go:build darwin || linux || windows

package app

import (
	"errors"
	"slices"
	"time"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/comments"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/servicewire"
)

var _ servicewire.CommentsReceiver = provider{}

// Comments is called by the service client on one of its goroutines.
func (p provider) Comments(up servicewire.CommentsUpdate) {
	p.sh.ui(func() {
		if v, _, err := p.sh.viewerShowing(up.Path); err == nil {
			v.commentsArrived(up)
		}
	})
}

// commentUI is the comments of the document in a window.
type commentUI struct {
	path     string // the document the list belongs to
	rev      int64
	doc      []comments.Comment // the comments of the document itself, as stored
	files    map[string]*fileComments
	list     []comments.Comment // doc and the files it shows, anchors in paths of the document
	mode     bool               // a click on the canvas places a comment
	resolved bool               // resolved comments keep their pins

	pins []Pin
	node *gfx.Node // the HUD node of the pins
	key  pinKey

	pop    *g.Popover // the thread that is open
	thread string     // its comment
	draft  string     // the unsent reply
}

// fileComments are the comments of a mounted file the document shows, and where its elements are in
// the document: the path of the first element that shows the file.
type fileComments struct {
	rev    int64
	list   []comments.Comment
	prefix []string
}

// pinKey is what the picture of the pins depends on besides the pins.
type pinKey struct {
	cam  gfx.Camera
	w, h float64
	mode bool
}

// sendComment runs op in the service; when no service runs the app writes the files itself, as nobody
// else does then.
func (sh *Shell) sendComment(op servicewire.CommentsOp) (servicewire.CommentsUpdate, error) {
	if sh.svc != nil {
		up, err := sh.svc.Comments(op)
		if !errors.Is(err, servicewire.ErrNotConnected) && !errors.Is(err, servicewire.ErrClosed) {
			return up, err
		}
	}
	return sh.localComment(op)
}

func (sh *Shell) localComment(op servicewire.CommentsOp) (servicewire.CommentsUpdate, error) {
	if sh.dir == "" {
		return servicewire.CommentsUpdate{}, errors.New("no config folder")
	}
	sh.cstoreOnce.Do(func() { sh.cstore = comments.NewStore(comments.DirIn(sh.dir)) })
	key := sh.repos.DocKey(op.Path)
	var d comments.Doc
	var err error
	if op.Op == servicewire.CommentList {
		d, err = sh.cstore.Load(key)
	} else {
		d, err = sh.cstore.Update(key, func(d *comments.Doc) error {
			_, _, err := op.Apply(d, time.Now())
			return err
		})
	}
	return servicewire.CommentsUpdate{V: servicewire.CommentsVersion, Path: op.Path, Rev: d.Rev, Comments: d.Comments}, err
}

// syncComments loads the comments when the window shows another file; the poll calls it. Later changes
// come from the service.
func (v *Viewer) syncComments() {
	path := v.state.Doc.Path
	if path == v.cm.path || v.sh.dir == "" || v.app == nil {
		return
	}
	v.cm.path, v.cm.rev, v.cm.list, v.cm.doc, v.cm.files = path, 0, nil, nil, nil
	v.syncPins()
	if path == "" {
		return
	}
	v.loadComments(realPath(path), true)
	for file := range v.mountPrefixes() {
		v.loadComments(file, false)
	}
}

// loadComments lists the comments of file: the document itself, or a mounted file it shows. Reading the
// store costs no service call; the changes come from the service as they happen.
func (v *Viewer) loadComments(file string, own bool) {
	path := v.state.Doc.Path
	op := servicewire.CommentsOp{Path: file, Op: servicewire.CommentList}
	go func() {
		up, err := v.sh.localComment(op)
		v.app.Async(func() {
			if err != nil || v.cm.path != path {
				return
			}
			v.commentsArrived(up)
		})
	}()
}

// commentsArrived takes an update of the document or of a mounted file it shows.
func (v *Viewer) commentsArrived(up servicewire.CommentsUpdate) {
	if realPath(v.state.Doc.Path) == realPath(up.Path) {
		v.onComments(up)
		return
	}
	v.onFileComments(realPath(up.Path), up)
}

// commentOp changes the comments of the document off the UI thread; the answer comes back as an update.
func (v *Viewer) commentOp(op servicewire.CommentsOp) {
	path := v.state.Doc.Path
	if path == "" {
		return
	}
	file := ""
	if op.Op == servicewire.CommentAdd && op.Anchor != nil {
		if f, a, ok := v.anchorInFile(*op.Anchor); ok {
			file, op.Anchor = f, &a
		}
	}
	if file == "" {
		file = v.commentFile(op.ID)
	}
	if file == "" {
		file = realPath(path)
	}
	op.Path = file
	go func() {
		up, err := v.sh.sendComment(op)
		v.app.Async(func() {
			if err != nil {
				v.notice("Comment: " + err.Error())
				return
			}
			v.commentsArrived(up)
		})
	}()
}

// onComments takes the comments of the document. Messages are handled concurrently, so one older than
// the list on screen is dropped.
func (v *Viewer) onComments(up servicewire.CommentsUpdate) {
	if up.Rev < v.cm.rev {
		return
	}
	v.cm.rev, v.cm.doc = up.Rev, up.Comments
	v.rebuildComments()
}

// onFileComments takes the comments of a mounted file the document shows.
func (v *Viewer) onFileComments(file string, up servicewire.CommentsUpdate) {
	fc := v.cm.files[file]
	if fc == nil {
		fc = &fileComments{prefix: v.mountPrefixes()[file]}
		if v.cm.files == nil {
			v.cm.files = map[string]*fileComments{}
		}
		v.cm.files[file] = fc
	}
	if up.Rev < fc.rev {
		return
	}
	fc.rev, fc.list = up.Rev, up.Comments
	v.rebuildComments()
}

// rebuildComments puts the comments of the document and of the files it shows in one list, the anchors of
// the mounted ones moved into the document, so the pins and the thread work with paths of the canvas.
func (v *Viewer) rebuildComments() {
	list := slices.Clone(v.cm.doc)
	for _, fc := range v.cm.files {
		for _, c := range fc.list {
			c.Anchor.Element = append(slices.Clone(fc.prefix), c.Anchor.Element...)
			list = append(list, c)
		}
	}
	v.cm.list = list
	v.syncPins()
	if id := v.cm.thread; v.cm.pop != nil && id != "" {
		if v.findComment(id) == nil {
			v.closePopover()
		} else {
			v.showThread(id)
		}
	}
}

// mountPrefixes maps the absolute path of a mounted file to the path in the document of the first
// element that shows it: a file mounted twice gets one place for its comments.
func (v *Viewer) mountPrefixes() map[string][]string {
	out := map[string][]string{}
	m := v.state.Doc.Model
	if m == nil {
		return out
	}
	var walk func(e *model.Element)
	walk = func(e *model.Element) {
		if md := v.mounts[e]; md != nil {
			if abs := realPath(v.libPath(md.file)); out[abs] == nil {
				out[abs] = PathOf(e)
			}
		}
		for _, c := range e.Elements {
			walk(c)
		}
	}
	walk(&m.Element)
	return out
}

// commentFile is the file a comment is stored with: the mounted file it points into, else "".
func (v *Viewer) commentFile(id string) string {
	for file, fc := range v.cm.files {
		for i := range fc.list {
			if fc.list[i].ID == id {
				return file
			}
		}
	}
	return ""
}

// anchorInFile moves an anchor of the document into the mounted file that holds its element: a comment
// about an imported part belongs to that file (spec: comments). false when the element is in the document.
func (v *Viewer) anchorInFile(a comments.Anchor) (string, comments.Anchor, bool) {
	mount := model.MountOf(v.state.Current, false)
	if mount == nil {
		return "", a, false
	}
	md := v.mounts[mount]
	if md == nil {
		return "", a, false
	}
	prefix := PathOf(mount)
	if len(a.Element) < len(prefix) || !slices.Equal(a.Element[:len(prefix)], prefix) {
		return "", a, false
	}
	a.Element = slices.Clone(a.Element[len(prefix):])
	return realPath(v.libPath(md.file)), a, true
}

func (v *Viewer) findComment(id string) *comments.Comment {
	for i := range v.cm.list {
		if v.cm.list[i].ID == id {
			return &v.cm.list[i]
		}
	}
	return nil
}
