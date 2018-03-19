//
//  ModelActions.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 29/01/2018.
//  Copyright © 2018 Andrey Sobolev. All rights reserved.
//

import Foundation
import Cocoa

public class ElementOperation {
    var model: ElementModel
    var isUndoCalled: Bool = true
    var sendEvent: Bool = true
    
    init( _ model: ElementModel ) {
        self.model = model
    }
    func apply() {
        
    }
    func undo() {
        
    }
}

/*
    A composite operation for Element inside changes.
*/
public class CompositeOperation: ElementOperation {
    var operations: [ElementOperation]
    var notifier: Element
    
    init(_ model:ElementModel, notifier: Element, _ ops: ElementOperation...) {
        self.operations = ops
        self.notifier = notifier
        for op in self.operations {
            op.sendEvent = false
        }
        super.init(model)
    }
    
    override func apply() {
        for op in self.operations {
            op.apply()
        }
        model.modified(self.notifier, .Structure )
    }
    override func undo() {
        for op in self.operations.reversed() {
            op.apply()
        }
        model.modified(self.notifier, .Structure )
    }
}

public class ElementModelStore {
    public let model: ElementModel
    
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
    }
    
    public func updateName( item: DiagramItem, _ newName: String, undoManager: UndoManager?, refresh: @escaping () -> Void) {
        execute(UpdateName(self.model, item.parent!, item, old: item.name, new: newName), undoManager, refresh)
    }
    
    public func updateName( element: Element, _ newName: String, undoManager: UndoManager?, refresh: @escaping () -> Void) {
        execute(UpdateElementName(self.model, element, old: element.name, new: newName), undoManager, refresh)
    }
    
    public func updatePosition( item: DiagramItem, newPos: CGPoint, undoManager: UndoManager?, refresh: @escaping () -> Void) {
        execute(UpdatePosition(self.model, item.parent!, item, old: CGPoint(x: item.x, y: item.y), new: newPos), undoManager, refresh)
    }
    public func add( _ parent: Element, _ child: Element, undoManager: UndoManager?, refresh: @escaping () -> Void ) {
        execute(AddElement(self.model, parent, child), undoManager, refresh)
    }
    
    public func remove( _ parent: Element, _ child: Element, undoManager: UndoManager?, refresh: @escaping () -> Void  ) {
        execute(RemoveElement(self.model, parent, child), undoManager, refresh)
    }
    
    public func add( _ element: Element, _ item: DiagramItem, undoManager: UndoManager?, refresh: @escaping () -> Void ) {
        //TODO: Need do via command
        element.add(item)
        self.model.modified(element, .Structure)
    }
    
    public func add( _ element: Element, source: DiagramItem, target: DiagramItem, undoManager: UndoManager?, refresh: @escaping () -> Void ) {
        //TODO: Need do via command
        element.add(source: source, target: target)
        
        self.model.modified(element, .Structure)
    }
    
    public func remove( _ element: Element, item: DiagramItem, undoManager: UndoManager?, refresh: @escaping () -> Void ) {
        //TODO: Need do via command
        element.remove(item)
        self.model.modified(element, .Structure)
    }
}

class AbstractUpdateValue<ValueType>: ElementOperation {
    let element: Element
    let item: DiagramItem
    let oldValue: ValueType
    let newValue: ValueType
    
    init( _ model: ElementModel, _ element: Element, _ item: DiagramItem, old: ValueType, new: ValueType) {
        self.element = element
        self.item = item
        self.oldValue = old
        self.newValue = new
        
        super.init(model)
    }
    func getEventKind() -> UpdateEventKind {
        return .Structure
    }
    
    fileprivate func doModify() {
        model.modified(self.element, getEventKind())
    }
    
    func apply(_ value: ValueType) {
    }
    
    override func apply() {
        self.apply(newValue)
        doModify()
        
        super.apply()
    }
    override func undo() {
        self.apply(oldValue)
        doModify()
        super.undo()
    }
}

class AbstractUpdateElementValue<ValueType>: ElementOperation {
    let element: Element
    
    let oldValue: ValueType
    let newValue: ValueType
    
    init( _ model: ElementModel, _ element: Element, old: ValueType, new: ValueType) {
        self.element = element
        self.oldValue = old
        self.newValue = new
        
        super.init(model)
    }
    func getEventKind() -> UpdateEventKind {
        return .Structure
    }
    
    fileprivate func doModify() {
        model.modified(self.element, getEventKind())
    }
    
    func apply(_ value: ValueType) {
    }
    
    override func apply() {
        self.apply(newValue)
        doModify()
        
        super.apply()
    }
    override func undo() {
        self.apply(oldValue)
        doModify()
        super.undo()
    }
}


class UpdatePosition: AbstractUpdateValue<CGPoint> {
    override func apply(_ value: CGPoint) {
        self.item.x = value.x
        self.item.y = value.y
    }
    override func getEventKind() -> UpdateEventKind {
        return .Layout
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
    init( _ model: ElementModel, _ element: Element, _ child: Element ) {
        self.parent = element
        self.child = child
        super.init(model)
        
    }
    override func apply() {
        self.parent.add(child)
    }
    override func undo() {
        _ = self.parent.remove(child)
    }
}


class RemoveElement: ElementOperation {
    let parent: Element
    let child: Element
    var removeIndex:Int = -1
    init( _ model: ElementModel, _ element: Element, _ child: Element ) {
        self.parent = element
        self.child = child
        super.init(model)
        
    }
    override func apply() {
        self.removeIndex = self.parent.remove(child)
    }
    override func undo() {
        self.parent.add(child, at: removeIndex)
    }
}
