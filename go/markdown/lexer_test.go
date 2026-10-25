package markdown

import (
	"reflect"
	"testing"

	"github.com/haiodo/tenniarb/go/tenn"
)

func expectToken(t *testing.T, tokens []Token, i int, typ TokenType, literal string, pos int) {
	t.Helper()
	if i >= len(tokens) {
		t.Fatalf("#%d: only %d tokens", i, len(tokens))
	}
	got := tokens[i]
	if got.Pos != pos || got.Literal != literal || got.Type != typ {
		t.Errorf("#%d: got %s %q pos %d, want %s %q pos %d", i, got.Type, got.Literal, got.Pos, typ, literal, pos)
	}
}

func expectCount(t *testing.T, tokens []Token, n int) {
	t.Helper()
	if len(tokens) != n {
		t.Fatalf("got %d tokens, want %d: %+v", len(tokens), n, tokens)
	}
}

const boldCode = `* 1. *Re-connect* local NSM only(if pod are same)
* 1.1 Modify NSMD(1/2) stored connection info
* 1.2 do Request() on local Dataplane
* 1.3 return connection to NSC/NSE.
* - Dataplane/NSMD1 is potential fail points here.
* 2. Cleanup and configure new connection.
* 2.1 NSMD1 do *Close()* on local Dataplane.
* 2.2 NSMD1 do Close() on remote NSMD2
* 2.3 NDMS2 do Close() on local DataPlane
* 2.4 NSMD2 do Close() on local NSE
* 2.5 do "Connection" with all steps again.`

const basicCode = "*box* text\n# title A\n\nRegular text *bold* line _italic_ line.\n\n@(my_image|640)\n"

// cmkdownTests.swift

func TestExample(t *testing.T) {
	tokens := GetTokens("*Display* queries/paths\nin *Responses*\nRM-13104")
	expectCount(t, tokens, 8)
	expectToken(t, tokens, 0, TokBold, "Display", 0)
}

func TestHWorld(t *testing.T) {
	tokens := GetTokens("Hello World!")
	expectCount(t, tokens, 2)
	expectToken(t, tokens, 0, TokText, "Hello World!", 0)
	if tokens[1].Type != TokEOF {
		t.Errorf("last token %s, want eof", tokens[1].Type)
	}
}

func TestBoldParsing2(t *testing.T) {
	tokens := GetTokens(boldCode)
	expectCount(t, tokens, 37)
	expectToken(t, tokens, 22, TokBold, "Close()", 279)
}

func TestBasicParsing(t *testing.T) {
	tokens := GetTokens(basicCode)
	expectCount(t, tokens, 15)
	if tokens[10].Pos != 61 {
		t.Errorf("#10 pos %d, want 61", tokens[10].Pos)
	}
	expectToken(t, tokens, 12, TokImage, "my_image|640", 65)
}

func TestItalicParsing(t *testing.T) {
	tokens := GetTokens("_italic_ text _more italic_")
	expectCount(t, tokens, 4)
	expectToken(t, tokens, 0, TokItalic, "italic", 0)
	expectToken(t, tokens, 2, TokItalic, "more italic", 14)
}

func TestCodeParsing(t *testing.T) {
	tokens := GetTokens("`code` and more `more code`")
	expectCount(t, tokens, 4)
	expectToken(t, tokens, 0, TokCode, "code", 1)
	expectToken(t, tokens, 2, TokCode, "more code", 17)
}

func TestTitleParsing(t *testing.T) {
	tokens := GetTokens("# Title\n## Second Title\n### Third Title")
	expectCount(t, tokens, 4)
	expectToken(t, tokens, 0, TokTitle, "# Title", 0)
	expectToken(t, tokens, 1, TokTitle, "## Second Title", 8)
	expectToken(t, tokens, 2, TokTitle, "### Third Title", 24)
}

func TestUnderlineParsing(t *testing.T) {
	tokens := GetTokens("<underline> text <more underline>")
	expectCount(t, tokens, 4)
	expectToken(t, tokens, 0, TokUnderline, "underline", 0)
	expectToken(t, tokens, 2, TokUnderline, "more underline", 17)
}

func TestScratchParsing(t *testing.T) {
	tokens := GetTokens("~scratch~ text ~more scratch~")
	expectCount(t, tokens, 4)
	expectToken(t, tokens, 0, TokScratch, "scratch", 0)
	expectToken(t, tokens, 2, TokScratch, "more scratch", 15)
}

func TestColorParsing(t *testing.T) {
	tokens := GetTokens("!(red) text !(#ff0000) more")
	expectCount(t, tokens, 5)
	expectToken(t, tokens, 0, TokColor, "red", 2)
	expectToken(t, tokens, 2, TokColor, "#ff0000", 14)
}

func TestFontParsing(t *testing.T) {
	tokens := GetTokens("&(14) text &(20) more")
	expectCount(t, tokens, 5)
	expectToken(t, tokens, 0, TokFont, "14", 2)
	expectToken(t, tokens, 2, TokFont, "20", 13)
}

func TestExpressionParsing(t *testing.T) {
	tokens := GetTokens("${expression} text ${another}")
	expectCount(t, tokens, 4)
	expectToken(t, tokens, 0, TokExpression, "expression", 2)
	expectToken(t, tokens, 2, TokExpression, "another", 21)
}

func TestComplexMarkdown(t *testing.T) {
	tokens := GetTokens("# Main Title\n\n" +
		"This is *bold* and _italic_ text with `code` and <underline> and ~scratch~.\n\n" +
		"* Bullet item\n  * Nested bullet\n\n" +
		"!(red) Colored text !(#ff0000) more colored.\n\n" +
		"&(14) Font size 14 &(20) font size 20.\n\n" +
		"${expression} and ${another expression}\n\n" +
		"@(image.png|640x480) Image with size.")
	expectCount(t, tokens, 43)
	expectToken(t, tokens, 0, TokTitle, "# Main Title", 0)
	expectToken(t, tokens, 3, TokBold, "bold", 22)
	expectToken(t, tokens, 5, TokItalic, "italic", 33)
	expectToken(t, tokens, 7, TokCode, "code", 53)
	expectToken(t, tokens, 9, TokUnderline, "underline", 63)
	expectToken(t, tokens, 11, TokScratch, "scratch", 79)
	expectToken(t, tokens, 14, TokText, "\n", 90)
	expectToken(t, tokens, 15, TokBullet, "*", 91)
	expectToken(t, tokens, 19, TokBullet, "*", 107)
	expectToken(t, tokens, 23, TokColor, "red", 126)
	expectToken(t, tokens, 25, TokColor, "#ff0000", 146)
	expectToken(t, tokens, 29, TokFont, "14", 172)
	expectToken(t, tokens, 31, TokFont, "20", 191)
	expectToken(t, tokens, 35, TokExpression, "expression", 212)
	expectToken(t, tokens, 37, TokExpression, "another expression", 230)
	expectToken(t, tokens, 40, TokImage, "image.png|640x480", 253)
}

// TenniarbTests/MarkDownTests.swift

func TestMarkdownBoldParsing(t *testing.T) {
	tokens := GetTokens("*Display* queries/paths\nin *Responses*\nRM-13104")
	expectCount(t, tokens, 8)
	expectToken(t, tokens, 0, TokBold, "Display", 0)
}

// Not covered by the Swift tests; expectations come from the Swift lexer (see cmd/mdfuzz).

type lc struct {
	typ       TokenType
	lit       string
	line, col int
}

func TestLineAndCol(t *testing.T) {
	var got []lc
	for _, tok := range GetTokens("a\nbb *x*") {
		got = append(got, lc{tok.Type, tok.Literal, tok.Line, tok.Col})
	}
	want := []lc{{TokText, "a", 0, 0}, {TokText, "\n", 0, 0}, {TokText, "bb ", 1, 0}, {TokBold, "x", 1, 5}, {TokEOF, "\x00", 1, 5}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func typeLits(tokens []Token) [][2]string {
	var out [][2]string
	for _, tok := range tokens {
		out = append(out, [2]string{string(tok.Type), tok.Literal})
	}
	return out
}

func TestUnterminatedConstructs(t *testing.T) {
	type lexErr struct {
		err        LexerError
		start, pos int
	}
	var errs []lexErr
	lex := func(s string) [][2]string {
		l := NewLexer(s)
		l.SetErrorHandler(func(e LexerError, a, b int) { errs = append(errs, lexErr{e, a, b}) })
		var out []Token
		for tok := l.GetToken(); tok != nil; tok = l.GetToken() {
			out = append(out, *tok)
		}
		return typeLits(out)
	}
	// The content of an unterminated expression is dropped, an unterminated code span is kept.
	if got, want := lex("a ${b"), [][2]string{{"text", "a "}, {"eof", "\x00"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("expression: got %v, want %v", got, want)
	}
	if got, want := lex("`ab"), [][2]string{{"code", "ab"}, {"eof", "\x00"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("code: got %v, want %v", got, want)
	}
	want := []lexErr{{EndOfExpressionReadError, 4, 5}, {EndOfLineReadString, 1, 3}}
	if !reflect.DeepEqual(errs, want) {
		t.Errorf("errors: got %v, want %v", errs, want)
	}
}

func TestEscapesEmptyTokensRevert(t *testing.T) {
	if got := typeLits(GetTokens(`a\*b \q`)); !reflect.DeepEqual(got, [][2]string{{"text", `a*b \q`}, {"eof", "\x00"}}) {
		t.Errorf("escapes: %v", got)
	}
	want := [][2]string{{"color", ""}, {"text", " "}, {"code", ""}, {"eof", "\x00"}}
	if got := typeLits(GetTokens("!() ``")); !reflect.DeepEqual(got, want) {
		t.Errorf("empty tokens: %v", got)
	}
	l := NewLexer("a")
	first := l.GetToken()
	l.Revert(first)
	if l.GetToken() != first {
		t.Error("revert did not return the same token")
	}
	l.GetToken()
	if l.GetToken() != nil {
		t.Error("token after eof")
	}
}

func TestCRLFIsOneCharacter(t *testing.T) {
	got := typeLits(GetTokens("a\r\n*b*"))
	if want := [][2]string{{"text", "a\r\n*b*"}, {"eof", "\x00"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// Pos and Size are bytes here (UTF-16 in TS); the Swift Character units come from GraphemeCount.
func TestPositionsInBytes(t *testing.T) {
	source := "\U0001F600 \u00e9 *b*"
	var bold Token
	for _, tok := range GetTokens(source) {
		if tok.Type == TokBold {
			bold = tok
		}
	}
	if bold.Pos != 8 || bold.Size != 1 {
		t.Errorf("bytes: pos %d size %d, want 8 1", bold.Pos, bold.Size)
	}
	if pos := tenn.GraphemeCount(source[:bold.Pos]); pos != 4 {
		t.Errorf("Swift pos %d, want 4", pos)
	}
}

func TestGraphemes(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"ab", []string{"a", "b"}},
		{"a\r\nb", []string{"a", "\r\n", "b"}},
		{"e\u0301x", []string{"e\u0301", "x"}},
		{"\U0001F1FA\U0001F1F8\U0001F1FA", []string{"\U0001F1FA\U0001F1F8", "\U0001F1FA"}},
		{"\U0001F468\u200d\U0001F469!", []string{"\U0001F468\u200d\U0001F469", "!"}},
	}
	for _, c := range cases {
		got := tenn.Graphemes(c.in)
		if len(got) != len(c.want) {
			t.Errorf("%q: got %q, want %q", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%q: got %q, want %q", c.in, got, c.want)
				break
			}
		}
		if len(got) != tenn.GraphemeCount(c.in) {
			t.Errorf("%q: %d clusters, GraphemeCount %d", c.in, len(got), tenn.GraphemeCount(c.in))
		}
	}
}
