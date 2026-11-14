package scene

import (
	"errors"
	"fmt"
	"strings"

	"github.com/dop251/goja"
	"github.com/haiodo/tenn/go"
)

// checkExpressions reports a ${...} that is not JavaScript: check and validate do not evaluate
// expressions, so a broken one would otherwise show only as an element that draws nothing.
func (l *loader) checkExpressions(nodes []*tenn.Node) {
	for _, n := range nodes {
		if n.Kind == tenn.KindCommand && cmdName(n) == "template" {
			continue // the text a component generates is code of its own, not an expression here
		}
		if msg := exprSyntaxError(n); msg != "" {
			l.t.fail(l.pos(n), "%s", msg)
		}
		l.checkExpressions(n.Children)
	}
}

// exprSyntaxError compiles an expression argument the way evalArg runs it; "" when it parses.
func exprSyntaxError(n *tenn.Node) string {
	var code string
	switch n.Kind {
	case tenn.KindExpression, tenn.KindExpressionBlock:
		code, _ = n.GetIdentText()
	case tenn.KindStringLit:
		s, _ := n.GetIdentText()
		if !strings.Contains(s, "${") {
			return ""
		}
		code = "`" + s + "`" // a string with ${...} runs as a template
	default:
		return ""
	}
	if _, err := goja.Compile("expr", code, false); err != nil {
		var se *goja.CompilerSyntaxError
		if errors.As(err, &se) {
			return fmt.Sprintf("expression %q: %s", code, se.Message)
		}
		return fmt.Sprintf("expression %q: %v", code, err)
	}
	return ""
}
