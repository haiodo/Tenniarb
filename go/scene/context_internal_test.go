package scene

import (
	"testing"

	"github.com/haiodo/tenn/go"
)

// TestUsesCtx: a property names ctx only as a bare name. A classic variable $ctx or a member a.ctx is not
// the context object of the scene, so the element model still gives those.
func TestUsesCtx(t *testing.T) {
	arg := func(kind tenn.NodeKind, code string) *Prop {
		return &Prop{Name: "title", Args: []*tenn.Node{{Kind: kind, Token: &tenn.Token{Literal: code}}}}
	}
	for _, c := range []struct {
		prop *Prop
		want bool
	}{
		{arg(tenn.KindExpressionBlock, "ctx.tone"), true},
		{arg(tenn.KindExpression, `ctx["tone"]`), true},
		{arg(tenn.KindStringLit, `the ctx is ${ctx.tone}`), true},
		{arg(tenn.KindExpressionBlock, "$ctx"), false},
		{arg(tenn.KindExpressionBlock, "box.ctx"), false},
		{arg(tenn.KindExpressionBlock, "myctx"), false},
		{arg(tenn.KindExpressionBlock, "size + 2"), false},
		{arg(tenn.KindStringLit, "the ctx is a word"), false},
		{arg(tenn.KindStringLit, "${size}"), false},
		{arg(tenn.KindIdent, "ctx"), false},
	} {
		if got := usesCtx(c.prop); got != c.want {
			code, _ := c.prop.Args[0].GetIdentText()
			t.Errorf("usesCtx(%s %q) = %v, want %v", c.prop.Args[0].Kind, code, got, c.want)
		}
	}
}
