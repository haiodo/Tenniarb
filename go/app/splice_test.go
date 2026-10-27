//go:build darwin || linux || windows

package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/model"
)

// A hub file: classic diagrams and UI screens together, the screens in compact form with an element
// level use before an item, which the Swift printer would move.
const hubDoc = "item Top { auto-layout row; item A { title a }; use kit.Button as ok }\n" +
	"element \"Map\" {\n    item \"Idea\" {\n        pos 0 0\n    }\n    item \"Plan\" {\n        pos 120 0\n    }\n" +
	"    link \"Idea\" \"Plan\"\n    element \"Notes\" {\n        item \"N\"\n    }\n}\n" +
	"element \"Screens\" {\n  item Bg { pos 0 0; width 300; height 200; color #eeeeee }\n  use kit.Button as ok\n" +
	"  item Login { pos 0 0; auto-layout column; item Title { title Hi } }\n" +
	"  element Inner { item Card { auto-layout row; item T { title x } } }\n}\n" +
	"element kit --from-file kit.tenn\nelement Empty\n"

// cmdText is the source of the command that starts with header, "" for the file's own lines.
func cmdText(text, header string) string {
	if header == "" {
		var out string
		for _, l := range strings.SplitAfter(text, "\n") {
			if strings.HasPrefix(l, "element kit ") || strings.HasPrefix(l, "item ") {
				out += l
			}
		}
		return out
	}
	i := strings.Index(text, header)
	if i < 0 {
		return ""
	}
	depth, j := 0, i
	for ; j < len(text); j++ {
		if text[j] == '{' {
			depth++
		} else if text[j] == '}' {
			if depth--; depth == 0 {
				break
			}
		}
	}
	return text[i:min(j+1, len(text))]
}

func pngOf(t *testing.T, v *Viewer, e *model.Element) []byte {
	t.Helper()
	v.session.SetElement(e)
	png, err := v.renderPNG(1)
	if err != nil {
		t.Fatal(err)
	}
	return png
}

func TestClassicEditKeepsSceneBytes(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "kit.tenn"), []byte("item Button { width 40; height 20; color #3366ff }\n"), 0o644)
	sh := &Shell{}
	v := newDoc(t, sh, filepath.Join(dir, "hub.tenn"), hubDoc)
	if v.kind != model.DocMixed || v.text != hubDoc {
		t.Fatalf("kind %s\n%s", v.kind, v.text)
	}
	root := &v.session.Root().Element
	mapEl, screens := root.Elements[0], root.Elements[1]
	want := map[string]string{}
	for _, h := range []string{"", "element \"Screens\" {", "element Inner {"} {
		want[h] = cmdText(hubDoc, h)
	}
	wantRoot, wantScreens := pngOf(t, v, root), pngOf(t, v, screens)

	check := func(step string) {
		t.Helper()
		for k, b := range want {
			if cmdText(v.text, k) != b {
				t.Errorf("%s: %q changed:\n%s", step, k, v.text)
			}
		}
		if got := model.ToTennStr(&ParseText(v.text).Model.Element, true, true); got != v.session.Text() {
			t.Errorf("%s: the text does not read back as the model:\n%s\nmodel:\n%s", step, v.text, v.session.Text())
		}
		if !bytes.Equal(pngOf(t, v, root), wantRoot) || !bytes.Equal(pngOf(t, v, screens), wantScreens) {
			t.Errorf("%s: a scene renders differently", step)
		}
	}
	steps := []struct {
		name string
		do   func()
		has  string
	}{
		{"move item", func() {
			v.session.SetElement(mapEl)
			v.session.Select(mapEl.Items[:1])
			v.session.MoveBy(10, 0)
		}, "pos 120.0 0.0"},
		{"add item", func() { v.session.SetElement(mapEl); v.session.AddTopItem() }, "item \"Untitled 1\""},
		{"rename element", func() { v.session.RenameElement(mapEl, "Map 2") }, "element \"Map 2\""},
		{"rename a scene element", func() { v.session.RenameElement(screens, "Screens 2") }, "element \"Screens 2\" {\n  item Bg"},
		{"add element", func() { v.session.AddElement(nil) }, "element Empty\nelement \"Unnamed element"},
		{"add sub-element to a scene", func() { v.session.AddElement(screens) }, "item T { title x } } }\n  element \"Unnamed"},
		{"move element", func() { v.session.MoveElement(mapEl.Elements[0], root, 0) }, "item \"N\"\n    }\nelement \"Map"},
		{"remove element", func() { v.session.RemoveElement(root.Elements[len(root.Elements)-2]) }, ""},
	}
	var texts []string
	for _, s := range steps {
		texts = append(texts, v.text)
		s.do()
		if v.text == texts[len(texts)-1] || !strings.Contains(v.text, s.has) {
			t.Fatalf("%s:\n%s", s.name, v.text)
		}
		// the two steps that edit a scene element change only its name or add one line before its brace
		switch s.name {
		case "rename a scene element":
			old := want["element \"Screens\" {"]
			delete(want, "element \"Screens\" {")
			want["element \"Screens 2\" {"] = strings.Replace(old, "Screens", "Screens 2", 1)
		case "add sub-element to a scene":
			old := want["element \"Screens 2\" {"]
			want["element \"Screens 2\" {"] = old[:len(old)-1] + "  element \"Unnamed element: 1\" {\n    }\n}"
		}
		check(s.name)
	}
	after := v.text
	for i := len(steps) - 1; i >= 0; i-- {
		v.session.Undo()
		if v.text != texts[i] {
			t.Fatalf("undo %s:\n%s\nwant:\n%s", steps[i].name, v.text, texts[i])
		}
	}
	if v.isDirty() {
		t.Error("dirty after undoing everything")
	}
	for range steps {
		v.session.Redo()
	}
	if v.text != after {
		t.Fatalf("redo:\n%s\nwant:\n%s", v.text, after)
	}
}

// A diagram document stays on the Swift printer: a classic edit writes the whole model.
func TestClassicEditPrintsDiagram(t *testing.T) {
	data, err := os.ReadFile("../../web/fixtures/Example.tenn")
	if err != nil {
		t.Fatal(err)
	}
	v := newDoc(t, &Shell{}, "", string(data))
	if v.kind != model.DocDiagram || v.text != v.session.Text() {
		t.Fatalf("kind %s", v.kind)
	}
	el := v.session.Root().Elements[0]
	v.session.SetElement(el)
	v.session.AddTopItem()
	v.session.RenameElement(el, "Renamed")
	if v.text != v.session.Text() {
		t.Error("a diagram document is not the Swift print")
	}
}
