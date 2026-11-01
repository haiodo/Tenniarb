package exec

// SceneEval evaluates one expression of a scene (spec 6): no item scope, just the code, and the value as
// text. It is what scene.Options.Eval takes.
//
// shortcut: no round limit, so a runaway expression hangs the build; add an interrupt by
// Options.RoundLimit when scenes take files from outside the workspace.
func SceneEval(code string) (string, bool) {
	j := newJSContext()
	v, failed := j.evaluate(code)
	if failed {
		return "", false
	}
	return jsToString(v), true
}
