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
    
    @IBOutlet weak var textArea: NSScrollView!
    
    @IBOutlet var textView: NSTextView!
    
    var elementModel:ElementModel?
    
    var selectedElement: Element?
    var activeElement: Element?
    
    override func viewDidLoad() {
        super.viewDidLoad()
        
        scene.onSelection.append({( element ) -> Void in
//            self.setActiveElement(element)
        })
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
            textView.string = strContent
        }
    }
    
    public func setElementModel(elementModel: ElementModel) {
        self.elementModel = elementModel
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
        //1
        if let el = item as? Element {
            return el.elements.count
        }
        //2
        if let em = elementModel {
            return em.elements.count
        }
        return 0
    }
    func outlineView(_ outlineView: NSOutlineView, child index: Int, ofItem item: Any?) -> Any {
        if let el = item as? Element {
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

