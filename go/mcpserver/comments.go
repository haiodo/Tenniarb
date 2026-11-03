package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/haiodo/tenniarb/go/comments"
	"github.com/haiodo/tenniarb/go/link"
	"github.com/haiodo/tenniarb/go/scene"
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
	abs, rel, err := s.resolve(in.File)
	if err != nil {
		return "", err
	}
	// resolve lets a file that is not there yet through, for the tools that write one: a comment list of a
	// missing file would say "no comments" and hide the real answer, which is that the path is wrong.
	if _, err := os.Stat(abs); err != nil {
		return "", fmt.Errorf("path %q: no such file", in.File)
	}
	if in.State != "" && in.State != comments.Open && in.State != "all" {
		return "", fmt.Errorf("state must be open or all, not %q", in.State)
	}
	s.noteNode("comments", in.File, "")
	out := []commentJSON{}
	seen := map[string]bool{}
	add := func(c comments.Comment) {
		if seen[c.ID] || (in.State != "all" && c.State != comments.Open) {
			return
		}
		seen[c.ID] = true
		out = append(out, toJSON(abs, c))
	}
	d, err := s.host.comments.store.Load(docKey(abs))
	if err != nil {
		return "", err
	}
	for _, c := range d.Comments {
		add(c)
	}
	// A comment on a part a document shows from this file, made before the app kept such comments with
	// their file, is still stored with that document: find it under the path of the mount.
	for _, m := range s.mountRefs(rel) {
		md, err := s.host.comments.store.Load(docKey(m.abs(s.root)))
		if err != nil {
			continue
		}
		for _, c := range md.Comments {
			if c2, ok := m.move(c); ok {
				add(c2)
			}
		}
	}
	return marshal(out)
}

// mountRef is a document that shows a file as a mounted element: the document's path and the element
// path of the mount, as the app writes element paths.
type mountRef struct {
	file   string   // the document, relative to the workspace root
	prefix []string // elements from the document root down to the mount
}

// abs is the absolute path of the document that mounts the file.
func (m mountRef) abs(root string) string { return filepath.Join(root, filepath.FromSlash(m.file)) }

// move is c with its element path made relative to the mounted file; false when the comment is not about
// an element under the mount.
func (m mountRef) move(c comments.Comment) (comments.Comment, bool) {
	if len(c.Anchor.Element) <= len(m.prefix) || !slices.Equal(c.Anchor.Element[:len(m.prefix)], m.prefix) {
		return c, false
	}
	c.Anchor.Element = slices.Clone(c.Anchor.Element[len(m.prefix):])
	return c, true
}

// mountRefs lists the documents that show rel as a mounted element, and where: the document of its
// entry and of the files of the workspace, or, when the workspace is too big to list, the documents the
// app has open.
func (s *Server) mountRefs(rel string) []mountRef {
	var docs []string
	seen := map[string]bool{}
	add := func(f string) {
		if f != "" && f != rel && !seen[f] {
			seen[f] = true
			docs = append(docs, f)
		}
	}
	if _, tree, err := s.diagnose(rel); err == nil && tree != nil {
		add(tree.Entry())
	}
	if files, err := s.files(); err == nil {
		for _, f := range files {
			add(f)
		}
	} else {
		for abs := range s.host.live.entries() {
			if within(s.root, abs) {
				add(s.fileOf(abs))
			}
		}
	}
	var out []mountRef
	for _, f := range docs {
		_, tree, err := s.diagnose(f)
		if err != nil || tree == nil {
			continue
		}
		if root := tree.File(f); root != nil {
			s.eachMount(root, nil, func(prefix []string, file string) {
				if file == rel {
					out = append(out, mountRef{file: f, prefix: prefix})
				}
			})
		}
	}
	return out
}

// eachMount calls visit for every element below parent that shows a file, with the element path of the
// mount and the file: the names as the app writes them, "#n" for a repeat among siblings.
func (s *Server) eachMount(parent *scene.Node, path []string, visit func(prefix []string, file string)) {
	seen := map[string]int{}
	for _, c := range parent.Nodes() {
		if c.Kind == scene.KUse {
			continue
		}
		if c.Kind != scene.KElement {
			s.eachMount(c, path, visit)
			continue
		}
		seg := c.Name
		if n := seen[c.Name]; n > 0 {
			seg += "#" + strconv.Itoa(n)
		}
		seen[c.Name]++
		p := append(append([]string{}, path...), seg)
		if c.Mount != nil {
			visit(p, c.Mount.Pos.File)
		}
		s.eachMount(c, p, visit)
	}
}

func (s *Server) answer(file, id string, op func(abs string) (comments.Doc, error), action string) (string, error) {
	abs, rel, err := s.resolve(file)
	if err != nil {
		return "", err
	}
	// A comment of a mounted part may still be stored with the document that shows it.
	owner := abs
	if d, err := s.host.comments.store.Load(docKey(abs)); err != nil || d.Find(id) == nil {
		for _, m := range s.mountRefs(rel) {
			if d, err := s.host.comments.store.Load(docKey(m.abs(s.root))); err == nil && d.Find(id) != nil {
				owner = m.abs(s.root)
				break
			}
		}
	}
	// Before the change: the user sees where the agent looks while it works.
	if d, err := s.host.comments.store.Load(docKey(owner)); err == nil {
		s.notePlace(action, file, d.Find(id))
	}
	d, err := op(owner)
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
	add(srv, "comments", "List the comments the user left on a .tenn file in Tenniarb: id, author, text, the anchor (element, node, and link_text that select takes), the replies. state open (default) or all. A comment on a part a document shows from this file is listed here too, with the path it has in this file. Call it after you finish a task, and answer each comment with reply or resolve.",
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
