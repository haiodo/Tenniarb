package exec

import (
	"strconv"
	"strings"
)

// SceneEvalVars evaluates one expression of a scene (spec 6, 5c) with variables in scope: the parameters of
// a component instance and the context as ctx.* keys (spec 5f), name to value as text. A value written as a
// number is a number, so ${size + 12} adds and ${ctx.radius * 2} too; anything else is a string. The ctx.*
// keys become one object named ctx. It is what scene.Options.EvalVars takes.
//
// shortcut: no round limit, so a runaway expression hangs the build; add an interrupt by
// Options.RoundLimit when scenes take files from outside the workspace.
func SceneEvalVars(code string, vars map[string]string) (string, bool) {
	j := newJSContext()
	ctx := map[string]any{}
	for name, v := range vars {
		if key, ok := strings.CutPrefix(name, "ctx."); ok {
			ctx[key] = sceneValue(v)
			continue
		}
		j.set(name, j.rt.ToValue(sceneValue(v)))
	}
	if len(ctx) > 0 {
		j.set("ctx", j.rt.ToValue(ctx))
	}
	v, failed := j.evaluate(code)
	if failed {
		return "", false
	}
	return jsToString(v), true
}

// sceneValue is v as a JS value: a number when it is written as one, else the text.
func sceneValue(v string) any {
	// "007" or "1.50" stay the text they are, or they would come back changed as 7 and 1.5.
	if f, err := strconv.ParseFloat(v, 64); err == nil && strconv.FormatFloat(f, 'f', -1, 64) == v {
		return f
	}
	return v
}

// SceneEval is SceneEvalVars without variables; it is what scene.Options.Eval takes.
func SceneEval(code string) (string, bool) { return SceneEvalVars(code, nil) }
