package mcpserver

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/haiodo/tenniarb/go/scene"
	"github.com/haiodo/tenniarb/go/servicewire"
)

// location is where in a document an action happened.
type location struct {
	Node, Element string
	Line, EndLine int
}

// touch is the last action of one session on one document.
type touch struct {
	action string
	loc    location
	at     int64 // Unix ms, unique and growing across the whole hub
}

// agentSession is one MCP session; presence knows it by the open documents it touched.
type agentSession struct {
	name    string
	touches map[string]touch // real path -> last action
}

// presenceHub tracks which sessions work on which documents and tells the app through send. The loop
// sends the current state of each marked document, so a slow app never blocks a tool call.
type presenceHub struct {
	ttl  time.Duration
	now  func() time.Time
	send func(abs string, p servicewire.Presence)

	mu       sync.Mutex
	sessions map[*agentSession]bool
	dirty    map[string]bool
	sent     map[string]int // documents last sent with agents; the count sent
	last     int64
	wake     chan struct{}
}

func newPresenceHub(send func(abs string, p servicewire.Presence)) *presenceHub {
	return &presenceHub{
		ttl: servicewire.PresenceTTL, now: time.Now, send: send,
		sessions: map[*agentSession]bool{}, dirty: map[string]bool{}, sent: map[string]int{},
		wake: make(chan struct{}, 1),
	}
}

// stamp is the clock in ms, made strictly growing so that two actions never share one. Hub lock held.
func (h *presenceHub) stamp() int64 {
	t := h.now().UnixMilli()
	if t <= h.last {
		t = h.last + 1
	}
	h.last = t
	return t
}

func (h *presenceHub) open() *agentSession {
	s := &agentSession{touches: map[string]touch{}}
	h.mu.Lock()
	h.sessions[s] = true
	h.mu.Unlock()
	return s
}

// close forgets s; the documents it touched are reported again.
func (h *presenceHub) close(s *agentSession) {
	h.mu.Lock()
	delete(h.sessions, s)
	for abs := range s.touches {
		h.dirty[abs] = true
	}
	h.mu.Unlock()
	h.poke()
}

func (h *presenceHub) setName(s *agentSession, name string) {
	h.mu.Lock()
	s.name = name
	for abs := range s.touches {
		h.dirty[abs] = true
	}
	h.mu.Unlock()
	h.poke()
}

// nameOf is the client name of s, empty when it did not give one.
func (h *presenceHub) nameOf(s *agentSession) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return s.name
}

func (h *presenceHub) note(s *agentSession, abs, action string, loc location) {
	h.mu.Lock()
	if !h.sessions[s] {
		h.mu.Unlock()
		return
	}
	s.touches[abs] = touch{action: action, loc: loc, at: h.stamp()}
	h.dirty[abs] = true
	h.mu.Unlock()
	h.poke()
}

func (h *presenceHub) poke() {
	select {
	case h.wake <- struct{}{}:
	default:
	}
}

// markTracked reports again the documents a session touched; used when an app registers them.
func (h *presenceHub) markTracked(paths []string) {
	h.mu.Lock()
	for _, abs := range paths {
		for s := range h.sessions {
			if _, ok := s.touches[abs]; ok {
				h.dirty[abs] = true
			}
		}
	}
	h.mu.Unlock()
	h.poke()
}

// snapshot is the presence of abs. Hub lock held.
func (h *presenceHub) snapshot(abs string) servicewire.Presence {
	p := servicewire.Presence{V: servicewire.PresenceVersion}
	edge := h.now().UnixMilli() - h.ttl.Milliseconds()
	var best touch
	for s := range h.sessions {
		t, ok := s.touches[abs]
		if !ok || t.at < edge {
			continue
		}
		p.Agents++
		if t.at > best.at {
			best, p.Name = t, s.name
		}
	}
	if p.Agents > 0 {
		p.Action, p.At = best.action, best.at
		p.Element, p.Node, p.Line, p.EndLine = best.loc.Element, best.loc.Node, best.loc.Line, best.loc.EndLine
	}
	return p
}

// sweep drops the touches idle for longer than the TTL and marks the documents whose picture changed.
func (h *presenceHub) sweep() {
	h.mu.Lock()
	edge := h.now().UnixMilli() - h.ttl.Milliseconds()
	for s := range h.sessions {
		for abs, t := range s.touches {
			if t.at < edge {
				delete(s.touches, abs)
			}
		}
	}
	for abs, n := range h.sent {
		if h.snapshot(abs).Agents != n {
			h.dirty[abs] = true
		}
	}
	h.mu.Unlock()
	h.poke()
}

// flush sends the state of every marked document.
func (h *presenceHub) flush() {
	type out struct {
		abs string
		p   servicewire.Presence
	}
	h.mu.Lock()
	var msgs []out
	for abs := range h.dirty {
		p := h.snapshot(abs)
		p.Seq = h.stamp()
		_, shown := h.sent[abs]
		if p.Agents == 0 && !shown {
			continue // the app has nothing to clear
		}
		if p.Agents > 0 {
			h.sent[abs] = p.Agents
		} else {
			delete(h.sent, abs)
		}
		msgs = append(msgs, out{abs, p})
	}
	h.dirty = map[string]bool{}
	h.mu.Unlock()
	for _, m := range msgs {
		h.send(m.abs, m.p)
	}
}

// run sends changes as they come and sweeps every sweepEvery, until ctx ends.
func (h *presenceHub) run(ctx context.Context, sweepEvery time.Duration) {
	tick := time.NewTicker(sweepEvery)
	defer tick.Stop()
	for {
		select {
		case <-h.wake:
			h.flush()
		case <-tick.C:
			h.sweep()
		case <-ctx.Done():
			return
		}
	}
}

// locate finds the node a dotted path names in file rel; the path is kept as given when it cannot
// be resolved, so the app can still try it.
func (s *Server) locate(rel, path string) location {
	loc := location{Node: path}
	_, tree, err := s.diagnose(rel)
	if err != nil || tree == nil {
		return loc
	}
	n, err := nodeAt(tree, rel, path)
	if err != nil || n.Pos.File != rel {
		return loc
	}
	if src, ok := tree.Source(rel); ok && n.Range.End <= len(src) {
		loc.Line = 1 + strings.Count(src[:n.Range.Start], "\n")
		loc.EndLine = loc.Line + strings.Count(src[n.Range.Start:n.Range.End], "\n")
	}
	owner := n
	for owner.Kind != scene.KElement && owner.Kind != scene.KFile {
		owner = owner.Parent
	}
	var segs []string
	for ; owner.Kind == scene.KElement; owner = owner.Parent {
		segs = append([]string{owner.Key}, segs...)
	}
	loc.Element = strings.Join(segs, ".")
	return loc
}

// locateEdit is the location of the first change between old and text, the new content of rel.
func locateEdit(tree *scene.Tree, rel, old, text string) location {
	a, b := changedRange(old, text)
	for a < b && strings.ContainsRune(" \t\r\n", rune(text[a])) {
		a++ // the node starts after the blank space the edit added in front of it
	}
	loc := location{Line: 1 + strings.Count(text[:a], "\n")}
	loc.EndLine = loc.Line + strings.Count(text[a:b], "\n")
	if f := fileRoot(tree, rel); f != nil {
		loc.Node, loc.Element = pathAt(f, a)
	}
	return loc
}

// changedRange is [a, b) of text that differs from old: the common start and end are cut off.
func changedRange(old, text string) (a, b int) {
	for a < len(old) && a < len(text) && old[a] == text[a] {
		a++
	}
	b = len(text)
	for i := len(old); b > a && i > a && old[i-1] == text[b-1]; i-- {
		b--
	}
	return a, b
}

// pathAt is the dotted path of the innermost node written in file root that holds the offset, and of
// the element around it.
func pathAt(root *scene.Node, off int) (node, element string) {
	var segs []string
	elems := 0
	for cur := root; ; {
		var next *scene.Node
		for _, c := range cur.Nodes() {
			if c.Key != "" && c.Range.Start <= off && off < c.Range.End {
				next = c
				break
			}
		}
		if next == nil {
			break
		}
		segs = append(segs, next.Key)
		if next.Kind == scene.KElement {
			elems = len(segs)
		}
		cur = next
	}
	return strings.Join(segs, "."), strings.Join(segs[:elems], ".")
}

// noteNode records that this session looked at path of file, before the tool runs. Nothing happens
// for a file the app has not open: presence is shown there.
func (s *Server) noteNode(action, file, path string) {
	abs, rel, err := s.resolve(file)
	if err != nil || s.agent == nil || s.host.live.lookup(abs) == nil {
		return
	}
	loc := location{}
	if path != "" {
		loc = s.locate(rel, path)
	}
	s.host.presence.note(s.agent, abs, action, loc)
}

// noteEdit records an edit of the open document abs, once the app applied it.
func (s *Server) noteEdit(abs, rel string, tree *scene.Tree, old, text string) {
	if s.agent != nil {
		s.host.presence.note(s.agent, abs, "edit", locateEdit(tree, rel, old, text))
	}
}
