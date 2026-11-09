//go:build darwin || linux || windows

package app

import (
	"fmt"
	"slices"

	g "github.com/haiodo/gowt"

	"github.com/haiodo/tenniarb/go/editor"
)

// A shadow layer is a `shadow x y blur color` command; a node may write several, which is how a soft
// shadow is built. The list below edits them one by one for a classic item, and the item defaults of
// the element the same way.

// ownShadows are the shadow commands the selection writes itself.
func (v *Viewer) ownShadows() []editor.Prop { return v.session.Props("shadow") }

// itemDefaultShadows are the shadows of the styles { item } block of the element.
func (v *Viewer) itemDefaultShadows() []editor.Prop {
	return v.session.StyleProps("item", "shadow")
}

// shownShadows are the shadows the selection is drawn with: its own, else the item defaults it takes
// from the element.
func (v *Viewer) shownShadows() []editor.Prop {
	if own := v.ownShadows(); len(own) > 0 {
		return own
	}
	return v.itemDefaultShadows()
}

// shadowsInherited says the list shows the item defaults, not commands of the item itself.
func (v *Viewer) shadowsInherited() bool {
	return len(v.ownShadows()) == 0 && len(v.itemDefaultShadows()) > 0
}

// setShadows writes the whole list as the item's own shadows, so a layer taken from the item defaults
// becomes an override.
func (v *Viewer) setShadows(layers []editor.Prop) bool {
	return v.session.SetProps("shadow", shadowLists(layers))
}

// setItemDefaultShadows writes the whole list into styles { item } of the element.
func (v *Viewer) setItemDefaultShadows(layers []editor.Prop) bool {
	return v.session.SetStyleProps("item", "shadow", shadowLists(layers))
}

// shadowLists are the layers as the argument lists a write takes.
func shadowLists(layers []editor.Prop) [][]string {
	out := make([][]string, len(layers))
	for i, p := range layers {
		out[i] = make([]string, len(p.Args))
		for j, a := range p.Args {
			out[i][j] = a.Text
		}
	}
	return out
}

// shadowKey names what the rows of the list are built from: the layers and where they come from.
func shadowKey(layers []editor.Prop, inherited bool) string {
	return fmt.Sprint(shadowLists(layers)) + fmt.Sprint(inherited)
}

// shadowList is the shadow editor: a row of fields per layer, the ones the selection writes, or the
// item defaults it inherits, with buttons to add a layer and to drop one. A change writes the whole
// list as one undo step.
func (in *inspector) shadowList(when func(selKind) bool, id string, layers func() []editor.Prop,
	inherited func() bool, write func([]editor.Prop) bool) func() g.Option {
	var body *g.Panel
	key := ""
	var refresh []func()
	inh := func() bool { return inherited != nil && inherited() }
	f := &inspField{when: when, refresh: func(selKind) {
		if k := shadowKey(layers(), inh()); k != key {
			key = k
			refresh = nil
			body.Clear()
			rows, refs := in.shadowRows(id, layers, inh, write)
			body.Add(rows...)
			refresh = append(refresh, refs...)
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

// shadowRows builds the rows of the list: X, Y and blur of a layer, its colour beside a button that
// drops it, and a button that adds one.
func (in *inspector) shadowRows(id string, layers func() []editor.Prop, inherited func() bool,
	write func([]editor.Prop) bool) ([]g.Option, []func()) {
	var rows []g.Option
	var refresh []func()
	if inherited() {
		rows = append(rows, styleRow("", g.Label("from the element's item styles", faint(), g.Wrap(), g.GrowX())))
	}
	all := layers()
	for i := range all {
		at := i
		caption := "Shadow"
		if i > 0 {
			caption = ""
		}
		src := func() propSrc { return shadowLayerSrc(layers, write, at) }
		prefix := fmt.Sprintf("%s-%d", id, at)
		xy, xyRefresh := in.pairRow(caption, "X", "Y",
			entryCfg{id: prefix + "-x", name: "shadow", idx: 0, numeric: true, src: src},
			entryCfg{id: prefix + "-y", name: "shadow", idx: 1, numeric: true, src: src})
		blur, blurRefresh := in.textRow("", entryCfg{id: prefix + "-blur", name: "shadow", idx: 2, numeric: true, src: src})
		color, colorRefresh := in.colorCtl(entryCfg{id: prefix + "-color", name: "shadow", idx: 3, src: src})
		remove := g.Button("−", g.Small(), g.Size(g.Scaled(24), 0), g.ID(prefix+"-remove"),
			g.Init(func(b *g.ButtonView) {
				b.SetTooltip("Remove this shadow")
				b.OnClick(func() {
					if in.loading {
						return
					}
					list := slices.Clone(layers())
					write(slices.Delete(list, at, at+1))
				})
			}))
		rows = append(rows,
			styleRow("", xy, blur),
			styleRow("", append(color, remove)...))
		refresh = append(refresh, xyRefresh, blurRefresh, func() { colorRefresh(kindNone) })
	}
	add := g.Button("+ Shadow", g.Small(), g.GrowX(), g.ID(id+"-add"), g.Init(func(b *g.ButtonView) {
		b.OnClick(func() {
			if in.loading {
				return
			}
			list := slices.Clone(layers())
			list = append(list, editor.Prop{Set: true, Args: []editor.Arg{{Text: "0"}, {Text: "-5"}, {Text: "5"}}})
			write(list)
		})
	}))
	rows = append(rows, styleRow("", add))
	return rows, refresh
}

// shadowLayerSrc reads and writes one layer: an argument of it is changed in the whole list, which is
// written back as the item's own shadows.
func shadowLayerSrc(layers func() []editor.Prop, write func([]editor.Prop) bool, at int) propSrc {
	update := func(change func(editor.Prop) editor.Prop) bool {
		all := layers()
		if at >= len(all) {
			return false
		}
		out := make([]editor.Prop, len(all))
		for i, p := range all {
			out[i] = editor.Prop{Set: p.Set, Args: slices.Clone(p.Args)}
		}
		out[at] = change(out[at])
		return write(out)
	}
	return propSrc{
		get: func(string) editor.Prop {
			if all := layers(); at < len(all) {
				return all[at]
			}
			return editor.Prop{}
		},
		set: func(_ string, args ...string) bool {
			update(func(editor.Prop) editor.Prop {
				return editor.Prop{Set: true, Args: argList(args)}
			})
			return true
		},
		arg: func(_ string, i int, text string) bool {
			return update(func(p editor.Prop) editor.Prop {
				for len(p.Args) <= i {
					p.Args = append(p.Args, editor.Arg{Text: "0"})
				}
				p.Args[i] = editor.Arg{Text: text}
				return p
			})
		},
		eff: func(string) string { return "" },
	}
}

func argList(texts []string) []editor.Arg {
	out := make([]editor.Arg, len(texts))
	for i, t := range texts {
		out[i] = editor.Arg{Text: t}
	}
	return out
}
