package markdown

import (
	"strings"

	"github.com/haiodo/tenniarb/go/tenn"
)

// Lexer works on Characters like the Swift lexer. Token Pos/Size and errorHandler positions are
// UTF-8 bytes (offsets maps Character index -> byte offset); Col is the Swift per-line Character
// counter and stays in Characters, it cannot be expressed as a source offset (see readUntil).
type Lexer struct {
	bufferCount int
	pos         int
	currentLine int
	currentChar int

	lineState bool

	tokenBuffer      []*Token
	chars            []string
	offsets          []int
	currentCharValue string
	prevCharacter    string
	r                strings.Builder

	errorHandler func(err LexerError, startPos, pos int)
}

func NewLexer(code string) *Lexer {
	l := &Lexer{lineState: true, prevCharacter: "\x00"}
	l.chars = tenn.Graphemes(code)
	l.bufferCount = len(l.chars)
	l.offsets = make([]int, 1, len(l.chars)+1)
	for _, c := range l.chars {
		l.offsets = append(l.offsets, l.offsets[len(l.offsets)-1]+len(c))
	}
	l.currentCharValue = l.charAt(0)
	return l
}

func (l *Lexer) SetErrorHandler(handler func(err LexerError, startPos, pos int)) {
	l.errorHandler = handler
}

func (l *Lexer) charAt(i int) string {
	if i < len(l.chars) {
		return l.chars[i]
	}
	return "\x00"
}

func (l *Lexer) reportError(err LexerError, startPos, pos int) {
	if l.errorHandler != nil {
		l.errorHandler(err, l.offsets[startPos], l.offsets[pos])
	}
}

func (l *Lexer) Revert(tok *Token) {
	l.tokenBuffer = append([]*Token{tok}, l.tokenBuffer...)
}

func (l *Lexer) add(typ TokenType, literal string, startPos int) {
	l.tokenBuffer = append(l.tokenBuffer, &Token{
		Type:    typ,
		Literal: literal,
		Line:    l.currentLine,
		Col:     l.currentChar - tenn.GraphemeCount(literal),
		Pos:     l.offsets[startPos],
		Size:    len(literal),
	})
}

func (l *Lexer) addCheck(startPos int) {
	if l.r.Len() > 0 {
		l.add(TokText, l.r.String(), startPos)
		l.r.Reset()
	}
}

func (l *Lexer) inc() {
	l.currentChar++
	l.pos++
	l.prevCharacter = l.currentCharValue
	l.currentCharValue = l.charAt(l.pos)
}

func (l *Lexer) next() string {
	return l.charAt(l.pos + 1)
}

// plainChar appends cc to the text buffer, starting a new text run at the current position.
func (l *Lexer) plainChar(cc string, startPos *int) {
	if l.r.Len() == 0 {
		*startPos = l.pos
	}
	l.r.WriteString(cc)
	l.inc()
}

func (l *Lexer) GetToken() *Token {
	if len(l.tokenBuffer) > 0 {
		return l.shift()
	}

	l.r.Reset()
	startPos := l.pos

	wasWhiteSpace := l.pos == 0
	if l.pos > 0 && l.pos < l.bufferCount {
		switch l.prevCharacter {
		case " ", "\t", "\r", "\n":
			wasWhiteSpace = true
		}
	}

	for l.pos < l.bufferCount {
		cc := l.currentCharValue
		switch cc {
		case " ", "\t":
			// Preserve spaces and tabs in text
			if l.r.Len() == 0 {
				startPos = l.pos
			}
			l.r.WriteString(cc)
			l.inc()
			wasWhiteSpace = true
		case "\r", "\n":
			// Add current text buffer before newline
			if l.r.Len() > 0 {
				l.addCheck(startPos)
				startPos = l.pos
			}
			// Add newline as separate token
			l.add(TokText, cc, l.pos)
			l.inc()
			if cc == "\n" {
				l.currentLine++
				l.currentChar = 0
				l.lineState = true // Mark as new line is started and we need to capture prefixes.
			}
			wasWhiteSpace = true
			startPos = l.pos
		case "\\":
			// Skip next if required to skip
			nc := l.next()
			if l.r.Len() == 0 {
				startPos = l.pos
			}
			l.inc()
			switch nc {
			case "@", "$", "*", "_", "#", "<", "~", "!":
				l.r.WriteString(nc)
				l.inc()
			default:
				l.r.WriteString(cc)
			}
			wasWhiteSpace = false
		case "@":
			if l.next() == "(" {
				l.addCheck(startPos)
				startPos = l.pos
				l.readUntil("(", ")", TokImage, false)
			} else {
				l.plainChar(cc, &startPos)
			}
			wasWhiteSpace = false
		case "!":
			if l.next() == "(" {
				l.addCheck(startPos)
				startPos = l.pos
				l.readUntil("(", ")", TokColor, true)
			} else {
				l.plainChar(cc, &startPos)
			}
			wasWhiteSpace = false
		case "&":
			if l.next() == "(" {
				l.addCheck(startPos)
				startPos = l.pos
				l.readUntil("(", ")", TokFont, true)
			} else {
				l.plainChar(cc, &startPos)
			}
			wasWhiteSpace = false
		case "*":
			// Check if this is bullets list, if we have at least 1 space and all spaces before it will be bullet.
			if l.lineState && l.next() == " " {
				// Only whitespaces before, and at least one space
				l.addCheck(startPos)
				startPos = l.pos
				l.r.WriteString(cc)
				l.inc()
				l.add(TokBullet, l.r.String(), startPos)
				l.r.Reset()
			} else if wasWhiteSpace && l.next() != " " {
				l.addCheck(startPos) // Add previous line
				startPos = l.pos
				l.inc()
				// This is potentially ** ** - strong or * * emphasize
				if l.processUntilCharExceptNewLine("*") {
					l.add(TokBold, l.r.String(), startPos)
				} else {
					if l.r.Len() == 0 {
						l.r.WriteString("*") // Just *
					}
					l.add(TokText, l.r.String(), startPos)
				}
				l.r.Reset()
			} else {
				l.plainChar(cc, &startPos)
			}
			wasWhiteSpace = false
		case "_", "<", "~":
			if wasWhiteSpace && l.next() != " " {
				l.addCheck(startPos)
				startPos = l.pos
				l.inc()
				end, typ := "_", TokItalic
				if cc == "<" {
					end, typ = ">", TokUnderline
				} else if cc == "~" {
					end, typ = "~", TokScratch
				}
				if l.processUntilCharExceptNewLine(end) {
					l.add(typ, l.r.String(), startPos)
				} else {
					l.add(TokText, l.r.String(), startPos)
				}
				l.r.Reset()
			} else {
				l.plainChar(cc, &startPos)
			}
			wasWhiteSpace = false
		case "#":
			l.addCheck(startPos)
			startPos = l.pos
			l.r.WriteString(cc)
			l.inc()
			l.processUntilNewLine()
			wasWhiteSpace = true
			l.add(TokTitle, l.r.String(), startPos)
			l.r.Reset()
		case "$":
			switch nc := l.next(); nc {
			case "(":
				l.addCheck(startPos)
				startPos = l.pos
				l.readUntil("(", ")", TokExpression, false)
			case "{":
				l.addCheck(startPos)
				startPos = l.pos
				l.readUntil("{", "}", TokExpression, false)
			default:
				l.plainChar(cc, &startPos)
			}
			wasWhiteSpace = false
		case "`":
			l.addCheck(startPos)
			startPos = l.pos
			l.readUntilWithEscaping("`", TokCode)
			wasWhiteSpace = false
		default:
			wasWhiteSpace = false
			l.plainChar(cc, &startPos)
		}
		// Do not collect more whitespace characters.
		if !wasWhiteSpace {
			l.lineState = false
		}
	}

	l.addCheck(startPos)

	if l.pos == l.bufferCount {
		l.add(TokEOF, "\x00", l.pos)
		l.inc()
	}

	if len(l.tokenBuffer) == 0 {
		return nil
	}
	return l.shift()
}

func (l *Lexer) shift() *Token {
	t := l.tokenBuffer[0]
	l.tokenBuffer = l.tokenBuffer[1:]
	return t
}

func (l *Lexer) processUntilNewLine() {
	// End of line comment
	for l.pos < l.bufferCount {
		cc := l.currentCharValue
		if cc == "\n" {
			l.currentLine++
			l.lineState = true // Mark as new line is started and we need to capture prefixes.
			l.inc()
			l.currentChar = 0
			break
		}
		l.r.WriteString(cc)
		l.inc()
	}
}

func (l *Lexer) processUntilCharExceptNewLine(c string) bool {
	for l.pos < l.bufferCount {
		cc := l.currentCharValue
		if cc == "\n" {
			l.currentLine++
			l.inc()
			l.currentChar = 0
			l.lineState = true // Mark as new line is started and we need to capture prefixes.
			return false
		}
		if cc == c {
			// We found out character, return
			l.inc()
			return true
		}
		l.r.WriteString(cc)
		l.inc()
	}
	return false
}

func (l *Lexer) readUntilWithEscaping(lit string, typ TokenType) {
	l.inc()

	foundEnd := false
	stPos := l.pos
	var content strings.Builder
	for l.pos < l.bufferCount {
		cur := l.currentCharValue
		if cur == "\n" {
			l.currentLine++
			l.currentChar = 0
			content.WriteString(cur)
		} else if cur == lit {
			l.add(typ, content.String(), stPos)
			content.Reset()
			l.inc()
			foundEnd = true
			break
		} else if cur == "\\" && l.next() == lit {
			content.WriteString(l.next())
			l.inc()
		} else {
			content.WriteString(cur)
		}
		l.inc()
	}
	if content.Len() > 0 {
		l.add(typ, content.String(), stPos)
	}
	if !foundEnd {
		l.reportError(EndOfLineReadString, stPos, l.pos)
	}
}

func (l *Lexer) readUntil(startLit, endLit string, typ TokenType, addEmpty bool) {
	l.inc()
	l.inc()

	stPos := l.pos
	foundEnd := false
	indent := 1
	startLine := l.currentLine
	var content strings.Builder
	for l.pos < l.bufferCount {
		cur := l.currentCharValue
		if cur == "\n" {
			l.currentLine++
			l.currentChar = 0
			content.WriteString(cur)
		} else if cur == startLit {
			indent++
			content.WriteString(cur)
		} else if cur == endLit {
			indent--
			if indent == 0 {
				foundEnd = true
				break
			}
			content.WriteString(cur)
		} else {
			content.WriteString(cur)
		}
		l.inc()
	}

	if !foundEnd {
		l.reportError(EndOfExpressionReadError, stPos, l.pos)
		return
	}
	if content.Len() > 0 || addEmpty {
		lit := content.String()
		// Line is the start line, Col the per-line counter after the content, as in Swift.
		l.tokenBuffer = append(l.tokenBuffer, &Token{
			Type:    typ,
			Literal: lit,
			Line:    startLine,
			Col:     l.currentChar - tenn.GraphemeCount(lit),
			Pos:     l.offsets[stPos],
			Size:    len(lit),
		})
	}
	l.inc()
}

// GetTokens lexes code up to and including the eof token.
func GetTokens(code string) []Token {
	l := NewLexer(code)
	var tokens []Token
	for t := l.GetToken(); t != nil; t = l.GetToken() {
		tokens = append(tokens, *t)
	}
	return tokens
}
