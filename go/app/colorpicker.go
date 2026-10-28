//go:build darwin || linux || windows

package app

import (
	"fmt"
	"slices"
	"strings"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/swt"

	"github.com/haiodo/tenniarb/go/markdown"
)

// The grid: row 0 holds None and the basic names, rows 1...6 the shades of the hue columns, the
// last row the recent colours after a gap.
const (
	pickSize      = 14 // a cell, in points
	pickGap       = 2
	pickPitch     = pickSize + pickGap
	pickPad       = 2 // room for the ring of a cell at the edge
	pickShadeRows = 6
	pickRecentRow = pickShadeRows + 1
	pickSep       = 8 // extra space above the recent row
	pickRecentMax = 8
)

// pickHues are the columns: the hue families of the format's palette (gray first, then the rainbow).
var pickHues = []string{
	"gray", "red", "pink", "purple", "deep-purple", "indigo", "blue", "light-blue", "cyan", "teal",
	"green", "light-green", "lime", "yellow", "amber", "orange", "deep-orange", "brown", "blue-grey",
}

// pickShades are the rows from light to dark.
var pickShades = [pickShadeRows]string{"100", "200", "300", "500", "700", "900"}

// pickBasics are the names after None in the first row; the Swift colour menu had the coloured ones.
var pickBasics = []string{"black", "white", "gray", "red", "orange", "yellow", "green", "blue", "purple", "brown"}

// pickCell is one cell of the grid; an empty name is None.
type pickCell struct {
	name     string
	col, row int
}

// paletteCells lists the fixed cells row by row, None first.
func paletteCells() []pickCell {
	cells := []pickCell{{"", 0, 0}}
	for i, n := range pickBasics {
		cells = append(cells, pickCell{n, i + 1, 0})
	}
	for r, shade := range pickShades {
		for c, hue := range pickHues {
			cells = append(cells, pickCell{hue + "-" + shade, c, r + 1})
		}
	}
	return cells
}

// withRecent appends the recent colours as the last row.
func withRecent(cells []pickCell, recent []string) []pickCell {
	cells = slices.Clone(cells)
	for i, n := range recent {
		cells = append(cells, pickCell{n, i, pickRecentRow})
	}
	return cells
}

// pushColor puts name first, drops its earlier copy and keeps at most max.
func pushColor(list []string, name string, max int) []string {
	out := []string{name}
	for _, n := range list {
		if n != name {
			out = append(out, n)
		}
	}
	return out[:min(len(out), max)]
}

func gridSize(recent bool) (w, h int) {
	rows := pickRecentRow
	if recent {
		rows++
	}
	h = rows*pickPitch - pickGap + 2*pickPad
	if recent {
		h += pickSep
	}
	return len(pickHues)*pickPitch - pickGap + 2*pickPad, h
}

// cellOrigin is the top left of the cell in the canvas.
func cellOrigin(c pickCell) (x, y int) {
	y = pickPad + c.row*pickPitch
	if c.row >= pickRecentRow {
		y += pickSep
	}
	return pickPad + c.col*pickPitch, y
}

// hitCell is the index of the cell under the canvas point, -1 for none. A gap counts to the cell before it.
func hitCell(cells []pickCell, x, y int) int {
	x, y = x-pickPad, y-pickPad
	if x < 0 || y < 0 {
		return -1
	}
	row := y / pickPitch
	if row >= pickRecentRow {
		y -= pickSep
		if y < pickRecentRow*pickPitch {
			return -1
		}
		row = y / pickPitch
	}
	col := x / pickPitch
	return slices.IndexFunc(cells, func(c pickCell) bool { return c.row == row && c.col == col })
}

// moveCell is the cell reached from i by an arrow key: left and right step through the list, up and
// down go to the nearest column of the neighbour row. It stays when there is nowhere to go.
func moveCell(cells []pickCell, i, dx, dy int) int {
	if dx != 0 {
		return max(0, min(len(cells)-1, i+dx))
	}
	row, best, gap := cells[i].row+dy, i, -1
	for j, c := range cells {
		d := c.col - cells[i].col
		if d < 0 {
			d = -d
		}
		if c.row == row && (gap < 0 || d < gap) {
			best, gap = j, d
		}
	}
	return best
}

// findCurrent is the cell that holds the colour text: the same name, else the same colour. "" is None.
func findCurrent(cells []pickCell, text string) int {
	if text == "" {
		return 0
	}
	low := strings.ToLower(text)
	if i := slices.IndexFunc(cells, func(c pickCell) bool { return c.name == low }); i >= 0 {
		return i
	}
	want, ok := colorOf(text)
	if !ok {
		return -1
	}
	return slices.IndexFunc(cells, func(c pickCell) bool {
		got, ok := colorOf(c.name)
		return c.name != "" && ok && got == want
	})
}

// parseColorInput reads what was typed in the hex field: a colour name, or 6 or 8 hex digits with an
// optional #. The result is the text to store.
func parseColorInput(s string) (string, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if _, ok := markdown.ColorNames[s]; ok && s != "" {
		return s, true
	}
	digits := strings.TrimPrefix(s, "#")
	if len(digits) != 6 && len(digits) != 8 {
		return "", false
	}
	for _, c := range digits {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return "", false
		}
	}
	return "#" + digits, true
}

// onColor is black or white, whichever reads on the fill.
func onColor(c g.RGB) g.RGB {
	if (int(c.R)*299+int(c.G)*587+int(c.B)*114)/1000 > 128 {
		return g.RGB{}
	}
	return g.RGB{R: 255, G: 255, B: 255}
}

// colorPicker is the popover with the colour grid over a swatch.
type colorPicker struct {
	v      *Viewer
	p      *g.Popover
	cv     *g.CanvasView
	cells  []pickCell
	rgb    []g.RGB
	cur    int // the cell with the current value, -1 when unknown
	cursor int // the cell Enter applies
	done   func(text string)
}

// pickColor opens the picker under the swatch. known says cur is the property value ("" for not set).
// done gets "" for None, else the colour name or hex.
func (v *Viewer) pickColor(anchor *g.CanvasView, cur string, known bool, start g.RGB, done func(text string)) {
	v.closePopover()
	cells := withRecent(paletteCells(), v.recentColors)
	pk := &colorPicker{v: v, cells: cells, done: done, cur: -1}
	if known {
		pk.cur = findCurrent(cells, cur)
	}
	pk.cursor = max(pk.cur, 0)
	for _, c := range cells {
		rgb, _ := colorOf(c.name)
		pk.rgb = append(pk.rgb, rgb)
	}
	w, h := gridSize(len(v.recentColors) > 0)
	pk.p = v.win.Popover(g.Gap(6), g.Padding(8))
	pk.p.Add(g.Canvas(pk.paint, g.Ref(&pk.cv), g.Size(w, h), g.Init(func(c *g.CanvasView) {
		c.OnMouseMove(pk.hover)
		c.OnMouseDown(func(m g.Mouse) {
			if i := hitCell(pk.cells, m.X, m.Y); m.Button == 1 && i >= 0 {
				pk.apply(pk.cells[i].name)
			}
		})
	})))
	pk.p.Add(g.HBox(g.Gap(6), g.Padding(0),
		g.Text(g.Focused(), g.GrowX(), g.Hint("#rrggbb or name"), g.Init(pk.initHex)),
		g.Button("More...", g.Init(func(b *g.ButtonView) {
			b.OnClick(func() { pk.more(start) })
		}))))
	v.openPopover(pk.p)
	pk.p.ShowAt(anchor, g.PopoverBelow)
}

// apply stores the choice as a recent colour, closes the popover and hands the text over.
func (pk *colorPicker) apply(text string) {
	if text != "" {
		pk.v.recentColors = pushColor(pk.v.recentColors, text, pickRecentMax)
	}
	pk.p.Close()
	pk.done(text)
}

// more leaves for the system dialog; it opens after the popover has gone, as a menu does in quickMenu.
func (pk *colorPicker) more(start g.RGB) {
	pk.p.Close()
	pk.v.app.Async(func() {
		if rgb, ok := pk.v.win.ColorDialog(start); ok {
			pk.v.recentColors = pushColor(pk.v.recentColors, fmt.Sprintf("#%02x%02x%02x", rgb.R, rgb.G, rgb.B), pickRecentMax)
			pk.done(pk.v.recentColors[0])
		}
	})
}

// initHex wires the hex field; it keeps the focus, so arrows and Enter go through it.
func (pk *colorPicker) initHex(t *g.TextView) {
	t.OnChange(func(s string) {
		var c g.Paint
		if _, ok := parseColorInput(s); strings.TrimSpace(s) != "" && !ok {
			c = g.RGB{R: 220, G: 50, B: 47}
		}
		t.SetForeground(c)
	})
	t.OnActivate(func() {
		s := strings.TrimSpace(t.Text())
		if s == "" {
			pk.apply(pk.cells[pk.cursor].name)
		} else if name, ok := parseColorInput(s); ok {
			pk.apply(name)
		}
	})
	t.Unwrap().AddKeyListener(swt.KeyListenerKeyPressedAdapter(func(e *swt.KeyEvent) {
		dx, dy := 0, 0
		switch e.KeyCode {
		case int32(swt.ARROW_UP):
			dy = -1
		case int32(swt.ARROW_DOWN):
			dy = 1
		case int32(swt.ARROW_LEFT):
			dx = -1
		case int32(swt.ARROW_RIGHT):
			dx = 1
		default:
			return
		}
		if dx != 0 && t.Text() != "" { // the caret moves in a typed text
			return
		}
		e.Doit = false
		pk.setCursor(moveCell(pk.cells, pk.cursor, dx, dy))
	}))
}

func (pk *colorPicker) hover(m g.Mouse) {
	i := hitCell(pk.cells, m.X, m.Y)
	if i < 0 {
		pk.cv.SetTooltip("")
		return
	}
	pk.cv.SetTooltip(pk.label(i))
	pk.setCursor(i)
}

func (pk *colorPicker) label(i int) string {
	if n := pk.cells[i].name; n != "" {
		return n
	}
	return "None"
}

func (pk *colorPicker) setCursor(i int) {
	if i != pk.cursor {
		pk.cursor = i
		pk.cv.Redraw()
	}
}

func (pk *colorPicker) paint(gc *g.GC) {
	dark := pk.v.app.Dark()
	hair, ring := g.RGB{R: 150, G: 150, B: 150}, g.RGB{}
	if dark {
		hair, ring = g.RGB{R: 100, G: 100, B: 100}, g.RGB{R: 255, G: 255, B: 255}
	}
	for i, c := range pk.cells {
		x, y := cellOrigin(c)
		dot := ring
		if c.name == "" { // the empty swatch of the inspector: hairline and a slash
			gc.SetColor(hair)
			gc.Rect(x, y, pickSize-1, pickSize-1)
			gc.Line(x+2, y+pickSize-3, x+pickSize-3, y+2)
		} else {
			gc.SetFill(pk.rgb[i])
			gc.FillRect(x, y, pickSize, pickSize)
			gc.SetColor(hair)
			gc.Rect(x, y, pickSize-1, pickSize-1)
			dot = onColor(pk.rgb[i])
		}
		if i == pk.cur {
			gc.SetFill(dot)
			gc.FillRect(x+pickSize/2-2, y+pickSize/2-2, 4, 4)
		}
	}
	x, y := cellOrigin(pk.cells[pk.cursor])
	gc.SetLineWidth(2)
	gc.SetColor(ring)
	gc.Rect(x-1, y-1, pickSize+1, pickSize+1)
	gc.SetLineWidth(1)
}
