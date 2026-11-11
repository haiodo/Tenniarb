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

// stylesDoc is a component with the groups of spec 5b and a screen that uses it.
const stylesDoc = `element "Button" {
  styles {
    primary { color #6b3ad3; text-color #ffffff }
    secondary { color #ffffff; border-color #1725471a; text-color #172547 }
    ghost { color #00000000; text-color #6b3ad3 }
    large { height 48; padding 0 16 }
    medium { height 40; padding 0 12 }
    outlined { border-color #1725471a }
    kind one-of primary secondary ghost
    size one-of large medium
    extras any-of outlined
  }
  item Btn {
    auto-layout row; gap 8; align center; justify center; corner-radius 8
    use-style primary medium
    item Label { title Button; font-size 14 }
  }
  item Split {
    use-style primary
    use-style medium
    item Mark { title Split }
  }
}

element "Screen" {
  width 390; height 844
  item Box { use Button as save }
}
`

// TestSceneStyles drives the Styles section of the inspector: the groups of the component the selected
// node belongs to, a one-of group as a switch, an any-of group as a check box, each writing one edit.
func TestSceneStyles(t *testing.T) {
	if os.Getenv("GOWT_GUI_TEST") == "" {
		t.Skip("set GOWT_GUI_TEST=1 to open windows")
	}
	h := qaHelpers{t: t, app: qatest.App(t)}
	a := h.app
	dir, err := os.MkdirTemp(os.Getenv("TENNIARB_GUI_DIR"), "scene-styles")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "Styles.tenn")
	if err := os.WriteFile(file, []byte(stylesDoc), 0o644); err != nil {
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

	// the component itself: its item Btn is inside the element that declares the groups
	a.State("select", "path", file, "element", []string{"Button"}, "nodes", []string{"Btn"})
	wait()
	if !visible("scene-style-kind") || !visible("scene-style-extras-outlined") {
		t.Fatalf("the Styles section of Btn: kind %v, extras %v", visible("scene-style-kind"), visible("scene-style-extras-outlined"))
	}
	before := h.text()
	if !strings.Contains(before, "use-style primary medium") {
		t.Fatalf("the fixture:\n%s", before)
	}
	shotTitle(a, "styles-1-groups", h.title(file))

	// a group one-of: the switch takes one of its styles, in place of the default it replaces
	a.State("style", "group", "kind", "value", "secondary")
	wait()
	if got := h.text(); !strings.Contains(got, "use-style secondary medium") {
		t.Fatalf("after the switch:\n%s", got)
	}
	canvas.Focus()
	a.Key("Mod+Z")
	wait()
	if got := h.text(); got != before {
		t.Fatalf("the switch took more than one undo step:\n%s", got)
	}

	// a group any-of: a check box, one click, one edit
	ex, ey := emptyPoint(t, a)
	canvas.ClickAt(ex, ey)
	a.State("select", "path", file, "element", []string{"Button"}, "nodes", []string{"Btn"})
	wait()
	a.Find(qa.ID("scene-style-extras-outlined")).Click()
	wait()
	if got := h.text(); !strings.Contains(got, "use-style primary medium outlined") {
		t.Fatalf("after the check box:\n%s", got)
	}
	shotTitle(a, "styles-2-outlined", h.title(file))
	canvas.Focus()
	a.Key("Mod+Z")
	wait()
	if got := h.text(); got != before {
		t.Fatalf("the check box took more than one undo step:\n%s", got)
	}

	// a part of an instance shows the defaults of its component and writes only the change into the use
	a.State("select", "path", file, "element", []string{"Screen"}, "nodes", []string{"Box.save/Btn"})
	wait()
	if !visible("scene-style-kind") {
		t.Fatal("the Styles section of a part of an instance")
	}
	a.State("style", "group", "kind", "value", "secondary")
	wait()
	if got := h.text(); !strings.Contains(got, "use-style -primary secondary") {
		t.Fatalf("the change of an instance:\n%s", got)
	}

	// a list split in two use-style commands: the style it drops goes by name, the other command stays
	a.State("select", "path", file, "element", []string{"Button"}, "nodes", []string{"Split"})
	wait()
	a.State("style", "group", "kind", "value", "ghost")
	wait()
	if got := h.text(); !strings.Contains(got, "use-style -primary ghost medium") {
		t.Fatalf("a split use-style list:\n%s", got)
	}

	// a node that names no style gets no Styles section: the styles of the component belong to Btn
	a.State("select", "path", file, "element", []string{"Button"}, "nodes", []string{"Btn/Label"})
	wait()
	if visible("scene-style-kind") || visible("scene-style-extras-outlined") {
		t.Fatal("a part of the component that names no style still shows the Styles section")
	}
	a.State("select", "path", file, "element", []string{"Screen"}, "nodes", []string{"Box"})
	wait()
	if visible("scene-style-kind") || visible("scene-style-extras-outlined") {
		t.Fatal("a node with no styles still shows the Styles section")
	}
}
