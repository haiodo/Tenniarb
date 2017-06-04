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
    
    var elementModel:ElementModel?
    
    override func viewDidLoad() {
        super.viewDidLoad()
    }
    

    override var representedObject: Any? {
        didSet {
        // Update the view, if already loaded.
        }
    }
    
    public func setElementModel(elementModel: ElementModel) {
        self.elementModel = elementModel
        scene.setElementModel(elementModel)
        
        worldTree.reloadData()
        // Expand all top level elements
        for e in elementModel.elements.enumerated() {
            worldTree.expandItem(e, expandChildren: true)
        }
        
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
    
    func outlineViewSelectionDidChange(_ notification: Notification) {
        //1
        guard let outlineView = notification.object as? NSOutlineView else {
            return
        }
        //2
        let selectedIndex = outlineView.selectedRow
        if let el = outlineView.item(atRow: selectedIndex) as? Element {
            //3
            self.scene.setActiveElement(el)
        }
    }
    
    override func controlTextDidEndEditing(_ obj: Notification) {
        //1
        guard let outlineView = obj.object as? NSOutlineView else {
            return
        }
        //2
        let selectedIndex = outlineView.selectedRow
        if let el = outlineView.item(atRow: selectedIndex) as? Element {
            //3
//            outlineView.viewCe
            self.scene.setActiveElement(el)
        }

    }
    
}

