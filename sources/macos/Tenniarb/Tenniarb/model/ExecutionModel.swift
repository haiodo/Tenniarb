//
//  ExecutionModel.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 01/12/2018.
//  Copyright © 2018 Andrey Sobolev. All rights reserved.
//

import Foundation
import JavaScriptCore

fileprivate func processBlock(_ node: TennNode,
                              _ currentContext: JSContext,
                              _ levelObject: inout [String : Any],
                              _ evaluated: inout [TennToken : JSValue]) -> Bool {
    var hasExpressions: Bool = false
    Element.traverseBlock(node, {(cmdName, blChild) -> Void in
        if blChild.count > 1 {
            if blChild.count == 2 {
                let ci = blChild.getChild(1)
                hasExpressions = hasExpressions || [.ExpressionBlock, .Expression].contains(ci?.kind)
                // We have simple field assignement, not array
                if let value = calculateValue(ci, currentContext, levelObject, &hasExpressions, &evaluated) {
                    levelObject[cmdName] = value
                    currentContext.setObject(value, forKeyedSubscript: cmdName as NSCopying & NSObjectProtocol)
                }
            } else {
                // We have array assignement
                var values:[Any?] = []
                for i in 1..<blChild.count {
                    let ci = blChild.getChild(i)
                    hasExpressions = hasExpressions || [.ExpressionBlock, .Expression].contains(ci?.kind)
                    values.append(calculateValue(ci, currentContext, levelObject, &hasExpressions, &evaluated ))
                }
                levelObject[cmdName] = values
                currentContext.setObject(values, forKeyedSubscript: cmdName as NSCopying & NSObjectProtocol)
            }
        } else {
            // Set empty value, to mark field defined
            levelObject[cmdName] = ""
            currentContext.setObject("", forKeyedSubscript: cmdName as NSCopying & NSObjectProtocol)
        }
    })
    return hasExpressions
}
fileprivate func calculateValue(_ node: TennNode?,
                                _ currentContext: JSContext,
                                _ currentScope: [String: Any],
                                _ hasExpressions: inout Bool,
                                _ evaluated: inout [TennToken : JSValue]) -> Any? {
    guard let nde = node else {
        return nil
    }
    
    switch nde.kind {
    case .FloatLit:
        if let identText = nde.getIdentText() {
            return Float(identText) ?? 0.0
        }
        return nil
    case .IntLit:
        if let identText = nde.getIdentText() {
            return Int(identText) ?? 0.0
        }
        return nil
    case .StringLit, .CharLit, .Ident, .MarkdownLit:
        if let identText = nde.getIdentText() {
            return identText
        }
        return nil
    case .BlockExpr:
        // A subcontext need to be constructed
        var blockScope: [String:Any] = [:]
        let he = processBlock(nde, currentContext, &blockScope, &evaluated)
        // We need to cleanup current context from inner scope values
        for (k,_) in blockScope {
            if let csv = currentScope[k] {
                currentContext.setObject(csv, forKeyedSubscript: k as NSCopying & NSObjectProtocol)
            }
        }
        hasExpressions = hasExpressions || he
        return blockScope
    case .ExpressionBlock:
        hasExpressions = true
        // Block do not have value usually.
        if let identText = nde.getIdentText() {
            var error: JSValue?
            let oldHandler = currentContext.exceptionHandler
            currentContext.exceptionHandler = {(a,b) in
                if let e = b {
                    error = e
                }
            }
            defer {
                currentContext.exceptionHandler = oldHandler
            }
            let result = currentContext.evaluateScript(identText)
            if let tk = nde.token {
                evaluated[tk] = result
                if let e = error {
                    evaluated[tk] = e
                }
            }
            
        }
        // Change original node to be updated one.
    case .Expression:
        hasExpressions = true
        if let identText = nde.getIdentText() {
            var error: JSValue?
            let oldHandler = currentContext.exceptionHandler
            currentContext.exceptionHandler = {(a,b) in
                if let e = b {
                    error = e
                }
            }
            defer {
                currentContext.exceptionHandler = oldHandler
            }
            if let result = currentContext.evaluateScript(identText) {
                if let tk = nde.token {
                    evaluated[tk] = result
                    if let e = error {
                        evaluated[tk] = e
                    }
                }
                return result
            }
        }
    default:
        return nil
    }
    return nil
}

@objc protocol ItemProtocol: JSExport {
    var properties: [String: Any] { get }
    var parent: ElementProtocol { get }
//    subscript( key: String ) -> Any? { get }
}

@objc protocol ElementProtocol: JSExport {
    var properties: [String: Any] { get }
    var items: [ItemProtocol] { get }
    var links: [ItemProtocol] { get }
    //    subscript( key: String ) -> Any? { get }
}

@objc public class ItemContext: NSObject, ItemProtocol {
    private var item: DiagramItem
    var itemObject: [String:Any] = [:] // To be used from references
    var hasExpressions: Bool = false
    var parentCtx: ElementContext
    
    var evaluated: [TennToken: JSValue] = [:]
    dynamic var parent: ElementProtocol {
        get {
            return parentCtx
        }
    }
    
    dynamic var properties: [String : Any] {
        get {
            return self.itemObject
        }
    }
    
    init(_ parentCtx: ElementContext ,_ item: DiagramItem ) {
        self.item = item
        self.parentCtx = parentCtx
        super.init()
        
        self.hasExpressions = self.updateContext();
    }
   
    
    func updateContext() -> Bool {
        var newItems: [String:Any] = [:]
        var newEvaluated: [TennToken: JSValue] = [:]
        
        let result = updateGetContext(nil, newItems: &newItems, newEvaluated: &newEvaluated)
                
        self.itemObject = newItems
        self.evaluated = newEvaluated
        
        return result
    }
    
    fileprivate func updateGetContext( _ node: TennNode?, newItems: inout [String:Any], newEvaluated: inout [TennToken: JSValue] ) -> Bool {
        self.parentCtx.jsContext.setObject(self.parentCtx, forKeyedSubscript: "parent" as NSCopying & NSObjectProtocol)
        
        // Update position
        self.parentCtx.jsContext.setObject([self.item.x, self.item.y], forKeyedSubscript: "pos" as NSCopying & NSObjectProtocol)
        
        // Update name
        self.parentCtx.jsContext.setObject(self.item.name, forKeyedSubscript: "name" as NSCopying & NSObjectProtocol)
        self.parentCtx.jsContext.setObject(self.item.kind.commandName, forKeyedSubscript: "kind" as NSCopying & NSObjectProtocol)
        self.parentCtx.jsContext.setObject(self.item.id.uuidString, forKeyedSubscript: "id" as NSCopying & NSObjectProtocol)
        
        return processBlock( node ?? self.item.properties.node, self.parentCtx.jsContext, &newItems, &newEvaluated)
    }
}

public class ElementContext: NSObject, ElementProtocol {
    var element: Element
    var elementObject: [String:Any] = [:]
    var context: ExecutionContext
    var jsContext: JSContext = JSContext()
    var hasExpressions: Bool = false
    var evaluated: [TennToken : JSValue] = [:]
    var itemsMap:[DiagramItem: ItemContext] = [:]
    
    dynamic var properties: [String : Any] {
        get {
            return elementObject
        }
    }
    dynamic var items: [ItemProtocol] {
        get {
            return itemsMap.filter({(k,_) in k.kind == .Item }).values.map({e in e as ItemProtocol})
        }
    }
    dynamic var links: [ItemProtocol] {
        get {
            return itemsMap.filter({(k,_) in k.kind == .Link }).values.map({e in e as ItemProtocol})
        }
    }
    
    init(_ context: ExecutionContext, _ element: Element ) {
        self.element = element
        self.context = context
        super.init()
        
        self.hasExpressions = self.updateContext()
        var withExprs: [ItemContext] = []
        for itm in element.items {
            let ic = ItemContext(self, itm)
            self.itemsMap[itm] = ic
            if ic.hasExpressions {
                withExprs.append(ic)
            }
        }
        if self.hasExpressions {
            _ = self.updateContext()
        }
        
        for ic in withExprs {
            _ = ic.updateContext()
        }
    }
    fileprivate func updateGetContext( _ node: TennNode?, newItems: inout [String:Any], newEvaluated: inout [TennToken: JSValue] ) -> Bool {
        return processBlock(node ?? self.element.properties.node, self.jsContext, &newItems, &newEvaluated)
    }
    
    func updateContext(_ node:TennNode? = nil )-> Bool {
        var newItems: [String:Any] = [:]
        var newEvaluated: [TennToken: JSValue] = [:]

        let result = updateGetContext(node, newItems: &newItems, newEvaluated: &newEvaluated)
        
        self.elementObject = newItems
        self.evaluated = newEvaluated
        
        return result
    }
    func processEvent(_ event: ModelEvent ) {
        for (k,v) in event.items {
            switch v {
            case .Append:
                // New item we need to add it to calculation
                self.itemsMap[k] = ItemContext(self, k)
            case .Remove:
                if let idx = self.itemsMap.index(forKey: k) {
                    self.itemsMap.remove(at: idx)
                }
            case .Update:
                if let ci = self.itemsMap[k] {
                    _ = ci.updateContext()
                }
            }
        }
        
        for ci in self.itemsMap.values {
            if ci.hasExpressions {
                _ = ci.updateContext()
            }
        }
    }
}

public class ExecutionContext: IElementModelListener {
    var elements: [Element:ElementContext] = [:]
    var rootCtx: ElementContext?
    private let internalQueue: DispatchQueue = DispatchQueue( label: "ExecutionContextQueue")

    
    public func setElement(_ element: Element) {
        rootCtx = ElementContext(self, element)
        self.elements[element] = rootCtx
    }
    
    public func notifyChanges(_ event: ModelEvent ) {
        self.internalQueue.sync( execute: {
            if let root = self.rootCtx {
                if event.element == root.element {
                   root.processEvent(event)
                }
            }
        })
    }
    public func getEvaluated(_ element: Element) -> [TennToken:JSValue] {
        var value: [TennToken:JSValue] = [:]
        self.internalQueue.sync( execute: {
            if let root = self.rootCtx, root.element == element {
                value = root.evaluated;
            }
        })
        return value
    }
    public func getEvaluated(_ item: DiagramItem) -> [TennToken:JSValue] {
        var value: [TennToken:JSValue] = [:]
        self.internalQueue.sync( execute: {
            if let root = self.rootCtx, let ic = root.itemsMap[item] {
                value = ic.evaluated;
            }
        })
        return value
    }
    public func getEvaluated(_ item: DiagramItem, _ node: TennNode ) -> [TennToken:JSValue] {
        var value: [TennToken:JSValue] = [:]
        self.internalQueue.sync( execute: {
            if let root = self.rootCtx, let ic = root.itemsMap[item] {
                var newItems: [String:Any] = [:]
                _ = ic.updateGetContext(node, newItems: &newItems, newEvaluated: &value)
            }
        })
        return value
    }
    public func getEvaluated(_ element: Element, _ node: TennNode ) -> [TennToken:JSValue] {
        var value: [TennToken:JSValue] = [:]
        self.internalQueue.sync( execute: {
            if let ic = rootCtx, ic.element == element {
                var newItems: [String:Any] = [:]
                _ = ic.updateGetContext(node, newItems: &newItems, newEvaluated: &value)
            }
        })
        return value
    }
}
