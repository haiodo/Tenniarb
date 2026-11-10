//go:build darwin || linux || windows

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A classic document opens with the text of the Swift printer, not with the bytes of the file: a
// position reads to a hundredth of a point, so the offsets of a text edit are the ones of the window.
// An edit addressed by the file instead lands elsewhere and the text stops parsing .
func TestClassicWindowText(t *testing.T) {
	data, err := os.ReadFile("../../web/fixtures/Example.tenn")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "Example.tenn")
	sh := &Shell{}
	v := newDoc(t, sh, path, string(data))
	if !strings.Contains(v.text, "-498.55 211.0") {
		t.Fatalf("the window does not hold the printer's positions:\n%s", v.text)
	}
	item := strings.Index(v.text, `item "Big yellow item"`)
	if item < 0 {
		t.Fatal("no \"Big yellow item\" in the text of the window")
	}
	rel := strings.Index(v.text[item:], "font-size 30")
	if rel < 0 {
		t.Fatal("no font-size 30 in \"Big yellow item\"")
	}
	at := item + rel + len("font-size ")
	// The same characters of the file sit at another offset: this is why the test of the window reads
	// its text instead of the file.
	if fileAt := strings.Index(string(data), "font-size 30"); fileAt == at {
		t.Fatalf("the file and the window agree on offset %d, so nothing is guarded here", at)
	}
	if _, _, err := sh.ApplyEdit(path, 0, TextEdit{at, at + 2, "24"}); err != nil {
		t.Fatalf("an edit at the offset of the window: %v", err)
	}
	if !strings.Contains(v.text, "font-size 24") {
		t.Fatalf("the edit did not reach the text:\n%s", v.text)
	}
	if !v.isDirty() {
		t.Fatal("the agent edit is not shown as unsaved")
	}
	if b, err := os.ReadFile(path); err != nil || string(b) != string(data) {
		t.Fatalf("the agent edit reached the disk before its autosave: %v", err)
	}
}
