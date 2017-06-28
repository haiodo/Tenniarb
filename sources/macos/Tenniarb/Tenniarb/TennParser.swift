//
//  TennParser.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 25/06/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//

import Foundation

public enum TennNodeKind {
    case Empty
    case Ident
    case CharLit
    case IntLit
    case FloatLit
    case StringLit
    case Command
    case Statements
    case BlockExpr
}

public class TennASTNode {
    public let kind: TennNodeKind
    public let token: TennToken?
    public var children: [TennASTNode]?
    
    var named: [String:TennASTNode]?
    
    public var count: Int {
        get {
            if let c = children {
                return c.count
            }
            return 0
        }
    }
    
    public init(kind: TennNodeKind, tok: TennToken?) {
        self.kind = kind
        self.token = tok
    }
    
    public func add( _ node: TennASTNode) {
        if children == nil {
            children = []
        }
        children?.append(node)
        
        if kind == .BlockExpr && node.isNamedElement() {
            if named == nil {
                named = [:]
            }
            if let name = self.getIdent(0) {
                named?[name] = node
            }
        }
    }
    
    public func getNamedElement(_ name: String) -> TennASTNode? {
        if kind != .BlockExpr {
            return nil
        }
        return named?[name]
    }
    
    public func getIdentText() -> String? {
        if kind == .Ident || kind == .StringLit || kind == .IntLit || kind == .FloatLit || kind == .CharLit  {
            return token?.literal
        }
        return nil
    }
    public func getIdent(_ childIndex: Int...) -> String? {
        let nde = getChild(childIndex)
        if let n = nde {
            return n.getIdentText()
        }
        return nil
    }
    public func isNamedElement( )-> Bool {
        return kind == .Command && count > 0 && self.children?[0].kind == .Ident
    }
    
    public func getChild(_ childIndex: [Int]) -> TennASTNode? {
        var nde = self
        for i in 0..<childIndex.count {
            if let nchilds = nde.children {
                let pos = childIndex[i]
                if 0 <= pos && pos < nchilds.count {
                    nde = nchilds[pos]
                }
                else {
                    return nil
                }
                
            } else {
                return nil
            }
        }
        return nde;
    }
    
    func getValueStr( _ name: String ) -> String? {
        if let cmd = self.getNamedElement(name) {
            if cmd.count > 1 {
                return cmd.getIdent(1)
            }
        }
        return nil
    }
    public func getValue(name: String, defaultValue: String = "") -> String {
        if let value = getValueStr(name) {
            return value.trimmingCharacters(in: CharacterSet.whitespacesAndNewlines)
        }
        return defaultValue
    }
    public func getValue(name: String, defaultValue: Int = 0) -> Int {
        if let value = getValueStr(name) {
            if let r = Int(value.trimmingCharacters(in: CharacterSet.whitespacesAndNewlines)) {
                return r
            }
        }
        return defaultValue
    }
    
    public func getValue(name: String, defaultValue: Bool = false) -> Bool {
        if let value = getValueStr(name) {
            if let r = Bool(value.trimmingCharacters(in: CharacterSet.whitespacesAndNewlines)) {
                return r
            }
        }
        return defaultValue
    }
}

public enum TennErrorCode {
    case ok
    case parseError
    case unexpectedToken
    case unexpectedInput
    case invalidCommandStart
    case wrongBlockTerminator
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
    var lexer: TennLexer?
    var tok: TennToken?
    
    var errors: TennErrorContainer = TennErrorContainer()
    
    public init() {
        
    }
    
    func reset( _ source: String) {
        self.lexer = TennLexer( source )
        
    }
    
    func getTok() -> TennToken? {
        self.tok = self.lexer?.getToken()
        return self.tok
    }
    func nextTok() {
        self.tok = self.lexer?.getToken()
    }
    
    private func eat(tokenType: TennTokenType) {
        if self.tok?.type == tokenType {
            self.nextTok()
        } else {
            errors.report(code: .unexpectedToken, msg: "Unexpected token: \(self.tok?.type ?? .invalid)", token: self.tok);
        }
    }
    
    func newIdent(_ token: TennToken) -> TennASTNode {
        return TennASTNode(kind: .Ident, tok: token )
    }
    func newNode(kind: TennNodeKind, _ token: TennToken?) -> TennASTNode {
        return TennASTNode(kind: kind, tok: token )
    }
    public func parse(_ source: String) -> TennASTNode {
        self.reset(source)
        
        let result = newNode(kind: .Statements, nil)
        self.nextTok()
        
        if self.tok == nil {
            errors.report(code: .unexpectedInput, msg: "Unexpected input...", token: nil);
            return result
        }
        
        var nextCmdMark: Set<TennTokenType> = Set()
        nextCmdMark.insert(TennTokenType.semiColon)
        nextCmdMark.insert(TennTokenType.eof)
        while tok != nil && tok?.type != .eof {
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
    func parseCommand( _ endTokens: Set<TennTokenType> ) -> TennASTNode? {
        
        // Skip all semicolons.
        while self.tok != nil && self.tok!.type == .semiColon {
            self.nextTok()
        }
        
        if self.tok == nil || endTokens.contains(self.tok!.type) {
            return nil
        }
        
        let cmdNode = newNode(kind: .Command, self.tok)
        
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
                cmdNode.add(self.newIdent(self.tok!))
            case .stringLit:
                cmdNode.add(self.newNode(kind: .StringLit, self.tok))
            case .intLit:
                cmdNode.add(self.newNode(kind: .IntLit, self.tok))
            case .floatLit:
                cmdNode.add(self.newNode(kind: .FloatLit, self.tok))
            case .curlyLe:
                if let stmtNode = self.parseBlock(TennTokenType.curlyLe, TennTokenType.curlyRi, self.tok!) {
                    cmdNode.add(stmtNode)
                }
                else {
                    //TODO: Add report error here
                    return cmdNode
                }
            default:
//                this.markError("Unexpected token:" + this.tok, this.tok);
                return cmdNode
            }
            if errors.hasErrors() {
                return cmdNode;
            }
            self.nextTok()
        }
        
        
        return cmdNode
    }
    func parseBlock( _ stToken: TennTokenType, _ edToken: TennTokenType, _ currentTok: TennToken  ) -> TennASTNode? {
        self.eat(tokenType: stToken)
        
        let result = self.newNode(kind: .BlockExpr, currentTok)
        
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


