//go:build darwin || linux || windows

package app_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/haiodo/gowt/qa"
	"github.com/haiodo/gowt/qa/qatest"
)

// spinDoc is a scene with an animation on one item: the inspector offers it for that item.
const spinDoc = `element "Spin" {
  animation spin {
    from { rotate 0 }
    to { rotate 360 }
  }
  item canvas {
    auto-layout row
    padding 40
    color #ffffff
    item Wheel {
      width 24; height 24; corner-radius 12; color #6b3ad3
      animation spin 1.2s linear infinite
    }
    item Plain { width 24; height 24; corner-radius 12; color #cf13a2 }
  }
}
`

// animState is what the QA state says about the playback.
type animState struct {
	Paused bool `json:"paused"`
}

// TestSceneAnimationPanel: the inspector shows the Animation section for an animated item only, the Play
// switch pauses the playback and the Frame slider scrubs through it.
func TestSceneAnimationPanel(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	file := restDoc(t, h, "Spin.tenn", spinDoc)
	visible := func(id string) bool {
		f := a.Find(qa.ID(id))
		return f.Exists() && f.Node().Visible
	}
	paused := func() bool {
		var s struct {
			Anim animState `json:"anim"`
		}
		if err := json.Unmarshal(a.State("state"), &s); err != nil {
			t.Fatal(err)
		}
		return s.Anim.Paused
	}

	// the animated item: the section is there, the animation plays
	a.State("select", "path", file, "element", []string{"Spin"}, "nodes", []string{"canvas.Wheel"})
	restWait(a)
	if !visible("scene-anim-play") || !visible("scene-anim-frame") {
		t.Fatal("no Animation section for an item with an animation")
	}
	if paused() {
		t.Error("the playback starts paused")
	}

	// the switch pauses it, the slider stops it at a frame
	a.State("anim", "play", false)
	restWait(a)
	if !paused() {
		t.Error("the Play switch did not pause the playback")
	}
	a.State("anim", "at", 0.5)
	restWait(a)
	if !paused() {
		t.Error("the scrub did not pause the playback")
	}
	a.State("anim", "play", true)
	restWait(a)
	if paused() {
		t.Error("the playback did not resume")
	}

	// an item without an animation has no section
	a.State("select", "path", file, "element", []string{"Spin"}, "nodes", []string{"canvas.Plain"})
	restWait(a)
	if visible("scene-anim-play") {
		t.Error("an item without an animation still shows the Animation section")
	}
}
