// Package ref builds the same JSON document as web/packages/core/fuzz/main.swift.
package ref

import "github.com/haiodo/tenniarb/go/tenn"

func token(t tenn.Token) map[string]any {
	end := t.Pos + t.Size
	if t.Type == tenn.TokSemiColon && t.Literal == ";" {
		end = t.Pos // Swift adds ";" before consuming it
	}
	size := tenn.GraphemeCount(t.Literal)
	return map[string]any{
		"type": string(t.Type), "literal": t.Literal, "line": t.Line, "col": t.Col,
		"pos": end - size, "size": size,
	}
}

func node(n *tenn.Node) map[string]any {
	out := map[string]any{"kind": string(n.Kind)}
	if n.Token != nil {
		out["token"] = token(*n.Token)
	}
	if n.Children != nil {
		children := make([]any, 0, len(n.Children))
		for _, c := range n.Children {
			children = append(children, node(c))
		}
		out["children"] = children
	}
	return out
}

// Run lexes and parses src and returns tokens, tree, printed output and errors in Swift units.
func Run(src string) map[string]any {
	lexer := tenn.NewLexer(src)
	tokens := []any{}
	for t := lexer.GetToken(); t != nil; t = lexer.GetToken() {
		tokens = append(tokens, token(*t))
		if t.Type == tenn.TokEOF || len(tokens) > 100000 {
			break
		}
	}
	parser := tenn.NewParser(nil)
	tree := parser.Parse(src)
	errs := []any{}
	for _, e := range parser.Errors.Errors {
		errs = append(errs, map[string]any{
			"code": string(e.ErrorCode), "message": e.Message, "line": e.Line, "col": e.Col,
		})
	}
	return map[string]any{
		"tokens": tokens, "tree": node(tree),
		"printed":      tenn.ToStr(tree, 0, false),
		"printedClean": tenn.ToStr(tree, 0, true),
		"errors":       errs,
	}
}
