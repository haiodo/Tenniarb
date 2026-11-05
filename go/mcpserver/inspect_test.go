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
	got := inspectScene(screen, inspectDepth)
	for _, want := range []string{
		"geometry:", "Plate", "Big", "leaves its parent: right",
		"overlap: A and B share 20x20",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("report lacks %q:\n%s", want, got)
		}
	}
	// The summary keeps the scene, the top level and the problems, and counts what it does not list.
	if clean := inspectScene(screen, 0); strings.Contains(clean, "ok: nothing leaves") {
		t.Errorf("a scene with problems says it is clean:\n%s", clean)
	}
	short := inspectScene(screen, 0)
	if len(short) >= len(got) || !strings.Contains(short, "… 1 below") || !strings.Contains(short, "leaves its parent") {
		t.Errorf("summary of %d bytes is not shorter than %d, or lost the problems:\n%s", len(short), len(got), short)
	}
}

// TestLayoutTool: layout reports the geometry without a renderer, and read takes several paths at once.
func TestLayoutTool(t *testing.T) {
	root := tempRoot(t, map[string]string{
		"s.tenn": "element S {\n" +
			"\titem Plate { pos 0 0; width 100; height 50; auto-layout abs\n" +
			"\t\titem Big { pos 10 10; width 120; height 20 }\n\t}\n}\n",
		"t.tenn": "element T {\n\titem A { pos 0 0; width 10; height 10 }\n\titem B { pos 20 0; width 10; height 10 }\n}\n",
		"u.tenn": "element U {\n\titem Box { auto-layout column; width 100; height 50\n\t\titem A { width 10; height 10 }\n\t}\n}\n",
	})
	cs := memSession(t, root)
	got := mustOK(t, cs, "layout", map[string]any{"file": "s.tenn", "element": "S"})
	if !strings.Contains(got, "leaves its parent: right") {
		t.Fatalf("layout:\n%s", got)
	}
	clean := mustOK(t, cs, "layout", map[string]any{"file": "u.tenn", "element": "U"})
	if !strings.Contains(clean, "ok: nothing leaves") {
		t.Fatalf("a clean scene does not say so:\n%s", clean)
	}
	got = mustOK(t, cs, "read", map[string]any{"file": "t.tenn", "paths": []string{"T.A", "T.B"}})
	if !strings.Contains(got, "item A") || !strings.Contains(got, "item B") {
		t.Fatalf("read of two paths:\n%s", got)
	}
}
