package scene_test

import (
	"strings"
	"testing"
)

// TestExpressionSyntax: a ${...} that is not JavaScript is an error at Load, where check and validate
// see it, not an element that quietly draws nothing.
func TestExpressionSyntax(t *testing.T) {
	_, err := tenner(t, "expr.tenn", `element "Screen" {
  item Box { width 40; height 20; color ${1 + } }
}`)
	if err == nil || !strings.Contains(err.Error(), "Unexpected end of input") {
		t.Fatalf("error %v, want the broken expression reported", err)
	}
}

// TestExpressionSyntaxTemplate: a string with ${...} runs as a template, so its placeholders are checked too.
func TestExpressionSyntaxTemplate(t *testing.T) {
	_, err := tenner(t, "expr.tenn", `element "Screen" {
  item Box { width 40; height 20; title "count ${1 +}" }
}`)
	if err == nil || !strings.Contains(err.Error(), "expression") {
		t.Fatalf("error %v, want the broken template reported", err)
	}
}

// TestExpressionSyntaxSkipsTemplate: the text of a template block is code a component generates, not an
// expression the scene runs.
func TestExpressionSyntaxSkipsTemplate(t *testing.T) {
	paramsTree(t, "expr.tenn", `element "Card" {
  kind Component
  template "${#each items}"
  item Box { width 40; height 20 }
}`)
}

// TestExpressionSyntaxRuntime: a valid expression naming a missing key is a runtime problem, not a
// syntax error: Load stays quiet.
func TestExpressionSyntaxRuntime(t *testing.T) {
	if _, err := tenner(t, "expr.tenn", `element "Screen" {
  item Box { width 40; height 20; color ${ctx.nope} }
}`); err != nil {
		t.Fatalf("Load: %v, want no error for a valid expression", err)
	}
}
