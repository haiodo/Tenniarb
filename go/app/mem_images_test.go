//go:build darwin

package app

import (
	"os"
	"runtime"
	"testing"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/gowt/gfx/offscreen"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
)

// TestMemImages: a classic element with inline images is edited n times; each edit rebuilds the scene
// and the tiles are drawn by one long-lived cg renderer, as the compositor does. Skipped unless MEMDOC
// is set. Headless: offscreen renderer, no window.
func TestMemImages(t *testing.T) {
	path := os.Getenv("MEMDOC")
	elName := os.Getenv("MEMELEMENT")
	if path == "" || elName == "" {
		t.Skip("MEMDOC / MEMELEMENT not set")
	}
	doc, err := Load(path)
	if err != nil || doc.Model == nil {
		t.Fatal(err, doc.Errors)
	}
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
	var el *model.Element
	walkElements(&d.session.Root().Element, func(e *model.Element) {
		if e.Name == elName {
			el = e
		}
	})
	if el == nil {
		t.Fatal("no element")
	}
	d.session.SetElement(el)
	r, err := offscreen.New(offscreen.Options{})
	if err != nil {
		t.Fatal(err)
	}
	sc := gfx.NewScene()
	var rendered *render.Rendered
	draw := func(i int) {
		ds := d.session.Scene()
		sc.Update(func(tx *gfx.Tx) {
			if rendered != nil {
				tx.Remove(rendered.Root)
			}
			rendered = ds.Realize(tx, nil)
		})
		sc.Commit()
		// the tiles in view: the whole element at scale 1 stands for them
		raster, err := gfx.RenderSubtree(r, sc, nil, gfx.Rect{}, 1, gfx.RGBA(1, 1, 1, 1))
		if err != nil {
			t.Fatal(err)
		}
		_ = raster
	}
	var items []*model.DiagramItem
	items = append(items, el.Items[0])
	memMark(t, "img 0 before")
	draw(0)
	memMark(t, "img 1 first draw")
	for i := 0; i < 10; i++ {
		d.session.Select(items)
		d.session.MoveBy(1, 1)
		d.session.EndEvent()
		draw(i + 1)
		if i == 0 || i == 4 || i == 9 {
			memMark(t, "img after edit")
		}
	}
	memHeapProfile(t, "img")
	runtime.KeepAlive(r)
}
