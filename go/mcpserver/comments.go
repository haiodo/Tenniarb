package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/haiodo/tenniarb/go/comments"
	"github.com/haiodo/tenniarb/go/link"
	"github.com/haiodo/tenniarb/go/servicewire"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// maxEvents is how many comment events the hub remembers for waiting sessions.
const maxEvents = 256

// Bounds of wait_comment, in seconds.
const (
	defaultWait = 60
	maxWait     = 600
)

// commentEvent is something the user did that an agent may want to answer.
type commentEvent struct {
	seq  int64
	key  string
	path string // absolute path of the document as the writer named it
	id   string
	kind string
}

// commentHub is the one writer of the comments: it stores them, tells the apps and wakes the
// sessions that wait for a comment.
type commentHub struct {
	store *comments.Store
	live  *registry
	now   func() time.Time

	mu      sync.Mutex
	seq     int64
	events  []commentEvent
	changed chan struct{} // closed and replaced when an event is added
}

func newCommentHub(store *comments.Store, live *registry) *commentHub {
	return &commentHub{store: store, live: live, now: time.Now, changed: make(chan struct{})}
}

// head is the sequence number of the last event: a session sees the events after it.
func (h *commentHub) head() int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.seq
}

func docKey(abs string) string { return linkRepos.DocKey(abs) }

// publish tells the apps that hold the document about d and, for the user's own doing, records an event.
func (h *commentHub) publish(abs string, d comments.Doc, id, kind, by string) {
	if by == comments.User && kind != "" {
		h.mu.Lock()
		h.seq++
		h.events = append(h.events, commentEvent{seq: h.seq, key: d.Key, path: abs, id: id, kind: kind})
		if len(h.events) > maxEvents {
			h.events = h.events[len(h.events)-maxEvents:]
		}
		close(h.changed)
		h.changed = make(chan struct{})
		h.mu.Unlock()
	}
	h.push(d)
}

// push sends d to every app that has the document open, whichever clone it is.
func (h *commentHub) push(d comments.Doc) {
	for abs, ld := range h.live.entries() {
		if ld.moved || docKey(abs) != d.Key {
			continue
		}
		up := servicewire.CommentsUpdate{V: servicewire.CommentsVersion, Path: ld.path, Rev: d.Rev, Comments: d.Comments}
		// One goroutine per message: a slow app must not hold a tool call; Rev orders what arrives.
		go ld.prov.peer.Notify(servicewire.MethodComments, up)
	}
}

// pushDoc sends the stored comments of abs to the app that has just opened it; nothing when there are none.
func (h *commentHub) pushDoc(abs string) {
	if d, err := h.store.Load(docKey(abs)); err == nil && d.Rev > 0 {
		h.push(d)
	}
}

// change applies f to the comments of abs. f returns the id of the comment it touched and the kind of
// event for the agents.
func (h *commentHub) change(abs, by string, f func(d *comments.Doc, now time.Time) (id, kind string, err error)) (comments.Doc, error) {
	var id, kind string
	d, err := h.store.Update(docKey(abs), func(d *comments.Doc) (err error) {
		id, kind, err = f(d, h.now())
		return err
	})
	if err != nil {
		return d, err
	}
	h.publish(abs, d, id, kind, by)
	return d, nil
}

func (h *commentHub) reply(abs, by, id, text string) (comments.Doc, error) {
	d, err := h.change(abs, by, func(d *comments.Doc, now time.Time) (string, string, error) {
		_, err := d.Reply(id, by, text, now)
		return id, "", err
	})
	return d, err
}

func (h *commentHub) setState(abs, by, id, state, text string) (comments.Doc, error) {
	d, err := h.change(abs, by, func(d *comments.Doc, now time.Time) (string, string, error) {
		_, err := d.SetState(id, state, by, text, now)
		return id, "", err
	})
	return d, err
}

// fromApp runs a change of the user of the app.
func (h *commentHub) fromApp(op servicewire.CommentsOp) (servicewire.CommentsUpdate, error) {
	if !filepath.IsAbs(op.Path) {
		return servicewire.CommentsUpdate{}, errors.New("path must be absolute")
	}
	abs := realPath(op.Path)
	var d comments.Doc
	var err error
	if op.Op == servicewire.CommentList {
		d, err = h.store.Load(docKey(abs))
	} else {
		d, err = h.change(abs, comments.User, op.Apply)
	}
	if err != nil {
		return servicewire.CommentsUpdate{}, err
	}
	return servicewire.CommentsUpdate{V: servicewire.CommentsVersion, Path: op.Path, Rev: d.Rev, Comments: d.Comments}, nil
}

// pending is the events after *cursor that match key ("" for any document) and moves *cursor past them.
func (h *commentHub) pending(cursor *commentCursor, key string) ([]commentEvent, <-chan struct{}) {
	cursor.mu.Lock()
	defer cursor.mu.Unlock()
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []commentEvent
	for _, e := range h.events {
		if e.seq > cursor.seq && (key == "" || e.key == key) {
			out = append(out, e)
		}
	}
	if len(out) > 0 {
		cursor.seq = out[len(out)-1].seq
	}
	return out, h.changed
}

// wait blocks until an event for key arrives after the cursor, the timeout passes or ctx ends.
func (h *commentHub) wait(ctx context.Context, cursor *commentCursor, key string, timeout time.Duration) []commentEvent {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		evs, changed := h.pending(cursor, key)
		if len(evs) > 0 {
			return evs
		}
		select {
		case <-changed:
		case <-timer.C:
			return nil
		case <-ctx.Done():
			return nil
		}
	}
}

// commentCursor is how far a session has been told of events; it starts at the head when the session opens.
type commentCursor struct {
	mu  sync.Mutex
	seq int64
}

type (
	commentsIn struct {
		File  string `json:"file" jsonschema:"path of a .tenn file relative to the workspace root"`
		State string `json:"state,omitempty" jsonschema:"open (default) or all"`
	}
	replyIn struct {
		File string `json:"file" jsonschema:"path of a .tenn file relative to the workspace root"`
		ID   string `json:"id" jsonschema:"id of the comment, from comments"`
		Text string `json:"text" jsonschema:"your answer"`
	}
	resolveIn struct {
		File string `json:"file" jsonschema:"path of a .tenn file relative to the workspace root"`
		ID   string `json:"id" jsonschema:"id of the comment, from comments"`
		Text string `json:"text,omitempty" jsonschema:"a closing reply, shown in the thread"`
	}
	waitIn struct {
		File     string `json:"file,omitempty" jsonschema:"only comments on this .tenn file; empty for any document"`
		TimeoutS int    `json:"timeout_s,omitempty" jsonschema:"seconds to wait, default 60, at most 600"`
	}
)

type anchorJSON struct {
	Element string          `json:"element,omitempty"`
	Node    string          `json:"node,omitempty"`
	Point   *comments.Point `json:"point,omitempty"`
}

type replyJSON struct {
	Author  string `json:"author"`
	Text    string `json:"text"`
	Created string `json:"created"`
}

type commentJSON struct {
	ID       string      `json:"id"`
	State    string      `json:"state"`
	Author   string      `json:"author"`
	Created  string      `json:"created"`
	Text     string      `json:"text"`
	Anchor   anchorJSON  `json:"anchor"`
	LinkText string      `json:"link_text,omitempty"`
	Replies  []replyJSON `json:"replies,omitempty"`
}

type eventJSON struct {
	File    string      `json:"file"`
	Event   string      `json:"event"`
	Comment commentJSON `json:"comment"`
}

// commentLink is the link text to the anchor of a comment: File.tenn#Element/node, which select takes.
func commentLink(abs string, a comments.Anchor) string {
	home, _ := os.UserHomeDir()
	l, _, err := linkRepos.Of(abs, home, a.Element, a.Node)
	if err != nil || l.Home {
		return ""
	}
	return l.Text()
}

func toJSON(abs string, c comments.Comment) commentJSON {
	out := commentJSON{ID: c.ID, State: c.State, Author: c.Author, Created: c.Created.Format(time.RFC3339), Text: c.Text,
		Anchor: anchorJSON{Element: strings.Join(c.Anchor.Element, "."), Node: c.Anchor.Node, Point: c.Anchor.Point}, LinkText: commentLink(abs, c.Anchor)}
	for _, r := range c.Replies {
		out.Replies = append(out.Replies, replyJSON{Author: r.Author, Text: r.Text, Created: r.Created.Format(time.RFC3339)})
	}
	return out
}

func marshal(v any) (string, error) {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false) // links have "&"
	enc.SetIndent("", " ")
	err := enc.Encode(v)
	return strings.TrimSuffix(b.String(), "\n"), err
}

// author is the name the replies of this session carry: the MCP client name, never the user's own.
func (s *Server) author() string {
	name := ""
	if s.agent != nil {
		name = s.host.presence.nameOf(s.agent)
	}
	if name == "" || name == comments.User {
		return "agent"
	}
	return name
}

// fileOf is how this session names abs: relative to its root, else absolute.
func (s *Server) fileOf(abs string) string {
	if within(s.root, abs) {
		r, _ := filepath.Rel(s.root, abs)
		return filepath.ToSlash(r)
	}
	return abs
}

// notePlace shows the agent at the anchor of c in the app.
func (s *Server) notePlace(action, file string, c *comments.Comment) {
	path := ""
	if c != nil {
		path = scenePath(link.Link{Element: c.Anchor.Element, Node: c.Anchor.Node})
	}
	s.noteNode(action, file, path)
}

func (s *Server) listComments(in commentsIn) (string, error) {
	abs, _, err := s.resolve(in.File)
	if err != nil {
		return "", err
	}
	if in.State != "" && in.State != comments.Open && in.State != "all" {
		return "", fmt.Errorf("state must be open or all, not %q", in.State)
	}
	d, err := s.host.comments.store.Load(docKey(abs))
	if err != nil {
		return "", err
	}
	s.noteNode("comments", in.File, "")
	out := []commentJSON{}
	for _, c := range d.Comments {
		if in.State == "all" || c.State == comments.Open {
			out = append(out, toJSON(abs, c))
		}
	}
	return marshal(out)
}

func (s *Server) answer(file, id string, op func(abs string) (comments.Doc, error), action string) (string, error) {
	abs, _, err := s.resolve(file)
	if err != nil {
		return "", err
	}
	// Before the change: the user sees where the agent looks while it works.
	if d, err := s.host.comments.store.Load(docKey(abs)); err == nil {
		s.notePlace(action, file, d.Find(id))
	}
	d, err := op(abs)
	if err != nil {
		return "", err
	}
	return marshal(toJSON(abs, *d.Find(id)))
}

func (s *Server) replyComment(in replyIn) (string, error) {
	return s.answer(in.File, in.ID, func(abs string) (comments.Doc, error) {
		return s.host.comments.reply(abs, s.author(), in.ID, in.Text)
	}, "reply")
}

func (s *Server) resolveComment(in resolveIn) (string, error) {
	return s.answer(in.File, in.ID, func(abs string) (comments.Doc, error) {
		return s.host.comments.setState(abs, s.author(), in.ID, comments.Resolved, in.Text)
	}, "resolve")
}

func (s *Server) waitComment(ctx context.Context, in waitIn) (string, error) {
	key := ""
	if in.File != "" {
		abs, _, err := s.resolve(in.File)
		if err != nil {
			return "", err
		}
		key = docKey(abs)
	}
	secs := in.TimeoutS
	switch {
	case secs <= 0:
		secs = defaultWait
	case secs > maxWait:
		secs = maxWait
	}
	out := []eventJSON{}
	for _, e := range s.host.comments.wait(ctx, &s.cursor, key, time.Duration(secs)*time.Second) {
		d, err := s.host.comments.store.Load(e.key)
		if err != nil {
			continue
		}
		if c := d.Find(e.id); c != nil {
			out = append(out, eventJSON{File: s.fileOf(e.path), Event: e.kind, Comment: toJSON(e.path, *c)})
		}
	}
	return marshal(out)
}

// addCommentTools registers the tools a user's comments reach an agent through.
func (s *Server) addCommentTools(srv *mcp.Server) {
	add(srv, "comments", "List the comments the user left on a .tenn file in Tenniarb: id, author, text, the anchor (element, node, and link_text that select takes), the replies. state open (default) or all. Call it after you finish a task, and answer each comment with reply or resolve.",
		s.listComments)
	add(srv, "reply", "Answer a comment of the user; the answer shows in the thread in Tenniarb at once.", s.replyComment)
	add(srv, "resolve", "Mark a comment as done, optionally with a closing reply; the user sees it resolved in Tenniarb.", s.resolveComment)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "wait_comment",
		Description: "Wait until the user leaves a new comment or reply (on file, else on any document) and return it as a list of events: file, event (comment, reply, reopen), the comment with its thread. Returns [] after timeout_s seconds (default 60, at most 600) with nothing new; call it again to stay on duty. Events before the session started or already returned are not repeated.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in waitIn) (*mcp.CallToolResult, any, error) {
		text, err := s.waitComment(ctx, in)
		if err != nil {
			return failure(err), nil, nil
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
	})
}
