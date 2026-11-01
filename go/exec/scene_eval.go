package exec

import "strconv"

// SceneEvalVars evaluates one expression of a scene (spec 6, 5c) with variables in scope: the parameters of
// a component instance, name to value as text. A value written as a number is a number, so ${size + 12}
// adds; anything else is a string. It is what scene.Options.EvalVars takes.
//
// shortcut: no round limit, so a runaway expression hangs the build; add an interrupt by
// Options.RoundLimit when scenes take files from outside the workspace.
func SceneEvalVars(code string, vars map[string]string) (string, bool) {
	j := newJSContext()
	for name, v := range vars {
		// "007" or "1.50" stay the text they are, or they would come back changed as 7 and 1.5.
		if f, err := strconv.ParseFloat(v, 64); err == nil && strconv.FormatFloat(f, 'f', -1, 64) == v {
			j.set(name, j.rt.ToValue(f))
			continue
		}
		j.set(name, j.rt.ToValue(v))
	}
	v, failed := j.evaluate(code)
	if failed {
		return "", false
	}
	return jsToString(v), true
}

// SceneEval is SceneEvalVars without variables; it is what scene.Options.Eval takes.
func SceneEval(code string) (string, bool) { return SceneEvalVars(code, nil) }
