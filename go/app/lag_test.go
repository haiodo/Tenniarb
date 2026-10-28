//go:build darwin

package app

import (
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/cg"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/render"
	"github.com/haiodo/tenniarb/go/scene"
)

// Headless measurement of an element switch (gfx tree build) and of tile rasterization. No window.
// Skipped unless LAGDOC is set.
func TestLagDoc(t *testing.T) {
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
		Render:        render.Options{ScreenShadows: true},
		OnModelChange: func() { d.sessionChanged() },
		OnElement:     func() { d.state.Current = d.session.Element() },
	})
	if err != nil {
		t.Fatal(err)
	}
	d.session = sess
	d.adopt(doc.Text)
	if d.stree == nil {
		t.Fatalf("no scene tree: %s", d.sceneErr)
	}
	var all []*scene.Node
	var collect func(n *scene.Node, path string)
	names := map[*scene.Node]string{}
	collect = func(n *scene.Node, p string) {
		for _, c := range n.Elems {
			all = append(all, c)
			names[c] = p + c.Name
			collect(c, p+c.Name+" > ")
		}
	}
	collect(d.file(), "")
	t.Logf("scene elements: %d", len(all))

	r := cg.NewRenderer(cg.Options{TextCache: true, ShadowSprite: true, LOD: true})
	type row struct {
		name          string
		nodes         int
		build, commit time.Duration
		w, h          float64
		root          *gfx.Node
		sc            *gfx.Scene
	}
	var rows []row
	seen := map[string]bool{}
	for _, n := range all {
		key := n.Pos.File + "|" + n.Name
		if n.Mount != nil {
			key = n.Mount.Pos.File
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		var best row
		for run := 0; run < 3; run++ {
			sc := gfx.NewScene()
			var root *gfx.Node
			t0 := time.Now()
			var err error
			sc.Update(func(tx *gfx.Tx) {
				root = gfx.NewNode(1)
				tx.Add(nil, root)
				_, err = d.stree.BuildElement(tx, root, n, scene.Options{Measurer: gfx.DefaultMeasurer, FirstID: 2})
			})
			tb := time.Since(t0)
			t1 := time.Now()
			sc.Commit()
			tc := time.Since(t1)
			if err != nil || root == nil {
				break
			}
			if run == 0 || tb+tc < best.build+best.commit {
				var b gfx.Rect
				sc.Read(func() { b = root.SubtreeBounds() })
				best = row{names[n], countNodes(root), tb, tc, b.W(), b.H(), root, sc}
			}
		}
		if best.root != nil {
			rows = append(rows, best)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].build+rows[i].commit > rows[j].build+rows[j].commit })
	filter := os.Getenv("LAGMATCH")
	for i, x := range rows {
		if i < 12 || filter != "" && strings.Contains(x.name, filter) {
			t.Logf("BUILD %-60s nodes=%6d build=%7.1fms commit=%6.1fms size=%.0fx%.0f", x.name, x.nodes, ms(x.build), ms(x.commit), x.w, x.h)
		}
	}
	var sum time.Duration
	for _, x := range rows {
		sum += x.build + x.commit
	}
	t.Logf("elements measured=%d mean build+commit=%.1fms", len(rows), ms(sum)/float64(len(rows)))

	// Tile raster of the 3 heaviest and 1 named element.
	pick := rows
	if len(pick) > 3 {
		pick = pick[:3]
	}
	for _, x := range rows {
		if filter != "" && strings.Contains(x.name, filter) {
			pick = append(pick, x)
			break
		}
	}
	const dpr, vw, vh, tilePx = 2.0, 1400.0, 900.0, 512
	for _, x := range pick {
		var b gfx.Rect
		x.sc.Read(func() { b = x.root.SubtreeBounds() })
		for _, zoom := range []float64{1, math.Min(vw/b.W(), vh/b.H()), 0.25} {
			if zoom <= 0 || math.IsInf(zoom, 0) {
				continue
			}
			scale := math.Exp2(math.Ceil(math.Log2(zoom*dpr) - 0.01))
			tw := tilePx / scale
			// view at the center of the element, margin 1 tile
			cx, cy := b.Min.X+b.W()/2, b.Min.Y+b.H()/2
			vr := gfx.R(cx-vw/zoom/2, cy-vh/zoom/2, vw/zoom, vh/zoom)
			x0, y0 := int(math.Floor(vr.Min.X/tw)), int(math.Floor(vr.Min.Y/tw))
			x1, y1 := int(math.Floor(vr.Max.X/tw)), int(math.Floor(vr.Max.Y/tw))
			tg := r.NewTarget(tilePx, tilePx)
			var inView, ring []float64
			var items gfx.Items
			for ty := y0 - 1; ty <= y1+1; ty++ {
				for tx := x0 - 1; tx <= x1+1; tx++ {
					rect := gfx.R(float64(tx)*tw, float64(ty)*tw, tw, tw)
					s0 := time.Now()
					x.sc.Collect(rect, &items)
					if ss, ok := tg.(gfx.ScreenScaler); ok {
						ss.SetScreenScale(dpr)
					}
					tg.Begin(scale, rect.Min, gfx.Color{})
					tg.Draw(items.List)
					img := tg.Snapshot(tilePx/dpr, tilePx/dpr)
					gfx.Release(img)
					el := ms(time.Since(s0))
					if tx >= x0 && tx <= x1 && ty >= y0 && ty <= y1 {
						inView = append(inView, el)
					} else {
						ring = append(ring, el)
					}
				}
			}
			tg.Close()
			t.Logf("TILES %-50s zoom=%.3f scale=%v tilePt=%.0f view=%d tiles %s | ring(margin)=%d tiles %s", x.name, zoom, scale, tw, len(inView), stat(inView), len(ring), stat(ring))
		}
	}
}

func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

func stat(v []float64) string {
	if len(v) == 0 {
		return "-"
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	sum := 0.0
	for _, x := range s {
		sum += x
	}
	return "sum=" + f1(sum) + "ms mean=" + f1(sum/float64(len(s))) + " p90=" + f1(s[len(s)*9/10]) + " max=" + f1(s[len(s)-1])
}

func f1(x float64) string { return strconv.FormatFloat(x, 'f', 1, 64) }
