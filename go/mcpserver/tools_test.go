package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
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

// TestFindOnceAndQuiet: a search lists a mounted file through the document that mounts it, once, and
// says nothing about a file without a match; a file named in the call still says there is none.
func TestFindOnceAndQuiet(t *testing.T) {
	cs := memSession(t, tempRoot(t, map[string]string{
		"entry.tenn": "element Lib --from-file \"lib.tenn\"\n",
		"lib.tenn":   "entry \"entry.tenn\"\nelement Kit {\n\titem Button { width 10; height 10 }\n}\n",
		"other.tenn": "element Other {\n\titem Nothing { width 1; height 1 }\n}\n",
	}))
	got := mustOK(t, cs, "get_state", map[string]any{"find": "Button"})
	if strings.Count(got, "Button") != 1 {
		t.Fatalf("Button listed %d times:\n%s", strings.Count(got, "Button"), got)
	}
	if strings.Contains(got, "other.tenn") || strings.Contains(got, "nothing named") {
		t.Fatalf("a file without a match is in the answer:\n%s", got)
	}
	if one := mustOK(t, cs, "get_state", map[string]any{"file": "other.tenn", "find": "Button"}); !strings.Contains(one, "nothing named") {
		t.Fatalf("a named file without a match says nothing:\n%s", one)
	}
}

// TestFindOnceAcrossDocuments: a library mounted by two documents is reached through both, and each
// node is listed once: their trees parse the same file separately, so only the position matches.
func TestFindOnceAcrossDocuments(t *testing.T) {
	cs := memSession(t, tempRoot(t, map[string]string{
		"a.tenn":   "element Lib --from-file \"lib.tenn\"\nelement A {\n\tuse Lib.Button as b\n}\n",
		"b.tenn":   "element Lib --from-file \"lib.tenn\"\nelement B {\n\tuse Lib.Button as b\n}\n",
		"lib.tenn": "element Kit {\n\titem Button { width 10; height 10 }\n}\n",
	}))
	got := mustOK(t, cs, "get_state", map[string]any{"find": "Button"})
	if n := strings.Count(got, "Button"); n != 1 {
		t.Fatalf("Button listed %d times:\n%s", n, got)
	}
}

// TestCallLog: a call is logged where TENNIARB_MCP_LOG says, with what it was asked and whether it
// worked, and "off" writes nothing.
func TestCallLog(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "calls.log")
	t.Setenv("TENNIARB_MCP_LOG", log)
	cs := memSession(t, tempRoot(t, map[string]string{"s.tenn": "element S { item A { width 10; height 10 } }\n"}))
	mustOK(t, cs, "get_state", map[string]any{"file": "s.tenn"})
	b, err := os.ReadFile(log)
	line := string(b)
	if err != nil || !strings.Contains(line, `"tool":"get_state"`) || !strings.Contains(line, `"ok":true`) ||
		!strings.Contains(line, `"args":"{\"file\":\"s.tenn\"}"`) {
		t.Fatalf("log: %q %v", b, err)
	}
	mustFail(t, cs, "read", map[string]any{"file": "no.tenn"}, "no.tenn")
	b, _ = os.ReadFile(log)
	if !strings.Contains(string(b), `"tool":"read"`) || !strings.Contains(string(b), `"ok":false`) {
		t.Fatalf("a failed call is not logged: %q", b)
	}
	// An edit carries a whole block; only its start belongs in the line.
	body := strings.Repeat("item A { width 10; height 10 }\n", 200)
	mustOK(t, cs, "edit", map[string]any{"file": "s.tenn", "edits": []map[string]any{{"find": "element S {", "replace": "element S {\n" + body}}})
	b, _ = os.ReadFile(log)
	last := strings.TrimSpace(strings.Split(strings.TrimSpace(string(b)), "\n")[2])
	var stat callStat
	if err := json.Unmarshal([]byte(last), &stat); err != nil {
		t.Fatalf("line: %q %v", last, err)
	}
	if stat.In <= maxCallLogArgs || len(stat.Args) > maxCallLogArgs+4 {
		t.Errorf("edit logged %d args of %d", len(stat.Args), stat.In)
	}
	t.Setenv("TENNIARB_MCP_LOG", "off")
	mustOK(t, cs, "get_state", map[string]any{})
	if b, _ := os.ReadFile(log); strings.Count(string(b), "get_state") != 1 {
		t.Fatalf("off wrote a line: %q", b)
	}
}

// TestEditEmptyFile: a file of zero length takes the body in one edit, as a new one does.
func TestEditEmptyFile(t *testing.T) {
	root := tempRoot(t, map[string]string{"e.tenn": ""})
	cs := memSession(t, root)
	if got := mustOK(t, cs, "edit", map[string]any{"file": "e.tenn", "edits": []map[string]any{{"find": "", "replace": "element E { item A { width 1; height 1 } }\n"}}}); !strings.Contains(got, "written") {
		t.Fatalf("edit of an empty file: %s", got)
	}
	if got := mustOK(t, cs, "read", map[string]any{"file": "e.tenn"}); !strings.Contains(got, "element E") {
		t.Fatalf("file after the edit: %s", got)
	}
}

// TestFindCapped: a search across the workspace stops at maxFoundBytes with a hint, not at the
// workspace listing limit.
func TestFindCapped(t *testing.T) {
	files := map[string]string{}
	for i := 0; i < 6; i++ {
		var b strings.Builder
		b.WriteString("element Board {\n")
		for j := 0; j < 40; j++ {
			fmt.Fprintf(&b, "\titem Button%d { width 10; height 10 }\n", j)
		}
		b.WriteString("}\n")
		files[fmt.Sprintf("b%d.tenn", i)] = b.String()
	}
	cs := memSession(t, tempRoot(t, files))
	got := mustOK(t, cs, "get_state", map[string]any{"find": "Button"})
	if len(got) > maxFoundBytes+300 {
		t.Fatalf("find returned %d bytes", len(got))
	}
	if !strings.Contains(got, "make the word longer") {
		t.Fatalf("no hint on a cut search:\n%s", got[len(got)-120:])
	}
}
