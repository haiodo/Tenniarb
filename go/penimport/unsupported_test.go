package penimport

import (
	"os"
	"strings"
	"testing"
)

// A ref to a component the converter does not build leaves a placeholder of the room the component asks
// for, and the warning names the type: an empty box of the right size keeps the layout of the frame
// around it, and "the component is not there" would be a lie about a node the file holds.
func TestUnsupportedComponentKeepsItsRoom(t *testing.T) {
	res, dir, f := convertJSON(t, `{"version":"2.21","children":[
	 {"type":"group","id":"grp","name":"Group","reusable":true,"width":32,"height":32},
	 {"type":"frame","id":"row","name":"Row","layout":"horizontal","width":200,"height":40,"children":[
	  {"type":"ref","id":"u1","name":"Icon","ref":"grp","width":32,"height":32}]}]}`)
	out := res.Tenn(f)

	if !strings.Contains(out, `item "missing: Icon" { width 32; height 32; absolute yes }`) {
		t.Errorf("the placeholder does not keep the room of the component:\n%s", out)
	}
	joined := strings.Join(f.Warnings, "\n")
	if !strings.Contains(joined, `the component is a "group", a type this converter does not build`) {
		t.Errorf("the warning does not name the type:\n%s", joined)
	}
	if strings.Contains(joined, "the component is not there") {
		t.Errorf("the warning says the component is missing, and the file holds it:\n%s", joined)
	}
	wantFidelity(t, f, dir)
}

// A text component carries the width of its sample and says it grows in a fixed width; an instance of it
// that grows of itself must say so, or it takes the width of the sample and sticks out of the frame it was
// placed in - which is what moved the taglines of the footer out of their column.
func TestTextInstanceDoesNotTakeTheSampleWidth(t *testing.T) {
	res, dir, f := convertJSON(t, `{"version":"2.21","children":[
	 {"type":"text","id":"body14","name":"Body 14","reusable":true,"textGrowth":"fixed-width","width":640,
	  "content":"Продаём, устанавливаем и обслуживаем зарядные станции","fontSize":14,"fill":"#000000"},
	 {"type":"frame","id":"brand","name":"Brand","layout":"vertical","width":300,"height":200,"children":[
	  {"type":"ref","id":"t1","name":"Tagline","ref":"body14","textGrowth":"auto","content":"Короткий текст","fill":"#000000"}]}]}`)
	out := res.Tenn(f)

	if !strings.Contains(out, "use ") || !strings.Contains(out, "width hug") {
		t.Errorf("the instance does not grow of itself:\n%s", out)
	}
	if strings.Count(out, "width 640") > 0 && strings.Contains(out, "Tagline") {
		// The component keeps its sample width; the instance must not repeat it.
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(line, "Tagline") && strings.Contains(line, "640") {
				t.Errorf("the sample width went into the instance: %s", line)
			}
		}
	}
	// The fidelity of a hand-made fixture is not Pen's: a text that grows of itself has no box of its own
	// in the file, and nothing to compare with.
	_ = dir
}

// An import leaves the document and nothing but the document: the map and the variables of the converter
// were written beside every converted file, and a reader who asked for a .tenn got three files.
func TestWriteLeavesOnlyTheDocument(t *testing.T) {
	res, _, _ := convertJSON(t, `{"version":"2.21","children":[
	 {"type":"frame","id":"f","name":"Screen","width":100,"height":50}]}`)
	dir := t.TempDir()
	if err := res.Write(dir); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if len(names) != 1 || !strings.HasSuffix(names[0], ".tenn") {
		t.Fatalf("the import wrote %v, want one .tenn", names)
	}
	// The files are still there for whoever debugs the converter, and only when asked.
	if err := res.WriteSidecars(dir); err != nil {
		t.Fatal(err)
	}
	if entries, err = os.ReadDir(dir); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("with the sidecars there are %d files, want three", len(entries))
	}
}

// A pen fill that is a picture is the picture of the box: .tenn has an image on an item and no paint that
// is one, so the fill becomes `image ... fit`. A fill mode the format does not know keeps a warning.
func TestFillImageBecomesTheImageOfTheItem(t *testing.T) {
	res, _, f := convertJSON(t, `{"version":"2.22","children":[
	 {"type":"frame","id":"ph","name":"Photo","width":200,"height":100,
	  "fill":{"type":"image","enabled":true,"url":"assets/photo.png","mode":"cover"}},
	 {"type":"rectangle","id":"tp","name":"Tiled","width":100,"height":100,
	  "fill":{"type":"image","enabled":true,"url":"assets/a.png","mode":"tile"}}]}`)
	out := res.Tenn(f)
	if !strings.Contains(out, `image "assets/photo.png" fit cover`) {
		t.Errorf("the picture fill is not the image of the item:\n%s", out)
	}
	if strings.Contains(out, `item Photo { width 200; height 100; color`) {
		t.Errorf("the picture fill also became a color:\n%s", out)
	}
	if f.Unsupported["fill image mode tile"] != 1 {
		t.Errorf("the unknown fill mode is not reported: %v", f.Unsupported)
	}
}

