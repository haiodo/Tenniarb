package mcpserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFromFileMount: get_state and validate see a mounted library; an edit goes to its file.
func TestFromFileMount(t *testing.T) {
	root := tempRoot(t, map[string]string{
		"main.tenn": "element \"kit\" --from-file \"lib.tenn\"\nelement \"S\" {\n    use kit.K.B as b\n}\n",
		"lib.tenn":  "element \"K\" {\n    item \"B\" { title \"b\"; width 10; height 10 }\n}\n",
	})
	cs := memSession(t, root)
	if got := mustOK(t, cs, "get_state", map[string]any{"file": "main.tenn", "path": "kit"}); !strings.Contains(got, "K") {
		t.Errorf("state does not list the mounted content: %s", got)
	}
	if got := mustOK(t, cs, "validate", map[string]any{"file": "main.tenn"}); !strings.Contains(got, "0 errors") {
		t.Fatalf("validate: %s", got)
	}
	mustOK(t, cs, "edit", withFile(edits(`title "b"`, `title "c"`), "lib.tenn"))
	main, _ := os.ReadFile(filepath.Join(root, "main.tenn"))
	lib, _ := os.ReadFile(filepath.Join(root, "lib.tenn"))
	if strings.Contains(string(main), "title") || !strings.Contains(string(lib), `title "c"`) {
		t.Errorf("the edit must land in lib.tenn only:\nmain: %s\nlib: %s", main, lib)
	}
	// The cached tree of main.tenn must notice the changed library.
	mustOK(t, cs, "edit", withFile(edits(`element "K"`, `element "K2"`), "lib.tenn"))
	if got := mustOK(t, cs, "validate", map[string]any{"file": "main.tenn"}); strings.Contains(got, "0 errors") {
		t.Errorf("a change of a mounted file was not seen: %s", got)
	}
}

// TestEntryContext: a part file resolves the library its entry mounts, in validate, get_state and edit.
func TestEntryContext(t *testing.T) {
	root := tempRoot(t, map[string]string{
		"design.tenn":    "element \"ui\" --from-file \"ui.tenn\"\nelement \"p\" --from-file \"apps/part.tenn\"\n",
		"ui.tenn":        "item Button { title \"b\"; width 10; height 10 }\n",
		"apps/part.tenn": "element \"S\" {\n    use ui.Button as b\n}\n",
	})
	cs := memSession(t, root)
	got := mustOK(t, cs, "validate", map[string]any{"file": "apps/part.tenn"})
	if !strings.Contains(got, "0 errors") || !strings.Contains(got, "apps/part.tenn checked in the context of design.tenn") {
		t.Fatalf("validate: %s", got)
	}
	if got := mustOK(t, cs, "get_state", map[string]any{"file": "apps/part.tenn"}); !strings.Contains(got, "(in design.tenn)") {
		t.Errorf("state: %s", got)
	}
	mustOK(t, cs, "edit", withFile(edits(`use ui.Button as b`, `use ui.Button as c`), "apps/part.tenn"))
	part, _ := os.ReadFile(filepath.Join(root, "apps/part.tenn"))
	if !strings.Contains(string(part), "as c") {
		t.Errorf("the edit did not land: %s", part)
	}
}
