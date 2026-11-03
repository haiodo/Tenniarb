package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

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
			Path   string
			Fill   string
			Shadow struct{ DX, DY, Blur float64 }
			Rect   struct{ X, Y, W, H float64 }
			Radius []float64
			Text   []string
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
	if box.Shadow.DX != 2 || box.Shadow.DY != -3 || box.Shadow.Blur != 4 {
		t.Errorf("shadow = %+v", box.Shadow)
	}
	if len(box.Text) != 1 || box.Text[0] != "Hi" {
		t.Errorf("text = %v", box.Text)
	}
}
