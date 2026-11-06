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
