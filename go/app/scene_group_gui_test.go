//go:build darwin || linux || windows

package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/gowt/qa"
	"github.com/haiodo/gowt/qa/qatest"
)

// TestSceneGroup: two items become one, the group moves both, ungroup gives the items back at their place,
// and every step is one undo step.
func TestSceneGroup(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	file := filepath.Join(t.TempDir(), "group.tenn")
	text := "element Screen {\n" +
		"\titem A { pos 0 0; width 60; height 40; auto-layout abs; color #ff0000 }\n" +
		"\titem B { pos 100 0; width 60; height 40; auto-layout abs; color #00ff00 }\n}\n"
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	defer h.open(fixture)
	h.open(file)
	a.State("command", "name", "close-others")
	a.Settle()

	a.State("select", "path", file, "element", []string{"Screen"}, "nodes", []string{"A", "B"})
	a.Settle()
	menu(a, "Group")
	a.Settle()

	grouped := h.text()
	if !strings.Contains(grouped, "item Group {") || !strings.Contains(grouped, "item B {") {
		t.Fatalf("not grouped:\n%s", grouped)
	}
	if st := h.state(); len(st.Nodes) != 1 || st.Nodes[0] != "Group" {
		t.Fatalf("the group is not selected: %v", st.Nodes)
	}

	// the group moves as one: a press on a child drags the group it belongs to
	nodes := h.scene()
	b := nodes["Group"].Box
	x, y := b[0]+b[2]*0.25, b[1]+b[3]/2
	a.Find(qa.ID("canvas")).DragTo(x, y, x+40, y+20)
	a.Settle()
	moved := h.text()
	if moved == grouped {
		t.Fatalf("the group did not move:\n%s", moved)
	}
	if st := h.state(); len(st.Nodes) != 1 || st.Nodes[0] != "Group" {
		t.Fatalf("the group lost the selection: %v", st.Nodes)
	}

	menu(a, "Ungroup")
	a.Settle()
	ungrouped := h.text()
	if strings.Contains(ungrouped, "item Group {") || !strings.Contains(ungrouped, "item A {") || !strings.Contains(ungrouped, "item B {") {
		t.Fatalf("not ungrouped:\n%s", ungrouped)
	}

	// two undo steps: the ungroup, then the move
	a.Key("Mod+Z")
	a.Settle()
	if got := h.text(); got != moved {
		t.Fatalf("undo of the ungroup:\n%s", got)
	}
	a.Key("Mod+Z")
	a.Settle()
	if got := h.text(); got != grouped {
		t.Fatalf("undo of the move:\n%s", got)
	}
}
