//go:build darwin || linux || windows

package app

import (
	"fmt"
	"strconv"
	"strings"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/look"
	"github.com/haiodo/gowt/swt"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/palette"
	"github.com/haiodo/tenniarb/go/scene"
)

// selKind says what the inspector shows: the element, or the selection by what it holds.
type selKind int

const (
	kindNone  selKind = iota // nothing selected: the element
	kindItem                 // one item
	kindLink                 // one link
	kindItems                // several items
	kindLinks                // several links
	kindMixed                // items and links
	kindScene                // a scene element is on the canvas: the scene sections
)

func (k selKind) item() bool  { return k == kindItem || k == kindItems }
func (k selKind) link() bool  { return k == kindLink || k == kindLinks }
func (k selKind) multi() bool { return k == kindItems || k == kindLinks || k == kindMixed }
func (k selKind) sel() bool   { return k != kindNone && k != kindScene }

func kindOf(sel []*model.DiagramItem) selKind {
	items, links := 0, 0
	for _, it := range sel {
		if it.Kind == model.ItemKindLink {
			links++
		} else {
			items++
		}
	}
	switch {
	case items == 0 && links == 0:
		return kindNone
	case links == 0:
		if items == 1 {
			return kindItem
		}
		return kindItems
	case items == 0:
		if links == 1 {
			return kindLink
		}
		return kindLinks
	}
	return kindMixed
}

// propSrc is where a field reads and writes: the selection, or a style group of the element.
type propSrc struct {
	get func(name string) editor.Prop
	set func(name string, args ...string) bool
	arg func(name string, i int, text string) bool
	eff func(name string) string
}

func (v *Viewer) selSrc() propSrc {
	return propSrc{
		get: func(n string) editor.Prop { return v.session.Prop(n) },
		set: func(n string, a ...string) bool { return v.session.SetProp(n, a...) },
		arg: func(n string, i int, t string) bool { return v.session.SetPropArg(n, i, t) },
		eff: func(n string) string { return v.session.Effective(n) },
	}
}

// styleSrc edits `styles { group { ... } }` of the element: the defaults of its items and links.
func (v *Viewer) styleSrc(group string) propSrc {
	src := propSrc{
		get: func(n string) editor.Prop { return v.session.StyleProp(group, n) },
		set: func(n string, a ...string) bool { return v.session.SetStyleProp(group, n, a...) },
		eff: func(string) string { return "" },
	}
	src.arg = func(n string, i int, t string) bool {
		var args []string
		for _, a := range src.get(n).Args {
			args = append(args, a.Text)
		}
		if t == "" {
			args = args[:min(i, len(args))]
		} else {
			for len(args) <= i {
				args = append(args, "0")
			}
			args[i] = t
		}
		return v.session.SetStyleProp(group, n, args...)
	}
	return src
}

// inspField is one row of the inspector.
type inspField struct {
	row     *g.Panel
	when    func(k selKind) bool
	refresh func(k selKind)
}

type inspSection struct {
	scene  bool // shown for a scene element instead of the classic sections
	box    *g.Panel
	d      *g.DisclosureView
	shown  string // the title the disclosure holds
	label  func(k selKind) string
	when   func(k selKind) bool
	fields []*inspField
}

// inspector is the right column: the properties of the selection as compact sections.
type inspector struct {
	v        *Viewer
	panel    *g.Panel
	scroll   *g.ScrollView
	sections []*inspSection
	cur      *inspSection
	building bool // the sections being built are scene sections
	kind     selKind
	loading  bool
	pending  bool
}

func (in *inspector) src() propSrc { return in.v.selSrc() }

func faint() g.Option { return g.Foreground(g.SecondaryText) }

// labelWidth is the width of the caption column of a row.
func labelWidth() int { return g.Scaled(68) }

// inspectorWidth is the width of the inspector column: fixed, and wide enough for its widest row - the
// Fixed/Hug/Fill segments and a pair of fields - so nothing is cut at the panel edge.
func inspectorWidth() int { return g.Scaled(272) }

// valueWidth is the room a control of an inspector row gets: the panel minus the caption column and the
// gaps around it. A control that cannot wrap is cut at it rather than widening the whole panel.
func valueWidth() int { return inspectorWidth() - labelWidth() - g.Scaled(24) }

// styleGroupText is one group declaration as the inspector lists it: "kind (one-of primary ghost)".
func styleGroupText(g scene.StyleGroup) string {
	mode := "one-of"
	if g.AnyOf {
		mode = "any-of"
	}
	return g.Name + " (" + mode + " " + strings.Join(g.Styles, " ") + ")"
}

// shortPath is the tail of a dotted path, the last two segments: what a narrow field can show.
func shortPath(p string) string {
	parts := strings.Split(p, ".")
	if len(parts) <= 2 {
		return p
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

// section starts a collapsible section; the fields built until the next call belong to it. The title
// follows the selection where it names what is shown.
func (in *inspector) section(label func(selKind) string, when func(selKind) bool, rows ...func() g.Option) g.Option {
	s := &inspSection{label: label, when: when, scene: in.building}
	in.sections = append(in.sections, s)
	in.cur = s
	body := []g.Option{g.GrowX(), g.Padding(0), g.Gap(g.Scaled(6))}
	for _, r := range rows {
		body = append(body, r())
	}
	return g.VBox(g.Ref(&s.box), g.GrowX(), g.Padding(0),
		g.Disclosure(label(kindNone), append([]g.Option{g.Ref(&s.d), g.GrowX(), g.Gap(g.Scaled(6))}, body...)...))
}

func sectionFont() g.Font {
	f := g.Type.Caption
	f.Weight = g.Semibold
	return f
}

func sectionTitle(s string) func(selKind) string { return func(selKind) string { return s } }

func always(selKind) bool { return true }

// row is a caption on the left and the controls on the right.
func (in *inspector) row(caption string, when func(selKind) bool, refresh func(selKind), ctl ...g.Option) func() g.Option {
	return func() g.Option {
		f := &inspField{when: when, refresh: refresh}
		in.cur.fields = append(in.cur.fields, f)
		opts := []g.Option{g.Ref(&f.row), g.GrowX(), g.Padding(0), g.Gap(g.Scaled(8)),
			g.Label(caption, faint(), g.Size(labelWidth(), 0), g.Place(g.AlignStart, g.AlignCenter))}
		return g.HBox(append(opts, ctl...)...)
	}
}

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
		if d, ok := v.session.Scene().Drawables[it]; ok {
			b := d.Bounds()
			return b.Width, b.Height
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

func bodyKind(v *Viewer) string {
	p := v.session.Prop("body")
	switch {
	case p.Mixed:
		return "Mixed"
	case !p.Set:
		return "None"
	}
	text := p.Arg(0).Text
	if strings.Contains(text, "image") {
		return "Image"
	}
	return "Markdown"
}

// nodeCfg is one kind of node in the sections both kinds show: where a row reads and writes, which
// rows apply, the id prefix its controls answer to, and whether its fields take words.
type nodeCfg struct {
	prefix  string
	src     func() propSrc
	words   bool // a size or a radius may be `hug`, `fill` or four corners, as a scene node's is
	layout  func(selKind) bool
	fill    func(selKind) bool
	text    func(selKind) bool
	border  func(selKind) bool
	shape   func(selKind) bool
	display func(selKind) bool
	layer   func(selKind) bool
}

func (c nodeCfg) id(row string) string { return c.prefix + "-" + row }

// entry is a field of the row; src nil follows the selection.
func (c nodeCfg) entry(row, name string) entryCfg {
	return entryCfg{id: c.id(row), name: name, src: c.src}
}

// num is a field that takes a number or an expression only.
func (c nodeCfg) num(row, name string, idx int) entryCfg {
	return entryCfg{id: c.id(row), name: name, idx: idx, numeric: true, src: c.src}
}

// size is a size or a corner radius: numeric for a classic item, words for a scene node.
func (c nodeCfg) size(row, name string) entryCfg {
	if c.words {
		return c.entry(row, name)
	}
	return c.num(row, name, 0)
}

// nodeRow is one row of a section both kinds share: the caption and the row itself.
type nodeRow struct {
	caption string
	build   func(in *inspector) func() g.Option
}

// layoutRows are the rows of Layout both a classic item and a scene node show.
func layoutRows(c nodeCfg) []nodeRow {
	return []nodeRow{
		{"Position", func(in *inspector) func() g.Option {
			x, y := c.num("pos-x", "pos", 0), c.num("pos-y", "pos", 1)
			x.keep, y.keep = true, true
			return in.pair("Position", "X", "Y", c.layout, x, y)
		}},
		{"Size", func(in *inspector) func() g.Option {
			return in.pair("Size", "W", "H", c.layout, c.size("width", "width"), c.size("height", "height"))
		}},
		{"W", func(in *inspector) func() g.Option {
			return in.sizeModes("W", c.id("width-mode"), "width", 0, c.layout, func() float64 { w, _ := in.v.itemSize(); return w }, c.src)
		}},
		{"H", func(in *inspector) func() g.Option {
			return in.sizeModes("H", c.id("height-mode"), "height", 0, c.layout, func() float64 { _, h := in.v.itemSize(); return h }, c.src)
		}},
		{"Padding", func(in *inspector) func() g.Option {
			cfg := c.entry("padding", "padding")
			cfg.list = true
			return in.text("Padding", c.layout, cfg)
		}},
	}
}

// fillRows are the rows of Fill both kinds show.
func fillRows(c nodeCfg) []nodeRow {
	return []nodeRow{
		{"Color", func(in *inspector) func() g.Option {
			return in.color("Color", c.fill, c.entry("color", "color"))
		}},
	}
}

// textRows are the rows of Text both kinds show; the alignment, the marker and the face are the
// kind's own.
func textRows(c nodeCfg) []nodeRow {
	return []nodeRow{
		{"Color", func(in *inspector) func() g.Option {
			return in.color("Color", c.text, c.entry("text-color", "text-color"))
		}},
		{"Size", func(in *inspector) func() g.Option {
			return in.text("Size", c.text, c.num("font-size", "font-size", 0))
		}},
	}
}

// borderRows are the rows of Border both kinds show; both engines read line-style, line-width and
// corner-radius of an item.
func borderRows(c nodeCfg) []nodeRow {
	return []nodeRow{
		{"Color", func(in *inspector) func() g.Option {
			return in.color("Color", c.border, c.entry("border-color", "border-color"))
		}},
		{"Width", func(in *inspector) func() g.Option {
			return in.text("Width", c.border, c.num("line-width", "line-width", 0))
		}},
		{"Style", func(in *inspector) func() g.Option {
			return in.combo("Style", c.border, c.id("line-style"), "line-style", []string{noneChoice, "solid", "dashed", "dotted"}, noneChoice, c.src)
		}},
		{"Radius", func(in *inspector) func() g.Option {
			return in.text("Radius", c.border, c.size("radius", "corner-radius"))
		}},
	}
}

// shapeRows are the rows of Shape both kinds show; `display` and `layer` are read by both engines.
func shapeRows(c nodeCfg) []nodeRow {
	return []nodeRow{
		{"Display", func(in *inspector) func() g.Option {
			return in.combo("Display", c.display, c.id("display"), "display", editor.ItemDisplay, "rect", c.src)
		}},
		{"Layer", func(in *inspector) func() g.Option {
			return in.combo("Layer", c.layer, c.id("layer"), "layer", []string{"default", "background", "hover"}, "default", c.src)
		}},
	}
}

// sharedSection is a section both kinds show: the shared rows with the rows of one kind around them.
func (in *inspector) sharedSection(title string, when func(selKind) bool, head []func() g.Option, rows []nodeRow, tail []func() g.Option) g.Option {
	all := append([]func() g.Option{}, head...)
	for _, r := range rows {
		all = append(all, r.build(in))
	}
	all = append(all, tail...)
	return in.section(sectionTitle(title), when, all...)
}

func (in *inspector) layoutSection(c nodeCfg, head, tail []func() g.Option) g.Option {
	return in.sharedSection("Layout", c.layout, head, layoutRows(c), tail)
}

func (in *inspector) fillSection(c nodeCfg, head, tail []func() g.Option) g.Option {
	return in.sharedSection("Fill", c.fill, head, fillRows(c), tail)
}

func (in *inspector) textSection(c nodeCfg, head, tail []func() g.Option) g.Option {
	return in.sharedSection("Text", c.text, head, textRows(c), tail)
}

func (in *inspector) borderSection(c nodeCfg, head, tail []func() g.Option) g.Option {
	return in.sharedSection("Border", c.border, head, borderRows(c), tail)
}

func (in *inspector) shapeSection(c nodeCfg, head, tail []func() g.Option) g.Option {
	return in.sharedSection("Shape", c.shape, head, shapeRows(c), tail)
}

// build lays out the inspector column. The panel it fills is the inspector of the window.
func (in *inspector) build() g.Option {
	v := in.v
	one := func(k selKind) bool { return k == kindItem || k == kindLink }
	items := func(k selKind) bool { return k.item() }
	links := func(k selKind) bool { return k.link() }
	selected := func(k selKind) bool { return k.sel() }
	notLink := func(k selKind) bool { return k.item() || k == kindMixed }
	none := func(k selKind) bool { return k == kindNone }
	pad := g.Spacing.MD

	itemStyles := func() propSrc { return v.styleSrc("item") }
	lineStyles := func() propSrc { return v.styleSrc("line") }
	n := func(id, name string) entryCfg { return entryCfg{id: id, name: name, numeric: true} }
	classic := nodeCfg{
		prefix:  "insp",
		layout:  items,
		fill:    notLink,
		text:    selected,
		border:  items,
		shape:   selected,
		display: items,
		layer:   selected,
	}

	title := func(k selKind) string {
		switch k {
		case kindItem:
			return "Item"
		case kindLink:
			return "Link"
		case kindNone:
			return "Element"
		}
		return fmt.Sprintf("%d objects", len(v.session.Selection()))
	}

	body := g.VBox(g.GrowX(), g.Padding(pad), g.Gap(g.Scaled(10)),
		in.section(title, always,
			in.text("Name", func(k selKind) bool { return k == kindItem || k == kindNone }, entryCfg{id: "insp-name", name: "name", keep: true}),
			// The text drawn instead of the name; empty removes it.
			in.text("Title", func(k selKind) bool { return k == kindItem }, entryCfg{id: "insp-title", name: "title"}),
			in.text("Label", func(k selKind) bool { return k == kindLink }, entryCfg{id: "insp-label", name: "label"}),
			in.info("Kind", one, func(k selKind) string {
				if k == kindLink {
					return "Link"
				}
				return "Item"
			}),
			in.info("Body", func(k selKind) bool { return k == kindItem }, func(selKind) string { return bodyKind(v) }),
			in.pair("Grid", "X", "Y", none,
				entryCfg{id: "insp-grid-x", name: "grid", numeric: true, keep: true},
				entryCfg{id: "insp-grid-y", name: "grid", idx: 1, numeric: true, keep: true})),
		in.section(func(k selKind) string {
			if k == kindNone {
				return "Description"
			}
			return "Body"
		}, func(k selKind) bool { return k == kindItem || k == kindNone }, in.bodyRow()),
		in.layoutSection(classic, nil, nil),
		in.fillSection(classic, nil, nil),
		in.textSection(classic, nil, []func() g.Option{
			in.layoutCombo("Align", notLink, "insp-align", horizontalLayouts, verticalLayouts),
			in.layoutCombo("Vertical", notLink, "insp-valign", verticalLayouts, horizontalLayouts),
			in.text("Marker", items, entryCfg{id: "insp-marker", name: "marker"}),
		}),
		in.borderSection(classic, nil, nil),
		in.shapeSection(classic, nil, nil),
		in.section(sectionTitle("Line"), links,
			in.color("Color", links, entryCfg{id: "insp-line-color", name: "color"}),
			in.text("Width", links, n("insp-link-width", "line-width")),
			in.combo("Style", links, "insp-link-style", "line-style", []string{"solid", "dashed", "dotted"}, "solid", nil),
			in.combo("Arrows", links, "insp-arrows", "display", []string{"solid", "arrow", "arrows", "arrow-source"}, "solid", nil)),
		in.section(sectionTitle("Effects"), notLink,
			in.shadowList(notLink, "insp-shadow", v.shownShadows, v.shadowsInherited, v.setShadows)),
		in.section(sectionTitle("Item defaults"), none,
			in.color("Fill", none, entryCfg{id: "insp-def-color", name: "color", src: itemStyles}),
			in.color("Border", none, entryCfg{id: "insp-def-border", name: "border-color", src: itemStyles}),
			in.color("Text", none, entryCfg{id: "insp-def-text", name: "text-color", src: itemStyles}),
			in.text("Font size", none, entryCfg{id: "insp-def-font", name: "font-size", numeric: true, src: itemStyles}),
			in.text("Padding", none, entryCfg{id: "insp-def-padding", name: "padding", list: true, src: itemStyles}),
			in.combo("Display", none, "insp-def-display", "display", editor.ItemDisplay, "rect", itemStyles),
			in.shadowList(none, "insp-def-shadow", v.itemDefaultShadows, nil, v.setItemDefaultShadows)),
		in.section(sectionTitle("Link defaults"), none,
			in.color("Color", none, entryCfg{id: "insp-def-line-color", name: "color", src: lineStyles}),
			in.text("Width", none, entryCfg{id: "insp-def-line-width", name: "line-width", numeric: true, src: lineStyles}),
			in.combo("Style", none, "insp-def-line-style", "line-style", []string{"solid", "dashed", "dotted"}, "solid", lineStyles),
			in.combo("Arrows", none, "insp-def-arrows", "display", []string{"solid", "arrow", "arrows", "arrow-source"}, "solid", lineStyles)),
		in.section(sectionTitle("Styles"), none,
			in.info("Named", none, func(selKind) string {
				if names := v.session.StyleNames(); len(names) > 0 {
					return strings.Join(names, ", ")
				}
				return "None"
			}),
			// The same declarations the scene offers as a choice: the classic inspector only lists them.
			in.info("Groups", none, func(selKind) string {
				groups := v.session.StyleGroups()
				if len(groups) == 0 {
					return "None"
				}
				out := make([]string, len(groups))
				for i, gr := range groups {
					out[i] = styleGroupText(gr)
				}
				return strings.Join(out, ", ")
			})),
		in.sceneSections())
	return g.Scroll(g.Grow(), g.Padding(0), g.Gap(0), g.NoHScroll(), g.Ref(&in.scroll), body)
}

func (in *inspector) relayout() {
	if in.scroll == nil || in.scroll.Unwrap().IsDisposed() {
		return
	}
	in.scroll.Content().Unwrap().LayoutOverload1(true)
	in.scroll.Fit()
	// Fit asks the content how wide it wants to be, and the pane takes that as its own width - about 390
	// points for the item inspector - so the content is held to the panel instead.
	if c := in.scroll.Unwrap(); c != nil && !c.IsDisposed() {
		c.SetMinSizeWidthHeight(int32(inspectorWidth()-2*g.Scaled(8)), 0)
	}
	look.SetInspectorWidth(in.v.win, inspectorWidth())
}

// schedule refreshes the inspector once, after the events that are queued: a drag asks for it at
// every step.
func (in *inspector) schedule() {
	if in.pending || in.scroll == nil {
		return
	}
	in.pending = true
	in.v.app.Async(func() {
		in.pending = false
		in.refresh()
	})
}

func (in *inspector) refresh() {
	v := in.v
	if in.scroll == nil || in.scroll.Unwrap().IsDisposed() || v.win.Unwrap().IsDisposed() {
		return
	}
	k := kindNone
	if v.session != nil {
		k = kindOf(v.session.Selection())
	}
	if v.sceneMode() {
		k = kindScene
	}
	changed := k != in.kind
	in.kind = k
	for _, s := range in.sections {
		// The sections of a rebuilt panel are disposed while the list still holds them, and the refresh
		// runs after the event that rebuilt it.
		if s.box == nil || s.box.Unwrap().IsDisposed() {
			continue
		}
		show := v.session != nil && s.scene == (k == kindScene) && s.when(k)
		if s.box.Visible() != show {
			s.box.SetVisible(show)
			changed = true
		}
		if !show {
			continue
		}
		if s.d != nil && !s.d.Unwrap().IsDisposed() {
			if t := s.label(k); t != s.shown {
				s.shown = t
				s.d.SetTitle(t)
			}
		}
		for _, f := range s.fields {
			if f.row == nil || f.row.Unwrap().IsDisposed() {
				continue
			}
			vis := f.when(k)
			if f.row.Visible() != vis {
				f.row.SetVisible(vis)
				changed = true
			}
			if vis {
				f.refresh(k)
			}
		}
	}
	if changed {
		in.relayout()
	}
}

// inspectorPane is the right column.
func (v *Viewer) inspectorPane() g.Option {
	v.insp = &inspector{v: v}
	return g.VBox(g.Ref(&v.insp.panel), g.Padding(0), g.Gap(0), v.insp.build())
}

// bodyText is the multi-line text of the body of the selected item, or of the description of the
// element when nothing is selected; the edit goes to the session after a pause, as the Code text does.
type bodyText struct {
	ctl     *g.TextView
	shown   string
	changes int
	focused bool
}

func (in *inspector) bodyRow() func() g.Option {
	b := &bodyText{}
	v := in.v
	read := func() string {
		if v.session == nil {
			return ""
		}
		if sel := v.session.Selection(); len(sel) == 1 {
			return v.session.BodyOf(sel[0])
		}
		return v.session.Description()
	}
	apply := func(item *model.DiagramItem, text string) {
		if v.session == nil {
			return
		}
		if item != nil {
			v.session.CommitEdit(item, editor.EditBody, text)
		} else {
			v.session.SetDescription(text)
		}
	}
	return func() g.Option {
		f := &inspField{when: always, refresh: func(selKind) {
			if b.focused {
				return
			}
			in.loading = true
			b.shown = read()
			b.ctl.SetText(b.shown)
			in.loading = false
		}}
		in.cur.fields = append(in.cur.fields, f)
		return g.VBox(g.Ref(&f.row), g.GrowX(), g.Padding(0), g.Gap(0),
			// The requested width is the room a value has: a multiline field asks for forty characters
			// otherwise, and that widened the panel.
			g.Text(g.ID("insp-body"), g.Multiline(), g.Border(), g.Scrollbars(),
				g.Size(valueWidth(), g.Scaled(96)),
				g.Ref(&b.ctl), g.Init(func(t *g.TextView) {
					t.OnChange(func(text string) {
						if in.loading || v.session == nil || text == b.shown {
							return
						}
						b.changes++
						n := b.changes
						var item *model.DiagramItem
						if sel := v.session.Selection(); len(sel) == 1 {
							item = sel[0]
						}
						v.app.After(propsDelay, func() {
							if n == b.changes {
								b.shown = text
								apply(item, text)
							}
						})
					})
					t.Unwrap().AddFocusListener(swt.FocusListenerFocusGainedAdapter(func(*swt.FocusEvent) {
						b.focused = true
						v.activeText = t.Unwrap()
						v.updateMenus()
					}))
					t.Unwrap().AddFocusListener(swt.FocusListenerFocusLostAdapter(func(*swt.FocusEvent) {
						b.focused = false
						if v.activeText == t.Unwrap() {
							v.activeText = nil
							v.updateMenus()
						}
						in.schedule()
					}))
				})))
	}
}
