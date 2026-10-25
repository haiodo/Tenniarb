package mcpserver

import (
	"bytes"
	"context"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/scene"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// workspace copies go/scene/testdata into a temp root and connects an in-memory client.
func workspace(t *testing.T) (*mcp.ClientSession, string) {
	t.Helper()
	root := t.TempDir()
	copyTree(t, filepath.Join("..", "scene", "testdata"), root)
	srv, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	ct, st := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := srv.MCP().Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs, root
}

func copyTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(to, rel), 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(to, rel), b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func call(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) (*mcp.CallToolResult, string) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	var texts []string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			texts = append(texts, tc.Text)
		}
	}
	return res, strings.Join(texts, "\n")
}

func mustOK(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) string {
	t.Helper()
	res, text := call(t, cs, name, args)
	if res.IsError {
		t.Fatalf("%s failed: %s", name, text)
	}
	return text
}

func mustFail(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any, want string) {
	t.Helper()
	res, text := call(t, cs, name, args)
	if !res.IsError {
		t.Fatalf("%s should fail, got: %s", name, text)
	}
	if !strings.Contains(text, want) {
		t.Fatalf("%s error %q lacks %q", name, text, want)
	}
}

func fileText(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func noStaging(t *testing.T, root string) {
	t.Helper()
	m, _ := filepath.Glob(filepath.Join(root, ".tenn-edit-*"))
	if len(m) > 0 {
		t.Fatalf("staging files left behind: %v", m)
	}
}

func TestToolList(t *testing.T) {
	cs, _ := workspace(t)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, tl := range res.Tools {
		got[tl.Name] = true
	}
	for _, n := range []string{"guide", "get_state", "read", "edit", "render", "validate"} {
		if !got[n] {
			t.Errorf("tool %s is not registered", n)
		}
	}
}

var tennBlock = regexp.MustCompile("(?s)```tenn\n(.*?)```")

func TestGuide(t *testing.T) {
	cs, root := workspace(t)
	if all := mustOK(t, cs, "guide", nil); !strings.Contains(all, "## 4. `use`") || !strings.Contains(all, "Y axis points up") {
		t.Errorf("guide without topic lacks spec or examples")
	}
	sec := mustOK(t, cs, "guide", map[string]any{"topic": "4.7a"})
	if !strings.Contains(sec, "Slots как в Svelte") || strings.Contains(sec, "4.8 Detach") {
		t.Errorf("topic 4.7a returned %q", sec[:min(len(sec), 80)])
	}
	if got := mustOK(t, cs, "guide", map[string]any{"topic": "nothing-like-this"}); !strings.Contains(got, "no section matches") {
		t.Errorf("unknown topic: %q", got)
	}
	blocks := tennBlock.FindAllStringSubmatch(examples, -1)
	if len(blocks) == 0 {
		t.Fatal("no examples")
	}
	for i, b := range blocks {
		name := filepath.Join(root, "example_"+string(rune('a'+i))+".tenn")
		if err := os.WriteFile(name, []byte(b[1]), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := scene.Load(root, filepath.Base(name)); err != nil {
			t.Errorf("example %d does not validate: %v", i, err)
		}
	}
}

func TestSpecCopyInSync(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("..", "..", "docs", "spec", "tenn-scene.md"))
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("UPDATE_SPEC") != "" {
		if err := os.WriteFile(filepath.Join("spec", "tenn-scene.md"), want, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if !bytes.Equal([]byte(specText), want) {
		t.Fatal("mcpserver/spec/tenn-scene.md differs from docs/spec/tenn-scene.md: run UPDATE_SPEC=1 go test ./mcpserver -run SpecCopy")
	}
}

func TestGetState(t *testing.T) {
	cs, _ := workspace(t)
	text := mustOK(t, cs, "get_state", nil)
	for _, want := range []string{`"file": "import.tenn"`, `"path": "Screens.Login"`, `"path": "Screens.Login.ok"`, `"file": "lib/kit.tenn"`, `"severity": "error"`} {
		if !strings.Contains(text, want) {
			t.Errorf("state lacks %s", want)
		}
	}
	if strings.Contains(text, ".tenn-edit") {
		t.Error("staging files listed")
	}
}

func TestReadFileAndNode(t *testing.T) {
	cs, root := workspace(t)
	whole := mustOK(t, cs, "read", map[string]any{"file": "import.tenn"})
	if !strings.HasPrefix(whole, "import.tenn (") || !strings.Contains(whole, "    1| import \"lib/kit.tenn\" as kit") {
		t.Errorf("whole file: %q", whole[:min(len(whole), 120)])
	}
	node := mustOK(t, cs, "read", map[string]any{"file": "import.tenn", "path": "Screens.Login"})
	src := fileText(t, root, "import.tenn")
	first := strings.Index(src, `element "Login"`)
	if wantFirst := 1 + strings.Count(src[:first], "\n"); wantFirst != 8 || !strings.Contains(node, `lines 8-13`) {
		t.Errorf("node header: %q", strings.SplitN(node, "\n", 2)[0])
	}
	lines := strings.Split(strings.TrimRight(node, "\n"), "\n")
	if got := lines[len(lines)-1]; got != "   13|     }" {
		t.Errorf("node ends with %q", got)
	}
	if !strings.Contains(node, "use kit.Button as hole") {
		t.Error("node lacks its last child")
	}
	item := mustOK(t, cs, "read", map[string]any{"file": "import.tenn", "path": "Screens.Button"})
	if !strings.Contains(item, "lines 4-7") || !strings.Contains(item, "width 33") {
		t.Errorf("item: %q", item)
	}
	mustFail(t, cs, "read", map[string]any{"file": "import.tenn", "path": "Screens.Nope"}, `nothing named "Nope"`)
}

func TestReadAmbiguousPath(t *testing.T) {
	cs, root := workspace(t)
	if err := os.WriteFile(filepath.Join(root, "dup.tenn"), []byte("element \"E\" {\n    item \"A\"\n    item \"A\"\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustFail(t, cs, "read", map[string]any{"file": "dup.tenn", "path": "E.A"}, "ambiguous")
}

func TestPathEscapeRejected(t *testing.T) {
	cs, root := workspace(t)
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.tenn")
	if err := os.WriteFile(secret, []byte("element \"S\" {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(root, "link.tenn")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linkdir")); err != nil {
		t.Fatal(err)
	}
	bad := []string{"../secret.tenn", "lib/../../secret.tenn", secret, "link.tenn", "linkdir/secret.tenn", "linkdir/new.tenn", "import.txt", ""}
	edits := []map[string]any{{"find": "", "replace": "element \"X\" {}\n"}}
	for _, p := range bad {
		for _, tool := range []string{"read", "edit", "validate", "render"} {
			args := map[string]any{"file": p}
			if tool == "edit" {
				args["edits"] = edits
			}
			if tool == "validate" && p == "" {
				continue // empty file means the whole workspace
			}
			res, text := call(t, cs, tool, args)
			if !res.IsError {
				t.Errorf("%s %q accepted: %s", tool, p, text)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(outside, "new.tenn")); err == nil {
		t.Error("edit wrote outside the root")
	}
	if got, _ := os.ReadFile(secret); string(got) != "element \"S\" {}\n" {
		t.Errorf("outside file changed: %q", got)
	}
}

func TestEditApplies(t *testing.T) {
	cs, root := workspace(t)
	text := mustOK(t, cs, "edit", map[string]any{"file": "import.tenn", "edits": []map[string]any{
		{"find": "color green", "replace": "color red"},
		{"find": "use kit.Button", "replace": "use kit.Button", "all": true},
	}})
	if !strings.HasPrefix(text, "ok: import.tenn written") {
		t.Errorf("result: %s", text)
	}
	if got := fileText(t, root, "import.tenn"); !strings.Contains(got, "color red") || strings.Contains(got, "color green") {
		t.Errorf("file not edited:\n%s", got)
	}
	noStaging(t, root)
}

func TestEditAmbiguousRejected(t *testing.T) {
	cs, root := workspace(t)
	before := fileText(t, root, "import.tenn")
	mustFail(t, cs, "edit", map[string]any{"file": "import.tenn", "edits": []map[string]any{
		{"find": "color green", "replace": "color red"},
		{"find": "use kit.Button", "replace": "use kit.Badge"},
	}}, "matches 3 times")
	if fileText(t, root, "import.tenn") != before {
		t.Error("file changed by a rejected edit")
	}
	mustFail(t, cs, "edit", map[string]any{"file": "import.tenn", "edits": []map[string]any{
		{"find": "no such text", "replace": "x"},
	}}, "not found")
	noStaging(t, root)
}

func TestEditParseErrorRollsBack(t *testing.T) {
	cs, root := workspace(t)
	before := fileText(t, root, "import.tenn")
	mustFail(t, cs, "edit", map[string]any{"file": "import.tenn", "edits": []map[string]any{
		{"find": "color green", "replace": "color red"},
		{"find": "use Button as cancel", "replace": "use Button as cancel {"},
	}}, "parse error")
	if fileText(t, root, "import.tenn") != before {
		t.Error("file changed by an edit with a parse error")
	}
	noStaging(t, root)
}

func TestEditNewValidationErrorRollsBack(t *testing.T) {
	cs, root := workspace(t)
	before := fileText(t, root, "import.tenn")
	mustFail(t, cs, "edit", map[string]any{"file": "import.tenn", "edits": []map[string]any{
		{"find": "use Button as cancel", "replace": "use Missing as cancel"},
	}}, "adds errors")
	if fileText(t, root, "import.tenn") != before {
		t.Error("file changed by an edit with a validation error")
	}
	noStaging(t, root)
}

func TestEditKeepsExistingErrors(t *testing.T) {
	cs, root := workspace(t)
	text := mustOK(t, cs, "edit", map[string]any{"file": "layout_errors.tenn", "edits": []map[string]any{
		{"find": "element \"E\" {\n", "replace": "// edited\nelement \"E\" {\n"},
	}})
	if !strings.Contains(text, "warning") || !strings.Contains(fileText(t, root, "layout_errors.tenn"), "// edited") {
		t.Errorf("edit of a file with old errors: %s", text)
	}
}

func TestEditCreatesFile(t *testing.T) {
	cs, root := workspace(t)
	mustOK(t, cs, "edit", map[string]any{"file": "new.tenn", "edits": []map[string]any{
		{"find": "", "replace": "element \"N\" {\n    item \"A\"\n}\n"},
	}})
	if !strings.Contains(fileText(t, root, "new.tenn"), `element "N"`) {
		t.Error("file not created")
	}
	mustFail(t, cs, "edit", map[string]any{"file": "other.tenn", "edits": []map[string]any{{"find": "a", "replace": "b"}}}, "does not exist")
}

func TestValidate(t *testing.T) {
	cs, _ := workspace(t)
	if got := mustOK(t, cs, "validate", map[string]any{"file": "import.tenn"}); !strings.Contains(got, "0 errors") {
		t.Errorf("import.tenn: %s", got)
	}
	if got := mustOK(t, cs, "validate", map[string]any{"file": "layout_errors.tenn"}); !strings.Contains(got, "layout_errors.tenn:") || strings.Contains(got, " 0 errors") {
		t.Errorf("layout_errors.tenn: %s", got)
	}
	if got := mustOK(t, cs, "validate", nil); !strings.Contains(got, "files checked") {
		t.Errorf("workspace: %s", got)
	}
}

func TestRenderPNG(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("render needs CoreGraphics")
	}
	cs, _ := workspace(t)
	res, text := call(t, cs, "render", map[string]any{"file": "layout.tenn", "element": "Row", "scale": 2})
	if res.IsError {
		t.Fatal(text)
	}
	var img *mcp.ImageContent
	for _, c := range res.Content {
		if ic, ok := c.(*mcp.ImageContent); ok {
			img = ic
		}
	}
	if img == nil || img.MIMEType != "image/png" || len(img.Data) == 0 {
		t.Fatalf("no PNG in result: %+v", res.Content)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(img.Data))
	if err != nil || cfg.Width < 10 || cfg.Height < 10 {
		t.Fatalf("bad PNG: %v %+v", err, cfg)
	}
	if !strings.Contains(text, "warnings") {
		t.Errorf("text part: %q", text)
	}
	mustFail(t, cs, "render", map[string]any{"file": "layout_errors.tenn", "element": "E"}, "fix the errors")
	mustFail(t, cs, "render", map[string]any{"file": "layout.tenn", "element": "Row", "scale": 100}, "scale")
}

func TestReadNodeThroughImport(t *testing.T) {
	cs, _ := workspace(t)
	out := mustOK(t, cs, "read", map[string]any{"file": "import.tenn", "path": "kit.Button"})
	if !strings.HasPrefix(out, "lib/kit.tenn item ") {
		t.Errorf("node of an imported file: %q", strings.SplitN(out, "\n", 2)[0])
	}
	mustFail(t, cs, "read", map[string]any{"file": "import.tenn", "path": "kit"}, "names an import")
}
