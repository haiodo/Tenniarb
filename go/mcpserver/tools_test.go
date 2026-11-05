package mcpserver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// componentDoc is a component with parameters and styles, a use of it, and a template.
const componentDoc = `element "Kit" {
  element "Button" {
    kind Component
    export let title = "Button"
    export let kind = primary
    styles {
      primary { color #6b3ad3 }
      secondary { color #ffffff }
      kind one-of primary secondary
    }
    template %{<button class="${kind}">${title}</button>%}
    item Btn { auto-layout row; use-style ${kind}; item Label { title ${title} } }
  }
}
element "Screen" {
  use Kit.Button as b { title "Save"; kind secondary }
}
`

// client connects an in-memory client to srv.
func client(t *testing.T, srv *Server) *mcp.ClientSession {
	t.Helper()
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
	return cs
}

// componentWorkspace writes componentDoc as kit.tenn into a temp root and connects a client.
func componentWorkspace(t *testing.T) (*mcp.ClientSession, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "kit.tenn"), []byte(componentDoc), 0o644); err != nil {
		t.Fatal(err)
	}
	srv, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	return client(t, srv), root
}

// TestStateShowsComponent: get_state names the parameters, the styles and the groups of a component, so an
// agent picks a variant without reading the file.
func TestStateShowsComponent(t *testing.T) {
	cs, _ := componentWorkspace(t)
	out := mustOK(t, cs, "get_state", map[string]any{"file": "kit.tenn", "depth": 2})
	for _, want := range []string{
		"kind=component",
		"params=title,kind",
		"styles=primary,secondary",
		"groups=kind:one-of(primary secondary)",
		"args=title=Save kind=secondary",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("get_state lacks %q:\n%s", want, out)
		}
	}
}

// TestGen: the template of the component with the parameters of a use, and with a -set style override.
func TestGen(t *testing.T) {
	cs, _ := componentWorkspace(t)
	got := mustOK(t, cs, "gen", map[string]any{"file": "kit.tenn", "element": "Screen.b"})
	if got != `<button class="secondary">Save</button>` {
		t.Errorf("gen of the use: %q", got)
	}
	got = mustOK(t, cs, "gen", map[string]any{"file": "kit.tenn", "element": "Kit.Button",
		"set": map[string]string{"title": "Go", "kind": "primary"}})
	if got != `<button class="primary">Go</button>` {
		t.Errorf("gen with set: %q", got)
	}
	res, text := call(t, cs, "gen", map[string]any{"file": "kit.tenn", "element": "Screen"})
	if !res.IsError || !strings.Contains(text, "template") {
		t.Errorf("gen of an element without a template: %v %q", res.IsError, text)
	}
}

// TestSuggest: the report of what could be tidied, for a file with a repeat.
func TestSuggest(t *testing.T) {
	root := t.TempDir()
	src := `element "Screen" {
  item col { auto-layout column
    item A { width 10; height 10; color #ff0000; title One }
    item B { width 10; height 10; color #00ff00; title Two }
  }
}
`
	if err := os.WriteFile(filepath.Join(root, "a.tenn"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	srv, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	cs := client(t, srv)
	out := mustOK(t, cs, "suggest", map[string]any{"file": "a.tenn"})
	if !strings.Contains(out, "finding") || !strings.Contains(out, "styles") {
		t.Errorf("suggest found nothing about the two items:\n%s", out)
	}
}

// TestStateCapped: the whole-workspace listing stops at maxState and points at file and find; one file stays whole.
func TestStateCapped(t *testing.T) {
	files := map[string]string{}
	for i := 0; i < 40; i++ {
		var b strings.Builder
		b.WriteString("element Board {\n")
		for j := 0; j < 20; j++ {
			b.WriteString("\titem Card { width 10; height 10 }\n")
		}
		b.WriteString("}\n")
		files["board"+string(rune('A'+i/26))+string(rune('A'+i%26))+".tenn"] = b.String()
	}
	cs := memSession(t, tempRoot(t, files))
	got := mustOK(t, cs, "get_state", map[string]any{})
	if len(got) > maxState+200 {
		t.Fatalf("whole workspace is %d bytes", len(got))
	}
	if !strings.Contains(got, "cut: get_state {file}") {
		t.Fatalf("no hint where to go on:\n%s", got[len(got)-200:])
	}
	if one := mustOK(t, cs, "get_state", map[string]any{"file": "boardAA.tenn"}); !strings.Contains(one, "Card") || strings.Contains(one, "cut:") {
		t.Fatalf("one file:\n%s", one)
	}
}

// TestCallLog: a call is logged where TENNIARB_MCP_LOG says, and "off" writes nothing.
func TestCallLog(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TENNIARB_MCP_LOG", filepath.Join(dir, "calls.log"))
	cs := memSession(t, tempRoot(t, map[string]string{"s.tenn": "element S { item A { width 10; height 10 } }\n"}))
	mustOK(t, cs, "get_state", map[string]any{})
	b, err := os.ReadFile(filepath.Join(dir, "calls.log"))
	if err != nil || !strings.Contains(string(b), `"tool":"get_state"`) {
		t.Fatalf("log: %q %v", b, err)
	}
	t.Setenv("TENNIARB_MCP_LOG", "off")
	mustOK(t, cs, "get_state", map[string]any{})
	if b, _ := os.ReadFile(filepath.Join(dir, "calls.log")); strings.Count(string(b), "get_state") != 1 {
		t.Fatalf("off wrote a line: %q", b)
	}
}
