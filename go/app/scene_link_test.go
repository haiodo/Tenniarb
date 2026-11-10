//go:build darwin || linux || windows

package app

import (
	"slices"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/scene"
)

// findElement is the node of element name in the file the tree was loaded from.
func findElement(t *testing.T, tree *scene.Tree, name string) *scene.Node {
	t.Helper()
	file := tree.File("e.tenn")
	if file == nil {
		t.Fatal("no file e.tenn")
	}
	for _, n := range append(slices.Clone(file.Elems), file.Content...) {
		if n.Kind == scene.KElement && n.Key == name {
			return n
		}
	}
	t.Fatalf("no element %q", name)
	return nil
}

// A link names its ends among the children of the item that holds it, so a name written twice needs the
// indexes: a file with `link Node Node { source-index 1; target-index 0 }` is read back that way.
func TestLinkIndexesOfRepeatedNames(t *testing.T) {
	const src = `element "E" {
  item "Node" { pos 0 0 }
  item "Box" { pos 0 100 }
  item "Node" { pos 100 0 }
}`
	tree, err := scene.LoadSource(t.TempDir(), "e.tenn", src)
	if err != nil {
		t.Fatal(err)
	}
	el := findElement(t, tree, "E")
	var nodes []*scene.Node
	for _, c := range el.Content {
		if c.Kind == scene.KItem {
			nodes = append(nodes, c)
		}
	}
	if len(nodes) != 3 {
		t.Fatalf("%d items, want 3", len(nodes))
	}
	if i, j, ok := linkIndexes(el, nodes[0], nodes[1]); !ok || i != 0 || j != 0 {
		t.Errorf("Node 0 -> Box: %d, %d, %v, want no indexes", i, j, ok)
	}
	i, j, ok := linkIndexes(el, nodes[0], nodes[2])
	if !ok || i != 0 || j != 1 {
		t.Errorf("Node 0 -> Node 1: %d, %d, %v, want 0 and 1", i, j, ok)
	}
	if i, j, ok := linkIndexes(el, nodes[2], nodes[0]); !ok || i != 1 || j != 0 {
		t.Errorf("Node 1 -> Node 0: %d, %d, %v, want 1 and 0", i, j, ok)
	}
}

// The command written for a link is the one the loader reads: `link A B`, and the indexes in the block.
func TestSceneLinkCommandText(t *testing.T) {
	if got := CommandText("link", "A", "B"); got != "link A B" {
		t.Errorf("plain link: %q", got)
	}
	cmd := CommandText("link", "A", "B") + " { " + CommandText("source-index", "1") + "; " +
		CommandText("target-index", "0") + " }"
	if !strings.Contains(cmd, "source-index 1") || !strings.Contains(cmd, "target-index 0") {
		t.Fatalf("link with indexes: %q", cmd)
	}
	tree, err := scene.LoadSource(t.TempDir(), "e.tenn", "element \"E\" {\n  item A { pos 0 0 }\n  item B { pos 9 0 }\n  "+cmd+"\n}\n")
	if err != nil {
		t.Fatalf("the file does not read: %v", err)
	}
	var link *scene.Node
	for _, c := range findElement(t, tree, "E").Content {
		if c.Kind == scene.KLink {
			link = c
		}
	}
	if link == nil || link.From != "A" || link.To != "B" {
		t.Fatalf("link %+v", link)
	}
}
