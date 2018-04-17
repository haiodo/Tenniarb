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

class OutlineViewControllerDelegate: NSObject, NSOutlineViewDataSource, NSOutlineViewDelegate {
    let controller: ViewController
    var draggingItem: Element? = nil
    
    init(_ controller: ViewController ) {
        self.controller = controller
        controller.worldTree.registerForDraggedTypes([NSPasteboard.PasteboardType.string])
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
                    if el.itemCount > 0 {
                        imageField.image = NSImage.init(named: NSImage.Name.init("small_logo_white"))
                    }
                    else {
                        imageField.image = NSImage.init(named: NSImage.Name.init("element_logo_white"))
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
    
    func outlineView(_ outlineView: NSOutlineView, validateDrop info: NSDraggingInfo, proposedItem item: Any?, proposedChildIndex index: Int) -> NSDragOperation {
        
        if let element = item as? Element, let dragItem = self.draggingItem {
            // Check if not moving item into one of its parents.
//            if element == dragItem {
//                return NSDragOperation.
//            }
//
            print( "validate: \(element.name) at \(index)")
        }
        
        
        
        return NSDragOperation.move
        //
    }
    
    func outlineView(_ outlineView: NSOutlineView, acceptDrop info: NSDraggingInfo, item: Any?, childIndex index: Int) -> Bool {
        
        print( "accept drop: \((item as? Element)?.name) at \(index)")
        draggingItem = nil
        return true
    }
    
}

