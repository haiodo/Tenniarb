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

    var activeElement: DiagramItem?
    
    var updateScheduled: Int = 0
    var updateKindScheduled: UpdateEventKind = .Layout
    
    var updateElements:[Element] = []
    
    var itemIndex = 0
    
    var outlineViewDelegate: OutlineViewControllerDelegate?
    var textViewDelegate: TextPropertiesDelegate?
    
    var updatingProperties: Bool = false
    
    @IBOutlet weak var toolsSegmentedControl: NSSegmentedControl!
        
    @IBAction func clickExtraButton(_ sender: NSSegmentedCell) {
        switch(sender.selectedSegment) {
        case 0: break;
        default: break;
        }
    }
    @IBOutlet weak var windowTitle: NSTextField!
    
    @IBAction func outlineTextChanged(_ sender: Any) {
        if let newValue = (sender as? NSTextField)?.stringValue, let active = selectedElement {
            elementModel?.operations.updateName(element: active, newValue, undoManager: self.undoManager, refresh: {() -> Void in } )
        }
    }
    
    override func viewDidLoad() {
        super.viewDidLoad()
        
        self.outlineViewDelegate = OutlineViewControllerDelegate(self)
        worldTree.delegate = self.outlineViewDelegate
        worldTree.dataSource = self.outlineViewDelegate
        
        self.textViewDelegate = TextPropertiesDelegate(self, self.textView!)
        
        scene.onLoad()
        
        if elementModel != nil && self.scene != nil {
            setElementModel(elementModel: elementModel!)
        }
    }
    
    @IBAction func elementToolbarAction(_ sender: NSSegmentedCell) {
        switch(sender.selectedSegment) {
        case 0: // This is add of new element.
            handleAddElement()
        case 1: // This is remove of selected element.
            handleRemoveElement()
        case 2: // This is options for selected element.
            handleElementOptions()
        default:
            break
        }
    }
    private func handleAddElement() {
        let newEl = Element(name: "Unnamed element: " + String(itemIndex))
        self.itemIndex += 1
        var active: Element?
        if let sel = self.selectedElement {
            active = sel
        }
        else {
            // Add root item
            active = self.elementModel
        }
        if let act = active {
//            act.add(newEl)
            elementModel?.operations.add(act, newEl, undoManager: self.undoManager, refresh: {()->Void in
                DispatchQueue.main.async(execute: {
                    if act.kind == .Root {
                        self.worldTree.reloadData()
                    }
                    else {
                        self.worldTree.reloadItem(act, reloadChildren: true )
                        self.worldTree.expandItem(act)
                    }
                })
            })
        }
        
    }
    private func handleRemoveElement() {
        if let active = self.selectedElement {
            if let parent = active.parent {
                parent.remove(active)
                
                if parent.kind == .Root {
                    selectedElement = nil
                }
                else {
                    selectedElement = parent
                }
                
                DispatchQueue.main.async(execute: {
                    if parent.kind == .Root {
                        self.worldTree.reloadData()
                    }
                    else {
                        self.worldTree.reloadItem(parent, reloadChildren: true)
                    }
                })

            }
        }
    }
    private func handleElementOptions() {
        
    }
    
    override var representedObject: Any? {
        didSet {
        // Update the view, if already loaded.
        }
    }
    
    override func presentViewController(_ viewController: NSViewController, asPopoverRelativeTo positioningRect: NSRect, of positioningView: NSView, preferredEdge: NSRectEdge, behavior: NSPopover.Behavior) {
        if let vc = viewController as? SourcePopoverViewController {
            if let active = self.selectedElement {
                vc.setElement(element: active)
                
                super.presentViewController(viewController, asPopoverRelativeTo: positioningRect , of: positioningView, preferredEdge: preferredEdge, behavior: behavior)
            }
        }
    }
    
    func onElementSelected(_ element: Element?) {
        if selectedElement != element {
            self.selectedElement = element
            self.activeElement = nil
            
            if let el = element {
                self.scene.setActiveElement(el)
            
                self.updateTextProperties()
            }
        }
    }
    
    func updateTextProperties( ) {
        if let element = self.selectedElement {
            DispatchQueue.main.async(execute: {
                let strContent = (self.activeElement == nil) ? element.toTennProps(): self.activeElement!.toTennProps()
                
                self.textViewDelegate?.setTextValue(strContent)
            })
        }
    }
    
    func updateWindowTitle() {
        let value = (self.elementModel?.modelName ?? "Unnamed model") + ((self.elementModel?.modified ?? true) ? "*":"")
        self.title = value
        self.windowTitle.stringValue = value
    }
    
    public func setElementModel(elementModel: ElementModel) {
        if let oldModel = self.elementModel {
            oldModel.onUpdate.removeAll()
        }
        self.elementModel = elementModel
        
        if let um = self.undoManager{
            um.removeAllActions()
        }
        if self.scene == nil {
            return
        }
        // Cleanup Undo stack
        if let um = self.undoManager {
            um.removeAllActions()
        }
        
        elementModel.onUpdate.append { (element, kind) in
            if self.updatingProperties {
                return
            }
            //TODO: Add optimizations based on particular element
            
            self.updateElements.append(element)
            if self.updateScheduled == 0 || (self.updateKindScheduled == .Layout && kind == .Structure ) {
                self.updateKindScheduled = kind
                self.updateScheduled = 1
                
                DispatchQueue.main.asyncAfter(deadline: .now() + 0.2, execute: {
                    
                    self.worldTree.beginUpdates()
                    for el in self.updateElements {
                        self.worldTree.reloadItem(el, reloadChildren: true)
                    }
                    self.updateElements.removeAll()
                    self.worldTree.endUpdates()
                    
                    //# Update text
                    self.updateTextProperties()
                    
                    self.updateScheduled = 0
                    
                    self.updateWindowTitle()
                })
            }
        }
        scene.setModel(model: elementModel)
        scene.onSelection.removeAll()
        scene.onSelection.append({( element ) -> Void in
            self.activeElement = element
            
            self.updateTextProperties()
        })

        scene.setActiveElement(elementModel)
        
        worldTree.reloadData()
        // Expand all top level elements
        
        self.updateWindowTitle()
        
        var firstChild:Element? = nil
        
        for e in elementModel.elements {
            if firstChild == nil {
                firstChild = e
            }
            worldTree.expandItem(e, expandChildren: true)
        }
//        if firstChild != nil {
//            let selectedIndex = worldTree.selectedRow
//            worldTree.
//        }
        
    }
    func mergeProperties(_ node: TennNode ) {
        updatingProperties = true
        if let active = activeElement {
            active.fromTennProps(node)
        }
        else if let element = self.selectedElement {
            element.fromTennProps(node)
        }
        updatingProperties = false
    }
}
