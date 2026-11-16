package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGoldensAreCurrent: every range golden is what this build makes of its fixture, so a change in
// go/scene that moves a range fails here, not by leaving the Swift stand green against an old one.
func TestGoldensAreCurrent(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "web", "fixtures", "scene")
	fixtures, err := filepath.Glob(filepath.Join(dir, "*.tenn"))
	if err != nil || len(fixtures) == 0 {
		t.Fatalf("fixtures under %s: %v", dir, err)
	}
	goldens := filepath.Join("..", "..", "..", "TenniarbTests", "Golden", "scene-ranges")
	for _, fixture := range fixtures {
		name := filepath.Base(fixture)
		golden := filepath.Join(goldens, strings.TrimSuffix(name, ".tenn")+".json")
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatalf("%s: %v (regenerate it from %s)", golden, err, name)
		}
		got, err := ranges(dir, name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s is stale: regenerate it from %s", golden, name)
		}
	}
}

// TestRangesCoverWholeCommands: End is after the last token of a command, the closing brace of its
// block included, so the text in a range is the command and nothing else.
func TestRangesCoverWholeCommands(t *testing.T) {
	const src = `element "E" {
    item Box {
        title "a } brace"
    }
}
`
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.tenn"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := ranges(root, "a.tenn")
	if err != nil {
		t.Fatal(err)
	}
	var d struct {
		Items []struct {
			Kind       string
			Path       string
			Start, End int
		}
	}
	if err := json.Unmarshal(out, &d); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"E":           src[:len(src)-1],
		"E.Box":       "item Box {\n        title \"a } brace\"\n    }",
		"E.Box.title": `title "a } brace"`,
	}
	for _, it := range d.Items {
		body, ok := want[it.Path]
		if !ok || it.Kind == "file" {
			continue
		}
		if got := src[it.Start:it.End]; got != body {
			t.Errorf("%s range = %q, want %q", it.Path, got, body)
		}
		delete(want, it.Path)
	}
	if len(want) > 0 {
		t.Errorf("not listed: %v\n%s", want, out)
	}
}
