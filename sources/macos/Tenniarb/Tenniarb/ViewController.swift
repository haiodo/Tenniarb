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
    
    var elementStore: ElementModelStore?
    
    var selectedElement: Element?

    var activeElement: DiagramItem?
    
    var updateScheduled: Int = 0
    var updateKindScheduled: ModelEventKind = .Layout
    
    var updateElements:[Element] = []
    
    var itemIndex = 0
    
    var outlineViewDelegate: OutlineViewControllerDelegate?
    var textViewDelegate: TextPropertiesDelegate?
    
    var updatingProperties: Bool = false
    
    @IBOutlet weak var toolsSegmentedControl: NSSegmentedControl!
    
    var searchBox: SearchBoxViewController?
        
    @IBAction func clickExtraButton(_ sender: NSSegmentedCell) {
        switch(sender.selectedSegment) {
        case 0:
            self.scene.addNewItem()
        case 1:
            self.scene.removeItem()
        case 2:
            self.showElementSource()
        default: break;
        }
    }
    @IBOutlet weak var windowTitle: NSTextField!
    
    @IBAction func outlineTextChanged(_ sender: Any) {
        if let newValue = (sender as? NSTextField)?.stringValue, let active = selectedElement {
            self.elementStore?.updateName(element: active, newValue, undoManager: self.undoManager, refresh: {() -> Void in } )
        }
    }
    
    override func viewDidLoad() {
        super.viewDidLoad()
        
        self.outlineViewDelegate = OutlineViewControllerDelegate(self)
        worldTree.delegate = self.outlineViewDelegate
        worldTree.dataSource = self.outlineViewDelegate
        
        self.textViewDelegate = TextPropertiesDelegate(self, self.textView!)
        
        scene.onLoad()
        
        if elementStore != nil && self.scene != nil {
            setElementModel(elementStore: elementStore!)
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
    
    fileprivate func hideSearchBox() {
        if let sb = searchBox {
            if sb.view.window != nil {
                dismissViewController(sb)
            }
        }
    }
    
    @IBAction func showSearchBox(_ sender: NSMenuItem ) {
        if let active = self.selectedElement {
            hideSearchBox()
            
            self.searchBox = self.storyboard?.instantiateController(withIdentifier: NSStoryboard.SceneIdentifier(rawValue: "SearchBox")) as! SearchBoxViewController
            
            searchBox!.setElement(active)
            
            searchBox!.parentView = self.view
            
            searchBox!.closeAction = {() in self.hideSearchBox()}
            searchBox!.setActive = {(item) in self.scene.setActiveElement(item)}
            
            self.presentViewController(searchBox!, asPopoverRelativeTo: self.view.frame, of: self.view, preferredEdge: .maxX, behavior: .transient)
        }
    }
    
    private func showElementSource() {
        let popupController = self.storyboard?.instantiateController(withIdentifier: NSStoryboard.SceneIdentifier(rawValue: "SourcePopup")) as! SourcePopoverViewController
        
        let popover = NSPopover()
        popover.contentViewController = popupController
        popover.contentSize = popupController.view.frame.size
        
        popover.behavior = .transient
        popover.animates = false
        
        
        if let active = self.selectedElement {
            popupController.setElement(element: active)
        }
        
        self.presentViewController(popupController, asPopoverRelativeTo: self.toolsSegmentedControl.bounds, of: self.toolsSegmentedControl, preferredEdge: .maxY, behavior: .transient)
        
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
            active = self.elementStore?.model
        }
        if let act = active {
            self.elementStore?.add(act, newEl, undoManager: self.undoManager, refresh: {()->Void in
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
                _ = parent.remove(active)
                
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
                return
            }
        }
        
        super.presentViewController(viewController, asPopoverRelativeTo: positioningRect, of: positioningView, preferredEdge: preferredEdge, behavior: behavior)
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
        let value = (self.elementStore?.model.modelName ?? "Unnamed model") + ((self.elementStore?.modified ?? true) ? "*":"")
        self.title = value
        self.windowTitle.stringValue = value
    }
    
    public func setElementModel(elementStore: ElementModelStore) {
        if let oldStore = self.elementStore {
            oldStore.onUpdate.removeAll()
        }
        self.elementStore = elementStore
        
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
        
        elementStore.onUpdate.append { (evt) in
            if self.updatingProperties {
                return
            }
            //TODO: Add optimizations based on particular element
            
            self.updateElements.append(evt.element)
            if self.updateScheduled == 0 || (self.updateKindScheduled == .Layout && evt.kind == .Structure ) {
                self.updateKindScheduled = evt.kind
                self.updateScheduled = 1
                
                DispatchQueue.main.asyncAfter(deadline: .now() + 0.2, execute: {
                    
                    self.worldTree.beginUpdates()
                    
                    if self.selectedElement != evt.element {
                        self.onElementSelected(evt.element)
                    }
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
        scene.setModel(store: self.elementStore!)
        scene.onSelection.removeAll()
        scene.onSelection.append({( element ) -> Void in
            self.activeElement = element
            
            self.updateTextProperties()
        })

        scene.setActiveElement(elementStore.model)
        
        worldTree.reloadData()
        // Expand all top level elements
        
        self.updateWindowTitle()
        
        var firstChild:Element? = nil
        
        for e in elementStore.model.elements {
            if firstChild == nil {
                firstChild = e
            }
            worldTree.expandItem(e, expandChildren: true)
        }        
    }
    func mergeProperties(_ node: TennNode ) {
        updatingProperties = true
        if let active = activeElement {
            active.fromTennProps(self.elementStore!, node)
        }
        else if let element = self.selectedElement {
            element.fromTennProps(self.elementStore!,  node)
        }
        updatingProperties = false
    }
    
    
    
}
