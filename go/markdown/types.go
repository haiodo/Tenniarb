// Package markdown is a port of the cmkdown lexer and HTML printer (web/packages/markdown, Swift cmkdown).
package markdown

type TokenType string

const (
	TokText       TokenType = "text"
	TokBold       TokenType = "bold"       // * bold \* text * - all until next *
	TokItalic     TokenType = "italic"     // _ italic \_ text _ - all until next _
	TokImage      TokenType = "image"      // @(image name|640x480), @(image name|640), @(image name|x480)
	TokColor      TokenType = "color"      // !(red), !(#ffeeff), !(red|word) !()- default color => global text color
	TokFont       TokenType = "font"       // &(20|word), &(20) Some text &() - Define a different font-size option
	TokExpression TokenType = "expression" // ${expression}
	TokTitle      TokenType = "title"      // ## Title value
	TokBullet     TokenType = "bullet"     // * some value
	TokCode       TokenType = "code"       // `some code`
	TokUnderline  TokenType = "underline"  // <underscore>
	TokScratch    TokenType = "scratch"    // ~text~
	TokEOF        TokenType = "eof"
)

// Token Pos and Size are UTF-8 bytes (Swift: Characters, TS: UTF-16); Col stays in Characters, see readUntil.
type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Col     int
	Pos     int
	Size    int
}

type LexerError string

const (
	EndOfLineReadString      LexerError = "EndOfLineReadString"
	EndOfExpressionReadError LexerError = "EndOfExpressionReadError"
	UTF8Error                LexerError = "UTF8Error"
)
