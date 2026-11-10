//go:build darwin || linux || windows

package app

import "testing"

// The pane keeps the draft of the user while the code echoes it, and follows the code otherwise: an
// undo, a redo or a control of the inspector changes the properties behind the caret.
func TestPropsKeepDraft(t *testing.T) {
	const blue, green = "name \"Item\"\ncolor blue", "name \"Item\"\ncolor green"
	for _, c := range []struct {
		name                    string
		focused, same, applying bool
		want, shown             string
		keep                    bool
	}{
		{"the echo of the pane's own edit keeps the draft", true, true, true, blue, green, true},
		{"an undo while the pane has the keyboard", true, true, false, green, blue, false},
		{"a redraw that changes nothing keeps the draft", true, true, false, blue, blue, true},
		{"a click on another item", true, false, false, blue, blue, false},
		{"the keyboard is elsewhere", false, true, false, green, blue, false},
	} {
		if got := propsKeepDraft(c.focused, c.same, c.applying, c.want, c.shown); got != c.keep {
			t.Errorf("%s: keep the draft %v, want %v", c.name, got, c.keep)
		}
	}
}
