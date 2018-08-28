//
//  ViewController.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 26/05/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//

import Cocoa

class ViewController: NSViewController, IElementModelListener {

    @IBOutlet weak var scene: SceneDrawView!
    
    @IBOutlet weak var worldTree: NSOutlineView!
    
    @IBOutlet var textView: NSTextView!
    
    var elementStore: ElementModelStore?
    
    var selectedElement: Element?

    var activeItems: [DiagramItem] = []
    
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
            let selectedRow = self.worldTree.selectedRow
            if let item = self.worldTree.item(atRow: selectedRow) as? Element {
                if item != active {
                    return; // Do not rename in this case
                }
                if item.name != newValue {
                    self.elementStore?.updateName(element: item, newValue, undoManager: self.undoManager, refresh: {() -> Void in } )
                }
            }
            
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
                dismiss(sb)
            }
        }
    }
    
    @IBAction func showSearchBox(_ sender: NSMenuItem ) {
        if let active = self.selectedElement {
            hideSearchBox()
            
            self.searchBox = self.storyboard?.instantiateController(withIdentifier: "SearchBox") as? SearchBoxViewController
            
            if let sb = searchBox {
                sb.setElement(active)
                
                sb.parentView = self.view
                
                sb.closeAction = {() in self.hideSearchBox()}
                sb.setActive = {(item) in self.scene.setActiveItem(item)}
                
                self.present(sb, asPopoverRelativeTo: self.view.frame, of: self.view, preferredEdge: .maxX, behavior: .transient)
            }
        }
    }
    
    @IBAction func selectAllItems(_ sender: NSMenuItem) {
        guard let responder = self.view.window?.firstResponder else {
            return
        }
        if responder  == self.scene {
            self.scene.selectAllItems()
        }
        else if responder == self.textView {
            self.textView.selectAll(sender)
        }
    }
    
    @IBAction func selectNoneItems(_ sender: NSMenuItem) {
        guard let responder = self.view.window?.firstResponder else {
            return
        }
        if responder == self.scene {
            self.scene.selectNoneItems()
        }
        else if responder == self.textView {
            self.textView.setSelectedRange(NSRange(location: 0, length: 0))
        }
    }
    
    @IBAction func addLinkedItem(_ sender: NSMenuItem ) {
        scene?.addNewItem()
    }
    
    @IBAction func addLinkedStyledItem(_ sender: NSMenuItem ) {
        scene?.addNewItem(copyProps: true)
    }
    
    @IBAction func addFreeItem(_ sender: NSMenuItem ) {
        scene?.addTopItem()
    }
    
    @IBAction func applyDefaultStyle(_ sender: NSMenuItem ) {
        
    }
    
    @IBAction func applyStyle( _ sender: NSMenuItem ) {
        guard let element = self.selectedElement else {
            return
        }
        if self.activeItems.count == 0 {
            return
        }
        guard let activeBounds = scene.getActiveItemBounds() else {
            return
        }
        let ctrl = self.storyboard?.instantiateController(withIdentifier: "StyleViewPopup")
        let popupController = ctrl  as! StyleViewController
        
        popupController.setElement(element: element)
        popupController.setActiveItems(self.activeItems)
        popupController.setViewController(self)
        
        let popover = NSPopover()
        popover.contentViewController = popupController
        popover.contentSize = popupController.view.frame.size
        
        popover.behavior = .transient
        popover.animates = false
        
        self.present(popupController, asPopoverRelativeTo: activeBounds, of: self.scene, preferredEdge: .maxX, behavior: .transient)
    }
    
    @IBAction func duplicateItem( _ sender: NSMenuItem ) {
        var target = 0
        if let responder = self.view.window?.firstResponder {
            if responder == self.worldTree {
                target = 1
            }
            else if responder == self.scene {
                target = 2
            }
            else {
                // Check if first responsed has super view of worldTree
                if let view = responder as? NSView {
                    
                    var p: NSView? = view
                    while p != nil {
                        if p == self.worldTree {
                            target = 1
                            break
                        }
                        p = p?.superview
                    }
                }
                
            }
        }
        
        switch target {
        case 1:
            if let active = self.selectedElement {
                let elementCopy = active.clone()
                
                self.elementStore?.add(active.parent!, elementCopy, undoManager: self.undoManager, refresh: {()->Void in
                    DispatchQueue.main.async(execute: {
                        if active.parent!.kind == .Root {
                            self.worldTree.reloadData()
                        }
                        else {
                            self.worldTree.reloadItem(active.parent!, reloadChildren: true )
                            self.worldTree.expandItem(active.parent!)
                        }
                    })
                })
            }
        case 2:
            scene?.duplicateItem()
        default:
            break
        }
    }
    
    private func showElementSource() {
        let popupController = self.storyboard?.instantiateController(withIdentifier: "SourcePopup") as! SourcePopoverViewController
        
        let popover = NSPopover()
        popover.contentViewController = popupController
        popover.contentSize = popupController.view.frame.size
        
        popover.behavior = .transient
        popover.animates = false
        
        
        if let active = self.selectedElement {
            popupController.setElement(element: active)
        }
        
        self.present(popupController, asPopoverRelativeTo: self.toolsSegmentedControl.bounds, of: self.toolsSegmentedControl, preferredEdge: .maxY, behavior: .transient)
        
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
    
    override func present(_ viewController: NSViewController, asPopoverRelativeTo positioningRect: NSRect, of positioningView: NSView, preferredEdge: NSRectEdge, behavior: NSPopover.Behavior) {
        
        if let vc = viewController as? SourcePopoverViewController {
            if let active = self.selectedElement {
                vc.setElement(element: active)
                
                super.present(viewController, asPopoverRelativeTo: positioningRect , of: positioningView, preferredEdge: preferredEdge, behavior: behavior)
                return
            }
        }
        
        if let vc = viewController as? ExportViewController {
            if let active = self.selectedElement {
                vc.setElement(element: active)
                vc.setScene(scene: self.scene.scene)
                vc.setViewController(self)
                
                super.present(viewController, asPopoverRelativeTo: positioningRect , of: positioningView, preferredEdge: preferredEdge, behavior: behavior)
                return
            }
        }
        if let vc = viewController as? SyncViewController {
            if let active = self.selectedElement {
                vc.setElement(element: active)
                vc.setViewController(self)
                
                super.present(viewController, asPopoverRelativeTo: positioningRect , of: positioningView, preferredEdge: preferredEdge, behavior: behavior)
                return
            }
        }
        
        super.present(viewController, asPopoverRelativeTo: positioningRect, of: positioningView, preferredEdge: preferredEdge, behavior: behavior)
    }
    
    func onElementSelected(_ element: Element?) {
        if selectedElement != element {
            self.selectedElement = element
            self.activeItems = []
            
            if let el = element {
                self.scene.setActiveElement(el)
            
                self.updateTextProperties()
            }
        }
    }
    
    func updateTextProperties( ) {
        if let element = self.selectedElement, let delegate = self.textViewDelegate {
            if delegate.needUpdate() {
                DispatchQueue.main.async(execute: {
                    
                    let strContent = (self.activeItems.count == 0) ? element.toTennProps(): self.activeItems[0].toTennProps()
                    
                    delegate.setTextValue(strContent)
                })
            }
        }
    }
    
    func updateWindowTitle() {
        let value = (self.elementStore?.model.modelName ?? "Unnamed model") + ((self.elementStore?.modified ?? true) ? "*":"")
        self.title = value
        self.windowTitle.stringValue = value
    }
    
    public func setElementModel(elementStore: ElementModelStore) {
        
        if let es = self.elementStore,  es.model == elementStore.model {
            return
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
        
        elementStore.onUpdate.append(self)

        scene.setModel(store: self.elementStore!)
        scene.onSelection.removeAll()
        scene.onSelection.append({( element ) -> Void in
            self.activeItems = element
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

    func notifyChanges(_ evt: ModelEvent) {
        if self.updatingProperties {
            self.updateWindowTitle()
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

    func mergeProperties(_ node: TennNode ) {
        updatingProperties = true
        if let active = activeItems.first {
            if let element = self.selectedElement {
                self.elementStore?.setProperties(element, active, node, undoManager: undoManager!, refresh: {()->Void in} )
            }
        }
        else if let element = self.selectedElement {
            self.elementStore?.setProperties(element, node, undoManager: undoManager!,  refresh: {()->Void in})
        }
        updatingProperties = false
    }
    
    
    
}
