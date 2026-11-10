package penimport

import (
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/tenniarb/go/scene"
)

// A pen icon node names a glyph of a library and keeps no geometry: the converter writes the path of
// the glyph into the item, which is how go/scene draws a vector, so the .tenn file needs no icon
// property and no file beside it.
func TestIconBecomesAPath(t *testing.T) {
	res, dir, f := convertJSON(t, `{"version":"2.21","variables":{"ink":{"type":"color","value":"#112233"}},
	 "children":[{"type":"icon","id":"ico","name":"Zap","reusable":true,"icon":"zap","library":"lucide",
	  "width":32,"height":32,"fill":"$ink"}]}`)
	out := res.Tenn(f)
	d := iconPath(t, "zap")
	for _, want := range []string{"item Zap {", "width 32; height 32", `path "` + d + `"`, "view-box 0 0 24 24",
		"border-color #112233", "line-width 2.67"} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in\n%s", want, out)
		}
	}
	// The stroke of the library is in the units of a 24 wide view box and the icon is 32 wide, so it
	// would be a third too thick at its own width.
	if strings.Contains(out, "line-width 2;") || strings.Contains(out, "line-width 2\n") {
		t.Errorf("the stroke of the glyph was not scaled to the size of the icon:\n%s", out)
	}
	if len(f.Placeholders) != 0 || f.Unsupported["unresolved ref"] != 0 || f.Unsupported["type icon"] != 0 {
		t.Errorf("an icon became a placeholder: %v %v", f.Placeholders, f.Unsupported)
	}
	if _, err := scene.Load(dir, f.Name+".tenn"); err != nil {
		t.Fatal(err)
	}
	// The icon takes its written room and draws no name as text: it is a leaf with a path.
	screen := buildScene(t, dir, f)
	b, ok := boxNamed(screen, "Zap")
	if !ok || b.W != 32 || b.H != 32 {
		t.Errorf("the item is %+v, want a box of 32x32", b)
	}
}

// An instance of an icon component carries a glyph of its own - the size presets of a design are one
// component showing different icons - and a size of its own. Both override the component.
func TestIconRefCarriesItsOwnGlyphAndSize(t *testing.T) {
	res, dir, f := convertJSON(t, `{"version":"2.21","variables":{"ink":{"type":"color","value":"#112233"}},
	 "children":[{"type":"icon","id":"ico","name":"UI / Icon / 20","reusable":true,"icon":"zap","library":"lucide",
	  "width":20,"height":20,"fill":"$ink"},
	 {"type":"frame","id":"row","name":"Row","x":100,"layout":"horizontal","gap":8,"children":[
	  {"type":"ref","id":"u1","name":"Plus","ref":"ico","icon":"plus"},
	  {"type":"ref","id":"u2","name":"Small","ref":"ico","icon":"check","width":12,"height":12}]}]}`)
	out := res.Tenn(f)
	plus, check := iconPath(t, "plus"), iconPath(t, "check")
	for _, want := range []string{
		"use UI___Icon___20 as Plus { view-box 0 0 24 24; path \"" + plus + "\"",
		`path "` + check + `"`,
		"width 12; height 12", "line-width 1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in\n%s", want, out)
		}
	}
	if len(f.Placeholders) != 0 || f.Unsupported["type icon"] != 0 {
		t.Errorf("a ref to an icon became a placeholder: %v %v", f.Placeholders, f.Unsupported)
	}
	if _, err := scene.Load(dir, f.Name+".tenn"); err != nil {
		t.Fatal(err)
	}
	// The instance is a leaf with a path, so it draws no text and keeps the room of its size.
	screen := buildScene(t, dir, f)
	for name, want := range map[string]float64{"Plus": 20, "Small": 12} {
		b, ok := boxNamed(screen, name)
		if !ok || b.W != want || b.H != want {
			t.Errorf("%s is %+v, want a box of %gx%g", name, b, want, want)
		}
	}
}

// A name the table does not have is a warning naming the icon, and the item keeps its room as an empty
// box: the layout around it stays as it was drawn, and nothing panics.
func TestUnknownIconKeepsItsRoom(t *testing.T) {
	res, dir, f := convertJSON(t, `{"version":"2.21","children":[
	 {"type":"icon","id":"ico","name":"Widget","reusable":true,"icon":"no-such-glyph","library":"lucide",
	  "width":24,"height":24},
	 {"type":"frame","id":"row","name":"Row","x":100,"layout":"horizontal","gap":8,"children":[
	  {"type":"ref","id":"a","name":"One","ref":"ico"},
	  {"type":"ref","id":"b","name":"Two","ref":"ico"}]}]}`)
	out := res.Tenn(f)
	joined := strings.Join(f.Warnings, "\n")
	if !strings.Contains(joined, `icon "no-such-glyph" is not in the icons library`) {
		t.Errorf("the warning does not name the icon:\n%s", joined)
	}
	// One warning for the name, not one for the icon node and one for each of its instances.
	if n := strings.Count(joined, "no-such-glyph"); n != 1 {
		t.Errorf("the warning is written %d times:\n%s", n, joined)
	}
	if strings.Contains(out, `path "`) {
		t.Errorf("a path was written for a glyph the table has not:\n%s", out)
	}
	for _, want := range []string{`item Widget { title ""; width 24; height 24 }`} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in\n%s", want, out)
		}
	}
	if len(f.Placeholders) != 0 {
		t.Errorf("an unknown glyph made the item a placeholder of a ref: %v", f.Placeholders)
	}
	if _, err := scene.Load(dir, f.Name+".tenn"); err != nil {
		t.Fatal(err)
	}
	screen := buildScene(t, dir, f)
	if b, ok := boxNamed(screen, "Widget"); !ok || b.W != 24 || b.H != 24 {
		t.Errorf("the icon lost its room: %+v", b)
	}
	for _, name := range []string{"One", "Two"} {
		if b, ok := boxNamed(screen, name); !ok || b.W != 24 || b.H != 24 {
			t.Errorf("%s lost its room: %+v", name, b)
		}
	}
}

// A library other than lucide is not in the table: the item keeps its room and the report says which
// library was left out, instead of drawing an icon of the wrong set.
func TestUnknownIconLibrary(t *testing.T) {
	_, dir, f := convertJSON(t, `{"version":"2.21","children":[
	 {"type":"icon","id":"ico","name":"Widget","icon":"zap","library":"material","width":20,"height":20}]}`)
	if f.Unsupported["icon library material"] != 1 {
		t.Errorf("unsupported %v", f.Unsupported)
	}
	if _, err := scene.Load(dir, f.Name+".tenn"); err != nil {
		t.Fatal(err)
	}
}

// iconPath is the path of a glyph the converter reads from libraries/icons.tenn.
func iconPath(t *testing.T, name string) string {
	t.Helper()
	set := iconSet()
	if set.err != nil {
		t.Fatalf("the icons library is not readable: %v", set.err)
	}
	g, ok := set.glyphs[name]
	if !ok {
		t.Fatalf("the icons library has no %s", name)
	}
	return g.paths[0].d
}

// The glyphs are the items of the shipped libraries/icons.tenn, not a table of the binary: the file is
// read once, and its view box and stroke are the ones the converter writes.
func TestIconsComeFromTheLibrary(t *testing.T) {
	set := iconSet()
	if set.err != nil {
		t.Fatalf("the icons library is not readable: %v", set.err)
	}
	if len(set.glyphs) < 2000 {
		t.Errorf("%d icons in the library, want the whole set", len(set.glyphs))
	}
	if g, ok := set.glyphs["zap"]; !ok || len(g.paths) == 0 || !strings.HasPrefix(g.paths[0].d, "M") {
		t.Errorf("zap = %+v, %v; want a path of the library", g, ok)
	}
	// A glyph with a filled detail keeps it as a path of its own, and the contour stays stroked.
	if g, ok := set.glyphs["tag"]; !ok || len(g.paths) != 2 || g.paths[0].filled || !g.paths[1].filled {
		t.Errorf("tag = %+v, %v; want a contour and a filled dot", g, ok)
	}
	if set.viewBox != [4]float64{0, 0, 24, 24} || set.stroke != 2 {
		t.Errorf("the set is drawn in %v with stroke %g, want 0 0 24 24 and 2", set.viewBox, set.stroke)
	}
}

// buildScene lays the converted file out; the boxes are what the screens draw.
func buildScene(t *testing.T, dir string, f *File) *scene.Screen {
	t.Helper()
	tree, err := scene.Load(dir, f.Name+".tenn")
	if err != nil {
		t.Fatal(err)
	}
	s := gfx.NewScene()
	var screen *scene.Screen
	s.Update(func(tx *gfx.Tx) { screen, err = tree.Build(tx, nil, "", scene.Options{}) })
	if err != nil {
		t.Fatal(err)
	}
	return screen
}

// boxNamed is the box of the item of that name; the root of the file is a box of its own with no name.
func boxNamed(screen *scene.Screen, name string) (scene.Box, bool) {
	for _, b := range screen.Boxes() {
		if b.Name == name {
			return b, true
		}
	}
	return scene.Box{}, false
}
