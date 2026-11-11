//go:build darwin || linux || windows

package app

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	g "github.com/haiodo/gowt"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/scene"
	"github.com/haiodo/tenniarb/go/tenn"
)

// sceneSrc reads the property a scene node writes itself (an override, for a use) and writes it as a
// text edit; eff is the value in effect, the component's or the default, shown as the hint.
func (v *Viewer) sceneSrc() propSrc {
	own := func(name string) *scene.Prop {
		if ref, ok := v.selectedInst(); ok {
			if at := atOf(ref.use, ref.at); at != nil {
				return atProp(at, name)
			}
			return nil
		}
		if n, _ := v.selectedScene(); n != nil {
			return ownProp(n, name)
		}
		return nil
	}
	values := func(name string) []string {
		if p := own(name); p != nil {
			return p.Values()
		}
		if _, pl := v.selectedScene(); pl != nil {
			if p := pl.Prop(name); p != nil {
				return p.Values()
			}
		}
		return nil
	}
	set := func(name string, args ...string) bool {
		if ref, ok := v.selectedInst(); ok {
			return v.setInstProp(ref, name, args...)
		}
		n, _ := v.selectedScene()
		if n == nil {
			return false
		}
		e, ok := SetPropEdit(v.fileText(v.ownFileOf(n)), n, name, args...)
		return ok && v.sceneEditIn(v.ownFileOf(n), n, e)
	}
	return propSrc{
		get: func(name string) editor.Prop {
			p := own(name)
			if p == nil {
				return editor.Prop{}
			}
			out := editor.Prop{Set: true}
			for i, val := range p.Values() {
				expr := p.Args[i].Kind == tenn.KindExpression || p.Args[i].Kind == tenn.KindExpressionBlock
				out.Args = append(out.Args, editor.Arg{Text: val, Expr: expr})
			}
			return out
		},
		set: set,
		arg: func(name string, i int, text string) bool {
			args := values(name)
			if text == "" && i == 0 {
				return set(name)
			}
			for len(args) <= i {
				args = append(args, "0")
			}
			args[i] = text
			return set(name, args...)
		},
		eff: func(name string) string {
			if _, pl := v.selectedScene(); pl != nil {
				if p := pl.Prop(name); p != nil {
					return strings.Join(p.Values(), " ")
				}
			}
			return ""
		},
	}
}

// sceneSections are the inspector of a scene element: the selected node's properties, by the names
// of spec section 5 and 5a.10.
func (in *inspector) sceneSections() g.Option {
	v := in.v
	in.building = true
	defer func() { in.building = false }()
	src := v.sceneSrc
	node := func(k selKind) bool { return k == kindScene && v.shownNode() != nil }
	// A part of a component has no node of its own: its shadows live in an at override, which the list
	// does not edit.
	own := func(k selKind) bool {
		n, _ := v.selectedScene()
		return node(k) && n != nil
	}
	scr := func(k selKind) bool { return k == kindScene }
	t := func(id, name string) entryCfg { return entryCfg{id: id, name: name, src: src} }
	num := func(id, name string, idx int) entryCfg {
		return entryCfg{id: id, name: name, idx: idx, numeric: true, src: src}
	}
	kindText := func(n *scene.Node) string {
		switch n.Kind {
		case scene.KUse:
			return "Instance"
		case scene.KSlot:
			return "Slot"
		case scene.KItem:
			if len(editor.SceneKids(n)) > 0 {
				return "Frame"
			}
		}
		return "Item"
	}
	title := func(selKind) string {
		if n := v.shownNode(); n != nil {
			return kindText(n)
		}
		return "Screen"
	}
	nameOf := func(selKind) string {
		if n := v.shownNode(); n != nil {
			return n.Name
		}
		if v.sv != nil && v.sv.el != nil {
			return v.sv.el.Name
		}
		return ""
	}
	size := func(selKind) string {
		_, pl := v.selectedScene()
		if pl == nil && v.sv != nil {
			pl = v.sv.placed
		}
		if pl == nil {
			return ""
		}
		f := func(x float64) string { return strconv.FormatFloat(x, 'f', -1, 64) }
		return f(pl.W) + " x " + f(pl.H)
	}
	isUse := func(k selKind) bool {
		n := v.shownNode()
		return k == kindScene && n != nil && n.Kind == scene.KUse
	}
	hasStyles := func(k selKind) bool { return node(k) && len(v.styleGroups()) > 0 }
	hasAnim := func(k selKind) bool { return node(k) && v.animOf() != nil }
	hasParams := func(k selKind) bool { return k == kindScene && v.paramNode() != nil }
	sceneCfg := nodeCfg{
		prefix:  "scene",
		src:     src,
		words:   true,
		layout:  node,
		fill:    node,
		text:    node,
		border:  node,
		shape:   node,
		display: node,
		layer:   node,
	}
	// The component of an instance is a way to the place it is defined, not only a name. A button cannot
	// wrap, so it shows the tail of the path and holds the whole one as a tooltip.
	var comp *g.ButtonView
	compText := func(selKind) {
		n := v.shownNode()
		if comp == nil || n == nil {
			return
		}
		comp.SetText(shortPath(n.Path))
		comp.Unwrap().SetToolTipText(n.Path)
	}
	return g.VBox(g.GrowX(), g.Padding(0), g.Gap(g.Scaled(10)),
		in.section(title, scr,
			in.info("Name", scr, nameOf),
			in.row("Component", isUse, compText,
				g.Button("", g.Small(), g.GrowX(), g.Size(valueWidth(), 0), g.Ref(&comp), g.ID("insp-component"), g.Init(func(b *g.ButtonView) {
					b.OnClick(v.goToComponent)
				}))),
			in.info("Size", scr, size)),
		in.section(sectionTitle("Parameters"), hasParams,
			in.sceneParams(hasParams)),
		in.section(sectionTitle("Animation"), hasAnim,
			in.sceneAnimation(hasAnim)),
		in.section(sectionTitle("Styles"), hasStyles,
			in.sceneStyles(hasStyles)),
		in.layoutSection(sceneCfg, []func() g.Option{
			in.combo("Auto layout", node, "scene-auto-layout", "auto-layout", []string{"none", "row", "column", "abs"}, "none", src),
		}, []func() g.Option{
			in.text("Gap", node, num("scene-gap", "gap", 0)),
			in.combo("Align", node, "scene-align", "align", []string{noneChoice, "start", "center", "end"}, noneChoice, src),
			in.combo("Justify", node, "scene-justify", "justify", []string{noneChoice, "start", "center", "end", "space-between"}, noneChoice, src),
			in.toggle("Clip", node, "scene-clip", "clip", []string{"yes"}, src),
		}),
		in.fillSection(sceneCfg, nil, nil),
		in.textSection(sceneCfg, []func() g.Option{
			in.text("Title", node, t("scene-title", "title")),
		}, []func() g.Option{
			in.text("Font", node, t("scene-font-family", "font-family")),
			in.text("Weight", node, t("scene-font-weight", "font-weight")),
		}),
		in.borderSection(sceneCfg, nil, nil),
		in.shapeSection(sceneCfg, nil, nil),
		in.section(sectionTitle("Effects"), node,
			in.text("Opacity", node, num("scene-opacity", "opacity", 0)),
			in.combo("Preset", node, "scene-shadow", "shadow", []string{noneChoice, "soft", "floating", customShadow}, noneChoice, v.shadowSrc),
			in.shadowList(own, "scene-shadow-list", v.sceneShadows, nil, v.setSceneShadows),
			in.toggle("Hidden", node, "scene-hidden", "hidden", []string{"yes"}, src)))
}

// sceneStyles is the Styles section of a scene: the groups the nearest styles block above the selected
// node declares (spec 5b), with what the node names. The rows are built again when the selection or its
// groups change, since a group belongs to one element.
func (in *inspector) sceneStyles(shown func(selKind) bool) func() g.Option {
	v := in.v
	var body *g.Panel
	key := ""
	var refresh []func()
	f := &inspField{when: shown, refresh: func(selKind) {
		if k := v.styleKey(); k != key {
			key = k
			refresh = nil
			body.Clear()
			for _, gr := range v.styleGroups() {
				rows, refs := in.styleGroup(gr)
				body.Add(rows...)
				refresh = append(refresh, refs...)
			}
		}
		for _, r := range refresh {
			r()
		}
	}}
	return func() g.Option {
		in.cur.fields = append(in.cur.fields, f)
		return g.VBox(g.Ref(&f.row), g.GrowX(), g.Padding(0), g.Gap(g.Scaled(6)), g.Ref(&body))
	}
}

// sceneParams is the Parameters section of a scene: the parameters the component of the selected use
// declares (spec 5c), a group of its styles as a switch or as check boxes, any other as a field. The rows
// are built again when the selection or the declaration changes, since the parameters belong to one
// component.
func (in *inspector) sceneParams(shown func(selKind) bool) func() g.Option {
	v := in.v
	var body *g.Panel
	key := ""
	var refresh []func()
	f := &inspField{when: shown, refresh: func(selKind) {
		if k := v.paramKey(); k != key {
			key = k
			refresh = nil
			body.Clear()
			if t := v.paramNode(); t != nil {
				for _, prm := range t.Params {
					row, r := in.paramRow(prm)
					body.Add(row)
					refresh = append(refresh, r)
				}
			}
		}
		for _, r := range refresh {
			r()
		}
	}}
	return func() g.Option {
		in.cur.fields = append(in.cur.fields, f)
		return g.VBox(g.Ref(&f.row), g.GrowX(), g.Padding(0), g.Gap(g.Scaled(6)), g.Ref(&body))
	}
}

// paramRow is one parameter of the selected use: a segmented switch for a one-of group of the component's
// styles, a check box per style for an any-of group, else a text field. It returns the row and the refresh
// of the state of its controls.
func (in *inspector) paramRow(prm scene.Param) (g.Option, func()) {
	v := in.v
	id := "scene-param-" + prm.Name
	gr := v.paramGroup(prm)
	switch {
	case gr != nil && gr.AnyOf:
		var rows []g.Option
		var refresh []func()
		for i, s := range gr.Styles {
			caption := prm.Name
			if i > 0 {
				caption = ""
			}
			style := s
			var btn *g.ButtonView
			rows = append(rows, styleRow(caption, g.Button(style, g.Check(), g.Small(),
				g.ID(id+"-"+style), g.Ref(&btn), g.Init(func(b *g.ButtonView) {
					b.OnClick(func() {
						if !in.loading {
							v.toggleParam(prm, style)
						}
					})
				}))))
			refresh = append(refresh, func() {
				in.loading = true
				btn.SetChecked(slices.Contains(v.paramValues(prm), style))
				in.loading = false
			})
		}
		return g.VBox(append([]g.Option{g.GrowX(), g.Padding(0), g.Gap(g.Scaled(6))}, rows...)...),
			func() {
				for _, r := range refresh {
					r()
				}
			}
	case gr != nil:
		var combo *g.ComboView
		ctl := g.Combo(gr.Styles, g.ReadOnly(), g.ID(id), g.Ref(&combo), g.GrowX(), g.Size(g.Scaled(60), 0),
			g.Init(func(c *g.ComboView) {
				c.OnSelect(func(i int) {
					if !in.loading && i >= 0 && i < len(gr.Styles) {
						v.setParam(prm, gr.Styles[i])
					}
				})
			}))
		refresh := func() {
			at := -1 // no style of the group named: nothing chosen
			for i, s := range gr.Styles {
				if slices.Contains(v.paramValues(prm), s) {
					at = i // the last one named wins
				}
			}
			in.loading = true
			combo.Select(at)
			in.loading = false
		}
		return styleRow(prm.Name, ctl), refresh
	}
	e := in.newEntry(entryCfg{id: id, name: prm.Name, src: func() propSrc { return v.paramSrc(prm) }})
	return styleRow(prm.Name, e.option()), e.refresh
}

// styleGroup is the control of one group: a segmented switch for one-of, which takes exactly one style,
// and a check box per style for any-of. It returns the rows and the refresh of state of their controls.
func (in *inspector) styleGroup(gr scene.StyleGroup) ([]g.Option, []func()) {
	v := in.v
	set := func(s string) { v.setGroupStyle(gr, s) }
	if gr.AnyOf {
		var rows []g.Option
		var refresh []func()
		for i, s := range gr.Styles {
			caption := gr.Name
			if i > 0 {
				caption = ""
			}
			var btn *g.ButtonView
			rows = append(rows, styleRow(caption, g.Button(s, g.Check(), g.Small(),
				g.ID("scene-style-"+gr.Name+"-"+s), g.Ref(&btn), g.Init(func(b *g.ButtonView) {
					b.OnClick(func() {
						if !in.loading {
							set(s)
						}
					})
				}))))
			refresh = append(refresh, func() {
				in.loading = true
				btn.SetChecked(slices.Contains(v.nodeStyles(), s))
				in.loading = false
			})
		}
		return rows, refresh
	}
	// A drop-down, not a switch: six segment labels make the whole inspector that wide.
	var combo *g.ComboView
	ctl := g.Combo(gr.Styles, g.ReadOnly(), g.ID("scene-style-"+gr.Name), g.Ref(&combo), g.GrowX(),
		g.Size(g.Scaled(60), 0), g.Init(func(c *g.ComboView) {
			c.OnSelect(func(i int) {
				if !in.loading && i >= 0 && i < len(gr.Styles) {
					set(gr.Styles[i])
				}
			})
		}))
	refresh := func() {
		at := -1 // no style of the group named: nothing chosen
		for i, s := range gr.Styles {
			if slices.Contains(v.nodeStyles(), s) {
				at = i // the last one named wins
			}
		}
		in.loading = true
		combo.Select(at)
		in.loading = false
	}
	return []g.Option{styleRow(gr.Name, ctl)}, []func(){refresh}
}

// animOf is the animation of the selected item, nil when it has none.
func (v *Viewer) animOf() *scene.Animation {
	_, pl := v.selectedScene()
	if pl == nil || pl.Gfx == nil || v.sv == nil || v.sv.screen == nil {
		return nil
	}
	return v.sv.screen.AnimationOf(pl.Gfx)
}

// sceneAnimation is the Animation section: what plays on the item, a play switch and a slider that walks
// through its frames - the editor previews every animation in a loop, the slider stops it at a moment.
func (in *inspector) sceneAnimation(shown func(selKind) bool) func() g.Option {
	v := in.v
	var play *g.ButtonView
	var frame *g.SliderView
	var text *g.LabelView
	f := &inspField{when: shown, refresh: func(selKind) {
		a := v.animOf()
		in.loading = true
		play.SetChecked(!v.animOff)
		if a != nil {
			text.SetText(animTitle(a))
			frame.Unwrap().SetEnabled(!v.animOff)
		}
		in.loading = false
	}}
	in.cur.fields = append(in.cur.fields, f)
	return func() g.Option {
		return g.VBox(g.GrowX(), g.Padding(0), g.Gap(g.Scaled(6)),
			styleRow("Animation", g.Label("", faint(), g.Wrap(), g.GrowX(), g.Size(g.Scaled(40), 0), g.Ref(&text))),
			styleRow("Play", g.Button("Play", g.Check(), g.Small(), g.ID("scene-anim-play"),
				g.Ref(&play), g.Init(func(b *g.ButtonView) {
					b.OnClick(func() {
						if in.loading {
							return
						}
						if v.animOff {
							v.animPlay()
						} else {
							v.animOff = true
							v.stopAnim()
						}
					})
				}))),
			// The hint bounds what the row asks for: ten scroller widths are wider than the inspector.
			styleRow("Frame", g.Slider(0, 100, g.GrowX(), g.Size(g.Scaled(40), 0), g.ID("scene-anim-frame"), g.Ref(&frame),
				g.Init(func(s *g.SliderView) {
					s.OnChange(func(at int) {
						if !in.loading {
							v.animScrub(float64(at) / 100)
						}
					})
				}))))
	}
}

// animTitle is the animation as the inspector shows it: the name, the duration and the easing.
func animTitle(a *scene.Animation) string {
	d := a.Duration
	if d%time.Second == 0 {
		return fmt.Sprintf("%s, %ds %s", a.Name, int(d/time.Second), a.Easing)
	}
	return fmt.Sprintf("%s, %dms %s", a.Name, d/time.Millisecond, a.Easing)
}

// styleRow is a row of the Styles or Parameters section: the caption on the left, the controls on the
// right, as every other row of the inspector.
func styleRow(caption string, ctl ...g.Option) g.Option {
	opts := []g.Option{g.GrowX(), g.Padding(0), g.Gap(g.Scaled(8)),
		g.Label(caption, faint(), g.Size(labelWidth(), 0), g.Place(g.AlignStart, g.AlignCenter))}
	return g.HBox(append(opts, ctl...)...)
}
