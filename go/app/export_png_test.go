//go:build darwin || linux || windows

package app

import (
	"bytes"
	"image/png"
	"testing"

	"github.com/haiodo/gowt/gfx"

	"github.com/haiodo/tenniarb/go/scene"
)

func TestUnionRect(t *testing.T) {
	if _, ok := unionRect(nil); ok {
		t.Fatal("empty union")
	}
	u, _ := unionRect([]gfx.Rect{gfx.R(0, 0, 10, 10), gfx.R(20, 5, 10, 10)})
	if u != gfx.R(0, 0, 30, 15) {
		t.Fatalf("union %v", u)
	}
}

func TestSelectionFileName(t *testing.T) {
	for _, c := range []struct {
		names []string
		want  string
	}{
		{nil, "Untitled.png"},
		{[]string{"Button"}, "Button.png"},
		{[]string{"a/b:c"}, "a_b_c.png"},
		{[]string{"  "}, "Untitled.png"},
		{[]string{"Card", "x", "y"}, "Card+2.png"},
	} {
		if got := selectionFileName(c.names); got != c.want {
			t.Errorf("%v: %q, want %q", c.names, got, c.want)
		}
	}
}

func TestClampExportScale(t *testing.T) {
	for in, want := range map[int]int{-1: 1, 0: 2, 1: 1, 3: 3, 9: 3} {
		if got := clampExportScale(in); got != want {
			t.Errorf("%d: %d, want %d", in, got, want)
		}
	}
}

// The function the menu calls draws a node of a scene into a PNG of the size of the node times the scale.
func TestRenderNodesPNGScene(t *testing.T) {
	sh := &Shell{}
	v := newDoc(t, sh, t.TempDir()+"/s.tenn", "item Screen { pos 0 0; auto-layout row; item A { width 40; height 20; color #ff0000 } }\n")
	s := gfx.NewScene()
	var screen *scene.Screen
	var err error
	s.Update(func(tx *gfx.Tx) {
		screen, err = v.stree.BuildElement(tx, nil, v.elementNode(v.state.Current), scene.Options{})
	})
	if err != nil {
		t.Fatal(err)
	}
	root := screen.Placed()
	if len(root.Kids) == 0 {
		t.Fatal("no kids")
	}
	a := root.Kids[0]
	data, err := renderNodesPNG(s, root.Gfx, []*gfx.Node{a.Gfx}, 2, gfx.Color{})
	if err != nil {
		t.Skipf("no offscreen renderer: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 80 || b.Dy() != 40 {
		t.Fatalf("size %v", b)
	}
	_, _, _, al := img.At(40, 20).RGBA()
	if al == 0 {
		t.Fatal("centre is transparent")
	}
}
