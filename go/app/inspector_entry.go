//go:build darwin || linux || windows

package app

import (
	"strconv"
	"strings"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/swt"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/palette"
)

// entryCfg describes a text entry bound to an argument of a command.
type entryCfg struct {
	id      string
	name    string
	idx     int
	numeric bool // only numbers and expressions
	keep    bool // an empty text leaves the value, as for a name
	list    bool // the field holds every argument, split on blanks, as `padding` does
	src     func() propSrc
	hint    string // the hint when the value is not set and there is no effective one
	prefix  string // what to call the value, inside the field: X, Y, W, H
}

// entry is a Text bound to argument idx of command name.
type entry struct {
	in      *inspector
	cfg     entryCfg
	ctl     *g.TextView
	shown   string // what the field held when it was set
	focused bool
	expr    string // the source of the expression the field shows evaluated
}

func (in *inspector) newEntry(cfg entryCfg) *entry {
	e := &entry{in: in, cfg: cfg}
	if e.cfg.src == nil {
		e.cfg.src = in.src
	}
	return e
}

func (e *entry) option() g.Option {
	opts := []g.Option{g.ID(e.cfg.id), g.Ref(&e.ctl), g.GrowX(), g.Place(g.AlignFill, g.AlignCenter),
		g.Small(), g.Size(g.Scaled(48), g.Scaled(22)), g.Inset(g.Scaled(6), 0), g.Radius(g.Scaled(4)),
		g.FieldFill(g.Adaptive{Light: g.RGB{R: 0xFF, G: 0xFF, B: 0xFF}, Dark: g.RGB{R: 0x1C, G: 0x1C, B: 0x1E}}),
		g.FieldStroke(g.RGB{R: 0x80, G: 0x80, B: 0x80}.Alpha(0x40))}
	if e.cfg.prefix != "" {
		opts = append(opts, g.Prefix(e.cfg.prefix))
	}
	// Centred in its row: a swatch beside it is taller than the field.
	return g.Text(append(opts, g.Init(func(t *g.TextView) {
		t.OnActivate(e.commit)
		t.Unwrap().AddFocusListener(swt.FocusListenerFocusGainedAdapter(func(*swt.FocusEvent) { e.gained() }))
		t.Unwrap().AddFocusListener(swt.FocusListenerFocusLostAdapter(func(*swt.FocusEvent) { e.lost() }))
	}))...)
}

func (e *entry) gained() {
	e.focused = true
	e.in.v.activeText = e.ctl.Unwrap()
	e.in.v.updateMenus()
	if e.expr != "" { // edit the source, not its value
		e.set(e.expr)
	}
}

func (e *entry) lost() {
	e.focused = false
	if e.in.v.activeText == e.ctl.Unwrap() {
		e.in.v.activeText = nil
		e.in.v.updateMenus()
	}
	e.commit()
	e.refresh()
}

func (e *entry) set(text string) {
	e.in.loading = true
	e.shown = text
	e.ctl.SetText(text)
	e.in.loading = false
}

// commit writes the text when the user changed it.
func (e *entry) commit() {
	if e.in.loading || e.in.v.session == nil || e.ctl.Text() == e.shown {
		return
	}
	text := strings.TrimSpace(e.ctl.Text())
	switch {
	case text == "" && e.cfg.keep:
	case e.cfg.numeric && text != "" && !isNumberOrExpr(text):
	case e.cfg.list:
		e.cfg.src().set(e.cfg.name, strings.Fields(text)...)
	default:
		e.cfg.src().arg(e.cfg.name, e.cfg.idx, text)
	}
	e.shown = e.ctl.Text()
}

// listText joins the arguments of a command written as one field, as `padding`.
func listText(p editor.Prop) string {
	parts := make([]string, len(p.Args))
	for i, a := range p.Args {
		parts[i] = a.Text
	}
	return strings.Join(parts, " ")
}

func isNumberOrExpr(s string) bool {
	if _, err := strconv.ParseFloat(s, 64); err == nil {
		return true
	}
	return strings.HasPrefix(s, "${") || strings.HasPrefix(s, "$(")
}

func (e *entry) refresh() {
	if e.ctl == nil || e.focused {
		return
	}
	src := e.cfg.src()
	p := src.get(e.cfg.name)
	a := p.Arg(e.cfg.idx)
	e.expr = ""
	hint := e.cfg.hint
	text := ""
	tip := ""
	switch {
	case p.Mixed:
		hint = "Mixed"
	case p.Set && e.cfg.list:
		text = listText(p)
	case p.Set && a.Expr:
		text, e.expr, tip = a.Value, a.Text, a.Text
	case p.Set:
		text = a.Text
	default:
		if eff := src.eff(e.cfg.name); eff != "" && e.cfg.idx == 0 {
			hint = eff
		}
	}
	e.set(text)
	e.ctl.SetHint(hint)
	e.ctl.SetTooltip(tip)
	e.ctl.SetForeground(e.foreground())
}

// foreground marks a field that holds an expression in the colour of expressions in the code.
func (e *entry) foreground() g.Paint {
	if e.expr == "" {
		return nil
	}
	mode := "light"
	if e.in.v.app.Dark() {
		mode = "dark"
	}
	c, _ := parseHex(editor.Colors[mode][editor.HighlightExpression])
	return c
}

// text adds a row with one text entry.
func (in *inspector) text(caption string, when func(selKind) bool, cfg entryCfg) func() g.Option {
	e := in.newEntry(cfg)
	return in.row(caption, when, func(selKind) { e.refresh() }, e.option())
}

// pair adds a row with two entries prefixed a and b.
func (in *inspector) pair(caption, a, b string, when func(selKind) bool, c1, c2 entryCfg) func() g.Option {
	c1.prefix, c2.prefix = a, b
	e1, e2 := in.newEntry(c1), in.newEntry(c2)
	return in.row(caption, when, func(selKind) { e1.refresh(); e2.refresh() },
		e1.option(), e2.option())
}

// segmentTexts turns words into segments of a Segmented control.
func segmentTexts(words []string) []g.Segment {
	segs := make([]g.Segment, len(words))
	for i, w := range words {
		segs[i] = g.Segment{Text: w}
	}
	return segs
}

// itemSize is the drawn size of the selection: the scene node's, or the box the scene draws a classic
// item with.
func (v *Viewer) itemSize() (w, h float64) {
	if _, pl := v.selectedScene(); pl != nil {
		return pl.W, pl.H
	}
	if v.session == nil {
		return 0, 0
	}
	for _, it := range v.session.Selection() {
		if b, ok := v.drawnRect(it); ok {
			return b.W(), b.H()
		}
	}
	return 0, 0
}

// sizeModes adds Fixed / Hug / Fill for one side of a size: Fixed takes the size the item has now,
// the other two the word. A size in `.tenn` is text, so an expression keeps the field and the segments
// say nothing.
func (in *inspector) sizeModes(caption, id, name string, idx int, when func(selKind) bool, measured func() float64, src func() propSrc) func() g.Option {
	if src == nil {
		src = in.src
	}
	var c *g.SegmentedView
	refresh := func(selKind) {
		s := src()
		p := s.get(name)
		i := 0 // a size that is not written behaves as fixed
		switch {
		case p.Mixed:
			i = -1
		case !p.Set:
		case p.Arg(idx).Expr:
			i = -1
		case p.Arg(idx).Text == "hug":
			i = 1
		case p.Arg(idx).Text == "fill":
			i = 2
		}
		in.loading = true
		c.Select(i)
		in.loading = false
	}
	// Three segments do not wrap: an unqualified control would widen the whole panel and cut the fields
	// beside it, so it is held to the room a value has (AGENTS.md, the inspector).
	ctl := g.Segmented(segmentTexts([]string{"Fixed", "Hug", "Fill"}), g.Small(),
		g.Size(valueWidth(), 0), g.ID(id), g.Ref(&c),
		g.Init(func(c *g.SegmentedView) {
			c.OnSelect(func(i int) {
				if in.loading || in.v.session == nil {
					return
				}
				switch i {
				case 1:
					src().set(name, "hug")
				case 2:
					src().set(name, "fill")
				case 0:
					src().arg(name, idx, strconv.FormatFloat(measured(), 'f', 0, 64))
				}
			})
		}))
	return in.row(caption, when, refresh, ctl)
}

// colorOf parses the text of a colour; false for an expression or a name it does not know.
func colorOf(text string) (g.RGB, bool) {
	c, ok := palette.Parse(text)
	if !ok {
		return g.RGB{}, false
	}
	to8 := func(f float64) uint8 { return uint8(f*255 + 0.5) }
	return g.RGB{R: to8(c.R), G: to8(c.G), B: to8(c.B)}, true
}

// color adds a row with a swatch that opens the colour picker and a text with the name or hex.
func (in *inspector) color(caption string, when func(selKind) bool, cfg entryCfg) func() g.Option {
	ctl, refresh := in.colorCtl(cfg)
	return in.row(caption, when, refresh, ctl...)
}

// colorCtl is the swatch and the text of a colour, without the row: a row built again with the
// selection, as a shadow layer, puts them beside its other fields.
func (in *inspector) colorCtl(cfg entryCfg) ([]g.Option, func(selKind)) {
	e := in.newEntry(cfg)
	var sw *g.CanvasView
	var cur g.RGB
	var curText string // the value of the property, "" when it is not set
	has, mixed := false, false
	paint := func(gc *g.GC) {
		w, h := gc.Size()
		raw := gc.Unwrap()
		raw.SetAntialias(swt.ON)
		switch {
		case mixed:
			raw.SetBackground(raw.GetDevice().GetSystemColor(swt.COLOR_WIDGET_BACKGROUND))
		case has:
			raw.SetBackground(swt.NewColorRedGreenBlue(int32(cur.R), int32(cur.G), int32(cur.B)))
		}
		if mixed || has { // no colour: the glass shows through, with the slash only
			raw.FillRoundRectangle(1, 1, int32(w-2), int32(h-2), 8, 8)
		}
		raw.SetForeground(swt.NewColorRedGreenBlueAlpha(128, 128, 128, 160))
		raw.DrawRoundRectangle(1, 1, int32(w-3), int32(h-3), 8, 8)
		if mixed || !has {
			raw.DrawLine(5, int32(h-5), int32(w-5), 5)
		}
	}
	size := g.Scaled(24)
	swatch := g.Canvas(paint, g.ID(cfg.id+"-swatch"), g.Ref(&sw), g.Size(size, size), g.Place(g.AlignStart, g.AlignCenter),
		g.Init(func(c *g.CanvasView) {
			c.SetTooltip("Choose color")
			c.OnMouseDown(func(m g.Mouse) {
				if m.Button != 1 || in.v.session == nil {
					return
				}
				start := cur
				if !has {
					start = g.RGB{R: 255, G: 255, B: 255}
				}
				in.v.pickColor(sw, curText, !mixed, start, func(text string) {
					e.cfg.src().arg(cfg.name, cfg.idx, text)
				})
			})
		}))
	refresh := func(selKind) {
		src := e.cfg.src()
		p := src.get(cfg.name)
		mixed = p.Mixed
		text := ""
		if cfg.idx == 0 {
			text = src.eff(cfg.name)
		}
		switch a := p.Arg(cfg.idx); {
		case p.Set && a.Expr:
			text = a.Value
		case p.Set:
			text = a.Text
		}
		cur, has = colorOf(text)
		curText = ""
		if p.Set {
			curText = text
		}
		e.refresh()
		sw.Redraw()
	}
	return []g.Option{swatch, e.option()}, refresh
}

// textRow is in.text for a row built again with the selection: it returns the row and its refresh.
func (in *inspector) textRow(caption string, cfg entryCfg) (g.Option, func()) {
	e := in.newEntry(cfg)
	return styleRow(caption, e.option()), e.refresh
}

// pairRow is in.pair for a row built again with the selection.
func (in *inspector) pairRow(caption, a, b string, c1, c2 entryCfg) (g.Option, func()) {
	c1.prefix, c2.prefix = a, b
	e1, e2 := in.newEntry(c1), in.newEntry(c2)
	return styleRow(caption, e1.option(), e2.option()), func() { e1.refresh(); e2.refresh() }
}

// combo adds a row with a read-only choice of a property. Choosing unset removes the property.
func (in *inspector) combo(caption string, when func(selKind) bool, id, name string, options []string, unset string, src func() propSrc) func() g.Option {
	if src == nil {
		src = in.src
	}
	var c *g.ComboView
	refresh := func(selKind) {
		s := src()
		p := s.get(name)
		want := ""
		switch {
		case p.Mixed:
		case p.Set:
			want = p.Arg(0).Text
		default:
			if want = s.eff(name); want == "" {
				want = unset
			}
		}
		i := -1
		for j, o := range options {
			if o == want {
				i = j
			}
		}
		in.loading = true
		c.Select(i)
		in.loading = false
	}
	ctl := g.Combo(options, g.ReadOnly(), g.ID(id), g.Ref(&c), g.GrowX(), g.Size(g.Scaled(60), 0), g.Init(func(c *g.ComboView) {
		c.OnSelect(func(i int) {
			if in.loading || i < 0 || i >= len(options) || in.v.session == nil {
				return
			}
			if options[i] == unset {
				src().set(name)
			} else {
				src().set(name, options[i])
			}
		})
	}))
	return in.row(caption, when, refresh, ctl)
}

var (
	horizontalLayouts = []string{"left", "center", "right"}
	verticalLayouts   = []string{"top", "middle", "bottom", "fill"}
)

// noneChoice is the choice for a property the item does not set: the inspector shows that, not the value
// the renderer would use, so an unset property is not mistaken for a written one.
const noneChoice = "none"

// layoutCombo chooses the horizontal or the vertical word of `layout`, keeping the other word. The first
// choice is none: it leaves the axis unset rather than writing the value the renderer defaults to.
func (in *inspector) layoutCombo(caption string, when func(selKind) bool, id string, mine, other []string) func() g.Option {
	options := append([]string{noneChoice}, mine...)
	var c *g.ComboView
	words := func() (m, o string) {
		for _, a := range in.src().get("layout").Args {
			for _, w := range strings.Split(strings.ReplaceAll(a.Text, ",", " "), " ") {
				switch {
				case contains(mine, w):
					m = w
				case contains(other, w):
					o = w
				}
			}
		}
		return
	}
	refresh := func(selKind) {
		p := in.src().get("layout")
		m, _ := words()
		i := 0
		if p.Mixed {
			i = -1 // the items disagree: no one word to show
		}
		if !p.Mixed && m != "" {
			for j, o := range mine {
				if o == m {
					i = j + 1
				}
			}
		}
		in.loading = true
		c.Select(i)
		in.loading = false
	}
	ctl := g.Combo(options, g.ReadOnly(), g.ID(id), g.Ref(&c), g.GrowX(), g.Size(g.Scaled(60), 0), g.Init(func(c *g.ComboView) {
		c.OnSelect(func(i int) {
			if in.loading || i < 0 || i >= len(options) || in.v.session == nil {
				return
			}
			_, o := words()
			args := []string{}
			if i > 0 {
				args = append(args, mine[i-1])
			}
			if o != "" {
				args = append(args, o)
			}
			if len(args) == 0 {
				in.src().set("layout")
				return
			}
			in.src().set("layout", args...)
		})
	}))
	return in.row(caption, when, refresh, ctl)
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// toggle adds a row with a switch: on gives the command the args, off removes it.
func (in *inspector) toggle(caption string, when func(selKind) bool, id, name string, on []string, src func() propSrc) func() g.Option {
	if src == nil {
		src = in.src
	}
	var sw *g.SwitchView
	refresh := func(selKind) {
		in.loading = true
		sw.SetChecked(src().get(name).Set)
		in.loading = false
	}
	ctl := g.Switch("", g.ID(id), g.Ref(&sw), g.Init(func(s *g.SwitchView) {
		s.OnChange(func(checked bool) {
			if in.loading || in.v.session == nil {
				return
			}
			if checked {
				src().set(name, on...)
			} else {
				src().set(name)
			}
		})
	}))
	return in.row(caption, when, refresh, ctl)
}

// info adds a read-only row. The label asks for the width hint, not for the length of its text: it wraps at the
// width the panel gives it and grows downwards, so a long list of styles does not widen the inspector.
func (in *inspector) info(caption string, when func(selKind) bool, text func(selKind) string) func() g.Option {
	var l *g.LabelView
	return in.row(caption, when, func(k selKind) { l.SetText(text(k)) },
		g.Label("", g.Ref(&l), g.Wrap(), g.GrowX(), g.Size(g.Scaled(40), 0)))
}
