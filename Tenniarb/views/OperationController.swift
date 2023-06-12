//
//  OperationController.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 30/03/2019.
//  Copyright © 2019 Andrey Sobolev. All rights reserved.
//
//  Licensed under the Eclipse Public License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License. You may
//  obtain a copy of the License at https://www.eclipse.org/legal/epl-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//
//  See the License for the specific language governing permissions and
//  limitations under the License.

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
    
    override func viewDidAppear() {
        self.operationsTextBox.becomeFirstResponder()
//        self.view.window?.center()
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
            if cmdName.starts(with: "-") {
                let commandName = String(cmdName.suffix(from: cmdName.index(cmdName.startIndex, offsetBy: 1)))
                if newItemProps.removeNamed(commandName) {
                    changed = true
                }
                return
            }
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
