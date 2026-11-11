//go:build darwin || linux || windows

package app

import (
	"math"
	"strings"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/scene"
)

// sceneEdit applies a text edit of the file of n, made by the window; a refused one shows in the banner.
func (v *Viewer) sceneEdit(n *scene.Node, e TextEdit) bool { return v.sceneEditIn(v.fileOf(n), n, e) }

// sceneEditIn applies an edit of file, about n: the file of a node's own block may differ from the file
// of what it shows (an element with --from-file).
func (v *Viewer) sceneEditIn(file string, n *scene.Node, e TextEdit) bool {
	text := v.fileText(file)
	if v.editAddsErrors(file, text, e) {
		v.setBanner("Not applied: the change adds scene errors")
		return false
	}
	var path string
	if v.sv != nil {
		path = NodePath(v.sv.node, n) // before the apply: it rebuilds the tree of the element
	}
	if err := v.applyFileText(file, e.Start, e.End, e.New); err != nil {
		v.pending = nil
		v.setBanner(err.Error())
		return false
	}
	v.markNode(path, markPulse, "")
	return true
}

// sceneKey: Delete removes the selected node, Escape clears the selection.
func (v *Viewer) sceneKey(name string) {
	switch name {
	case "Delete", "Backspace":
		v.deleteSceneNode()
	case "Escape":
		v.selectScene(nil)
	}
}

func (v *Viewer) deleteSceneNode() {
	n, _ := v.selectedScene()
	if n == nil {
		return
	}
	v.selNodes = nil
	v.sceneEditIn(v.ownFileOf(n), n, DeleteEdit(v.fileText(v.ownFileOf(n)), n))
}

// freeSpot is a place right of the content of the element for a new top level node, as pos x y.
func (v *Viewer) freeSpot() (float64, float64) {
	if v.sv == nil || v.sv.placed == nil || len(v.sv.placed.Kids) == 0 {
		return 0, 0
	}
	maxX, minY := math.Inf(-1), math.Inf(1)
	for _, k := range v.sv.placed.Kids {
		maxX, minY = math.Max(maxX, k.X+k.W), math.Min(minY, k.Y)
	}
	return math.Round(maxX + topGap), math.Round(-minY)
}

// insertComponent adds `use <path> as <name>` where InsertTarget says and selects it; for an instance
// with named slots the slot is asked for in a menu.
func (v *Viewer) insertComponent(path, name string) {
	if !v.ensureSceneElement() || v.sv.node == nil {
		return
	}
	sel, _ := v.selectedScene()
	into, slots := InsertTarget(v.stree, sel)
	if into == nil || len(slots) == 0 {
		v.insertUse(into, "", path, name)
		return
	}
	var entries []editor.Entry
	for _, s := range slots {
		label := "Into slot " + s
		if s == "" {
			label = "Into the default slot"
		}
		entries = append(entries, editor.Entry{Label: label, Run: func() { v.insertUse(into, s, path, name) }})
	}
	entries = append(entries, editor.Entry{Sep: true}, editor.Entry{Label: "On the canvas", Run: func() { v.insertUse(nil, "", path, name) }})
	v.app.Async(func() {
		v.menuOverride = entries
		v.canvasMenu.Show()
	})
}

// insertUse writes the use into the item or instance into, in its slot; into nil puts it on the
// canvas at a free spot.
func (v *Viewer) insertUse(into *scene.Node, slot, path, name string) {
	v.insertUseAt(into, slot, path, name, nil)
}

// insertUseAt is insertUse with the spot of the dropped use: nil keeps the free spot the button picks.
func (v *Viewer) insertUseAt(into *scene.Node, slot, path, name string, at *gfx.Point) {
	target := into
	if target == nil {
		target = v.sv.node
	}
	name = FreeName(target, name)
	use := CommandText("use", path, "as", name)
	cmd := SlotCommand(slot, use)
	if into == nil {
		x, y := v.freeSpot()
		if at != nil {
			x, y = at.X, at.Y
		}
		use += " { " + CommandText("pos", numText(x), numText(y)) + " }"
		cmd = use
	}
	v.insertAndSelect(target, cmd, use)
}

// ScreenSizes are the frames New Screen offers.
var ScreenSizes = []struct {
	Name string
	W, H int
}{{"Desktop", 1440, 900}, {"Laptop", 1280, 800}, {"Tablet", 834, 1194}, {"Phone", 390, 844}}

// newScreen adds an empty frame of w x h right of the content of the element.
func (v *Viewer) newScreen(w, h int) {
	if !v.ensureSceneElement() {
		return
	}
	target := v.sv.node
	if target == nil {
		return
	}
	name := FreeName(target, "Screen")
	x, y := v.freeSpot()
	head := CommandText("item", name)
	cmd := head + " { " + strings.Join([]string{
		CommandText("pos", numText(x), numText(y)), CommandText("width", numText(float64(w))),
		CommandText("height", numText(float64(h))), "auto-layout column", "color #ffffff"}, "; ") + " }"
	v.selNodes = nil
	v.insertAndSelect(target, cmd, head)
}

// insertAndSelect writes cmd at the end of target and selects the node that starts with head in it.
func (v *Viewer) insertAndSelect(target *scene.Node, cmd, head string) {
	owner := target
	if target.Mount != nil { // the content of an element with --from-file is written in that file
		if f := v.stree.File(v.fileOf(target)); f != nil {
			owner = f
		}
	}
	e := InsertEdit(v.fileText(v.fileOf(owner)), owner, cmd)
	at := strings.Index(e.New, head)
	if at < 0 || !v.sceneEdit(owner, e) {
		return
	}
	if n := nodeStartingAt(v.sv.node, e.Start+at); n != nil {
		v.selectScene([]string{NodePath(v.sv.node, n)})
		v.revealNode()
	}
}

// duplicateScene is Cmd+D on the selected scene nodes: a copy of each beside it in its parent, as one
// undo step, the copies selected.
func (v *Viewer) duplicateScene() {
	if !v.sceneMode() || v.sv.node == nil {
		return
	}
	var srcs []DupSource
	for _, p := range v.selNodes {
		n := NodeAt(v.sv.node, p)
		if n == nil {
			continue
		}
		s := DupSource{Node: n}
		if pl := v.sv.byNode[n]; pl != nil {
			s.At, s.Free = &Box{pl.X, pl.Y, pl.W, pl.H}, pl.Free
			if pr := pl.Prop("pos"); pr != nil {
				if vals := pr.Values(); len(vals) == 2 {
					s.Pos = [2]float64{parseNum(vals[0]), parseNum(vals[1])}
				}
			}
			if pl.Parent != nil {
				for _, k := range pl.Parent.Kids {
					s.Others = append(s.Others, Box{k.X, k.Y, k.W, k.H})
				}
			}
		}
		srcs = append(srcs, s)
	}
	e, starts, ok := DuplicateEdit(v.fileText(v.fileOf(v.sv.node)), srcs)
	if !ok || !v.sceneEdit(v.sv.node, e) {
		return
	}
	var paths []string
	for _, off := range starts {
		if n := nodeStartingAt(v.sv.node, off); n != nil {
			paths = append(paths, NodePath(v.sv.node, n))
		}
	}
	v.selectScene(paths)
	v.revealNode()
}

// ensureSceneElement puts a classic element that is empty on the scene renderer; content of a classic
// element with items stays classic and refuses scene commands.
func (v *Viewer) ensureSceneElement() bool {
	if v.sceneMode() {
		return true
	}
	e := v.state.Current
	if e == nil {
		v.setBanner("Choose an element first: a screen goes into a scene element.")
		return false
	}
	for _, it := range e.Items {
		if it.IsLink() {
			v.setBanner("Screens and components go into a scene element, and this one holds links, which a scene does not draw: use an empty element or a new screen.")
			return false
		}
	}
	if v.stree == nil {
		v.kind = model.DocScene
		v.loadScene()
	}
	if v.stree == nil {
		return false
	}
	v.realizeScene()
	return v.sv != nil && v.sv.node != nil
}
