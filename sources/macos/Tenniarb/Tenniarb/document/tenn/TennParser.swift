//
//  TennParser.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 25/06/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//

import Foundation

public enum TennErrorCode {
    case ok
    case parseError
    case unexpectedToken
    case unexpectedInput
    case invalidCommandStart
    case wrongBlockTerminator
    case EndOfFileDuringStringRead
}

public class TennError {
    let errorCode: TennErrorCode
    let col: Int
    let line: Int
    let message: String
    init(code: TennErrorCode,  message: String, line: Int, col: Int) {
        self.errorCode = code
        self.message = message
        self.line = line
        self.col = col
    }
}
public class TennErrorContainer {
    var errors: [TennError] = []
    init() {
        
    }
    func report( code: TennErrorCode, msg: String, token: TennToken?) {
        var line = 0
        var column = 0
        if let t = token {
            line = t.line
            column = t.col
        }
        errors.append(TennError(code: code, message: msg, line: line, col: column))
    }
    func hasErrors() -> Bool {
        return !errors.isEmpty
    }
}

public class TennParser {
    var lexer: TennLexerProtocol?
    var tok: TennToken?
    
    var errors: TennErrorContainer = TennErrorContainer()
    
    var factory: (( _ source: String ) -> TennLexerProtocol) = { source in TennLexer( source )}
    
    public init() {
    }
    public convenience init( lexerFactory: @escaping (( _ source: String ) -> TennLexerProtocol) ) {
        self.init()
        factory = lexerFactory
    }
    
    func reset( _ source: String) {
        self.lexer = factory(source)
        self.lexer?.errorHandler = { (_ code: LexerError, _ stPos: Int, _ pos:Int) -> Void in
            switch code {
            case .EndOfExpressionReadError:
                self.errors.report(code: .wrongBlockTerminator, msg: "Unclosed expression terminal", token: nil)
            case .EndOfLineReadString:
                self.errors.report(code: .EndOfFileDuringStringRead, msg: "Unclosed string terminal", token: nil)
            }
        }
    }
    
    private func getTok() -> TennToken? {
        self.tok = self.lexer?.getToken()
        return self.tok
    }
    private func nextTok() {
        self.tok = self.lexer?.getToken()
    }
    
    private func eat(tokenType: TennTokenType) {
        if self.tok?.type == tokenType {
            self.nextTok()
        } else {
            errors.report(code: .unexpectedToken, msg: "Unexpected token: \(self.tok?.type ?? .invalid)", token: self.tok);
        }
    }
    
    
    public func parse(_ source: String) -> TennNode {
        self.reset(source)
        
        let result = TennNode.newNode(kind: .Statements, nil)
        self.nextTok()
        
        if self.tok == nil {
            errors.report(code: .unexpectedInput, msg: "Unexpected input...", token: nil);
            return result
        }
        
        var nextCmdMark: Set<TennTokenType> = Set()
        nextCmdMark.insert(TennTokenType.semiColon)
        nextCmdMark.insert(TennTokenType.eof)
        while self.tok != nil && self.tok!.type != .eof {
            if let node = self.parseCommand(nextCmdMark) {
                result.add(node)
            }
            if errors.hasErrors() {
                return result
            }
            self.nextTok()
        }
        
        return result
    }
    func revert(_ token: TennToken) {
        self.lexer?.revert(tok: self.tok!)
        self.tok = token
    }
    private func parseCommand( _ endTokens: Set<TennTokenType> ) -> TennNode? {
        
        // Skip all semicolons.
        while self.tok != nil && self.tok!.type == .semiColon {
            self.nextTok()
        }
        
        if self.tok == nil || endTokens.contains(self.tok!.type) {
            return nil
        }
        
        let cmdNode = TennNode.newNode(kind: .Command, self.tok)
        
        if self.tok!.type != .symbol {
            errors.report(code: .invalidCommandStart, msg: "Invalid command start symbol: \(self.tok!.literal) ", token: self.tok);
            return nil
        }
        
        func checkEnd()-> Bool {
            if self.tok != nil && endTokens.contains(self.tok!.type) {
                if self.tok!.type == .semiColon && "\n" == tok?.literal {
                    var curToken:[TennToken] = []
                    while self.tok != nil && self.tok?.type == .semiColon && "\n" == self.tok?.literal {
                        curToken.append(self.tok!)
                        nextTok()
                    }
                    if self.tok != nil && (self.tok!.type == .stringLit || self.tok!.type == .curlyLe) {
                        return false
                    } else {
                        while !curToken.isEmpty {
                            self.revert(curToken.removeLast());
                        }
                        return true;
                    }
                }
                return true;
            }
            return false
        }
        
        while self.tok != nil && !checkEnd() {
            switch self.tok!.type {
            case .symbol:
                cmdNode.add(TennNode.newIdent(self.tok!))
            case .stringLit:
                cmdNode.add(TennNode.newNode(kind: .StringLit, self.tok))
            case .markdownLit:
                cmdNode.add(TennNode.newNode(kind: .MarkdownLit, self.tok))
            case .intLit:
                cmdNode.add(TennNode.newNode(kind: .IntLit, self.tok))
            case .floatLit:
                cmdNode.add(TennNode.newNode(kind: .FloatLit, self.tok))
            case .expression:
                cmdNode.add(TennNode.newNode(kind: .Expression, self.tok))
            case .expressionBlock:
                cmdNode.add(TennNode.newNode(kind: .ExpressionBlock, self.tok))
            case .imageData:
                cmdNode.add(TennNode.newNode(kind: .Image, self.tok))
            case .curlyLe:
                if let stmtNode = self.parseBlock(TennTokenType.curlyLe, TennTokenType.curlyRi, self.tok!) {
                    cmdNode.add(stmtNode)
                }
                else {
                    return cmdNode
                }
            default:
                return cmdNode
            }
            if errors.hasErrors() {
                return cmdNode;
            }
            self.nextTok()
        }
        
        
        return cmdNode
    }
    private func parseBlock( _ stToken: TennTokenType, _ edToken: TennTokenType, _ currentTok: TennToken  ) -> TennNode? {
        self.eat(tokenType: stToken)
        
        let result = TennNode.newNode(kind: .BlockExpr, currentTok)
        
        if self.tok == nil {
            errors.report(code: .parseError, msg: "No more tokens parsing block", token: nil);
            return result
        }
        
        var nextCmdMark: Set<TennTokenType> = Set()
        nextCmdMark.insert(TennTokenType.semiColon)
        nextCmdMark.insert(TennTokenType.eof)
        nextCmdMark.insert(edToken)
        
        var endCheck: Set<TennTokenType> = Set()
        endCheck.insert(edToken)
        endCheck.insert(TennTokenType.eof)
        
        var curTok = currentTok
        
        while self.tok != nil && !endCheck.contains(self.tok!.type) {
            curTok = self.tok!
            
            if let node = parseCommand(nextCmdMark) {
                result.add(node)
            }
            if endCheck.contains(self.tok!.type) {
                break
            }
            if errors.hasErrors() {
                return result
            }
            self.nextTok()
        }
        
        if self.tok == nil || self.tok!.type != edToken {
            errors.report(code: .wrongBlockTerminator, msg: "Wrong statements terminator", token: curTok);
            return result
        }
        
        
        return result
    }
}

extension TennNode {
    public static func newIdent(_ token: TennToken) -> TennNode {
        return TennNode(kind: .Ident, tok: token )
    }
    public static func newIdent(_ literal: String) -> TennNode {
        return TennNode(kind: .Ident, tok: TennToken(type: .symbol, literal: literal) )
    }
    public static func newStrNode(_ literal: String) -> TennNode {
        return TennNode(kind: .StringLit, tok: TennToken(type: .stringLit, literal: literal) )
    }
    public static func newImageNode(_ literal: String) -> TennNode {
        return TennNode(kind: .Image, tok: TennToken(type: .imageData, literal: literal) )
    }
    public static func newMarkdownNode(_ literal: String) -> TennNode {
        return TennNode(kind: .MarkdownLit, tok: TennToken(type: .markdownLit, literal: literal) )
    }
    public static func newFloatNode(_ value: Double ) -> TennNode {
        return TennNode(kind: .FloatLit, tok: TennToken(type: .floatLit, literal: String(value)) )
    }
    public static func newIntNode(_ value: Int ) -> TennNode {
        return TennNode(kind: .IntLit, tok: TennToken(type: .intLit, literal: String(value)) )
    }
    public static func newNode(kind: TennNodeKind, _ token: TennToken? = nil) -> TennNode {
        return TennNode(kind: kind, tok: token )
    }
    public static func newBlockExpr(_ children: TennNode...) -> TennNode {
        let nde = TennNode(kind: .BlockExpr, tok: nil )
        nde.add(children)
        return nde
    }
    public static func newCommand( _ name: String, _ childNodes: TennNode... ) -> TennNode {
        let nde = TennNode(kind: .Command)
        nde.add(newIdent(name))
        for n in childNodes {
            nde.add(n)
        }
        return nde
    }
}


