package mcpserver

import (
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/exec"
	"github.com/haiodo/tenniarb/go/scene"
)

// TestInspectScene: the report names the size, the boxes, the item that leaves its parent and an overlap.
func TestInspectScene(t *testing.T) {
	const text = "element S {\n" +
		"\titem Plate { pos 0 0; width 100; height 50; auto-layout abs\n" +
		"\t\titem Big { pos 10 10; width 120; height 20 }\n" +
		"\t}\n" +
		"\titem A { pos 0 100; width 40; height 20; auto-layout abs }\n" +
		"\titem B { pos 20 100; width 40; height 20; auto-layout abs }\n" +
		"}\n"
	tree, err := scene.LoadSource(t.TempDir(), "s.tenn", text)
	if err != nil {
		t.Fatal(err)
	}
	sc := gfx.NewScene()
	var screen *scene.Screen
	sc.Update(func(tx *gfx.Tx) {
		screen, err = tree.Build(tx, nil, "S", scene.Options{Measurer: gfx.EstimateText{}, EvalVars: exec.SceneEvalVars})
	})
	if err != nil {
		t.Fatal(err)
	}
	got := inspectScene(screen)
	for _, want := range []string{
		"geometry:", "Plate", "Big", "leaves its parent: right",
		"overlap: A and B share 20x20",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("report lacks %q:\n%s", want, got)
		}
	}
}
