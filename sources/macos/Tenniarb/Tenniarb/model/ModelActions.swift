//
//  ModelActions.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 29/01/2018.
//  Copyright © 2018 Andrey Sobolev. All rights reserved.
//

import Foundation
import Cocoa

class ModelAction {
    var model: ElementModel
    var isUndoCalled: Bool = true
    
    init( _ model: ElementModel ) {
        self.model = model
    }
    func apply( ) {
        self.isUndoCalled = false
    }
    func undo( ) {
        self.isUndoCalled = true
    }
}

class UndoActionExecutor {
    let manager: UndoManager
    let view: NSView
    let model: ElementModel
    init( _ undoManager: UndoManager, _ view: NSView, _ model: ElementModel ) {
        self.manager = undoManager
        self.view = view
        self.model = model
    }
    func execute( _ action: ModelAction) {
        self.manager.registerUndo(withTarget: self, handler: {(ae: UndoActionExecutor) -> Void in
            ae.execute(action)
        })
        
        if !action.isUndoCalled {
            action.undo()
        }
        else {
            action.apply()
        }
        self.view.needsDisplay = true
    }
    func updateName( item: DiagramItem, _ newName: String) {
        execute(UpdateName(self.model, item.parent!, item, old: item.name, new: newName))
    }
    func updatePosition( item: DiagramItem, _ newPos: CGPoint) {
        execute(UpdatePosition(self.model, item.parent!, item, old: CGPoint(x: item.x, y: item.y), new: newPos))
    }
}

class AbstractUpdateValue<ValueType>: ModelAction {
    let element: Element
    let item: DiagramItem
    let oldValue: ValueType
    let newValue: ValueType
    var undoCounter: Int = 0
    
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
        undoCounter += 1
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
        return undoCounter > 1 ? .Structure :.Layout
    }
}

class UpdateName: AbstractUpdateValue<String> {
    override func apply(_ value: String) {
        self.item.name = value
    }
}
