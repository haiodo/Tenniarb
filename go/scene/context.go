package scene

import (
	"regexp"
	"strings"

	"github.com/haiodo/tenniarb/go/tenn"
)

// ctxDecl is one "context <key> <value>" or "context <key> ?= <value>" (spec 5f).
type ctxDecl struct {
	key, val string
	soft     bool // ?=: only fills a key no layer above has
}

// scope is the variables of an expression: parameters by name and context keys with the ctx. prefix
// (spec 5c, 5f). Without context the map is the one of the parameters, unchanged.
func (x *expander) scope() map[string]string { return x.scopeOf(x.vars, x.env) }

func (x *expander) scopeOf(vars, env map[string]string) map[string]string {
	if len(env) == 0 {
		return vars
	}
	out := make(map[string]string, len(vars)+len(env))
	for k, v := range vars {
		out[k] = v
	}
	for k, v := range env {
		out["ctx."+k] = v
	}
	return out
}

// context is env with the context commands of a node's own properties applied, their values evaluated
// in the parameters and the inherited context.
func (x *expander) context(props []*Prop, env map[string]string) map[string]string {
	if env == nil {
		env = x.env
	}
	return applyCtx(env, x.ctxDecls(props, x.scopeOf(x.vars, env)))
}

// ctxDecls reads the context commands of props in file order; the value is the rest of the arguments,
// expressions evaluated in scope.
func (x *expander) ctxDecls(props []*Prop, scope map[string]string) []ctxDecl {
	var out []ctxDecl
	for _, pr := range props {
		if pr.Name != "context" {
			continue
		}
		if len(pr.Args) < 2 {
			x.t.warn(pr.Pos, "context is written: context <key> [?=] <value>")
			continue
		}
		d := ctxDecl{key: argText(pr.Args, 0)}
		rest := pr.Args[1:]
		if s, _ := pr.Args[1].GetIdentText(); s == "?=" {
			d.soft, rest = true, pr.Args[2:]
		}
		if len(rest) == 0 {
			x.t.warn(pr.Pos, "context %s needs a value", d.key)
			continue
		}
		d.val = x.value(&Prop{Name: "context", Args: rest, Pos: pr.Pos}, scope)
		out = append(out, d)
	}
	return out
}

// applyCtx is env with the declarations applied; a soft one fills only an absent key.
func applyCtx(env map[string]string, decls []ctxDecl) map[string]string {
	if len(decls) == 0 {
		return env
	}
	out := make(map[string]string, len(env)+len(decls))
	for k, v := range env {
		out[k] = v
	}
	for _, d := range decls {
		if d.soft {
			if _, ok := out[d.key]; ok {
				continue
			}
		}
		out[d.key] = d.val
	}
	return out
}

// blockProps is the properties of a use block in file order; a node of the block is no declaration.
func blockProps(block []*Override) []*Prop {
	var out []*Prop
	for _, ov := range block {
		if ov.Kind == OvProp {
			out = append(out, ov.Prop)
		}
	}
	return out
}

// ctxEnv is the context part of a scope, the keys with the ctx. prefix.
func ctxEnv(vars map[string]string) map[string]string {
	var out map[string]string
	for k, v := range vars {
		if key, ok := strings.CutPrefix(k, "ctx."); ok {
			if out == nil {
				out = map[string]string{}
			}
			out[key] = v
		}
	}
	return out
}

// fileRoot is the root of the file n is written in: the node above every element of it (spec 5f).
func fileRoot(n *Node) *Node {
	for n.Parent != nil {
		n = n.Parent
	}
	return n
}

// fileDefaults is the `?=` context of the file a definition is written in, evaluated in the place of the
// use: a default travels with the component, a hard declaration binds its own file (spec 5f).
func (x *expander) fileDefaults(target *Node, inherited map[string]string) []ctxDecl {
	file := fileRoot(target)
	if file == target {
		return nil
	}
	var out []ctxDecl
	for _, d := range x.ctxDecls(file.Props, x.scopeOf(nil, inherited)) {
		if d.soft {
			out = append(out, d)
		}
	}
	return out
}

// ctxName is the object an expression reads context through (spec 5f): a bare name, not `$ctx` or `a.ctx`.
var ctxName = regexp.MustCompile(`(^|[^.\w$])ctx\b`)

// usesCtx reports whether pr names ctx in an expression. The element model of a classic diagram has no ctx,
// so such a property is the scene's to evaluate (spec 5f, 6).
//
// shortcut: the code is scanned as text, so a property that mentions ctx inside a string or mixes it with
// the model scope (sum, items) is left unevaluated; give the model the context when such a file turns up.
func usesCtx(pr *Prop) bool {
	for _, a := range pr.Args {
		if a.Kind != tenn.KindExpression && a.Kind != tenn.KindExpressionBlock && a.Kind != tenn.KindStringLit {
			continue
		}
		code, _ := a.GetIdentText()
		if a.Kind == tenn.KindStringLit && !strings.Contains(code, "${") {
			continue
		}
		if ctxName.MatchString(code) {
			return true
		}
	}
	return false
}
