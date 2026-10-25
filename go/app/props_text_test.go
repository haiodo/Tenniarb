package app

import (
	"strings"
	"testing"

	"github.com/haiodo/tenniarb/go/model"
)

func spanText(text string, s PropSpan) string {
	r := []rune(text)
	return string(r[s.Start : s.Start+s.Length])
}

func TestPropSpansRuneOffsets(t *testing.T) {
	text := "name \"😀 é\"\nfontSize $(1 + 2)\npos 10 2.5"
	var got []string
	for _, s := range PropSpans(text) {
		got = append(got, string(s.Class)+":"+spanText(text, s))
	}
	want := []string{"symbol:name", "string:\"😀 é\"", "symbol:fontSize", "expression:$(1 + 2)", "symbol:pos", "number:10", "number:2.5"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

func TestPropSpansErrorAfterEmoji(t *testing.T) {
	text := "name \"😀\"\nbad {"
	var errs []string
	for _, s := range PropSpans(text) {
		if s.Error {
			errs = append(errs, spanText(text, s))
		}
	}
	if len(errs) != 1 || errs[0] != "bad {" {
		t.Fatalf("error spans %q", errs)
	}
}

func TestRuneIndex(t *testing.T) {
	idx := runeIndex("a😀b")
	for b, want := range map[int]int{0: 0, 1: 1, 5: 2, 6: 3, 99: 3, -1: 0} {
		if got := idx(b); got != want {
			t.Errorf("byte %d: rune %d, want %d", b, got, want)
		}
	}
}

func TestZoneAt(t *testing.T) {
	for y, want := range map[int]DropZone{20: DropBefore, 22: DropBefore, 26: DropInto, 30: DropInto, 38: DropAfter, 40: DropAfter} {
		if got := ZoneAt(y, 20, 20); got != want {
			t.Errorf("y %d: zone %d, want %d", y, got, want)
		}
	}
}

func TestDropPlace(t *testing.T) {
	m := ParseText("element \"A\" {\n element \"A1\" {}\n element \"A2\" {}\n}\nelement \"B\" {}\nelement \"C\" {}\n").Model
	root := &m.Element
	a, b, c := m.Elements[0], m.Elements[1], m.Elements[2]
	a1 := a.Elements[0]
	cases := []struct {
		name         string
		dragged, tgt *model.Element
		zone         DropZone
		parent       *model.Element
		index        int
		ok           bool
	}{
		{"into", c, b, DropInto, b, 0, true},
		{"before", c, a, DropBefore, root, 0, true},
		{"after", c, a, DropAfter, root, 1, true},
		{"nested after", c, a1, DropAfter, a, 1, true},
		{"empty space", a1, nil, DropInto, root, 3, true},
		{"onto itself", b, b, DropInto, nil, 0, false},
		{"before the next one", a, b, DropBefore, nil, 0, false},
		{"after itself", b, a, DropAfter, nil, 0, false},
		{"into own subtree", a, a1, DropInto, a1, 0, true}, // the session turns this into a copy
	}
	for _, c := range cases {
		p, i, ok := DropPlace(root, c.dragged, c.tgt, c.zone)
		if p != c.parent || i != c.index || ok != c.ok {
			t.Errorf("%s: got %v %d %v", c.name, p, i, ok)
		}
	}
}
