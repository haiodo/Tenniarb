//go:build darwin || linux || windows

package app

import (
	"fmt"
	"strings"

	g "github.com/haiodo/gowt"
	"github.com/haiodo/gowt/look"
	"github.com/haiodo/gowt/swt"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/model"
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
	// insp-scroll names the pane for the QA driver: a window test reads the geometry of what it scrolls,
	// the only place the scrollable height shows.
	return g.Scroll(g.Grow(), g.Padding(0), g.Gap(0), g.NoHScroll(), g.ID("insp-scroll"), g.Ref(&in.scroll), body)
}

func (in *inspector) relayout() {
	if in.scroll == nil || in.scroll.Unwrap().IsDisposed() {
		return
	}
	in.scroll.Content().Unwrap().LayoutOverload1(true)
	in.scroll.Fit()
	// Fit leaves the scrollable size at the natural size of the content. Its height is what the vertical
	// bar scrolls over and must stay: SetMinSizeWidthHeight(w, 0) threw the measured height away and a
	// long inspector had nothing to scroll - 8e826d5 clamped the width that way and lost the scroll.
	// Only the width is held to the room the panel gives it, so a row that cannot wrap is cut instead of
	// the pane growing past the fixed width of the inspector.
	if c := in.scroll.Unwrap(); c != nil && !c.IsDisposed() {
		c.SetMinWidth(int32(inspectorWidth() - 2*g.Scaled(8)))
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
