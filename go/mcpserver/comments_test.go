package mcpserver

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/haiodo/tenniarb/go/comments"
	"github.com/haiodo/tenniarb/go/servicewire"
	"github.com/haiodo/tenniarb/go/servicewire/wiretest"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const commentDoc = "element \"A\" {\n    item \"x\"\n}\n"

// commentEnv is a service with an app that has a.tenn open and an agent session in the same folder.
type commentEnv struct {
	dir, root, abs string
	fake           *wiretest.Fake
	agent          *mcp.ClientSession
	config         string
}

func newCommentEnv(t *testing.T) *commentEnv {
	t.Helper()
	e := &commentEnv{dir: serviceDir(t)}
	e.config = startService(t, e.dir, "v1", nil, 0).sv.Host.comments.store.Dir()
	e.root = tempRoot(t, map[string]string{"a.tenn": commentDoc})
	e.agent = session(t, e.dir, e.root, "v1", nil)
	// The first tool call names the client; the session cursor starts when the session opens.
	mustOK(t, e.agent, "get_state", nil)
	e.fake, _ = connectFake(t, e.dir)
	e.abs = filepath.Join(e.root, "a.tenn")
	if err := e.fake.Open(e.abs, commentDoc); err != nil {
		t.Fatal(err)
	}
	return e
}

// post is a comment of the user made in the app.
func (e *commentEnv) post(t *testing.T, text string) servicewire.CommentsUpdate {
	t.Helper()
	up, err := e.fake.Client.Comments(servicewire.CommentsOp{Path: e.abs, Op: servicewire.CommentAdd, Text: text,
		Anchor: &comments.Anchor{Element: []string{"A"}, Node: "x", Point: &comments.Point{X: 1, Y: 2}}})
	if err != nil {
		t.Fatal(err)
	}
	return up
}

func parseComments(t *testing.T, text string) []commentJSON {
	t.Helper()
	var out []commentJSON
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("%v in %s", err, text)
	}
	return out
}

func parseEvents(t *testing.T, text string) []eventJSON {
	t.Helper()
	var out []eventJSON
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("%v in %s", err, text)
	}
	return out
}

// lastUpdate waits for a notification of the app with at least rev.
func (e *commentEnv) lastUpdate(t *testing.T, rev int64) servicewire.CommentsUpdate {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var best servicewire.CommentsUpdate
		for _, u := range e.fake.CommentUpdates() {
			if u.Rev > best.Rev {
				best = u
			}
		}
		if best.Rev >= rev {
			return best
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no comments notification with rev %d, got %+v", rev, e.fake.CommentUpdates())
	return servicewire.CommentsUpdate{}
}

func TestCommentsThroughService(t *testing.T) {
	e := newCommentEnv(t)
	up := e.post(t, "title is too tight")
	if up.Rev != 1 || len(up.Comments) != 1 || up.Comments[0].Author != comments.User || up.Comments[0].State != comments.Open {
		t.Fatalf("add: %+v", up)
	}
	id := up.Comments[0].ID
	// The app hears of its own change as well, with the path it sent.
	if n := e.lastUpdate(t, 1); n.Path != e.abs || len(n.Comments) != 1 {
		t.Fatalf("notification: %+v", n)
	}

	list := parseComments(t, mustOK(t, e.agent, "comments", map[string]any{"file": "a.tenn"}))
	if len(list) != 1 || list[0].ID != id || list[0].Text != "title is too tight" || list[0].Anchor.Element != "A" || list[0].Anchor.Node != "x" || list[0].Anchor.Point.Y != 2 {
		t.Fatalf("comments: %+v", list)
	}

	mustOK(t, e.agent, "reply", map[string]any{"file": "a.tenn", "id": id, "text": "padding is 8 now"})
	n := e.lastUpdate(t, 2)
	if len(n.Comments[0].Replies) != 1 || n.Comments[0].Replies[0].Author != "test" || n.Comments[0].Replies[0].Text != "padding is 8 now" {
		t.Fatalf("reply did not reach the app: %+v", n)
	}
	mustOK(t, e.agent, "resolve", map[string]any{"file": "a.tenn", "id": id, "text": "done"})
	n = e.lastUpdate(t, 3)
	if n.Comments[0].State != comments.Resolved || len(n.Comments[0].Replies) != 2 {
		t.Fatalf("resolve did not reach the app: %+v", n)
	}
	if got := parseComments(t, mustOK(t, e.agent, "comments", map[string]any{"file": "a.tenn"})); len(got) != 0 {
		t.Fatalf("a resolved comment is listed as open: %+v", got)
	}
	if got := parseComments(t, mustOK(t, e.agent, "comments", map[string]any{"file": "a.tenn", "state": "all"})); len(got) != 1 {
		t.Fatalf("state all: %+v", got)
	}

	// The user reopens and deletes; a delete of a comment of another author is refused.
	if _, err := e.fake.Client.Comments(servicewire.CommentsOp{Path: e.abs, Op: servicewire.CommentReopen, ID: id}); err != nil {
		t.Fatal(err)
	}
	if up, _ := e.fake.Client.Comments(servicewire.CommentsOp{Path: e.abs, Op: servicewire.CommentList}); up.Comments[0].State != comments.Open {
		t.Fatalf("reopen: %+v", up)
	}
	if _, err := e.fake.Client.Comments(servicewire.CommentsOp{Path: e.abs, Op: servicewire.CommentDelete, ID: id}); err != nil {
		t.Fatal(err)
	}
	if up, _ := e.fake.Client.Comments(servicewire.CommentsOp{Path: e.abs, Op: servicewire.CommentList}); len(up.Comments) != 0 {
		t.Fatalf("delete: %+v", up)
	}
	if _, err := e.fake.Client.Comments(servicewire.CommentsOp{Path: e.abs, Op: "bogus"}); err == nil {
		t.Fatal("an unknown operation was accepted")
	}
	if _, err := e.fake.Client.Comments(servicewire.CommentsOp{Path: "a.tenn", Op: servicewire.CommentList}); err == nil {
		t.Fatal("a relative path was accepted")
	}
	mustFail(t, e.agent, "reply", map[string]any{"file": "a.tenn", "id": "nope", "text": "x"}, "no such comment")
}

func TestCommentsStayOutOfTheWorkspace(t *testing.T) {
	e := newCommentEnv(t)
	e.post(t, "x")
	ents, _ := os.ReadDir(e.config)
	if len(ents) != 1 {
		t.Fatalf("config folder: %v", ents)
	}
	if left, _ := os.ReadDir(e.root); len(left) != 1 {
		t.Fatalf("the workspace changed: %v", left)
	}
}

func TestWaitCommentTimeout(t *testing.T) {
	e := newCommentEnv(t)
	start := time.Now()
	got := mustOK(t, e.agent, "wait_comment", map[string]any{"timeout_s": 1})
	if d := time.Since(start); d < 900*time.Millisecond || d > 4*time.Second {
		t.Fatalf("waited %v", d)
	}
	if evs := parseEvents(t, got); len(evs) != 0 {
		t.Fatalf("events on timeout: %s", got)
	}
}

func TestWaitCommentWakesUp(t *testing.T) {
	e := newCommentEnv(t)
	type res struct {
		text string
		took time.Duration
	}
	done := make(chan res, 1)
	start := time.Now()
	go func() {
		_, text := call(t, e.agent, "wait_comment", map[string]any{"timeout_s": 30})
		done <- res{text, time.Since(start)}
	}()
	time.Sleep(200 * time.Millisecond)
	up := e.post(t, "move it")
	select {
	case r := <-done:
		evs := parseEvents(t, r.text)
		if len(evs) != 1 || evs[0].Event != "comment" || evs[0].File != "a.tenn" || evs[0].Comment.ID != up.Comments[0].ID || r.took > 10*time.Second {
			t.Fatalf("after %v: %s", r.took, r.text)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("wait_comment did not wake up")
	}

	// The agent's own reply wakes nobody; the user's reply does. A comment made between two waits is
	// not lost, and one already told is not told again.
	id := up.Comments[0].ID
	mustOK(t, e.agent, "reply", map[string]any{"file": "a.tenn", "id": id, "text": "ok"})
	if evs := parseEvents(t, mustOK(t, e.agent, "wait_comment", map[string]any{"timeout_s": 1})); len(evs) != 0 {
		t.Fatalf("own reply or repeated event: %+v", evs)
	}
	if _, err := e.fake.Client.Comments(servicewire.CommentsOp{Path: e.abs, Op: servicewire.CommentReply, ID: id, Text: "thanks"}); err != nil {
		t.Fatal(err)
	}
	e.post(t, "and this")
	evs := parseEvents(t, mustOK(t, e.agent, "wait_comment", map[string]any{"file": "a.tenn", "timeout_s": 5}))
	if len(evs) != 2 || evs[0].Event != "reply" || evs[1].Event != "comment" || len(evs[0].Comment.Replies) != 2 {
		t.Fatalf("events: %+v", evs)
	}
}

func TestWaitCommentSeesOnlyNewEvents(t *testing.T) {
	e := newCommentEnv(t)
	e.post(t, "before the second session")
	other := session(t, e.dir, e.root, "v1", nil)
	if evs := parseEvents(t, mustOK(t, other, "wait_comment", map[string]any{"timeout_s": 1})); len(evs) != 0 {
		t.Fatalf("a session was told of a comment from before it started: %+v", evs)
	}
	// The first session started earlier and is told.
	if evs := parseEvents(t, mustOK(t, e.agent, "wait_comment", map[string]any{"timeout_s": 1})); len(evs) != 1 {
		t.Fatalf("events: %+v", evs)
	}
}

func TestCommentsFollowTheDocumentAcrossClones(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	e := newCommentEnv(t)
	base := t.TempDir()
	clone := func(name string) string {
		d := filepath.Join(base, name)
		os.MkdirAll(filepath.Join(d, "design"), 0o755)
		os.WriteFile(filepath.Join(d, "design", "P.tenn"), []byte(commentDoc), 0o644)
		for _, args := range [][]string{{"init", "-q"}, {"remote", "add", "origin", "git@github.com:haiodo/foundation.git"}} {
			if out, err := exec.Command("git", append([]string{"-C", d}, args...)...).CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v\n%s", args, err, out)
			}
		}
		return d
	}
	a, b := clone("a"), clone("b")
	pa := filepath.Join(a, "design", "P.tenn")
	if err := e.fake.Open(pa, commentDoc); err != nil {
		t.Fatal(err)
	}
	if _, err := e.fake.Client.Comments(servicewire.CommentsOp{Path: pa, Op: servicewire.CommentAdd, Text: "from clone a", Anchor: &comments.Anchor{Element: []string{"A"}, Node: "x"}}); err != nil {
		t.Fatal(err)
	}
	// An agent working in the other clone finds it, with a link that select takes.
	agent := session(t, e.dir, b, "v1", nil)
	list := parseComments(t, mustOK(t, agent, "comments", map[string]any{"file": "design/P.tenn"}))
	if len(list) != 1 || list[0].Text != "from clone a" || list[0].LinkText != "design/P.tenn#A/x" {
		t.Fatalf("comments in the other clone: %+v", list)
	}
	if !strings.Contains(mustOK(t, e.agent, "get_state", nil), "a.tenn") {
		t.Fatal("setup")
	}
}
