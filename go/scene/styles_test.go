package scene_test

import (
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/scene"
)

const stylesDoc = `element "Button" {
  styles {
    primary { color #6b3ad3; text-color #ffffff }
    large { height 48; padding 0 16 }
    medium { height 40; padding 0 12 }
    outlined { border-color #1725471a }
    wide_label { Label { width 200 } }
    kind one-of primary secondary
  }
  item Btn {
    auto-layout row
    align center
    justify center
    gap 8
    use-style primary
    use-style medium
    item Label { title Button; font-size 14 }
  }
}

element "Screen" {
  use Button as plain
  use Button as big { at Btn { use-style large } }
  use Button as small { at Btn { use-style -medium } }
  use Button as wide { at Btn { use-style large; height 60 } }
  use Button as nested { at Btn { use-style wide_label } }
}
`

// styleBoxes builds Screen of stylesDoc: the height of every use shows the style that won.
func styleBoxes(t *testing.T) map[string]scene.Box {
	t.Helper()
	tree, err := scene.LoadSource(t.TempDir(), "styles.tenn", stylesDoc)
	if err != nil {
		t.Fatal(err)
	}
	screen, _ := build(t, tree, "Screen")
	m := map[string]scene.Box{}
	for _, b := range screen.Boxes() {
		m[b.Path] = b
	}
	return m
}

func TestStylesFromUseStyle(t *testing.T) {
	m := styleBoxes(t)
	wantH := map[string]float64{"plain.Btn": 40, "big.Btn": 48, "wide.Btn": 60}
	if b := m["small.Btn"]; b.H == 40 || b.H <= 0 {
		t.Errorf("small.Btn height %v, want the content size after -medium", b.H)
	}
	if b := m["nested.Btn.Label"]; b.W != 200 {
		t.Errorf("nested label width %v, want 200 from the style's Label block", b.W)
	}
	for path, h := range wantH {
		b, ok := m[path]
		if !ok {
			t.Fatalf("no box %q in %v", path, m)
		}
		if b.H != h {
			t.Errorf("%s height %v, want %v", path, b.H, h)
		}
	}
}

// TestStyleGroups: the group declarations of a styles block are read in declaration order, both kinds
// apart, and they stay out of the styles: a group is no style and a style is no group.
func TestStyleGroups(t *testing.T) {
	const doc = `element "Button" {
  styles {
    kind one-of primary secondary ghost
    primary { color #6b3ad3 }
    size any-of large wide_label
    large { height 48 }
    bare
  }
  item Btn { use-style primary large }
}`
	tree, err := scene.LoadSource(t.TempDir(), "groups.tenn", doc)
	if err != nil {
		t.Fatal(err)
	}
	el := tree.File("groups.tenn").Elems[0]
	want := []scene.StyleGroup{
		{Name: "kind", Styles: []string{"primary", "secondary", "ghost"}},
		{Name: "size", AnyOf: true, Styles: []string{"large", "wide_label"}},
	}
	if len(el.StyleGroups) != len(want) {
		t.Fatalf("groups %v, want %v", el.StyleGroups, want)
	}
	for i, g := range el.StyleGroups {
		if g.Name != want[i].Name || g.AnyOf != want[i].AnyOf || strings.Join(g.Styles, ",") != strings.Join(want[i].Styles, ",") {
			t.Errorf("group %d is %+v, want %+v", i, g, want[i])
		}
	}
	for _, name := range []string{"primary", "large"} {
		if el.Styles[name] == nil {
			t.Errorf("no style %q in %v", name, el.Styles)
		}
	}
	for _, name := range []string{"kind", "size", "secondary", "ghost", "wide_label", "bare"} {
		if el.Styles[name] != nil {
			t.Errorf("%q is no style but is in the styles map", name)
		}
	}
	if scope := scene.StyleScope(ownItem(t, el, "Btn")); scope != el {
		t.Errorf("the scope of Btn is %v, want the element", scope)
	}
}

// ownItem is the kid of n with that name.
func ownItem(t *testing.T, n *scene.Node, name string) *scene.Node {
	t.Helper()
	for _, c := range n.Content {
		if c.Kind == scene.KItem && c.Name == name {
			return c
		}
	}
	t.Fatalf("no item %q in %q", name, n.Name)
	return nil
}

// TestStylesFromUseStyleProps: a property written on the node wins over the style.
func TestStylesFromUseStyleProps(t *testing.T) {
	tree, err := scene.LoadSource(t.TempDir(), "styles.tenn", `element "E" {
  styles { big { height 120; padding 0 20 } }
  item Btn { auto-layout row; use-style big; height 30; item L { title x } }
}`)
	if err != nil {
		t.Fatal(err)
	}
	screen, _ := build(t, tree, "E")
	for _, b := range screen.Boxes() {
		if b.Path == "Btn" && b.H != 30 {
			t.Errorf("Btn height %v, want the property 30", b.H)
		}
	}
}
