package scene_test

import (
	"testing"

	"github.com/haiodo/gowt/gfx/cg"
	"github.com/haiodo/tenniarb/go/scene"
)

func TestBoldIsWiderOnCoreText(t *testing.T) {
	screen, _ := buildWith(t, "Font", scene.Options{Measurer: cg.NewRenderer(cg.Options{}), FontName: "Helvetica"})
	m := map[string]scene.Box{}
	for _, b := range screen.Boxes() {
		m[b.Path] = b
	}
	if m["Bold"].W <= m["Plain"].W {
		t.Errorf("bold %v is not wider than plain %v", m["Bold"].W, m["Plain"].W)
	}
	if m["Spaced"].W <= m["Plain"].W {
		t.Errorf("letter-spacing %v does not widen plain %v", m["Spaced"].W, m["Plain"].W)
	}
}
