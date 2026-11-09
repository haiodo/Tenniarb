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
			if len(sceneKids(n)) > 0 {
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
		in.section(sectionTitle("Layout"), node,
			in.combo("Auto layout", node, "scene-auto-layout", "auto-layout", []string{"none", "row", "column", "abs"}, "none", src),
			in.pair("Position", "X", "Y", node, num("scene-pos-x", "pos", 0), num("scene-pos-y", "pos", 1)),
			in.pair("Size", "W", "H", node, t("scene-width", "width"), t("scene-height", "height")),
			in.sizeModes("W", "scene-width-mode", "width", 0, node, func() float64 { w, _ := v.itemSize(); return w }, src),
			in.sizeModes("H", "scene-height-mode", "height", 0, node, func() float64 { _, h := v.itemSize(); return h }, src),
			in.text("Gap", node, num("scene-gap", "gap", 0)),
			in.text("Padding", node, t("scene-padding", "padding")),
			in.combo("Align", node, "scene-align", "align", []string{noneChoice, "start", "center", "end"}, noneChoice, src),
			in.combo("Justify", node, "scene-justify", "justify", []string{noneChoice, "start", "center", "end", "space-between"}, noneChoice, src),
			in.toggle("Clip", node, "scene-clip", "clip", []string{"yes"}, src)),
		in.section(sectionTitle("Fill"), node,
			in.color("Color", node, t("scene-color", "color"))),
		in.section(sectionTitle("Border"), node,
			in.color("Color", node, t("scene-border-color", "border-color")),
			in.text("Width", node, num("scene-line-width", "line-width", 0)),
			in.text("Radius", node, t("scene-radius", "corner-radius"))),
		in.section(sectionTitle("Text"), node,
			in.text("Title", node, t("scene-title", "title")),
			in.color("Color", node, t("scene-text-color", "text-color")),
			in.text("Size", node, num("scene-font-size", "font-size", 0)),
			in.text("Font", node, t("scene-font-family", "font-family")),
			in.text("Weight", node, t("scene-font-weight", "font-weight"))),
		in.section(sectionTitle("Effects"), node,
			in.text("Opacity", node, num("scene-opacity", "opacity", 0)),
			in.combo("Shadow", node, "scene-shadow", "shadow", []string{noneChoice, "soft", "floating", customShadow}, noneChoice, v.shadowSrc),
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

// styleGroups are the groups of the styles the selected node already names: the nearest styles block
// above it. A node that names no style gets no choice - the styles of a component belong to the part they
// were written for, not to every item inside it. A group that names no style is left out.
func (v *Viewer) styleGroups() []scene.StyleGroup {
	n := v.shownNode()
	if n == nil || len(v.nodeStyles()) == 0 {
		return nil
	}
	scope := scene.StyleScope(n)
	if scope == nil {
		return nil
	}
	var out []scene.StyleGroup
	for _, gr := range scope.StyleGroups {
		if len(gr.Styles) > 0 {
			out = append(out, gr)
		}
	}
	return out
}

// styleKey names what the rows of the Styles section are built from: the selected node and its groups.
func (v *Viewer) styleKey() string { return v.sceneCodeKey() + fmt.Sprint(v.styleGroups()) }

// paramUse is the use the Parameters section describes: the selected node when it is the root of an
// instance written in the element. A part of an instance has no use of its own, and its parameters belong
// to the instance, not to the part.
func (v *Viewer) paramUse() *scene.Node {
	n, _ := v.selectedScene()
	if n == nil || n.Kind != scene.KUse {
		return nil
	}
	return n
}

// paramNode is the component the selected use draws, nil when it declares no parameter: the section is
// absent then, not empty.
func (v *Viewer) paramNode() *scene.Node {
	use := v.paramUse()
	if use == nil || use.Parent == nil || v.stree == nil {
		return nil
	}
	t, err := v.stree.Resolve(use.Parent, use.Path)
	if err != nil || t == nil || len(t.Params) == 0 {
		return nil
	}
	return t
}

// paramKey names what the rows of the Parameters section are built from: the selected use and the
// declaration of its component.
func (v *Viewer) paramKey() string {
	t := v.paramNode()
	if t == nil {
		return ""
	}
	key := v.sceneCodeKey()
	for _, prm := range t.Params {
		key += "\x01" + prm.Name
		if prm.Default != nil {
			key += "=" + strings.Join(prm.Default.Values(), " ")
		}
	}
	for _, gr := range t.StyleGroups {
		key += "\x02" + fmt.Sprint(gr)
	}
	return key
}

// paramGroup is the group of the component's styles the default of prm names, nil for a parameter that is
// no choice among declared styles and so gets a field.
func (v *Viewer) paramGroup(prm scene.Param) *scene.StyleGroup {
	t := v.paramNode()
	if t == nil || prm.Default == nil || len(prm.Default.Args) == 0 {
		return nil
	}
	def := scene.NormalizeName(prm.Default.Values()[0])
	scope := t
	if len(scope.StyleGroups) == 0 {
		scope = scene.StyleScope(t) // a library declares the styles above the component
	}
	if scope == nil {
		return nil
	}
	for i := range scope.StyleGroups {
		if slices.Contains(scope.StyleGroups[i].Styles, def) {
			return &scope.StyleGroups[i]
		}
	}
	return nil
}

// paramValues are the values the parameter of the selected use holds in effect, as style names: what the
// use sets for it, else the default of the declaration.
func (v *Viewer) paramValues(prm scene.Param) []string {
	raw := v.paramCurrent(prm)
	out := make([]string, len(raw))
	for i, s := range raw {
		out[i] = scene.NormalizeName(s)
	}
	return out
}

// paramCurrent are the values of the parameter as the file writes them.
func (v *Viewer) paramCurrent(prm scene.Param) []string {
	if use := v.paramUse(); use != nil {
		if p := ownProp(use, prm.Name); p != nil {
			return p.Values()
		}
		if prm.Default != nil {
			return prm.Default.Values()
		}
	}
	return nil
}

// setParam writes the values of parameter prm into the block of the selected use: one edit, one undo step.
func (v *Viewer) setParam(prm scene.Param, values ...string) bool {
	use := v.paramUse()
	if use == nil {
		return false
	}
	e, ok := ParamEdit(v.fileText(v.ownFileOf(use)), use, prm, values...)
	return ok && v.sceneEditIn(v.ownFileOf(use), use, e)
}

// removeParam deletes the value of parameter prm of the selected use, so the default of the declaration
// comes back: one edit, one undo step.
func (v *Viewer) removeParam(prm scene.Param) bool {
	use := v.paramUse()
	if use == nil {
		return false
	}
	e, ok := RemoveParamEdit(v.fileText(v.ownFileOf(use)), use, prm)
	return ok && v.sceneEditIn(v.ownFileOf(use), use, e)
}

// toggleParam adds s to the values of an any-of parameter, or drops it.
func (v *Viewer) toggleParam(prm scene.Param, s string) bool {
	want := v.paramValues(prm)
	if slices.Contains(want, s) {
		want = slices.DeleteFunc(slices.Clone(want), func(n string) bool { return n == s })
	} else {
		want = append(want, s)
	}
	return v.setParam(prm, want...)
}

// paramSrc reads and writes the value of parameter prm of the selected use: the command of that name in
// the block of the use, else the default of the declaration, which the field shows as its value.
func (v *Viewer) paramSrc(prm scene.Param) propSrc {
	write := func(values ...string) bool { return v.setParam(prm, values...) }
	prop := func() *scene.Prop {
		use := v.paramUse()
		if use == nil {
			return nil
		}
		if p := ownProp(use, prm.Name); p != nil {
			return p
		}
		return prm.Default
	}
	return propSrc{
		get: func(string) editor.Prop {
			p := prop()
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
		set: func(_ string, args ...string) bool { return write(args...) },
		arg: func(_ string, i int, text string) bool {
			if text == "" && i == 0 { // an empty field drops the value: the default comes back
				return v.removeParam(prm)
			}
			args := slices.Clone(v.paramCurrent(prm))
			for len(args) <= i {
				args = append(args, "0")
			}
			args[i] = text
			return write(args...)
		},
		eff: func(string) string {
			if prm.Default == nil {
				return ""
			}
			return strings.Join(prm.Default.Values(), " ")
		},
	}
}

// nodeStyles are the styles the selected node names in effect: the list of a node of the file, and on a
// part of an instance the defaults of its component with the overrides of the use applied.
func (v *Viewer) nodeStyles() []string {
	if _, pl := v.selectedScene(); pl != nil {
		return scene.StyleRefs(pl.Props())
	}
	if n := v.shownNode(); n != nil {
		return scene.StyleRefs(n.Props)
	}
	return nil
}

// setGroupStyle makes s the style of group gr the node names: a one-of group keeps that one, an any-of
// group adds or drops it. The styles of the other groups stay as they are.
func (v *Viewer) setGroupStyle(gr scene.StyleGroup, s string) bool {
	want := v.nodeStyles()
	if gr.AnyOf && slices.Contains(want, s) {
		want = slices.DeleteFunc(slices.Clone(want), func(n string) bool { return n == s })
	} else {
		want = withStyle(want, gr, s)
	}
	return v.setStyles(want)
}

// withStyle puts s where the styles of the group are in want, so a choice takes the place of the default
// it replaces; at the end when the group holds none of them.
func withStyle(want []string, gr scene.StyleGroup, s string) []string {
	out := make([]string, 0, len(want)+1)
	done := false
	for _, n := range want {
		if slices.Contains(gr.Styles, n) {
			if !done {
				out, done = append(out, s), true
			}
			continue
		}
		out = append(out, n)
	}
	if !done {
		out = append(out, s)
	}
	return out
}

// setStyles writes the styles the node names as one edit: the list itself on a node of the file; on a part
// of an instance what differs from the defaults of its component, a dropped default by name (-medium).
func (v *Viewer) setStyles(want []string) bool {
	n := v.shownNode()
	if n == nil {
		return false
	}
	args := want
	if _, inst := v.selectedInst(); inst {
		// The parts of a component are drawn with its own use-style already evaluated: the styles the
		// override adds or drops are the ones the node shows, not the words of its expression.
		base := v.nodeStyles()
		if slices.Equal(base, want) {
			return true
		}
		args = append(removals(base, want), additions(base, want)...)
	} else if ownUseStyle(n) > 1 {
		// the edit replaces one command, the styles this list drops are removed by name
		args = append(removals(v.nodeStyles(), want), want...)
	}
	return v.sceneSrc().set("use-style", args...)
}

// ownUseStyle counts the use-style commands written on n itself.
func ownUseStyle(n *scene.Node) int {
	if n.Kind == scene.KUse {
		count := 0
		for _, ov := range n.Block {
			if ov.Kind == scene.OvProp && ov.Prop.Name == "use-style" {
				count++
			}
		}
		return count
	}
	count := 0
	for _, p := range n.Props {
		if p.Name == "use-style" {
			count++
		}
	}
	return count
}

// removals are the styles of have that want drops, as use-style drops them: -name.
func removals(have, want []string) []string {
	var out []string
	for _, s := range have {
		if !slices.Contains(want, s) {
			out = append(out, "-"+s)
		}
	}
	return out
}

// additions are the styles of want that have does not name.
func additions(have, want []string) []string {
	var out []string
	for _, s := range want {
		if !slices.Contains(have, s) {
			out = append(out, s)
		}
	}
	return out
}

// pressStyle makes value the choice of the style group name, as a segment of its switch does: the QA
// driver cannot press a segment of a segmented control.
func (v *Viewer) pressStyle(name, value string) error {
	for _, gr := range v.styleGroups() {
		if gr.Name != name {
			continue
		}
		if !slices.Contains(gr.Styles, value) {
			return fmt.Errorf("%q is not a style of the group %q", value, name)
		}
		if !v.setGroupStyle(gr, value) {
			return fmt.Errorf("the style was not applied")
		}
		return nil
	}
	return fmt.Errorf("no style group %q", name)
}

// pressParam makes value the value of parameter name of the selected use, as a segment of its switch does:
// the QA driver cannot press a segment of a segmented control.
func (v *Viewer) pressParam(name, value string) error {
	t := v.paramNode()
	if t == nil {
		return fmt.Errorf("the selection is no instance with parameters")
	}
	prm := t.Param(name)
	if prm == nil {
		return fmt.Errorf("no parameter %q", name)
	}
	gr := v.paramGroup(*prm)
	if gr == nil || gr.AnyOf {
		return fmt.Errorf("parameter %q is no one-of group", name)
	}
	if !slices.Contains(gr.Styles, scene.NormalizeName(value)) {
		return fmt.Errorf("%q is not a style of the group %q", value, gr.Name)
	}
	if !v.setParam(*prm, scene.NormalizeName(value)) {
		return fmt.Errorf("the value was not applied")
	}
	return nil
}

// sceneCodeKey names what the Code section shows for a scene element: the element and the node.
func (v *Viewer) sceneCodeKey() string {
	if v.state.Current == nil {
		return ""
	}
	key := pathKey(v.state.Current) + "\x01"
	if len(v.selNodes) == 0 {
		return key
	}
	return key + v.selNodes[0]
}

// sceneCodeNode is the node the Code section shows: the selected one, or the element itself when nothing
// is selected, so its own properties are there to edit.
func (v *Viewer) sceneCodeNode() (*scene.Node, *scene.Placed) {
	if n, pl := v.selectedScene(); n != nil || pl != nil {
		return n, pl
	}
	return v.elementNode(v.state.Current), nil
}

// refreshSceneCode shows the source of the selected node in the Code section, its kids as one line each;
// with nothing selected, the element itself.
func (v *Viewer) refreshSceneCode() {
	n, pl := v.sceneCodeNode()
	key := v.sceneCodeKey()
	// The keyboard is in the pane: its text is the draft, and the redraw inside the edit arrives before
	// props.want records the edit, so replacing the text here would move the caret after every keystroke.
	if v.propsFocused() && key == v.props.sceneKey {
		return
	}
	want := ""
	switch {
	case n != nil:
		want = OwnText(v.fileText(v.ownFileOf(n)), n)
	case pl != nil:
		want = EffectiveText(pl)
	}
	if key == v.props.sceneKey && v.props.text.Text() == want {
		return
	}
	// What the user typed stays on screen until the code changes the text: a kid written in full keeps
	// its properties, the kids already there stay stubs.
	if key == v.props.sceneKey && want == v.props.want {
		return
	}
	v.setPropsText(want, editor.Target{})
	v.props.sceneKey = key
	v.props.values = nil
}

// applySceneCode puts the text of the Code section in place of the node it was opened for; the kids stay
// as they are written.
func (v *Viewer) applySceneCode(key, text string) {
	if v.props.text.Text() == text {
		v.styleProps(text)
	}
	if key == "" || key != v.sceneCodeKey() || len(editor.ParseDiagnostics(text)) > 0 {
		return
	}
	n, pl := v.sceneCodeNode()
	if n == nil && pl != nil { // a part of a component has no code of its own
		if text != EffectiveText(pl) {
			v.setBanner("The code of a part of an instance is read only: change it in the inspector")
			v.refreshSceneCode()
		}
		return
	}
	src := v.fileText(v.ownFileOf(n))
	if n == nil || OwnText(src, n) == text {
		return
	}
	// The kids are hidden behind stubs: an emptied pane would delete a subtree the user never saw.
	// No refresh here, the user may be retyping the node.
	if strings.TrimSpace(text) == "" {
		v.setBanner("Not applied: delete the item from the Items tree")
		return
	}
	e := ReplaceOwnEdit(src, n, text)
	if e.New == src[e.Start:e.End] { // only stubs changed: show the kids again
		v.refreshSceneCode()
		return
	}
	start := n.Range.Start
	at, length := v.props.text.Selection()
	if !v.sceneEdit(n, e) {
		return
	}
	// A renamed node keeps the selection: it starts where the old one did.
	if len(v.selNodes) == 1 && NodeAt(v.sv.node, v.selNodes[0]) == nil {
		if m := nodeStartingAt(v.sv.node, start); m != nil {
			v.selNodes = []string{NodePath(v.sv.node, m)}
			v.props.sceneKey = v.sceneCodeKey()
			v.drawSceneOverlay()
		}
	}
	// The pane keeps what was typed, as the code now holds it: a kid written in full stays open for the
	// next lines, the kids already there stay stubs.
	if m, _ := v.sceneCodeNode(); m != nil {
		if v.props.text.Text() != text {
			v.setPropsText(text, editor.Target{})
			v.props.text.SetSelection(at, length)
		}
		v.props.want = OwnText(v.fileText(v.ownFileOf(m)), m)
	}
}

func nodeStartingAt(n *scene.Node, start int) *scene.Node {
	for _, k := range sceneKids(n) {
		if k.Range.Start == start {
			return k
		}
		if k.Range.Start < start && start < k.Range.End {
			return nodeStartingAt(k, start)
		}
	}
	return nil
}
