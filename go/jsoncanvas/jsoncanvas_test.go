package jsoncanvas

import (
	"math"
	"os"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/tenniarb/go/scene"
)

func TestRoundTrip(t *testing.T) {
	data, err := os.ReadFile("testdata/example.canvas")
	if err != nil {
		t.Fatal(err)
	}
	in, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	text, warnings, err := ToTenn(in, "example")
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) == 0 {
		t.Errorf("the palette has no exact shade for 1..6, want warnings")
	}

	root := t.TempDir()
	tree, err := scene.LoadSource(root, "example.tenn", text)
	if err != nil {
		t.Fatalf("%v\ntext:\n%s", err, text)
	}
	if file := tree.File("example.tenn"); file == nil || len(file.Elems) != 1 {
		t.Fatalf("the imported text must hold exactly one element")
	}
	sc := gfx.NewScene()
	var screen *scene.Screen
	sc.Update(func(tx *gfx.Tx) {
		screen, err = tree.BuildElement(tx, nil, tree.File("example.tenn"), scene.Options{})
	})
	if err != nil {
		t.Fatalf("%v\ntext:\n%s", err, text)
	}
	if screen == nil {
		t.Fatal("no screen")
	}

	out, _, err := FromTree(tree)
	if err != nil {
		t.Fatal(err)
	}
	checkNodes(t, in, out)
	checkEdges(t, in, out)
	groups := 0
	for _, n := range out.Nodes {
		if n.Type != "group" {
			continue
		}
		groups++
		switch n.Label {
		case "Outer group", "Inner group", "Frame":
		default:
			t.Errorf("unexpected group %q: the wrapper element must not become a group", n.Label)
		}
	}
	if groups != 3 {
		t.Errorf("groups = %d, want the three of the canvas", groups)
	}
}

func checkNodes(t *testing.T, want, got *Canvas) {
	t.Helper()
	if len(got.Nodes) != len(want.Nodes) {
		t.Fatalf("nodes: got %d, want %d", len(got.Nodes), len(want.Nodes))
	}
	used := make([]bool, len(got.Nodes))
	for _, w := range want.Nodes {
		found := false
		for i, g := range got.Nodes {
			if used[i] || g.Type != w.Type || !near(g.X, w.X) || !near(g.Y, w.Y) ||
				!near(g.Width, w.Width) || !near(g.Height, w.Height) {
				continue
			}
			used[i], found = true, true
			if g.Color != w.Color {
				t.Errorf("node %q: color %q, want %q", w.ID, g.Color, w.Color)
			}
			switch w.Type {
			case "text":
				if g.Text != w.Text {
					t.Errorf("node %q: text %q, want %q", w.ID, g.Text, w.Text)
				}
			case "file":
				if g.File != w.File {
					t.Errorf("node %q: file %q, want %q", w.ID, g.File, w.File)
				}
			case "link":
				if g.URL != w.URL {
					t.Errorf("node %q: url %q, want %q", w.ID, g.URL, w.URL)
				}
			case "group":
				if g.Label != w.Label {
					t.Errorf("node %q: label %q, want %q", w.ID, g.Label, w.Label)
				}
			}
			break
		}
		if !found {
			t.Errorf("node %q (%s at %.2f,%.2f %.2fx%.2f) is missing", w.ID, w.Type, w.X, w.Y, w.Width, w.Height)
		}
	}
}

func checkEdges(t *testing.T, want, got *Canvas) {
	t.Helper()
	if len(got.Edges) != len(want.Edges) {
		t.Fatalf("edges: got %d, want %d", len(got.Edges), len(want.Edges))
	}
	byID := func(c *Canvas, id string) Node {
		for _, n := range c.Nodes {
			if n.ID == id {
				return n
			}
		}
		t.Fatalf("no node %q", id)
		return Node{}
	}
	key := func(c *Canvas, e Edge) string {
		a, b := byID(c, e.FromNode), byID(c, e.ToNode)
		return strings.Join([]string{a.Type, b.Type, e.FromSide, e.ToSide, e.ToEnd, e.Color, e.Label}, "|")
	}
	keys := map[string]int{}
	for _, e := range want.Edges {
		keys[key(want, e)]++
	}
	for _, e := range got.Edges {
		k := key(got, e)
		if keys[k] == 0 {
			t.Errorf("edge %q: %s is extra", e.ID, k)
			continue
		}
		keys[k]--
	}
	for k, n := range keys {
		if n > 0 {
			t.Errorf("edge %s is missing", k)
		}
	}
}

func near(a, b float64) bool { return math.Abs(a-b) <= 0.011 }

func TestParseErrors(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"bad json", `{`, "json canvas"},
		{"no id", `{"nodes":[{"type":"text"}]}`, "node 1 has no id"},
		{"unknown type", `{"nodes":[{"id":"a","type":"shape"}]}`, `node "a" has unknown type "shape"`},
		{"file without path", `{"nodes":[{"id":"a","type":"file"}]}`, `file node "a" has no file`},
		{"link without url", `{"nodes":[{"id":"a","type":"link"}]}`, `link node "a" has no url`},
		{"dangling edge", `{"nodes":[{"id":"a","type":"text"}],"edges":[{"id":"e","fromNode":"a","toNode":"b"}]}`, `edge "e": toNode "b" is not a node`},
		{"repeated node", `{"nodes":[{"id":"a","type":"text"},{"id":"a","type":"group"}]}`, `node "a" is repeated`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := Parse([]byte(c.src)); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error = %v, want %q", err, c.want)
			}
		})
	}
}

func TestParseIgnoresUnknownFields(t *testing.T) {
	c, err := Parse([]byte(`{"nodes":[{"id":"a","type":"text","future":1}],"unknown":{}}`))
	if err != nil || len(c.Nodes) != 1 {
		t.Fatalf("%v, %d nodes", err, len(c.Nodes))
	}
	if b, err := Marshal(c); err != nil || !strings.Contains(string(b), `"edges": []`) {
		t.Errorf("marshal = %s, %v", b, err)
	}
}

func TestExternalFileBecomesLink(t *testing.T) {
	text, warnings, err := ToTenn(&Canvas{Nodes: []Node{{
		ID: "a", Type: "file", File: "https://example.com/pic.png", Width: 10, Height: 10,
	}}}, "test")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(text, "image") || !strings.Contains(text, `[https://example.com/pic.png](https://example.com/pic.png)`) {
		t.Errorf("text:\n%s", text)
	}
	if len(warnings) == 0 {
		t.Error("want a warning about the external file")
	}
}

func TestCrossGroupEdgeWarns(t *testing.T) {
	c := &Canvas{
		Nodes: []Node{
			{ID: "g", Type: "group", X: 0, Y: 0, Width: 100, Height: 100},
			{ID: "in", Type: "text", X: 10, Y: 10, Width: 20, Height: 20, Text: "in"},
			{ID: "out", Type: "text", X: 200, Y: 10, Width: 20, Height: 20, Text: "out"},
		},
		Edges: []Edge{{ID: "e", FromNode: "in", ToNode: "out"}},
	}
	text, warnings, err := ToTenn(c, "test")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(text, "link") {
		t.Errorf("a cross-group edge must not become a link:\n%s", text)
	}
	found := false
	for _, w := range warnings {
		found = found || strings.Contains(w, "siblings only")
	}
	if !found {
		t.Errorf("warnings = %v", warnings)
	}
}
