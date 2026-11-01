package scene_test

import (
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/exec"
	"github.com/haiodo/tenniarb/go/scene"
)

// TestTemplateClosing: "%{ ... %}" is what a reader writes, and the percent of the closing marker does not
// land in the generated text.
func TestTemplateClosing(t *testing.T) {
	tree := paramsTree(t, "close.tenn", `element "B" {
  kind Component
  export let title = "Save"
  template %{
<button>${title}</button>
%}
}`)
	got, err := tree.Render(tree.File("close.tenn").Elems[0], nil, scene.Options{EvalVars: exec.SceneEvalVars})
	if err != nil {
		t.Fatal(err)
	}
	if got != "<button>Save</button>" {
		t.Errorf("template = %q, want the text without the closing percent", got)
	}
}

// TestTemplateFallback: a slot the use leaves empty gets the fallback the component writes for it.
func TestTemplateFallback(t *testing.T) {
	tree := paramsTree(t, "fallback.tenn", `element "Card" {
  kind Component
  template %{<div><slot/></div>%}
  item Box {
    slot {
      item Empty { title "empty" }
    }
  }
}
element "Screen" {
  item col { auto-layout column
    use Card as one
    use Card as two { item Body { title "body" } }
  }
}`)
	o := scene.Options{EvalVars: exec.SceneEvalVars}
	for _, c := range []struct{ path, want string }{
		{"Card", "<div>empty</div>"},
		{"Screen.col.one", "<div>empty</div>"},
		{"Screen.col.two", "<div>body</div>"},
	} {
		node, err := tree.Resolve(tree.File("fallback.tenn"), c.path)
		if err != nil {
			t.Fatalf("%s: %v", c.path, err)
		}
		got, err := tree.Render(node, nil, o)
		if err != nil {
			t.Fatalf("%s: %v", c.path, err)
		}
		if strings.TrimSpace(got) != c.want {
			t.Errorf("%s = %q, want %q", c.path, strings.TrimSpace(got), c.want)
		}
	}
}
