//
//  ExecutionModel.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 01/12/2018.
//  Copyright © 2018 Andrey Sobolev. All rights reserved.
//

import Foundation
import JavaScriptCore
import Cocoa

fileprivate func convertNameJS(_ name: String) -> String? {
    var result = ""
    for c in name.unicodeScalars {
        let cc = Character(c)
        if CharacterSet.alphanumerics.contains(c) {
            result.append(cc)
        } else {
            result.append("_")
        }
    }
    if result.count > 0 {
        return result
    }
    return nil
}
fileprivate func processBlock(_ node: TennNode,
                              _ currentContext: JSContext,
                              _ levelObject: inout [String : Any],
                              _ evaluated: inout [TennToken : JSValue]) -> Bool {
    var hasExpressions: Bool = false
    Element.traverseBlock(node, {(cmdNameRaw, blChild) -> Void in
        
        guard let cmdName = convertNameJS(cmdNameRaw) else {
            return
        }
        // we need to check if cmdName is valid command for java-script
        
        
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
            return Double(identText) ?? 0.0
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

@objc protocol UtilsProtocol: JSExport {
    func now() -> Double
    
    func textWidth(_ text: String, _ fontSize: Any?) -> Double
    func textSize(_ text: String, _ fontSize: Any?) -> [Double]
}
    
@objc public class UtilsContext: NSObject, UtilsProtocol {
    func now() -> Double {
        return NSDate().timeIntervalSince1970
    }
    func textWidth(_ text: String, _ fontSize: Any?) -> Double {
        return textSize(text, fontSize)[0]
    }
    func textSize(_ text: String, _ fontSize: Any?) -> [Double] {
        let textStyle = NSMutableParagraphStyle.default.mutableCopy() as! NSMutableParagraphStyle
        textStyle.alignment = NSTextAlignment.center
        
        var fontSizeValue:CGFloat = 18
        
        if let size = fontSize as? Double {
            fontSizeValue = CGFloat(size)
        }
        if let size = fontSize as? Int {
            fontSizeValue = CGFloat(size)
        }
        
        let textFontAttributes = [
            NSAttributedString.Key.paragraphStyle: textStyle,
            NSAttributedString.Key.font: NSFont.systemFont(ofSize: fontSizeValue)
        ]
        let attrString = NSAttributedString(string: text, attributes: textFontAttributes)
        
        let fs = CTFramesetterCreateWithAttributedString(attrString)
        let frameSize = CTFramesetterSuggestFrameSizeWithConstraints(fs, CFRangeMake(0, attrString.length), nil, CGSize(width: 1000, height: 1000), nil)
        return [Double(frameSize.width+6), Double(frameSize.height+4)]
    }
}


@objc protocol ParentProtocol: JSExport {
    var items: [Any] {
        get
    }
    var links: [Any] {
        get
    }
}

@objc protocol PositionProtocol: JSExport {
    var x: Double {
        get
    }
    var y: Double {
        get
    }
}

@objc public class PositionItem: NSObject, PositionProtocol {
    var x: Double
    var y: Double
    init( _ x: CGFloat, _ y: CGFloat) {
        self.x = Double(x)
        self.y = Double(y)
    }
    init( _ pos: CGPoint ) {
        self.x = Double(pos.x)
        self.y = Double(pos.y)
    }
}


@objc protocol VisualProtocol: JSExport {
    var width: Double {
        get
    }
    var height: Double {
        get
    }
}


@objc public class VisualItem: NSObject, VisualProtocol {
    var drawable: Drawable
    init( _ item: Drawable ) {
        self.drawable = item
    }
    var width: Double {
        get {
            return round(Double(self.drawable.getSelectorBounds().width))
        }
    }
    var height: Double {
        get {
            return round(Double(self.drawable.getSelectorBounds().height))
        }
    }
}

@objc public class ParentObject: NSObject, ParentProtocol {
    var parent: ElementContext
    var item: ItemContext
    init(_ parent: ElementContext, _ item: ItemContext ) {
        self.parent = parent
        self.item = item
    }
    var items: [Any] {
        get {
            return parent.itemsMap.values.filter({ e in e.item.kind == .Item } ).map{e in e.itemObject}
        }
    }
    var links: [Any] {
        get {
            return parent.itemsMap.values.filter({ e in e.item.kind == .Link } ).map{e in e.itemObject}
        }
    }
}


@objc public class ItemContext: NSObject {
    var item: DiagramItem
    var itemObject: [String:Any] = [:] // To be used from references
    var hasExpressions: Bool = false
    var parentCtx: ElementContext
    
    var evaluated: [TennToken: JSValue] = [:]
    
    var dependencies: Set<ItemContext> = Set()
    
    init(_ parentCtx: ElementContext ,_ item: DiagramItem ) {
        self.item = item
        self.parentCtx = parentCtx
        super.init()
    }
    
    
    func updateContext(_ scene: DrawableScene ) -> Bool {
        var newItems: [String:Any] = [:]
        var newEvaluated: [TennToken: JSValue] = [:]
        
        let container = DrawableContainer()
        scene.buildItemDrawable(self.item, container)
        self.hasExpressions = updateGetContext(nil, newItems: &newItems, newEvaluated: &newEvaluated, scene.drawables[self.item])
        
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
    
    fileprivate func registerSum() {
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
            self.parentCtx.itemsMap.values.filter({e in e.itemObject[tagName] != nil}).forEach({e in
                self.dependencies.insert(e)
                let vv = e.itemObject[field]
                if let v = vv as? Int {
                    result += Double(v)
                } else if let v = vv as? Double {
                    result += v
                } else if let v = vv as? Float {
                    result += Double(v)
                } else if let v = vv as? JSValue {
                    result += v.toDouble()
                }
            })
            return result
        }
        
        self.parentCtx.jsContext.setObject(sum, forKeyedSubscript: "sum" as NSString)
    }
    
    fileprivate func registerByTag() {
        let byTag: @convention(block) (String) -> Any? = { tagName in
            // Make it in right order every time.
            return self.parentCtx.element.items.map({(e) -> ItemContext? in self.parentCtx.itemsMap[e]}).filter({e in e != nil && e!.itemObject[tagName] != nil}).map { e in e!.itemObject }
        }
        
        self.parentCtx.jsContext.setObject(byTag, forKeyedSubscript: "byTag" as NSString)
    }
    
    fileprivate func registerInputsOutputs() {
        // Edge operations
        let edges: @convention(block) () -> Any? = { () in
            for itm in self.parentCtx.element.getRelatedItems(self.item, source: true, target: true) {
                if let ctx = self.parentCtx.itemsMap[itm] {
                    self.dependencies.insert(ctx)
                }
            }
            return self.getRelativeItems(source: true, target: true)
        }
        
        let inputs: @convention(block) () -> Any? = { () in
            for itm in self.parentCtx.element.getRelatedItems(self.item, source: false, target: true) {
                if let ctx = self.parentCtx.itemsMap[itm] {
                    self.dependencies.insert(ctx)
                }
            }
            return self.getRelativeItems(source: false, target: true)
        }
        let outputs: @convention(block) () -> Any? = { () in
            for itm in self.parentCtx.element.getRelatedItems(self.item, source: true, target: false) {
                if let ctx = self.parentCtx.itemsMap[itm] {
                    self.dependencies.insert(ctx)
                }
            }
            return self.getRelativeItems(source: true, target: false)
        }
        
        self.parentCtx.jsContext.setObject(edges, forKeyedSubscript: "edges" as NSString)
        self.parentCtx.jsContext.setObject(inputs, forKeyedSubscript: "inputs" as NSString)
        self.parentCtx.jsContext.setObject(outputs, forKeyedSubscript: "outputs" as NSString)
    }
    
    fileprivate func registerItems( ) {
        let valueForKey: @convention(block) (Any?, String) -> Any? = { target, key in
            if let itm = self.parentCtx.namedItems[key] {
                self.dependencies.insert(itm)
                return itm.itemObject
            }
            return nil
        }
        
        self.parentCtx.jsContext.setObject(valueForKey, forKeyedSubscript: "__valueForKey" as NSString)
        self.parentCtx.jsContext.evaluateScript("items = new Proxy({}, { get: __valueForKey })")
    }
    
    fileprivate func cleanContext() {
        // We need to set old values to be empty
        for (k, _) in self.itemObject {
            if !k.contains("-") && !k.contains("+") && !k.contains("[") && !k.contains("]") && !k.contains("\\") {
                self.parentCtx.jsContext.evaluateScript("delete \(k)")
            }
        }
    }
    
    fileprivate func updateGetContext( _ node: TennNode?, newItems: inout [String:Any], newEvaluated: inout [TennToken: JSValue], _ drawable: Drawable? ) -> Bool {
        cleanContext()
        
        self.parentCtx.jsContext.setObject(self.parentCtx, forKeyedSubscript: "parent" as NSString)
        
        registerItems()
        
        let parentObj = ParentObject(self.parentCtx, self)
        
        self.parentCtx.jsContext.setObject(parentObj, forKeyedSubscript: "parent" as NSString)
        self.parentCtx.jsContext.setObject(self.parentCtx.utils, forKeyedSubscript: "utils" as NSString)
        
        // Update position
        let posObj = PositionItem(self.item.x, self.item.y)
        self.parentCtx.jsContext.setObject(posObj, forKeyedSubscript: "pos" as NSString)
        
        if let dr = drawable {
            let visualObj = VisualItem(dr)
            self.parentCtx.jsContext.setObject(visualObj, forKeyedSubscript: "defaults" as NSString)
            self.parentCtx.jsContext.setObject(visualObj.width, forKeyedSubscript: "width" as NSString)
            self.parentCtx.jsContext.setObject(visualObj.height, forKeyedSubscript: "height" as NSString)
            
            newItems["width"] = visualObj.width
            newItems["height"] = visualObj.height
        }
        
        // Update name
        self.parentCtx.jsContext.setObject(self.item.name, forKeyedSubscript: "name" as NSString)
        self.parentCtx.jsContext.setObject(self.item.kind.commandName, forKeyedSubscript: "kind" as NSString)
        self.parentCtx.jsContext.setObject(self.item.id.uuidString, forKeyedSubscript: "id" as NSString)
        newItems["name"] = self.item.name
        newItems["pos"] = posObj
        
        registerInputsOutputs()
        
        registerByTag()
        registerSum()
        
        let result = processBlock( node ?? self.item.properties.node, self.parentCtx.jsContext, &newItems, &newEvaluated)
        
        
        // Cleanup most of context
        cleanContext()
        return result
    }
}

public class ElementContext: NSObject {
    var element: Element
    var elementObject: [String:Any] = [:]
    var context: ExecutionContext
    var jsContext: JSContext = JSContext()
    var hasExpressions: Bool = false
    var evaluated: [TennToken : JSValue] = [:]
    var itemsMap:[DiagramItem: ItemContext] = [:]
    
    var namedItems:[String: ItemContext] = [:]
    
    var utils = UtilsContext()
    
    fileprivate func reCalculate(_ withExprs: [ItemContext], _ scene: DrawableScene) -> [DiagramItem] {
        var iterations = 100
        var changes: [DiagramItem:Bool] = [:]
        var toCheck: [ItemContext] = []
        toCheck.append(contentsOf: withExprs)
        //TODO: Add more smart cycle detection logic
        while iterations > 0 && toCheck.count > 0 {
            var changed = 0
            for ic in toCheck {
                if ic.hasExpressions && ic.updateContext(scene) {
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
        
        let scene = DrawableScene(self.element, darkMode: false, executionContext: self.context.evalContext, scaleFactor: self.context.scaleFactor, buildChildren: false)
        
        var withExprs: [ItemContext] = []
        for itm in element.items {
            let ic = ItemContext(self, itm)
            _ = ic.updateContext(scene)
            self.itemsMap[itm] = ic
            if ic.hasExpressions {
                withExprs.append(ic)
            }
            self.namedItems[convertNameJS(itm.name) ?? itm.name] = ic
        }
        if self.hasExpressions {
            self.updateContext()
        }
        
        _ = reCalculate(withExprs, scene)
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
        let scene = DrawableScene(self.element, darkMode: false, executionContext: self.context.evalContext, buildChildren: false)
        for (k,v) in event.items {
            switch v {
            case .Append:
                // New item we need to add it to calculation
                let ikc = ItemContext(self, k)
                _ = ikc.updateContext(scene)
                self.itemsMap[k] = ikc
                needUpdateNamed = true
            case .Remove:
                if let idx = self.itemsMap.index(forKey: k) {
                    self.itemsMap.remove(at: idx)
                    self.namedItems.removeValue(forKey: convertNameJS(k.name) ?? k.name)
                }
                needUpdateNamed = true
            case .Update:
                if let ci = self.itemsMap[k] {
                    _ = ci.updateContext(scene)
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
        
        let changed = reCalculate(withExprs, scene)
        for c in changed {
            if event.items.index(forKey: c) == nil {
                event.items[c] = .Update
            }
        }
    }
}

public protocol ExecutionContextEvaluator {
    func getEvaluated(_ element: Element) -> [TennToken:JSValue]
    func getEvaluated(_ item: DiagramItem) -> [TennToken:JSValue]
}

public class ExecutionContextEval: ExecutionContextEvaluator {
    var context: ExecutionContext!
    init( ) {
    }
    public func getEvaluated(_ element: Element) -> [TennToken : JSValue] {
        return self.context.getEvaluatedNoSync(element)
    }
    
    public func getEvaluated(_ item: DiagramItem) -> [TennToken : JSValue] {
        return self.context.getEvaluatedNoSync(item)
    }
    
    
}

public class ExecutionContext:  ExecutionContextEvaluator {
    var elements: [Element:ElementContext] = [:]
    var rootCtx: ElementContext?
    private let internalGroup: DispatchGroup = DispatchGroup()

    let evalContext: ExecutionContextEval = ExecutionContextEval()
    var scaleFactor: CGFloat = 1
    
    var refreshOp: ((_ evt: ModelEvent ) -> Void)!
    
    init() {
        self.evalContext.context = self
    }
    
    public func setScaleFactor(_ factor: CGFloat) {
        self.scaleFactor = factor
    }
    
    public func setElement(_ element: Element) {
        rootCtx = ElementContext(self, element)
        self.elements[element] = rootCtx
    }
    public func updateAll(_ notifier: @escaping () -> Void) {
        DispatchQueue.global(qos: .utility).async( group: self.internalGroup, execute: {
            if let root = self.rootCtx {
                self.internalGroup.enter()
                defer {
                    self.internalGroup.leave()
                }
               _ = root.updateContext()
                let scene = DrawableScene(root.element, darkMode: false, executionContext: self.evalContext, scaleFactor: self.scaleFactor, buildChildren: false)
                // We need to recalculate all stuff
                for ci in root.itemsMap.values {
                    if ci.hasExpressions {
                        _ = ci.updateContext(scene)
                    }
                }
                notifier()
            }
        })
    }
    
    public func notifyChanges(_ event: ModelEvent ) {
        if let root = self.rootCtx {
            if event.element == root.element {
                self.internalGroup.enter()
                defer {
                    self.internalGroup.leave()
                }
               root.processEvent(event)
            }
        }
    }
    public func getEvaluated(_ element: Element) -> [TennToken:JSValue] {
        var value: [TennToken:JSValue] = [:]
        self.internalGroup.enter()
        defer {
            self.internalGroup.leave()
        }
        if let root = self.rootCtx, root.element == element {
            value = root.evaluated;
        }
        return value
    }
    public func getEvaluatedNoSync(_ element: Element) -> [TennToken:JSValue] {
        var value: [TennToken:JSValue] = [:]
        if let root = self.rootCtx, root.element == element {
            value = root.evaluated;
        }
        return value
    }
    public func getEvaluated(_ item: DiagramItem) -> [TennToken:JSValue] {
        var value: [TennToken:JSValue] = [:]
        self.internalGroup.enter()
        defer {
            self.internalGroup.leave()
        }
        if let root = self.rootCtx, let ic = root.itemsMap[item] {
            value = ic.evaluated;
        }
        return value
    }
    public func getEvaluatedNoSync(_ item: DiagramItem) -> [TennToken:JSValue] {
        var value: [TennToken:JSValue] = [:]
        if let root = self.rootCtx, let ic = root.itemsMap[item] {
            value = ic.evaluated;
        }
        return value
    }
    public func getEvaluated(_ item: DiagramItem, _ node: TennNode, _ drawable: Drawable? ) -> [TennToken:JSValue] {
        var value: [TennToken:JSValue] = [:]
        self.internalGroup.enter()
        defer {
            self.internalGroup.leave()
        }
        if let root = self.rootCtx, let ic = root.itemsMap[item] {
            var newItems: [String:Any] = [:]
            _ = ic.updateGetContext(node, newItems: &newItems, newEvaluated: &value, drawable)
        }
        return value
    }
    public func getEvaluated(_ element: Element, _ node: TennNode ) -> [TennToken:JSValue] {
        var value: [TennToken:JSValue] = [:]
        self.internalGroup.enter()
        defer {
            self.internalGroup.leave()
        }
        if let ic = rootCtx, ic.element == element {
            var newItems: [String:Any] = [:]
            _ = ic.updateGetContext(node, newItems: &newItems, newEvaluated: &value)
        }
        return value
    }
}
