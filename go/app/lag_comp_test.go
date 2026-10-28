//go:build darwin

package app

import (
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/cg"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/render"
	"github.com/haiodo/tenniarb/go/scene"
)

// Headless run of the gowt compositor on a fake host: how much of the view is blank frame by frame
// after an element switch, a fast zoom, a pan. No window. Skipped unless LAGDOC is set.

type fakeLayer struct {
	h      *fakeHost
	frame  gfx.Rect
	img    bool
	hidden bool
	grp    gfx.Group
	gone   bool
}

func (l *fakeLayer) SetImage(gfx.Image)                        { l.img = true }
func (l *fakeLayer) Set(f gfx.Rect, _ gfx.Point, _ gfx.Affine) { l.frame = f }
func (l *fakeLayer) SetOpacity(float64)                        {}
func (l *fakeLayer) SetHidden(b bool)                          { l.hidden = b }
func (l *fakeLayer) Remove()                                   { l.gone = true }

type fakeHost struct{ layers []*fakeLayer }

func (h *fakeHost) NewLayer(g gfx.Group) gfx.Layer {
	l := &fakeLayer{h: h, grp: g}
	h.layers = append(h.layers, l)
	return l
}
func (h *fakeHost) Begin() {}
func (h *fakeHost) Commit() {
	w := h.layers[:0]
	for _, l := range h.layers {
		if !l.gone {
			w = append(w, l)
		}
	}
	h.layers = w
}

// blank is the fraction of the view not under a tile that has an image.
func (h *fakeHost) blank(vw, vh float64) float64 {
	const nx, ny = 56, 36
	miss := 0
	for j := 0; j < ny; j++ {
		for i := 0; i < nx; i++ {
			p := gfx.Point{X: (float64(i) + .5) * vw / nx, Y: (float64(j) + .5) * vh / ny}
			ok := false
			for _, l := range h.layers {
				if l.grp == gfx.GroupTiles && l.img && !l.hidden && l.frame.Min.X <= p.X && p.X < l.frame.Max.X && l.frame.Min.Y <= p.Y && p.Y < l.frame.Max.Y {
					ok = true
					break
				}
			}
			if !ok {
				miss++
			}
		}
	}
	return float64(miss) / (nx * ny)
}

type lagRig struct {
	t    *testing.T
	d    *docCore
	sc   *gfx.Scene
	comp *gfx.Compositor
	host *fakeHost
	root *gfx.Node
	ov   *gfx.Node
	t0   time.Time
	seq  uint64
}

const lvw, lvh, ldpr = 1400.0, 900.0, 2.0

func (r *lagRig) show(n *scene.Node) gfx.Rect {
	var b gfx.Rect
	r.sc.Update(func(tx *gfx.Tx) {
		if r.root != nil {
			tx.Remove(r.root)
		}
		r.root = gfx.NewNode(1)
		tx.Add(nil, r.root)
		if _, err := r.d.stree.BuildElement(tx, r.root, n, scene.Options{Measurer: gfx.DefaultMeasurer, FirstID: 2}); err != nil {
			r.t.Fatal(err)
		}
	})
	// the compositor's Frame commits r.sc and takes the damage; bounds come from a scratch scene
	tmp := gfx.NewScene()
	tmp.Update(func(tx *gfx.Tx) {
		root := gfx.NewNode(1)
		tx.Add(nil, root)
		r.d.stree.BuildElement(tx, root, n, scene.Options{Measurer: gfx.DefaultMeasurer, FirstID: 2})
	})
	tmp.Commit()
	tmp.Read(func() { b = tmp.Root().SubtreeBounds() })
	return b
}

func fit(b gfx.Rect) gfx.Camera {
	z := math.Min(lvw/b.W(), lvh/b.H()) * 0.95
	cx, cy := b.Min.X+b.W()/2, b.Min.Y+b.H()/2
	return gfx.Camera{Origin: gfx.Point{X: cx - lvw/z/2, Y: cy - lvh/z/2}, Zoom: z}
}

type frameRec struct {
	at    time.Duration
	blank float64
	cpu   time.Duration
	app   int
	pend  int
}

// tick runs one 60 Hz frame.
func (r *lagRig) tick() frameRec {
	time.Sleep(16 * time.Millisecond)
	r.seq++
	r.comp.Frame(gfx.FrameInfo{Time: time.Since(r.t0), Interval: 16666 * time.Microsecond, Seq: r.seq})
	st := r.comp.Stats()
	return frameRec{time.Since(r.t0), r.host.blank(lvw, lvh), st.FrameTime, st.TilesApplied, st.TilesPending}
}

// settle runs frames until the compositor has nothing to do; returns the time it took.
func (r *lagRig) settle() time.Duration {
	s := time.Now()
	for i := 0; i < 400; i++ {
		r.tick()
		if !r.comp.NeedsFrame() && r.comp.Stats().TilesPending == 0 {
			break
		}
	}
	return time.Since(s)
}

func summarize(t *testing.T, name string, recs []frameRec, start time.Duration) {
	var blankFrames int
	var maxBlank float64
	var blankUntil, sharpAt time.Duration
	var maxCPU time.Duration
	var sumBlank float64
	for _, f := range recs {
		if f.blank > 0 {
			blankFrames++
			blankUntil = f.at - start
		}
		sumBlank += f.blank
		maxBlank = math.Max(maxBlank, f.blank)
		if f.cpu > maxCPU {
			maxCPU = f.cpu
		}
		if f.pend > 0 || f.app > 0 {
			sharpAt = f.at - start
		}
	}
	t.Logf("%-34s frames=%d blankFrames=%d maxBlank=%.0f%% meanBlank=%.1f%% lastBlankAt=%dms lastTileActivity=%dms maxFrameCPU=%.1fms",
		name, len(recs), blankFrames, maxBlank*100, sumBlank/float64(len(recs))*100, blankUntil.Milliseconds(), sharpAt.Milliseconds(), ms(maxCPU))
}

func TestLagComp(t *testing.T) {
	path := os.Getenv("LAGDOC")
	if path == "" {
		t.Skip("LAGDOC not set")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doc := ParseText(string(data))
	doc.Path = path
	d := &docCore{state: NewState(doc)}
	sess, err := editor.New(d.state.Current, editor.Options{
		Render: render.Options{ScreenShadows: true}, OnModelChange: func() { d.sessionChanged() },
		OnElement: func() { d.state.Current = d.session.Element() },
	})
	if err != nil {
		t.Fatal(err)
	}
	d.session = sess
	d.adopt(doc.Text)
	byName := map[string]*scene.Node{}
	var walk func(n *scene.Node, p string)
	walk = func(n *scene.Node, p string) {
		for _, c := range n.Elems {
			byName[p+c.Name] = c
			walk(c, p+c.Name+" > ")
		}
	}
	walk(d.file(), "")
	pick := func(sub string) *scene.Node {
		for k, n := range byName {
			if strings.Contains(k, sub) {
				return n
			}
		}
		t.Fatalf("no element %q", sub)
		return nil
	}
	A, B := pick(os.Getenv("LAGA")), pick(os.Getenv("LAGB"))

	r := &lagRig{t: t, d: d, sc: gfx.NewScene(), host: &fakeHost{}, t0: time.Now()}
	r.comp = gfx.NewCompositor(r.sc, cg.NewRenderer(cg.Options{TextCache: true, ShadowSprite: true, LOD: true}), r.host, gfx.Options{})
	defer r.comp.Close()
	r.comp.SetViewport(lvw, lvh, ldpr)
	t.Logf("workers=%d", r.comp.Workers())

	bA := r.show(A)
	r.comp.SetCamera(fit(bA))
	cold := r.settle()
	t.Logf("cold open of A (%v): settled in %dms", os.Getenv("LAGA"), cold.Milliseconds())

	// 1. element switch with a fit camera, as realizeScene + applyInitialCamera do
	for i := 0; i < 3; i++ {
		el, bb := B, (gfx.Rect{})
		if i%2 == 1 {
			el = A
		}
		start := time.Since(r.t0)
		bb = r.show(el)
		r.comp.SetCamera(fit(bb))
		var recs []frameRec
		for k := 0; k < 120; k++ {
			f := r.tick()
			recs = append(recs, f)
			if k > 3 && !r.comp.NeedsFrame() && f.pend == 0 {
				break
			}
		}
		summarize(t, "switch #"+string(rune('1'+i)), recs, start)
		r.settle()
	}

	// 2. fast zoom out x8 in 12 frames from zoom 1 (centered), then back in, 3 cycles without pause
	bb := r.show(A)
	cx, cy := bb.Min.X+bb.W()/2, bb.Min.Y+bb.H()/2
	r.comp.SetCamera(gfx.Camera{Origin: gfx.Point{X: cx - lvw/2, Y: cy - lvh/2}, Zoom: 1})
	r.settle()
	zoomTo := func(name string, from, to float64, frames int, pause int) {
		start := time.Since(r.t0)
		var recs []frameRec
		for k := 1; k <= frames; k++ {
			z := from * math.Pow(to/from, float64(k)/float64(frames))
			r.comp.SetCamera(gfx.Camera{Origin: gfx.Point{X: cx - lvw/z/2, Y: cy - lvh/z/2}, Zoom: z})
			recs = append(recs, r.tick())
		}
		for k := 0; k < pause; k++ {
			recs = append(recs, r.tick())
		}
		summarize(t, name, recs, start)
	}
	zoomTo("zoom out 1->0.125 (12f) + 30f wait", 1, 0.125, 12, 30)
	r.settle()
	zoomTo("zoom in  0.125->1 (12f) + 30f wait", 0.125, 1, 12, 30)
	r.settle()
	for c := 0; c < 3; c++ {
		zoomTo("back-forth out 1->0.25 (8f)", 1, 0.25, 8, 0)
		zoomTo("back-forth in  0.25->1 (8f)", 0.25, 1, 8, 6)
	}
	r.settle()
	// 3. slow-ish zoom out x2 then wait (typical pinch)
	zoomTo("zoom out 1->0.5 (10f) + 30f", 1, 0.5, 10, 30)
	r.settle()

	// sweep of the zoom-out factor from a settled zoom 1, 6 frames, then 40 frames of waiting
	for _, f := range []float64{1.5, 2, 3, 4, 8} {
		r.comp.SetCamera(gfx.Camera{Origin: gfx.Point{X: cx - lvw/2, Y: cy - lvh/2}, Zoom: 1})
		r.settle()
		zoomTo(fmt.Sprintf("sweep out x%.1f (6f)+40f", f), 1, 1/f, 6, 40)
	}
	// selection overlay redrawn on every zoom frame, as the FrameReport handler does (drawSceneOverlay)
	for _, sel := range []float64{0, 1, 2} {
		bb := r.show(A)
		r.ov = nil
		var hud *gfx.Node // sel 2: the frame lives on the HUD, in view points
		if sel == 2 {
			hud = gfx.NewNode(3)
			r.comp.HUD().Update(func(tx *gfx.Tx) { tx.Add(nil, hud) })
		}
		if sel == 1 {
			r.sc.Update(func(tx *gfx.Tx) {
				r.ov = gfx.NewNode(1 << 20)
				tx.Add(r.root, r.ov)
				tx.SetZ(r.ov, 1<<20)
			})
		}
		redraw := func(z float64) {
			if hud != nil {
				cam := r.comp.Camera()
				rc := gfx.NewRecorder()
				rc.StrokeRect(gfx.R((bb.Min.X+200-cam.Origin.X)*cam.Zoom, (bb.Min.Y+200-cam.Origin.Y)*cam.Zoom, (bb.W()-400)*cam.Zoom, (bb.H()-400)*cam.Zoom), gfx.StrokeStyle{Width: 2}, accent)
				pic := rc.Finish()
				r.comp.HUD().Update(func(tx *gfx.Tx) { tx.SetPicture(hud, pic) })
				return
			}
			if r.ov == nil {
				return
			}
			rc := gfx.NewRecorder()
			rc.StrokeRect(gfx.R(bb.Min.X+200, bb.Min.Y+200, bb.W()-400, bb.H()-400), gfx.StrokeStyle{Width: 2 / z}, accent)
			pic := rc.Finish()
			r.sc.Update(func(tx *gfx.Tx) { tx.SetPicture(r.ov, pic) })
		}
		r.comp.SetCamera(gfx.Camera{Origin: gfx.Point{X: cx - lvw/2, Y: cy - lvh/2}, Zoom: 1})
		redraw(1)
		r.settle()
		before := r.comp.Stats().Rastered
		start := time.Since(r.t0)
		var recs []frameRec
		for k := 1; k <= 12; k++ {
			z := math.Pow(0.2, float64(k)/12)
			r.comp.SetCamera(gfx.Camera{Origin: gfx.Point{X: cx - lvw/z/2, Y: cy - lvh/z/2}, Zoom: z})
			redraw(z)
			recs = append(recs, r.tick())
		}
		for k := 0; k < 30; k++ {
			recs = append(recs, r.tick())
		}
		summarize(t, fmt.Sprintf("zoom out x5 12f, overlay=%v", []string{"none", "scene", "HUD"}[int(sel)]), recs, start)
		t.Logf("  tiles rastered during it: %d", r.comp.Stats().Rastered-before)
		if hud != nil {
			r.comp.HUD().Update(func(tx *gfx.Tx) { tx.Remove(hud) })
		}
		r.ov = nil
	}
	// pan speeds (pt/s), 30 frames
	for _, sp := range []float64{3000, 6000, 12000, 24000} {
		r.comp.SetCamera(gfx.Camera{Origin: gfx.Point{X: cx - lvw/2, Y: cy - lvh/2}, Zoom: 1})
		r.settle()
		cam := r.comp.Camera()
		start := time.Since(r.t0)
		var recs []frameRec
		for k := 0; k < 30; k++ {
			cam.Origin.X += sp / 60.0 / cam.Zoom
			r.comp.SetCamera(cam)
			recs = append(recs, r.tick())
		}
		summarize(t, fmt.Sprintf("pan %.0fpt/s 30f", sp), recs, start)
	}
}

// TestLagCache compares building the gfx tree of an element with reattaching a detached one.
func TestLagCache(t *testing.T) {
	path := os.Getenv("LAGDOC")
	if path == "" {
		t.Skip("LAGDOC not set")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doc := ParseText(string(data))
	doc.Path = path
	d := &docCore{state: NewState(doc)}
	sess, err := editor.New(d.state.Current, editor.Options{Render: render.Options{ScreenShadows: true}})
	if err != nil {
		t.Fatal(err)
	}
	d.session = sess
	d.adopt(doc.Text)
	var nodes []*scene.Node
	for _, key := range []string{os.Getenv("LAGA"), os.Getenv("LAGB")} {
		var find func(n *scene.Node, p string)
		find = func(n *scene.Node, p string) {
			for _, c := range n.Elems {
				if p+c.Name == key {
					nodes = append(nodes, c)
				}
				find(c, p+c.Name+" > ")
			}
		}
		find(d.file(), "")
	}
	if len(nodes) != 2 {
		t.Fatalf("elements not found: %d", len(nodes))
	}
	sc := gfx.NewScene()
	roots := make([]*gfx.Node, 2)
	for i, n := range nodes {
		t0 := time.Now()
		sc.Update(func(tx *gfx.Tx) {
			roots[i] = gfx.NewNode(1)
			tx.Add(nil, roots[i])
			d.stree.BuildElement(tx, roots[i], n, scene.Options{Measurer: gfx.DefaultMeasurer, FirstID: 2})
		})
		tb := time.Since(t0)
		t1 := time.Now()
		sc.Commit()
		t.Logf("build #%d: %.1fms + commit %.1fms", i, ms(tb), ms(time.Since(t1)))
		sc.Update(func(tx *gfx.Tx) { tx.Remove(roots[i]) })
		sc.Commit()
	}
	for k := 0; k < 4; k++ {
		i := k % 2
		t0 := time.Now()
		sc.Update(func(tx *gfx.Tx) { tx.Add(nil, roots[i]) })
		ta := time.Since(t0)
		t1 := time.Now()
		sc.Commit()
		t.Logf("reattach #%d: %.2fms + commit %.1fms", i, ms(ta), ms(time.Since(t1)))
		sc.Update(func(tx *gfx.Tx) { tx.Remove(roots[i]) })
		sc.Commit()
	}
}
