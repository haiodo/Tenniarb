package penimport

import (
	"strings"
	"testing"
)

// A ref to a component the converter does not build leaves a placeholder of the room the component asks
// for, and the warning names the type: an empty box of the right size keeps the layout of the frame
// around it, and "the component is not there" would be a lie about a node the file holds.
func TestUnsupportedComponentKeepsItsRoom(t *testing.T) {
	res, dir, f := convertJSON(t, `{"version":"2.21","children":[
	 {"type":"icon","id":"ico","name":"Zap","reusable":true,"icon":"zap","library":"lucide","width":32,"height":32},
	 {"type":"frame","id":"row","name":"Row","layout":"horizontal","width":200,"height":40,"children":[
	  {"type":"ref","id":"u1","name":"Icon","ref":"ico","width":32,"height":32}]}]}`)
	out := res.Tenn(f)

	if !strings.Contains(out, `item "missing: Icon" { width 32; height 32; absolute yes }`) {
		t.Errorf("the placeholder does not keep the room of the component:\n%s", out)
	}
	joined := strings.Join(f.Warnings, "\n")
	if !strings.Contains(joined, `the component is a "icon", a type this converter does not build`) {
		t.Errorf("the warning does not name the type:\n%s", joined)
	}
	if strings.Contains(joined, "the component is not there") {
		t.Errorf("the warning says the component is missing, and the file holds it:\n%s", joined)
	}
	wantFidelity(t, f, dir)
}
