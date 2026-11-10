package scene_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/tenniarb/go/scene"
)

// iconLib is a library of one icon, the form libraries/icons.tenn has.
const iconLib = `item "zap" {
	width 24; height 24; path "M1 2 L3 4"
	view-box 0 0 24 24; border-color #000000; line-width 2
}
`

// libraryEnv puts lib into the folder the scheme looks at before the shipped ones, and returns the
// folder. The config folder is emptied too, so a library installed on the machine cannot shadow it.
func libraryEnv(t *testing.T, lib string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "icons.tenn"), []byte(lib), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TENNIARB_LIBRARIES", dir)
	t.Setenv("TENNIARB_CONFIG_DIR", t.TempDir())
	t.Setenv("GOWT_QA_DATA", "")
	return dir
}

// A document mounts a library of the application by name, wherever the document lies: the text of the
// library goes into the overlay under a reserved path, so no file beside the document is needed.
func TestMountLibraryScheme(t *testing.T) {
	libraryEnv(t, iconLib)
	root := writeFiles(t, map[string]string{
		"main.tenn": "element \"icons\" --from-file \"tenniarb-library://icons\"\n" +
			"element \"S\" {\n  use icons.zap as Z { width 24; height 24; border-color #000000 }\n}\n",
	})
	diags, tree, err := scene.Diagnose(root, "main.tenn", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(diags) != 0 {
		t.Fatalf("diagnostics: %v", diags)
	}
	reserved := ".tenniarb-library/icons.tenn"
	if f := tree.File(reserved); f == nil {
		t.Fatalf("the library is not mounted as %s: files %v", reserved, tree.Files())
	}
	if src, ok := tree.Source(reserved); !ok || !strings.Contains(src, "M1 2 L3 4") {
		t.Errorf("the source of the mounted library is %q, %v", src, ok)
	}
	main := tree.File("main.tenn")
	z, err := tree.Resolve(main, "icons.zap")
	if err != nil {
		t.Fatal(err)
	}
	if z.Pos.File != reserved {
		t.Errorf("icons.zap comes from %q, want %q", z.Pos.File, reserved)
	}
	sc := gfx.NewScene()
	sc.Update(func(tx *gfx.Tx) {
		if _, err := tree.Build(tx, nil, "S", scene.Options{}); err != nil {
			t.Error(err)
		}
	})
}

// A scheme name the folders do not have is an error that says what was looked for and where.
func TestMountLibraryUnknownName(t *testing.T) {
	dir := libraryEnv(t, iconLib)
	root := writeFiles(t, map[string]string{
		"main.tenn": "element \"icons\" --from-file \"tenniarb-library://nope\"\n",
	})
	diags, _, err := scene.Diagnose(root, "main.tenn", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(diags) == 0 {
		t.Fatal("mounting a library that does not exist has no error")
	}
	msg := diags[0].Error()
	for _, want := range []string{"library nope not found", "nope.tenn", dir} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q does not name %q", msg, want)
		}
	}
}

// A file of the same name beside the document is not the library: the scheme always reads the one the
// application ships or the user installed.
func TestMountLibraryIgnoresLocalFile(t *testing.T) {
	libraryEnv(t, iconLib)
	root := writeFiles(t, map[string]string{
		"icons.tenn": "item \"local\" { width 10; height 10 }\n",
		"main.tenn": "element \"icons\" --from-file \"tenniarb-library://icons\"\n" +
			"element \"S\" { use icons.zap as Z { width 24; height 24 } }\n" +
			"element \"L\" --from-file \"icons.tenn\"\n" +
			"element \"T\" { use L.local as P { width 10; height 10 } }\n",
	})
	diags, tree, err := scene.Diagnose(root, "main.tenn", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(diags) != 0 {
		t.Fatalf("diagnostics: %v", diags)
	}
	main := tree.File("main.tenn")
	z, err := tree.Resolve(main, "icons.zap")
	if err != nil || z.Pos.File != ".tenniarb-library/icons.tenn" {
		t.Errorf("icons.zap = %v, %v; want it from the library of the application", z, err)
	}
	local, err := tree.Resolve(main, "L.local")
	if err != nil || local.Pos.File != "icons.tenn" {
		t.Errorf("L.local = %v, %v; want it from the file beside the document", local, err)
	}
}

// A bad scheme name is reported at the element, not read as a path: the reserved folder is not a place
// a document may reach.
func TestMountLibraryBadName(t *testing.T) {
	libraryEnv(t, iconLib)
	root := writeFiles(t, map[string]string{
		"main.tenn": "element \"icons\" --from-file \"tenniarb-library://../secret\"\n",
	})
	diags, _, err := scene.Diagnose(root, "main.tenn", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(diags) == 0 {
		t.Fatal("a library name that leaves its folder has no error")
	}
	if msg := diags[0].Error(); !strings.Contains(msg, "library name") {
		t.Errorf("error %q does not name the rule of a library name", msg)
	}
}
