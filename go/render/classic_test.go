package render

import (
	"testing"

	"github.com/haiodo/tenniarb/go/scene"
)

// TestClassicValues: the values a classic diagram reads, by item name and index, by a template string and
// by a colour array; a literal comes back as written, an element property too.
func TestClassicValues(t *testing.T) {
	el := parseDiagram(t, `element Root {
    width ${10 + 20}
    item A {
        pos 0 0
        my_value 3
        value ${my_value + 1}
        title "${value}"
        color ${[my_value, 2, 3]}
    }
    item A {
        pos 100 0
        my_value 7
        value ${my_value * 2}
    }
}`)
	ctx := NewExecContext(el, testOpts())
	valu := ClassicValues(el, ctx)

	at := func(nth int) scene.ClassicRef { return scene.ClassicRef{Kind: scene.RItem, Name: "A", Nth: nth} }
	if got, ok := valu(at(0), "title"); !ok || len(got) != 1 || got[0] != "4" {
		t.Errorf("first A title %v, %v; want 4", got, ok)
	}
	if got, ok := valu(at(0), "color"); !ok || len(got) != 1 || got[0] != "3,2,3" {
		t.Errorf("first A color %v, %v; want 3,2,3", got, ok)
	}
	if got, ok := valu(at(0), "my_value"); !ok || len(got) != 1 || got[0] != "3" {
		t.Errorf("first A my_value %v, %v; want the literal 3", got, ok)
	}
	if got, ok := valu(at(1), "value"); !ok || len(got) != 1 || got[0] != "14" {
		t.Errorf("second A value %v, %v; want 14", got, ok)
	}
	root := scene.ClassicRef{Kind: scene.RItem, Name: "Root", Root: true}
	if got, ok := valu(root, "width"); !ok || len(got) != 1 || got[0] != "30" {
		t.Errorf("element width %v, %v; want 30", got, ok)
	}
	if _, ok := valu(at(2), "value"); ok {
		t.Errorf("a third A was found")
	}
	if _, ok := valu(at(0), "unknown"); ok {
		t.Errorf("an unknown property was found")
	}
}

// TestClassicValuesLink: a link is named by its ends, as the diagram writes them.
func TestClassicValuesLink(t *testing.T) {
	el := parseDiagram(t, `element Root {
    item A { pos 0 0 }
    item B { pos 100 0 }
    link A B { line-width ${2 * 3} }
}`)
	valu := ClassicValues(el, NewExecContext(el, testOpts()))
	got, ok := valu(scene.ClassicRef{Kind: scene.RLink, From: "A", To: "B"}, "line-width")
	if !ok || len(got) != 1 || got[0] != "6" {
		t.Errorf("link line-width %v, %v; want 6", got, ok)
	}
}
