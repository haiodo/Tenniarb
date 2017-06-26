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
}

public class TennASTNode {
    public let kind: TennNodeKind
    public let token: TennToken
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
    
    public init(kind: TennNodeKind, tok: TennToken) {
        self.kind = kind
        self.token = tok
    }
    
    public func add( _ node: TennASTNode) {
        if children == nil {
            children = []
        }
        children?.append(node)
        
        if kind == .Statements && node.isNamedElement() {
            if named == nil {
                named = [:]
            }
            named[node.]
        }
        
    }
    public func isNamedElement( )-> Bool {
        return kind == .Command && count > 0
    }
    
}

public enum TennErrorCode {
    case ok
    case parseError
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
    
    func getValue(node: TennASTNode, name: String, defaultValue: String) {
        
    }
    
    public func parse(_ source: String) -> TennASTNode? {
        self.reset(source)
        return nil
    }
}


