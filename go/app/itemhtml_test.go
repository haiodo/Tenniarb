//go:build darwin || linux || windows

package app

import (
	htmlesc "html"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/model"
)

// words are the words of an HTML text, without the markup: what a copy of it must carry.
func words(raw string) []string {
	htmlText := htmlesc.UnescapeString(raw)
	var out []string
	depth := 0
	start := 0
	for i := 0; i < len(htmlText); i++ {
		switch htmlText[i] {
		case '<':
			if depth == 0 && i > start {
				out = append(out, htmlText[start:i])
			}
			depth++
		case '>':
			depth--
			start = i + 1
		}
	}
	if start < len(htmlText) {
		out = append(out, htmlText[start:])
	}
	var fields []string
	for _, s := range out {
		for _, w := range strings.Fields(s) {
			// a bullet or a dash of the markup carries no word: only the letters are compared
			if strings.IndexFunc(w, func(r rune) bool {
				return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
			}) >= 0 {
				fields = append(fields, w)
			}
		}
	}
	return fields
}

// The text of an item on the clipboard is built from what the canvas drew, not from the boxes of the old
// renderer. The markup may differ; the words must not: they are the text a reader copies.
func TestItemHTMLKeepsTheWords(t *testing.T) {
	data, err := os.ReadFile("../../docs/Example.tenn")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	work := filepath.Join(dir, "Example.tenn")
	if err := os.WriteFile(work, data, 0o644); err != nil {
		t.Fatal(err)
	}
	m := model.ReadTenn(string(data))
	var el *model.Element
	var walk func(*model.Element)
	walk = func(e *model.Element) {
		if e.Name == "Basic steps" {
			el = e
		}
		for _, c := range e.Elements {
			walk(c)
		}
	}
	walk(&m.Element)
	if el == nil {
		t.Fatal("no element Basic steps")
	}
	v := newDoc(t, &Shell{dir: dir}, work, string(data))
	v.session.SetElement(el)
	v.state.Current = el
	sv := newSceneView(el, v.elementNode(el))
	sv.classic = true
	v.scene = gfx.NewScene()
	if err := v.buildSceneView(sv); err != nil {
		t.Fatal(err)
	}
	v.sv = sv
	sv.index(sv.placed)

	checked := 0
	for _, item := range el.Items {
		if item.Kind != model.ItemKindItem {
			continue
		}
		got := v.itemHTML(item)
		if got == "" {
			continue // nothing drawn for it
		}
		checked++
		if len(words(got)) == 0 {
			t.Errorf("%q: no words in %q", item.Name, got)
		}
	}
	if checked < 3 {
		t.Fatalf("%d items exported, want more", checked)
	}
	// An item whose title is its name says that name in the export.
	first := el.Items[0]
	got := v.itemHTML(first)
	have := map[string]bool{}
	for _, x := range words(got) {
		have[x] = true
	}
	for _, x := range words(first.Name) {
		if !have[x] {
			t.Errorf("%q: %q misses the word %q of its name", first.Name, got, x)
			break
		}
	}
}
