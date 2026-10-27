//go:build darwin || linux || windows

package app

import (
	"strconv"
	"strings"

	g "github.com/haiodo/gowt"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/scene"
	"github.com/haiodo/tenniarb/go/tenn"
)

// sceneSrc reads the property a scene node writes itself (an override, for a use) and writes it as a
// text edit; eff is the value in effect, the component's or the default, shown as the hint.
func (v *Viewer) sceneSrc() propSrc {
	own := func(name string) *scene.Prop {
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
		n, _ := v.selectedScene()
		if n == nil {
			return false
		}
		e, ok := SetPropEdit(v.fileText(v.fileOf(n)), n, name, args...)
		return ok && v.sceneEdit(n, e)
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
	node := func(k selKind) bool {
		n, _ := v.selectedScene()
		return k == kindScene && n != nil
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
			if len(sceneKids(n)) > 0 {
				return "Frame"
			}
		}
		return "Item"
	}
	title := func(selKind) string {
		if n, _ := v.selectedScene(); n != nil {
			return kindText(n)
		}
		return "Screen"
	}
	nameOf := func(selKind) string {
		if n, _ := v.selectedScene(); n != nil {
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
		n, _ := v.selectedScene()
		return k == kindScene && n != nil && n.Kind == scene.KUse
	}
	return g.VBox(g.GrowX(), g.Padding(0), g.Gap(g.Scaled(10)),
		in.section(title, scr,
			in.info("Name", scr, nameOf),
			in.info("Component", isUse, func(selKind) string {
				n, _ := v.selectedScene()
				return n.Path
			}),
			in.info("Size", scr, size)),
		in.section(sectionTitle("Layout"), node,
			in.combo("Auto layout", node, "scene-auto-layout", "auto-layout", []string{"none", "row", "column", "abs"}, "none", src),
			in.pair("Position", "X", "Y", node, num("scene-pos-x", "pos", 0), num("scene-pos-y", "pos", 1)),
			in.pair("Size", "W", "H", node, t("scene-width", "width"), t("scene-height", "height")),
			in.text("Gap", node, num("scene-gap", "gap", 0)),
			in.text("Padding", node, t("scene-padding", "padding")),
			in.combo("Align", node, "scene-align", "align", []string{"start", "center", "end"}, "start", src),
			in.combo("Justify", node, "scene-justify", "justify", []string{"start", "center", "end", "space-between"}, "start", src),
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
			in.toggle("Hidden", node, "scene-hidden", "hidden", []string{"yes"}, src)))
}

// sceneCodeKey names what the Code section shows for a scene element: the element and the node.
func (v *Viewer) sceneCodeKey() string {
	if len(v.selNodes) == 0 {
		return ""
	}
	return pathKey(v.state.Current) + "\x01" + v.selNodes[0]
}

// refreshSceneCode shows the source of the selected node in the Code section, its kids as one line each;
// nothing selected, nothing shown: the text of a whole element can be the whole file.
func (v *Viewer) refreshSceneCode() {
	n, _ := v.selectedScene()
	key := v.sceneCodeKey()
	want := ""
	if n != nil {
		want = OwnText(v.fileText(v.fileOf(n)), n)
	}
	if key == v.props.sceneKey && v.props.text.Text() == want {
		return
	}
	if key == v.props.sceneKey && v.propsFocused() {
		if _, p := tenn.Parse(v.props.text.Text()); p.Errors.HasErrors() {
			return // being typed
		}
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
	n, _ := v.selectedScene()
	src := v.fileText(v.fileOf(n))
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
	if !v.sceneEdit(n, e) {
		return
	}
	// A renamed node keeps the selection: it starts where the old one did.
	if NodeAt(v.sv.node, v.selNodes[0]) == nil {
		if m := nodeStartingAt(v.sv.node, start); m != nil {
			v.selNodes = []string{NodePath(v.sv.node, m)}
			v.props.sceneKey = v.sceneCodeKey()
			v.drawSceneOverlay()
		}
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
