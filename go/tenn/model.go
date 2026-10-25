// Package tenn is a port of the Tenn format core (lexer, parser, node model, printer)
// from web/packages/core, which in turn is a 1:1 port of the Swift original.
//
// Unlike the TypeScript port (UTF-16 code units) all positions here are UTF-8 bytes, because
// Go strings are UTF-8. Swift reports col in UTF-8 bytes too, so Token.Col needs no conversion.
package tenn

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// TokenType values are the Swift case names.
type TokenType string

const (
	TokInvalid         TokenType = "invalid"
	TokEOF             TokenType = "eof"
	TokSymbol          TokenType = "symbol"
	TokFloatLit        TokenType = "floatLit"
	TokIntLit          TokenType = "intLit"
	TokStringLit       TokenType = "stringLit"
	TokCurlyLe         TokenType = "curlyLe"
	TokCurlyRi         TokenType = "curlyRi"
	TokComma           TokenType = "comma"
	TokColon           TokenType = "colon"
	TokSemiColon       TokenType = "semiColon"
	TokExpression      TokenType = "expression"
	TokExpressionBlock TokenType = "expressionBlock"
	TokMarkdownLit     TokenType = "markdownLit"
	TokImageData       TokenType = "imageData"
)

// Token positions are UTF-8 bytes of the source:
//
//	Pos  - start of the token's source span; for string/expression/markdown/image literals the span
//	       is the content between the delimiters, for eof it is empty at the end of input
//	Size - length of that span (differs from len(Literal) for strings with escapes)
//	Col  - bytes since the line counter was last reset, read after the token (as in Swift)
//
// The golden test converts Size and Pos to the values the Swift lexer reports.
type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Col     int
	Pos     int
	Size    int
}

// EndPos is the byte offset just after the token, closing delimiter of a literal included.
func (t *Token) EndPos() int {
	switch t.Type {
	case TokStringLit, TokExpression, TokExpressionBlock, TokMarkdownLit, TokImageData:
		return t.Pos + t.Size + 1
	}
	return t.Pos + t.Size
}

func NewToken(typ TokenType, literal string) *Token {
	return &Token{Type: typ, Literal: literal}
}

type LexerError string

const (
	ErrEndOfLineReadString    LexerError = "EndOfLineReadString"
	ErrEndOfExpressionReadErr LexerError = "EndOfExpressionReadError"
	ErrUTF8                   LexerError = "UTF8Error"
)

// LexerProtocol is what the parser needs from a lexer. GetToken returns nil at the end of input.
type LexerProtocol interface {
	GetToken() *Token
	Revert(tok *Token)
	SetErrorHandler(handler func(err LexerError, startPos, pos int))
}

// NodeKind values are the Swift case names.
type NodeKind string

const (
	KindEmpty           NodeKind = "Empty"
	KindIdent           NodeKind = "Ident"
	KindCharLit         NodeKind = "CharLit"
	KindIntLit          NodeKind = "IntLit"
	KindFloatLit        NodeKind = "FloatLit"
	KindStringLit       NodeKind = "StringLit"
	KindMarkdownLit     NodeKind = "MarkdownLit"
	KindCommand         NodeKind = "Command"
	KindStatements      NodeKind = "Statements"
	KindBlockExpr       NodeKind = "BlockExpr"
	KindExpression      NodeKind = "Expression"
	KindExpressionBlock NodeKind = "ExpressionBlock"
	KindImage           NodeKind = "Image"
)

var (
	intRe   = regexp.MustCompile(`^[+-]?\d+$`)
	floatRe = regexp.MustCompile(`^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$`)
)

// isSwiftWhitespace is CharacterSet.whitespacesAndNewlines, enumerated from Foundation.
func isSwiftWhitespace(r rune) bool {
	switch {
	case r >= 0x09 && r <= 0x0d, r == 0x20, r == 0x85, r == 0xa0, r == 0x1680:
		return true
	case r >= 0x2000 && r <= 0x200b, r == 0x2028, r == 0x2029, r == 0x202f, r == 0x205f, r == 0x3000:
		return true
	}
	return false
}

// Node.Children == nil means "no children"; an empty non-nil slice is possible after RemoveNamed.
type Node struct {
	Kind     NodeKind
	Token    *Token
	Children []*Node
	named    map[string]*Node
	// End is the byte offset just after the last token of a Command; Token.Pos is its start.
	End int
}

func NewNode(kind NodeKind, tok *Token) *Node {
	return &Node{Kind: kind, Token: tok}
}

func (n *Node) Count() int {
	return len(n.Children)
}

func (n *Node) Clone() *Node {
	result := NewNode(n.Kind, n.Token)
	result.End = n.End
	if n.Children != nil {
		result.Children = []*Node{}
		for _, c := range n.Children {
			result.Add(c.Clone())
		}
	}
	return result
}

func (n *Node) Replace(node *Node) {
	n.Children = nil
	for _, c := range node.Children {
		n.Add(c.Clone())
	}
}

// Traverse walks depth-first and stops as soon as the visitor returns false.
func (n *Node) Traverse(visitor func(*Node) bool) bool {
	if !visitor(n) {
		return false
	}
	for _, c := range n.Children {
		if !c.Traverse(visitor) {
			return false
		}
	}
	return true
}

func (n *Node) Add(nodes ...*Node) {
	for _, node := range nodes {
		if n.Children == nil {
			n.Children = []*Node{}
		}
		n.Children = append(n.Children, node)

		if n.Kind != KindBlockExpr {
			continue
		}
		if n.named == nil {
			n.named = map[string]*Node{}
		}
		if name, ok := node.GetIdent(0); ok {
			n.named[name] = node
		}
	}
}

func (n *Node) GetNamedElement(name string) *Node {
	if n.Kind != KindBlockExpr {
		return nil
	}
	return n.named[name]
}

// Swift removes from a copy of `named`, so the lookup table keeps the entry. Kept as is.
func (n *Node) RemoveNamed(name string) bool {
	if n.Children == nil {
		return false
	}
	oldSize := len(n.Children)
	kept := []*Node{}
	for _, itm := range n.Children {
		if ident, _ := itm.GetIdent(0); ident != name {
			kept = append(kept, itm)
		}
	}
	n.Children = kept
	return oldSize != len(kept)
}

func (n *Node) GetBlock(index int) []*Node {
	if c := n.GetChild(index); c != nil {
		return c.Children
	}
	return nil
}

func (n *Node) GetIdentText() (string, bool) {
	switch n.Kind {
	case KindIdent, KindStringLit, KindIntLit, KindFloatLit, KindCharLit, KindExpressionBlock,
		KindExpression, KindMarkdownLit, KindImage:
		if n.Token != nil {
			return n.Token.Literal, true
		}
	}
	return "", false
}

func (n *Node) GetIdent(path ...int) (string, bool) {
	if c := n.GetChild(path...); c != nil {
		return c.GetIdentText()
	}
	return "", false
}

// GetInt differs from TS (Number) on values outside int range: Swift Int(String) gives nil, so does this.
func (n *Node) GetInt(path ...int) (int, bool) {
	val, ok := n.GetIdent(path...)
	if !ok || !intRe.MatchString(val) {
		return 0, false
	}
	v, err := strconv.Atoi(val)
	return v, err == nil
}

// GetFloat rounds to float32 like Swift Float(String); the result is kept in a float64.
func (n *Node) GetFloat(path ...int) (float64, bool) {
	val, ok := n.GetIdent(path...)
	if !ok || !floatRe.MatchString(val) {
		return 0, false
	}
	v, _ := strconv.ParseFloat(val, 32) // out of range gives +-Inf, as Math.fround does
	return v, true
}

func (n *Node) IsNamedElement() bool {
	return n.Kind == KindCommand && n.Count() > 0 && n.Children[0].Kind == KindIdent
}

func (n *Node) GetChild(path ...int) *Node {
	nde := n
	for _, pos := range path {
		if pos < 0 || pos >= len(nde.Children) {
			return nil
		}
		nde = nde.Children[pos]
	}
	return nde
}

func (n *Node) valueStr(name string) (string, bool) {
	if cmd := n.GetNamedElement(name); cmd != nil && cmd.Count() > 1 {
		return cmd.GetIdent(1)
	}
	return "", false
}

// Swift has three getValue overloads (String/Int/Bool); Go gets one function per type.
func (n *Node) GetValueString(name, defaultValue string) string {
	raw, ok := n.valueStr(name)
	if !ok {
		return defaultValue
	}
	return strings.TrimFunc(raw, isSwiftWhitespace)
}

func (n *Node) GetValueInt(name string, defaultValue int) int {
	raw, ok := n.valueStr(name)
	if !ok {
		return defaultValue
	}
	value := strings.TrimFunc(raw, isSwiftWhitespace)
	if v, err := strconv.Atoi(value); err == nil && intRe.MatchString(value) {
		return v
	}
	return defaultValue
}

func (n *Node) GetValueBool(name string, defaultValue bool) bool {
	raw, ok := n.valueStr(name)
	if !ok {
		return defaultValue
	}
	switch strings.TrimFunc(raw, isSwiftWhitespace) {
	case "true":
		return true
	case "false":
		return false
	}
	return defaultValue
}

func NewIdent(name string) *Node {
	return NewNode(KindIdent, NewToken(TokSymbol, name))
}

func NewIdentTok(tok *Token) *Node {
	return NewNode(KindIdent, tok)
}

func NewStrNode(literal string) *Node {
	return NewNode(KindStringLit, NewToken(TokStringLit, literal))
}

func NewImageNode(literal string) *Node {
	return NewNode(KindImage, NewToken(TokImageData, literal))
}

func NewMarkdownNode(literal string) *Node {
	return NewNode(KindMarkdownLit, NewToken(TokMarkdownLit, literal))
}

// SwiftDouble mirrors Swift String(Double): shortest digits, exponent form outside [1e-4, 2^53],
// two-digit exponent.
func SwiftDouble(value float64) string {
	switch {
	case math.IsNaN(value):
		return "nan"
	case math.IsInf(value, 1):
		return "inf"
	case math.IsInf(value, -1):
		return "-inf"
	case value == 0:
		if math.Signbit(value) {
			return "-0.0"
		}
		return "0.0"
	}
	sign := ""
	if value < 0 {
		sign = "-"
	}
	a := math.Abs(value)
	mant, expStr, _ := strings.Cut(strconv.FormatFloat(a, 'e', -1, 64), "e")
	digits := strings.Replace(mant, ".", "", 1)
	e, _ := strconv.Atoi(expStr)
	if a > math.Pow(2, 53) || a < 1e-4 {
		frac := ""
		if len(digits) > 1 {
			frac = "." + digits[1:]
		}
		expSign := "+"
		if e < 0 {
			expSign = "-"
			e = -e
		}
		return sign + digits[:1] + frac + "e" + expSign + leftPad2(strconv.Itoa(e))
	}
	if e < 0 {
		return sign + "0." + strings.Repeat("0", -e-1) + digits
	}
	if len(digits) < e+1 {
		digits += strings.Repeat("0", e+1-len(digits))
	}
	frac := digits[e+1:]
	if frac == "" {
		frac = "0"
	}
	return sign + digits[:e+1] + "." + frac
}

func leftPad2(s string) string {
	if len(s) < 2 {
		return "0" + s
	}
	return s
}

func NewFloatNode(value float64) *Node {
	return NewNode(KindFloatLit, NewToken(TokFloatLit, SwiftDouble(value)))
}

func NewIntNode(value int) *Node {
	return NewNode(KindIntLit, NewToken(TokIntLit, strconv.Itoa(value)))
}

func NewBlockExpr(children ...*Node) *Node {
	nde := NewNode(KindBlockExpr, nil)
	nde.Add(children...)
	return nde
}

func NewCommand(name string, childNodes ...*Node) *Node {
	nde := NewNode(KindCommand, nil)
	nde.Add(NewIdent(name))
	nde.Add(childNodes...)
	return nde
}
