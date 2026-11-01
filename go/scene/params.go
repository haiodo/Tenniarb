package scene

import (
	"strings"

	"github.com/haiodo/tenniarb/go/tenn"
)

// evalFunc evaluates the code of an expression (spec 6) with variables in scope: the parameters of the
// component instance the node belongs to, by name and as text (spec 5c).
type evalFunc func(code string, vars map[string]string) (string, bool)

// evaluator is Options.EvalVars or Options.Eval as one function; nil when there is neither. EvalVars goes
// first: only it can see the parameters of an instance, and Eval is the same without variables.
func evaluator(o Options) evalFunc {
	switch {
	case o.EvalVars != nil:
		return func(code string, vars map[string]string) (string, bool) {
			if len(vars) == 0 && o.Eval != nil {
				return o.Eval(code)
			}
			return o.EvalVars(code, vars)
		}
	case o.Eval != nil:
		return func(code string, _ map[string]string) (string, bool) { return o.Eval(code) }
	}
	return nil
}

// evalArg is the text of an argument: as written, or the value of an expression; a string holding ${...} is
// a template, run as one. ok is false when an expression failed, or when there is no evaluator.
func evalArg(eval evalFunc, a *tenn.Node, vars map[string]string) (string, bool) {
	code, _ := a.GetIdentText()
	switch {
	case a.Kind == tenn.KindExpression || a.Kind == tenn.KindExpressionBlock:
	case a.Kind == tenn.KindStringLit && strings.Contains(code, "${"):
		code = "`" + code + "`" // a template string, as the classic engine runs it
	default:
		return code, true
	}
	if eval == nil {
		return "", false
	}
	return eval(code, vars)
}

// params are the parameters of instance target in use: a value the use sets by name, evaluated in the
// scope of the use, over the default from the declaration, evaluated in the scope of the definition
// (spec 5c). used holds the overrides that are parameter values, so they do not become properties of the
// instance root. A parameter without a default that the use does not set is an error.
func (x *expander) params(target, use *Node, caller map[string]string) (values map[string]string, used map[*Override]bool) {
	if len(target.Params) == 0 {
		return nil, nil
	}
	used = map[*Override]bool{}
	given := map[string]*Prop{}
	for _, ov := range use.Block {
		if ov.Kind == OvProp && target.Param(ov.Prop.Name) != nil {
			given[ov.Prop.Name] = ov.Prop
			used[ov] = true
		}
	}
	values = make(map[string]string, len(target.Params))
	var missing []string
	for _, prm := range target.Params {
		if pr := given[prm.Name]; pr != nil {
			values[prm.Name] = x.value(pr, caller)
		} else if prm.Default != nil {
			values[prm.Name] = x.value(prm.Default, nil)
		} else {
			missing = append(missing, prm.Name)
		}
	}
	if len(missing) > 0 {
		x.t.fail(use.Pos, "element %q needs %s: set it in the block of the use", target.Name, quoteAll(missing))
	}
	return values, used
}

// defaults are the parameter values of an element expanded on its own, not through a use: a component
// shown as a screen looks like an instance that sets nothing.
func (x *expander) defaults(n *Node) map[string]string {
	if len(n.Params) == 0 {
		return nil
	}
	values := make(map[string]string, len(n.Params))
	for _, prm := range n.Params {
		if prm.Default != nil {
			values[prm.Name] = x.value(prm.Default, nil)
			continue
		}
		values[prm.Name] = "" // a required parameter has no value on its own, and no warning either
	}
	return values
}

// value is the value of one parameter: its arguments as words, each an expression evaluated in vars. A
// list of styles is written as several words: "extras outlined rounded" (spec 5b, any-of).
func (x *expander) value(pr *Prop, vars map[string]string) string {
	words := make([]string, 0, len(pr.Args))
	for _, a := range pr.Args {
		v, ok := evalArg(x.eval, a, vars)
		if !ok {
			if x.eval != nil {
				x.t.warn(pr.Pos, "%s: the expression is not evaluated", pr.Name)
			}
			return strings.Join(words, " ")
		}
		words = append(words, v)
	}
	return strings.Join(words, " ")
}

func quoteAll(names []string) string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = `"` + n + `"`
	}
	return strings.Join(out, ", ")
}
