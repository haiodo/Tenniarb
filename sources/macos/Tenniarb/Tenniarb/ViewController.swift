//
//  ViewController.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 26/05/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//

import Cocoa

class ViewController: NSViewController {

    @IBOutlet weak var scene: SceneDrawView!
    
    @IBOutlet weak var worldTree: NSOutlineView!
    
    @IBOutlet var textView: NSTextView!
    
    var elementModel:ElementModel?
    
    var selectedElement: Element?
    var activeElement: Element?
    
    var updateScheduled: Int = 0
    
    override func viewDidLoad() {
        super.viewDidLoad()
        
        scene.onSelection.append({( element ) -> Void in
//            self.setActiveElement(element)
        })
        
        if elementModel != nil && self.scene != nil {
            setElementModel(elementModel: elementModel!)
        }
    }
    

    override var representedObject: Any? {
        didSet {
        // Update the view, if already loaded.
        }
    }
    
    func onElementSelected(_ element: Element?) {
        if selectedElement != element {
            self.selectedElement = element
            
            if let el = element {
                self.scene.setElementModel(el)
            
                self.setActiveElement(el)
            }
        }
    }
    
    func setActiveElement( _ element: Element? ) {
        if let el = element {
            self.activeElement = el
        }
        else {
            self.activeElement = selectedElement
        }
        if let e = self.activeElement {
            let strContent = e.toTennStr()
            
            let style = NSMutableParagraphStyle()
            style.headIndent = 50
            style.alignment = .justified
            style.firstLineHeadIndent = 50
            
            textView.font = NSFont.systemFont(ofSize: 15.0)
            
            textView.string = strContent
        }
    }
    
    public func setElementModel(elementModel: ElementModel) {
        self.elementModel = elementModel
        if self.scene == nil {
            return
        }
        
        elementModel.onUpdate.append {
            //TODO: Add optimizations based on particular element
            
            if self.updateScheduled == 0 {
                self.updateScheduled = 1
                DispatchQueue.main.asyncAfter(deadline: .now() + 0.2, execute: {
                    self.worldTree.reloadData()
                    //# Update text
                    self.setActiveElement(self.activeElement)
                    
                    self.updateScheduled = 0
                })
            }
        }
        scene.setElementModel(elementModel)
        
        worldTree.reloadData()
        // Expand all top level elements
        
        var firstChild:Element? = nil
        
        for e in elementModel.elements {
            if firstChild == nil {
                firstChild = e
            }
            worldTree.expandItem(e, expandChildren: false)
        }
//        if firstChild != nil {
//            let selectedIndex = worldTree.selectedRow
//            worldTree.
//        }
        
    }
}

extension ViewController: NSOutlineViewDataSource {
    
    
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
        if let em = elementModel {
            return em.elements.count
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
        
        if let em = elementModel {
            return em.elements[index]
        }
        return ""
    }
    func outlineView(_ outlineView: NSOutlineView, isItemExpandable item: Any) -> Bool {
        if let el = item as? Element {
            return el.elements.count > 0
        }
        
        return false
    }
    
    // Using "Cell Based" content mode
    func outlineView(_ outlineView: NSOutlineView, objectValueFor tableColumn: NSTableColumn?, byItem item: Any?) ->  Any? {
        //1
        if let el = item as? Element {
            return el.name
        }
        return nil
    }
    
    
    @objc func outlineViewSelectionDidChange(_ notification: Notification) {
        
        let selectedIndex = worldTree.selectedRow
        if let el = worldTree.item(atRow: selectedIndex) as? Element {
            self.onElementSelected(el)
        }
        else {
            self.onElementSelected(elementModel)
        }
    }
    
    override func controlTextDidEndEditing(_ obj: Notification) {
        let selectedIndex = worldTree.selectedRow
        let cell = worldTree.selectedCell() as? NSTextFieldCell
        
        let strValue = cell?.stringValue
        if let el = worldTree.item(atRow: selectedIndex) as? Element {
            if let str = strValue  {
                el.name = str
                scene.needsDisplay = true
            }
        }

    }
    
}

