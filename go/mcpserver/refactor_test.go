package mcpserver

import (
	"strings"
	"testing"

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
