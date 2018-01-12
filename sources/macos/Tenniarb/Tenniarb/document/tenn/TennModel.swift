//
//  TennModel.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 08/01/2018.
//  Copyright © 2018 Andrey Sobolev. All rights reserved.
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

public class TennNode {
    public let kind: TennNodeKind
    public let token: TennToken?
    public var children: [TennNode]?
    
    var named: [String:TennNode]?
    
    public var count: Int {
        get {
            if let c = children {
                return c.count
            }
            return 0
        }
    }
    
    public func clone() -> TennNode {
        let result = TennNode(kind: self.kind, tok: self.token)
        if children != nil {
            result.children = []
            for c in self.children! {
                result.children?.append(c.clone())
            }
        }
        return result
    }
    
    public init(kind: TennNodeKind, tok: TennToken? = nil) {
        self.kind = kind
        self.token = tok
    }
    
    public func add( _ nodes: TennNode...) {
        for n in nodes {
            self.add(n)
        }
    }
    public func add( _ node: TennNode) {
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
    
    public func getNamedElement(_ name: String) -> TennNode? {
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
    
    public func getInt(_ childIndex: Int...) -> Int? {
        let nde = getChild(childIndex)
        if let n = nde {
            if let val = n.getIdentText() {
                return Int(val)
            }
        }
        return nil
    }
    public func getFloat(_ childIndex: Int...) -> Float? {
        let nde = getChild(childIndex)
        if let n = nde {
            if let val = n.getIdentText() {
                return Float(val)
            }
        }
        return nil
    }
    
    public func isNamedElement( )-> Bool {
        return kind == .Command && count > 0 && self.children?[0].kind == .Ident
    }
    
    public func getChild(_ childIndex: Int) -> TennNode? {
        return getChild([childIndex])
    }
    public func getChild(_ childIndex: [Int]) -> TennNode? {
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
