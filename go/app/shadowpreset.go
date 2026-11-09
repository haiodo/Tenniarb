//go:build darwin || linux || windows

package app

import (
	"slices"

	"github.com/haiodo/tenniarb/go/editor"
	"github.com/haiodo/tenniarb/go/scene"
)

// shadowPresets are the shadows the inspector writes, by the name its row shows: a tight contact layer
// and a wide faint one, which is what reads as a soft shadow on a card or a plate.
var shadowPresets = map[string][][]string{
	"soft":     {{"0", "-4", "12", "#00000014"}, {"0", "-14", "32", "#00000010"}},
	"floating": {{"0", "-12", "20", "#00000012"}, {"0", "-32", "64", "#0000000f"}},
}

// customShadow is the row's state for layers written by hand: the presets cannot name them, and
// picking it changes nothing.
const customShadow = "custom"

// shadowSrc is the Shadow row of a scene node: it reads the preset the node was given and writes the
// layers of the chosen one as one undo step.
func (v *Viewer) shadowSrc() propSrc {
	return propSrc{
		get: func(string) editor.Prop {
			n, _ := v.selectedScene()
			if n == nil {
				return editor.Prop{}
			}
			sh := shadowsOf(n)
			if len(sh) == 0 {
				return editor.Prop{}
			}
			for name, layers := range shadowPresets {
				if sameShadows(sh, layers) {
					return editor.Prop{Set: true, Args: []editor.Arg{{Text: name}}}
				}
			}
			return editor.Prop{Set: true, Args: []editor.Arg{{Text: customShadow}}}
		},
		set: func(_ string, args ...string) bool {
			if len(args) == 0 {
				return v.setShadowPreset("")
			}
			if args[0] == customShadow { // a state, not a preset: the layers the node already has stay
				return true
			}
			return v.setShadowPreset(args[0])
		},
		// The row shows what the node writes, not what the renderer would use.
		eff: func(string) string { return "" },
	}
}

// sameShadows reports whether the commands of the node are the layers of a preset, as written.
func sameShadows(sh []*scene.Prop, layers [][]string) bool {
	if len(sh) != len(layers) {
		return false
	}
	for i, p := range sh {
		if !slices.Equal(p.Values(), layers[i]) {
			return false
		}
	}
	return true
}

// setShadowPreset writes the layers of a preset on the selected node; an empty name removes every shadow.
func (v *Viewer) setShadowPreset(preset string) bool {
	var cmds []string
	for _, args := range shadowPresets[preset] {
		cmds = append(cmds, CommandText("shadow", args...))
	}
	return v.setSceneShadowCmds(cmds...)
}

// sceneShadows are the layers the selected node writes itself, read the way the list edits them.
func (v *Viewer) sceneShadows() []editor.Prop {
	n, _ := v.selectedScene()
	if n == nil {
		return nil
	}
	sh := shadowsOf(n)
	out := make([]editor.Prop, len(sh))
	for i, p := range sh {
		out[i] = editor.Prop{Set: true, Args: argList(p.Values())}
	}
	return out
}

// setSceneShadows writes the list as the node's own shadows: one undo step, and the file holds exactly
// these layers.
func (v *Viewer) setSceneShadows(layers []editor.Prop) bool {
	var cmds []string
	for _, args := range shadowLists(layers) {
		cmds = append(cmds, CommandText("shadow", args...))
	}
	return v.setSceneShadowCmds(cmds...)
}

// setSceneShadowCmds replaces every shadow of the selected node with cmds as one undo step; none removes
// them all.
func (v *Viewer) setSceneShadowCmds(cmds ...string) bool {
	n, _ := v.selectedScene()
	if n == nil || v.session == nil {
		return false
	}
	file := v.ownFileOf(n)
	edits := SetShadowsEdit(v.fileText(file), n, cmds...)
	if len(edits) == 0 {
		return false
	}
	ok := true
	v.session.Batch(func() {
		for _, e := range edits {
			if err := v.applyFileText(file, e.Start, e.End, e.New); err != nil {
				v.setBanner(err.Error())
				ok = false
				return
			}
		}
	})
	return ok
}
