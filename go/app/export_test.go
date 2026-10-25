package app

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/model"
)

func exampleElement(t *testing.T) (*model.ElementModel, *model.Element) {
	t.Helper()
	text, err := os.ReadFile("../../web/fixtures/Example.tenn")
	if err != nil {
		t.Fatal(err)
	}
	doc := ParseText(string(text))
	if doc.Model == nil {
		t.Fatal(doc.Errors)
	}
	e := Resolve(&doc.Model.Element, []string{"Documentation", "Basic steps"})
	if e == nil {
		t.Fatal("no element")
	}
	return doc.Model, e
}

func TestExportFormats(t *testing.T) {
	_, e := exampleElement(t)
	p := DefaultPrefs()

	png, err := ExportPNG(e, p, 1)
	if err != nil || !bytes.HasPrefix(png, []byte("\x89PNG")) {
		t.Fatalf("png: %v", err)
	}
	svg, err := ExportSVG(e, p)
	if err != nil || !strings.Contains(string(svg), "<svg") || !strings.Contains(string(svg), "Basic editing functions") {
		t.Fatalf("svg: %v", err)
	}
	// the background is the export setting
	withBg := string(svg)
	p.ExportBackground = false
	plain, _ := ExportSVG(e, p)
	if len(withBg) <= len(plain) {
		t.Fatal("the background rectangle is missing")
	}

	html, err := ExportHTML(e, DefaultPrefs(), 1)
	if err != nil {
		t.Fatal(err)
	}
	_, b64, ok := strings.Cut(string(html), "base64,")
	if !ok {
		t.Fatal("no inline image")
	}
	raw, err := base64.StdEncoding.DecodeString(b64[:strings.Index(b64, "\"")])
	if err != nil || !bytes.HasPrefix(raw, []byte("\x89PNG")) {
		t.Fatalf("inline image: %v", err)
	}
}

func TestInteractiveHTML(t *testing.T) {
	m, e := exampleElement(t)
	out := string(InteractiveHTML("/*bundle*/", "element \"A\" {\n}\n", e))
	if !strings.Contains(out, "<script>/*bundle*/</script>") || !strings.Contains(out, `data-element="Documentation/Basic steps"`) {
		t.Fatalf("page:\n%s", out)
	}
	want := base64.StdEncoding.EncodeToString([]byte("element \"A\" {\n}\n"))
	if !strings.Contains(out, ">"+want+"</script>") {
		t.Fatal("document text is not embedded as base64")
	}
	quoted := &model.Element{Name: `A "b" <c> & d`}
	if out := string(InteractiveHTML("", "", quoted)); !strings.Contains(out, "A &quot;b&quot; &lt;c> &amp; d") {
		t.Fatalf("title is not escaped:\n%s", out)
	}
	_ = m
}

func TestFindEmbedBundle(t *testing.T) {
	t.Setenv("TENNIARB_EMBED", "")
	dir := t.TempDir()
	if _, err := FindEmbedBundle(filepath.Join(dir, "bin", "tenniarb"), filepath.Join(dir, "cfg")); err != ErrNoEmbedBundle {
		t.Fatalf("nothing there: %v", err)
	}
	res := filepath.Join(dir, "Resources")
	os.MkdirAll(res, 0o755)
	os.MkdirAll(filepath.Join(dir, "MacOS"), 0o755)
	os.WriteFile(filepath.Join(res, EmbedBundleName), []byte("x"), 0o644)
	got, err := FindEmbedBundle(filepath.Join(dir, "MacOS", "tenniarb"), "")
	if err != nil || filepath.Clean(got) != filepath.Join(dir, "MacOS", "..", "Resources", EmbedBundleName) && filepath.Clean(got) != filepath.Join(res, EmbedBundleName) {
		t.Fatalf("app bundle: %q %v", got, err)
	}
	t.Setenv("TENNIARB_EMBED", filepath.Join(res, EmbedBundleName))
	if got, err = FindEmbedBundle("", ""); err != nil || got != filepath.Join(res, EmbedBundleName) {
		t.Fatalf("env: %q %v", got, err)
	}
}

func TestExportName(t *testing.T) {
	if got := ExportName(&model.Element{Name: "a/b:c"}, ".png"); got != "a_b_c.png" {
		t.Fatal(got)
	}
	if got := ExportName(&model.Element{}, ".svg"); got != "Untitled.svg" {
		t.Fatal(got)
	}
}
