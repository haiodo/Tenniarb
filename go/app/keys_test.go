//go:build darwin || linux || windows

package app

import (
	"testing"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/swt"

	"github.com/haiodo/tenniarb/go/editor"
)

func TestKeyName(t *testing.T) {
	cases := []struct {
		code int
		char rune
		want string
	}{
		{int(swt.TAB), '\t', "Tab"},
		{int(swt.CR), '\r', "Enter"},
		{int(swt.BS), 8, "Backspace"},
		{int(swt.DEL), 127, "Delete"},
		{int(swt.ARROW_UP), 0, "ArrowUp"},
		{int(swt.SPACE), ' ', " "},
		{'z', 26, "z"},  // Cmd+Z: control character
		{'a', 'A', "a"}, // Shift
		{'x', 'x', "x"},
		{int(swt.SHIFT), 0, ""},
	}
	for _, c := range cases {
		if got := KeyName(c.code, c.char); got != c.want {
			t.Errorf("KeyName(%d,%q) = %q, want %q", c.code, c.char, got, c.want)
		}
	}
}

func TestModsAndMenuKeys(t *testing.T) {
	m := ModsOf(g.Mods{Cmd: true, Shift: true})
	if !m.Has(editor.ModMeta) || !m.Has(editor.ModShift) || m.Has(editor.ModCtrl) {
		t.Fatalf("mods %v", m)
	}
	if !menuOwned("z", g.Mods{Cmd: true}, false) || menuOwned("z", g.Mods{Ctrl: true}, false) || !menuOwned("z", g.Mods{Ctrl: true}, true) {
		t.Fatal("menuOwned")
	}
	if menuOwned("ArrowUp", g.Mods{Cmd: true}, false) || menuOwned("z", g.Mods{}, false) {
		t.Fatal("arrows and plain keys stay with the canvas")
	}
}
