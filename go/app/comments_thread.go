//go:build darwin || linux || windows

package app

import (
	"fmt"
	"github.com/haiodo/tenniarb/go/servicewire"
	"slices"
	"strings"
	"time"

	g "github.com/haiodo/gowt"

	"github.com/haiodo/tenniarb/go/comments"
	"github.com/haiodo/tenniarb/go/geometry"
	"github.com/haiodo/tenniarb/go/link"
)

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

func renderPoint(p comments.Point) geometry.Point { return geometry.Point{X: p.X, Y: p.Y} }
