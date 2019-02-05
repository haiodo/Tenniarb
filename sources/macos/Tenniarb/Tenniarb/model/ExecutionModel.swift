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
                              _ levelObject: inout [String : Any?],
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
                                _ currentScope: [String: Any?],
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
        var blockScope: [String:Any?] = [:]
        let he = processBlock(nde, currentContext, &blockScope, &evaluated)
        // We need to cleanup current context from inner scope values
        for (k,v) in blockScope {
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
            let result = currentContext.evaluateScript(identText)
    //            Swift.debugPrint("Evaluate: \(identText) == \(result?.toObject()) ")
            if let tk = nde.token {
                evaluated[tk] = result
            }
        }
        // Change original node to be updated one.
    case .Expression:
        hasExpressions = true
        if let identText = nde.getIdentText() {
            let result = currentContext.evaluateScript(identText)
    //            Swift.debugPrint("Evaluate: \(identText) == \(result?.toObject()) ")
            if let tk = nde.token {
                evaluated[tk] = result
            }
            return result
        }
    default:
        return nil
    }
    return nil
}

public class ItemContext {
    var item: DiagramItem
    var itemObject: [String:Any?] = [:] // To be used from references
    var context: ExecutionContext
    
    var hasExpressions: Bool = false
    var parentCtx: ElementContext
    
    var evaluated: [TennToken: JSValue] = [:]
    
    init(_ context: ExecutionContext, _ item: DiagramItem, _ parentCtx: ElementContext ) {
        self.item = item
        self.context = context
        self.parentCtx = parentCtx
        
        self.hasExpressions = self.updateContext();
    }
   
    
    func updateContext(_ node:TennNode? = nil ) -> Bool {
        self.itemObject.removeAll()
        
        self.parentCtx.jsContext.setObject(self.parentCtx.elementObject, forKeyedSubscript: "parent" as NSCopying & NSObjectProtocol)
        self.evaluated.removeAll()
        return processBlock( node ?? self.item.toTennAsProps(), self.parentCtx.jsContext, &self.itemObject, &evaluated)
    }
}

public class ElementContext {
    var element: Element
    var elementObject: [String:Any?] = [:]
    var context: ExecutionContext
    var jsContext: JSContext = JSContext()
    var hasExpressions: Bool = false
    var evaluated: [TennToken : JSValue] = [:]
    
    init(_ context: ExecutionContext, _ element: Element ) {
        self.element = element
        self.context = context
        
        self.hasExpressions = self.updateContext()
    }
    func updateContext(_ node:TennNode? = nil )-> Bool {
        self.elementObject.removeAll()
        self.evaluated.removeAll()
        return processBlock(node ?? self.element.toTennAsProps(), self.jsContext, &self.elementObject, &evaluated)
    }
}

public class ExecutionContext: IElementModelListener {
    var items:[DiagramItem: ItemContext] = [:]
    var elements: [Element:ElementContext] = [:]
    var rootCtx: ElementContext?
    
    public func setElement(_ element: Element) {
        //
        rootCtx = ElementContext(self, element)
        self.elements[element] = rootCtx
        
        var withExprs: [ItemContext] = []
        for itm in element.items {
            let ic = ItemContext(self, itm, rootCtx!)
            self.items[itm] = ic
            if ic.hasExpressions {
                withExprs.append(ic)
            }
        }
        
        for ic in withExprs {
            _ = ic.updateContext()
        }
    }
    
    public func notifyChanges(_ event: ModelEvent ) {
        if event.element == self.rootCtx?.element {
           _ =  self.rootCtx?.updateContext()
        }
        
        for (k,v) in event.items {
            switch v {
            case .Append:
                // New item we need to add it to calculation
                self.items[k] = ItemContext(self, k, self.rootCtx!)
            case .Remove:
                if let idx = self.items.index(forKey: k) {
                    self.items.remove(at: idx)
                }
            case .Update:
                if let ci = self.items[k] {
                    _ = ci.updateContext()
                }
            }
        }
        
        for ci in self.items.values {
            if ci.hasExpressions {
                _ = ci.updateContext()
            }
        }
        
    }
}

/**
    Used for temporary evaluation of values edited
 */
public class TemporaryExecutionContext {
    var context = JSContext()
    var element: Element?
    var item: DiagramItem?
    init() {
    }
    
    public func reset(_ element: Element, _ diagramItem: DiagramItem?) {
        self.item = diagramItem
        self.element = element
    }
    public func updateSource(_ source: TennNode ) {
        self.context = JSContext()
        
        if self.item != nil {
            // This is diagram item
            Element.traverseBlock(source, {(cmdName, blChild) -> Void in
                if blChild.count > 1 {
                    if let nde = blChild.getChild(1), let identText = nde.getIdentText() {
                        var value: Any = identText
                        switch nde.kind {
                        case .FloatLit:
                            value = Float(identText) ?? -1.0
                        case .IntLit:
                            value = Int(identText) ?? 0
                        case .ExpressionBlock:
                            self.context?.evaluateScript(identText)
                        default:
                            break
                        }
                        self.context?.setObject(value, forKeyedSubscript: cmdName as NSCopying & NSObjectProtocol)
                    
                    }
                }
            })
        }
        else {
            // This is element model
        }
        
    }
        
    public func evaluate( _ text: String) -> String? {
        return self.context?.evaluateScript(text)?.toString()
    }
}

