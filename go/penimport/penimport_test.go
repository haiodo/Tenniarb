package penimport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/tenniarb/go/scene"
	"github.com/haiodo/tenniarb/go/tenn"
)

// convert converts the fixtures (names without the extension) into a temp dir and returns the result
// and the dir. The fixture files import each other by their file names.
func convert(t *testing.T, main string) (*Result, string) {
	t.Helper()
	res, err := Convert([]string{filepath.Join("testdata", main+".pen")}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := res.Write(dir); err != nil {
		t.Fatal(err)
	}
	return res, dir
}

// convertJSON is convert for a fixture written in the test.
func convertJSON(t *testing.T, body string) (*Result, string, *File) {
	t.Helper()
	src := filepath.Join(t.TempDir(), "inline.pen")
	if err := os.WriteFile(src, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Convert([]string{src}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := res.Write(dir); err != nil {
		t.Fatal(err)
	}
	return res, dir, res.Files[len(res.Files)-1]
}

// wantFidelity requires every top level node of f to lay out as gfx/pen lays it out.
func wantFidelity(t *testing.T, f *File, dir string) *Report {
	t.Helper()
	rep, err := Compare(f, dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Problems) > 0 {
		t.Fatalf("go/scene rejects %s:\n%s", f.Name, strings.Join(rep.Problems, "\n"))
	}
	if len(rep.Screens) == 0 {
		t.Fatalf("%s: nothing was compared", f.Name)
	}
	for _, s := range rep.Screens {
		t.Logf("%s / %s: %d boxes, %d within 0.5, max delta %g", f.Name, s.Name, s.Compared, s.Within, s.MaxDelta)
		if s.Err != "" || s.Within != s.Compared || s.NameDiffs != 0 || s.PenOnly != 0 || s.SceneOnly != 0 {
			t.Errorf("%s / %s: %+v", f.Name, s.Name, s)
		}
	}
	return rep
}

func fileNamed(res *Result, name string) *File {
	for _, f := range res.Files {
		if f.Name == name {
			return f
		}
	}
	return nil
}

func TestLayoutFidelity(t *testing.T) {
	res, dir := convert(t, "layout")
	f := fileNamed(res, "layout")
	rep := wantFidelity(t, f, dir)
	if rep.Screens[0].Compared < 12 {
		t.Errorf("only %d boxes compared", rep.Screens[0].Compared)
	}
	out := res.Tenn(f)
	// y points up: pen y 20 is pos y -20
	for _, want := range []string{"pos 10 -20", "auto-layout column", "justify space-between", "padding 10 20", "width fill",
		"height fill", `title "Hello \"quoted\" world"`, "auto-layout abs", "clip yes", "shadow 0 2 4 #00000040", "corner-radius 10", "pos 5 -7"} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in\n%s", want, out)
		}
	}
	if f.Unsupported["type group"] != 1 || f.Unsupported["size fallback in fill_container"] != 1 || f.Unsupported["ellipse of flexible width"] != 1 {
		t.Errorf("unsupported: %v", f.Unsupported)
	}
}

func TestComponentsFidelity(t *testing.T) {
	res, dir := convert(t, "components")
	f := fileNamed(res, "components")
	wantFidelity(t, f, dir)
	out := res.Tenn(f)
	for _, want := range []string{"use Library.Card as Plain", "at Title", "replace Footer.Ok", "at Footer.Ok", "use Library.Wrapper as Nested",
		"at Inner_card", "use Screen.Plain as \"Plain 2\"", "at Footer.Cancel { hidden no }", "replace Footer"} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in\n%s", want, out)
		}
	}
	// a disabled node stays in the component, hidden, so that an instance can switch it on
	if !strings.Contains(out, "item Ghost { hidden yes;") || strings.Contains(out, "slot opt_") {
		t.Errorf("the disabled node is not hidden:\n%s", out)
	}
}

func TestSlotsFidelity(t *testing.T) {
	res, dir := convert(t, "slots")
	f := fileNamed(res, "slots")
	wantFidelity(t, f, dir)
	out := res.Tenn(f)
	for _, want := range []string{"slot Content {", "slot Bar {", "slot Extra {"} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in\n%s", want, out)
		}
	}
}

func TestImportsFidelity(t *testing.T) {
	res, dir := convert(t, "imports")
	lib, f := fileNamed(res, "lib"), fileNamed(res, "imports")
	wantFidelity(t, lib, dir)
	wantFidelity(t, f, dir)
	if res.Files[0] != lib {
		t.Errorf("the library must come first")
	}
	out := res.Tenn(f)
	for _, want := range []string{`element lib --from-file lib.tenn`, "use lib.Kit_Buttons.Button as Save", "at Icon { hidden no }", "missing: Gone"} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in\n%s", want, out)
		}
	}
	if len(f.Placeholders) != 1 || len(f.Warnings) != 1 {
		t.Errorf("placeholders %v warnings %v", f.Placeholders, f.Warnings)
	}
	if lib.Variables["accent"] == 0 || lib.Variables["bg"] == 0 {
		t.Errorf("variables %v", lib.Variables)
	}
	if !strings.Contains(res.Tenn(lib), "color #3366ff") || !strings.Contains(res.Tenn(lib), "text-color #ffffff") {
		t.Errorf("variables are not resolved to the light theme:\n%s", res.Tenn(lib))
	}
	if f.Unsupported["override of an id that is not in the file or library"] != 1 {
		t.Errorf("unsupported: %v", f.Unsupported)
	}
}

func TestOutputParses(t *testing.T) {
	for _, name := range []string{"layout", "components", "slots", "imports"} {
		res, _ := convert(t, name)
		for _, f := range res.Files {
			out := res.Tenn(f)
			n, p := tenn.Parse(out)
			if p.Errors.HasErrors() {
				t.Fatalf("%s: %v", f.Name, p.Errors.Errors)
			}
			if again := tenn.CompactAs(n, outStyle) + "\n"; again != out {
				t.Errorf("%s: printing the parsed file gives other text", f.Name)
			}
		}
	}
}

func TestMapListsNodes(t *testing.T) {
	res, _ := convert(t, "components")
	m := res.Map(fileNamed(res, "components"))
	for _, want := range []string{"card\tLibrary.Card\n", "cok\tLibrary.Card.Footer.Ok\n", "s1\tScreen\n"} {
		if !strings.Contains(m, want) {
			t.Errorf("no %q in\n%s", want, m)
		}
	}
}

func TestAbsoluteChildLeavesTheFlow(t *testing.T) {
	// gfx/pen has no absolute children, so the expected boxes are written out
	_, dir, f := convertJSON(t, `{"version":"2.21","children":[
	 {"type":"frame","id":"p","name":"Parent","layout":"vertical","gap":5,"padding":10,"children":[
	  {"type":"rectangle","id":"a","name":"A","width":40,"height":20},
	  {"type":"rectangle","id":"b","name":"Badge","layoutPosition":"absolute","x":30,"y":40,"width":8,"height":9},
	  {"type":"rectangle","id":"c","name":"C","width":40,"height":20}]}]}`)
	tree, err := scene.Load(dir, f.Name+".tenn")
	if err != nil {
		t.Fatal(err)
	}
	sc := gfx.NewScene()
	var screen *scene.Screen
	sc.Update(func(tx *gfx.Tx) { screen, err = tree.Build(tx, nil, "", scene.Options{}) })
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]scene.Box{}
	for _, b := range screen.Boxes() {
		got[b.Path] = b
	}
	for path, want := range map[string][4]float64{
		"Parent":       {0, 0, 60, 65},
		"Parent.A":     {10, 10, 40, 20},
		"Parent.C":     {10, 35, 40, 20},
		"Parent.Badge": {30, 40, 8, 9},
	} {
		b := got[path]
		if b.X != want[0] || b.Y != want[1] || b.W != want[2] || b.H != want[3] {
			t.Errorf("%s: %+v, want %v", path, b, want)
		}
	}
}

func TestNamesStayUnambiguous(t *testing.T) {
	// two siblings with one name, names that normalize alike, a name that starts with a digit,
	// and a decoy named like the first segment of a path
	res, dir, f := convertJSON(t, `{"version":"2.21","children":[
	 {"type":"frame","id":"c","name":"Card","reusable":true,"layout":"vertical","children":[
	  {"type":"rectangle","id":"r1","name":"Row","width":10,"height":10},
	  {"type":"rectangle","id":"r2","name":"Row","width":11,"height":11},
	  {"type":"rectangle","id":"r3","name":"a b","width":12,"height":12},
	  {"type":"rectangle","id":"r4","name":"a_b","width":13,"height":13},
	  {"type":"rectangle","id":"r5","name":"2024","width":14,"height":14}]},
	 {"type":"frame","id":"pg","name":"Page","x":300,"layout":"vertical","children":[
	  {"type":"frame","id":"decoy","name":"Card","children":[
	   {"type":"ref","id":"u","name":"Use","ref":"c","descendants":{"r2":{"width":30},"r4":{"height":31},"r5":{"width":32},"r1":{"height":33}}}]}]}]}`)
	wantFidelity(t, f, dir)
	out := res.Tenn(f)
	if strings.Count(out, "item Row {") != 1 || !strings.Contains(out, `item "Row 2"`) {
		t.Errorf("siblings are not told apart:\n%s", out)
	}
}

func TestTextLineHeightAndAlign(t *testing.T) {
	// 14 * 1.25 = 17.5 rounds to 18 in gfx/pen; the file carries the box, not a difference to the font's own
	res, _, f := convertJSON(t, `{"version":"2.21","children":[{"type":"frame","id":"p","name":"P","width":400,"height":300,"children":[
	 {"type":"text","id":"a","name":"A","content":"x","fontSize":14,"lineHeight":1.25},
	 {"type":"text","id":"b","name":"B","content":"x","textAlign":"center"},
	 {"type":"text","id":"c","name":"C","content":"x","textAlign":"right"},
	 {"type":"text","id":"d","name":"D","content":"x"}]}]}`)
	out := res.Tenn(f)
	for _, want := range []string{"line-height 18", "layout middle", "layout right", "layout left"} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "line-spacing") {
		t.Errorf("line-spacing is written:\n%s", out)
	}
}

func TestRefCyclesAndMissingRefsGivePlaceholders(t *testing.T) {
	_, dir, f := convertJSON(t, `{"version":"2.21","children":[
	 {"type":"frame","id":"a","name":"A","reusable":true,"children":[{"type":"ref","id":"ab","name":"ToB","ref":"b"}]},
	 {"type":"frame","id":"b","name":"B","reusable":true,"children":[{"type":"ref","id":"ba","name":"ToA","ref":"a"}]},
	 {"type":"ref","id":"x","name":"Use A","ref":"a","x":100},
	 {"type":"ref","id":"y","name":"Nowhere","ref":"zzz","x":200}]}`)
	if len(f.Warnings) == 0 || f.Unsupported["unresolved ref"] == 0 {
		t.Errorf("warnings %v unsupported %v", f.Warnings, f.Unsupported)
	}
	if _, err := scene.Load(dir, f.Name+".tenn"); err != nil {
		t.Fatal(err)
	}
}

func TestTextWithBackslashesParses(t *testing.T) {
	res, _, f := convertJSON(t, `{"version":"2.21","children":[{"type":"text","id":"t","name":"T","content":"a\\\"b\\","fontSize":14}]}`)
	_, p := tenn.Parse(res.Tenn(f))
	if p.Errors.HasErrors() {
		t.Fatalf("%v\n%s", p.Errors.Errors, res.Tenn(f))
	}
}

func TestOverrideReachesSlotFallbackOfNestedInstance(t *testing.T) {
	// Row holds an instance of Cell; Cell has a slot with fallback content. The screen changes the fallback
	// text of one Row and fills the slot of the other, which needs an at on the nested instance.
	res, dir, f := convertJSON(t, `{"version":"2.21","children":[
	 {"type":"frame","id":"cell","name":"Cell","reusable":true,"layout":"vertical","children":[
	  {"type":"frame","id":"cs","name":"Cells","layout":"vertical","slot":[],"children":[
	   {"type":"text","id":"fb","name":"Fallback","content":"short","fontSize":14}]}]},
	 {"type":"frame","id":"row","name":"Row","reusable":true,"layout":"vertical","children":[
	  {"type":"ref","id":"r1","name":"Inner","ref":"cell"}]},
	 {"type":"frame","id":"scr","name":"Screen","x":300,"layout":"vertical","gap":10,"children":[
	  {"type":"ref","id":"a","name":"Edited","ref":"row","descendants":{"fb":{"content":"a much longer fallback text"}}},
	  {"type":"ref","id":"b","name":"Filled","ref":"row","descendants":{"cs":{"children":[
	   {"type":"rectangle","id":"n1","name":"Box","width":50,"height":40}]}}}]}]}`)
	wantFidelity(t, f, dir)
	out := res.Tenn(f)
	for _, want := range []string{"at Inner.Cells.Cells.Fallback { title", "at Inner {\n", "slot Cells {\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in\n%s", want, out)
		}
	}
	for k, n := range f.Unsupported {
		if strings.Contains(k, "fallback") || strings.Contains(k, "not found") {
			t.Errorf("unsupported %s (%d)", k, n)
		}
	}
}

func TestPathOpacityAndTitle(t *testing.T) {
	res, dir, f := convertJSON(t, `{"version":"2.21","children":[
	 {"type":"rectangle","id":"top","name":"Top","width":20,"height":20},
	 {"type":"frame","id":"fr","name":"Frame","x":100,"layout":"horizontal","opacity":0.5,"children":[
	  {"type":"rectangle","id":"in","name":"Inside","width":20,"height":20},
	  {"type":"path","id":"ic","name":"Icon","geometry":"M0 0L10 10","viewBox":[0,0,10,10],"fillRule":"evenodd","width":16,"height":16,"fill":"#112233"},
	  {"type":"path","id":"ic2","name":"Line","geometry":"M0 0L8 8","width":8,"height":8,"stroke":"#445566","strokeWidth":2}]}]}`)
	out := res.Tenn(f)
	for _, want := range []string{`item Top { title ""`, "opacity 0.5", `path "M0 0L10 10"`, "view-box 0 0 10 10", "fill-rule evenodd", "color #112233",
		`path "M0 0L8 8"`, "view-box 0 0 8 8", "border-color #445566"} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in\n%s", want, out)
		}
	}
	if strings.Count(out, `title ""`) != 1 {
		t.Errorf("title is written where the layout hides the name:\n%s", out)
	}
	if _, err := scene.Load(dir, f.Name+".tenn"); err != nil {
		t.Fatal(err)
	}
	wantFidelity(t, f, dir)
	if f.Unsupported["opacity"] != 0 || f.Unsupported["path geometry"] != 0 {
		t.Errorf("unsupported %v", f.Unsupported)
	}
}

func TestFontAndPerCornerRadius(t *testing.T) {
	res, dir, f := convertJSON(t, `{"version":"2.21","children":[
	 {"type":"frame","id":"cmp","name":"Cmp","reusable":true,"layout":"vertical","cornerRadius":[8,8,0,0],"children":[
	  {"type":"text","id":"t","name":"T","content":"hello","fontFamily":"Menlo","fontWeight":"700","fontStyle":"italic","letterSpacing":0.5,"fontSize":14},
	  {"type":"text","id":"p","name":"Plain","content":"plain","fontFamily":"Menlo","fontWeight":"normal","fontStyle":"normal","letterSpacing":0,"fontSize":14}]},
	 {"type":"frame","id":"scr","name":"Screen","x":300,"layout":"vertical","children":[
	  {"type":"ref","id":"a","name":"Inst","ref":"cmp","descendants":{"t":{"fontWeight":"normal","letterSpacing":0},"p":{"fontWeight":600}}}]}]}`)
	out := res.Tenn(f)
	for _, want := range []string{"font-family Menlo", "font-weight 700", "font-style italic", "letter-spacing 0.5", "corner-radius 8 8 0 0",
		"font-weight normal", "font-weight 600"} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in\n%s", want, out)
		}
	}
	// defaults are written only in overrides
	if strings.Count(out, "font-weight normal") != 1 || strings.Contains(out, "font-style normal") {
		t.Errorf("default font values leak into the base nodes:\n%s", out)
	}
	if _, err := scene.Load(dir, f.Name+".tenn"); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"fontFamily", "fontWeight", "fontStyle", "letterSpacing", "per-corner radius"} {
		if f.Unsupported[k] != 0 {
			t.Errorf("unsupported %s", k)
		}
	}
}

// fracText has a fractional line box (17.4), which gfx/pen rounds down and go/scene used to round up.
type fracText struct{}

func (fracText) MeasureText(s string, f gfx.Font) gfx.TextMetrics {
	return gfx.TextMetrics{Width: 7 * float64(len([]rune(s))), Ascent: 8.3, Descent: 9.1}
}

func TestFidelityWithFractionalMetrics(t *testing.T) {
	_, dir, f := convertJSON(t, `{"version":"2.21","children":[{"type":"frame","id":"p","name":"P","layout":"vertical","children":[
	 {"type":"text","id":"a","name":"A","content":"one"},{"type":"text","id":"b","name":"B","content":"two"}]}]}`)
	rep, err := CompareWith(f, dir, true, fracText{})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range rep.Screens {
		if s.Within != s.Compared {
			t.Errorf("%s: %d of %d boxes within 0.5, max delta %g at %s", s.Name, s.Within, s.Compared, s.MaxDelta, s.MaxAt)
		}
	}
}
