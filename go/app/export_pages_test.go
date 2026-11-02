//go:build darwin || linux || windows

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/model"
)

// The pages of an element are the element and what is inside it, depth first, mounted content included.
func TestPagesOf(t *testing.T) {
	dir := t.TempDir()
	lib := "element \"Parts\" {\n  item Button { width 40; height 20 }\n  element \"Dialogs\" { item Box { width 10; height 10 } }\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "kit.lib.tenn"), []byte(lib), 0o644); err != nil {
		t.Fatal(err)
	}
	text := `element "Kit" --from-file "kit.lib.tenn"
element "Book" {
    element "Chapter" {
        item A { width 40; height 20 }
        element "Section" {
            element "Part" { item B { width 10; height 10 } }
        }
    }
}
`
	v := newDoc(t, &Shell{}, filepath.Join(dir, "main.tenn"), text)
	root := v.session.Root()
	if len(root.Elements) != 2 {
		t.Fatalf("root elements: %d", len(root.Elements))
	}
	paths := func(e *model.Element) []string {
		var out []string
		for _, p := range pagesOf(e) {
			out = append(out, strings.Join(PathOf(p), "/"))
		}
		return out
	}
	if got, want := paths(root.Elements[1]), []string{"Book", "Book/Chapter", "Book/Chapter/Section", "Book/Chapter/Section/Part"}; !equalStrings(got, want) {
		t.Errorf("Book pages %v, want %v", got, want)
	}
	// the mounted elements are pages of the element that shows the file
	if got, want := paths(root.Elements[0]), []string{"Kit", "Kit/Parts", "Kit/Parts/Dialogs"}; !equalStrings(got, want) {
		t.Errorf("mounted pages %v, want %v", got, want)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestPageName(t *testing.T) {
	for _, c := range []struct{ name, want string }{
		{"Chapter One", "Chapter_One"},
		{"a/b:c", "a_b_c"},
		{"a\tb", "a_b"},
		{".hidden.", "hidden"},
		{" ", "Untitled"},
		{"", "Untitled"},
		{"Café", "Café"},
	} {
		if got := pageName(c.name); got != c.want {
			t.Errorf("%q: %q, want %q", c.name, got, c.want)
		}
	}
}

func TestPageFileName(t *testing.T) {
	for _, c := range []struct {
		i, n int
		name string
		want string
	}{
		{1, 4, "Chapter One", "01-Chapter_One.png"},
		{3, 4, "", "03-Untitled.png"},
		{12, 120, "x", "012-x.png"},
	} {
		if got := pageFileName(c.i, c.n, c.name, ".png"); got != c.want {
			t.Errorf("%d/%d %q: %q, want %q", c.i, c.n, c.name, got, c.want)
		}
	}
}
