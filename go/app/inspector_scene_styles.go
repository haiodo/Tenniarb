//go:build darwin || linux || windows

package app

import (
	"fmt"
	"slices"

	"github.com/haiodo/tenniarb/go/scene"
)

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
