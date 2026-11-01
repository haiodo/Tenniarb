package scene_test

import (
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/scene"
)

// TestJoinedBlocksWarn: two blocks written on one line join into one command, and the check says so.
func TestJoinedBlocksWarn(t *testing.T) {
	tree, err := scene.LoadSource(t.TempDir(), "joined.tenn", `element "T" {
  item a { item b { title B } item c { title C } }
}
element "S" {
  use T as s { at a.b { title One } at a.c { title Two } }
}`)
	if err != nil {
		t.Fatal(err)
	}
	var said []string
	for _, d := range tree.Diagnostics() {
		if !d.Warn {
			t.Errorf("%s is not a warning: %s", d.Pos, d.Msg)
		}
		said = append(said, d.Msg)
	}
	if len(said) != 2 {
		t.Fatalf("diagnostics %v, want two joined-block warnings", said)
	}
	for _, m := range said {
		if !strings.Contains(m, "blocks in one command") {
			t.Errorf("unexpected diagnostic %q", m)
		}
	}
}
