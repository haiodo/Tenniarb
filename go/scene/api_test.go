package scene_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/tenniarb/go/scene"
)

func TestNormalizeName(t *testing.T) {
	for in, want := range map[string]string{"Primary button": "Primary_button", "a-b.c": "a_b_c", "Кнопка 1": "Кнопка_1", "": ""} {
		if got := scene.NormalizeName(in); got != want {
			t.Errorf("NormalizeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolveFromScope(t *testing.T) {
	tree := load(t, "import.tenn")
	file := tree.File("import.tenn")
	n, err := tree.Resolve(file, "Screens.Login")
	if err != nil || n.Kind != scene.KElement {
		t.Fatalf("Screens.Login = %v, %v", n, err)
	}
	// short path from the scope of Login goes up to Screens.Button; a path through the mounting element leaves the file
	b, err := tree.Resolve(n, "Button")
	if err != nil || b.Pos.File != "import.tenn" {
		t.Errorf("Button from Login = %v, %v", b, err)
	}
	k, err := tree.Resolve(n, "kit.Button")
	if err != nil || k.Pos.File != "lib/kit.tenn" {
		t.Errorf("kit.Button = %v, %v", k, err)
	}
	if _, err := tree.Resolve(n, "Nope"); err == nil || !strings.Contains(err.Error(), "not defined here or above") {
		t.Errorf("Nope: %v", err)
	}
	if len(tree.Diagnostics()) != 0 {
		t.Errorf("Resolve reported into diagnostics: %v", tree.Diagnostics())
	}
	amb, _ := scene.Load("testdata", "ambiguous.tenn")
	if _, err := amb.Resolve(amb.File("ambiguous.tenn"), "A.Item"); err == nil || !strings.Contains(err.Error(), "ambiguous path A.Item: candidates ambiguous.tenn:2:5") {
		t.Errorf("ambiguous: %v", err)
	}
}

func TestNodeRange(t *testing.T) {
	tree := load(t, "import.tenn")
	src, ok := tree.Source("import.tenn")
	if !ok {
		t.Fatal("no source")
	}
	n, err := tree.Resolve(tree.File("import.tenn"), "Screens.Button")
	if err != nil {
		t.Fatal(err)
	}
	got := src[n.Range.Start:n.Range.End]
	if !strings.HasPrefix(got, `item "Button"`) || !strings.HasSuffix(got, "}") || strings.Count(got, "\n") != 3 {
		t.Errorf("range text = %q", got)
	}
	names := []string{}
	for _, c := range tree.File("import.tenn").Nodes() {
		names = append(names, c.Kind.String()+":"+c.Name)
	}
	if strings.Join(names, " ") != "element:kit element:Screens" {
		t.Errorf("file nodes = %v", names)
	}
}

func TestLoadSource(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "lib.tenn"), []byte("item \"Dot\" { width 4; height 4 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	text := "element lib --from-file \"lib.tenn\"\nelement \"E\" { use lib.Dot as d }\n"
	// the file is not on disk: only the in-memory text exists
	tree, err := scene.LoadSource(root, "main.tenn", text)
	if err != nil {
		t.Fatal(err)
	}
	if n, err := tree.Resolve(tree.File("main.tenn"), "E.d"); err != nil || n.Kind != scene.KUse {
		t.Errorf("E.d = %v, %v", n, err)
	}
	// the same name on disk is ignored in favour of the text
	if err := os.WriteFile(filepath.Join(root, "main.tenn"), []byte("garbage {"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := scene.LoadSource(root, "main.tenn", text); err != nil {
		t.Errorf("disk copy leaked: %v", err)
	}
	_, err = scene.LoadSource(root, "main.tenn", "element \"E\" { use lib.Nope as d }\nelement lib --from-file \"lib.tenn\"\n")
	if err == nil || !strings.Contains(err.Error(), "main.tenn:1:") {
		t.Errorf("want an error with a position, got %v", err)
	}
}

func TestPlacedAndBody(t *testing.T) {
	text := "item Kit { item Button { width 40; height 20 } }\nelement \"E\" {\n  item Card { auto-layout column; use Kit.Button as ok }\n}\nelement \"E\" { item Other }\n"
	tree, err := scene.LoadSource(t.TempDir(), "a.tenn", text)
	if err != nil {
		t.Fatal(err)
	}
	card := tree.File("a.tenn").Elems[0].Content[0]
	if b := text[card.Body.Start:card.Body.End]; b != "{ auto-layout column; use Kit.Button as ok }" {
		t.Errorf("body %q", b)
	}
	// the second element named E is reachable by its node, not by a path
	sc := gfx.NewScene()
	var screen *scene.Screen
	sc.Update(func(tx *gfx.Tx) {
		screen, err = tree.BuildElement(tx, nil, tree.File("a.tenn").Elems[0], scene.Options{})
	})
	if err != nil {
		t.Fatal(err)
	}
	p := screen.Placed()
	if len(p.Kids) != 1 || p.Kids[0].Src != card || !p.Kids[0].Free || p.Kids[0].Gfx == nil {
		t.Fatalf("placed %+v", p.Kids)
	}
	ok := p.Kids[0].Kids[0]
	if !ok.Inst || ok.Src.Kind != scene.KUse || ok.Free || ok.W != 40 || ok.Prop("width").Values()[0] != "40" {
		t.Errorf("instance %+v", ok)
	}
	if screen.NextID <= ok.Gfx.ID() {
		t.Errorf("next id %d", screen.NextID)
	}
	if f := tree.Files(); len(f) != 1 || f[0] != "a.tenn" {
		t.Errorf("files %v", f)
	}
}
