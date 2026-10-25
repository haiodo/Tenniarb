//go:build darwin || linux || windows

package app

import (
	"unicode"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/swt"

	"github.com/haiodo/tenniarb/go/editor"
)

// KeyName is the DOM spelling editor.KeyEvent wants for an SWT key code and character; "" for a key
// the editor has no use for (modifiers alone).
func KeyName(code int, char rune) string {
	switch int32(code) {
	case int32(swt.TAB):
		return "Tab"
	case int32(swt.CR), swt.KEYPAD_CR:
		return "Enter"
	case int32(swt.BS):
		return "Backspace"
	case int32(swt.DEL):
		return "Delete"
	case swt.ARROW_LEFT:
		return "ArrowLeft"
	case swt.ARROW_RIGHT:
		return "ArrowRight"
	case swt.ARROW_UP:
		return "ArrowUp"
	case swt.ARROW_DOWN:
		return "ArrowDown"
	case int32(swt.SPACE):
		return " "
	}
	// With Cmd held the character is a control code or empty; the key code is the lower-case letter.
	if code > ' ' && code < unicode.MaxASCII && unicode.IsLetter(rune(code)) {
		return string(unicode.ToLower(rune(code)))
	}
	if char > ' ' && unicode.IsPrint(char) {
		return string(char)
	}
	return ""
}

// ModsOf turns the modifiers of a gowt event into the editor's. Cmd is Meta.
func ModsOf(m g.Mods) editor.Modifiers {
	var out editor.Modifiers
	if m.Shift {
		out |= editor.ModShift
	}
	if m.Ctrl {
		out |= editor.ModCtrl
	}
	if m.Alt {
		out |= editor.ModAlt
	}
	if m.Cmd {
		out |= editor.ModMeta
	}
	return out
}

// menuOwned reports whether a key chord is a menu shortcut (Undo, Redo, Duplicate, Select All, Cut,
// Copy, Paste), which the menu runs; the canvas must not run it a second time.
func menuOwned(name string, m g.Mods, cmdIsCtrl bool) bool {
	cmd := m.Cmd
	if cmdIsCtrl {
		cmd = m.Ctrl
	}
	if !cmd {
		return false
	}
	switch name {
	case "z", "d", "a", "x", "c", "v":
		return true
	}
	return false
}
