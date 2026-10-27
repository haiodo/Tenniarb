package main

import (
	"bytes"
	"context"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/haiodo/tenniarb/go/servicewire"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestProxyDeps keeps the proxy small: none of the heavy packages may reach its binary.
func TestProxyDeps(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, dep := range strings.Fields(string(out)) {
		for _, banned := range []string{"/go/scene", "/go/render", "/go/mcpserver", "haiodo/gowt", "dop251/goja", "modelcontextprotocol"} {
			if strings.Contains(dep, banned) {
				t.Errorf("tenniarb-mcp links %s", dep)
			}
		}
	}
}

func build(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("go", "build", "-o", dir+string(filepath.Separator), ".", "../tenniarb-service")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
}

// TestEndToEnd spawns the built proxy as an agent would; the proxy starts the built service.
func TestEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("builds both binaries")
	}
	bin := t.TempDir()
	build(t, bin)
	svcDir, err := os.MkdirTemp("", "tme")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(svcDir) })
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "ui.tenn"), []byte("element \"Login\" {\n    item \"Card\" {\n        auto-layout column\n        item \"Title\" { title \"Sign in\" }\n    }\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "map.tenn"), []byte("element \"Map\" {\n    item \"Idea\" { pos 0 0 }\n    item \"Next\" { pos 120 0 }\n    link \"Idea\" \"Next\"\n}\n"), 0o644)

	env := append(os.Environ(), servicewire.EnvDir+"="+svcDir, "TENNIARB_SERVICE_IDLE=2s")
	connect := func() (*mcp.ClientSession, *exec.Cmd) {
		cmd := exec.Command(filepath.Join(bin, "tenniarb-mcp"), "-root", root)
		cmd.Env = env
		cmd.Stderr = os.Stderr
		cs, err := mcp.NewClient(&mcp.Implementation{Name: "e2e"}, nil).Connect(context.Background(), &mcp.CommandTransport{Command: cmd}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return cs, cmd
	}
	t.Cleanup(func() {
		if e, err := servicewire.ReadEndpoint(svcDir); err == nil {
			if p, err := os.FindProcess(e.PID); err == nil {
				p.Kill()
			}
		}
	})

	cs, proxy := connect()
	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil || len(tools.Tools) < 8 {
		t.Fatalf("tools: %v %v", tools, err)
	}
	text := func(name string, args map[string]any) (*mcp.CallToolResult, string) {
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
		if res.IsError {
			t.Fatalf("%s failed: %s", name, b.String())
		}
		return res, b.String()
	}
	if _, got := text("read", map[string]any{"file": "ui.tenn", "path": "Login.Card"}); !strings.Contains(got, "auto-layout column") {
		t.Errorf("read: %s", got)
	}
	text("edit", map[string]any{"file": "ui.tenn", "edits": []map[string]any{{"find": `"Sign in"`, "replace": `"Log in"`}}})
	if b, _ := os.ReadFile(filepath.Join(root, "ui.tenn")); !strings.Contains(string(b), `"Log in"`) {
		t.Error("edit not written")
	}
	if runtime.GOOS == "darwin" {
		for _, el := range []struct{ file, element string }{{"ui.tenn", "Login"}, {"map.tenn", "Map"}} {
			res, _ := text("render", map[string]any{"file": el.file, "element": el.element})
			img := res.Content[0].(*mcp.ImageContent)
			if _, err := png.DecodeConfig(bytes.NewReader(img.Data)); err != nil {
				t.Errorf("render %s: %v", el.file, err)
			}
		}
	}

	// A second agent session shares the running service.
	cs2, proxy2 := connect()
	if _, err := cs2.ListTools(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	e, err := servicewire.ReadEndpoint(svcDir)
	if err != nil {
		t.Fatal(err)
	}
	var rss []string
	for _, pid := range []int{proxy.Process.Pid, proxy2.Process.Pid, e.PID} {
		out, _ := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(pid)).Output()
		rss = append(rss, strings.TrimSpace(string(out)))
	}
	t.Logf("RSS KB: proxy %s, proxy %s, service %s", rss[0], rss[1], rss[2])
	cs.Close()
	cs2.Close()

	// Without sessions the service exits after its idle time and removes the endpoint.
	deadline := time.Now().Add(15 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(svcDir, "endpoint")); os.IsNotExist(err) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the service did not exit when idle")
		}
		time.Sleep(100 * time.Millisecond)
	}
}
