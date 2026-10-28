//go:build darwin || linux || windows

package app

import (
	"slices"
	"testing"

	"github.com/haiodo/tenniarb/go/markdown"
)

func TestPaletteCells(t *testing.T) {
	cells := paletteCells()
	if cells[0].name != "" || cells[0].row != 0 || cells[0].col != 0 {
		t.Errorf("first cell %+v, want None at 0,0", cells[0])
	}
	seen := map[[2]int]bool{}
	for _, c := range cells {
		pos := [2]int{c.row, c.col}
		if seen[pos] {
			t.Errorf("two cells at row %d col %d", c.row, c.col)
		}
		seen[pos] = true
		if c.name == "" {
			continue
		}
		if _, ok := markdown.ColorNames[c.name]; !ok {
			t.Errorf("%q is not a colour name of the format", c.name)
		}
		if _, ok := colorOf(c.name); !ok {
			t.Errorf("%q does not parse", c.name)
		}
	}
	if want := 1 + len(pickBasics) + len(pickHues)*pickShadeRows; len(cells) != want {
		t.Errorf("%d cells, want %d", len(cells), want)
	}
}

func TestHitCell(t *testing.T) {
	cells := withRecent(paletteCells(), []string{"#112233", "red"})
	for i, c := range cells {
		x, y := cellOrigin(c)
		for _, p := range [][2]int{{x, y}, {x + pickSize - 1, y + pickSize - 1}} {
			if got := hitCell(cells, p[0], p[1]); got != i {
				t.Errorf("cell %d (%+v) at %v: hit %d", i, c, p, got)
			}
		}
	}
	x, y := cellOrigin(pickCell{row: pickRecentRow})
	for _, p := range [][2]int{{-1, 5}, {5, -1}, {x, y - 1}, {x + 5*pickPitch, y}, {1000, 5}} {
		if got := hitCell(cells, p[0], p[1]); got != -1 {
			t.Errorf("point %v: hit %d, want -1", p, got)
		}
	}
}

func TestMoveCell(t *testing.T) {
	cells := withRecent(paletteCells(), []string{"red"})
	at := func(row, col int) int {
		return slices.IndexFunc(cells, func(c pickCell) bool { return c.row == row && c.col == col })
	}
	cases := []struct {
		from     int
		dx, dy   int
		wantRow  int
		wantCol  int
		describe string
	}{
		{at(1, 5), 0, 1, 2, 5, "down keeps the column"},
		{at(1, 15), 0, -1, 0, 10, "up from a column the first row lacks goes to its last cell"},
		{at(6, 3), 0, 1, 7, 0, "down into the recent row"},
		{at(0, 0), 0, -1, 0, 0, "up from the top stays"},
		{at(7, 0), 0, 1, 7, 0, "down from the last row stays"},
		{at(0, 0), 1, 0, 0, 1, "right"},
		{at(7, 0), 1, 0, 7, 0, "right from the last cell stays"},
	}
	for _, c := range cases {
		got := cells[moveCell(cells, c.from, c.dx, c.dy)]
		if got.row != c.wantRow || got.col != c.wantCol {
			t.Errorf("%s: got row %d col %d, want %d %d", c.describe, got.row, got.col, c.wantRow, c.wantCol)
		}
	}
}

func TestFindCurrent(t *testing.T) {
	cells := withRecent(paletteCells(), []string{"#123456"})
	name := func(i int) string {
		if i < 0 {
			return "none found"
		}
		return cells[i].name
	}
	cases := []struct{ text, want string }{
		{"", ""},
		{"RED", "red"},
		{"red-200", "red-200"},
		{"#EF9A9A", "red-200"}, // the same colour as a named cell
		{"#123456", "#123456"},
		{"#010203", "none found"},
		{"$(x)", "none found"},
	}
	for _, c := range cases {
		if got := name(findCurrent(cells, c.text)); got != c.want {
			t.Errorf("findCurrent(%q) = %q, want %q", c.text, got, c.want)
		}
	}
}

func TestParseColorInput(t *testing.T) {
	cases := []struct {
		in, want string
		ok       bool
	}{
		{"#FF0000", "#ff0000", true},
		{"ff0000", "#ff0000", true},
		{" #AABBCC80 ", "#aabbcc80", true},
		{"Red", "red", true},
		{"red-200", "red-200", true},
		{"#fff", "", false},
		{"#gg0000", "", false},
		{"nonsense", "", false},
		{"", "", false},
		{"#", "", false},
	}
	for _, c := range cases {
		got, ok := parseColorInput(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("parseColorInput(%q) = %q, %v; want %q, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestPushColor(t *testing.T) {
	list := pushColor(nil, "a", 3)
	list = pushColor(list, "b", 3)
	list = pushColor(list, "a", 3)
	if !slices.Equal(list, []string{"a", "b"}) {
		t.Errorf("repeat moves to the front: %v", list)
	}
	list = pushColor(pushColor(list, "c", 3), "d", 3)
	if !slices.Equal(list, []string{"d", "c", "a"}) {
		t.Errorf("limit: %v", list)
	}
}
