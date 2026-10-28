//go:build darwin || linux || windows

package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"strconv"
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
	"github.com/haiodo/tenniarb/go/scene"
)

// Headless memory measurement of opening a document. Not a GUI test: nothing here creates a window.
// It is skipped unless MEMDOC is set.
//
//	go test -c -o mem.test ./app
//	MEMDOC=/path/doc.tenn MEMOUT=/dir ./mem.test -test.run TestMemDoc -test.v

type memStage struct {
	name              string
	heapAlloc, inuse  uint64
	sys, heapSys, rss uint64
	objs              uint64
}

var memStages []memStage

func rssKB() uint64 {
	out, err := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(os.Getpid())).Output()
	if err != nil {
		return 0
	}
	n, _ := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
	return n
}

func memMark(t *testing.T, name string) {
	t.Helper()
	if os.Getenv("MEMNATURAL") == "" {
		runtime.GC()
		debug.FreeOSMemory()
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	s := memStage{name: name, heapAlloc: m.HeapAlloc, inuse: m.HeapInuse, sys: m.Sys, heapSys: m.HeapSys, rss: rssKB() << 10, objs: m.HeapObjects}
	memStages = append(memStages, s)
	if os.Getenv("MEMNATURAL") != "" {
		t.Logf("%-38s natural: heapAlloc=%7.1fMB heapSys=%7.1fMB idle=%7.1fMB released=%7.1fMB nextGC=%7.1fMB sys=%7.1fMB rss=%7.1fMB numGC=%d", name,
			mb(m.HeapAlloc), mb(m.HeapSys), mb(m.HeapIdle), mb(m.HeapReleased), mb(m.NextGC), mb(m.Sys), mb(s.rss), m.NumGC)
		return
	}
	t.Logf("%-38s heapAlloc=%7.1fMB inuse=%7.1fMB sys=%7.1fMB rss=%7.1fMB objs=%d", name, mb(s.heapAlloc), mb(s.inuse), mb(s.sys), mb(s.rss), s.objs)
}

func mb(b uint64) float64 { return float64(b) / (1 << 20) }

func memHeapProfile(t *testing.T, name string) {
	dir := os.Getenv("MEMOUT")
	if dir == "" {
		return
	}
	runtime.GC()
	f, err := os.Create(filepath.Join(dir, name+".heap"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := pprof.Lookup("heap").WriteTo(f, 0); err != nil {
		t.Fatal(err)
	}
}

func countNodes(n *gfx.Node) int {
	c := 1
	for _, k := range n.Children() {
		c += countNodes(k)
	}
	return c
}

// buildInto replaces the content of sc by the scene element node, as realizeScene does.
func buildInto(d *docCore, sc *gfx.Scene, old *gfx.Node, node *scene.Node) (root *gfx.Node, screen *scene.Screen, err error) {
	sc.Update(func(tx *gfx.Tx) {
		if old != nil {
			tx.Remove(old)
		}
		root = gfx.NewNode(1)
		tx.Add(nil, root)
		screen, err = d.stree.BuildElement(tx, root, node, scene.Options{Measurer: gfx.DefaultMeasurer, FirstID: 2})
	})
	sc.Commit() // the window's frame loop does this
	return
}

func TestMemDoc(t *testing.T) {
	path := os.Getenv("MEMDOC")
	if path == "" {
		t.Skip("MEMDOC not set")
	}
	edits, _ := strconv.Atoi(os.Getenv("MEMEDITS"))
	if edits == 0 {
		edits = 50
	}
	runtime.MemProfileRate = 4096
	memMark(t, "0 baseline")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("file %d bytes", len(data))
	memMark(t, "1 file text (alive: data)")

	doc := ParseText(string(data))
	doc.Path = path
	if doc.Model == nil {
		t.Fatalf("parse: %v", doc.Errors)
	}
	data = nil
	memMark(t, "2 ParseText (tenn.Parse+ParseTenn)")
	memHeapProfile(t, "2parse")

	d := &docCore{state: NewState(doc)}
	sess, err := editor.New(d.state.Current, editor.Options{
		Render:        render.Options{ScreenShadows: true},
		OnModelChange: func() { d.sessionChanged() },
		OnElement:     func() { d.state.Current = d.session.Element() },
	})
	if err != nil {
		t.Fatal(err)
	}
	d.session = sess
	memMark(t, "3 editor.New (goja+classic scene)")
	memHeapProfile(t, "3session")

	d.adopt(doc.Text)
	memMark(t, "4 adopt (LoadOverlay+MountFiles)")
	memHeapProfile(t, "4adopt")
	nfiles := 0
	if d.stree != nil {
		nfiles = len(d.stree.Files())
	}
	t.Logf("kind=%s files=%d mounts=%d scene=%v err=%q", d.kind, nfiles, len(d.mounts), d.stree != nil, d.sceneErr)

	// A second tree alive beside the first: what one LoadOverlay costs.
	if d.stree != nil {
		ov := d.overlay()
		ov[d.rel()] = d.text
		tr2, _ := scene.LoadOverlay(d.root, d.rel(), ov)
		memMark(t, "4b +second LoadOverlay alive")
		runtime.KeepAlive(tr2)
		tr2 = nil
	}

	nEl, nMountEl, nItems := 0, 0, 0
	walkElements(&d.session.Root().Element, func(e *model.Element) {
		nEl++
		nItems += len(e.Items)
		if e.FromFile != "" {
			nMountEl++
		}
	})
	t.Logf("model elements=%d (from-file=%d) classic items=%d", nEl, nMountEl, nItems)

	sc := gfx.NewScene()
	var cur *gfx.Node
	if d.stree != nil && d.isScene(d.state.Current) {
		n := d.elementNode(d.state.Current)
		var screen *scene.Screen
		cur, screen, err = buildInto(d, sc, nil, n)
		if cur != nil {
			t.Logf("realize root: err=%v nodes=%d", err, countNodes(cur))
		}
		runtime.KeepAlive(screen)
		memMark(t, "5 gfx build of the current element")
	}

	if d.stree != nil {
		var best struct {
			name  string
			nodes int
		}
		var all []*scene.Node
		var collect func(n *scene.Node)
		collect = func(n *scene.Node) {
			for _, c := range n.Elems {
				all = append(all, c)
				collect(c)
			}
		}
		collect(d.file())
		t.Logf("scene elements in the tree of the main file: %d", len(all))
		seenFile := map[string]bool{}
		for _, n := range all {
			key := n.Pos.File + "|" + n.Name
			if n.Mount != nil {
				key = n.Mount.Pos.File
			}
			if seenFile[key] {
				continue
			}
			seenFile[key] = true
			s2 := gfx.NewScene()
			root, screen, err := buildInto(d, s2, nil, n)
			if err != nil || root == nil {
				continue
			}
			nn := countNodes(root)
			if os.Getenv("MEMVERBOSE") != "" {
				t.Logf("build %q (%s): nodes=%d", n.Name, key, nn)
			}
			if nn > best.nodes {
				best.name, best.nodes = n.Name, nn
			}
			runtime.KeepAlive(screen)
		}
		t.Logf("biggest element %q: nodes=%d", best.name, best.nodes)
		for _, n := range all {
			if best.nodes > 0 && n.Name == best.name {
				c2, screen, _ := buildInto(d, sc, cur, n)
				cur = c2
				memMark(t, "5b gfx build of the biggest element")
				memHeapProfile(t, "5build")
				runtime.KeepAlive(screen)
				break
			}
		}
	}

	memEdits(t, d, sc, edits)
	memMark(t, "9 end")
	memHeapProfile(t, "9end")
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	t.Logf("TotalAlloc=%.0fMB Mallocs=%d NumGC=%d", mb(ms.TotalAlloc), ms.Mallocs, ms.NumGC)
	if dir := os.Getenv("MEMOUT"); dir != "" {
		var b strings.Builder
		b.WriteString("stage\theapAlloc_MB\theapInuse_MB\tsys_MB\trss_MB\tobjs\n")
		for _, s := range memStages {
			fmt.Fprintf(&b, "%s\t%.1f\t%.1f\t%.1f\t%.1f\t%d\n", s.name, mb(s.heapAlloc), mb(s.inuse), mb(s.sys), mb(s.rss), s.objs)
		}
		os.WriteFile(filepath.Join(dir, "stages.tsv"), []byte(b.String()), 0o644)
	}
	runtime.KeepAlive(d)
	runtime.KeepAlive(sc)
}

// memEdits makes n property edits through the path of the inspector: SetPropEdit, the error count of the
// file before and after, applyFileText (one undo step), the scene reload and a rebuild of the canvas.
func memEdits(t *testing.T, d *docCore, sc *gfx.Scene, n int) {
	if d.stree == nil || os.Getenv("MEMCLASSIC") != "" {
		memClassicEdits(t, d, n)
		return
	}
	var target *scene.Node
	var find func(x *scene.Node, mainOnly bool)
	find = func(x *scene.Node, mainOnly bool) {
		for _, c := range x.Content {
			if target != nil {
				return
			}
			if c.Kind == scene.KItem && (!mainOnly || c.Pos.File == d.rel()) {
				target = c
				return
			}
			find(c, mainOnly)
		}
		for _, e := range x.Elems {
			if target != nil {
				return
			}
			find(e, mainOnly)
		}
	}
	find(d.file(), true)
	if target == nil {
		t.Log("no item in the main file for edits; using a mounted file")
		for _, f := range d.stree.Files()[1:] {
			find(d.stree.File(f), false)
			if target != nil {
				break
			}
		}
	}
	if target == nil {
		t.Log("no scene item to edit")
		return
	}
	t.Logf("edit target %q in %s", target.Name, d.fileOf(target))
	var root *gfx.Node
	memMark(t, "6 before edits")
	var m0 runtime.MemStats
	runtime.ReadMemStats(&m0)
	for i := 0; i < n; i++ {
		target = reFind(d, target)
		if target == nil {
			t.Fatalf("edit %d: target lost", i)
		}
		file := d.fileOf(target)
		text := d.fileText(file)
		e, ok := SetPropEdit(text, target, "pos", strconv.Itoa(i+1), strconv.Itoa(2*i+1))
		if !ok {
			t.Fatalf("edit %d: no edit", i)
		}
		d.editAddsErrors(file, text, e)
		if err := d.applyFileText(file, e.Start, e.End, e.New); err != nil {
			t.Fatalf("edit %d: %v", i, err)
		}
		d.session.EndEvent()
		if d.stree != nil {
			if shown := d.elementNode(d.state.Current); shown != nil {
				root, _, _ = buildInto(d, sc, root, shown)
			}
		}
		if i == 0 || i == n/2 || i == n-1 {
			memMark(t, fmt.Sprintf("7 after edit #%d", i+1))
		}
	}
	memHeapProfile(t, "7edits")
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)
	t.Logf("edits: %d, TotalAlloc per edit %.1f MB, mallocs per edit %d", n, mb(m1.TotalAlloc-m0.TotalAlloc)/float64(n), (m1.Mallocs-m0.Mallocs)/uint64(n))
	for d.session.UndoManager().CanUndo() {
		if err := d.session.UndoManager().Undo(); err != nil {
			t.Fatal(err)
		}
	}
	memMark(t, "8 after undo of all")
}

// reFind finds the item with the same key under a parent of the same key in the same file after a reload.
func reFind(d *docCore, old *scene.Node) *scene.Node {
	var found *scene.Node
	var walk func(x *scene.Node)
	walk = func(x *scene.Node) {
		for _, c := range x.Content {
			if found != nil {
				return
			}
			if c.Kind == scene.KItem && c.Key == old.Key && c.Pos.File == old.Pos.File &&
				(c.Parent == nil || old.Parent == nil || c.Parent.Key == old.Parent.Key) {
				found = c
				return
			}
			walk(c)
		}
		for _, e := range x.Elems {
			if found != nil {
				return
			}
			walk(e)
		}
	}
	if f := d.stree.File(old.Pos.File); f != nil {
		walk(f)
	}
	return found
}

// memClassicEdits: a classic document edits through the session; the first item is moved n times.
func memClassicEdits(t *testing.T, d *docCore, n int) {
	var el *model.Element
	walkElements(&d.session.Root().Element, func(e *model.Element) {
		if el == nil && len(e.Items) > 0 {
			el = e
		}
	})
	if el == nil {
		t.Log("no classic items")
		return
	}
	d.session.SetElement(el)
	it := el.Items[0]
	memMark(t, "6 before classic edits")
	var m0 runtime.MemStats
	runtime.ReadMemStats(&m0)
	for i := 0; i < n; i++ {
		d.session.Select([]*model.DiagramItem{it})
		d.session.MoveBy(1, 1)
		d.session.EndEvent()
		if i == 0 || i == n/2 || i == n-1 {
			memMark(t, fmt.Sprintf("7 after edit #%d", i+1))
		}
	}
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)
	t.Logf("classic edits: %d, TotalAlloc per edit %.1f MB, mallocs per edit %d", n, mb(m1.TotalAlloc-m0.TotalAlloc)/float64(n), (m1.Mallocs-m0.Mallocs)/uint64(n))
	memHeapProfile(t, "7edits")
}
