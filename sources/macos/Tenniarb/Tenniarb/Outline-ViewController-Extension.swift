//
//  OutlineViewController.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 06/01/2018.
//  Copyright © 2018 Andrey Sobolev. All rights reserved.
//

import Foundation
import Cocoa

class OutlineTextFieldCell: NSTextFieldCell {
    static let myTextColor: NSColor = NSColor(red: 253/255, green: 246/255, blue: 227/255, alpha: 1.0)
    
    override func edit(withFrame rect: NSRect, in controlView: NSView, editor textObj: NSText, delegate: Any?, event: NSEvent?) {
        super.edit(withFrame: rect, in: controlView, editor: textObj, delegate: delegate, event: event)
        textObj.textColor = NSColor.black
        textObj.backgroundColor = NSColor.white
    }
    override func select(withFrame rect: NSRect, in controlView: NSView, editor textObj: NSText, delegate: Any?, start selStart: Int, length selLength: Int) {
        super.select(withFrame: rect, in: controlView, editor: textObj, delegate: delegate, start: selStart, length: selLength)
        textObj.textColor = NSColor.black
        textObj.backgroundColor = NSColor.white
    }
    
}
class OutlineNSTableRowView: NSTableRowView {
    override func drawSelection(in dirtyRect: NSRect) {
        if self.selectionHighlightStyle != .none {
            let selectionRect = NSInsetRect(self.bounds, 1.5, 1.5)
            NSColor(calibratedWhite: 0.35, alpha: 1).setStroke()
            NSColor(calibratedWhite: 0.62, alpha: 1).setFill()
            let selectionPath = NSBezierPath.init(roundedRect: selectionRect, xRadius: 6, yRadius: 6)
            selectionPath.fill()
            selectionPath.stroke()
        }
    }
}

class OutlineNSOutlineView: NSOutlineView, NSMenuItemValidation, NSMenuDelegate {
    func validateMenuItem(_ menuItem: NSMenuItem) -> Bool {
        return true
    }
    override func keyDown(with event: NSEvent) {
        if let delegate = self.delegate as? OutlineViewControllerDelegate {
            if delegate.keyDown(for: event, self) {
               return
            }
        }
        super.keyDown(with: event)
    }
    override func editColumn(_ column: Int, row: Int, with event: NSEvent?, select: Bool) {
        if let evt = event, evt.characters == "\u{0D}" {
            super.editColumn(column, row: row, with: event, select: select)
        }
    }
}

class OutlineViewControllerDelegate: NSObject, NSOutlineViewDataSource, NSOutlineViewDelegate,NSMenuItemValidation {
    let controller: ViewController
    var draggingItem: Element? = nil
    
    init(_ controller: ViewController ) {
        self.controller = controller
        controller.worldTree.registerForDraggedTypes([NSPasteboard.PasteboardType.string])
        super.init()
        controller.worldTree.menu = createMenu()
    }
    
    func outlineView(_ outlineView: NSOutlineView, shouldEdit tableColumn: NSTableColumn?, item: Any) -> Bool {
        return false
    }
    
    func control(_ control: NSControl, textShouldBeginEditing fieldEditor: NSText) -> Bool {
        Swift.debugPrint("HI")
        return true
    }
    
    func keyDown( for event: NSEvent, _ outline: OutlineNSOutlineView ) -> Bool {
        if event.characters == "\u{0D}" {
            let selectedIndex = controller.worldTree.selectedRow
            
            if let el = controller.worldTree.item(atRow: selectedIndex) as? Element {
                outline.editColumn(0, row: selectedIndex, with: event, select: true)
                return true
            }
        }
        return false
    }
    @objc  func addElementAction(_ sender: NSMenuItem) {
        controller.handleAddElement()
    }
    
    @objc  func duplicateElementAction(_ sender: NSMenuItem) {
        controller.duplicateItem(sender)
    }
    
    @objc  func deleteElementAction(_ sender: NSMenuItem) {
        controller.handleRemoveElement()
    }
    
     func validateMenuItem(_ menuItem: NSMenuItem) -> Bool {
        let selectedIndex = controller.worldTree.selectedRow
        
        if let el = controller.worldTree.item(atRow: selectedIndex) as? Element {
            if el.kind == .Root && menuItem.action == #selector(deleteElementAction) {
                return false
            }
            return true
        }
        return false
    }
    
    func createMenu() -> NSMenu? {
        Swift.debugPrint("Menu")
        
        let menu = NSMenu()
        let addAction = NSMenuItem(
            title: "New element", action: #selector(addElementAction), keyEquivalent: "")
        addAction.target = self
        
        let duplicateAction = NSMenuItem(
            title: "Duplicate", action: #selector(duplicateElementAction), keyEquivalent: "")
        
        duplicateAction.target = self
        
        let deleteAction = NSMenuItem(
            title: "Delete", action: #selector(deleteElementAction), keyEquivalent: "")
        deleteAction.target = self
        menu.addItem(addAction)
        menu.addItem(duplicateAction)
        menu.addItem(NSMenuItem.separator())
        menu.addItem(deleteAction)
        return menu
    }
    
    func outlineView(_ outlineView: NSOutlineView, numberOfChildrenOfItem item: Any?) -> Int {
        // Elements and diagram items
        if let el = item as? Element {
            var count = 0
            for e in el.elements {
                if e.kind == .Element {
                    count += 1
                }
            }
            return count
        }
        // Root has only elements
        if let es = controller.elementStore {
            return es.model.elements.count
        }
        return 0
    }
    func outlineView(_ outlineView: NSOutlineView, child index: Int, ofItem item: Any?) -> Any {
        if let el = item as? Element {
            var i = 0
            for e in el.elements {
                if e.kind == .Element {
                    if i == index {
                        return e
                    }
                    i += 1
                }
            }
            if el.elements.count <= index {
                return ""
            }
            return el.elements[index]
        }
        
        if let es = controller.elementStore {
            return es.model.elements[index]
        }
        return ""
    }
    func outlineView(_ outlineView: NSOutlineView, isItemExpandable item: Any) -> Bool {
        if let el = item as? Element {
            return el.elements.count > 0
        }
        
        return false
    }
    
    
    func outlineView(_ outlineView: NSOutlineView, objectValueFor tableColumn: NSTableColumn?, byItem item: Any?) ->  Any? {
        //1
        if let el = item as? Element {
            return el.name
        }
        return nil
    }
    
    //    func outlineView(_ outlineView: NSOutlineView, isGroupItem item: Any) -> Bool {
    //        if let el = item as? Element {
    //            return el.elements.count > 0
    //        }
    //        return false
    //    }
    
    func outlineView(_ outlineView: NSOutlineView, viewFor viewForTableColumn: NSTableColumn?, item: Any) -> NSView? {
        if let el = item as? Element {
            if let view = outlineView.makeView(withIdentifier: NSUserInterfaceItemIdentifier(rawValue: "ItemCell"), owner: self) as? NSTableCellView {
                if let textField = view.textField {
                    textField.stringValue = el.name
                }
                
                if let imageField = view.viewWithTag(0) as? NSImageView {
                    if el.items.count > 0 {
                        imageField.image = NSImage.init(named: "ico-group")
                    }
                    else {
                        imageField.image = NSImage.init(named: "ico-component")
                    }
                }
                return view
            }
        }
        return nil
    }
    func outlineView(_ outlineView: NSOutlineView, rowViewForItem item: Any) -> NSTableRowView? {
        return OutlineNSTableRowView()
    }
    
    
    @objc func outlineViewSelectionDidChange(_ notification: Notification) {
        let selectedIndex = controller.worldTree.selectedRow

        if let el = controller.worldTree.item(atRow: selectedIndex) as? Element {
            self.controller.onElementSelected(el)
        }
        else {
            self.controller.onElementSelected(controller.elementStore?.model)
        }
    }
    
    func outlineView(_ outlineView: NSOutlineView, pasteboardWriterForItem item: Any) -> NSPasteboardWriting? {
        
        let pp = NSPasteboardItem()
        
        // working as expected here
        if let fi = item as? Element {
            if fi.kind == .Root {
                return nil
            }
            draggingItem = fi
            pp.setString( fi.toTennStr(), forType: NSPasteboard.PasteboardType.string )
        }
        
        return pp
    }
    
    func isParentOf(_ rootElement: Element, _ element: Element) -> Bool {
        var e:Element? = element
        
        while e != nil {
            if e == rootElement {
                return true
            }
            e = e!.parent
        }
        return false
    }
    
    func outlineView(_ outlineView: NSOutlineView, validateDrop info: NSDraggingInfo, proposedItem item: Any?, proposedChildIndex index: Int) -> NSDragOperation {
        
        if let element = item as? Element, let dragItem = self.draggingItem {
            // Check if not moving item into one of its parents.
            
            if self.isParentOf(dragItem, element) {
                return NSDragOperation.copy
            }
        }
        
        return NSDragOperation.move
        //
    }
    
    func outlineView(_ outlineView: NSOutlineView, acceptDrop info: NSDraggingInfo, item: Any?, childIndex index: Int) -> Bool {
        
        if let dragItem = self.draggingItem {
            if let element = item as? Element {
                // Check if not moving item into one of its parents.
                
                if self.isParentOf(dragItem, element) {
                    // Do copy of element diagram only
                    let diCopy = dragItem.clone(cloneItems: true, cloneElement: false)
                    
                    controller.elementStore?.add(element, diCopy, undoManager: controller.undoManager, refresh:{() in self.controller.worldTree.reloadItem(element)}, index: index)
                    
                }
                else {
                    // Do move of element
                    controller.elementStore?.move(dragItem, element, undoManager: controller.undoManager, refresh:{() in self.controller.worldTree.reloadData()}, index: index)
                }
            }
            else {
                controller.elementStore?.move(dragItem, controller.elementStore!.model, undoManager: controller.undoManager, refresh:{() in self.controller.worldTree.reloadData()}, index: index)
            }
        }
        
        
        draggingItem = nil
        return true
    }
}

