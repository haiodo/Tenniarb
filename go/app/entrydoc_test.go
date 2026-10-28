package app

import (
	"os"
	"path/filepath"
	"testing"
)

// A part file opened on its own loads in the context of the entry that mounts it.
func TestDocLoadsInEntryContext(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil { // the workspace root
		t.Fatal(err)
	}
	files := map[string]string{
		"design.tenn":    "element ui --from-file ui.tenn\nelement p --from-file \"apps/part.tenn\"\n",
		"ui.tenn":        "item Button { width 40; height 20 }\n",
		"apps/part.tenn": "element \"Screen\" { use ui.Button as b }\n",
	}
	for name, text := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	v := newDoc(t, &Shell{}, filepath.Join(dir, "apps/part.tenn"), files["apps/part.tenn"])
	if v.sceneErr != "" || filepath.Base(v.entry) != "design.tenn" {
		t.Errorf("entry %q, errors %q", v.entry, v.sceneErr)
	}
	v = newDoc(t, &Shell{}, filepath.Join(dir, "ui.tenn"), files["ui.tenn"])
	if v.entry != "" {
		t.Errorf("a file that loads alone must keep its own context, got %q", v.entry)
	}
}
