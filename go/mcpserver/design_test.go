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
