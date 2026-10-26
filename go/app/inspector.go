//go:build darwin || linux || windows

package app

import (
	"fmt"
	"strconv"
	"strings"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/swt"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/model"
	"github.com/haiodo/tenniarb/go/render"
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
)

func (k selKind) item() bool  { return k == kindItem || k == kindItems }
func (k selKind) link() bool  { return k == kindLink || k == kindLinks }
func (k selKind) multi() bool { return k == kindItems || k == kindLinks || k == kindMixed }
func (k selKind) sel() bool   { return k != kindNone }

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
	box    *g.Panel
	title  *g.LabelView
	label  func(k selKind) string
	when   func(k selKind) bool
	fields []*inspField
}

// inspector is the right column: the properties of the selection as compact sections, and the text
// of the same properties as the last, collapsible section.
type inspector struct {
	v        *Viewer
	panel    *g.Panel
	scroll   *g.ScrollView
	sections []*inspSection
	cur      *inspSection
	kind     selKind
	loading  bool
	pending  bool

	codeBody *g.Panel
	codeHead *g.ButtonView
	codeOpen bool
}

func (in *inspector) src() propSrc { return in.v.selSrc() }

func faint() g.Option { return g.Foreground(g.SecondaryText) }

// labelWidth is the width of the caption column of a row.
func labelWidth() int { return g.Scaled(68) }

// section starts a section; the fields built until the next call belong to it.
func (in *inspector) section(label func(selKind) string, when func(selKind) bool, rows ...func() g.Option) g.Option {
	s := &inspSection{label: label, when: when}
	in.sections = append(in.sections, s)
	in.cur = s
	opts := []g.Option{g.Ref(&s.box), g.GrowX(), g.Padding(0), g.Gap(g.Scaled(6)),
		g.Separator(g.GrowX()),
		g.Label("", g.Ref(&s.title), g.GrowX(), g.Typeface(sectionFont()))}
	for _, r := range rows {
		opts = append(opts, r())
	}
	return g.VBox(opts...)
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
	src     func() propSrc
	hint    string // the hint when the value is not set and there is no effective one
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
	return g.Text(g.ID(e.cfg.id), g.Ref(&e.ctl), g.GrowX(), g.Size(g.Scaled(48), 0), g.Init(func(t *g.TextView) {
		t.OnActivate(e.commit)
		t.Unwrap().AddFocusListener(swt.FocusListenerFocusGainedAdapter(func(*swt.FocusEvent) { e.gained() }))
		t.Unwrap().AddFocusListener(swt.FocusListenerFocusLostAdapter(func(*swt.FocusEvent) { e.lost() }))
	}))
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
	default:
		e.cfg.src().arg(e.cfg.name, e.cfg.idx, text)
	}
	e.shown = e.ctl.Text()
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

// pair adds a row with two entries labelled a and b.
func (in *inspector) pair(caption, a, b string, when func(selKind) bool, c1, c2 entryCfg) func() g.Option {
	e1, e2 := in.newEntry(c1), in.newEntry(c2)
	small := func(s string) g.Option {
		return g.Label(s, faint(), g.Typeface(g.Type.Caption), g.Place(g.AlignStart, g.AlignCenter))
	}
	return in.row(caption, when, func(selKind) { e1.refresh(); e2.refresh() },
		small(a), e1.option(), small(b), e2.option())
}

// colorOf parses the text of a colour; false for an expression or a name it does not know.
func colorOf(text string) (g.RGB, bool) {
	c, ok := render.ParseColor(text)
	if !ok {
		return g.RGB{}, false
	}
	to8 := func(f float64) uint8 { return uint8(f*255 + 0.5) }
	return g.RGB{R: to8(c.R), G: to8(c.G), B: to8(c.B)}, true
}

// color adds a row with a swatch that opens the colour chooser and a text with the name or hex.
func (in *inspector) color(caption string, when func(selKind) bool, cfg entryCfg) func() g.Option {
	e := in.newEntry(cfg)
	var sw *g.CanvasView
	cur := g.RGB{R: 255, G: 255, B: 255}
	mixed := false
	paint := func(gc *g.GC) {
		w, h := gc.Size()
		raw := gc.Unwrap()
		raw.SetAntialias(swt.ON)
		if mixed {
			raw.SetBackground(raw.GetDevice().GetSystemColor(swt.COLOR_WIDGET_BACKGROUND))
		} else {
			raw.SetBackground(swt.NewColorRedGreenBlue(int32(cur.R), int32(cur.G), int32(cur.B)))
		}
		raw.FillRoundRectangle(1, 1, int32(w-2), int32(h-2), 8, 8)
		raw.SetForeground(swt.NewColorRedGreenBlueAlpha(128, 128, 128, 160))
		raw.DrawRoundRectangle(1, 1, int32(w-3), int32(h-3), 8, 8)
		if mixed {
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
				if rgb, ok := in.v.win.ColorDialog(cur); ok {
					e.cfg.src().arg(cfg.name, cfg.idx, fmt.Sprintf("#%02x%02x%02x", rgb.R, rgb.G, rgb.B))
				}
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
		if c, ok := colorOf(text); ok {
			cur = c
		}
		e.refresh()
		sw.Redraw()
	}
	return in.row(caption, when, refresh, swatch, e.option())
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

// layoutCombo chooses the horizontal or the vertical word of `layout`, keeping the other word.
func (in *inspector) layoutCombo(caption string, when func(selKind) bool, id string, mine, other []string, dflt string) func() g.Option {
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
		if m == "" {
			m = dflt
		}
		i := -1
		if !p.Mixed {
			for j, o := range mine {
				if o == m {
					i = j
				}
			}
		}
		in.loading = true
		c.Select(i)
		in.loading = false
	}
	ctl := g.Combo(mine, g.ReadOnly(), g.ID(id), g.Ref(&c), g.GrowX(), g.Size(g.Scaled(60), 0), g.Init(func(c *g.ComboView) {
		c.OnSelect(func(i int) {
			if in.loading || i < 0 || in.v.session == nil {
				return
			}
			_, o := words()
			args := []string{mine[i]}
			if o != "" {
				args = append(args, o)
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

// info adds a read-only row.
func (in *inspector) info(caption string, when func(selKind) bool, text func(selKind) string) func() g.Option {
	var l *g.LabelView
	return in.row(caption, when, func(k selKind) { l.SetText(text(k)) }, g.Label("", g.Ref(&l), g.GrowX(), g.Size(g.Scaled(40), 0)))
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
				entryCfg{id: "insp-grid-y", name: "grid", idx: 1, numeric: true, keep: true}),
			in.combo("Layer", selected, "insp-layer", "layer", []string{"default", "background", "hover"}, "default", nil)),
		in.section(sectionTitle("Layout"), items,
			in.pair("Position", "X", "Y", items,
				entryCfg{id: "insp-pos-x", name: "pos", numeric: true, keep: true},
				entryCfg{id: "insp-pos-y", name: "pos", idx: 1, numeric: true, keep: true}),
			in.pair("Size", "W", "H", items, n("insp-width", "width"), n("insp-height", "height"))),
		in.section(sectionTitle("Fill"), notLink,
			in.color("Color", notLink, entryCfg{id: "insp-color", name: "color"})),
		in.section(sectionTitle("Text"), selected,
			in.color("Color", selected, entryCfg{id: "insp-text-color", name: "text-color"}),
			in.text("Size", selected, n("insp-font-size", "font-size")),
			in.layoutCombo("Align", notLink, "insp-align", horizontalLayouts, verticalLayouts, "center"),
			in.layoutCombo("Vertical", notLink, "insp-valign", verticalLayouts, horizontalLayouts, "middle"),
			in.text("Marker", items, entryCfg{id: "insp-marker", name: "marker"})),
		in.section(sectionTitle("Border"), items,
			in.color("Color", items, entryCfg{id: "insp-border-color", name: "border-color"}),
			in.text("Width", items, n("insp-line-width", "line-width")),
			in.combo("Style", items, "insp-line-style", "line-style", []string{"solid", "dashed", "dotted"}, "solid", nil),
			in.text("Radius", items, n("insp-radius", "corner-radius"))),
		in.section(sectionTitle("Shape"), items,
			in.combo("Display", items, "insp-display", "display", editor.ItemDisplay, "rect", nil)),
		in.section(sectionTitle("Line"), links,
			in.color("Color", links, entryCfg{id: "insp-line-color", name: "color"}),
			in.text("Width", links, n("insp-link-width", "line-width")),
			in.combo("Style", links, "insp-link-style", "line-style", []string{"solid", "dashed", "dotted"}, "solid", nil),
			in.combo("Arrows", links, "insp-arrows", "display", []string{"solid", "arrow", "arrows", "arrow-source"}, "solid", nil)),
		in.section(sectionTitle("Effects"), notLink,
			in.toggle("Shadow", notLink, "insp-shadow", "shadow", []string{"-5", "-5", "5"}, nil),
			in.pair("Offset", "X", "Y", notLink,
				entryCfg{id: "insp-shadow-x", name: "shadow", numeric: true},
				entryCfg{id: "insp-shadow-y", name: "shadow", idx: 1, numeric: true}),
			in.text("Blur", notLink, entryCfg{id: "insp-shadow-blur", name: "shadow", idx: 2, numeric: true}),
			in.color("Color", notLink, entryCfg{id: "insp-shadow-color", name: "shadow", idx: 3})),
		in.section(sectionTitle("Item defaults"), none,
			in.color("Fill", none, entryCfg{id: "insp-def-color", name: "color", src: itemStyles}),
			in.color("Border", none, entryCfg{id: "insp-def-border", name: "border-color", src: itemStyles}),
			in.color("Text", none, entryCfg{id: "insp-def-text", name: "text-color", src: itemStyles}),
			in.text("Font size", none, entryCfg{id: "insp-def-font", name: "font-size", numeric: true, src: itemStyles}),
			in.combo("Display", none, "insp-def-display", "display", editor.ItemDisplay, "rect", itemStyles),
			in.toggle("Shadow", none, "insp-def-shadow", "shadow", []string{"-5", "-5", "5"}, itemStyles)),
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
			})),
		in.section(func(k selKind) string {
			if k == kindNone {
				return "Description"
			}
			return "Body"
		}, func(k selKind) bool { return k == kindItem || k == kindNone }, in.bodyRow()),
		in.codeSection())
	return g.Scroll(g.Grow(), g.Padding(0), g.Gap(0), g.Ref(&in.scroll), body)
}

// codeSection is the text of the properties, closed by default: the raw .tenn the sections above edit.
func (in *inspector) codeSection() g.Option {
	return g.VBox(g.GrowX(), g.Padding(0), g.Gap(g.Scaled(6)),
		g.Separator(g.GrowX()),
		g.Button("Code", g.Borderless(), g.ID("insp-code"), g.Ref(&in.codeHead), g.Place(g.AlignStart, g.AlignCenter), g.Init(func(b *g.ButtonView) {
			b.OnClick(func() { in.setCode(!in.codeOpen) })
		})),
		g.VBox(g.Ref(&in.codeBody), g.GrowX(), g.Padding(0),
			g.StyledText(g.Scrollbars(), g.Border(), g.GrowX(), g.Size(0, g.Scaled(220)), g.ID("props"), g.Init(in.v.initProps))))
}

func (in *inspector) setCode(open bool) {
	in.codeOpen = open
	in.codeBody.SetVisible(open)
	chevron := "▸ Code"
	if open {
		chevron = "▾ Code"
	}
	in.codeHead.SetText(chevron)
	in.relayout()
}

func (in *inspector) relayout() {
	if in.scroll == nil || in.scroll.Unwrap().IsDisposed() {
		return
	}
	in.scroll.Content().Unwrap().LayoutOverload1(true)
	in.scroll.Fit()
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
	if in.scroll == nil || in.scroll.Unwrap().IsDisposed() {
		return
	}
	k := kindNone
	if v.session != nil {
		k = kindOf(v.session.Selection())
	}
	changed := k != in.kind
	in.kind = k
	for _, s := range in.sections {
		show := v.session != nil && s.when(k)
		if s.box.Visible() != show {
			s.box.SetVisible(show)
			changed = true
		}
		if !show {
			continue
		}
		s.title.SetText(s.label(k))
		for _, f := range s.fields {
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
			g.Text(g.ID("insp-body"), g.Multiline(), g.Border(), g.Scrollbars(), g.GrowX(), g.Size(g.Scaled(48), g.Scaled(96)),
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
