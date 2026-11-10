//go:build darwin || linux || windows

package app

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/tenn"
)

// pictureData is base64 of n bytes: neither the parser nor the Code section reads the picture, so a
// test needs only the shape of the data.
func pictureData(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i*7 + 3)
	}
	return base64.StdEncoding.EncodeToString(b)
}

// imageDoc is an item that writes the classic inline image line, as the Code section shows it for
// the item of a scene element.
func imageDoc(line string) string {
	return "item \"Pic\" {\n  width 100\n  height 50\n  " + line + "\n  title Hi\n}\n"
}

// closeImageLine makes a shortened line whole: a golden file may carry the head of the base64 only,
// with the paren that ends it cut away.
func closeImageLine(line string) string {
	s := strings.TrimRight(line, " \t")
	if strings.HasSuffix(s, ")") {
		return s
	}
	return s + ")"
}

// inlineData is the data of an image line between @( and the paren that ends it.
func inlineData(line string) string {
	i := strings.Index(line, "@(")
	j := strings.LastIndex(line, ")")
	if i < 0 || j <= i {
		return ""
	}
	return line[i+2 : j]
}

// base64Run is the longest run of base64 characters in s: data shows up as a long one, a stub and
// its words do not.
func base64Run(s string) int {
	is := func(c byte) bool {
		return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '+' || c == '/' || c == '='
	}
	best, n := 0, 0
	for i := 0; i < len(s); i++ {
		if is(s[i]) {
			n++
			if n > best {
				best = n
			}
		} else {
			n = 0
		}
	}
	return best
}

func TestOwnTextHidesImageData(t *testing.T) {
	data := pictureData(4096)
	text := "item Pic {\n  pos 0 0\n  image \"photo.png\" @(" + data + ")\n  title Hi\n}\n"
	n := nodeOf(t, text, "Pic")
	code := OwnText(text, n)
	if strings.Contains(code, data) || strings.Contains(code[:min(len(code), 200)], "iVBOR") {
		t.Fatalf("the data is in the Code section:\n%s", code)
	}
	if len(code) > 300 || base64Run(code) > 200 {
		t.Fatalf("the Code section still holds data: %d bytes, run %d\n%s", len(code), base64Run(code), code)
	}
	if !strings.Contains(code, `image "photo.png" @(image data 5.3 KB, hidden `) {
		t.Fatalf("no stub for the picture:\n%s", code)
	}
	// a stub is tenn: it parses and prints back
	ast, p := tenn.Parse(code)
	if p.Errors.HasErrors() {
		t.Fatalf("the stub does not parse: %v\n%s", p.Errors, code)
	}
	if printed := tenn.ToStr(ast, 0, false); !strings.Contains(printed, "image data 5.3 KB") || strings.Contains(printed, data) {
		t.Fatalf("the stub does not print back:\n%s", printed)
	}
	// and the hidden data goes back byte for byte
	if got := merge(t, text, "Pic", code); got != text {
		t.Fatalf("round trip lost the data:\n%s", got)
	}
}

func TestOwnTextHidesEveryImage(t *testing.T) {
	first, second := pictureData(3000), pictureData(1500)
	text := "item Pic {\n  image \"one.png\" @(" + first + ")\n  image \"two.png\" @(" + second + ")\n  title Hi\n}\n"
	n := nodeOf(t, text, "Pic")
	code := OwnText(text, n)
	if strings.Contains(code, first) || strings.Contains(code, second) {
		t.Fatalf("a picture is in the Code section:\n%s", code)
	}
	if got := strings.Count(code, "@(image data "); got != 2 {
		t.Fatalf("%d stubs, want 2:\n%s", got, code)
	}
	if got := merge(t, text, "Pic", code); got != text {
		t.Fatalf("round trip lost a picture:\n%s", got)
	}
}

func TestReplaceOwnEditKeepsImageData(t *testing.T) {
	data := pictureData(4096)
	text := "item Pic {\n  pos 0 0\n  image \"photo.png\" @(" + data + ")\n  title Hi\n}\n"
	n := nodeOf(t, text, "Pic")
	panel := strings.Replace(OwnText(text, n), "title Hi", "title Hello", 1)
	got := merge(t, text, "Pic", panel)
	want := strings.Replace(text, "title Hi", "title Hello", 1)
	if got != want {
		t.Fatalf("the edit lost the data or did not land:\n%q\nwant\n%q", got, want)
	}
	// with a kid too: the stub of a kid and the stub of a picture stay apart
	withKid := "item Page {\n  image \"photo.png\" @(" + data + ")\n  item Row { gap 1 }\n}\n"
	pn := nodeOf(t, withKid, "Page")
	panel = strings.Replace(OwnText(withKid, pn), "gap 1", "gap 3", 1)
	if got, want := merge(t, withKid, "Page", panel), strings.Replace(withKid, "gap 1", "gap 3", 1); got != want {
		t.Fatalf("edit of a node with a kid:\n%q\nwant\n%q", got, want)
	}
}

// TestImageInHiddenKidStaysWhole hides only the pictures the Code section shows: the one of a kid that
// is cut to its head line is not in the pane, and its bytes travel with the kid.
func TestImageInHiddenKidStaysWhole(t *testing.T) {
	top, inKid := pictureData(1000), pictureData(2000)
	text := "item Page {\n  image \"top.png\" @(" + top + ")\n  item Row {\n    image \"row.png\" @(" + inKid + ")\n    item Cell { title C }\n  }\n}\n"
	n := nodeOf(t, text, "Page")
	code := OwnText(text, n)
	if strings.Contains(code, top) {
		t.Fatalf("the picture of the item is in the pane:\n%s", code)
	}
	if strings.Contains(code, inKid) {
		t.Fatalf("the picture of the hidden kid is in the pane:\n%s", code)
	}
	if got := strings.Count(code, "@(image data "); got != 1 {
		t.Fatalf("%d stubs, want the one of the item:\n%s", got, code)
	}
	if got := merge(t, text, "Page", code); got != text {
		t.Fatalf("round trip lost a picture:\n%s", got)
	}
	// the kid itself shows its own picture as a stub
	row := nodeOf(t, text, "Page.Row")
	rowCode := OwnText(text, row)
	if strings.Contains(rowCode, inKid) || strings.Count(rowCode, "@(image data ") != 1 {
		t.Fatalf("the kid pane:\n%s", rowCode)
	}
	if got := merge(t, text, "Page.Row", rowCode); got != text {
		t.Fatalf("round trip of the kid:\n%s", got)
	}
}

func TestFileImageIsNotHidden(t *testing.T) {
	text := "item Pic {\n  pos 0 0\n  image \"assets/photo.png\"\n  title %{@(assets/photo.png|800)}\n}\n"
	n := nodeOf(t, text, "Pic")
	code := OwnText(text, n)
	if code != NodeText(text, n) {
		t.Fatalf("an image from a file was hidden:\n%s", code)
	}
	if strings.Contains(code, "image data") {
		t.Fatalf("an image from a file got a stub:\n%s", code)
	}
	if got := merge(t, text, "Pic", code); got != text {
		t.Fatalf("round trip of an image from a file:\n%s", got)
	}
}

func TestHiddenImageDataIsNoTrap(t *testing.T) {
	cases := []string{
		"",
		"item X { image \"p.png\" }",
		"item X { image \"p.png\" @() }",
		"item X { image \"p.png\" @(unclosed",
		"item X { image \"p.png\" @(!!not base64!!) }",
		"item X { title %{@(p.png|100)} }",
	}
	for _, text := range cases {
		hidden := hideImageData(text)
		if back := restoreImageData(hidden, text); back != text {
			t.Errorf("hide and restore of %q gave %q", text, back)
		}
		if again := hideImageData(hidden); again != hidden {
			t.Errorf("hiding hid a stub again: %q -> %q", hidden, again)
		}
	}
	// a stub whose digest was retyped pairs by name and order when the numbers agree
	stub := "item X { image \"p.png\" @(image data 2.0 KB, hidden 0badf00d) }"
	if got, want := restoreImageData(stub, "item X { image \"p.png\" @(AAAA) }"), "item X { image \"p.png\" @(AAAA) }"; got != want {
		t.Errorf("a stub of the same name did not pair: %q", got)
	}
	// but it does not take a picture of another name
	stranger := "item X { image \"q.png\" @(image data 2.0 KB, hidden 0badf00d) }"
	if got := restoreImageData(stranger, "item X { image \"p.png\" @(AAAA) }"); got != stranger {
		t.Errorf("a stub found a stranger's data: %q", got)
	}
}

// imageLineOf is the line of a file that writes the data of a classic image, the longest one at
// least min bytes long.
func imageLineOf(t *testing.T, path string, min int) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("%s: %v", path, err)
	}
	best := ""
	for _, line := range strings.Split(string(data), "\n") {
		s := strings.TrimSpace(line)
		if len(s) >= min && strings.HasPrefix(s, `image "`) && strings.Contains(s, "@(") && len(s) > len(best) {
			best = s
		}
	}
	if best == "" {
		t.Skipf("no inline image line in %s", path)
	}
	return best
}

// itemProps is the props text the Code section of a classic diagram shows for the item named name.
func itemProps(t *testing.T, text, name string) string {
	t.Helper()
	ast, p := tenn.Parse(text)
	if p.Errors.HasErrors() {
		t.Fatal(p.Errors)
	}
	em := model.ParseTenn(ast)
	var found *model.DiagramItem
	var walk func(e *model.Element)
	walk = func(e *model.Element) {
		if found != nil {
			return
		}
		for _, it := range e.Items {
			if it.Name == name {
				found = it
				return
			}
		}
		for _, c := range e.Elements {
			walk(c)
		}
	}
	walk(&em.Element)
	if found == nil {
		t.Fatalf("no item %q", name)
	}
	return tenn.ToStr(model.ItemToTennAsProps(found, tenn.KindStatements, false), 0, false)
}

// TestFixtureImageLine hides the picture of the fixture the tests share, the shortened base64 of the
// classic screenshot.
func TestFixtureImageLine(t *testing.T) {
	line := closeImageLine(imageLineOf(t, "../../web/fixtures/Example.saved.tenn", 0))
	data := inlineData(line)
	if len(data) < 20 {
		t.Fatalf("the fixture line holds no data: %q", line)
	}
	text := imageDoc(line)
	n := nodeOf(t, text, "Pic")
	code := OwnText(text, n)
	if strings.Contains(code, data) {
		t.Fatalf("the fixture data is in the Code section:\n%s", code)
	}
	if !strings.Contains(code, "image data ") {
		t.Fatalf("no stub for the fixture picture:\n%s", code)
	}
	if got := merge(t, text, "Pic", code); got != text {
		t.Fatalf("round trip of the fixture line:\n%s", got)
	}
	// the fixture itself keeps the cut literal of the golden file, newlines inside and all: the props
	// text of the classic Code section hides it and gives it back whole
	raw, err := os.ReadFile("../../web/fixtures/Example.saved.tenn")
	if err != nil {
		t.Fatal(err)
	}
	full := itemProps(t, string(raw), "Main Window")
	hiddenProps := hideImageData(full)
	if len(hiddenProps) > 400 || base64Run(hiddenProps) > 200 {
		t.Fatalf("the fixture props still hold data (%d bytes):\n%s", len(hiddenProps), hiddenProps)
	}
	if back := restoreImageData(hiddenProps, full); back != full {
		t.Fatal("the fixture props round trip lost the data")
	}
	t.Logf("fixture props: %d bytes -> %d bytes", len(full), len(hiddenProps))
}

// TestRealImageLine does the same with a line of a real document: a screenshot of megabytes in an
// element of docs/Example.tenn (read only, never written).
func TestRealImageLine(t *testing.T) {
	line := closeImageLine(imageLineOf(t, "../../docs/Example.tenn", 1<<20))
	text := imageDoc(line)
	n := nodeOf(t, text, "Pic")
	start := len(text)
	code := OwnText(text, n)
	if len(code) > 400 || base64Run(code) > 200 {
		t.Fatalf("the Code section still holds data: %d bytes, run %d", len(code), base64Run(code))
	}
	if got := merge(t, text, "Pic", code); got != text {
		t.Fatalf("round trip of a real line differs")
	}
	t.Logf("real line: document %d bytes -> Code section %d bytes", start, len(code))

	// the classic Code section is the props text of the model: it takes the same stub
	classic := "element \"General info\" {\n    item \"Main Window\" {\n        pos 0 0\n        " + line + "\n        title Hi\n    }\n}\n"
	ast, p := tenn.Parse(classic)
	if p.Errors.HasErrors() {
		t.Fatal(p.Errors)
	}
	it := model.ParseTenn(ast).Elements[0].Items[0]
	full := tenn.ToStr(model.ItemToTennAsProps(it, tenn.KindStatements, false), 0, false)
	hidden := hideImageData(full)
	if strings.Contains(hidden, line[strings.Index(line, "@(")+2:]) {
		t.Fatal("the props text still holds the data")
	}
	if back := restoreImageData(hidden, full); back != full {
		t.Fatal("the props text round trip lost the data")
	}
	t.Logf("classic props: %d bytes -> %d bytes", len(full), len(hidden))
}

// TestSceneImageEditUndo edits the Code section of a picture and undoes it through the real session:
// the text of the document keeps the data whole at every step.
func TestSceneImageEditUndo(t *testing.T) {
	data := pictureData(4096)
	text := "element \"Screen\" {\n  item \"Pic\" {\n    width 100\n    height 50\n    image \"photo.png\" @(" + data + ")\n    title Hi\n  }\n}\n"
	sh := &Shell{}
	v := newDoc(t, sh, filepath.Join(t.TempDir(), "s.tenn"), text)
	before := v.text
	if len(v.session.Root().Elements) == 0 {
		t.Fatalf("no element in\n%s", before)
	}
	v.session.SetElement(v.session.Root().Elements[0])
	n := NodeAt(v.elementNode(v.session.Root().Elements[0]), "Pic")
	if n == nil {
		t.Fatal("no item")
	}
	panel := strings.Replace(OwnText(before, n), "title Hi", "title Yo", 1)
	e := ReplaceOwnEdit(before, n, panel)
	if err := v.applyText(e.Start, e.End, e.New); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(v.text, data) || !strings.Contains(v.text, "title Yo") || !v.isDirty() {
		t.Fatalf("the edit lost the data or did not land (dirty %v)", v.isDirty())
	}
	v.session.Undo()
	if v.text != before || v.isDirty() {
		t.Fatalf("undo did not bring the document back:\n%s", v.text)
	}
	v.session.Redo()
	if !strings.Contains(v.text, data) || !strings.Contains(v.text, "title Yo") {
		t.Fatalf("redo lost the data or the edit:\n%s", v.text)
	}
}

// TestClassicPropsImageEditUndo is the Code section of a classic diagram: the text comes from the
// model, and an edit of the visible part keeps the data through apply, undo and redo.
func TestClassicPropsImageEditUndo(t *testing.T) {
	data := pictureData(4096)
	text := "element \"General info\" {\n    item \"Main Window\" {\n        pos 0 0\n        image \"photo.png\" @(" + data + ")\n        title Hi\n    }\n}\n"
	sh := &Shell{}
	v := newDoc(t, sh, filepath.Join(t.TempDir(), "c.tenn"), text)
	v.session.SetElement(v.session.Root().Elements[0])
	before := v.text
	if v.kind != model.DocDiagram || !strings.Contains(before, data) {
		t.Fatalf("kind %v, the data is not in the document:\n%s", v.kind, before)
	}
	item := v.session.Root().Elements[0].Items[0]
	target := editor.Target{Item: item}
	full := v.session.PropsText(target)
	hidden := hideImageData(full)
	if len(hidden) > 300 || strings.Contains(hidden, data) {
		t.Fatalf("the props text still holds the data (%d bytes):\n%s", len(hidden), hidden)
	}
	edited := strings.Replace(hidden, "title Hi", "title Yo", 1)
	if !v.session.ApplyProps(target, restoreImageData(edited, full)) {
		t.Fatal("the props edit was refused")
	}
	if !strings.Contains(v.text, data) || !strings.Contains(v.text, "title Yo") || !v.isDirty() {
		t.Fatalf("the edit lost the data or did not land (dirty %v):\n%s", v.isDirty(), v.text)
	}
	v.session.Undo()
	if v.text != before || v.isDirty() {
		t.Fatalf("undo did not bring the document back:\n%s", v.text)
	}
	v.session.Redo()
	if !strings.Contains(v.text, data) || !strings.Contains(v.text, "title Yo") {
		t.Fatalf("redo lost the data or the edit:\n%s", v.text)
	}
}
