package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestRecoveryRoundTrip: a stopped run keeps the unsaved texts, and the next start finds them.
func TestRecoveryRoundTrip(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, 11, 5, 23, 30, 0, 0, time.UTC)
	texts := map[string]string{
		"/w/design.tenn":    "element A {}\n",
		"/w/lib/forms.tenn": "item Input { width 10 }\n",
	}
	folder, err := WriteRecovery(dir, texts, at)
	if err != nil || folder == "" {
		t.Fatalf("write: %q %v", folder, err)
	}
	got, m, err := LoadRecovery(dir)
	if err != nil || got != folder || len(m.Entries) != 2 {
		t.Fatalf("load: %q %+v %v", got, m, err)
	}
	for _, e := range m.Entries {
		text, err := RecoveryText(got, e.File)
		if err != nil || text != texts[e.Path] {
			t.Errorf("%s: %q %v, want %q", e.Path, text, err, texts[e.Path])
		}
	}
	if err := DropRecovery(got); err != nil {
		t.Fatal(err)
	}
	if again, _, _ := LoadRecovery(dir); again != "" {
		t.Errorf("a dropped folder is still offered: %q", again)
	}
}

// TestRecoveryNothing: a run with nothing unsaved leaves no folder.
func TestRecoveryNothing(t *testing.T) {
	dir := t.TempDir()
	if folder, err := WriteRecovery(dir, nil, time.Now()); folder != "" || err != nil {
		t.Fatalf("empty run: %q %v", folder, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "recovery")); !os.IsNotExist(err) {
		t.Error("the recovery folder was made for nothing")
	}
}
