package diff_test

import (
	"reflect"
	"testing"

	"github.com/haiodo/tenniarb/go/diff"
)

// compare compares two texts of the file x.tenn in a temporary workspace.
func compare(t *testing.T, oldText, newText string) *diff.Report {
	t.Helper()
	r, err := diff.Compare(t.TempDir(), "x.tenn", oldText, newText)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// statuses lists the nodes with a status, by path.
func statuses(r *diff.Report) map[string]diff.Status {
	out := map[string]diff.Status{}
	var walk func(n *diff.Node)
	walk = func(n *diff.Node) {
		if n.Status != "" {
			out[n.Path] = n.Status
		}
		for _, k := range n.Kids {
			walk(k)
		}
	}
	walk(r.Root)
	return out
}

const card = `element "Card" {
  width 240
  item Header {
    title "Header"
    item Mark { title "*" }
  }
  item Body { title "Body" }
}
`

func TestCompareUnchanged(t *testing.T) {
	r := compare(t, card, card)
	if r.Changed() {
		t.Fatalf("the same text reports changes: %v", statuses(r))
	}
	if len(r.Paths(diff.Changed)) != 0 {
		t.Fatalf("changed paths: %v", r.Paths(diff.Changed))
	}
}

func TestCompareChanged(t *testing.T) {
	r := compare(t, card, replace(t, card, "width 240", "width 200"))
	if got := statuses(r); !reflect.DeepEqual(got, map[string]diff.Status{"Card": diff.Changed}) {
		t.Fatalf("statuses %v", got)
	}
	props := r.Node("Card").Props
	want := []diff.Prop{{Name: "width", Old: "240", New: "200"}}
	if !reflect.DeepEqual(props, want) {
		t.Fatalf("props %+v, want %+v", props, want)
	}
}

func TestCompareNestedAndAddedRemoved(t *testing.T) {
	oldText := card
	newText := replace(t, card, `item Mark { title "*" }`, `item Mark { title "+" }`)
	newText = replace(t, newText, `  item Body { title "Body" }`, "  item Body { title \"Body\" }\n  item Footer { title \"Footer\" }")
	r := compare(t, oldText, newText)
	got := statuses(r)
	if got["Card.Header.Mark"] != diff.Changed || got["Card.Footer"] != diff.Added || len(got) != 2 {
		t.Fatalf("statuses %v", got)
	}
	if p := r.Node("Card.Header.Mark").Props; len(p) != 1 || p[0].Name != "title" || p[0].Old != "*" || p[0].New != "+" {
		t.Fatalf("nested props %+v", p)
	}

	// the same item taken out is removed, with the nested node under it
	r = compare(t, newText, oldText)
	got = statuses(r)
	if got["Card.Footer"] != diff.Removed || got["Card.Header.Mark"] != diff.Changed || len(got) != 2 {
		t.Fatalf("statuses after removal %v", got)
	}
}

func TestCompareMovedParent(t *testing.T) {
	oldText := `element "A" {
  item X { title "x" }
}
element "B" {
  item Y { title "y" }
}
`
	newText := `element "A" {
}
element "B" {
  item Y { title "y" }
  item X { title "x" }
}
`
	r := compare(t, oldText, newText)
	got := statuses(r)
	if got["B.X"] != diff.Moved || len(got) != 1 {
		t.Fatalf("statuses %v", got)
	}
	if n := r.Node("B.X"); n.Note != "from A" {
		t.Fatalf("note %q", n.Note)
	}
	// the text of the moved node differs from nothing: revert keeps it in place, as one edit
	if _, ok := r.Revert("B.X"); !ok {
		t.Fatal("the moved node cannot be reverted")
	}
}

func TestCompareMovedOrder(t *testing.T) {
	oldText := `element "A" {
  item One { title "1" }
  item Two { title "2" }
  item Three { title "3" }
}
`
	newText := `element "A" {
  item Three { title "3" }
  item One { title "1" }
  item Two { title "2" }
}
`
	r := compare(t, oldText, newText)
	got := statuses(r)
	if got["A.Three"] != diff.Moved || len(got) != 1 {
		t.Fatalf("statuses %v, want only A.Three moved", got)
	}
}

func TestCompareUseAndStyles(t *testing.T) {
	oldText := `element "Button" {
  color #ffffff
  styles {
    primary { color #6b3ad3; text-color #ffffff }
    secondary { color #000000 }
  }
}
element "Screen" {
  item Box {
    use Button as save
  }
}
`
	newText := replace(t, oldText, `    use Button as save`, "    use Button as save { use-style secondary }")
	r := compare(t, oldText, newText)
	got := statuses(r)
	if got["Screen.Box.save"] != diff.Changed {
		t.Fatalf("the use is not changed: %v", got)
	}
	if p := r.Node("Screen.Box.save").Props; len(p) != 1 || p[0].Name != "use-style" || p[0].New != "secondary" {
		t.Fatalf("use props %+v", p)
	}

	styled := replace(t, oldText, "primary { color #6b3ad3; text-color #ffffff }", "primary { color #111111; text-color #ffffff }")
	r = compare(t, oldText, styled)
	got = statuses(r)
	if got["Button"] != diff.Changed {
		t.Fatalf("a style change is not a change of the element: %v", got)
	}
	found := false
	for _, p := range r.Node("Button").Props {
		if p.Name == "style primary" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the style is not among the changed properties: %+v", r.Node("Button").Props)
	}
}

// apply runs an edit on the new text.
func apply(text string, e diff.Edit) string { return text[:e.Start] + e.New + text[e.End:] }

func TestRevertChanged(t *testing.T) {
	newText := replace(t, card, "width 240", "width 200")
	r := compare(t, card, newText)
	e, ok := r.Revert("Card")
	if !ok {
		t.Fatal("no revert")
	}
	if got := apply(newText, e); got != card {
		t.Fatalf("revert gives\n%s\nwant\n%s", got, card)
	}
	// a nested node reverts alone
	newText = replace(t, card, `title "Body"`, `title "Figure"`)
	r = compare(t, card, newText)
	e, ok = r.Revert("Card.Body")
	if !ok || apply(newText, e) != card {
		t.Fatalf("nested revert %+v\n%s", e, apply(newText, e))
	}
}

func TestRevertAddedAndRemoved(t *testing.T) {
	added := replace(t, card, `  item Body { title "Body" }`, "  item Body { title \"Body\" }\n  item Footer { title \"Footer\" }")
	r := compare(t, card, added)
	e, ok := r.Revert("Card.Footer")
	if !ok {
		t.Fatal("no revert of an added node")
	}
	if got := apply(added, e); got != card {
		t.Fatalf("removing the added node gives\n%q\nwant\n%q", got, card)
	}

	r = compare(t, added, card)
	e, ok = r.Revert("Card.Footer")
	if !ok {
		t.Fatal("no revert of a removed node")
	}
	if got := apply(card, e); got != added {
		t.Fatalf("writing the removed node back gives\n%q\nwant\n%q", got, added)
	}
	// the reverted text still parses and holds the node
	if r2 := compare(t, added, apply(card, e)); r2.Changed() {
		t.Fatalf("the restored text differs: %v", statuses(r2))
	}
}

// replace swaps the first occurrence of old in text.
func replace(t *testing.T, text, old, new string) string {
	t.Helper()
	for i := 0; i+len(old) <= len(text); i++ {
		if text[i:i+len(old)] == old {
			return text[:i] + new + text[i+len(old):]
		}
	}
	t.Fatalf("%q is not in the text", old)
	return ""
}
