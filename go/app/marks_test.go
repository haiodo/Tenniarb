//go:build darwin || linux || windows

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/haiodo/tenniarb/go/scene"
)

const marksDoc = `item "Card" {
    pos 0 0
    width 200
    item "Title" { pos 20 90; width 100 }
}
`

// TestNodeAtText finds the node an edit of the text touched: the item that holds the offset, not the
// element around it and not its own property.
func TestNodeAtText(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "marks.tenn"), []byte(marksDoc), 0o644); err != nil {
		t.Fatal(err)
	}
	tree, err := scene.Load(dir, "marks.tenn")
	if err != nil {
		t.Fatal(err)
	}
	file := tree.File("marks.tenn")
	for _, c := range []struct {
		text, want string
	}{
		{"width 200", "Card"},
		{"width 100", "Title"},
	} {
		if n := nodeAtText(file, strings.Index(marksDoc, c.text)); n == nil || n.Key != c.want {
			t.Errorf("the node of %q: %v, want %q", c.text, nName(n), c.want)
		}
	}
	if n := nodeAtText(file, len(marksDoc)); n != nil {
		t.Errorf("an offset after the file names %v", nName(n))
	}
}

func nName(n *scene.Node) string {
	if n == nil {
		return "nothing"
	}
	return n.Key
}

// TestMarksGo shows a mark over a node with no window and reads it back from the QA state: it lasts its
// time and leaves nothing behind.
func TestMarksGo(t *testing.T) {
	v := &Viewer{}
	v.markNode("Card", markSelect, markLabel)
	v.markNode("", markEdit, markLabel) // no node: no mark
	got := v.qaMarks()
	if len(got) != 1 || got[0].Path != "Card" || got[0].Kind != "select" || got[0].Left <= 0 {
		t.Fatalf("marks: %+v", got)
	}
	v.marks.list[0].until = time.Now().Add(-time.Millisecond)
	if got := v.qaMarks(); len(got) != 0 {
		t.Fatalf("a mark outlived its time: %+v", got)
	}
	// the same node of the same kind is one mark, the newest
	v.markNode("Card", markPulse, "")
	v.markNode("Card", markPulse, "")
	if got := v.qaMarks(); len(got) != 1 || got[0].Kind != "pulse" {
		t.Fatalf("marks: %+v", got)
	}
}
