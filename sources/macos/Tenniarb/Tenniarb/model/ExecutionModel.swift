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
}
