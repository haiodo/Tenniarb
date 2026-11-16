package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

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
