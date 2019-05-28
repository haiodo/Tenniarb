//
//  SearchBoxController.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 17/04/2018.
//  Copyright © 2018 Andrey Sobolev. All rights reserved.
//

import Foundation
import Cocoa


class SearchBoxViewController: NSViewController, NSTextFieldDelegate, NSTextViewDelegate, NSPopoverDelegate {
    
    @IBOutlet weak var searchBox: NSTextField!
    
    var parentView: NSView?
    
    var element: Element?
    
    var changes:Int = 0
    
    var currentItems: [DiagramItem] = []
    
    var closeAction: (() -> Void)?
    var setActive: ((_ item: DiagramItem) -> Void)?
    
    var searchResultDelegate: SearchBoxResultDelegate?
    
    @IBOutlet weak var resultView: NSOutlineView!
    override func viewDidLoad() {
        self.searchBox.delegate = self
        let delegate = SearchBoxResultDelegate(self)
        self.searchResultDelegate = delegate
        resultView.delegate = delegate
        resultView.dataSource = delegate
    }
    
    func setElement(_ element: Element) {
        self.element = element
    }
    
    //    func windowDidResignMain(_ notification: Notification) {
    //        close()
    //    }
    
    override func viewWillAppear() {
        self.view.window?.hidesOnDeactivate = true
        
        //        self.searc
        self.searchBox.becomeFirstResponder()
        
        self.view.window?.center()
    }
    
    func control(_ control: NSControl, textView: NSTextView, doCommandBy commandSelector: Selector) -> Bool {
        if commandSelector == #selector(NSView.cancelOperation(_:)) {
            close()
            return true
        }
        if commandSelector == #selector(NSView.insertNewline(_:)) {
            self.close()
            return true
        }
        
//        if commandSelector == #selector(NSView.keyDown(with:)) {
//            return true
//        }
        
        if commandSelector == #selector(NSView.moveUp(_:)) {
            let row = self.resultView.selectedRow
            if row > 0 {
                self.resultView.selectRowIndexes(NSIndexSet(index: row - 1) as IndexSet, byExtendingSelection: false)
                self.resultView.scrollRowToVisible(row - 1 )
            } else {
                self.resultView.selectRowIndexes(NSIndexSet(index: self.currentItems.count - 1) as IndexSet, byExtendingSelection: false)
                self.resultView.scrollRowToVisible( self.currentItems.count - 1 )
            }
            
            return true
        }
        if commandSelector == #selector(NSView.moveDown(_:)) {
            let row = self.resultView.selectedRow
            if row + 1 < self.currentItems.count {
                self.resultView.selectRowIndexes(NSIndexSet(index: row + 1) as IndexSet, byExtendingSelection: false)
                self.resultView.scrollRowToVisible(row + 1 )
            } else {
                self.resultView.selectRowIndexes(NSIndexSet(index: 0) as IndexSet, byExtendingSelection: false)
                self.resultView.scrollRowToVisible(0 )
            }
            return true
        }
        
        // TODO: Resize both text and drawed item to fit value smoothly.
        
        return false
    }
    
    override func selectAll(_ sender: Any?) {
        self.searchBox.selectAll( sender )
    }
    
    func controlTextDidChange(_ notification: Notification) {
        changes += 1
        sheduleUpdate()
    }
    
    fileprivate func sheduleUpdate( ) {
        let curChanges = self.changes
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.3, execute: {
            if curChanges == self.changes {
                let textContent = self.searchBox.stringValue
                
                if let el = self.element {
                    self.currentItems = el.items.filter({(item) in
                        
                        let nameMatched = item.name.lowercased().contains(textContent.lowercased())
                        
                        var textValue = ""
                        SceneDrawView.getBodyText(item, nil , &textValue)
                        
                        let bodyMatched = textValue.lowercased().contains(textContent.lowercased())
                        
                        return nameMatched || bodyMatched
                    }).sorted(by: {(a,b) in a.name.lexicographicallyPrecedes(b.name)})
                    
                    self.resultView.reloadData()
                    if self.currentItems.count > 0 {
                        self.resultView.selectRowIndexes(NSIndexSet(index: 0) as IndexSet, byExtendingSelection: false)
                    }
                }
                
                self.view.needsDisplay = true
            }
        })
    }
    
    override func keyDown(with event: NSEvent) {
    }
    
    @IBAction func applyClose(_ sender: NSButton) {
        close()
    }
    func close() {
        if let cl = self.closeAction {
            cl()
        }
    }
}


class SearchBoxResultDelegate: NSObject, NSOutlineViewDataSource, NSOutlineViewDelegate {
    let controller: SearchBoxViewController
    init(_ controller: SearchBoxViewController ) {
        self.controller = controller
    }
    
    func outlineView(_ outlineView: NSOutlineView, numberOfChildrenOfItem item: Any?) -> Int {
        if item ==  nil {
            return self.controller.currentItems.count
        }
        return 0
    }
    func outlineView(_ outlineView: NSOutlineView, child index: Int, ofItem item: Any?) -> Any {
        if item == nil {
            return controller.currentItems[index]
        }
        return ""
    }
    
    func outlineView(_ outlineView: NSOutlineView, isItemExpandable item: Any) -> Bool {
        return false
    }
    
    
    func outlineView(_ outlineView: NSOutlineView, objectValueFor tableColumn: NSTableColumn?, byItem item: Any?) ->  Any? {
        //1
        if let el = item as? DiagramItem {
            return el.name
        }
        return nil
    }
    
    func outlineView(_ outlineView: NSOutlineView, viewFor viewForTableColumn: NSTableColumn?, item: Any) -> NSView? {
        if let el = item as? DiagramItem {
            if let view = outlineView.makeView(withIdentifier: NSUserInterfaceItemIdentifier(rawValue: "SearchItemCell"), owner: self) as? NSTableCellView {
                if let textField = view.textField {
                    
                    var textValue = ""
                    SceneDrawView.getBodyText(el, nil , &textValue)
                    
                    if textValue.count > 0 {
                        textField.stringValue = (el.name + " - " + textValue ) .replacingOccurrences(of: "\n", with: "\\n")
                    } else {
                        textField.stringValue = el.name.replacingOccurrences(of: "\n", with: "\\n")
                    }
                }
                return view
            }
        }
        return nil
    }
    
    //    func outlineView(_ outlineView: NSOutlineView, rowViewForItem item: Any) -> NSTableRowView? {
    //        return OutlineNSTableRowView()
    //    }
    
    @objc func outlineViewSelectionDidChange(_ notification: Notification) {
        
        let selectedIndex = controller.resultView.selectedRow
        if let el = controller.resultView.item(atRow: selectedIndex) as? DiagramItem {
            self.controller.setActive!(el)
        }
    }
}
