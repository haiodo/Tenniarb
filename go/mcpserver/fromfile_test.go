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
