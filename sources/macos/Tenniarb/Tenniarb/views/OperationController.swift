//
//  OperationController.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 30/03/2019.
//  Copyright © 2019 Andrey Sobolev. All rights reserved.
//

import Foundation
import Cocoa

class OperationTextDelegate: NSObject, NSTextFieldDelegate, NSTextDelegate {
    var controller: OperationController!
    func control(_ control: NSControl, textView: NSTextView, doCommandBy commandSelector: Selector) -> Bool {
        if commandSelector == #selector(NSView.cancelOperation(_:)) {
            self.controller.dismiss(nil)
            return true
        }
        if commandSelector == #selector(NSView.insertNewline(_:)) {
            self.controller.commit()
            return true
        }        
        return false
    }
}

class OperationController: NSViewController, NSTextViewDelegate {
    @IBOutlet weak var operationsTextBox: NSTextField!
    var delegate = OperationTextDelegate()
    var store: ElementModelStore!
    var element: Element!
    var items: [DiagramItem] = []
    var controller: ViewController!
    
    override func viewDidLoad() {
        operationsTextBox.stringValue = ""
        
        self.delegate.controller = self
        operationsTextBox.delegate = self.delegate
    }
    
    func setController(_ controller: ViewController ) {
        self.controller = controller
    }
    
    func setStore(_ store: ElementModelStore ) {
        self.store = store
    }
    func setElement(_ element: Element) {
        self.element = element
    }
    
    func setItems(_ items: [DiagramItem] ) {
        self.items.append(contentsOf: items)
    }
    
    func createOperation( _ item: DiagramItem, _ node: TennNode ) -> ElementOperation? {
        let newItemProps = item.toTennAsProps(.BlockExpr)
        var changed = false
        
        Element.traverseBlock(node, {(cmdName, node) in
            if let itmProp = newItemProps.getNamedElement(cmdName), let children = node.children {
                // Property exists, we need to replace value
                itmProp.children?.removeAll()
                itmProp.add( children)
                    changed = true
            }
            else {
                // Just add new property
                newItemProps.add(node)
                changed = true
            }
        })
        if changed  {
            return self.store.createProperties(self.element, item, newItemProps)
        }
        return nil
    }
    
    func commit() {
        // We need to validate if content is valid one.
        
        let parser = TennParser()
        let node = parser.parse(operationsTextBox.stringValue)
        if parser.errors.hasErrors() {
            operationsTextBox.backgroundColor = NSColor.red
            return
        }
        
        // Apply to all items
        var operations: [ElementOperation] = []
        
        for itm in self.items {
            if let op = createOperation(itm, node) {
                operations.append(op)
            }
        }
        
        
        self.store.compositeOperation(notifier: element, undoManaget: self.controller.view.undoManager, refresh: self.controller.scene.scheduleRedraw, operations)
        self.controller?.hideOperationBox()
    }
}
