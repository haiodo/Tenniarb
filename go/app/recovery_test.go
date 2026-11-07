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

func TestRecoverySkipsIncompleteNewest(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 5, 23, 30, 0, 0, time.UTC)
	want, err := WriteRecovery(dir, map[string]string{"/w/a.tenn": "item A"}, now)
	if err != nil {
		t.Fatal(err)
	}
	incomplete := filepath.Join(recoveryRoot(dir), now.Add(time.Second).Format("20060102-150405"))
	if err := os.MkdirAll(incomplete, 0o755); err != nil {
		t.Fatal(err)
	}
	corrupt := filepath.Join(recoveryRoot(dir), now.Add(2*time.Second).Format("20060102-150405"))
	if err := os.MkdirAll(corrupt, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(corrupt, "manifest.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, _, err := LoadRecovery(dir)
	if err != nil || got != want {
		t.Fatalf("load: %q %v, want %q", got, err, want)
	}
}

func TestRecoveryKeepsFailedEntries(t *testing.T) {
	for _, failure := range []string{"unreadable copy", "invalid source"} {
		t.Run(failure, func(t *testing.T) {
			dir := t.TempDir()
			sh := &Shell{}
			first := newDoc(t, sh, filepath.Join(dir, "a.tenn"), "item A { width 10 }\n")
			second := newDoc(t, sh, filepath.Join(dir, "b.tenn"), "item B { width 10 }\n")
			texts := map[string]string{first.state.Doc.Path: "item A { width 20 }\n", second.state.Doc.Path: "item B { width 20 }\n"}
			folder, err := WriteRecovery(dir, texts, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			_, manifest, err := LoadRecovery(dir)
			if err != nil {
				t.Fatal(err)
			}
			if failure == "unreadable copy" {
				if err := os.Remove(filepath.Join(folder, manifest.Entries[1].File)); err != nil {
					t.Fatal(err)
				}
			} else {
				second.session = nil
				second.text = texts[second.state.Doc.Path]
				second.state.Doc = ParseText("element {")
				second.state.Doc.Path = filepath.Join(dir, "b.tenn")
			}
			if err := sh.restoreRecovery(folder, manifest); err == nil {
				t.Fatal("recovery error was ignored")
			}
			if _, err := os.Stat(folder); err != nil {
				t.Fatalf("failed recovery was deleted: %v", err)
			}
			if first.text != texts[first.state.Doc.Path] {
				t.Fatal("successful entry was not recovered")
			}
		})
	}
}

func TestRecoveryDropsSuccessfulRun(t *testing.T) {
	dir := t.TempDir()
	sh := &Shell{}
	v := newDoc(t, sh, filepath.Join(dir, "a.tenn"), "item A { width 10 }\n")
	before := v.text
	want := "item A { width 20 }\n"
	folder, err := WriteRecovery(dir, map[string]string{v.state.Doc.Path: want}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	_, manifest, err := LoadRecovery(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := sh.restoreRecovery(folder, manifest); err != nil {
		t.Fatal(err)
	}
	if v.text != want || !v.isDirty() {
		t.Fatal("recovery did not leave an unsaved draft")
	}
	if _, err := os.Stat(folder); !os.IsNotExist(err) {
		t.Fatalf("successful recovery remains: %v", err)
	}
	v.session.Undo()
	if v.text != before || v.isDirty() {
		t.Fatal("undo did not restore the original document")
	}
}
