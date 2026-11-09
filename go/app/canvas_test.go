//go:build darwin || linux || windows

package app

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/jsoncanvas"
	"github.com/haiodo/tenniarb/go/scene"
)

func readCanvas(t *testing.T, path string) *jsoncanvas.Canvas {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	c, err := jsoncanvas.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// copyCanvas puts the package fixture in dir under name.
func copyCanvas(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("../jsoncanvas/testdata", "example.canvas"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestLoadCanvas: a .canvas file is read as the text of a .tenn document that keeps its path and
// becomes the same canvas again through the scene.
func TestLoadCanvas(t *testing.T) {
	dir := t.TempDir()
	path := copyCanvas(t, dir, "example.canvas")
	doc, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Model == nil {
		t.Fatalf("parse: %v", doc.Errors)
	}
	if doc.Path != path {
		t.Errorf("path %q, want %q", doc.Path, path)
	}
	if !strings.HasPrefix(doc.Text, "element example {") {
		t.Errorf("text does not start with the named element:\n%s", doc.Text)
	}
	if len(doc.Notes) == 0 {
		t.Error("want the notes of the conversion")
	}
	tree, err := scene.LoadSource(dir, "example.canvas", doc.Text)
	if err != nil {
		t.Fatalf("%v\ntext:\n%s", err, doc.Text)
	}
	out, _, err := jsoncanvas.FromTree(tree)
	if err != nil {
		t.Fatal(err)
	}
	checkCanvas(t, readCanvas(t, path), out)
}

// TestSaveCanvas: writeFile on a .canvas path writes the canvas, not the text, and the document
// keeps that path.
func TestSaveCanvas(t *testing.T) {
	dir := t.TempDir()
	path := copyCanvas(t, dir, "example.canvas")
	doc, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	v := newDocFrom(t, &Shell{dir: t.TempDir()}, doc)
	saved := v.text
	dst := filepath.Join(dir, "out.canvas")
	if err := v.writeFile(dst); err != nil {
		t.Fatal(err)
	}
	if v.text != saved {
		t.Error("saving to .canvas changed the text of the document")
	}
	if v.state.Doc.Path != dst {
		t.Errorf("path %q, want %q", v.state.Doc.Path, dst)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "element") {
		t.Fatalf("a .tenn text was written into the canvas:\n%s", data)
	}
	out, err := jsoncanvas.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	checkCanvas(t, readCanvas(t, path), out)
}

// TestCanvasAutosaveWritesCanvas: the autosave goes through the same writeFile and writes a canvas.
func TestCanvasAutosaveWritesCanvas(t *testing.T) {
	dir := t.TempDir()
	src := copyCanvas(t, dir, "example.canvas")
	doc, err := Load(src)
	if err != nil {
		t.Fatal(err)
	}
	v := newDocFrom(t, &Shell{dir: t.TempDir()}, doc)
	edited := strings.Replace(v.text, "title Notes", "title Notes!", 1)
	if edited == v.text {
		t.Fatal("the fixture has no Notes title")
	}
	if err := v.applyText(0, len(v.text), edited); err != nil {
		t.Fatal(err)
	}
	v.autosaveNow()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	c, err := jsoncanvas.Parse(data)
	if err != nil {
		t.Fatalf("%v\n%s", err, data)
	}
	found := false
	for _, n := range c.Nodes {
		found = found || n.Type == "text" && n.Text == "Notes!\n\nA **markdown** body with a `code` span."
	}
	if !found {
		t.Errorf("the autosaved canvas has no edited node:\n%s", data)
	}
}

// TestCanvasNotesGroupsElements: a single element warns about nothing, two elements do.
func TestCanvasNotesGroupsElements(t *testing.T) {
	dir := t.TempDir()
	doc, err := Load(copyCanvas(t, dir, "example.canvas"))
	if err != nil {
		t.Fatal(err)
	}
	v := newDocFrom(t, &Shell{dir: t.TempDir()}, doc)
	if notes, ok := v.canvasNotes(); ok {
		t.Errorf("one element warns: %q", notes)
	}

	two := ParseText("element \"A\" { item X { pos 0 0 } }\nelement \"B\" { item Y { pos 10 10 } }\n")
	two.Path = filepath.Join(dir, "two.canvas")
	w := newDocFrom(t, &Shell{dir: t.TempDir()}, two)
	notes, ok := w.canvasNotes()
	if !ok || !strings.Contains(notes, "group") {
		t.Errorf("two elements: ok=%v notes=%q", ok, notes)
	}
}

// checkCanvas compares the nodes by type and box and the edges by the ends they join, with the
// tolerance of the format.
func checkCanvas(t *testing.T, want, got *jsoncanvas.Canvas) {
	t.Helper()
	if len(got.Nodes) != len(want.Nodes) {
		t.Fatalf("nodes: got %d, want %d", len(got.Nodes), len(want.Nodes))
	}
	used := make([]bool, len(got.Nodes))
	for _, w := range want.Nodes {
		found := false
		for i, g := range got.Nodes {
			if used[i] || g.Type != w.Type || !near2(g.X, w.X) || !near2(g.Y, w.Y) ||
				!near2(g.Width, w.Width) || !near2(g.Height, w.Height) {
				continue
			}
			used[i], found = true, true
			if g.Color != w.Color {
				t.Errorf("node %q: colour %q, want %q", w.ID, g.Color, w.Color)
			}
			if g.Text != w.Text || g.File != w.File || g.URL != w.URL || g.Label != w.Label {
				t.Errorf("node %q: content changed", w.ID)
			}
			break
		}
		if !found {
			t.Errorf("node %q (%s at %.2f,%.2f %.2fx%.2f) is missing", w.ID, w.Type, w.X, w.Y, w.Width, w.Height)
		}
	}
	key := func(c *jsoncanvas.Canvas, e jsoncanvas.Edge) string {
		typeOf := func(id string) string {
			for _, n := range c.Nodes {
				if n.ID == id {
					return n.Type
				}
			}
			return "?"
		}
		return fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s", typeOf(e.FromNode), typeOf(e.ToNode),
			e.FromSide, e.ToSide, e.FromEnd, e.ToEnd, e.Color)
	}
	keys := map[string][]string{}
	for _, e := range want.Edges {
		keys[key(want, e)] = append(keys[key(want, e)], e.Label)
	}
	for _, e := range got.Edges {
		k := key(got, e)
		labels := keys[k]
		if len(labels) == 0 {
			t.Errorf("edge %q: %s is extra", e.ID, k)
			continue
		}
		if labels[0] != e.Label {
			t.Errorf("edge %q: label %q, want %q", e.ID, e.Label, labels[0])
		}
		keys[k] = labels[1:]
	}
	for k, labels := range keys {
		if len(labels) > 0 {
			t.Errorf("edge %s (%d) is missing", k, len(labels))
		}
	}
}

func near2(a, b float64) bool { return math.Abs(a-b) <= 0.011 }
