//go:build darwin || linux || windows

package app

import "slices"

// navMax caps the selections kept for Back and Forward.
const navMax = 100

// navHistory is the selections the Code panel showed, oldest first; at is the one on screen.
type navHistory struct {
	list []Selection
	at   int
}

func sameSelection(a, b Selection) bool {
	return slices.Equal(a.Element, b.Element) && slices.Equal(a.Nodes, b.Nodes) && slices.Equal(a.Items, b.Items)
}

// push makes s the entry on screen and drops what was ahead of it; the entry already on screen is not repeated.
func (h *navHistory) push(s Selection) {
	if len(h.list) > 0 {
		if sameSelection(h.list[h.at], s) {
			return
		}
		h.list = h.list[:h.at+1]
	}
	h.list = append(h.list, s)
	if len(h.list) > navMax {
		h.list = h.list[len(h.list)-navMax:]
	}
	h.at = len(h.list) - 1
}

// find is the index of the nearest entry from the one on screen, dir -1 back and +1 forward, that valid
// accepts; -1 when there is none. Entries valid refuses stay in the list.
func (h *navHistory) find(dir int, valid func(Selection) bool) int {
	for i := h.at + dir; i >= 0 && i < len(h.list); i += dir {
		if valid(h.list[i]) {
			return i
		}
	}
	return -1
}

func (h *navHistory) can(dir int, valid func(Selection) bool) bool { return h.find(dir, valid) >= 0 }

// move puts the nearest valid entry in dir on screen and returns it.
func (h *navHistory) move(dir int, valid func(Selection) bool) (Selection, bool) {
	i := h.find(dir, valid)
	if i < 0 {
		return Selection{}, false
	}
	h.at = i
	return h.list[i], true
}
