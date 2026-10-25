package tenn

import (
	"reflect"
	"testing"
)

func tokenize(source string) []*Token {
	lexer := NewLexer(source)
	var result []*Token
	for len(result) < 512 {
		t := lexer.GetToken()
		if t == nil {
			break
		}
		result = append(result, t)
		if t.Type == TokEOF {
			break
		}
	}
	return result
}

func noEOF(source string) []*Token {
	var out []*Token
	for _, t := range tokenize(source) {
		if t.Type != TokEOF {
			out = append(out, t)
		}
	}
	return out
}

func typesOf(tokens []*Token) []TokenType {
	out := make([]TokenType, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, t.Type)
	}
	return out
}

func literalsOf(tokens []*Token) []string {
	out := make([]string, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, t.Literal)
	}
	return out
}

func withoutNewlines(lits []string) []string {
	out := []string{}
	for _, l := range lits {
		if l != "\n" {
			out = append(out, l)
		}
	}
	return out
}

func TestLexerSymbolsAndEOF(t *testing.T) {
	lexer := NewLexer("qwe asd")
	tokens := []*Token{lexer.GetToken(), lexer.GetToken(), lexer.GetToken()}
	if got := literalsOf(tokens); !reflect.DeepEqual(got, []string{"qwe", "asd", "\x00"}) {
		t.Errorf("literals %q", got)
	}
	if got := typesOf(tokens); !reflect.DeepEqual(got, []TokenType{TokSymbol, TokSymbol, TokEOF}) {
		t.Errorf("types %v", got)
	}
	if lexer.GetToken() != nil {
		t.Error("expected nil after eof")
	}
}

func TestLexerTokens(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		types     []TokenType // nil: not checked
		literals  []string    // nil: not checked; eof and, if withoutNL, newline separators are dropped
		withoutNL bool
	}{
		{name: "emoji symbol", src: "qwe 😈", types: []TokenType{TokSymbol, TokSymbol, TokEOF}},
		{name: "ints", src: "1 42 -7", types: []TokenType{TokIntLit, TokIntLit, TokIntLit, TokEOF}},
		{name: "floats", src: "1.5 -0.25", types: []TokenType{TokFloatLit, TokFloatLit, TokEOF}},
		{name: "not numbers", src: "12ab 1.2.3", types: []TokenType{TokSymbol, TokSymbol, TokEOF}},
		{name: "double quoted", src: `name "hello world"`, literals: []string{"name", "hello world"}},
		{name: "single quoted", src: "name 'hello'", literals: []string{"name", "hello"}},
		{name: "escaped quote", src: `s "a\"b"`, literals: []string{"s", `a"b`}},
		{name: "block", src: "a { b }", types: []TokenType{TokSymbol, TokCurlyLe, TokSymbol, TokCurlyRi, TokEOF}},
		{name: "semicolon", src: "a; b", types: []TokenType{TokSymbol, TokSemiColon, TokSymbol, TokEOF}},
		{name: "newline separator", src: "a\nb", literals: []string{"a", "\n", "b"}},
		{name: "line comment", src: "a // ignored\nb", literals: []string{"a", "b"}, withoutNL: true},
		{name: "c comment", src: "a /* ignored */ b", literals: []string{"a", "b"}},
		{name: "multiline c comment", src: "a /* line one\nline two */ b", literals: []string{"a", "b"}, withoutNL: true},
		{name: "expression", src: "x $(1 + 2)", types: []TokenType{TokSymbol, TokExpression, TokEOF}, literals: []string{"x", "1 + 2"}},
		{name: "expression block", src: "x ${var a = 1; a}", types: []TokenType{TokSymbol, TokExpressionBlock, TokEOF}, literals: []string{"x", "var a = 1; a"}},
		{name: "markdown", src: "doc %{**bold**}", types: []TokenType{TokSymbol, TokMarkdownLit, TokEOF}, literals: []string{"doc", "**bold**"}},
		{name: "image", src: "img @(AAAA)", types: []TokenType{TokSymbol, TokImageData, TokEOF}, literals: []string{"img", "AAAA"}},
		{name: "lone dollar", src: "$x", types: []TokenType{TokSymbol, TokEOF}},
		{name: "empty", src: "", types: []TokenType{TokEOF}},
		{name: "whitespace only", src: "   \t  ", types: []TokenType{TokEOF}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			all := tokenize(tc.src)
			if tc.types != nil {
				if got := typesOf(all); !reflect.DeepEqual(got, tc.types) {
					t.Errorf("types %v, want %v", got, tc.types)
				}
			}
			if tc.literals != nil {
				lits := literalsOf(noEOF(tc.src))
				if tc.withoutNL {
					lits = withoutNewlines(lits)
				}
				if !reflect.DeepEqual(lits, tc.literals) {
					t.Errorf("literals %q, want %q", lits, tc.literals)
				}
			}
		})
	}
}

func TestLexerNestedBlocks(t *testing.T) {
	counts := map[TokenType]int{}
	for _, tok := range tokenize("a { b { c } }") {
		counts[tok.Type]++
	}
	if counts[TokCurlyLe] != 2 || counts[TokCurlyRi] != 2 {
		t.Errorf("counts %v", counts)
	}
}

func TestLexerUnterminatedString(t *testing.T) {
	lexer := NewLexer(`name "unterminated`)
	var errs []LexerError
	lexer.SetErrorHandler(func(e LexerError, _, _ int) { errs = append(errs, e) })
	for lexer.GetToken() != nil {
	}
	if !reflect.DeepEqual(errs, []LexerError{ErrEndOfLineReadString}) {
		t.Errorf("errors %v", errs)
	}
}

func TestLexerLineAndSize(t *testing.T) {
	var syms []*Token
	for _, tok := range tokenize("a\nbb") {
		if tok.Type == TokSymbol {
			syms = append(syms, tok)
		}
	}
	if syms[0].Line != 0 || syms[1].Line != 1 || syms[1].Size != 2 {
		t.Errorf("line/size %+v %+v", syms[0], syms[1])
	}
}

func TestLexerRevert(t *testing.T) {
	lexer := NewLexer("a b")
	first := lexer.GetToken()
	lexer.Revert(first)
	if got := lexer.GetToken().Literal; got != "a" {
		t.Errorf("got %q", got)
	}
	if got := lexer.GetToken().Literal; got != "b" {
		t.Errorf("got %q", got)
	}
}

// pos/size/col are UTF-8 bytes; the Swift values come from toSwiftToken.
func TestLexerPosSizeColUnits(t *testing.T) {
	tests := []struct {
		name string
		src  string
		idx  int // index in the token list including eof; negative counts from the end
		lit  string
		want [3]int // pos, size, col
	}{
		{"ascii", "a 😈", 0, "a", [3]int{0, 1, 1}},
		{"4-byte emoji", "a 😈", 1, "😈", [3]int{2, 4, 6}},
		{"eof after 2-byte char", "é", -1, "\x00", [3]int{2, 0, 2}},
		{"string content span", `x "a\"b"`, 1, `a"b`, [3]int{3, 4, 7}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			toks := tokenize(tc.src)
			i := tc.idx
			if i < 0 {
				i += len(toks)
			}
			tok := toks[i]
			if tok.Literal != tc.lit || [3]int{tok.Pos, tok.Size, tok.Col} != tc.want {
				t.Errorf("got %q %v, want %q %v", tok.Literal, [3]int{tok.Pos, tok.Size, tok.Col}, tc.lit, tc.want)
			}
		})
	}
}

func TestLexerSwiftUnitConversion(t *testing.T) {
	toks := tokenize("qwe 😈")
	sw := toSwiftToken(*toks[1])
	if sw["col"] != 8 || sw["pos"] != 7 || sw["size"] != 1 {
		t.Errorf("got %v", sw)
	}
}

// Swift decodes each slice with String(bytes:encoding:), which drops a leading U+FEFF.
func TestLexerBOMInsideSymbol(t *testing.T) {
	toks := noEOF("a \xef\xbb\xbfb")
	if got := literalsOf(toks); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("literals %q", got)
	}
}

// The line comment leaves its LF to the main loop, which counts it again.
func TestLexerLineCounterAfterComment(t *testing.T) {
	toks := tokenize("a // c\nb")
	for _, tok := range toks {
		if tok.Literal == "b" && tok.Line != 2 {
			t.Errorf("line of b = %d, want 2 (Swift quirk)", tok.Line)
		}
	}
}

func TestGraphemeCount(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"", 0}, {"abc", 3}, {"éx", 2}, {"\r\n", 1}, {"😈", 1},
		{"👨‍👩‍👧", 1}, {"🇩🇪🇫🇷", 2}, {"🇩🇪🇫", 2}, {"👍🏽", 1},
		{"가", 1}, {"각", 1}, {"a‍b", 2}, {"é", 1},
	}
	for _, tc := range tests {
		if got := graphemeCount(tc.in); got != tc.want {
			t.Errorf("graphemeCount(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestDetectSymbolType(t *testing.T) {
	tests := []struct {
		in   string
		want TokenType
	}{
		{"12", TokIntLit}, {"-7", TokIntLit}, {"1.5", TokFloatLit}, {"-0.25", TokFloatLit},
		{".5", TokSymbol}, {"1..2", TokSymbol}, {"1.2.3", TokSymbol}, {"12ab", TokSymbol},
		{"abc", TokSymbol}, {"٣", TokIntLit}, {"1́", TokIntLit}, {"-", TokIntLit},
	}
	for _, tc := range tests {
		if got := DetectSymbolType(tc.in); got != tc.want {
			t.Errorf("DetectSymbolType(%q) = %s, want %s", tc.in, got, tc.want)
		}
	}
}
