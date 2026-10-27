package mcpserver

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/haiodo/tenniarb/go/servicewire"
	"github.com/haiodo/tenniarb/go/servicewire/wiretest"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// connectFake connects a fake app to the service of dir and waits for it to register.
func connectFake(t *testing.T, dir string) (*wiretest.Fake, context.CancelFunc) {
	t.Helper()
	fake := wiretest.New()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	c, err := servicewire.RunProvider(ctx, dir, fake, 20*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	fake.Client = c
	wctx, wcancel := context.WithTimeout(ctx, 5*time.Second)
	defer wcancel()
	if err := c.WaitConnected(wctx); err != nil {
		t.Fatal(err)
	}
	return fake, cancel
}

func edits(find, replace string) map[string]any {
	return map[string]any{"edits": []map[string]any{{"find": find, "replace": replace}}}
}

func withFile(m map[string]any, file string) map[string]any {
	m["file"] = file
	return m
}

const liveDisk = "element \"A\" {\n    item \"x\"\n}\n"

func TestLiveRouting(t *testing.T) {
	dir := serviceDir(t)
	startService(t, dir, "v1", nil, 0)
	root := tempRoot(t, map[string]string{"a.tenn": liveDisk, "b.tenn": liveDisk})
	cs := session(t, dir, root, "v1", nil)
	mustFail(t, cs, "selection", nil, "not connected")

	fake, _ := connectFake(t, dir)
	abs := filepath.Join(root, "a.tenn")
	if err := fake.Open(abs, liveDisk); err != nil {
		t.Fatal(err)
	}
	typed := strings.Replace(liveDisk, "}\n", "    item \"typed\"\n}\n", 1)
	if err := fake.Type(abs, typed); err != nil {
		t.Fatal(err)
	}

	if got := mustOK(t, cs, "read", map[string]any{"file": "a.tenn"}); !strings.Contains(got, `item "typed"`) || !strings.Contains(got, "unsaved changes") {
		t.Errorf("read must return the unsaved text: %s", got)
	}
	if got := mustOK(t, cs, "get_state", nil); !strings.Contains(got, "a.tenn, open in Tenniarb, unsaved:") || !strings.Contains(got, "A.typed item") {
		t.Errorf("state must show the open document: %s", got)
	}

	got := mustOK(t, cs, "edit", withFile(edits(`item "typed"`, `item "agent"`), "a.tenn"))
	if !strings.Contains(got, "one undo step") {
		t.Errorf("edit result: %s", got)
	}
	if fileText(t, root, "a.tenn") != liveDisk {
		t.Error("an edit of an open document must not touch the file")
	}
	text, undo := fake.TextOf(abs)
	if !strings.Contains(text, `item "agent"`) || undo != 2 {
		t.Errorf("app text %q, undo depth %d", text, undo)
	}

	mustFail(t, cs, "edit", withFile(edits(`item "agent"`, `item "agent" {`), "a.tenn"), "parse error")
	mustFail(t, cs, "edit", withFile(edits(`item "agent"`, `use Nope.Thing as n`), "a.tenn"), "adds errors")
	if _, undo := fake.TextOf(abs); undo != 2 {
		t.Error("a refused edit reached the app")
	}

	// The user types while the agent's edit is on its way: the app refuses, nothing is lost.
	fake.BeforeApply = func() { fake.Type(abs, strings.Replace(text, `"agent"`, `"user"`, 1)) }
	mustFail(t, cs, "edit", withFile(edits(`item "x"`, `item "y"`), "a.tenn"), "document changed")
	fake.BeforeApply = nil
	if now, _ := fake.TextOf(abs); !strings.Contains(now, `item "user"`) || strings.Contains(now, `item "y"`) {
		t.Errorf("the user's typing must win: %q", now)
	}

	fake.SetSelection(servicewire.Selection{Path: abs, Element: "A", Nodes: []string{"A.x"}})
	if got := mustOK(t, cs, "selection", nil); !strings.Contains(got, `"file": "a.tenn"`) || !strings.Contains(got, `"A.x"`) {
		t.Errorf("selection: %s", got)
	}
	mustOK(t, cs, "select", map[string]any{"file": "a.tenn", "nodes": []string{"A.x"}, "reveal": true})
	if sel := fake.Selected(); len(sel) != 1 || sel[0].Path != abs || !sel[0].Reveal || sel[0].Nodes[0] != "A.x" {
		t.Errorf("select reached the app as %+v", sel)
	}
	mustFail(t, cs, "select", map[string]any{"file": "a.tenn", "nodes": []string{"A.nope"}}, "nothing named")
	mustFail(t, cs, "select", map[string]any{"file": "b.tenn", "nodes": []string{"A.x"}}, "not open in Tenniarb")

	res, _ := call(t, cs, "render", map[string]any{"file": "a.tenn", "view": true})
	if res.IsError || len(res.Content) == 0 || !bytes.Equal(res.Content[0].(*mcp.ImageContent).Data, wiretest.PNG) {
		t.Errorf("render view: %+v", res.Content)
	}

	// A file the app does not have open is edited on disk.
	mustOK(t, cs, "edit", withFile(edits(`item "x"`, `item "disk"`), "b.tenn"))
	if !strings.Contains(fileText(t, root, "b.tenn"), `item "disk"`) {
		t.Error("headless edit not written")
	}
	// Closed in the app: back to disk.
	fake.Close(abs)
	mustOK(t, cs, "edit", withFile(edits(`item "x"`, `item "closed"`), "a.tenn"))
	if !strings.Contains(fileText(t, root, "a.tenn"), `item "closed"`) {
		t.Error("edit after close not written to disk")
	}
}

func TestLiveAppDisconnects(t *testing.T) {
	dir := serviceDir(t)
	startService(t, dir, "v1", nil, 0)
	root := tempRoot(t, map[string]string{"a.tenn": liveDisk})
	cs := session(t, dir, root, "v1", nil)
	fake, stop := connectFake(t, dir)
	abs := filepath.Join(root, "a.tenn")
	fake.Open(abs, liveDisk)
	fake.Type(abs, strings.Replace(liveDisk, "x", "live", 1))
	if got := mustOK(t, cs, "read", map[string]any{"file": "a.tenn"}); !strings.Contains(got, "live") {
		t.Fatalf("read: %s", got)
	}
	stop()
	for i := 0; ; i++ {
		res, text := call(t, cs, "selection", nil)
		if res.IsError && strings.Contains(text, "not connected") {
			break
		}
		if i > 200 {
			t.Fatal("the service did not notice the app left")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := mustOK(t, cs, "read", map[string]any{"file": "a.tenn"}); strings.Contains(got, "live") {
		t.Errorf("without the app read must use the disk: %s", got)
	}
}

// TestLiveRestartTombstone: after a version restart the old service must not edit a file the app
// has open (the app now talks to the new service); the new service routes the edit to the app.
func TestLiveRestartTombstone(t *testing.T) {
	dir := serviceDir(t)
	root := tempRoot(t, map[string]string{"a.tenn": liveDisk})
	startService(t, dir, "v1", nil, 0)
	oldSession := session(t, dir, root, "v1", nil)
	fake, _ := connectFake(t, dir)
	abs := filepath.Join(root, "a.tenn")
	fake.Open(abs, liveDisk)

	newSession := session(t, dir, root, "v2", func() error { startService(t, dir, "v2", nil, 0); return nil })
	mustFail(t, oldSession, "edit", withFile(edits(`item "x"`, `item "old"`), "a.tenn"), "newer tenniarb-service")
	if fileText(t, root, "a.tenn") != liveDisk {
		t.Fatal("the old service wrote a file open in the app")
	}
	for i := 0; ; i++ {
		if res, _ := call(t, newSession, "selection", nil); !res.IsError {
			break
		}
		if i > 300 {
			t.Fatal("the app did not reconnect to the new service")
		}
		time.Sleep(10 * time.Millisecond)
	}
	mustOK(t, newSession, "edit", withFile(edits(`item "x"`, `item "new"`), "a.tenn"))
	if text, _ := fake.TextOf(abs); !strings.Contains(text, `item "new"`) || fileText(t, root, "a.tenn") != liveDisk {
		t.Errorf("edit after reconnect: app %q", text)
	}
}
