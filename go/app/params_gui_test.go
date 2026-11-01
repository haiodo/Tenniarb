//go:build darwin || linux || windows

package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/haiodo/gowt/qa"
	"github.com/haiodo/gowt/qa/qatest"
)

// paramsInspectorDoc is a component with a parameter of each shape (spec 5c) - a field, a one-of group and
// an any-of group - a component without parameters, and a screen that uses both.
const paramsInspectorDoc = `element "Button" kind Component {
  export let title = "Button"
  export let kind = primary
  export let extras = outlined
  styles {
    primary { color #6b3ad3; text-color #ffffff }
    secondary { color #ffffff; border-color #1725471a }
    outlined { border-color #1725471a }
    rounded { corner-radius 8 }
    kind one-of primary secondary
    extras any-of outlined rounded
  }
  item Btn {
    auto-layout row
    use-style ${kind} ${extras}
    item Label { title ${title}; font-size 14 }
  }
}

element "Plain" kind Component {
  item Box { width 10; height 10 }
}

element "Screen" {
  width 390; height 844
  item Box {
    use Button as save
    use Plain as plain
  }
}
`

// TestSceneParams drives the Parameters section of the inspector: the parameters the component of the
// selected use declares, a style group of the component as a switch or as check boxes and the rest as a
// field, each writing one command in the block of the use; a use of a component without parameters gets no
// section, nor does a part of an instance.
func TestSceneParams(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	file := filepath.Join(t.TempDir(), "Params.tenn")
	if err := os.WriteFile(file, []byte(paramsInspectorDoc), 0o644); err != nil {
		t.Fatal(err)
	}
	// the later tests start from a clean window with the fixture, as they find it
	t.Cleanup(func() {
		h.open(fixture)
		a.State("command", "name", "close-others")
	})
	wait := func() { time.Sleep(300 * time.Millisecond); a.Settle() }
	visible := func(id string) bool {
		f := a.Find(qa.ID(id))
		return f.Exists() && f.Node().Visible
	}
	canvas := a.Find(qa.ID("canvas"))

	h.open(file)
	a.State("command", "name", "close-others")

	// the root of an instance: the parameters of its component, in the rows of the Styles section
	a.State("select", "path", file, "element", []string{"Screen"}, "nodes", []string{"Box.save"})
	wait()
	for _, id := range []string{"scene-param-title", "scene-param-kind", "scene-param-extras-outlined", "scene-param-extras-rounded"} {
		if !visible(id) {
			t.Fatalf("the Parameters section of the use: %s is not shown", id)
		}
	}
	before := h.text()
	a.ShotTitle("params-1-form", h.title(file))

	// a parameter that is no style group: a field, the value written as the declaration writes it
	h.enter("scene-param-title", "Save")
	wait()
	if got := h.text(); !strings.Contains(got, `title "Save"`) {
		t.Fatalf("after the field:\n%s", got)
	}
	a.ShotTitle("params-2-title", h.title(file))
	canvas.Focus()
	a.Key("Mod+Z")
	wait()
	if got := h.text(); got != before {
		t.Fatalf("the field took more than one undo step:\n%s", got)
	}

	// a one-of group: a switch, one command in the block of the use
	a.State("param", "name", "kind", "value", "secondary")
	wait()
	if got := h.text(); !strings.Contains(got, "kind secondary") {
		t.Fatalf("after the switch:\n%s", got)
	}
	canvas.Focus()
	a.Key("Mod+Z")
	wait()
	if got := h.text(); got != before {
		t.Fatalf("the switch took more than one undo step:\n%s", got)
	}

	// an any-of group: a check box, the value written as the list of the styles it keeps
	a.State("select", "path", file, "element", []string{"Screen"}, "nodes", []string{"Box.save"})
	wait()
	a.Find(qa.ID("scene-param-extras-rounded")).Click()
	wait()
	if got := h.text(); !strings.Contains(got, "extras outlined rounded") {
		t.Fatalf("after the check box:\n%s", got)
	}
	a.ShotTitle("params-3-extras", h.title(file))
	canvas.Focus()
	a.Key("Mod+Z")
	wait()
	if got := h.text(); got != before {
		t.Fatalf("the check box took more than one undo step:\n%s", got)
	}

	// a part of an instance: the parameters belong to the instance, not to the part
	a.State("select", "path", file, "element", []string{"Screen"}, "nodes", []string{"Box.save/Btn"})
	wait()
	if visible("scene-param-title") || visible("scene-param-kind") {
		t.Fatal("a part of an instance shows the Parameters section")
	}

	// an instance of a component without parameters gets no section at all
	a.State("select", "path", file, "element", []string{"Screen"}, "nodes", []string{"Box.plain"})
	wait()
	if visible("scene-param-title") || visible("scene-param-kind") || visible("scene-param-extras-outlined") {
		t.Fatal("an instance of a component without parameters shows the Parameters section")
	}
}
