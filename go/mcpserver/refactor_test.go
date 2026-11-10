package mcpserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/servicewire"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestRefactorRename: the name changes in its own file, and a use of it in the file that mounts it
// follows, both kept as one change.
func TestRefactorRename(t *testing.T) {
	root := tempRoot(t, map[string]string{
		"kit.tenn":  "element Parts {\n\titem Button { width 40 }\n}\n",
		"main.tenn": "element kit --from-file kit.tenn\nelement Screen {\n\tuse kit.Parts.Button as b\n}\n",
	})
	cs := memSession(t, root)
	got := mustOK(t, cs, "refactor", map[string]any{"file": "main.tenn", "node": "kit.Parts.Button", "rename": "PrimaryButton"})
	if !strings.Contains(got, "2 places") || !strings.Contains(got, "2 files") {
		t.Fatalf("answer:\n%s", got)
	}
	if kit := fileText(t, root, "kit.tenn"); !strings.Contains(kit, "item PrimaryButton") {
		t.Errorf("kit.tenn:\n%s", kit)
	}
	main := fileText(t, root, "main.tenn")
	if !strings.Contains(main, "use kit.Parts.PrimaryButton as b") {
		t.Errorf("main.tenn:\n%s", main)
	}
}

// TestRefactorRenameAtAndReplace: a path of at is relative to the instance, and only its segment
// changes, as the rest of it is written from the instance root.
func TestRefactorRenameAtAndReplace(t *testing.T) {
	root := tempRoot(t, map[string]string{
		"m.tenn": "element S {\n\titem Card {\n\t\titem title { }\n\t}\n\tuse Card as c {\n\t\tat title { color red }\n\t}\n}\n",
	})
	cs := memSession(t, root)
	mustOK(t, cs, "refactor", map[string]any{"file": "m.tenn", "node": "S.Card.title", "rename": "heading"})
	got := fileText(t, root, "m.tenn")
	if !strings.Contains(got, "at heading { color red }") || !strings.Contains(got, "item heading { }") {
		t.Errorf("m.tenn:\n%s", got)
	}
}

// TestRefactorRenameTaken: a name that another sibling already has is refused, and nothing changes.
func TestRefactorRenameTaken(t *testing.T) {
	root := tempRoot(t, map[string]string{"m.tenn": "element S {\n\titem A { }\n\titem B { }\n}\n"})
	cs := memSession(t, root)
	mustFail(t, cs, "refactor", map[string]any{"file": "m.tenn", "node": "S.A", "rename": "B"}, "already")
	if got := fileText(t, root, "m.tenn"); !strings.Contains(got, "item A { }") {
		t.Errorf("the file changed:\n%s", got)
	}
}

// TestRefactorMoveToRoot: "." is the file root, which has no block but its text; the node lands flush
// left at the end of the file and a use of it is written again.
func TestRefactorMoveToRoot(t *testing.T) {
	root := tempRoot(t, map[string]string{
		"m.tenn": "element S {\n\titem A {\n\t\titem B { width 10 }\n\t}\n}\n",
	})
	cs := memSession(t, root)
	if got := mustOK(t, cs, "refactor", map[string]any{"file": "m.tenn", "node": "S.A.B", "parent": "."}); !strings.Contains(got, "moved to .") {
		t.Fatalf("answer:\n%s", got)
	}
	got := fileText(t, root, "m.tenn")
	if !strings.HasSuffix(got, "\nitem B { width 10 }\n") {
		t.Fatalf("the node did not land at the root:\n%s", got)
	}
	mustOK(t, cs, "validate", map[string]any{"file": "m.tenn"})
}

// TestRefactorMove: the node leaves its parent, takes the indentation of the new one, and a use of it
// in another file is written from the entry document again.
func TestRefactorMove(t *testing.T) {
	root := tempRoot(t, map[string]string{
		"lib.tenn":  "element One {\n\titem Thing { width 10 }\n}\nelement Two {\n\titem Box { }\n}\n",
		"main.tenn": "element lib --from-file lib.tenn\nelement Screen {\n\tuse lib.One.Thing as t\n}\n",
	})
	cs := memSession(t, root)
	got := mustOK(t, cs, "refactor", map[string]any{"file": "main.tenn", "node": "lib.One.Thing", "parent": "lib.Two.Box"})
	if !strings.Contains(got, "moved to") {
		t.Fatalf("answer:\n%s", got)
	}
	lib := fileText(t, root, "lib.tenn")
	if strings.Contains(lib, "element One {\n\titem Thing") || !strings.Contains(lib, "item Box {\n\t\titem Thing { width 10 }\n\t}") {
		t.Errorf("lib.tenn:\n%s", lib)
	}
	if main := fileText(t, root, "main.tenn"); !strings.Contains(main, "use lib.Two.Box.Thing as t") {
		t.Errorf("main.tenn:\n%s", main)
	}
}

// TestRefactorMoveRefusesAt: a path of at names the node relative to the instance, so a move cannot
// write it again; the call says so and changes nothing.
func TestRefactorMoveRefusesAt(t *testing.T) {
	src := "element S {\n\titem Card {\n\t\titem title { }\n\t}\n\titem Other { }\n\tuse Card as c {\n\t\tat title { color red }\n\t}\n}\n"
	root := tempRoot(t, map[string]string{"m.tenn": src})
	cs := memSession(t, root)
	mustFail(t, cs, "refactor", map[string]any{"file": "m.tenn", "node": "S.Card.title", "parent": "S.Other"}, "relative to the instance")
	if got := fileText(t, root, "m.tenn"); got != src {
		t.Errorf("the file changed:\n%s", got)
	}
}

// TestRefactorMoveRefusesInside: a node cannot move into itself.
func TestRefactorMoveRefusesInside(t *testing.T) {
	root := tempRoot(t, map[string]string{"m.tenn": "element S {\n\titem A {\n\t\titem B { }\n\t}\n}\n"})
	cs := memSession(t, root)
	mustFail(t, cs, "refactor", map[string]any{"file": "m.tenn", "node": "S.A", "parent": "S.A.B"}, "inside the node")
}

// TestRenderElements: one call draws several elements, each as its own image and text.
func TestRenderElements(t *testing.T) {
	if _, err := newRasterizer(); err != nil {
		t.Skip("no rasterizer on this platform")
	}
	cs := memSession(t, tempRoot(t, map[string]string{
		"s.tenn": "element A { item x { width 10; height 10; color #ff0000 } }\n" +
			"element B { item y { width 10; height 10; color #00ff00 } }\n",
	}))
	res, text := call(t, cs, "render", map[string]any{"file": "s.tenn", "elements": []string{"A", "B"}})
	images := 0
	for _, c := range res.Content {
		if _, ok := c.(*mcp.ImageContent); ok {
			images++
		}
	}
	if res.IsError || images != 2 {
		t.Fatalf("%d images, error %v: %s", images, res.IsError, text)
	}
	if !strings.Contains(text, `"A"`) || !strings.Contains(text, `"B"`) {
		t.Fatalf("answer:\n%s", text)
	}
}

// TestEditRendersAfterChange: an edit that names an element answers with its picture, so the check of
// the result is the same call.
func TestEditRendersAfterChange(t *testing.T) {
	if _, err := newRasterizer(); err != nil {
		t.Skip("no rasterizer on this platform")
	}
	cs := memSession(t, tempRoot(t, map[string]string{
		"s.tenn": "element A { item x { width 10; height 10 } }\n",
	}))
	res, text := call(t, cs, "edit", map[string]any{
		"file": "s.tenn", "edits": []map[string]any{{"find": "width 10", "replace": "width 30"}}, "render": "A",
	})
	if res.IsError || !strings.Contains(text, "written") {
		t.Fatalf("edit: %s", text)
	}
	seen := false
	for _, c := range res.Content {
		if _, ok := c.(*mcp.ImageContent); ok {
			seen = true
		}
	}
	if !seen {
		t.Fatalf("no image:\n%s", text)
	}
}

// TestRefactorRenameQuotes: a name with a space is written in quotes in every path, and the segments
// before it keep their own form.
func TestRefactorRenameQuotes(t *testing.T) {
	root := tempRoot(t, map[string]string{
		"kit.tenn":  "element Parts {\n\titem Button { width 40 }\n}\n",
		"main.tenn": "element kit --from-file kit.tenn\nelement Screen {\n\tuse kit.Parts.Button as b\n}\n",
	})
	cs := memSession(t, root)
	mustOK(t, cs, "refactor", map[string]any{"file": "main.tenn", "node": "kit.Parts.Button", "rename": "Primary Button"})
	if got := fileText(t, root, "kit.tenn"); !strings.Contains(got, `item "Primary Button"`) {
		t.Errorf("kit.tenn:\n%s", got)
	}
	if got := fileText(t, root, "main.tenn"); !strings.Contains(got, `use kit.Parts."Primary Button" as b`) {
		t.Errorf("main.tenn:\n%s", got)
	}
}

func TestRefactorRefusesStaleFilesBeforeApplying(t *testing.T) {
	dir := serviceDir(t)
	running := startService(t, dir, "v1", nil, 0)
	main := "element B --from-file b.tenn\n" + liveDisk
	root := tempRoot(t, map[string]string{"a.tenn": main, "b.tenn": liveDisk})
	fake, _ := connectFake(t, dir)
	for _, file := range []string{"a.tenn", "b.tenn"} {
		if err := fake.Open(filepath.Join(root, file), fileText(t, root, file)); err != nil {
			t.Fatal(err)
		}
	}
	s, err := running.sv.Host.Session(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	_, tree, err := s.diagnose("a.tenn")
	if err != nil {
		t.Fatal(err)
	}
	user := strings.Replace(liveDisk, "}\n", "    item user\n}\n", 1)
	if err := fake.Type(filepath.Join(root, "b.tenn"), user); err != nil {
		t.Fatal(err)
	}
	next := map[string]string{"a.tenn": strings.Replace(main, `"x"`, `"renamed"`, 1), "b.tenn": strings.Replace(liveDisk, `"x"`, `"renamed"`, 1)}
	if err := s.applyTexts(tree, next); err == nil || !strings.Contains(err.Error(), "document changed: b.tenn") {
		t.Fatalf("got %v, want stale b.tenn refusal", err)
	}
	if got, _ := fake.TextOf(filepath.Join(root, "a.tenn")); got != main {
		t.Fatalf("first document changed: %s", got)
	}
	if got, _ := fake.TextOf(filepath.Join(root, "b.tenn")); got != user {
		t.Fatalf("user edit lost: %s", got)
	}
}

func TestRefactorRefusesMovedFilesBeforeWriting(t *testing.T) {
	root := tempRoot(t, map[string]string{"a.tenn": "element B --from-file b.tenn\n" + liveDisk, "b.tenn": liveDisk})
	h := NewHostIn(t.TempDir())
	s, err := h.Session(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	_, tree, err := s.diagnose("a.tenn")
	if err != nil {
		t.Fatal(err)
	}
	abs := filepath.Join(root, "b.tenn")
	p := &provider{}
	h.live.set(p, []servicewire.Doc{{Path: abs, Version: 1}})
	h.live.drop(p, true)
	next := map[string]string{"a.tenn": strings.Replace(fileText(t, root, "a.tenn"), `"x"`, `"renamed"`, 1), "b.tenn": strings.Replace(liveDisk, `"x"`, `"renamed"`, 1)}
	if err := s.applyTexts(tree, next); err != errMoved {
		t.Fatalf("got %v, want errMoved", err)
	}
	if got := fileText(t, root, "a.tenn"); got != "element B --from-file b.tenn\n"+liveDisk {
		t.Fatalf("first file changed: %s", got)
	}
	if got := fileText(t, root, "b.tenn"); got != liveDisk {
		t.Fatalf("moved file changed: %s", got)
	}
	if _, err := s.refactor(refactorIn{File: "b.tenn", Node: "A.x", Rename: "renamed"}); err != errMoved {
		t.Fatalf("refactor got %v, want errMoved", err)
	}
}

func TestRefactorRefusesStaleDiskFilesBeforeWriting(t *testing.T) {
	main := "element B --from-file b.tenn\n" + liveDisk
	root := tempRoot(t, map[string]string{"a.tenn": main, "b.tenn": liveDisk})
	s, err := NewHostIn(t.TempDir()).Session(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	_, tree, err := s.diagnose("a.tenn")
	if err != nil {
		t.Fatal(err)
	}
	user := strings.Replace(liveDisk, `"x"`, `"user"`, 1)
	if err := os.WriteFile(filepath.Join(root, "b.tenn"), []byte(user), 0o644); err != nil {
		t.Fatal(err)
	}
	next := map[string]string{"a.tenn": strings.Replace(main, `"x"`, `"renamed"`, 1), "b.tenn": strings.Replace(liveDisk, `"x"`, `"renamed"`, 1)}
	if err := s.applyTexts(tree, next); err == nil || !strings.Contains(err.Error(), "file changed: b.tenn") {
		t.Fatalf("got %v, want stale b.tenn refusal", err)
	}
	if got := fileText(t, root, "a.tenn"); got != main {
		t.Fatalf("first file changed: %s", got)
	}
	if got := fileText(t, root, "b.tenn"); got != user {
		t.Fatalf("disk edit lost: %s", got)
	}
}
