//go:build darwin || linux || windows

package app

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/penimport"
	"github.com/haiodo/tenniarb/go/render"
	"github.com/haiodo/tenniarb/go/scene"
)

func walkElements(e *model.Element, f func(*model.Element)) {
	f(e)
	for _, c := range e.Elements {
		walkElements(c, f)
	}
}

// The classic fixtures stay on go/render: no element of them is a scene element.
func TestSceneElementRuleKeepsFixturesClassic(t *testing.T) {
	files, _ := filepath.Glob("../../web/fixtures/*.tenn")
	docs, _ := filepath.Glob("../../docs/*.tenn")
	files = append(files, docs...)
	if len(files) < 50 {
		t.Fatalf("only %d fixtures", len(files))
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		doc := ParseText(string(data))
		if doc.Model == nil {
			continue
		}
		walkElements(&doc.Model.Element, func(e *model.Element) {
			if model.SceneElement(e) {
				t.Errorf("%s: element %q is a scene element", f, e.Name)
			}
		})
		if k := model.KindOfModel(doc.Model); k != model.DocDiagram {
			t.Errorf("%s: kind %s", f, k)
		}
	}
}

// What penimport writes is drawn by go/scene: the content sits at the root of the file.
func TestSceneElementRulePicksPenimportOutput(t *testing.T) {
	for _, name := range []string{"layout", "components", "slots", "imports"} {
		res, err := penimport.Convert([]string{filepath.Join("../penimport/testdata", name+".pen")}, penimport.Options{})
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range res.Files {
			doc := ParseText(res.Tenn(f))
			if doc.Model == nil {
				t.Fatalf("%s: %v", f.Name, doc.Errors)
			}
			if !model.SceneElement(&doc.Model.Element) || model.KindOfModel(doc.Model) != model.DocScene {
				t.Errorf("%s: root scene=%v kind=%s", f.Name, model.SceneElement(&doc.Model.Element), model.KindOfModel(doc.Model))
			}
		}
	}
}

func TestSceneElementRuleCases(t *testing.T) {
	for text, want := range map[string]bool{
		`element "A" { item "X" { item "Y" } }`:                          true,
		`element "A" { item "X" { auto-layout column } }`:                true,
		`element "A" { use kit.Button as ok }`:                           true,
		`element "A" { item "X" { slot } }`:                              true,
		`element "A" { item "X"; item "Y"; link "X" "Y" { use } }`:       false,
		`element "A" { import "lib.tenn" as lib; item "X" { pos 1 2 } }`: false,
		`element "A" { item "X" { min-width 100 } }`:                     false,
	} {
		doc := ParseText(text)
		if got := model.SceneElement(doc.Model.Elements[0]); got != want {
			t.Errorf("%s: %v, want %v", text, got, want)
		}
	}
	if k := model.KindOfModel(ParseText("import \"a.tenn\" as a\nelement \"A\" { item \"X\" }").Model); k != model.DocMixed {
		t.Errorf("import with classic items: %s", k)
	}
}

func TestNodePathRoundTrip(t *testing.T) {
	for _, f := range []string{"slots.tenn", "import.tenn", "overrides.tenn", "nested.tenn"} {
		tree, _ := scene.Load("../scene/testdata", f)
		if tree == nil {
			t.Fatalf("%s did not load", f)
		}
		src, _ := tree.Source(f)
		count := 0
		var each func(el, n *scene.Node)
		each = func(el, n *scene.Node) {
			for _, k := range sceneKids(n) {
				p := NodePath(el, k)
				if got := NodeAt(el, p); got != k {
					t.Errorf("%s: %q gives %v", f, p, got)
				}
				if cmd := strings.Fields(src[k.Range.Start:k.Range.End])[0]; cmd != map[scene.Kind]string{scene.KItem: "item", scene.KUse: "use", scene.KSlot: "slot"}[k.Kind] {
					t.Errorf("%s: %q starts with %q", f, p, cmd)
				}
				count++
				each(el, k)
			}
		}
		var elems func(el *scene.Node)
		elems = func(el *scene.Node) {
			each(el, el)
			for _, e := range el.Elems {
				elems(e)
			}
		}
		elems(tree.File(f))
		if count == 0 {
			t.Errorf("%s: no nodes", f)
		}
	}
	// repeated names get #n
	tree, _ := scene.LoadSource(t.TempDir(), "a.tenn", "item A\nitem A { item B }\n")
	f := tree.File("a.tenn")
	if p := NodePath(f, f.Content[1].Content[0]); p != "A#1.B" {
		t.Errorf("path %q", p)
	}
}

func apply(text string, e TextEdit) string { return text[:e.Start] + e.New + text[e.End:] }

func nodeOf(t *testing.T, text, path string) *scene.Node {
	t.Helper()
	tree, err := scene.LoadSource(t.TempDir(), "a.tenn", text)
	if tree == nil {
		t.Fatal(err)
	}
	n := NodeAt(tree.File("a.tenn"), path)
	if n == nil {
		t.Fatalf("no %s in %s", path, text)
	}
	return n
}

func TestSceneTextEdits(t *testing.T) {
	text := "item Card {\n  auto-layout column\n  item Title { title Hi; color red }\n  item Empty {}\n  item Bare\n}\n"
	cases := []struct {
		path, name string
		args       []string
		want       string
	}{
		{"Card.Title", "title", []string{"Sign in"}, `item Title { title "Sign in"; color red }`},
		{"Card.Title", "gap", []string{"8"}, `item Title { title Hi; color red; gap 8 }`},
		{"Card.Title", "color", nil, `item Title { title Hi }`},
		{"Card.Title", "title", nil, `item Title { color red }`},
		{"Card.Empty", "width", []string{"fill"}, `item Empty { width fill }`},
		{"Card.Bare", "color", []string{"#ffffff"}, `item Bare { color #ffffff }`},
		{"Card", "gap", []string{"4"}, "  auto-layout column\n  item Title { title Hi; color red }\n  item Empty {}\n  item Bare\n  gap 4\n}"},
		{"Card", "auto-layout", nil, "item Card {\n  item Title"},
	}
	for _, c := range cases {
		e, ok := SetPropEdit(text, nodeOf(t, text, c.path), c.name, c.args...)
		got := apply(text, e)
		if !ok || !strings.Contains(got, c.want) {
			t.Errorf("%s %s %v:\n%s", c.path, c.name, c.args, got)
		}
		if _, err := scene.LoadSource(t.TempDir(), "a.tenn", got); err != nil {
			t.Errorf("%s %s: %v", c.path, c.name, err)
		}
	}
	if got := apply(text, DeleteEdit(text, nodeOf(t, text, "Card.Empty"))); strings.Contains(got, "Empty") || !strings.Contains(got, "}\n  item Bare") {
		t.Errorf("delete:\n%s", got)
	}
	if got := apply(text, InsertEdit(text, nodeOf(t, text, "Card.Bare"), "use kit.Button as ok")); !strings.Contains(got, "item Bare { use kit.Button as ok }") {
		t.Errorf("insert into a bare item:\n%s", got)
	}
	nested := "item Page {\n  item Card {\n    gap 1\n  }\n}\n"
	n := nodeOf(t, nested, "Page.Card")
	code := NodeText(nested, n)
	if code != "item Card {\n  gap 1\n}" {
		t.Errorf("node text:\n%s", code)
	}
	if got := apply(nested, ReplaceNodeEdit(nested, n, code+"\n")); got != nested {
		t.Errorf("replace with its own text:\n%s", got)
	}
	if got := CommandText("use", "ui.Button", "as", "Primary button"); got != `use ui.Button as "Primary button"` {
		t.Errorf("command %q", got)
	}
}

// newDoc is a document without a window, its session as the window makes it.
func newDoc(t *testing.T, sh *Shell, path, text string) *Viewer {
	t.Helper()
	if path != "" {
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	doc := ParseText(text)
	doc.Path = path
	v := &Viewer{sh: sh, docCore: &docCore{state: NewState(doc), others: sh.othersText}}
	if path != "" {
		v.watch = Watcher{Path: path}
		v.watch.Poll()
	}
	s, err := editor.New(v.state.Current, editor.Options{
		Render: render.Options{Measurer: gfx.EstimateText{}, NoEvaluate: true},
		OnModelChange: func() {
			v.sessionChanged()
			sh.docChanged(v)
			sh.publish(DocEvent{Path: path, Kind: EventChange, Version: v.version})
		},
		OnElement: func() { v.state.Current = v.session.Element() },
	})
	if err != nil {
		t.Fatal(err)
	}
	v.session = s
	v.adopt(doc.Text)
	sh.viewers = append(sh.viewers, v)
	sh.active = v
	return v
}

const mixedDoc = "import kit.tenn as kit\n" +
	"element \"Map\" {\n    item \"Idea\" {\n        pos 0 0\n    }\n}\n" +
	"element \"Screens\" {\n  item Login { pos 0 0; width 200; auto-layout column; item Title { title Hi } }\n}\n"

func TestTextEditUndoKeepsText(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "kit.tenn"), []byte("item Button { width 40; height 20 }\n"), 0o644)
	sh := &Shell{}
	v := newDoc(t, sh, filepath.Join(dir, "main.tenn"), mixedDoc)
	if v.kind != model.DocMixed || v.text != mixedDoc || v.isDirty() {
		t.Fatalf("kind %s dirty %v text\n%s", v.kind, v.isDirty(), v.text)
	}
	screens := v.session.Root().Elements[1]
	v.session.SetElement(screens)
	if !v.isScene(screens) || v.isScene(v.session.Root().Elements[0]) {
		t.Fatal("renderer choice")
	}
	el := v.elementNode(screens)
	title := NodeAt(el, "Login.Title")
	e, _ := SetPropEdit(v.text, title, "title", "Sign in")
	ver := v.version
	if err := v.applyText(e.Start, e.End, e.New); err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(mixedDoc, "title Hi", `title "Sign in"`, 1)
	if v.text != want || v.version != ver+1 || !v.isDirty() {
		t.Fatalf("after edit (version %d):\n%s", v.version, v.text)
	}
	if v.session.Root().Elements[1] != screens || !strings.Contains(model.ToTennStr(screens, false, true), "Sign in") {
		t.Errorf("the model does not follow the text")
	}
	v.session.Undo()
	if v.text != mixedDoc || v.isDirty() {
		t.Fatalf("undo:\n%s", v.text)
	}
	v.session.Redo()
	if v.text != want {
		t.Fatalf("redo:\n%s", v.text)
	}
	// a classic edit prints the model, the scene edit included
	v.session.RenameElement(v.session.Root().Elements[0], "Map 2")
	if !strings.Contains(v.text, `element "Map 2"`) || !strings.Contains(v.text, "Sign in") {
		t.Fatalf("classic edit:\n%s", v.text)
	}
	v.session.Undo()
	v.session.Undo()
	if strings.Contains(v.text, "Sign in") || strings.Contains(v.text, "Map 2") {
		t.Fatalf("undo both:\n%s", v.text)
	}
	if err := v.applyText(0, 0, "element {"); err == nil || v.text == "element {" {
		t.Errorf("a broken text was taken: %v", err)
	}
}

func TestEditAddingElementSwapsChildren(t *testing.T) {
	sh := &Shell{}
	v := newDoc(t, sh, "", "item A { auto-layout row }\n")
	end := len(v.text)
	if err := v.applyText(end, end, "element \"New\" { item B { item C } }\n"); err != nil {
		t.Fatal(err)
	}
	root := &v.session.Root().Element
	if len(root.Elements) != 1 || root.Elements[0].Parent != root || root.Elements[0].Model != root.Model {
		t.Fatalf("elements %v", root.Elements)
	}
	v.session.Undo()
	if len(root.Elements) != 0 || v.text != "item A { auto-layout row }\n" {
		t.Fatalf("undo: %v\n%s", root.Elements, v.text)
	}
}

const libText = "item Button { auto-layout row; width 40; height 20; color #3366ff }\n"
const screenText = "import lib.tenn as lib\nitem Screen { pos 0 0; auto-layout column; use lib.Button as ok }\n"

func widthOf(t *testing.T, v *Viewer, path string) float64 {
	t.Helper()
	s := gfx.NewScene()
	var sc *scene.Screen
	var err error
	s.Update(func(tx *gfx.Tx) {
		sc, err = v.stree.BuildElement(tx, nil, v.elementNode(v.state.Current), scene.Options{})
	})
	if err != nil {
		t.Fatal(err)
	}
	var find func(p *scene.Placed) float64
	find = func(p *scene.Placed) float64 {
		if p.Src != nil && NodePath(v.elementNode(v.state.Current), p.Src) == path {
			return p.W
		}
		for _, k := range p.Kids {
			if w := find(k); w >= 0 {
				return w
			}
		}
		return -1
	}
	return find(sc.Placed())
}

// A screen shows the library as the other window has it, unsaved edits included, and follows the disk.
func TestOverlayReload(t *testing.T) {
	dir := t.TempDir()
	sh := &Shell{}
	lib := newDoc(t, sh, filepath.Join(dir, "lib.tenn"), libText)
	screen := newDoc(t, sh, filepath.Join(dir, "screen.tenn"), screenText)
	if w := widthOf(t, screen, "Screen.ok"); w != 40 {
		t.Fatalf("width %v", w)
	}
	e, _ := SetPropEdit(lib.text, NodeAt(lib.elementNode(lib.state.Current), "Button"), "width", "64")
	if _, _, err := sh.ApplyEdit(lib.state.Doc.Path, 0, e); err != nil {
		t.Fatal(err)
	}
	if w := widthOf(t, screen, "Screen.ok"); w != 64 {
		t.Fatalf("unsaved library edit not seen: width %v", w)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "lib.tenn")); string(b) != libText {
		t.Fatal("the edit reached the disk")
	}
	// another program writes the library; the open window still overrides it, so close that one
	sh.viewers = sh.viewers[1:]
	os.WriteFile(filepath.Join(dir, "lib.tenn"), []byte("item Button { auto-layout row; width 90; height 20 }\n"), 0o644)
	future := lib.watch.Last.ModTime.Add(2e9)
	os.Chtimes(filepath.Join(dir, "lib.tenn"), future, future)
	if !screen.libsChanged() {
		t.Fatal("change on disk not seen")
	}
	screen.sceneReload()
	if w := widthOf(t, screen, "Screen.ok"); w != 90 {
		t.Fatalf("disk change: width %v", w)
	}
}

func TestDocAPI(t *testing.T) {
	dir := t.TempDir()
	sh := &Shell{}
	var events []DocEvent
	cancel := sh.Subscribe(func(e DocEvent) { events = append(events, e) })
	newDoc(t, sh, filepath.Join(dir, "lib.tenn"), libText)
	path := filepath.Join(dir, "screen.tenn")
	newDoc(t, sh, path, screenText)

	docs := sh.Documents()
	if len(docs) != 2 || docs[1].Path != path || !docs[1].Active || docs[1].Kind != model.DocScene || docs[1].Dirty {
		t.Fatalf("documents %+v", docs)
	}
	if err := sh.Select(path, Selection{Nodes: []string{"Screen.ok"}}); err != nil {
		t.Fatal(err)
	}
	if sel, _ := sh.Selection(path); len(sel.Element) != 0 || len(sel.Nodes) != 1 || sel.Nodes[0] != "Screen.ok" {
		t.Fatalf("selection %+v", sel)
	}
	if err := sh.Select(path, Selection{Nodes: []string{"Screen.nope"}}); err == nil {
		t.Error("a missing node was selected")
	}
	text, ver, _ := sh.Text(path)
	at := strings.Index(text, "pos 0 0")
	if _, _, err := sh.ApplyEdit(path, ver+5, TextEdit{at, at + 7, "pos 10 0"}); !errors.Is(err, ErrStale) {
		t.Fatalf("stale edit: %v", err)
	}
	got, ver2, err := sh.ApplyEdit(path, ver, TextEdit{at, at + 7, "pos 10 0"})
	if err != nil || ver2 != ver+1 || !strings.Contains(got, "pos 10 0") {
		t.Fatalf("edit: %v %d\n%s", err, ver2, got)
	}
	if _, _, err := sh.ApplyEdit(path, 0, TextEdit{0, 0, "}"}); err == nil {
		t.Error("an edit that breaks the text was taken")
	}
	if len(events) != 1 || events[0].Kind != EventChange || events[0].Version != ver2 || events[0].Path != path {
		t.Fatalf("events %+v", events)
	}
	cancel()
	png, err := sh.RenderPNG(path, 1)
	if err != nil || !bytes.HasPrefix(png, []byte("\x89PNG")) {
		t.Fatalf("png: %v %d bytes", err, len(png))
	}
	if _, err := sh.Selection(filepath.Join(dir, "other.tenn")); !errors.Is(err, ErrNoDocument) {
		t.Errorf("unknown document: %v", err)
	}
}

// An insert into an instance without a default slot is a scene error, so the window refuses it.
func TestSceneErrorCountSeesInsertIntoInstance(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "kit.tenn"), []byte("item Card { width 40; height 20; auto-layout column }\n"), 0o644)
	text := "import kit.tenn as kit\nuse kit.Card as C {\n  pos 0 0\n}\n"
	v := newDoc(t, &Shell{}, filepath.Join(dir, "main.tenn"), text)
	if n := v.sceneErrorCount(text); n != 0 {
		t.Fatalf("errors before: %d (%s)", n, v.sceneErr)
	}
	bad := strings.Replace(text, "  pos 0 0\n", "  pos 0 0\n  use kit.Card as D\n", 1)
	if n := v.sceneErrorCount(bad); n < 1 {
		t.Fatalf("errors after insert into the instance: %d, want at least 1", n)
	}
}
