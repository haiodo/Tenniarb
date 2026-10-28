package scene_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/scene"
)

// refList prints the references as file:line with the argument as written.
func refList(t *testing.T, tree *scene.Tree, refs []scene.Ref) []string {
	t.Helper()
	var out []string
	for _, r := range refs {
		src, _ := tree.Source(r.File)
		got := src[r.Arg.Start:r.Arg.End]
		out = append(out, fmt.Sprintf("%s:%d %s", r.File, strings.Count(src[:r.Arg.Start], "\n")+1, got))
	}
	return out
}

func wantRefs(t *testing.T, tree *scene.Tree, refs []scene.Ref, want ...string) {
	t.Helper()
	if got := refList(t, tree, refs); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("references\n got %q\nwant %q", got, want)
	}
}

func mustResolve(t *testing.T, tree *scene.Tree, scope, path string) *scene.Node {
	t.Helper()
	n, err := tree.Resolve(tree.File(scope), path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return n
}

// A path counts when it goes through the element, short or full, inside the mounted file or outside
// it. The same name in another scope is left alone.
func TestReferencesToElement(t *testing.T) {
	root := writeFiles(t, map[string]string{
		"kit.tenn": "element Parts {\n  item Button { width 40 }\n}\nelement Demo {\n  use Parts.Button as a\n}\n",
		"main.tenn": "element kit --from-file kit.tenn\n" +
			"element Screen {\n  use kit.Parts.Button as b\n  use kit.Demo.a as c\n}\n" +
			"element Other {\n  item Parts { }\n  use Parts as p\n}\n",
	})
	tree, err := scene.Load(root, "main.tenn")
	if err != nil {
		t.Fatal(err)
	}
	before := fmt.Sprint(tree.Diagnostics())
	parts := mustResolve(t, tree, "main.tenn", "kit.Parts")
	refs, blocked := tree.References(parts)
	wantRefs(t, tree, refs, "kit.tenn:5 Parts.Button", "main.tenn:3 kit.Parts.Button")
	if blocked != "" || refs[0].Seg != 0 || refs[1].Seg != 1 {
		t.Errorf("blocked %q, segments %d %d", blocked, refs[0].Seg, refs[1].Seg)
	}
	other := mustResolve(t, tree, "main.tenn", "Other.Parts")
	refs, _ = tree.References(other)
	wantRefs(t, tree, refs, "main.tenn:8 Parts")
	alias := mustResolve(t, tree, "main.tenn", "kit")
	refs, _ = tree.References(alias)
	wantRefs(t, tree, refs, "main.tenn:3 kit.Parts.Button", "main.tenn:4 kit.Demo.a")
	if after := fmt.Sprint(tree.Diagnostics()); after != before {
		t.Errorf("finding references changed the diagnostics:\n%s\n%s", before, after)
	}
}

// A part opened on its own is loaded in the context of its entry: the references in the entry and in
// the other parts are found from there.
func TestReferencesThroughEntry(t *testing.T) {
	root := writeFiles(t, map[string]string{
		"design.tenn":     "element ui --from-file ui.tenn\nelement p --from-file \"apps/part.tenn\"\nelement q --from-file \"apps/other.tenn\"\n",
		"ui.tenn":         "item Button { width 40 }\n",
		"apps/part.tenn":  "element Screen { use ui.Button as b }\n",
		"apps/other.tenn": "element Screen2 { use ui.Button as b }\n",
	})
	tree, err := scene.LoadIn(root, "apps/part.tenn", "design.tenn", nil)
	if err != nil {
		t.Fatal(err)
	}
	btn := mustResolve(t, tree, "design.tenn", "ui.Button")
	refs, _ := tree.References(btn)
	wantRefs(t, tree, refs, "apps/other.tenn:1 ui.Button", "apps/part.tenn:1 ui.Button")
}

const componentText = `element S {
  item Card { item title { }; item body { item line { } } }
  use Card as c1 { at title { color red }; at body.line { color blue } }
  use Card as c2 { replace body.line { item other { } } }
  use c1 as c3 { at body { }; at title { } }
  item A { }
  item B { }
  link A B
  link "B" A
}
`

// at and replace are followed through the instances; a use is a component too.
func TestReferencesThroughInstances(t *testing.T) {
	root := writeFiles(t, map[string]string{"m.tenn": componentText})
	tree, err := scene.Load(root, "m.tenn")
	if err != nil {
		t.Fatal(err)
	}
	title := mustResolve(t, tree, "m.tenn", "S.Card.title")
	refs, _ := tree.References(title)
	wantRefs(t, tree, refs, "m.tenn:3 title", "m.tenn:5 title")
	line := mustResolve(t, tree, "m.tenn", "S.Card.body.line")
	refs, _ = tree.References(line)
	wantRefs(t, tree, refs, "m.tenn:3 body.line", "m.tenn:4 body.line")
	if refs[0].Kind != scene.RefAt || refs[1].Kind != scene.RefReplace || refs[0].Seg != 1 {
		t.Errorf("kinds %v %v, segment %d", refs[0].Kind, refs[1].Kind, refs[0].Seg)
	}
	c1 := mustResolve(t, tree, "m.tenn", "S.c1")
	refs, _ = tree.References(c1)
	wantRefs(t, tree, refs, "m.tenn:5 c1")
	card := mustResolve(t, tree, "m.tenn", "S.Card")
	refs, _ = tree.References(card)
	wantRefs(t, tree, refs, "m.tenn:3 Card", "m.tenn:4 Card")
}

// A link end names a sibling by its name; with several of that name the rename is blocked.
func TestReferencesLinkEnds(t *testing.T) {
	root := writeFiles(t, map[string]string{"m.tenn": componentText + "element T {\n  item X { }\n  item X { }\n  link X X { target-index 1 }\n  item Y { }\n  link Y Y\n}\n"})
	tree, err := scene.Load(root, "m.tenn")
	if err != nil {
		t.Fatal(err)
	}
	a := mustResolve(t, tree, "m.tenn", "S.A")
	refs, blocked := tree.References(a)
	wantRefs(t, tree, refs, "m.tenn:8 A", "m.tenn:9 A")
	if blocked != "" || refs[0].Kind != scene.RefLink || refs[0].Seg != -1 {
		t.Errorf("blocked %q, %+v", blocked, refs[0])
	}
	b := mustResolve(t, tree, "m.tenn", "S.B")
	refs, _ = tree.References(b)
	wantRefs(t, tree, refs, "m.tenn:8 B", `m.tenn:9 "B"`)
	x := mustResolve(t, tree, "m.tenn", "T").Nodes()[0]
	if _, blocked = tree.References(x); !strings.Contains(blocked, "several") {
		t.Errorf("an indexed link end must block the rename, got %q", blocked)
	}
	y := mustResolve(t, tree, "m.tenn", "T.Y")
	refs, blocked = tree.References(y)
	if len(refs) != 2 || blocked != "" {
		t.Errorf("both ends of the link Y Y: %d, blocked %q", len(refs), blocked)
	}
}
