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
    init( _ undoManager: UndoManager, _ view: NSView ) {
        self.manager = undoManager
        self.view = view
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
    
}

class UpdatePosition: ModelAction {
    let element: Element
    let item: DiagramItem
    let oldPos: CGPoint
    let newPos: CGPoint
    var undoCounter: Int = 0
    
    init( _ model: ElementModel, _ element: Element, _ item: DiagramItem, old: CGPoint, new: CGPoint) {
        self.element = element
        self.item = item
        self.oldPos = old
        self.newPos = new
        
        super.init(model)
    }
    fileprivate func doModify() {
        undoCounter += 1
        model.modified(self.element, undoCounter > 1 ? .Structure :.Layout)
    }
    
    override func apply() {
        self.item.x = self.newPos.x
        self.item.y = self.newPos.y
        doModify()
        
        super.apply()
    }
    override func undo() {
        self.item.x = self.oldPos.x
        self.item.y = self.oldPos.y
        
        doModify()
        
        super.undo()
    }

}
