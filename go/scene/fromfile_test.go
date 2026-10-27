package scene_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/tenniarb/go/scene"
)

func writeFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, text := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

const kitText = "element \"Parts\" {\n  item Button { width 40; height 20 }\n}\nitem Card { width 10; height 10 }\n"

func TestFromFileMount(t *testing.T) {
	root := writeFiles(t, map[string]string{
		"app/kit.lib.tenn": kitText,
		"app/main.tenn": "element \"kit\" --from-file \"kit.lib.tenn\"\n" +
			"element \"again\" --from-file \"kit.lib.tenn\"\n" +
			"element \"Screen\" { use kit.Parts.Button as b; use kit.Card as c }\n",
	})
	tree, err := scene.Load(root, "app/main.tenn")
	if err != nil {
		t.Fatal(err)
	}
	main := tree.File("app/main.tenn")
	b, err := tree.Resolve(main, "Screen.b")
	if err != nil || b.Kind != scene.KUse {
		t.Fatalf("Screen.b = %v, %v", b, err)
	}
	btn, err := tree.Resolve(main, "kit.Parts.Button")
	if err != nil || btn.Pos.File != "app/kit.lib.tenn" {
		t.Fatalf("kit.Parts.Button = %v, %v", btn, err)
	}
	again, _ := tree.Resolve(main, "again.Parts.Button")
	if again != btn {
		t.Error("the same file mounted twice must be the same subtree")
	}
	if f := tree.Files(); len(f) != 2 || f[1] != "app/kit.lib.tenn" {
		t.Errorf("files %v", f)
	}
	sc := gfx.NewScene()
	sc.Update(func(tx *gfx.Tx) {
		if _, err = tree.Build(tx, nil, "Screen", scene.Options{}); err != nil {
			t.Error(err)
		}
	})
}

func TestFromFileErrors(t *testing.T) {
	root := writeFiles(t, map[string]string{
		"a.tenn": "element \"x\" --from-file \"b.tenn\"\n",
		"b.tenn": "element \"y\" --from-file \"a.tenn\"\n",
		"c.tenn": "element \"out\" --from-file \"../outside.tenn\"\nelement \"gone\" --from-file \"nope.tenn\"\n",
	})
	_, err := scene.Load(root, "a.tenn")
	if err == nil || !strings.Contains(err.Error(), "import cycle") || !strings.Contains(err.Error(), "b.tenn:1:1") {
		t.Errorf("cycle: %v", err)
	}
	tree, err := scene.Load(root, "c.tenn")
	if err == nil || !strings.Contains(err.Error(), "leaves the workspace root") || !strings.Contains(err.Error(), "cannot read nope.tenn") {
		t.Errorf("outside/missing: %v", err)
	}
	if tree == nil || len(tree.File("c.tenn").Elems) != 2 {
		t.Error("an element with a bad file must stay in the tree, empty")
	}
}
