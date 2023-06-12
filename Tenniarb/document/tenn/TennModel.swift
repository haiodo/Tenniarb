//
//  TennModel.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 08/01/2018.
//  Copyright © 2018 Andrey Sobolev. All rights reserved.
//
//  Licensed under the Eclipse Public License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License. You may
//  obtain a copy of the License at https://www.eclipse.org/legal/epl-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//
//  See the License for the specific language governing permissions and
//  limitations under the License.

import Foundation

public enum TennNodeKind {
    case Empty
    case Ident
    case CharLit
    case IntLit
    case FloatLit
    case StringLit
    case MarkdownLit
    case Command
    case Statements
    case BlockExpr
    case Expression
    case ExpressionBlock
    case Image
}

public class TennNode {
    public let kind: TennNodeKind
    public var token: TennToken?
    public var children: [TennNode]?
    
    var named: [String:TennNode]?
    
    @inlinable
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
                result.add(c.clone())
            }
        }
        return result
    }
    public func replace(_ node: TennNode) {
        // Remove all existing children.
        children = nil
        // Just copy all from node
        if let nc = node.children {
            for c in nc {
                add(c.clone())
            }
        }
    }
    
    public func traverse(_ visitor: (_ node: TennNode) -> Void ) {
        visitor(self)
        
        if children != nil {
            for c in self.children! {
                c.traverse(visitor)
            }
        }
    }
    
    
    
    public init(kind: TennNodeKind, tok: TennToken? = nil) {
        self.kind = kind
        self.token = tok
    }
    
    @inlinable
    public func add( _ nodes: TennNode...) {
        for n in nodes {
            self.add(n)
        }
    }
    
    @inlinable
    public func add( _ nodes: [TennNode]) {
        for n in nodes {
            self.add(n)
        }
    }
    
    public func add( _ node: TennNode) {
        if children == nil {
            children = []
        }
        children?.append(node)
        
        if kind == .BlockExpr {
            if named == nil {
                named = [:]
            }
            if let name = node.getIdent(0) {
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
    public func removeNamed( _ name: String ) -> Bool {
        if var n = self.named {
            n.removeValue(forKey: name)
        }
        if let chld = children {
            let oldSize = chld.count
            self.children = chld.filter({itm in itm.getIdent(0) != name})
            // Do we really removd any field
            return oldSize != self.children!.count
        }
        return false
    }
    
    public func getBlock(_ index: Int) -> [TennNode] {
        if let bl = getChild(index), let childs = bl.children {
            return childs
        }
        return []
    }
    
    @inlinable
    public func getIdentText() -> String? {
        if kind == .Ident || kind == .StringLit || kind == .IntLit || kind == .FloatLit || kind == .CharLit || kind == .ExpressionBlock || kind == .Expression || kind == .MarkdownLit || kind == .Image  {
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
    
    @inlinable
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
