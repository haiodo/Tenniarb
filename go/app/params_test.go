//go:build darwin || linux || windows

package app

import (
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/scene"
)

const paramEditDoc = `element "Button" kind Component {
  export let title = "Button"
  export let kind = primary
  export let n = 40
  styles {
    primary { color red }
    secondary { color blue }
    kind one-of primary secondary
  }
  item Btn { title ${title} }
}
element "Screen" { use Button as save }
`

// TestParamEdit: a value of a parameter goes into the block of the use as its declaration writes it - a
// string quoted, a number or a word bare - in place of the command already there, and a removed value
// gives the default back.
func TestParamEdit(t *testing.T) {
	file := func(text string) (*scene.Node, *scene.Node) {
		t.Helper()
		tree, err := scene.LoadSource(t.TempDir(), "p.tenn", text)
		if err != nil {
			t.Fatal(err)
		}
		btn, screen := tree.File("p.tenn").Elems[0], tree.File("p.tenn").Elems[1]
		return btn, NodeAt(screen, "save")
	}
	btn, use := file(paramEditDoc)
	for _, c := range []struct{ name, value, want string }{
		{"title", "Save", `use Button as save { title "Save" }`},
		{"title", "Two words", `use Button as save { title "Two words" }`},
		{"kind", "secondary", `use Button as save { kind secondary }`},
		{"n", "48", `use Button as save { n 48 }`},
	} {
		e, ok := ParamEdit(paramEditDoc, use, *btn.Param(c.name), c.value)
		if !ok {
			t.Fatalf("%s: no edit", c.name)
		}
		if got := apply(paramEditDoc, e); !strings.Contains(got, c.want) {
			t.Errorf("%s %q:\n%s\nwant\n%s", c.name, c.value, got, c.want)
		}
	}

	// the command already in the block is replaced, not written twice; no value removes it
	withKind := `use Button as save { kind secondary }`
	text := strings.Replace(paramEditDoc, "use Button as save", withKind, 1)
	btn, use = file(text)
	if e, ok := ParamEdit(text, use, *btn.Param("kind"), "primary"); !ok {
		t.Fatal("no edit in place")
	} else if got := apply(text, e); !strings.Contains(got, "use Button as save { kind primary }") {
		t.Errorf("in place:\n%s", got)
	}
	if e, ok := RemoveParamEdit(text, use, *btn.Param("kind")); !ok {
		t.Fatal("no removal")
	} else if got := apply(text, e); !strings.Contains(got, "use Button as save") || strings.Contains(got, "kind secondary") {
		t.Errorf("removal:\n%s", got)
	}
}
