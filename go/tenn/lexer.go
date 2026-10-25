// Port of TennLexer.swift. Positions are UTF-8 bytes, see Token.
package tenn

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const nul = 0

// DetectSymbolType classifies a bare word as intLit, floatLit or symbol.
func DetectSymbolType(value string) TokenType {
	chars := graphemes(value)
	skipFirst := len(chars) > 0 && chars[0] == "-"
	dot := false
	i := 0
	for _, c := range chars {
		if skipFirst {
			skipFirst = false
			continue
		}
		if c == "." {
			if i == 0 || dot {
				return TokSymbol
			}
			dot = true
			continue
		}
		// CharacterSet.decimalDigits on the first unicode scalar of the character
		first, _ := utf8.DecodeRuneInString(c)
		if !unicode.Is(unicode.Nd, first) {
			return TokSymbol
		}
		i++
	}
	if dot {
		return TokFloatLit
	}
	return TokIntLit
}

type Lexer struct {
	currentLine int
	currentChar int
	bufferCount int
	pos         int

	tokenBuffer []*Token
	blockState  []TokenType // top of the stack is the last element
	code        string

	currentCharValue byte

	errorHandler func(err LexerError, startPos, pos int)
}

var _ LexerProtocol = (*Lexer)(nil)

func NewLexer(code string) *Lexer {
	l := &Lexer{code: code, bufferCount: len(code)}
	if len(code) > 0 {
		l.currentCharValue = code[0]
	}
	return l
}

func (l *Lexer) SetErrorHandler(handler func(err LexerError, startPos, pos int)) {
	l.errorHandler = handler
}

func (l *Lexer) reportError(err LexerError, startPos, pos int) {
	if l.errorHandler != nil {
		l.errorHandler(err, startPos, pos)
	}
}

func (l *Lexer) Revert(tok *Token) {
	l.tokenBuffer = append([]*Token{tok}, l.tokenBuffer...)
}

// add: span is code[start:end]; line/col are taken now, like in Swift.
func (l *Lexer) add(typ TokenType, literal string, start, end int) {
	l.tokenBuffer = append(l.tokenBuffer, &Token{
		Type: typ, Literal: literal, Line: l.currentLine, Col: l.currentChar, Pos: start, Size: end - start,
	})
}

// slice: Swift decodes every slice with String(bytes:encoding:), which drops a leading U+FEFF.
func (l *Lexer) slice(start, end int) string {
	return strings.TrimPrefix(l.code[start:end], "\xef\xbb\xbf")
}

func (l *Lexer) addCheck(pattern string) {
	if len(pattern) > 0 {
		l.add(DetectSymbolType(pattern), pattern, l.pos-len(pattern), l.pos)
	}
}

// emitPending turns the text collected since stPos into a symbol/number token.
func (l *Lexer) emitPending(stPos int) {
	if stPos < l.pos {
		l.addCheck(l.slice(stPos, l.pos))
	}
}

func (l *Lexer) inc() {
	l.currentChar++
	l.pos++
	l.currentCharValue = nul
	if l.pos < l.bufferCount {
		l.currentCharValue = l.code[l.pos]
	}
}

func (l *Lexer) next() byte {
	if l.pos+1 < l.bufferCount {
		return l.code[l.pos+1]
	}
	return nul
}

func (l *Lexer) newLine() {
	l.currentLine++
	l.currentChar = 0
}

func (l *Lexer) readString(lit byte) {
	l.inc()

	foundEnd := false
	stPos := l.pos
	contentStart := l.pos
	r := ""
	for l.pos < l.bufferCount {
		cc := l.currentCharValue
		if cc == '\n' {
			l.newLine()
		} else if cc == lit {
			r += l.slice(stPos, l.pos)
			l.add(TokStringLit, r, contentStart, l.pos)
			r = ""
			l.inc()
			stPos = l.pos
			foundEnd = true
			break
		} else if cc == '\\' && l.next() == lit {
			r += l.slice(stPos, l.pos) + string([]byte{lit})
			l.inc()
			l.inc()
			stPos = l.pos
			continue
		}
		l.inc()
	}
	r += l.slice(stPos, l.pos)
	if len(r) > 0 {
		l.add(TokStringLit, r, contentStart, l.pos)
	}
	if !foundEnd {
		l.reportError(ErrEndOfLineReadString, stPos, l.pos)
	}
}

func (l *Lexer) skipCComment() {
	l.inc() // Skip /
	l.inc() // Skip *

	for l.pos < l.bufferCount {
		if l.currentCharValue == '\n' {
			l.newLine()
		} else if l.currentCharValue == '*' && l.next() == '/' {
			l.inc()
			l.inc()
			return
		}
		l.inc()
	}
}

// processComment: the line comment stops at LF without consuming it, so the main loop counts
// that LF a second time (line counter quirk of the Swift lexer).
func (l *Lexer) processComment() {
	switch l.next() {
	case '*':
		l.skipCComment()
	case '/':
		for l.pos < l.bufferCount {
			if l.currentCharValue == '\n' {
				l.newLine()
				break
			}
			l.inc()
		}
	default:
		l.inc()
	}
}

func (l *Lexer) processNewLine(cc byte) bool {
	l.inc()
	if cc == '\n' {
		l.newLine()
		// Check if we need to send a delimiter semicolon symbol.
		if len(l.blockState) == 0 || l.blockState[len(l.blockState)-1] == TokCurlyLe {
			l.add(TokSemiColon, "\n", l.pos-1, l.pos)
		}
	}
	return len(l.tokenBuffer) > 0
}

func (l *Lexer) returnToken() *Token {
	tok := l.tokenBuffer[0]
	l.tokenBuffer = l.tokenBuffer[1:]
	return tok
}

// GetToken returns the next token, or nil once eof has been delivered.
func (l *Lexer) GetToken() *Token {
	if len(l.tokenBuffer) > 0 {
		return l.returnToken()
	}

	stPos := l.pos
	for l.pos < l.bufferCount {
		cc := l.currentCharValue
		switch cc {
		case ' ', '\t', '\r', '\n':
			l.emitPending(stPos)
			if l.processNewLine(cc) {
				return l.returnToken()
			}
			stPos = l.pos
		case '{':
			l.emitPending(stPos)
			l.processCurlyOpen(cc)
			stPos = l.pos
		case '}':
			l.emitPending(stPos)
			if l.processCurlyClose(cc) {
				return l.returnToken()
			}
			stPos = l.pos
		case ';':
			l.emitPending(stPos)
			l.add(TokSemiColon, ";", l.pos, l.pos+1)
			l.inc()
			if len(l.tokenBuffer) > 0 {
				return l.returnToken()
			}
			stPos = l.pos
		case '/':
			l.emitPending(stPos)
			l.processComment()
			stPos = l.pos
		case '%', '@', '$':
			stPos = l.processPrefixed(cc, stPos)
		case '\'', '"':
			l.emitPending(stPos)
			l.readString(cc)
			if len(l.tokenBuffer) > 0 {
				return l.returnToken()
			}
			stPos = l.pos
		default:
			l.inc()
		}
	}

	l.emitPending(stPos)

	if l.pos == l.bufferCount {
		l.add(TokEOF, "\x00", l.pos, l.pos)
		l.inc()
	}

	if len(l.tokenBuffer) > 0 {
		return l.returnToken()
	}
	return nil
}

// processPrefixed handles %{ @( $( ${. A lone prefix char stays part of the current word.
func (l *Lexer) processPrefixed(cc byte, stPos int) int {
	nc := l.next()
	switch {
	case cc == '%' && nc == '{':
		l.emitPending(stPos)
		l.readExpression('{', '}', TokMarkdownLit)
	case cc == '@' && nc == '(':
		l.emitPending(stPos)
		l.readImage(')', TokImageData)
	case cc == '$' && nc == '(':
		l.emitPending(stPos)
		l.readExpression('(', ')', TokExpression)
	case cc == '$' && nc == '{':
		l.emitPending(stPos)
		l.readExpression('{', '}', TokExpressionBlock)
	default:
		l.inc()
		return stPos
	}
	return l.pos
}

func (l *Lexer) processCurlyOpen(cc byte) {
	l.inc()
	l.add(TokCurlyLe, string([]byte{cc}), l.pos-1, l.pos)
	l.blockState = append(l.blockState, TokCurlyLe)
}

func (l *Lexer) processCurlyClose(cc byte) bool {
	l.inc()
	l.add(TokCurlyRi, string([]byte{cc}), l.pos-1, l.pos)

	if len(l.blockState) == 0 {
		return false
	}
	l.blockState = l.blockState[:len(l.blockState)-1]
	return len(l.tokenBuffer) > 0
}

func (l *Lexer) readExpression(startLit, endLit byte, typ TokenType) {
	l.inc()
	l.inc()

	stPos := l.pos
	foundEnd := false
	indent := 1
	startLine := l.currentLine
	for l.pos < l.bufferCount && !foundEnd {
		switch l.currentCharValue {
		case '\n':
			l.newLine()
		case startLit:
			indent++
		case endLit:
			indent--
			foundEnd = indent == 0
		}
		if !foundEnd {
			l.inc()
		}
	}
	l.finishExpression(stPos, foundEnd, startLine, typ)
}

func (l *Lexer) readImage(endLit byte, typ TokenType) {
	l.inc()
	l.inc()

	stPos := l.pos
	foundEnd := false
	startLine := l.currentLine
	for l.pos < l.bufferCount {
		cc := l.currentCharValue
		if cc == '\n' {
			l.newLine()
		} else if cc == endLit {
			foundEnd = true
			break
		}
		l.inc()
	}
	l.finishExpression(stPos, foundEnd, startLine, typ)
}

// finishExpression is the tail shared by readExpression and readImage in Swift (identical code there).
func (l *Lexer) finishExpression(stPos int, foundEnd bool, startLine int, typ TokenType) {
	r := l.slice(stPos, l.pos)
	if !foundEnd {
		l.reportError(ErrEndOfExpressionReadErr, stPos, l.pos)
		return
	}
	if len(r) > 0 {
		l.tokenBuffer = append(l.tokenBuffer, &Token{
			Type: typ, Literal: r, Line: startLine, Col: l.currentChar, Pos: stPos, Size: l.pos - stPos,
		})
	}
	l.inc()
}
