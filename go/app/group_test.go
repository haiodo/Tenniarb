package app

import (
	"testing"

	"github.com/haiodo/tenniarb/go/scene"
)

// groupDoc is a scene of four items placed at pos, the two to wrap first and a sibling between them.
const groupDoc = "element S {\n" +
	"\titem A { pos 10 10; width 40; height 20 }\n" +
	"\titem Keep { pos 0 -200; width 10; height 10 }\n" +
	"\titem B { pos 100 -50; width 30; height 30 }\n" +
	"}\n"

func parseGroupDoc(t *testing.T, text string) (*scene.Tree, *scene.Node) {
	t.Helper()
	tree, err := scene.LoadSource(t.TempDir(), "s.tenn", text)
	if err != nil {
		t.Fatal(err)
	}
	f := tree.File("s.tenn")
	if f == nil || len(f.Elems) != 1 {
		t.Fatalf("no element in\n%s", text)
	}
	return tree, f.Elems[0]
}

func nodeIn(t *testing.T, root *scene.Node, key string) *scene.Node {
	t.Helper()
	if n := NodeAt(root, key); n != nil {
		return n
	}
	t.Fatalf("no %q in\n%s", key, root.Key)
	return nil
}

func posIn(t *testing.T, n *scene.Node) (x, y float64) {
	t.Helper()
	pr := ownProp(n, "pos")
	if pr == nil {
		return 0, 0
	}
	v := pr.Values()
	if len(v) != 2 {
		t.Fatalf("%s: pos %v", n.Key, v)
	}
	return parseNum(v[0]), parseNum(v[1])
}

// TestGroupEdit: two items become one at the corner of their box, keep their place inside it, and a
// sibling written between them stays outside the group.
func TestGroupEdit(t *testing.T) {
	_, el := parseGroupDoc(t, groupDoc)
	a, b := nodeIn(t, el, "A"), nodeIn(t, el, "B")
	items := []GroupItem{
		{Node: a, X: 10, Y: 10, W: 40, H: 20},
		{Node: b, X: 100, Y: -50, W: 30, H: 30},
	}
	e, at, ok := GroupEdit(groupDoc, items, "Group")
	if !ok {
		t.Fatal("refused")
	}
	text := groupDoc[:e.Start] + e.New + groupDoc[e.End:]
	_, el2 := parseGroupDoc(t, text)
	if len(sceneKids(el2)) != 2 {
		t.Fatalf("the element holds %d nodes:\n%s", len(sceneKids(el2)), text)
	}
	g := nodeIn(t, el2, "Group")
	if x, y := posIn(t, g); x != 10 || y != 10 {
		t.Fatalf("group at %v,%v:\n%s", x, y, text)
	}
	if x, y := posIn(t, nodeIn(t, g, "A")); x != 0 || y != 0 {
		t.Fatalf("A at %v,%v:\n%s", x, y, text)
	}
	if x, y := posIn(t, nodeIn(t, g, "B")); x != 90 || y != -60 {
		t.Fatalf("B at %v,%v:\n%s", x, y, text)
	}
	if k := nodeIn(t, el2, "Keep"); k == nil {
		t.Fatalf("the sibling between them is gone:\n%s", text)
	}
	if at < 0 || nodeStartingAt(el2, at) == nil || nodeStartingAt(el2, at).Key != "Group" {
		t.Fatalf("the group does not start at %d:\n%s", at, text)
	}

	// ungroup gives the places back
	u, _, ok := UngroupEdit(text, g)
	if !ok {
		t.Fatal("ungroup refused")
	}
	text2 := text[:u.Start] + u.New + text[u.End:]
	_, el3 := parseGroupDoc(t, text2)
	if NodeAt(el3, "Group") != nil {
		t.Fatalf("the group stayed:\n%s", text2)
	}
	if x, y := posIn(t, nodeIn(t, el3, "A")); x != 10 || y != 10 {
		t.Fatalf("A came back at %v,%v:\n%s", x, y, text2)
	}
	if x, y := posIn(t, nodeIn(t, el3, "B")); x != 100 || y != -50 {
		t.Fatalf("B came back at %v,%v:\n%s", x, y, text2)
	}
}

// TestGroupEditRefuses: nodes of different parents are no group, and one item alone is none either.
func TestGroupEditRefuses(t *testing.T) {
	text := "element S {\n\titem P { auto-layout abs\n\t\titem A { pos 0 0; width 10; height 10 }\n\t}\n\titem B { pos 0 0; width 10; height 10 }\n}\n"
	_, el := parseGroupDoc(t, text)
	a := nodeIn(t, nodeIn(t, el, "P"), "A")
	b := nodeIn(t, el, "B")
	if _, _, ok := GroupEdit(text, []GroupItem{{Node: a}, {Node: b}}, "Group"); ok {
		t.Fatal("grouped items of two parents")
	}
	if _, _, ok := GroupEdit(text, []GroupItem{{Node: b}}, "Group"); ok {
		t.Fatal("grouped one item")
	}
}
