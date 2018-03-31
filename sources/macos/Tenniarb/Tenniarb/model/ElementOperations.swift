//
//  ModelActions.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 29/01/2018.
//  Copyright © 2018 Andrey Sobolev. All rights reserved.
//

import Foundation
import Cocoa

public enum ModelEventKind {
    case Structure
    case Layout
}

public class ModelEvent {
    let kind: ModelEventKind
    var element: Element
    var items: [DiagramItem] = []
    init(kind: ModelEventKind, element: Element) {
        self.kind = kind
        self.element = element
    }
    
    func addItem( _ item: DiagramItem ) {
        self.items.append(item)
    }
}

public class ElementOperation {
    var store: ElementModelStore
    var isUndoCalled: Bool = true
    
    init( _ store: ElementModelStore ) {
        self.store = store
    }
    func apply() {
        
    }
    func undo() {
        
    }
    
    func getNotifier() -> Element {
        return store.model
    }
    func getEventKind() -> ModelEventKind {
        return .Structure
    }
}

/*
    A composite operation for Element inside changes.
*/
public class CompositeOperation: ElementOperation {
    var operations: [ElementOperation]
    var notifier: Element
    
    init(_ store:ElementModelStore, _ notifier: Element, _ ops: ElementOperation...) {
        self.operations = ops
        self.notifier = notifier
        super.init(store)
    }
    
    func add(_ ops: ElementOperation...) {
        self.operations.append(contentsOf: ops)
    }
    
    override func apply() {
        for op in self.operations {
            op.apply()
        }
    }
    override func undo() {
        for op in self.operations.reversed() {
            op.undo()
        }
    }
    override func getNotifier() -> Element {
        return self.notifier
    }
}

public class ElementModelStore {
    public let model: ElementModel
    
    public var onUpdate: [(_ item:Element, _ kind: ModelEventKind) -> Void] = []
    public var modified: Bool = false

    
    init(_ model:ElementModel ) {
        self.model = model
    }
    
    func execute( _ action: ElementOperation, _ undoManager: UndoManager?, _ refresh: @escaping () -> Void) {
        if let manager = undoManager {
            manager.registerUndo(withTarget: self, handler: {(ae: ElementModelStore) -> Void in
                ae.execute(action, manager, refresh)
            })
        }
        
        if !action.isUndoCalled {
            action.undo()
            action.isUndoCalled = true
        }
        else {
            action.apply()
            action.isUndoCalled = false
        }
        refresh()
        self.modified(action.getNotifier(), action.getEventKind())
    }
    
    public func updateName( item: DiagramItem, _ newName: String, undoManager: UndoManager?, refresh: @escaping () -> Void) {
        execute(UpdateName(self, item.parent!, item, old: item.name, new: newName), undoManager, refresh)
    }
    
    public func updateName( element: Element, _ newName: String, undoManager: UndoManager?, refresh: @escaping () -> Void) {
        execute(UpdateElementName(self, element, old: element.name, new: newName), undoManager, refresh)
    }
    
    public func updatePosition( item: DiagramItem, newPos: CGPoint, undoManager: UndoManager?, refresh: @escaping () -> Void) {
        execute(UpdatePosition(self, item.parent!, item, old: CGPoint(x: item.x, y: item.y), new: newPos), undoManager, refresh)
    }
    public func add( _ parent: Element, _ child: Element, undoManager: UndoManager?, refresh: @escaping () -> Void ) {
        execute(AddElement(self, parent, child), undoManager, refresh)
    }
    
    public func remove( _ parent: Element, _ child: Element, undoManager: UndoManager?, refresh: @escaping () -> Void  ) {
        execute(RemoveElement(self, parent, child), undoManager, refresh)
    }
    
    public func add( _ element: Element, _ item: DiagramItem, undoManager: UndoManager?, refresh: @escaping () -> Void ) {
        execute(AddItem(self, element, item), undoManager, refresh)
    }
    
    public func add( _ element: Element, source: DiagramItem, target: DiagramItem, undoManager: UndoManager?, refresh: @escaping () -> Void ) {
        //TODO: Need do via command\
        
        let op = CompositeOperation(self, element)
        
        let link = DiagramItem(kind: .Link, name:"")
        link.setData(.LinkData, LinkElementData(source: source, target: target))
        
        if !element.items.contains(source) {
            op.add(AddItem(self, element, source))
        }
        
        if !element.items.contains(target) {
            op.add(AddItem(self, element, target))
        }
        
        op.add(AddItem(self, element, link))
        
        execute(op, undoManager, refresh)
    }
    
    public func remove( _ element: Element, item: DiagramItem, undoManager: UndoManager?, refresh: @escaping () -> Void ) {
        //TODO: Need do via command
        
        let op = CompositeOperation(self, element)
        let items = element.getRelatedItems (item)
        
        for item in items {
            op.add(RemoveItem(self, element, item ))
        }
        execute(op, undoManager, refresh)
    }
    func makeNonModified() {
        modified = false
    }
    
    func modified(_ el: Element, _ kind: ModelEventKind ) {
        modified = true
        for op in onUpdate {
            op(el, kind)
        }
    }
}

class AbstractUpdateValue<ValueType>: ElementOperation {
    let element: Element
    let item: DiagramItem
    let oldValue: ValueType
    let newValue: ValueType
    
    init( _ store: ElementModelStore, _ element: Element, _ item: DiagramItem, old: ValueType, new: ValueType) {
        self.element = element
        self.item = item
        self.oldValue = old
        self.newValue = new
        
        super.init(store)
    }
    
    func apply(_ value: ValueType) {
    }
    
    override func apply() {
        self.apply(newValue)
        super.apply()
    }
    override func undo() {
        self.apply(oldValue)
        super.undo()
    }
}

class AbstractUpdateElementValue<ValueType>: ElementOperation {
    let element: Element
    
    let oldValue: ValueType
    let newValue: ValueType
    
    init( _ store: ElementModelStore, _ element: Element, old: ValueType, new: ValueType) {
        self.element = element
        self.oldValue = old
        self.newValue = new
        
        super.init(store)
    }
    
    func apply(_ value: ValueType) {
    }
    
    override func apply() {
        self.apply(newValue)
        super.apply()
    }
    override func undo() {
        self.apply(oldValue)
        super.undo()
    }
    override func getNotifier() -> Element {
        return element
    }
}


class UpdatePosition: AbstractUpdateValue<CGPoint> {
    override func apply(_ value: CGPoint) {
        self.item.x = value.x
        self.item.y = value.y
    }
    override func getEventKind() -> ModelEventKind {
        return isUndoCalled ? .Structure:  .Layout
    }
}

class UpdateName: AbstractUpdateValue<String> {
    override func apply(_ value: String) {
        self.item.name = value
    }
}

class UpdateElementName: AbstractUpdateElementValue<String> {
    override func apply(_ value: String) {
        self.element.name = value
    }
}

class AddElement: ElementOperation {
    let parent: Element
    let child: Element
    init( _ store: ElementModelStore, _ element: Element, _ child: Element ) {
        self.parent = element
        self.child = child
        super.init(store)
        
    }
    override func apply() {
        self.parent.add(child)
    }
    override func undo() {
        _ = self.parent.remove(child)
    }
    override func getNotifier() -> Element {
        return self.parent
    }
}

class AddItem: ElementOperation {
    let parent: Element
    let item: DiagramItem
    init( _ store: ElementModelStore, _ element: Element, _ item: DiagramItem ) {
        self.parent = element
        self.item = item
        super.init(store)
        
    }
    override func apply() {
        self.parent.add(item)
    }
    override func undo() {
        _ = self.parent.remove(item)
    }
    override func getNotifier() -> Element {
        return self.parent
    }
}


class RemoveElement: ElementOperation {
    let parent: Element
    let child: Element
    var removeIndex:Int = -1
    init( _ store: ElementModelStore, _ element: Element, _ child: Element ) {
        self.parent = element
        self.child = child
        super.init(store)
        
    }
    override func apply() {
        self.removeIndex = self.parent.remove(child)
    }
    override func undo() {
        self.parent.add(child, at: removeIndex)
    }
    override func getNotifier() -> Element {
        return self.parent
    }
}

class RemoveItem: ElementOperation {
    let parent: Element
    let child: DiagramItem
    var removeIndex:Int = -1
    init( _ store: ElementModelStore, _ element: Element, _ child: DiagramItem ) {
        self.parent = element
        self.child = child
        super.init(store)
        
    }
    override func apply() {
        self.removeIndex = self.parent.remove(child)
    }
    override func undo() {
        self.parent.add(child, at: removeIndex)
    }
    override func getNotifier() -> Element {
        return self.parent
    }
}
