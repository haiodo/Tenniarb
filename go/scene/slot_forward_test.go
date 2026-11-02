package scene_test

import (
	"strings"
	"testing"

	"github.com/haiodo/gowt/gfx"
	"github.com/haiodo/tenniarb/go/scene"
)

// forwardLib is a component with a named slot, and a wrapper that uses it and declares no slot of its own.
const forwardLib = `item "Card" {
    auto-layout column
    width 100; height 100
    slot "content" { item "Fallback" { width 1; height 1 } }
}
item "Page" {
    auto-layout column
    width 200; height 200
    use Card as card
}
`

func forwardScreen(t *testing.T, lib, block string) (*scene.Screen, *scene.Tree) {
	t.Helper()
	tree, err := scene.LoadSource(t.TempDir(), "f.tenn",
		lib+"element \"Screen\" {\n  use Page as p {\n"+block+"\n  }\n}\n")
	if err != nil {
		t.Fatal(err)
	}
	sc := gfx.NewScene()
	var screen *scene.Screen
	sc.Update(func(tx *gfx.Tx) { screen, err = tree.Build(tx, nil, "Screen", scene.Options{}) })
	if err != nil {
		t.Fatal(err)
	}
	return screen, tree
}

// TestSlotForwardedThroughWrapper: a fill of a name the wrapper does not declare goes to the slot of the
// component it uses (spec 4.7a).
func TestSlotForwardedThroughWrapper(t *testing.T) {
	screen, tree := forwardScreen(t, forwardLib, `    slot content { item "Passed" { width 3; height 3 } }`)
	m := boxMap(screen)
	wantBox(t, m, "p.card.Passed", 0, 0, 3, 3)
	if _, ok := m["p.card.Fallback"]; ok {
		t.Errorf("the fallback is still drawn: %v", paths(screen))
	}
	if len(tree.Diagnostics()) > 0 || len(screen.Warnings) > 0 {
		t.Errorf("diagnostics %v, warnings %v", tree.Diagnostics(), screen.Warnings)
	}
}

// TestSlotForwardedTwoLevels: the wrapper of a wrapper passes the name down again.
func TestSlotForwardedTwoLevels(t *testing.T) {
	lib := `item "Card" {
    auto-layout column
    width 100; height 100
    slot "content" { item "Fallback" { width 1; height 1 } }
}
item "Middle" {
    auto-layout column
    width 150; height 150
    use Card as card
}
item "Page" {
    auto-layout column
    width 200; height 200
    use Middle as mid
}
`
	screen, _ := forwardScreen(t, lib, `    slot content { item "Passed" { width 3; height 3 } }`)
	wantBox(t, boxMap(screen), "p.mid.card.Passed", 0, 0, 3, 3)
}

// TestSlotOwnWinsOverForwarded: a wrapper that declares the slot itself takes the fill; the slots of the
// components it uses keep their fallback.
func TestSlotOwnWinsOverForwarded(t *testing.T) {
	lib := `item "Card" {
    auto-layout column
    width 100; height 100
    slot "content" { item "Fallback" { width 1; height 1 } }
}
item "Page" {
    auto-layout column
    width 200; height 200
    slot "content" { item "OwnFallback" { width 2; height 2 } }
    use Card as card
}
`
	screen, _ := forwardScreen(t, lib, `    slot content { item "Passed" { width 3; height 3 } }`)
	m := boxMap(screen)
	wantBox(t, m, "p.Passed", 0, 0, 3, 3)
	wantBox(t, m, "p.card.Fallback", 0, 0, 1, 1)
	if _, ok := m["p.OwnFallback"]; ok {
		t.Errorf("the own fallback is still drawn: %v", paths(screen))
	}
}

// TestSlotForwardAmbiguous: two components of the wrapper hold the name, so the fill names neither and
// the report lists both.
func TestSlotForwardAmbiguous(t *testing.T) {
	lib := `item "Card" {
    auto-layout column
    width 100; height 100
    slot "content" { item "Fallback" { width 1; height 1 } }
}
item "Page" {
    auto-layout column
    width 200; height 200
    use Card as a
    use Card as b
}
`
	_, err := scene.LoadSource(t.TempDir(), "f.tenn",
		lib+"element \"Screen\" {\n  use Page as p {\n    slot content { item \"Passed\" { width 3; height 3 } }\n  }\n}\n")
	if err == nil {
		t.Fatal("two components with the same slot name must be an error")
	}
	for _, want := range []string{`slot "content" is in 2 nested components`, "a.content", "b.content"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error lacks %q: %v", want, err)
		}
	}
}

// TestSlotNotForwarded: a name no component below holds is still an error, as before.
func TestSlotNotForwarded(t *testing.T) {
	_, err := scene.LoadSource(t.TempDir(), "f.tenn",
		forwardLib+"element \"Screen\" {\n  use Page as p {\n    slot nosuch { item \"Passed\" { width 3; height 3 } }\n  }\n}\n")
	if err == nil || !strings.Contains(err.Error(), `unknown slot "nosuch"`) {
		t.Fatalf("error = %v, want an unknown slot", err)
	}
}
