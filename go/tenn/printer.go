// Port of TennPrinter.swift.
package tenn

import "strings"

const spaces = "    "

func quote(val string) string {
	return strings.ReplaceAll(val, `"`, `\"`)
}

func ChildsToStr(node *Node, ind int, clean bool) string {
	var sb strings.Builder
	sep := " "
	if node.Kind == KindBlockExpr || node.Kind == KindStatements {
		sep = "\n"
	}
	for i, c := range node.Children {
		sb.WriteString(ToStr(c, ind, clean))
		if i != len(node.Children)-1 {
			sb.WriteString(sep)
		}
	}
	return sb.String()
}

func ToStr(node *Node, indent int, clean bool) string {
	result := ""

	if node.Kind == KindCommand {
		result += strings.Repeat(spaces, indent)
	}
	if tok := node.Token; tok != nil {
		switch node.Kind {
		case KindCharLit, KindIntLit, KindIdent, KindFloatLit:
			result += tok.Literal
		case KindStringLit:
			if clean {
				result += tok.Literal
			} else {
				result += `"` + quote(tok.Literal) + `"`
			}
		case KindExpression:
			result += "$(" + tok.Literal + ")"
		case KindImage:
			result += "@(" + tok.Literal + ")"
		case KindExpressionBlock:
			result += "${" + tok.Literal + "}"
		case KindMarkdownLit:
			result += "%{" + tok.Literal + "}"
		}
	}
	ind := indent
	postfix := ""
	if node.Kind == KindBlockExpr {
		result += "{\n"
		if node.Count() > 0 {
			postfix = "\n" + strings.Repeat(spaces, indent) + "}"
		} else {
			postfix = strings.Repeat(spaces, indent) + "}"
		}
		ind++
	}
	return result + ChildsToStr(node, ind, clean) + postfix
}
