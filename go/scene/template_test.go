package scene_test

import (
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/exec"
	"github.com/haiodo/tenniarb/go/scene"
)

// templateDoc is three components that generate text (spec 5c, 1d.5): Chip fills ${...} from its
// parameters, Card puts the nodes of the use into its slots, Calc evaluates an expression.
const templateDoc = `element "Chip" {
  kind Component
  export let label = "Chip"
  export let tone = plain
  export let extras = outlined
  template %{<span class="${tone} ${extras}">${label}</span>}
}

element "Card" {
  kind Component
  template {
    <div class="card">
      <slot/>
      <slot actions/>
    </div>
  }
  item Box {
    auto-layout column
    item Title { title Card }
    slot
    slot actions
  }
}

element "Calc" {
  kind Component
  export let n = 40
  template %{<i>${n + 12}</i>}
}

element "Panel" {
  kind Component
  export let width
  template %{<aside style="width: ${width}"></aside>}
}

element "Screen" {
  item col {
    auto-layout column
    use Chip as one
    use Chip as two { label "Save"; tone primary; extras rounded outlined }
    use Calc as sum { n 1 }
    use Card as card {
      item Body { title "<p>Hello</p>" }
      slot actions { item Go { title "<button>Go</button>" } }
    }
    use Panel as side { width 320 }
  }
}
`

// renderOf resolves path in the main file of tree and renders it with the values given, evaluating
// ${...} as the app does.
func renderOf(t *testing.T, tree *scene.Tree, path string, values map[string]string) (string, error) {
	t.Helper()
	node, err := tree.Resolve(tree.File(tree.Files()[0]), path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return tree.Render(node, values, scene.Options{EvalVars: exec.SceneEvalVars})
}

// TestTemplateDefaults: the parameters render with their defaults and -set overrides them; an
// undeclared name is an error that lists the parameters.
func TestTemplateDefaults(t *testing.T) {
	tree := paramsTree(t, "tpl.tenn", templateDoc)
	if d := tree.Diagnostics(); len(d) != 0 {
		t.Fatalf("diagnostics: %v", d)
	}
	got, err := renderOf(t, tree, "Chip", nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := `<span class="plain outlined">Chip</span>`; got != want {
		t.Errorf("defaults: %q, want %q", got, want)
	}
	got, err = renderOf(t, tree, "Chip", map[string]string{"label": "Save", "tone": "primary", "extras": "rounded outlined"})
	if err != nil {
		t.Fatal(err)
	}
	if want := `<span class="primary rounded outlined">Save</span>`; got != want {
		t.Errorf("values: %q, want %q", got, want)
	}
	_, err = renderOf(t, tree, "Chip", map[string]string{"nope": "1"})
	if err == nil || !strings.Contains(err.Error(), `unknown parameter "nope"`) ||
		!strings.Contains(err.Error(), `parameters: "label", "tone", "extras"`) {
		t.Errorf("want an error about the unknown parameter and the list of parameters, got %v", err)
	}
}

// TestTemplateUse: a use of the component gives the values and the slot content, and a parameter
// may be a list of words.
func TestTemplateUse(t *testing.T) {
	tree := paramsTree(t, "tpl.tenn", templateDoc)
	cases := map[string]string{
		"Screen.col.one": `<span class="plain outlined">Chip</span>`,
		"Screen.col.two": `<span class="primary rounded outlined">Save</span>`,
		"Screen.col.sum": `<i>13</i>`,
	}
	for path, want := range cases {
		got, err := renderOf(t, tree, path, nil)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if got != want {
			t.Errorf("%s: %q, want %q", path, got, want)
		}
	}
}

// TestTemplateSlots: <slot/> takes the nodes the use passes as their titles, named slots too; a
// slot the use does not fill is an error.
func TestTemplateSlots(t *testing.T) {
	tree := paramsTree(t, "tpl.tenn", templateDoc)
	got, err := renderOf(t, tree, "Screen.col.card", nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "<div class=\"card\">\n  <p>Hello</p>\n  <button>Go</button>\n</div>"
	if got != want {
		t.Errorf("slots: %q, want %q", got, want)
	}
	_, err = renderOf(t, tree, "Card", nil)
	if err == nil || !strings.Contains(err.Error(), "the default slot has no content") {
		t.Errorf("want an error about the empty default slot, got %v", err)
	}
}

// TestTemplateRequired: a parameter without a default and without a value is an error; -set and the
// block of a use both give it.
func TestTemplateRequired(t *testing.T) {
	tree := paramsTree(t, "tpl.tenn", templateDoc)
	_, err := renderOf(t, tree, "Panel", nil)
	if err == nil || !strings.Contains(err.Error(), `element "Panel" needs "width"`) {
		t.Errorf("want an error about the missing width, got %v", err)
	}
	got, err := renderOf(t, tree, "Panel", map[string]string{"width": "100%"})
	if err != nil {
		t.Fatal(err)
	}
	if want := `<aside style="width: 100%"></aside>`; got != want {
		t.Errorf("-set: %q, want %q", got, want)
	}
	if got, err = renderOf(t, tree, "Screen.col.side", nil); err != nil {
		t.Fatal(err)
	}
	if want := `<aside style="width: 320"></aside>`; got != want {
		t.Errorf("use: %q, want %q", got, want)
	}
}

// TestTemplateNone: an element without a template generates nothing, and a file without templates is
// unaffected: the template command is not a property of anything else.
func TestTemplateNone(t *testing.T) {
	tree := paramsTree(t, "params.tenn", componentDoc)
	if tree.File("params.tenn").Elems[0].Template != nil {
		t.Error("Button has a template it does not declare")
	}
	_, err := renderOf(t, tree, "Button", nil)
	if err == nil || !strings.Contains(err.Error(), "has no template") {
		t.Errorf("want an error about the missing template, got %v", err)
	}
	// template outside an element block is a property, as before
	plain := paramsTree(t, "item.tenn", `element "Screen" {
  item Box { template %{<b>x</b>}; width 10 }
}`)
	if d := plain.Diagnostics(); len(d) != 0 {
		t.Errorf("diagnostics: %v", d)
	}
	if plain.File("item.tenn").Elems[0].Content[0].Props[0].Name != "template" {
		t.Error("template inside an item is not kept as a property")
	}
}

// elementDoc is a screen of plain boxes and templated components: the code of the screen is the
// code of the uses, the boxes only place them.
const elementDoc = `element "Item" {
  kind Component
  export let text = "x"
  template %{<span>${text}</span>}
}

element "List" {
  kind Component
  template %{<ul><slot/></ul>}
  item Box { auto-layout column; slot }
}

element "Screen" {
  template %{<main><slot/></main>}
  item Page {
    auto-layout column
    use List as list {
      use Item as a { text "A" }
      use Item as b { text "B" }
    }
  }
}
`

// TestTemplateNested: a use inside the slot of another prints its own template with the values of
// its own block, not its name.
func TestTemplateNested(t *testing.T) {
	tree := paramsTree(t, "elem.tenn", elementDoc)
	if d := tree.Diagnostics(); len(d) != 0 {
		t.Fatalf("diagnostics: %v", d)
	}
	got, err := renderOf(t, tree, "Screen.Page.list", nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := "<ul><span>A</span>\n<span>B</span></ul>"; got != want {
		t.Errorf("nested: %q, want %q", got, want)
	}
}

// TestRenderElement: an element prints the templates of the uses below it in file order, and its own
// template wraps them in the default slot; a template given instead replaces it.
func TestRenderElement(t *testing.T) {
	tree := paramsTree(t, "elem.tenn", elementDoc)
	node, err := tree.Resolve(tree.File(tree.Files()[0]), "Screen")
	if err != nil {
		t.Fatal(err)
	}
	got, err := tree.RenderElement(node, nil, nil, scene.Options{EvalVars: exec.SceneEvalVars})
	if err != nil {
		t.Fatal(err)
	}
	if want := "<main><ul><span>A</span>\n<span>B</span></ul></main>"; got != want {
		t.Errorf("element: %q, want %q", got, want)
	}
	wrap := &scene.Template{Text: "<x>\n<slot/>\n</x>"}
	got, err = tree.RenderElement(node, wrap, nil, scene.Options{EvalVars: exec.SceneEvalVars})
	if err != nil {
		t.Fatal(err)
	}
	if want := "<x>\n<ul><span>A</span>\n<span>B</span></ul>\n</x>"; got != want {
		t.Errorf("wrapper: %q, want %q", got, want)
	}
}
