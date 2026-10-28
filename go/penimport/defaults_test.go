package penimport

import (
	"strings"
	"testing"
)

func TestDefaultsAreLeftOut(t *testing.T) {
	res, dir, f := convertJSON(t, `{"version":"2.21","children":[
	 {"type":"rectangle","id":"top","name":"Top","width":20,"height":20,"fill":"#ffffff"},
	 {"type":"frame","id":"p","name":"P","x":100,"layout":"vertical","width":400,"height":300,"children":[
	  {"type":"rectangle","id":"r","name":"R","width":20,"height":20,"fill":"#112233"},
	  {"type":"rectangle","id":"s","name":"S","width":20,"height":20,"stroke":"#445566","strokeWidth":1},
	  {"type":"ellipse","id":"e","name":"E","width":20,"height":20},
	  {"type":"text","id":"t","name":"T","content":"x","fill":"#000000"},
	  {"type":"text","id":"k","name":"K","content":"y","fill":"#ffffff"},
	  {"type":"frame","id":"d","name":"D","padding":3,"layout":"horizontal","children":[
	   {"type":"text","id":"w","name":"W","content":"z"}]}]}]}`)
	out := res.Tenn(f)
	for _, gone := range []string{"display", "color #00000000", "text-color #000000", "line-width 1", "item T { auto-layout"} {
		if strings.Contains(out, gone) {
			t.Errorf("%q is written:\n%s", gone, out)
		}
	}
	// outside a layout the item has a white fill and a black border: line-width 0 says there is none
	if strings.Count(out, "line-width 0") != 1 || !strings.Contains(out, "item Top { title \"\"; width 20; height 20; color #ffffff; line-width 0 }") {
		t.Errorf("line-width 0 is dropped outside a layout:\n%s", out)
	}
	// a color on a dark fill and the layout of a node that has children stay
	for _, want := range []string{"text-color #ffffff", "item D {\n\t\tauto-layout row"} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in\n%s", want, out)
		}
	}
	wantFidelity(t, f, dir)
}

func TestOverrideRepeatingTheComponentIsLeftOut(t *testing.T) {
	res, dir, f := convertJSON(t, `{"version":"2.21","children":[
	 {"type":"frame","id":"cmp","name":"Cmp","reusable":true,"layout":"vertical","width":100,"height":50,"fill":"$bg","children":[
	  {"type":"text","id":"lbl","name":"Lbl","content":"a","fontSize":14,"fill":"#222222"}]},
	 {"type":"frame","id":"scr","name":"Screen","x":300,"layout":"vertical","children":[
	  {"type":"ref","id":"a","name":"Same","ref":"cmp","width":100,"height":60,"fill":"$bg","descendants":{"lbl":{"content":"b","fontSize":14,"fill":"#333333"}}}]}],
	 "variables":{"bg":{"type":"color","value":"#eeeeee"}}}`)
	out := res.Tenn(f)
	// a value that names a variable stays: the theme of the instance may be another
	for _, want := range []string{"use Cmp as Same {", "height 60", "color #eeeeee", "at Lbl { title b; text-color #333333 }"} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in\n%s", want, out)
		}
	}
	// the size the component has and the font size are not repeated
	if strings.Count(out, "color #eeeeee") != 2 {
		t.Errorf("the variable override is dropped:\n%s", out)
	}
	block := out[strings.Index(out, "use Cmp as Same"):]
	if strings.Contains(block, "width") || strings.Contains(block, "font-size") {
		t.Errorf("a repeated value is written:\n%s", block)
	}
	wantFidelity(t, f, dir)
}

func TestOutputIsIndentedWithTabs(t *testing.T) {
	res, _ := convert(t, "layout")
	for _, f := range res.Files {
		out := res.Tenn(f)
		if !strings.Contains(out, "\n\t") || strings.Contains(out, "\n  ") {
			t.Errorf("%s is not indented with tabs:\n%s", f.Name, out)
		}
	}
}
