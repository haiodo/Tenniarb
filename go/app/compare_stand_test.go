//go:build darwin || linux || windows

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
)

// The comparison stand of E4/E7: every classic element of a .tenn file is drawn by the old renderer
// (go/render, the port of the Swift one) and by the canvas (go/scene), and the two pictures are
// compared. TENNIARB_COMPARE is the folder for the pictures and for the report; without it the stand
// does nothing, so a normal test run stays quiet.
//
//	TENNIARB_COMPARE=$PWD/../.work/compare go test ./app -run TestCompareStand -v
//
// TENNIARB_COMPARE is a folder path, absolute or relative to go/app. TENNIARB_COMPARE_FILES narrows the
// files (space separated, relative to go/app).
//
// Two differences are wanted: a body of escaped expressions shows its ${...} (the old renderer drops it),
// and a link label in a body is drawn as its label (the old renderer draws the raw `[label](url)`, cut at
// the `#` of the url as a heading). Both are bugs of the old renderer, not parity to keep.
func TestCompareStand(t *testing.T) {
	dir := os.Getenv("TENNIARB_COMPARE")
	if dir == "" {
		t.Skip("TENNIARB_COMPARE is not set")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := strings.Fields(os.Getenv("TENNIARB_COMPARE_FILES"))
	if len(files) == 0 {
		// The test runs in go/app.
		patterns := []string{"../../docs/*.tenn", "../../examples/*.tenn", "../../tenniarb.tenn", "../../libraries/*.tenn"}
		for _, p := range patterns {
			got, _ := filepath.Glob(p)
			files = append(files, got...)
		}
	}
	var report []string
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		m := model.ReadTenn(string(data))
		if m == nil {
			continue
		}
		for _, e := range classicElements(&m.Element) {
			name := strings.TrimSuffix(filepath.Base(file), ".tenn") + "-" + e.Name
			line := compareElement(t, dir, file, data, e, name)
			if line != "" {
				report = append(report, line)
			}
		}
	}
	sort.Strings(report)
	os.WriteFile(filepath.Join(dir, "report.txt"), []byte(strings.Join(report, "\n")+"\n"), 0o644)
	for _, l := range report {
		fmt.Println(l)
	}
	t.Logf("%d elements compared, report in %s", len(report), filepath.Join(dir, "report.txt"))
}

// classicElements are the elements whose picture both renderers draw: a classic diagram with items.
func classicElements(e *model.Element) []*model.Element {
	var out []*model.Element
	var walk func(*model.Element)
	walk = func(e *model.Element) {
		if len(e.Items) > 0 && !model.SceneElement(e) {
			out = append(out, e)
		}
		for _, c := range e.Elements {
			walk(c)
		}
	}
	walk(e)
	return out
}

// compareElement draws e both ways, writes the pictures and returns the line of the report.
func compareElement(t *testing.T, dir, file string, data []byte, e *model.Element, name string) string {
	t.Helper()
	opts := render.Options{Measurer: gfx.DefaultMeasurer}
	oldPNG, err := render.PNG(e, opts, 1, gfx.RGBA(255, 255, 255, 1))
	if err != nil {
		return fmt.Sprintf("%-46s old: %v", name, err)
	}
	work := filepath.Join(t.TempDir(), filepath.Base(file))
	if err := os.WriteFile(work, data, 0o644); err != nil {
		t.Fatal(err)
	}
	v := newDoc(t, &Shell{dir: filepath.Dir(work)}, work, string(data))
	v.session.SetElement(e)
	v.state.Current = e
	sv := newSceneView(e, v.elementNode(e))
	sv.classic = true
	v.scene = gfx.NewScene()
	if err := v.buildSceneView(sv); err != nil {
		return fmt.Sprintf("%-46s new: %v", name, err)
	}
	sv.index(sv.placed) // as realizeScene does: the paths of the items are what the report matches by
	newPNG, err := renderNodesPNG(v.scene, sv.root, []*gfx.Node{sv.root}, 1, gfx.RGBA(255, 255, 255, 1))
	if err != nil {
		return fmt.Sprintf("%-46s new: %v", name, err)
	}
	os.WriteFile(filepath.Join(dir, name+"-old.png"), oldPNG, 0o644)
	os.WriteFile(filepath.Join(dir, name+"-new.png"), newPNG, 0o644)
	return itemReport(name, e, sv)
}

// itemReport compares the box of every item as the old renderer gives it (the selection bounds, without
// the shadow its picture includes) with the box the scene drew, and reports what is left after the frame
// offset of the two coordinate spaces.
func itemReport(name string, e *model.Element, sv *sceneView) string {
	old := render.Build(e, render.Options{Measurer: gfx.DefaultMeasurer})
	type box struct {
		name           string
		ox, oy, ow, oh float64
		nx, ny, nw, nh float64
		seen           bool
	}
	var boxes []box
	seen := map[*model.DiagramItem]bool{}
	for _, pl := range sv.byPath {
		// The canvas names a drawn node by the model item it stands for, duplicates included.
		it := classicItemAt(e, pl.Src)
		if it == nil || seen[it] {
			continue
		}
		d := old.Drawables[it]
		if d == nil {
			continue
		}
		seen[it] = true
		b := d.SelectorBounds()
		boxes = append(boxes, box{
			name: it.Name,
			ox:   b.X, oy: b.Y, ow: b.Width, oh: b.Height,
			nx: pl.X, ny: -(pl.Y + pl.H), nw: pl.W, nh: pl.H,
			seen: true,
		})
	}
	if len(boxes) == 0 {
		return fmt.Sprintf("%-46s no item drawn by both", name)
	}
	// The two spaces differ by a constant offset: the picture of the scene is drawn from its own origin.
	var sx, sy float64
	for _, b := range boxes {
		sx += b.nx - b.ox
		sy += b.ny - b.oy
	}
	sx, sy = sx/float64(len(boxes)), sy/float64(len(boxes))
	var worst, sum float64
	var bad int
	var worstBox box
	for _, b := range boxes {
		d := max(abs(b.nx-sx-b.ox), abs(b.ny-sy-b.oy), abs(b.nw-b.ow), abs(b.nh-b.oh))
		sum += d
		if d > 0.5 {
			bad++
		}
		if d > worst {
			worst, worstBox = d, b
		}
	}
	return fmt.Sprintf("%-40s items %3d off %+5.1f,%+5.1f | mean %5.2f, off-by>0.5: %3d | worst %7.2f %q old %.1fx%.1f new %.1fx%.1f",
		name, len(boxes), sx, sy, sum/float64(len(boxes)), bad, worst, worstBox.name,
		worstBox.ow, worstBox.oh, worstBox.nw, worstBox.nh)
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
