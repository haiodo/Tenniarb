package mcpserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const designDoc = "element S {\n" +
	"\titem A { pos 0 0; width 40; height 20; color #ffffff }\n" +
	"\titem B { pos 60 0; width 40; height 20 }\n}\n"

// TestDesign: set, move, insert, delete and replace_all by node paths, one call, one change.
func TestDesign(t *testing.T) {
	root := tempRoot(t, map[string]string{"s.tenn": designDoc})
	cs := memSession(t, root)
	out := mustOK(t, cs, "design", map[string]any{
		"file": "s.tenn",
		"ops": []map[string]any{
			{"op": "set", "node": "S.A", "name": "corner-radius", "value": "6"},
			{"op": "move", "node": "S.A", "dx": 10, "dy": -4},
			{"op": "insert", "parent": "S", "text": "item C { width 10; height 10 }"},
			{"op": "delete", "node": "S.B"},
			{"op": "replace_all", "find": "#ffffff", "replace": "#eeeeee"},
		},
	})
	for _, want := range []string{"1. set", "4. delete", "5. replace_all"} {
		if !strings.Contains(out, want) {
			t.Errorf("result lacks %q:\n%s", want, out)
		}
	}
	b, _ := os.ReadFile(filepath.Join(root, "s.tenn"))
	got := string(b)
	for _, want := range []string{"corner-radius 6", "color #eeeeee", "item C {"} {
		if !strings.Contains(got, want) {
			t.Errorf("file lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "item B {") {
		t.Errorf("B was not deleted:\n%s", got)
	}
}

// TestDesignRefuses: two operations that change the same text, or a name that is not there, change nothing.
func TestDesignRefuses(t *testing.T) {
	root := tempRoot(t, map[string]string{"s.tenn": designDoc})
	cs := memSession(t, root)
	_, out := call(t, cs, "design", map[string]any{
		"file": "s.tenn",
		"ops": []map[string]any{
			{"op": "set", "node": "S.A", "name": "width", "value": "80"},
			{"op": "delete", "node": "S.A"},
		},
	})
	if !strings.Contains(out, "same text") {
		t.Fatalf("overlap not refused: %s", out)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "s.tenn")); string(b) != designDoc {
		t.Fatal("a refused call wrote the file")
	}
}

// TestDesignOtherFile: a node of a mounted file is refused with the file that holds it.
func TestDesignOtherFile(t *testing.T) {
	root := tempRoot(t, map[string]string{
		"main.tenn": "element ui --from-file \"lib.tenn\"\n",
		"lib.tenn":  "item Button { pos 0 0; width 40; height 20 }\n",
	})
	cs := memSession(t, root)
	_, out := call(t, cs, "design", map[string]any{
		"file": "main.tenn",
		"ops":  []map[string]any{{"op": "set", "node": "ui.Button", "name": "width", "value": "80"}},
	})
	if !strings.Contains(out, "lib.tenn") || !strings.Contains(out, "run design on that file") {
		t.Fatalf("a node of a mounted file was not refused with its file: %s", out)
	}
	if got := mustOK(t, cs, "design", map[string]any{
		"file": "lib.tenn",
		"ops":  []map[string]any{{"op": "set", "node": "Button", "name": "width", "value": "80"}},
	}); !strings.Contains(got, "width of Button") {
		t.Fatalf("design on the mounted file itself: %s", got)
	}
}

// TestDesignUseBlockProp: a property in the block of a use is replaced where it stands, not written a
// second time under it.
func TestDesignUseBlockProp(t *testing.T) {
	root := tempRoot(t, map[string]string{
		"s.tenn": "element S {\n\titem A { width 40; height 20 }\n\tuse A as a { pos 10 10; width 50 }\n}\n",
	})
	cs := memSession(t, root)
	mustOK(t, cs, "design", map[string]any{"file": "s.tenn", "ops": []map[string]any{
		{"op": "move", "node": "S.a", "dx": 5, "dy": -5},
	}})
	got := fileText(t, root, "s.tenn")
	if n := strings.Count(got, "pos "); n != 1 || !strings.Contains(got, "pos 15 5") {
		t.Fatalf("%d pos lines in the block of the use:\n%s", n, got)
	}
	if got := mustOK(t, cs, "design", map[string]any{"file": "s.tenn", "ops": []map[string]any{
		{"op": "set", "node": "S.a", "name": "width", "value": "60"},
	}}); !strings.Contains(got, "width of a") {
		t.Fatalf("set in the block: %s", got)
	}
	if txt := fileText(t, root, "s.tenn"); strings.Count(txt, "width ") != 2 {
		t.Fatalf("width lines:\n%s", txt)
	}
}

// TestDesignOnPart: design on a file its entry mounts resolves the names the entry defines, as the
// canvas does; loading the file alone would find neither Libraries nor the mount.
func TestDesignOnPart(t *testing.T) {
	root := tempRoot(t, map[string]string{
		"entry.tenn": "element Lib --from-file \"part.tenn\"\n",
		"part.tenn": "entry \"entry.tenn\"\nelement Kit {\n\titem Button { width 40; height 20 }\n}\n" +
			"element Screen {\n\tuse Lib.Kit.Button as b { color #000000 }\n}\n",
	})
	cs := memSession(t, root)
	got := mustOK(t, cs, "design", map[string]any{"file": "part.tenn", "ops": []map[string]any{
		{"op": "set", "node": "Screen.b", "name": "color", "value": "#ff0000"},
	}})
	if !strings.Contains(got, "color of b") {
		t.Fatalf("design on a part: %s", got)
	}
	if txt := fileText(t, root, "part.tenn"); !strings.Contains(txt, "color #ff0000") {
		t.Fatalf("part.tenn:\n%s", txt)
	}
}

// TestDesignSetIndex: a property written twice (shadow here) is the last one unless index names another;
// an index the node does not have fails with the count, and nothing is written.
func TestDesignSetIndex(t *testing.T) {
	const doc = "element S {\n\titem A { width 40; height 20; shadow 1 1 2 #000000; shadow 4 4 8 #111111 }\n}\n"
	root := tempRoot(t, map[string]string{"s.tenn": doc})
	cs := memSession(t, root)
	out := mustOK(t, cs, "design", map[string]any{"file": "s.tenn", "ops": []map[string]any{
		{"op": "set", "node": "S.A", "name": "shadow", "value": "2 2 3 #ff0000", "index": 0},
	}})
	if !strings.Contains(out, "shadow of A (index 0 of 2)") {
		t.Fatalf("the answer does not name the index: %s", out)
	}
	got := fileText(t, root, "s.tenn")
	if !strings.Contains(got, "shadow 2 2 3 #ff0000; shadow 4 4 8 #111111") {
		t.Fatalf("the first shadow was not the one changed:\n%s", got)
	}
	out = mustOK(t, cs, "design", map[string]any{"file": "s.tenn", "ops": []map[string]any{
		{"op": "set", "node": "S.A", "name": "shadow", "value": "9 9 9"},
	}})
	if !strings.Contains(out, "(index 1 of 2)") {
		t.Fatalf("without index the last shadow must be the one: %s", out)
	}
	if got := fileText(t, root, "s.tenn"); !strings.Contains(got, "shadow 2 2 3 #ff0000; shadow 9 9 9") {
		t.Fatalf("the last shadow was not the one changed:\n%s", got)
	}
	before := fileText(t, root, "s.tenn")
	mustFail(t, cs, "design", map[string]any{"file": "s.tenn", "ops": []map[string]any{
		{"op": "set", "node": "S.A", "name": "shadow", "value": "0 0 0", "index": 5},
	}}, "has 2 properties named shadow: no index 5")
	if fileText(t, root, "s.tenn") != before {
		t.Fatal("a refused index wrote the file")
	}
	// an empty value removes the shadow at that index, not the last one
	out = mustOK(t, cs, "design", map[string]any{"file": "s.tenn", "ops": []map[string]any{
		{"op": "set", "node": "S.A", "name": "shadow", "value": "", "index": 0},
	}})
	if !strings.Contains(out, "(index 0 of 2) removed") {
		t.Fatalf("the removal does not name the index: %s", out)
	}
	if got := fileText(t, root, "s.tenn"); strings.Contains(got, "shadow 2 2 3") || !strings.Contains(got, "shadow 9 9 9") {
		t.Fatalf("the removal took the wrong shadow:\n%s", got)
	}
}

func TestDesignUsesUnsavedLibraries(t *testing.T) {
	dir := serviceDir(t)
	startService(t, dir, "v1", nil, 0)
	root := tempRoot(t, map[string]string{"lib.tenn": "element Parts { item Old {} }\n", "main.tenn": "element lib --from-file lib.tenn\nelement Screen { use lib.Parts.Old as x {} }\n"})
	cs := session(t, dir, root, "v1", nil)
	fake, _ := connectFake(t, dir)
	if err := fake.Open(filepath.Join(root, "lib.tenn"), "element Parts { item New {} }\n"); err != nil {
		t.Fatal(err)
	}
	if err := fake.Open(filepath.Join(root, "main.tenn"), "element lib --from-file lib.tenn\nelement Screen { use lib.Parts.New as x {} }\n"); err != nil {
		t.Fatal(err)
	}
	mustOK(t, cs, "validate", map[string]any{"file": "main.tenn"})
	mustOK(t, cs, "design", map[string]any{"file": "main.tenn", "ops": []map[string]any{{"op": "set", "node": "Screen.x", "name": "width", "value": "60"}}})
	if got, _ := fake.TextOf(filepath.Join(root, "main.tenn")); !strings.Contains(got, "width 60") {
		t.Fatalf("design not applied: %s", got)
	}
}

func TestDesignRefusesStaleSnapshot(t *testing.T) {
	dir := serviceDir(t)
	startService(t, dir, "v1", nil, 0)
	root := tempRoot(t, map[string]string{"a.tenn": liveDisk})
	cs := session(t, dir, root, "v1", nil)
	fake, _ := connectFake(t, dir)
	abs := filepath.Join(root, "a.tenn")
	if err := fake.Open(abs, liveDisk); err != nil {
		t.Fatal(err)
	}
	user := strings.Replace(liveDisk, "}\n", "    item user\n}\n", 1)
	reads := 0
	fake.AfterText = func() {
		reads++
		if reads == 1 {
			if err := fake.Type(abs, user); err != nil {
				t.Error(err)
			}
		}
	}
	mustFail(t, cs, "design", map[string]any{"file": "a.tenn", "ops": []map[string]any{{"op": "insert", "parent": "A", "text": "item agent"}}}, "document changed")
	if got, _ := fake.TextOf(abs); got != user {
		t.Fatalf("user edit lost: %s", got)
	}
}

// TestDesignInsertThenDelete: an insert leaves the closing brace of the parent on its own line, so the
// delete of a node in that block does not take the brace with the line.
func TestDesignInsertThenDelete(t *testing.T) {
	root := tempRoot(t, map[string]string{"s.tenn": designDoc})
	cs := memSession(t, root)
	mustOK(t, cs, "design", map[string]any{"file": "s.tenn", "ops": []map[string]any{
		{"op": "insert", "parent": "S", "text": "item C { width 10; height 10 }"},
	}})
	if got := fileText(t, root, "s.tenn"); strings.Contains(got, "}}") {
		t.Fatalf("the insert glued the brace of S to the new node:\n%s", got)
	}
	mustOK(t, cs, "design", map[string]any{"file": "s.tenn", "ops": []map[string]any{
		{"op": "delete", "node": "S.B"},
	}})
	if got := mustOK(t, cs, "design", map[string]any{"file": "s.tenn", "ops": []map[string]any{
		{"op": "delete", "node": "S.C"},
	}}); !strings.Contains(got, "C deleted") {
		t.Fatalf("delete of the last node of the block: %s", got)
	}
	got := fileText(t, root, "s.tenn")
	if strings.Contains(got, "item B") || strings.Contains(got, "item C") || !strings.HasSuffix(got, "}\n") {
		t.Fatalf("the block lost a line or its brace:\n%s", got)
	}
	mustOK(t, cs, "validate", map[string]any{"file": "s.tenn"})
}

// TestDesignInsertAtRoot: an empty parent is the file root, which has no block but its text.
func TestDesignInsertAtRoot(t *testing.T) {
	root := tempRoot(t, map[string]string{"s.tenn": designDoc})
	cs := memSession(t, root)
	if out := mustOK(t, cs, "design", map[string]any{"file": "s.tenn", "ops": []map[string]any{
		{"op": "insert", "parent": "", "text": "element T {\n\titem x { width 1 }\n}"},
	}}); !strings.Contains(out, "into s.tenn") {
		t.Fatalf("insert at the root: %s", out)
	}
	got := fileText(t, root, "s.tenn")
	if !strings.HasSuffix(got, "}\n") || strings.Contains(got, "}\n\n") {
		t.Fatalf("the root insert left the end of the file wrong:\n%s", got)
	}
	mustOK(t, cs, "design", map[string]any{"file": "s.tenn", "ops": []map[string]any{
		{"op": "delete", "node": "T"},
	}})
	if got := fileText(t, root, "s.tenn"); strings.Contains(got, "element T") {
		t.Fatalf("T was not deleted:\n%s", got)
	}
	mustOK(t, cs, "validate", map[string]any{"file": "s.tenn"})
}

// TestDesignSetThenRemove: a property added at the end of a block does not glue the brace either, or
// removing it deletes the brace and the file stops parsing.
func TestDesignSetThenRemove(t *testing.T) {
	root := tempRoot(t, map[string]string{"s.tenn": "element S {\n\titem A {\n\t\twidth 40\n\t}\n}\n"})
	cs := memSession(t, root)
	mustOK(t, cs, "design", map[string]any{"file": "s.tenn", "ops": []map[string]any{
		{"op": "set", "node": "S.A", "name": "corner-radius", "value": "6"},
	}})
	mustOK(t, cs, "design", map[string]any{"file": "s.tenn", "ops": []map[string]any{
		{"op": "set", "node": "S.A", "name": "corner-radius", "value": ""},
	}})
	got := fileText(t, root, "s.tenn")
	if strings.Contains(got, "corner-radius") || !strings.Contains(got, "width 40\n\t}\n}") {
		t.Fatalf("the block of A is not whole:\n%s", got)
	}
	mustOK(t, cs, "validate", map[string]any{"file": "s.tenn"})
}

// TestDesignReplaceAllInNode: a node bounds the search, so the same text in another frame stays.
func TestDesignReplaceAllInNode(t *testing.T) {
	doc := "element S {\n\titem A { title Card }\n\titem B { title Card }\n}\n"
	root := tempRoot(t, map[string]string{"s.tenn": doc})
	cs := memSession(t, root)
	out := mustOK(t, cs, "design", map[string]any{"file": "s.tenn", "ops": []map[string]any{
		{"op": "replace_all", "node": "S.A", "find": "title Card", "replace": "title Board"},
	}})
	if !strings.Contains(out, "1 places in A") {
		t.Fatalf("replace_all does not name the node it searched: %s", out)
	}
	got := fileText(t, root, "s.tenn")
	if !strings.Contains(got, "item A { title Board }") || !strings.Contains(got, "item B { title Card }") {
		t.Fatalf("replace_all left its node:\n%s", got)
	}
	mustFail(t, cs, "design", map[string]any{"file": "s.tenn", "ops": []map[string]any{
		{"op": "replace_all", "node": "S.A", "find": "item B", "replace": "item Z"},
	}}, "not found in A")
	if fileText(t, root, "s.tenn") != got {
		t.Fatal("a refused call wrote the file")
	}
}

// TestDesignInsertIntoUse: the block of a use holds overrides and the nodes of the default slot, so a
// use is an insert parent like an item.
func TestDesignInsertIntoUse(t *testing.T) {
	root := tempRoot(t, map[string]string{"s.tenn": "element Kit {\n\titem Row { width 40; slot { } }\n}\n" +
		"element Screen {\n\tuse Kit.Row as r {\n\t\twidth 50\n\t}\n\tuse Kit.Row as bare\n}\n"})
	cs := memSession(t, root)
	if out := mustOK(t, cs, "design", map[string]any{"file": "s.tenn", "ops": []map[string]any{
		{"op": "insert", "parent": "Screen.r", "text": "item Extra { width 5 }"},
	}}); !strings.Contains(out, "into r") {
		t.Fatalf("insert into a use: %s", out)
	}
	got := fileText(t, root, "s.tenn")
	if !strings.Contains(got, "\t\twidth 50\n\t\titem Extra { width 5 }\n\t}") {
		t.Fatalf("the node went elsewhere:\n%s", got)
	}
	mustOK(t, cs, "validate", map[string]any{"file": "s.tenn"})
	mustFail(t, cs, "design", map[string]any{"file": "s.tenn", "ops": []map[string]any{
		{"op": "insert", "parent": "Screen.bare", "text": "item Extra { width 5 }"},
	}}, "write { } after it first")
}
