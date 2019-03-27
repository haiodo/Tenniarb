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
                    if let v = value as? NSDictionary {
                        if v.count > 0 {
                            currentContext.setObject(value, forKeyedSubscript: cmdName as NSCopying & NSObjectProtocol)
                        }
                    } else {
                        currentContext.setObject(value, forKeyedSubscript: cmdName as NSCopying & NSObjectProtocol)
                    }
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
            
            if identText.contains("${") {
                hasExpressions = true
                // We need to perform substituion
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
                if let result = currentContext.evaluateScript("`" + identText + "`") {
                    if let tk = nde.token {
                        evaluated[tk] = result
                        if let e = error {
                            evaluated[tk] = e
                        }
                    }
                    return result
                }
            }
            
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
            return result            
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
}

@objc protocol ElementProtocol: JSExport {
    var properties: [String: Any] { get }
    var items: [NSDictionary] { get }
    var links: [NSDictionary] { get }
}

@objc protocol UtilsProtocol: JSExport {
    func now() -> Double
}
    
@objc public class UtilsContext: NSObject, UtilsProtocol {
    func now() -> Double {
        return NSDate().timeIntervalSince1970
    }
}

@objc public class ItemContext: NSObject, ItemProtocol {
    var item: DiagramItem
    var itemObject: [String:Any] = [:] // To be used from references
    var hasExpressions: Bool = false
    var parentCtx: ElementContext
    
    var evaluated: [TennToken: JSValue] = [:]
    
    dynamic var properties: [String : Any] {
        get {
            return self.itemObject
        }
    }
    
    init(_ parentCtx: ElementContext ,_ item: DiagramItem ) {
        self.item = item
        self.parentCtx = parentCtx
        super.init()
        
        _ = self.updateContext();
    }
    
    
    func updateContext() -> Bool {
        var newItems: [String:Any] = [:]
        var newEvaluated: [TennToken: JSValue] = [:]
        
        self.hasExpressions = updateGetContext(nil, newItems: &newItems, newEvaluated: &newEvaluated)
        
        // Check if we had value changes
        let result = checkChanges(self.itemObject, newItems)
        
        self.itemObject = newItems
        self.evaluated = newEvaluated
        
        return result
    }
    
    func checkChanges(_ oldItems: [String:Any], _ newItems: [String: Any]) -> Bool {
        var result = false
        if oldItems.count != newItems.count {
            result = true
        }
        else {
            // We had same values count
            for (k, v) in oldItems {
                if let nk = newItems[k] {
                    if let nkv = nk as? JSValue, let nv = v as? JSValue {
                        let nkvs = nkv.toString()
                        let nvs = nv.toString()
                        if nkvs != nvs {
                            result = true
                            break
                        }
                    }
                    if let nkv = nk as? String, let nv = v as? String {
                        if nkv != nv {
                            result = true
                            break
                        }
                    }
                    if let nkv = nk as? Int, let nv = v as? Int {
                        if nkv != nv {
                            result = true
                            break
                        }
                    }
                    if let nkv = nk as? Float, let nv = v as? Float {
                        if nkv != nv {
                            result = true
                            break
                        }
                    }
                } else {
                    // Not pressent, we had changed.
                    result = true
                    break
                }
            }
        }
        return result
    }
    
    func getRelativeItems(source: Bool, target: Bool) -> [Any] {
        var result: [Any] = []
        for itm  in self.parentCtx.element.getRelatedItems(self.item, source: source, target: target) {
            if itm.id == self.item.id || itm.kind != .Link {
                continue
            }
            guard let di = itm as? LinkItem else {
                continue
            }
            if let opposite = (di.source == self.item ? di.target : di.source) {
                if let ictx = self.parentCtx.itemsMap[opposite] {
                    result.append(ictx.itemObject)
                }
            }
        }
        return result
    }
    
    fileprivate func updateGetContext( _ node: TennNode?, newItems: inout [String:Any], newEvaluated: inout [TennToken: JSValue] ) -> Bool {
        // We need to set old values to be empty
        for (k, _) in self.itemObject {
            if !k.contains("-") {
                self.parentCtx.jsContext.evaluateScript("delete \(k)")
            }
        }
        
        self.parentCtx.jsContext.setObject(self.parentCtx, forKeyedSubscript: "parent" as NSString)
        
        let valueForKey: @convention(block) (Any?, String) -> Any? = { target, key in
            if let itm = self.parentCtx.namedItems[key] {
                return itm.properties
            }
            return nil
        }
        
        self.parentCtx.jsContext.setObject(valueForKey, forKeyedSubscript: "__valueForKey" as NSString)
        self.parentCtx.jsContext.evaluateScript("items = new Proxy({}, { get: __valueForKey })")
        
        
        self.parentCtx.jsContext.setObject(self.parentCtx.utils, forKeyedSubscript: "utils" as NSString)
        
        // Update position
        self.parentCtx.jsContext.setObject([self.item.x, self.item.y], forKeyedSubscript: "pos" as NSString)
        
        // Update name
        self.parentCtx.jsContext.setObject(self.item.name, forKeyedSubscript: "name" as NSString)
        self.parentCtx.jsContext.setObject(self.item.kind.commandName, forKeyedSubscript: "kind" as NSString)
        self.parentCtx.jsContext.setObject(self.item.id.uuidString, forKeyedSubscript: "id" as NSString)
        newItems["name"] = self.item.name
        
        // Edge operations
        let edges: @convention(block) () -> Any? = { () in
            return self.getRelativeItems(source: true, target: true)
        }
        
        let inputs: @convention(block) () -> Any? = { () in
           return self.getRelativeItems(source: false, target: true)
        }
        let outputs: @convention(block) () -> Any? = { () in
            return self.getRelativeItems(source: true, target: false)
        }
        
        self.parentCtx.jsContext.setObject(edges, forKeyedSubscript: "edges" as NSString)
        self.parentCtx.jsContext.setObject(inputs, forKeyedSubscript: "inputs" as NSString)
        self.parentCtx.jsContext.setObject(outputs, forKeyedSubscript: "outputs" as NSString)
        
        let byTag: @convention(block) (String) -> Any? = { tagName in
            // Make it in right order every time.
            return self.parentCtx.element.items.map({(e) -> ItemContext? in self.parentCtx.itemsMap[e]}).filter({e in e != nil && e!.properties[tagName] != nil}).map { e in e!.properties }
        }
        
        self.parentCtx.jsContext.setObject(byTag, forKeyedSubscript: "byTag" as NSString)
        
        let sum: @convention(block) (String, Any?) -> Any? = { (tagName, fieldName) in
            var result: Double = 0
            var field = ""
            if let f = fieldName as? String {
                field = f
            }
            if let f = fieldName as? JSValue {
                field = f.toString()
            }
            if field.count == 0 {
                field = tagName
            }
            self.parentCtx.namedItems.values.filter({e in e.properties[tagName] != nil}).forEach({e in
                if let v = e.properties[field] as? Int {
                    result += Double(v)
                }
                if let v = e.properties[field] as? Double {
                    result += v
                }
                if let v = e.properties[field] as? JSValue {
                    result += v.toDouble()
                }
            })
            return result
        }
        
        self.parentCtx.jsContext.setObject(byTag, forKeyedSubscript: "byTag" as NSString)
        self.parentCtx.jsContext.setObject(sum, forKeyedSubscript: "sum" as NSString)
        
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
    
    var namedItems:[String: ItemContext] = [:]
    
    var utils = UtilsContext()
    
    dynamic var properties: [String : Any] {
        get {
            return elementObject
        }
    }
    dynamic var items: [NSDictionary] {
        get {
            return itemsMap.filter({(k,_) in k.kind == .Item }).values.map({e in e.properties as NSDictionary})
        }
    }
    dynamic var links: [NSDictionary] {
        get {
            return itemsMap.filter({(k,_) in k.kind == .Link }).values.map({e in e.properties as NSDictionary})
        }
    }
    
    fileprivate func reCalculate(_ withExprs: [ItemContext]) -> [DiagramItem] {
        var iterations = 100
        var changes: [DiagramItem:Bool] = [:]
        var toCheck: [ItemContext] = []
        toCheck.append(contentsOf: withExprs)
        //TODO: Add more smart cycle detection logic
        while iterations > 0 && toCheck.count > 0 {
            var changed = 0
            for ic in toCheck {
                if ic.hasExpressions && ic.updateContext() {
                    changes[ic.item] = true
                    changed += 1
                }
            }
            if changed == 0 {
                break
            }
            iterations -= 1;
        }
        var result: [DiagramItem] = []
        result.append(contentsOf: changes.keys)
        return result
    }
    
    init(_ context: ExecutionContext, _ element: Element ) {
        self.element = element
        self.context = context
        super.init()
        
        self.updateContext()
        var withExprs: [ItemContext] = []
        for itm in element.items {
            let ic = ItemContext(self, itm)
            self.itemsMap[itm] = ic
            if ic.hasExpressions {
                withExprs.append(ic)
            }
            self.namedItems[itm.name] = ic
        }
        if self.hasExpressions {
            self.updateContext()
        }
        
        _ = reCalculate(withExprs)
    }
    fileprivate func updateGetContext( _ node: TennNode?, newItems: inout [String:Any], newEvaluated: inout [TennToken: JSValue] ) -> Bool {
        // We need to set old values to be empty
        for (k, _) in self.elementObject {
            self.jsContext.evaluateScript("delete \(k)")
        }
        
        self.jsContext.setObject(self.utils, forKeyedSubscript: "utils" as NSCopying & NSObjectProtocol)

        return processBlock(node ?? self.element.properties.node, self.jsContext, &newItems, &newEvaluated)
    }
    
    func updateContext(_ node:TennNode? = nil ) {
        var newItems: [String:Any] = [:]
        var newEvaluated: [TennToken: JSValue] = [:]

        self.hasExpressions = updateGetContext(node, newItems: &newItems, newEvaluated: &newEvaluated)
        
        self.elementObject = newItems
        self.evaluated = newEvaluated
    }
    func processEvent(_ event: ModelEvent ) {
        var needUpdateNamed = false
        for (k,v) in event.items {
            switch v {
            case .Append:
                // New item we need to add it to calculation
                let ikc = ItemContext(self, k)
                self.itemsMap[k] = ikc
                self.namedItems[k.name] = ikc
            case .Remove:
                if let idx = self.itemsMap.index(forKey: k) {
                    self.itemsMap.remove(at: idx)
                    self.namedItems.removeValue(forKey: k.name)
                }
            case .Update:
                if let ci = self.itemsMap[k] {
                    _ = ci.updateContext()
                }
                // Probable we need to update list of named items.
                
                needUpdateNamed = true
            }
        }
        
        if needUpdateNamed {
            self.namedItems.removeAll()
            for itm in element.items {
                self.namedItems[itm.name] = self.itemsMap[itm]
            }
        }
        
        var withExprs: [ItemContext] = []
        withExprs.append(contentsOf: self.itemsMap.values.filter({ e in e.hasExpressions }))
        
        let changed = reCalculate(withExprs)
        for c in changed {
            if event.items.index(forKey: c) == nil {
                event.items[c] = .Update
            }
        }
    }
}

public class ExecutionContext: IElementModelListener {
    var elements: [Element:ElementContext] = [:]
    var rootCtx: ElementContext?
    private let internalQueue: DispatchQueue = DispatchQueue( label: "ExecutionContextQueue",
                                                              qos: .background, autoreleaseFrequency: .never )

    
    public func setElement(_ element: Element) {
        rootCtx = ElementContext(self, element)
        self.elements[element] = rootCtx
    }
    public func updateAll(_ notifier: @escaping () -> Void) {
        self.internalQueue.async( execute: {
            if let root = self.rootCtx {
               _ = root.updateContext()
                
                for ci in root.itemsMap.values {
                    if ci.hasExpressions {
                        _ = ci.updateContext()
                    }
                }
                notifier()
            }
        })
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
