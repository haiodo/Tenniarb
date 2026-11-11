//go:build darwin || linux || windows

package app

import (
	"fmt"
	"slices"
	"strings"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/scene"
	"github.com/haiodo/tenniarb/go/tenn"
)

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
