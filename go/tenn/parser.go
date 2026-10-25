// Port of TennParser.swift.
package tenn

import "fmt"

type ErrorCode string

const (
	CodeOK                        ErrorCode = "ok"
	CodeParseError                ErrorCode = "parseError"
	CodeUnexpectedToken           ErrorCode = "unexpectedToken"
	CodeUnexpectedInput           ErrorCode = "unexpectedInput"
	CodeInvalidCommandStart       ErrorCode = "invalidCommandStart"
	CodeWrongBlockTerminator      ErrorCode = "wrongBlockTerminator"
	CodeEndOfFileDuringStringRead ErrorCode = "EndOfFileDuringStringRead"
)

type Error struct {
	ErrorCode ErrorCode
	Message   string
	Line      int
	Col       int
}

type ErrorContainer struct {
	Errors []Error
}

func (c *ErrorContainer) Report(code ErrorCode, msg string, token *Token) {
	e := Error{ErrorCode: code, Message: msg}
	if token != nil {
		e.Line, e.Col = token.Line, token.Col
	}
	c.Errors = append(c.Errors, e)
}

func (c *ErrorContainer) HasErrors() bool {
	return len(c.Errors) > 0
}

type tokenSet map[TokenType]bool

type Parser struct {
	lexer LexerProtocol
	tok   *Token

	Errors *ErrorContainer

	Factory func(source string) LexerProtocol
}

// NewParser uses the standard Lexer when factory is nil.
func NewParser(factory func(source string) LexerProtocol) *Parser {
	if factory == nil {
		factory = func(source string) LexerProtocol { return NewLexer(source) }
	}
	return &Parser{Errors: &ErrorContainer{}, Factory: factory}
}

func (p *Parser) reset(source string) {
	p.lexer = p.Factory(source)
	p.lexer.SetErrorHandler(func(code LexerError, _, _ int) {
		switch code {
		case ErrEndOfExpressionReadErr:
			p.Errors.Report(CodeWrongBlockTerminator, "Unclosed expression terminal", nil)
		case ErrEndOfLineReadString:
			p.Errors.Report(CodeEndOfFileDuringStringRead, "Unclosed string terminal", nil)
		case ErrUTF8:
			p.Errors.Report(CodeUnexpectedInput, "UTF8 conversion error", nil)
		}
	})
}

func (p *Parser) nextTok() {
	p.tok = p.lexer.GetToken()
}

func (p *Parser) tokIn(set tokenSet) bool {
	return p.tok != nil && set[p.tok.Type]
}

func (p *Parser) eat(tokenType TokenType) {
	if p.tok != nil && p.tok.Type == tokenType {
		p.nextTok()
		return
	}
	got := TokInvalid
	if p.tok != nil {
		got = p.tok.Type
	}
	p.Errors.Report(CodeUnexpectedToken, fmt.Sprintf("Unexpected token: %s", got), p.tok)
}

func (p *Parser) Parse(source string) *Node {
	p.reset(source)

	result := NewNode(KindStatements, nil)
	p.nextTok()

	if p.tok == nil {
		p.Errors.Report(CodeUnexpectedInput, "Unexpected input...", nil)
		return result
	}

	nextCmdMark := tokenSet{TokSemiColon: true, TokEOF: true}
	for p.tok != nil && p.tok.Type != TokEOF {
		if node := p.parseCommand(nextCmdMark); node != nil {
			result.Add(node)
		}
		if p.Errors.HasErrors() {
			return result
		}
		p.nextTok()
	}

	return result
}

func (p *Parser) Revert(token *Token) {
	p.lexer.Revert(p.tok)
	p.tok = token
}

var literalKinds = map[TokenType]NodeKind{
	TokSymbol:          KindIdent,
	TokStringLit:       KindStringLit,
	TokMarkdownLit:     KindMarkdownLit,
	TokIntLit:          KindIntLit,
	TokFloatLit:        KindFloatLit,
	TokExpression:      KindExpression,
	TokExpressionBlock: KindExpressionBlock,
	TokImageData:       KindImage,
}

// checkEnd reports whether the command ends at the current token. A newline separator is skipped
// (returns false) when a string or block follows it, so a command can continue on the next line.
func (p *Parser) checkEnd(endTokens tokenSet) bool {
	if !p.tokIn(endTokens) {
		return false
	}
	isNewline := func() bool {
		return p.tok != nil && p.tok.Type == TokSemiColon && p.tok.Literal == "\n"
	}
	if !isNewline() {
		return true
	}
	var curToken []*Token
	for isNewline() {
		curToken = append(curToken, p.tok)
		p.nextTok()
	}
	if p.tok != nil && (p.tok.Type == TokStringLit || p.tok.Type == TokCurlyLe) {
		return false
	}
	for len(curToken) > 0 {
		p.Revert(curToken[len(curToken)-1])
		curToken = curToken[:len(curToken)-1]
	}
	return true
}

func (p *Parser) parseCommand(endTokens tokenSet) *Node {
	// Skip all semicolons.
	for p.tok != nil && p.tok.Type == TokSemiColon {
		p.nextTok()
	}

	if p.tok == nil || endTokens[p.tok.Type] {
		return nil
	}

	cmdNode := NewNode(KindCommand, p.tok)
	cmdNode.End = p.tok.EndPos()

	if p.tok.Type != TokSymbol {
		p.Errors.Report(CodeInvalidCommandStart, fmt.Sprintf("Invalid command start symbol: %s ", p.tok.Literal), p.tok)
		return nil
	}

	for p.tok != nil && !p.checkEnd(endTokens) {
		// checkEnd may have advanced tok; Swift re-reads self.tok! here and so do we.
		tok := p.tok
		if tok.Type == TokCurlyLe {
			cmdNode.Add(p.parseBlock(TokCurlyLe, TokCurlyRi, tok))
			if p.tok != nil {
				cmdNode.End = p.tok.EndPos() // the closing brace
			}
		} else if kind, ok := literalKinds[tok.Type]; ok {
			cmdNode.Add(NewNode(kind, tok))
			cmdNode.End = tok.EndPos()
		} else {
			return cmdNode
		}
		if p.Errors.HasErrors() {
			return cmdNode
		}
		p.nextTok()
	}

	return cmdNode
}

func (p *Parser) parseBlock(stToken, edToken TokenType, currentTok *Token) *Node {
	p.eat(stToken)

	result := NewNode(KindBlockExpr, currentTok)

	if p.tok == nil {
		p.Errors.Report(CodeParseError, "No more tokens parsing block", nil)
		return result
	}

	nextCmdMark := tokenSet{TokSemiColon: true, TokEOF: true, edToken: true}
	endCheck := tokenSet{edToken: true, TokEOF: true}

	curTok := currentTok

	for p.tok != nil && !endCheck[p.tok.Type] {
		curTok = p.tok

		if node := p.parseCommand(nextCmdMark); node != nil {
			result.Add(node)
		}
		if p.tokIn(endCheck) {
			break
		}
		if p.Errors.HasErrors() {
			return result
		}
		p.nextTok()
	}

	if p.tok == nil || p.tok.Type != edToken {
		p.Errors.Report(CodeWrongBlockTerminator, "Wrong statements terminator", curTok)
	}
	return result
}

// Parse is a shortcut for a new Parser with the default lexer.
func Parse(source string) (*Node, *Parser) {
	parser := NewParser(nil)
	return parser.Parse(source), parser
}
