package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestGoldensAreCurrent: every golden of the web parity test is what this build makes of its fixture.
// A Go change that alters a picture fails here instead of leaving the web test green against an old one.
func TestGoldensAreCurrent(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "web", "fixtures", "scene")
	goldens, err := filepath.Glob(filepath.Join(dir, "*.layout.json"))
	if err != nil || len(goldens) == 0 {
		t.Fatalf("fixture goldens under %s: %v", dir, err)
	}
	for _, golden := range goldens {
		name := filepath.Base(golden)
		name = name[:len(name)-len(".layout.json")]
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatal(err)
		}
		var header struct{ Element string }
		if err := json.Unmarshal(want, &header); err != nil {
			t.Fatalf("%s: %v", golden, err)
		}
		got, err := layout(dir, name+".tenn", header.Element)
		if err != nil {
			t.Fatalf("%s: %v", golden, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s is stale: regenerate it from %s", golden, name+".tenn")
		}
	}
}

func TestLayout(t *testing.T) {
	const src = `element "E" {
    item Box {
        width 40; height 20; color #112233; corner-radius 4
        shadow 2 3 4 #ff000080
        title "Hi"
    }
}
`
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.tenn"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := layout(root, "a.tenn", "E")
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Width, Height float64
		Nodes         []struct {
			Path    string
			Fill    string
			Shadows []struct{ DX, DY, Blur float64 }
			Rect    struct{ X, Y, W, H float64 }
			Radius  []float64
			Text    []string
		}
		Links []struct {
			Parent string
			Path   [][]any
			Color  string
			Width  float64
			Dash   []float64
			Arrow  bool
		}
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Nodes) != 2 {
		t.Fatalf("%d nodes, want the element and its item: %s", len(got.Nodes), out)
	}
	box := got.Nodes[1]
	if box.Path != "Box" || box.Rect.W != 40 || box.Rect.H != 20 {
		t.Errorf("box = %+v", box)
	}
	if box.Fill != "#112233" || box.Radius[0] != 4 {
		t.Errorf("fill %q radius %v", box.Fill, box.Radius)
	}
	// shadow dx dy is y-up, the screen offset is -dy.
	if len(box.Shadows) != 1 || box.Shadows[0].DX != 2 || box.Shadows[0].DY != -3 || box.Shadows[0].Blur != 4 {
		t.Errorf("shadows = %+v", box.Shadows)
	}
	if len(box.Text) != 1 || box.Text[0] != "Hi" {
		t.Errorf("text = %v", box.Text)
	}
}

func TestTwoShadowsAndElbowLink(t *testing.T) {
	const src = `element "E" {
    item Stage {
        width 300; height 200; auto-layout abs
        item A { pos 20 -20; width 60; height 40; color #ffffff }
        item B { pos 200 -140; width 60; height 40; color #ffffff }
        link A B { route elbow right left; line-style dashed; display arrow; color #ff0000 }
    }
}
`
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.tenn"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := layout(root, "a.tenn", "E")
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Nodes []struct {
			Path    string
			Shadows []struct{ DX, DY, Blur float64 }
		}
		Links []struct {
			Parent string
			Path   [][]any
			Color  string
			Width  float64
			Dash   []float64
			Arrow  bool
		}
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Links) != 1 {
		t.Fatalf("%d links, want 1: %s", len(got.Links), out)
	}
	lk := got.Links[0]
	if lk.Parent != "Stage" || lk.Color != "#ff0000" || !lk.Arrow || len(lk.Dash) != 2 || lk.Dash[0] != 5 {
		t.Errorf("link = %+v", lk)
	}
	// An elbow has at least two corners: m, two or three l/q steps, then the last point.
	if len(lk.Path) < 4 || lk.Path[0][0] != "m" {
		t.Errorf("elbow path = %v", lk.Path)
	}
}

func TestTwoShadows(t *testing.T) {
	const src = `element "E" {
    item Box {
        width 40; height 20; color #ffffff
        shadow 1 2 3 #00000020
        shadow 0 4 8 #00000040
    }
}
`
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.tenn"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := layout(root, "a.tenn", "E")
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Nodes []struct {
			Shadows []struct{ DX, DY, Blur float64 }
		}
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	box := got.Nodes[len(got.Nodes)-1]
	if len(box.Shadows) != 2 || box.Shadows[1].Blur != 8 {
		t.Errorf("shadows = %+v", box.Shadows)
	}
}
