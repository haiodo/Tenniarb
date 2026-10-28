package mcpserver

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/haiodo/tenniarb/go/scene"
	"github.com/haiodo/tenniarb/go/servicewire"
)

type sink struct {
	mu   sync.Mutex
	msgs []servicewire.Presence
}

func (s *sink) send(_ string, p servicewire.Presence) {
	s.mu.Lock()
	s.msgs = append(s.msgs, p)
	s.mu.Unlock()
}

func (s *sink) take() []servicewire.Presence {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.msgs
	s.msgs = nil
	return out
}

func testHub(now *time.Time) (*presenceHub, *sink) {
	sk := &sink{}
	h := newPresenceHub(sk.send)
	h.ttl = 10 * time.Minute
	h.now = func() time.Time { return *now }
	return h, sk
}

func TestPresenceHubTracksSessionsPerPath(t *testing.T) {
	now := time.Unix(1000, 0)
	h, sk := testHub(&now)
	a, b := h.open(), h.open()
	h.setName(a, "claude-code")

	h.note(a, "/x.tenn", "read", location{Node: "A.x", Element: "A"})
	h.note(b, "/y.tenn", "edit", location{})
	h.flush()
	got := map[string]servicewire.Presence{}
	for _, p := range sk.take() {
		got[p.Action] = p
	}
	if p := got["read"]; p.Agents != 1 || p.Name != "claude-code" || p.Node != "A.x" || p.Element != "A" || p.V != servicewire.PresenceVersion {
		t.Errorf("read presence: %+v", p)
	}
	if p := got["edit"]; p.Agents != 1 || p.Name != "" {
		t.Errorf("edit presence: %+v", p)
	}

	// Two sessions on one path: the count is 2, the details are the later action's.
	now = now.Add(time.Second)
	h.note(b, "/x.tenn", "select", location{Node: "A.y"})
	h.flush()
	ps := sk.take()
	if len(ps) != 1 || ps[0].Agents != 2 || ps[0].Action != "select" || ps[0].Node != "A.y" || ps[0].Name != "" {
		t.Errorf("two sessions: %+v", ps)
	}
	if ps[0].Seq <= got["read"].Seq || ps[0].At <= got["read"].At {
		t.Errorf("seq and at must grow: %+v after %+v", ps[0], got["read"])
	}
}

func TestPresenceHubIdleAndClose(t *testing.T) {
	now := time.Unix(1000, 0)
	h, sk := testHub(&now)
	a, b := h.open(), h.open()
	h.note(a, "/x.tenn", "read", location{})
	now = now.Add(9 * time.Minute)
	h.note(b, "/x.tenn", "read", location{})
	h.flush()
	if ps := sk.take(); len(ps) != 1 || ps[0].Agents != 2 {
		t.Fatalf("both active: %+v", ps)
	}

	now = now.Add(2 * time.Minute) // a is idle for 11 minutes, b for 2
	h.sweep()
	h.flush()
	if ps := sk.take(); len(ps) != 1 || ps[0].Agents != 1 {
		t.Fatalf("a idle must drop out: %+v", ps)
	}
	h.sweep()
	h.flush()
	if ps := sk.take(); len(ps) != 0 {
		t.Errorf("nothing changed, nothing sent: %+v", ps)
	}

	h.close(b) // a disconnect drops out at once, without waiting for the TTL
	h.flush()
	if ps := sk.take(); len(ps) != 1 || ps[0].Agents != 0 || ps[0].Action != "" {
		t.Fatalf("closed session: %+v", ps)
	}
	h.close(a)
	h.flush()
	if ps := sk.take(); len(ps) != 0 {
		t.Errorf("an already cleared document is not sent again: %+v", ps)
	}
}

func TestPresenceHubAllIdleClears(t *testing.T) {
	now := time.Unix(1000, 0)
	h, sk := testHub(&now)
	h.note(h.open(), "/x.tenn", "edit", location{})
	h.flush()
	sk.take()
	now = now.Add(11 * time.Minute)
	h.sweep()
	h.flush()
	if ps := sk.take(); len(ps) != 1 || ps[0].Agents != 0 {
		t.Errorf("idle session must clear the document: %+v", ps)
	}
}

func TestPresenceHubMarkTracked(t *testing.T) {
	now := time.Unix(1000, 0)
	h, sk := testHub(&now)
	h.note(h.open(), "/x.tenn", "read", location{})
	h.flush()
	sk.take()
	h.markTracked([]string{"/x.tenn", "/other.tenn"})
	h.flush()
	if ps := sk.take(); len(ps) != 1 || ps[0].Agents != 1 {
		t.Errorf("an app that registers the document must get its presence: %+v", ps)
	}
}

const presenceDoc = "element \"A\" {\n    item \"x\"\n    element \"Inner\" {\n        item \"deep\"\n    }\n}\n"

func TestPathAtAndChangedRange(t *testing.T) {
	tree, err := scene.LoadOverlay(t.TempDir(), "p.tenn", map[string]string{"p.tenn": presenceDoc})
	if err != nil {
		t.Fatal(err)
	}
	f := tree.File("p.tenn")
	off := func(s string) int {
		for i := range presenceDoc {
			if presenceDoc[i:i+len(s)] == s {
				return i
			}
		}
		t.Fatalf("no %q", s)
		return 0
	}
	for _, c := range []struct{ at, node, elem string }{
		{`item "x"`, "A.x", "A"},
		{`item "deep"`, "A.Inner.deep", "A.Inner"},
		{`element "Inner"`, "A.Inner", "A.Inner"},
		{`element "A"`, "A", "A"},
	} {
		if n, e := pathAt(f, off(c.at)); n != c.node || e != c.elem {
			t.Errorf("pathAt %q = %q, %q; want %q, %q", c.at, n, e, c.node, c.elem)
		}
	}

	for _, c := range []struct {
		old, new string
		a, b     int
	}{
		{"abcdef", "abXdef", 2, 3},
		{"abc", "abcd", 3, 4},
		{"aXa", "aXXa", 2, 3},
		{"abc", "abc", 3, 3},
		{"abcd", "ad", 1, 1},
	} {
		if a, b := changedRange(c.old, c.new); a != c.a || b != c.b {
			t.Errorf("changedRange(%q, %q) = %d, %d; want %d, %d", c.old, c.new, a, b, c.a, c.b)
		}
	}
}

// waitPresence polls the fake app for a presence that satisfies ok.
func waitPresence(t *testing.T, get func() []servicewire.Presence, ok func(servicewire.Presence) bool) servicewire.Presence {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		ps := get()
		for i := len(ps) - 1; i >= 0; i-- {
			if ok(ps[i]) {
				return ps[i]
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no matching presence in %+v", get())
	return servicewire.Presence{}
}

func TestServiceSendsPresence(t *testing.T) {
	dir := serviceDir(t)
	startService(t, dir, "v1", nil, 0)
	root := tempRoot(t, map[string]string{"a.tenn": liveDisk, "b.tenn": liveDisk})
	fake, _ := connectFake(t, dir)
	abs := filepath.Join(root, "a.tenn")
	if err := fake.Open(abs, liveDisk); err != nil {
		t.Fatal(err)
	}
	cs := session(t, dir, root, "v1", nil)

	mustOK(t, cs, "read", map[string]any{"file": "b.tenn"}) // not open in the app: no presence
	mustOK(t, cs, "read", map[string]any{"file": "a.tenn", "path": "A.x"})
	// The client name arrives with the initialized notification, which may follow the first tool call.
	p := waitPresence(t, fake.Presences, func(p servicewire.Presence) bool { return p.Action == "read" && p.Name == "test" })
	if p.Path != abs || p.Agents != 1 || p.Node != "A.x" || p.Element != "A" || p.Line != 2 {
		t.Errorf("read: %+v", p)
	}

	mustOK(t, cs, "edit", withFile(edits(`item "x"`, `item "y"`), "a.tenn"))
	p = waitPresence(t, fake.Presences, func(p servicewire.Presence) bool { return p.Action == "edit" })
	if p.Node != "A.y" || p.Element != "A" || p.Line != 2 {
		t.Errorf("edit location: %+v", p)
	}

	mustOK(t, cs, "select", map[string]any{"file": "a.tenn", "nodes": []string{"A.y"}})
	waitPresence(t, fake.Presences, func(p servicewire.Presence) bool { return p.Action == "select" && p.Node == "A.y" })

	cs.Close()
	waitPresence(t, fake.Presences, func(p servicewire.Presence) bool { return p.Agents == 0 })
	for _, p := range fake.Presences() {
		if p.Path != abs {
			t.Errorf("presence for %s, only a.tenn is open", p.Path)
		}
	}
}
