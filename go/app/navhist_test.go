//go:build darwin || linux || windows

package app

import (
	"strconv"
	"testing"
)

func sel(n string) Selection { return Selection{Element: []string{"E"}, Nodes: []string{n}} }

func TestNavHistory(t *testing.T) {
	all := func(Selection) bool { return true }
	var h navHistory
	h.push(sel("a"))
	h.push(sel("a")) // the entry on screen is not repeated
	h.push(sel("b"))
	h.push(sel("c"))
	if len(h.list) != 3 || h.can(1, all) || !h.can(-1, all) {
		t.Fatalf("after pushes: %+v", h)
	}
	if s, ok := h.move(-1, all); !ok || s.Nodes[0] != "b" {
		t.Fatalf("back: %v %v", s, ok)
	}
	if s, ok := h.move(1, all); !ok || s.Nodes[0] != "c" {
		t.Fatalf("forward: %v %v", s, ok)
	}
	h.move(-1, all)
	h.move(-1, all)
	if _, ok := h.move(-1, all); ok {
		t.Fatal("back past the first entry")
	}
	h.move(1, all)   // on b
	h.push(sel("d")) // a new entry drops c
	if len(h.list) != 3 || h.can(1, all) || h.list[2].Nodes[0] != "d" {
		t.Fatalf("truncate: %+v", h)
	}
}

func TestNavHistorySkipsInvalid(t *testing.T) {
	var h navHistory
	for _, n := range []string{"a", "gone", "gone2", "d"} {
		h.push(sel(n))
	}
	valid := func(s Selection) bool { return s.Nodes[0] == "a" || s.Nodes[0] == "d" }
	if s, ok := h.move(-1, valid); !ok || s.Nodes[0] != "a" {
		t.Fatalf("back skips: %v %v", s, ok)
	}
	if s, ok := h.move(1, valid); !ok || s.Nodes[0] != "d" {
		t.Fatalf("forward skips: %v %v", s, ok)
	}
	h.move(-1, valid)
	if _, ok := h.move(-1, valid); ok || h.at != 0 {
		t.Fatal("no valid entry back: the position must stay")
	}
}

func TestNavHistoryCap(t *testing.T) {
	var h navHistory
	for i := range navMax + 20 {
		h.push(sel(strconv.Itoa(i)))
	}
	if len(h.list) != navMax || h.list[0].Nodes[0] != "20" || h.at != navMax-1 {
		t.Fatalf("cap: len %d first %v at %d", len(h.list), h.list[0], h.at)
	}
}
