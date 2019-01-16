//
//  ExecutionModel.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 01/12/2018.
//  Copyright © 2018 Andrey Sobolev. All rights reserved.
//

import Foundation
import JavaScriptCore

public class ItemContext {
    var item: DiagramItem
    var cachedValues: [String:String] = [:]
    
    init(_ item: DiagramItem ) {
        self.item = item
    }
}

public class ElementContext {
    var element: Element
    var cachedValues: [String:String] = [:]
    
    init(_ element: Element ) {
        self.element = element
    }
}

public class ExecutionContext: IElementModelListener {
    let context = JSContext()
    
    var items:[DiagramItem: ItemContext] = [:]
    let elementContext: ElementContext
    let store: ElementModelStore
    
    init(_ store: ElementModelStore, _ element: Element ) {
        self.store = store
        self.elementContext = ElementContext(element)
        self.store.onUpdate.append(self)
    }
    public func notifyChanges(_ event: ModelEvent ) {
//        event.items
    }
    public func setContext(_ item: DiagramItem) {
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

