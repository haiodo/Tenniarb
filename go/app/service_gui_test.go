//go:build darwin || linux || windows

package app_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/haiodo/gowt/qa"
	"github.com/haiodo/gowt/qa/qatest"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func docOf(t *testing.T, app *qatest.Client, path string) docState {
	t.Helper()
	var d docState
	if err := json.Unmarshal(app.State("doc", "path", path), &d); err != nil {
		t.Fatal(err)
	}
	return d
}

// TestServiceProvider runs the app as a document provider of a real tenniarb-service and drives it
// through the real proxy, as an agent does.
func TestServiceProvider(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	bin := t.TempDir()
	if out, err := exec.Command("go", "build", "-o", bin+string(filepath.Separator), "../cmd/tenniarb-service", "../cmd/tenniarb-mcp").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	exe := func(name string) string {
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		return filepath.Join(bin, name)
	}
	// serviceDir is the TENNIARB_SERVICE_DIR TestMain gave the app.
	svc := exec.Command(exe("tenniarb-service"), "-dir", serviceDir, "-idle", "0")
	svc.Stderr = os.Stderr
	if err := svc.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { svc.Process.Kill(); svc.Wait() })

	root := t.TempDir()
	copyFile := func(src, dst string) string {
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(root, dst)
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	copyFile("../scene/testdata/lib/kit.tenn", "lib/kit.tenn")
	copyFile("../scene/testdata/lib/base.tenn", "lib/base.tenn")
	screen := copyFile("../scene/testdata/slots.tenn", "slots.tenn")
	// the fixtures pile their items up, and a double click must hit one
	diagram := filepath.Join(root, "items.tenn")
	os.WriteFile(diagram, []byte("element \"Items\" {\n    item \"plain\" {\n        pos -200 0\n    }\n    item \"zero\" { pos 0 0 }\n    link \"plain\" \"zero\"\n}\n"), 0o644)
	screenText, _ := os.ReadFile(screen)

	app := qatest.App(t)
	var start struct{ Path string }
	json.Unmarshal(app.State("state"), &start)
	for _, p := range []string{screen, diagram} {
		app.State("command", "name", "open-document", "path", p)
	}
	app.Settle()
	// the later tests start from the window the earlier ones left
	t.Cleanup(func() {
		app.State("command", "name", "open-document", "path", start.Path)
		app.State("activate", "path", start.Path)
		app.State("command", "name", "close-others")
	})

	proxy := exec.Command(exe("tenniarb-mcp"), "-root", root)
	proxy.Env = append(os.Environ(), "TENNIARB_SERVICE_DIR="+serviceDir)
	proxy.Stderr = os.Stderr
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "e2e"}, nil).Connect(context.Background(), &mcp.CommandTransport{Command: proxy}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	call := func(name string, args map[string]any) (string, bool) {
		t.Helper()
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var b strings.Builder
		for _, c := range res.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				b.WriteString(tc.Text)
			}
		}
		return b.String(), res.IsError
	}

	// the app polls for the service every 2 s
	var sel struct {
		Open []struct{ File string }
	}
	app.Eventually(10*time.Second, func() (any, bool) {
		out, failed := call("selection", nil)
		sel.Open = nil
		json.Unmarshal([]byte(out), &sel)
		return out, !failed && len(sel.Open) == 2
	})
	state, failed := call("get_state", nil)
	if failed || !strings.Contains(state, "slots.tenn") || !strings.Contains(state, "items.tenn") {
		t.Fatalf("get_state:\n%s", state)
	}

	// one window at a time, so keys and the inline editor go to it
	app.State("activate", "path", screen)
	app.State("command", "name", "close-others")
	app.Settle()

	// an agent edit lands in the window as one undo step, and the file stays as it was
	before := docOf(t, app, screen)
	if out, failed := call("edit", map[string]any{"file": "slots.tenn", "edits": []map[string]any{{"find": `item "Email" { height 10 }`, "replace": `item "Email" { height 14 }`}}}); failed {
		t.Fatalf("edit: %s", out)
	}
	after := docOf(t, app, screen)
	if text := string(app.State("text")); after.Version <= before.Version || !after.Dirty || !strings.Contains(text, "height 14") {
		t.Fatalf("edit not in the window: %+v\n%s", after, text)
	}
	if b, _ := os.ReadFile(screen); string(b) != string(screenText) {
		t.Fatal("the edit reached the disk")
	}
	app.Key("Mod+Z")
	var text string
	json.Unmarshal(app.State("text"), &text)
	if d := docOf(t, app, screen); d.Dirty || text != string(screenText) {
		t.Fatalf("Cmd+Z: %+v\n%s", d, text)
	}
	if out, _ := call("read", map[string]any{"file": "slots.tenn", "path": "Slots.w1.Email"}); !strings.Contains(out, "height 10") {
		t.Errorf("the service does not see the undo:\n%s", out)
	}

	// select reveals a node: the window comes to front with the node selected
	if out, failed := call("select", map[string]any{"file": "slots.tenn", "nodes": []string{"Slots.w2"}, "reveal": true}); failed {
		t.Fatalf("select: %s", out)
	}
	if d := docOf(t, app, screen); !slices.Equal(d.Nodes, []string{"w2"}) {
		t.Fatalf("selected %v", d.Nodes)
	}
	var st struct{ Path string }
	json.Unmarshal(app.State("state"), &st)
	if st.Path != screen {
		t.Fatalf("front window %s", st.Path)
	}
	if out, _ := call("selection", nil); !strings.Contains(out, `"Slots.w2"`) {
		t.Errorf("selection:\n%s", out)
	}

	// the toolbar shows the agent; with Follow on the window goes where the agent reads, without it not
	app.Eventually(5*time.Second, func() (any, bool) {
		d := docOf(t, app, screen)
		return d.Agent, strings.Contains(d.Agent, "e2e")
	})
	app.State("command", "name", "toolbar", "item", "follow")
	if out, failed := call("read", map[string]any{"file": "slots.tenn", "path": "Slots.w1"}); failed {
		t.Fatalf("read: %s", out)
	}
	app.Eventually(5*time.Second, func() (any, bool) {
		d := docOf(t, app, screen)
		return d.Nodes, d.Follow && slices.Equal(d.Nodes, []string{"w1"})
	})
	app.State("command", "name", "toolbar", "item", "follow")
	if out, failed := call("read", map[string]any{"file": "slots.tenn", "path": "Slots.w2"}); failed {
		t.Fatalf("read: %s", out)
	}
	time.Sleep(500 * time.Millisecond) // a presence that must not move the window needs time to not arrive
	if d := docOf(t, app, screen); d.Follow || !slices.Equal(d.Nodes, []string{"w1"}) {
		t.Fatalf("follow off moved the selection: %+v", d)
	}

	// while the user types in the window an agent edit is refused; after the commit it goes through
	app.State("command", "name", "open-document", "path", diagram)
	app.State("command", "name", "close-others")
	app.Settle()
	app.Eventually(5*time.Second, func() (any, bool) {
		out, _ := call("selection", nil)
		return out, strings.Contains(out, `"items.tenn"`) && !strings.Contains(out, `"slots.tenn"`)
	})
	if out, failed := call("select", map[string]any{"file": "items.tenn", "nodes": []string{"Items.plain"}, "reveal": true}); failed {
		t.Fatalf("select item: %s", out)
	}
	s := editor(t, app)
	if s.Path != diagram || !slices.Equal(s.Selection, []string{"plain"}) {
		t.Fatalf("classic select: %+v", s)
	}
	app.Find(qa.ID("canvas")).DoubleClickAt(center(s.Screen["plain"]))
	if s = editor(t, app); !s.Inline {
		t.Fatal("no inline editor")
	}
	app.Find(qa.Class("Text").In(qa.ID("canvas"))).Type("typed")
	edit := map[string]any{"file": "items.tenn", "edits": []map[string]any{{"find": `item "zero"`, "replace": "item \"agent\"\n    item \"zero\""}}}
	if out, failed := call("edit", edit); !failed || !strings.Contains(out, "typing") {
		t.Fatalf("an edit while typing was taken: %s", out)
	}
	app.Key("Enter")
	if out, failed := call("edit", edit); failed {
		t.Fatalf("edit after typing: %s", out)
	}
	json.Unmarshal(app.State("text"), &text)
	if !strings.Contains(text, "typed") || !strings.Contains(text, `item "agent"`) {
		t.Fatalf("typed text and agent edit:\n%s", text)
	}
}
