//go:build darwin || linux || windows

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/model"
)

// The element tree shows the mounted file, and edits never write its content into the document.
func TestFromFileDocument(t *testing.T) {
	dir := t.TempDir()
	lib := "element \"Parts\" {\n  item Button { width 40; height 20 }\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "kit.lib.tenn"), []byte(lib), 0o644); err != nil {
		t.Fatal(err)
	}
	text := "element \"kit\" --from-file \"kit.lib.tenn\"\nelement \"Map\" {\n    item \"Idea\" {\n        pos 0 0\n    }\n}\n"
	v := newDoc(t, &Shell{}, filepath.Join(dir, "main.tenn"), text)
	kit := v.session.Root().Elements[0]
	if len(kit.Elements) != 1 || kit.Elements[0].Name != "Parts" {
		t.Fatalf("the mounted element has no content: %+v", kit.Elements)
	}
	v.session.RenameElement(v.session.Root().Elements[1], "Map 2")
	if !strings.Contains(v.text, `element "Map 2"`) || !strings.Contains(v.text, `--from-file`) ||
		strings.Contains(v.text, "Parts") || strings.Contains(v.text, "Button") {
		t.Fatalf("a classic edit inlined or lost the library line:\n%s", v.text)
	}
	if v.session.Root().Elements[0] != kit || len(kit.Elements) != 1 {
		t.Error("an edit elsewhere must keep the mounted elements")
	}
	if got := model.KindOfModel(v.session.Root()); got == model.DocDiagram {
		t.Errorf("kind %s", got)
	}
	ver := v.version
	v.session.RenameElement(kit.Elements[0], "Other")
	if v.version != ver || kit.Elements[0].Name != "Parts" {
		t.Error("an edit inside the mounted file must be refused")
	}
}
