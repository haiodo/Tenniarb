//go:build darwin || linux || windows

package app

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/view"
	"github.com/haiodo/gowt/swt"

	"github.com/haiodo/tenniarb/go/comments"
	"github.com/haiodo/tenniarb/go/link"
	"github.com/haiodo/tenniarb/go/render"
	"github.com/haiodo/tenniarb/go/servicewire"
)

var _ servicewire.CommentsReceiver = provider{}

// Comments is called by the service client on one of its goroutines.
func (p provider) Comments(up servicewire.CommentsUpdate) {
	p.sh.ui(func() {
		if v, err := p.sh.viewer(up.Path); err == nil {
			v.onComments(up)
		}
	})
}

// commentUI is the comments of the document in a window.
type commentUI struct {
	path     string // the document the list belongs to
	rev      int64
	list     []comments.Comment
	mode     bool // a click on the canvas places a comment
	resolved bool // resolved comments keep their pins

	pins []Pin
	node *gfx.Node // the HUD node of the pins
	key  pinKey

	pop    *g.Popover // the thread that is open
	thread string     // its comment
	draft  string     // the unsent reply
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
	v.cm.path, v.cm.rev, v.cm.list = path, 0, nil
	v.syncPins()
	if path == "" {
		return
	}
	op := servicewire.CommentsOp{Path: realPath(path), Op: servicewire.CommentList}
	go func() {
		up, err := v.sh.localComment(op)
		v.app.Async(func() {
			if err == nil && v.cm.path == path {
				v.onComments(up)
			}
		})
	}()
}

// commentOp changes the comments of the document off the UI thread; the answer comes back as an update.
func (v *Viewer) commentOp(op servicewire.CommentsOp) {
	path := v.state.Doc.Path
	if path == "" {
		return
	}
	op.Path = realPath(path)
	go func() {
		up, err := v.sh.sendComment(op)
		v.app.Async(func() {
			if err != nil {
				v.notice("Comment: " + err.Error())
				return
			}
			v.onComments(up)
		})
	}()
}

// onComments takes the comments of the document. Messages are handled concurrently, so one older than
// the list on screen is dropped.
func (v *Viewer) onComments(up servicewire.CommentsUpdate) {
	if up.Rev < v.cm.rev {
		return
	}
	v.cm.rev, v.cm.list = up.Rev, up.Comments
	v.syncPins()
	if id := v.cm.thread; v.cm.pop != nil && id != "" {
		if v.findComment(id) == nil {
			v.closePopover()
		} else {
			v.showThread(id)
		}
	}
}

func (v *Viewer) findComment(id string) *comments.Comment {
	for i := range v.cm.list {
		if v.cm.list[i].ID == id {
			return &v.cm.list[i]
		}
	}
	return nil
}

func (v *Viewer) toggleCommentMode() { v.setCommentMode(!v.cm.mode) }

func (v *Viewer) setCommentMode(on bool) {
	if on && (v.session == nil || v.view == nil) {
		return
	}
	if on && v.state.Doc.Path == "" {
		v.notice("Save the file to comment on it.")
		return
	}
	v.cm.mode = on
	if v.view != nil {
		cur := view.CursorArrow
		if on {
			cur = view.CursorCrosshair
		}
		v.view.SetCursor(cur)
	}
	v.syncPins()
}

// commentKey: C toggles the comment mode and Escape leaves it. It reports whether it took the key.
func (v *Viewer) commentKey(k *view.Key) bool {
	if v.cm.mode && int32(k.Code) == int32(swt.ESC) {
		v.setCommentMode(false)
		return true
	}
	if k.Mods.Cmd || k.Mods.Ctrl || k.Mods.Alt || v.userTyping() || KeyName(k.Code, k.Char) != "c" {
		return false
	}
	v.toggleCommentMode()
	return true
}

// commentPointer takes a left press on a pin (always) or, in the comment mode, anywhere on the canvas.
func (v *Viewer) commentPointer(p *view.Pointer) bool {
	if p.Button != 1 || p.Kind != view.PointerDown || v.session == nil {
		return false
	}
	if pin, ok := PinAtView(v.cm.pins, v.view.Compositor().Camera(), p.Pos); ok {
		v.showThread(pin.ID)
		return true
	}
	if !v.cm.mode {
		return false
	}
	v.compose(p.Scene, p.Pos)
	return true
}

// itemAt is the classic item under the gfx point at, the smallest one when they overlap.
func (v *Viewer) itemAt(at gfx.Point) (string, gfx.Rect) {
	name, best := "", gfx.Rect{}
	ds := v.session.Scene()
	for _, it := range ds.Items() {
		d := ds.Drawables[it]
		if d == nil || it.IsLink() || it.Name == "" {
			continue
		}
		r := v.smap.RectToGfx(d.Bounds())
		if at.X < r.Min.X || at.X > r.Max.X || at.Y < r.Min.Y || at.Y > r.Max.Y {
			continue
		}
		if name == "" || r.W()*r.H() < best.W()*best.H() {
			name, best = it.Name, r
		}
	}
	return name, best
}

// anchorAt is the anchor of a click at the gfx scene point at.
func (v *Viewer) anchorAt(at gfx.Point) comments.Anchor {
	element := PathOf(v.state.Current)
	if v.sceneMode() {
		off := v.sv.off
		toElement := func(p gfx.Point) comments.Point { return comments.Point{X: p.X - off.X, Y: p.Y - off.Y} }
		if hit := v.sv.hitScene(gfx.Point{X: at.X - off.X, Y: at.Y - off.Y}); hit != nil {
			return AnchorAt(element, v.sv.pathOf[hit], gfx.R(hit.X+off.X, hit.Y+off.Y, hit.W, hit.H), at, toElement)
		}
		return AnchorAt(element, "", gfx.Rect{}, at, toElement)
	}
	toElement := func(p gfx.Point) comments.Point {
		s := v.smap.ToScene(p)
		return comments.Point{X: s.X, Y: s.Y}
	}
	name, box := v.itemAt(at)
	return AnchorAt(element, name, box, at, toElement)
}

// placePins are the pins of the comments on the element on the canvas.
func (v *Viewer) placePins() []Pin {
	if v.state.Current == nil || len(v.cm.list) == 0 {
		return nil
	}
	element := PathOf(v.state.Current)
	switch {
	case v.sceneMode():
		off := v.sv.off
		box := func(node string) (gfx.Rect, bool) {
			pl := v.sv.byPath[node]
			if pl == nil {
				return gfx.Rect{}, false
			}
			return gfx.R(pl.X+off.X, pl.Y+off.Y, pl.W, pl.H), true
		}
		return PlacePins(v.cm.list, element, v.cm.resolved, box, func(p comments.Point) gfx.Point {
			return gfx.Point{X: off.X + p.X, Y: off.Y + p.Y}
		})
	case v.session != nil && v.rendered != nil:
		ds := v.session.Scene()
		box := func(node string) (gfx.Rect, bool) {
			for _, it := range ds.Items() {
				if d := ds.Drawables[it]; d != nil && !it.IsLink() && it.Name == node {
					return v.smap.RectToGfx(d.Bounds()), true
				}
			}
			return gfx.Rect{}, false
		}
		return PlacePins(v.cm.list, element, v.cm.resolved, box, func(p comments.Point) gfx.Point {
			return v.smap.ToGfx(renderPoint(p))
		})
	}
	return nil
}

// syncPins redraws the pins of the HUD after the camera, the view size, the comments or the mode
// changed. The HUD scene is touched only when the picture would differ.
func (v *Viewer) syncPins() {
	if v.view == nil {
		return
	}
	comp := v.view.Compositor()
	w, h, _ := comp.Viewport()
	cam := comp.Camera()
	pins := v.placePins()
	key := pinKey{cam, w, h, v.cm.mode}
	cm := &v.cm
	if cm.node != nil && key == cm.key && slices.Equal(pins, cm.pins) {
		return
	}
	cm.pins, cm.key = pins, key
	if cm.node == nil && len(pins) == 0 && !cm.mode {
		return
	}
	pic := pinsPicture(pins, cam, w, cm.mode)
	comp.HUD().Update(func(tx *gfx.Tx) {
		if cm.node == nil {
			cm.node = gfx.NewNode(2)
			tx.Add(nil, cm.node)
			tx.SetShape(cm.node, nil, gfx.HitNone) // clicks on a pin are found by PinAtView
		}
		tx.SetPicture(cm.node, pic)
	})
}

var (
	pinOpen     = gfx.RGB(255, 149, 0)
	pinResolved = gfx.RGB(142, 142, 147)
	pinFont     = gfx.Font{Size: 11, Weight: 700}
	hintFont    = gfx.Font{Size: 12}
)

// pinsPicture draws the markers in view points: a filled disc with the number; a detached pin is a ring.
func pinsPicture(pins []Pin, cam gfx.Camera, viewW float64, mode bool) *gfx.Picture {
	rec := gfx.NewRecorder()
	sh := gfx.Shadow{DX: 0, DY: 1, Blur: 3, Color: gfx.RGBA(0, 0, 0, 0.35)}
	for _, p := range pins {
		c := pinView(p, cam)
		col := pinOpen
		if p.Resolved {
			col = pinResolved
		}
		disc := gfx.EllipsePath(gfx.R(c.X-pinRadius, c.Y-pinRadius, 2*pinRadius, 2*pinRadius))
		text := gfx.RGB(255, 255, 255)
		if p.Detached {
			rec.FillShadowPath(disc, gfx.RGB(255, 255, 255), sh)
			rec.StrokePath(disc, gfx.StrokeStyle{Width: 2, MiterLimit: 10}, col)
			text = col
		} else {
			rec.FillShadowPath(disc, col, sh)
		}
		s := fmt.Sprint(p.N)
		m := gfx.DefaultMeasurer.MeasureText(s, pinFont)
		rec.Text(s, c.X-m.Width/2, c.Y+(m.Ascent-m.Descent)/2, pinFont, text)
	}
	if mode {
		s := "Comment mode: click to place a comment, Esc to leave"
		m := gfx.DefaultMeasurer.MeasureText(s, hintFont)
		r := gfx.R(viewW/2-m.Width/2-10, 8, m.Width+20, m.Ascent+m.Descent+10)
		rec.FillRRect(r, 8, gfx.RGBA(30, 30, 30, 0.82))
		rec.Text(s, r.Min.X+10, r.Min.Y+5+m.Ascent, hintFont, gfx.RGB(255, 255, 255))
	}
	return rec.Finish()
}

// compose opens the editor of a new comment at the click: at in the gfx scene, pos in the view.
func (v *Viewer) compose(at, pos gfx.Point) {
	anchor := v.anchorAt(at)
	v.closePopover()
	p := v.win.Popover(g.Gap(6), g.Padding(8))
	var field *g.TextView
	p.Add(g.Text(g.Focused(), g.Multiline(), g.Border(), g.Scrollbars(), g.Hint("Comment"), g.ID("comment-text"),
		g.Size(g.Scaled(300), g.Scaled(80)), g.Ref(&field)))
	post := func() {
		text := strings.TrimSpace(field.Text())
		if text == "" {
			return
		}
		p.Close()
		v.commentOp(servicewire.CommentsOp{Op: servicewire.CommentAdd, Text: text, Anchor: &anchor})
	}
	p.Add(g.HBox(g.Gap(6), g.Padding(0),
		g.Label("", g.GrowX()),
		g.Button("Cancel", g.ID("comment-cancel"), g.Init(func(b *g.ButtonView) { b.OnClick(p.Close) })),
		g.Button("Post", g.ID("comment-post"), g.Init(func(b *g.ButtonView) { b.OnClick(post) }))))
	v.openPopover(p)
	v.activeText = field.Unwrap()
	p.ShowAtPoint(v.view.Canvas(), int(pos.X), int(pos.Y), g.PopoverBelow)
}

// showThread opens the thread of comment id at its pin: the texts, a field for the reply, and the buttons.
func (v *Viewer) showThread(id string) {
	c := v.findComment(id)
	if c == nil {
		return
	}
	draft := ""
	if v.cm.thread == id {
		draft = v.cm.draft
	}
	v.closePopover()
	cm := &v.cm
	w := g.Scaled(320)
	p := v.win.Popover(g.Gap(6), g.Padding(8))
	now := time.Now()
	head := fmt.Sprintf("#%d  %s  %s", numberOf(cm.list, id), c.Author, commentWhen(c.Created, now))
	if c.State == comments.Resolved {
		head += "  (resolved)"
	}
	p.Add(g.Label(head, faint()))
	if place := anchorText(c.Anchor); place != "" {
		p.Add(g.Label(place, faint(), g.Wrap(), g.Size(w, 0)))
	}
	if i := slices.IndexFunc(cm.pins, func(p Pin) bool { return p.ID == id }); i >= 0 && cm.pins[i].Detached {
		p.Add(g.Label("The node is gone: the pin sits at the element origin.", faint(), g.Wrap(), g.Size(w, 0)))
	}
	p.Add(g.Label(c.Text, g.Wrap(), g.Size(w, 0)))
	for _, r := range c.Replies {
		p.Add(g.Label(fmt.Sprintf("%s  %s", r.Author, commentWhen(r.Created, now)), faint()))
		p.Add(g.Label(r.Text, g.Wrap(), g.Size(w, 0)))
	}
	var field *g.TextView
	p.Add(g.Text(g.Multiline(), g.Border(), g.Scrollbars(), g.Hint("Reply"), g.ID("comment-reply"), g.Size(w, g.Scaled(54)), g.Ref(&field),
		g.Init(func(t *g.TextView) {
			t.SetText(draft)
			t.OnChange(func(s string) { cm.draft = s })
		})))
	send := func(op string, text string) {
		p.Close()
		v.commentOp(servicewire.CommentsOp{Op: op, ID: id, Text: text})
	}
	buttons := []g.Option{g.Gap(6), g.Padding(0),
		g.Button("Reply", g.ID("comment-send"), g.Init(func(b *g.ButtonView) {
			b.OnClick(func() {
				if text := strings.TrimSpace(field.Text()); text != "" {
					cm.draft = ""
					v.commentOp(servicewire.CommentsOp{Op: servicewire.CommentReply, ID: id, Text: text})
					field.SetText("")
				}
			})
		}))}
	if c.State == comments.Resolved {
		buttons = append(buttons, g.Button("Reopen", g.ID("comment-reopen"), g.Init(func(b *g.ButtonView) {
			b.OnClick(func() { send(servicewire.CommentReopen, "") })
		})))
	} else {
		buttons = append(buttons, g.Button("Resolve", g.ID("comment-resolve"), g.Init(func(b *g.ButtonView) {
			b.OnClick(func() { cm.draft = ""; send(servicewire.CommentResolve, strings.TrimSpace(field.Text())) })
		})))
	}
	if c.Author == comments.User {
		buttons = append(buttons, g.Button("Delete", g.ID("comment-delete"), g.Init(func(b *g.ButtonView) {
			b.OnClick(func() { send(servicewire.CommentDelete, "") })
		})))
	}
	p.Add(g.HBox(buttons...))
	v.openPopover(p)
	cm.pop, cm.thread = p, id
	p.OnClose(func() {
		if cm.pop == p {
			cm.pop, cm.thread = nil, ""
		}
	})
	v.activeText = field.Unwrap()
	x, y := v.view.Canvas().Size()
	x, y = x/2, y/3
	if i := slices.IndexFunc(cm.pins, func(p Pin) bool { return p.ID == id }); i >= 0 {
		c := pinView(cm.pins[i], v.view.Compositor().Camera())
		x, y = int(c.X), int(c.Y)+pinRadius
	}
	p.ShowAtPoint(v.view.Canvas(), x, y, g.PopoverBelow)
}

// anchorText is the place of a comment for a thread: "Element.Sub / node".
func anchorText(a comments.Anchor) string {
	place := strings.Join(a.Element, ".")
	if a.Node != "" {
		place += " / " + a.Node
	}
	return place
}

// maxListed caps the rows of the comment list.
const maxListed = 30

// showCommentList is View > Comments: the comments of the document; a row goes to its place.
func (v *Viewer) showCommentList() {
	if v.session == nil {
		return
	}
	v.closePopover()
	p := v.win.Popover(g.Gap(4), g.Padding(6))
	p.Add(g.Button("Show resolved", g.Check(), g.ID("comments-resolved"), g.Checked(v.cm.resolved), g.Init(func(b *g.ButtonView) {
		b.OnClick(func() {
			v.cm.resolved = b.Checked()
			v.syncPins()
			p.Close()
			v.app.Async(v.showCommentList)
		})
	})))
	shown := 0
	for i, c := range v.cm.list {
		if c.State == comments.Resolved && !v.cm.resolved {
			continue
		}
		if shown++; shown > maxListed {
			p.Add(g.Label("...", faint()))
			break
		}
		id := c.ID
		p.Add(g.Button(commentLabel(i+1, c), g.ID("comment-row"), g.Init(func(b *g.ButtonView) {
			b.OnClick(func() {
				p.Close()
				v.app.Async(func() { v.jumpToComment(id) })
			})
		})))
	}
	if shown == 0 {
		p.Add(g.Label("No comments.", faint()))
	}
	v.openPopover(p)
	w, _ := v.view.Canvas().Size()
	p.ShowAtPoint(v.view.Canvas(), w/2, 0, g.PopoverBelow)
}

// jumpToComment shows the place of a comment and opens its thread.
func (v *Viewer) jumpToComment(id string) {
	c := v.findComment(id)
	if c == nil {
		return
	}
	if missing := v.gotoLink(link.Link{Element: c.Anchor.Element, Node: c.Anchor.Node}); missing != "" {
		v.notice(missing)
	}
	v.app.Async(func() {
		v.syncPins()
		v.showThread(id)
	})
}

func renderPoint(p comments.Point) render.Point { return render.Point{X: p.X, Y: p.Y} }
