//
//  ViewController.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 26/05/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//

import Cocoa

class ViewController: NSViewController, IElementModelListener, NSMenuItemValidation {

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
    
    var operationBox: OperationController?
    
    var exportMgr = ExportManager()
        
    @IBOutlet weak var exportSegments: NSSegmentedCell!
    @IBAction func clickExtraButton(_ sender: NSSegmentedCell) {
        switch(sender.selectedSegment) {
        case 0:
            self.scene.addNewItem()
        case 1:
            self.scene.removeItem()
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
        
        scene.onLoad(self)
        
        if elementStore != nil && self.scene != nil {
            setElementModel(elementStore: elementStore!)
        }
        
        exportMgr.setViewController(self)
        
        let exportMenu = exportMgr.createMenu()
        
        exportSegments.setMenu(exportMenu, forSegment: 0)
        
        DistributedNotificationCenter.default().addObserver(self, selector: #selector(darkModeChanged), name: NSNotification.Name(rawValue: "AppleInterfaceThemeChangedNotification"), object: nil)
    }
    
    override func viewDidAppear() {
        scene.onAppear()
    }
    
    @objc func darkModeChanged(_ notif: NSNotification) {
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.2, execute: {
            self.scene.scheduleRedraw()
            if let delegate = self.textViewDelegate {
                delegate.highlight()
            }
        })
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
        hideView(searchBox)
        searchBox = nil
    }
    
    fileprivate func hideView(_ controller: NSViewController? ) {
        if let c = controller {
            if c.view.window != nil {
                dismiss(c)
            }
        }
    }
    func hideOperationBox() {
        hideView(operationBox)
        operationBox = nil
    }
    
    @IBAction func showSearchBox(_ sender: NSMenuItem ) {
        if let active = self.selectedElement {
            hideSearchBox()
            
            self.searchBox = self.storyboard?.instantiateController(withIdentifier: "SearchBox") as? SearchBoxViewController
            
            if let sb = searchBox {
                sb.setElement(active)
                
                sb.parentView = self.view
                
                sb.closeAction = {() in self.hideSearchBox()}
                sb.setActive = { (item) in
                    self.scene.setActiveItem(item)
                    self.scene.centerItem(item, 120)
                }
                
                self.present(sb, asPopoverRelativeTo: self.view.frame, of: self.view, preferredEdge: .maxX, behavior: .transient)
            }
        }
    }
    
    @IBAction func showOperationBox(_ sender: NSMenuItem ) {
        showOperationBox()
    }
    func showOperationBox() {
        if self.activeItems.count > 0, let element = self.selectedElement, let store = self.elementStore {
            hideOperationBox()
            
            self.operationBox = self.storyboard?.instantiateController(withIdentifier: "operationBox") as? OperationController
            
            if let operartions = operationBox {
                operartions.setController(self)
                operartions.setStore(store)
                operartions.setElement(element)
                operartions.setItems(self.activeItems)
                
                // Get a
                
                let bounds = self.scene!.getSelectionBounds()
                
                self.present(operartions, asPopoverRelativeTo: NSRect(origin: bounds.origin, size: bounds.size),
                             of: self.scene, preferredEdge: .minY, behavior: .transient)
            }
        }
    }
    
    @IBAction func selectAllItems(_ sender: NSMenuItem) {
        guard let responder = self.view.window?.firstResponder else {
            return
        }
        if responder  == self.scene {
            self.scene.selectAllItems()
            return
        }
        else if responder == self.textView {
            self.textView.selectAll(sender)
            return
        }
        super.selectAll(sender)
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
    
    @IBAction func editTitle(_ sender: NSMenuItem ) {
        if let active = self.scene?.activeItems.first  {
            scene.setActiveItem(active)
            scene?.editTitle(active, .Name)
        }
    }
    @IBAction func editBody(_ sender: NSMenuItem ) {
        if let active = self.scene?.activeItems.first  {
            scene.setActiveItem(active)
            scene?.editTitle(active, .Body)
        }
    }
    
    @IBAction func editValue(_ sender: NSMenuItem ) {
           if let active = self.scene?.activeItems.first  {
               scene.setActiveItem(active)
               scene?.editTitle(active, .Value)
           }
       }
    
    @IBAction func quickEdit(_ sender: NSMenuItem ) {
        showOperationBox()
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
    
    func validateMenuItem(_ menuItem: NSMenuItem) -> Bool {
        if let action = menuItem.action {
            if action == #selector(selectAllItems(_:)) {
                if self.view.window?.firstResponder == self.scene {
                    return true
                }
            }
            if action == #selector(self.showSearchBox(_:)) {
                return true
            }
            if action == #selector(self.editTitle(_:)) || action == #selector(self.editBody(_:)) || action == #selector(self.editValue(_:)) ||
                action == #selector(self.showOperationBox(_:)) {
                return !self.scene.activeItems.isEmpty
            }
            if action == #selector(duplicateItem) || action == #selector(inheritItem) {
                switch findTarget() {
                case .WorldTree:
                    return true
                case .SceneView:
                    return !self.scene.activeItems.isEmpty
                default:
                    break
                }
            }
        }
        return false
    }
    
    enum FindTargetResult {
        case Unknown
        case WorldTree
        case SceneView
    }
    
    fileprivate func findTarget() -> FindTargetResult {
        var target: FindTargetResult = .Unknown
        if let responder = self.view.window?.firstResponder {
            if responder == self.worldTree {
                target = .WorldTree
            }
            else if responder == self.scene {
                target = .SceneView
            }
            else {
                // Check if first responsed has super view of worldTree
                if let view = responder as? NSView {
                    
                    var p: NSView? = view
                    while p != nil {
                        if p == self.worldTree {
                            target = .WorldTree
                            break
                        }
                        p = p?.superview
                    }
                }
                
            }
        }
        return target
    }
    
    @IBAction public func duplicateItem( _ sender: NSMenuItem ) {
        switch findTarget() {
        case .WorldTree:
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
        case .SceneView:
            scene?.duplicateItem()
        default:
            break
        }
    }
    
    @IBAction public func inheritItem( _ sender: NSMenuItem ) {
        switch findTarget() {
        case .WorldTree,.SceneView:
            if let active = self.selectedElement {
                let elementCopy = active.clone()
                
                let refs = Element.prepareItemRefs(elementCopy.items)
                
                for itm in elementCopy.items {
                    // Make clean propertirs
                    itm.properties = ModelProperties()
                    
                    // Calculate index of item
                    let cmd = TennNode.newCommand(PersistenceStyleKind.Inherit.name,TennNode.newStrNode("../\(itm.name)"))
                    if let ind = refs[itm] {
                        cmd.add(TennNode.newIntNode(ind))
                    }
                    itm.properties.append(cmd)
                }
                
                self.elementStore?.add(active, elementCopy, undoManager: self.undoManager, refresh: {()->Void in
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
        default:
            break
        }
    }

    public func handleAddElement() {
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
    public func handleRemoveElement() {
        if let active = self.selectedElement {
            if let parent = active.parent {
//                _ = parent.remove(active)
                
                if parent.kind == .Root {
                    selectedElement = nil
                }
                else {
                    selectedElement = parent
                }
                
                self.elementStore?.remove(parent, active, undoManager: self.undoManager, refresh: {()->Void in
                    DispatchQueue.main.async(execute: {
                        if parent.kind == .Root {
                            self.worldTree.reloadData()
                        }
                        else {
                            self.worldTree.reloadItem(parent, reloadChildren: true )
                            self.worldTree.expandItem(parent)
                        }
                    })
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
            DispatchQueue.main.async(execute: {
                delegate.setTextValue(element, self.activeItems.first)
            })
        }
    }
    
    func updateWindowTitle() {
        var value = (self.elementStore?.model.modelName ?? "Unnamed model")
        if value.hasSuffix(".tenn") {
            value = String(value[value.startIndex..<value.index(value.endIndex, offsetBy: -5)])
        }
        if self.elementStore?.modified ?? false {
           value += "*"
        }
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
                
        if PreferenceConstants.preference.autoExpand {
            self.expandItems(elementStore.model.elements, PreferenceConstants.preference.autoExpandLevel)
        }
    }
    
    
    func expandItems(_ elements: [Element], _ level: Int) {
        for e in elements {
            worldTree.expandItem(e, expandChildren: false)
            if level > 0 {
                expandItems(e.elements, level - 1)
            }
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
                
                // Check if operation added some items and in this case select it.
                var selectionUpdated = false
                for (el, kind) in evt.elements {
                    if kind == .Append {
                        if self.selectedElement != el {
                            self.onElementSelected(el)
                            selectionUpdated = true
                        }
                    }
                }
                if !selectionUpdated {
                    if self.selectedElement != evt.element {
                        self.onElementSelected(evt.element)
                    }
                }
                for el in self.updateElements {
                    self.worldTree.reloadItem(el, reloadChildren: true)
                }
                self.updateElements.removeAll()
                
                self.worldTree.endUpdates()
                
                if let sel = self.selectedElement {
                    let childIndex = self.worldTree.row(forItem: sel)
                    self.worldTree.selectRowIndexes(IndexSet.init(arrayLiteral: childIndex),
                                                    byExtendingSelection: false)
                }
                                
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
