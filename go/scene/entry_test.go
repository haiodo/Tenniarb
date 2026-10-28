package scene_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/tenniarb/go/scene"
)

const (
	libText  = "element \"Parts\" { item Button { width 40; height 20 } }\nitem Card { width 10; height 10 }\n"
	partText = "element \"Screen\" { use ui.Parts.Button as b }\n"
)

// instWidth builds path of tree and returns the width of the first instance in it.
func instWidth(t *testing.T, tree *scene.Tree, path string) float64 {
	t.Helper()
	sc := gfx.NewScene()
	var screen *scene.Screen
	var err error
	sc.Update(func(tx *gfx.Tx) { screen, err = tree.Build(tx, nil, path, scene.Options{}) })
	if err != nil {
		t.Fatal(err)
	}
	var find func(p *scene.Placed) float64
	find = func(p *scene.Placed) float64 {
		if p.Inst {
			return p.W
		}
		for _, k := range p.Kids {
			if w := find(k); w != 0 {
				return w
			}
		}
		return 0
	}
	return find(screen.Placed())
}

func TestEntryProvidesLibrary(t *testing.T) {
	root := writeFiles(t, map[string]string{
		"design.tenn":       "element \"ui\" --from-file \"lib/ui.tenn\"\nelement \"tracker\" --from-file \"apps/tracker.tenn\"\n",
		"lib/ui.tenn":       libText,
		"apps/tracker.tenn": partText,
	})
	if _, err := scene.Load(root, "apps/tracker.tenn"); err == nil || !strings.Contains(err.Error(), `apps/tracker.tenn:1:`) || !strings.Contains(err.Error(), "is not defined here or above") {
		t.Fatalf("alone: %v", err)
	}
	tree, err := scene.LoadIn(root, "apps/tracker.tenn", "design.tenn", nil)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Entry() != "design.tenn" || tree.Files()[0] != "apps/tracker.tenn" {
		t.Errorf("entry %q files %v", tree.Entry(), tree.Files())
	}
	if w := instWidth(t, tree, "Screen"); w != 40 {
		t.Errorf("width %v", w)
	}
	n, err := tree.Resolve(tree.File("apps/tracker.tenn"), "ui.Parts.Button")
	if err != nil || n.Pos.File != "lib/ui.tenn" {
		t.Errorf("resolve %v, %v", n, err)
	}
}

func TestEntryOwnFileWins(t *testing.T) {
	root := writeFiles(t, map[string]string{
		"design.tenn": "element \"ui\" --from-file \"lib/ui.tenn\"\nelement \"part\" --from-file \"part.tenn\"\n",
		"lib/ui.tenn": libText,
		"part.tenn": "element \"ui\" { element \"Parts\" { item Button { width 7; height 7 } } }\n" +
			"element \"Screen\" { use ui.Parts.Button as b }\n",
	})
	tree, err := scene.LoadIn(root, "part.tenn", "design.tenn", nil)
	if err != nil {
		t.Fatal(err)
	}
	if w := instWidth(t, tree, "Screen"); w != 7 {
		t.Errorf("the file's own ui must win over the entry's: width %v", w)
	}
}

func TestEntryTwoEntriesTwoLibraries(t *testing.T) {
	root := writeFiles(t, map[string]string{
		"a/design.tenn": "element \"ui\" --from-file \"../lib/a.tenn\"\nelement \"p\" --from-file \"../part.tenn\"\n",
		"b/design.tenn": "element \"ui\" --from-file \"../lib/b.tenn\"\nelement \"p\" --from-file \"../part.tenn\"\n",
		"lib/a.tenn":    libText,
		"lib/b.tenn":    strings.Replace(libText, "width 40", "width 90", 1),
		"part.tenn":     partText,
	})
	for entry, want := range map[string]float64{"a/design.tenn": 40, "b/design.tenn": 90} {
		tree, err := scene.LoadIn(root, "part.tenn", entry, nil)
		if err != nil {
			t.Fatal(err)
		}
		if w := instWidth(t, tree, "Screen"); w != want {
			t.Errorf("%s: width %v, want %v", entry, w, want)
		}
	}
}

func TestEntrySameFileTwoMounts(t *testing.T) {
	root := writeFiles(t, map[string]string{
		"design.tenn": "element \"l1\" { element \"ui\" --from-file \"lib/a.tenn\"; element \"p\" --from-file \"part.tenn\" }\n" +
			"element \"l2\" { element \"ui\" --from-file \"lib/b.tenn\"; element \"p\" --from-file \"part.tenn\" }\n" +
			"element \"Show1\" { use l1.p.Screen as s }\nelement \"Show2\" { use l2.p.Screen as s }\n",
		"lib/a.tenn": libText,
		"lib/b.tenn": strings.Replace(libText, "width 40", "width 90", 1),
		"part.tenn":  partText,
	})
	tree, err := scene.Load(root, "design.tenn")
	if err != nil {
		t.Fatal(err)
	}
	if w := instWidth(t, tree, "Show1"); w != 40 {
		t.Errorf("first mount: %v", w)
	}
	if w := instWidth(t, tree, "Show2"); w != 90 {
		t.Errorf("second mount must resolve in its own context: %v", w)
	}
}

func TestEntryCheckedOnce(t *testing.T) {
	root := writeFiles(t, map[string]string{
		"design.tenn":       "element \"ui\" --from-file \"lib/ui.tenn\"\nelement \"tracker\" --from-file \"apps/tracker.tenn\"\n",
		"lib/ui.tenn":       libText,
		"apps/tracker.tenn": "element \"Screen\" { use ui.Parts.Nope as b }\n",
	})
	es, _, err := scene.Diagnose(root, "design.tenn", nil)
	if err != nil || len(es) != 1 || es[0].Pos.File != "apps/tracker.tenn" {
		t.Errorf("%v, %v", es, err)
	}
}

func TestEntryNotMountingFile(t *testing.T) {
	root := writeFiles(t, map[string]string{"design.tenn": "element \"x\" {}\n", "part.tenn": partText})
	if _, err := scene.LoadIn(root, "part.tenn", "design.tenn", nil); err == nil || !strings.Contains(err.Error(), "does not mount part.tenn") {
		t.Errorf("%v", err)
	}
}

func TestEntryExplicitProperty(t *testing.T) {
	root := writeFiles(t, map[string]string{
		"design.tenn":       "element \"ui\" --from-file \"lib/ui.tenn\"\nelement \"p\" --from-file \"deep/er/part.tenn\"\n",
		"lib/ui.tenn":       libText,
		"deep/er/part.tenn": "entry \"../../design.tenn\"\n" + partText,
		"deep/er/bad.tenn":  "entry \"../../nope.tenn\"\n" + partText,
	})
	tree, err := scene.LoadAuto(root, "deep/er/part.tenn", nil)
	if err != nil || tree.Entry() != "design.tenn" {
		t.Fatalf("%v entry %q", err, tree.Entry())
	}
	// The property is on by itself: the file loads in its entry even when it would load alone.
	tree, err = scene.Load(root, "deep/er/part.tenn")
	if err == nil {
		t.Errorf("alone it must fail, got %v", tree)
	}
	_, err = scene.LoadAuto(root, "deep/er/bad.tenn", nil)
	if err == nil || !strings.Contains(err.Error(), "deep/er/bad.tenn:1:1: entry:") {
		t.Errorf("bad entry: %v", err)
	}
}

func TestEntryDiscovery(t *testing.T) {
	root := writeFiles(t, map[string]string{
		"foundation/Design.tenn":         "element \"ui\" --from-file \"lib/ui.tenn\"\nelement \"cat\" --from-file \"categories/all.tenn\"\n",
		"foundation/lib/ui.tenn":         libText,
		"foundation/categories/all.tenn": "element \"tracker\" --from-file \"../apps/tracker.tenn\"\n",
		"foundation/apps/tracker.tenn":   partText,
		"foundation/apps/other.tenn":     "element \"x\" {}\n",
	})
	got, ok := scene.FindEntry(root, "foundation/apps/tracker.tenn")
	want, _ := filepath.EvalSymlinks(filepath.Join(root, "foundation/Design.tenn"))
	if !ok || got != want {
		t.Fatalf("entry %q %v, want %q (the top of the chain, not categories/all.tenn)", got, ok, want)
	}
	if _, ok := scene.FindEntry(root, "foundation/apps/other.tenn"); ok {
		t.Error("a file nothing mounts has no entry")
	}
	tree, err := scene.LoadAuto(root, "foundation/apps/tracker.tenn", nil)
	if err != nil || tree.Entry() != "foundation/Design.tenn" {
		t.Fatalf("%v entry %q", err, tree.Entry())
	}
	// A file that loads clean alone keeps its own context.
	tree, err = scene.LoadAuto(root, "foundation/apps/other.tenn", nil)
	if err != nil || tree.Entry() != "" {
		t.Errorf("%v entry %q", err, tree.Entry())
	}
}

func TestEntryDiscoveryUsesNearest(t *testing.T) {
	root := writeFiles(t, map[string]string{
		"top.tenn":      "element \"ui\" --from-file \"lib/ui.tenn\"\nelement \"p\" --from-file \"sub/part.tenn\"\n",
		"lib/ui.tenn":   libText,
		"sub/part.tenn": partText,
		"sub/near.tenn": "element \"ui\" --from-file \"../lib/ui.tenn\"\nelement \"p\" --from-file \"part.tenn\"\n",
	})
	got, ok := scene.FindEntry(root, "sub/part.tenn")
	if !ok || filepath.Base(got) != "near.tenn" {
		// near.tenn is not mounted by top.tenn, so the chain ends at it.
		t.Errorf("entry %q %v", got, ok)
	}
	if err := os.Remove(filepath.Join(root, "sub/near.tenn")); err != nil {
		t.Fatal(err)
	}
	got, ok = scene.FindEntry(root, "sub/part.tenn")
	if !ok || filepath.Base(got) != "top.tenn" {
		t.Errorf("after removing: %q %v", got, ok)
	}
}

func TestEntryLibraryUsesSiblingLibrary(t *testing.T) {
	root := writeFiles(t, map[string]string{
		"design.tenn": "element \"icons\" --from-file \"icons.tenn\"\nelement \"ui\" --from-file \"ui.tenn\"\nelement \"app\" --from-file \"app.tenn\"\n",
		"icons.tenn":  "item Star { width 12; height 12 }\n",
		"ui.tenn":     "item Badge { use icons.Star as s }\n",
		"app.tenn":    "element \"Screen\" { use ui.Badge as b }\n",
	})
	tree, err := scene.Load(root, "design.tenn")
	if err != nil {
		t.Fatal(err)
	}
	if w := instWidth(t, tree, "app.Screen"); w == 0 {
		t.Error("the instance was not built")
	}
	if _, err := scene.Load(root, "ui.tenn"); err == nil {
		t.Error("ui.tenn alone has no icons")
	}
}
