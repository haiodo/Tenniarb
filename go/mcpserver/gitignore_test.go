package mcpserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIgnored: patterns of the .gitignore files from the root down decide what the walk skips.
func TestIgnored(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, text string) {
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".gitignore", "# deps\nnode_modules/\nbuild\n!keep\n/out/\nsrc/gen\n*.tmp\n")
	write("sub/.gitignore", "tmp/\n")

	ig := loadIgnores(root)
	ig.read(filepath.Join(root, "sub"), "sub") // the walk reads it on entering sub
	for p, want := range map[string]bool{
		"node_modules":     true,
		"a/node_modules":   true,
		"build":            true,
		"deep/build":       true,
		"keep":             false,
		"out":              true,
		"sub/out":          false,
		"src/gen":          true,
		"sub/src/gen":      false,
		"sub/tmp":          true,
		"lib":              false,
		"web/node_modules": true,
		"web/lib":          false,
	} {
		if got := ig.match(p); got != want {
			t.Errorf("match(%q) = %v, want %v", p, got, want)
		}
	}
}

// TestIgnoredAbove: the .gitignore of the repository above the workspace root counts too.
func TestIgnoredAbove(t *testing.T) {
	base := t.TempDir()
	ws := filepath.Join(base, "dev")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, ".gitignore"), []byte("node_modules\n/generated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ig := loadIgnores(ws)
	if !ig.match("node_modules") || !ig.match("ui/node_modules") {
		t.Error("an unanchored pattern of the file above is not applied")
	}
	if ig.match("generated") {
		t.Error("a pattern anchored above the workspace cannot name anything inside it")
	}
}

// TestStateSkipsIgnored: a directory a .gitignore names is not searched, and one it does not stays.
func TestStateSkipsIgnored(t *testing.T) {
	root := tempRoot(t, map[string]string{
		".gitignore":                     "node_modules/\n",
		"design.tenn":                    "element Screen { item Btn { title Hi } }\n",
		"node_modules/@x/pkg/other.tenn": "element Lib { item A { title no } }\n",
		"packages/kept/pkg.tenn":         "element Kept { item A { title yes } }\n",
	})
	got := mustOK(t, memSession(t, root), "get_state", map[string]any{})
	if !strings.Contains(got, "design.tenn") || !strings.Contains(got, "packages/kept/pkg.tenn") {
		t.Fatalf("a file that is not ignored was left out:\n%s", got)
	}
	if strings.Contains(got, "node_modules") {
		t.Fatalf("an ignored directory was searched:\n%s", got)
	}
}
