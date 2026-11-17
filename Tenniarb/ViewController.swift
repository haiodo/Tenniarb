//
//  ViewController.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 26/05/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//
//  Licensed under the Eclipse Public License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License. You may
//  obtain a copy of the License at https://www.eclipse.org/legal/epl-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//
//  See the License for the specific language governing permissions and
//  limitations under the License.

import Cocoa
import Tenn
import UniformTypeIdentifiers

class ViewController: NSViewController, IElementModelListener, NSMenuItemValidation, NSSplitViewDelegate {

    @IBOutlet weak var scene: SceneDrawView!

    @IBOutlet weak var worldTree: NSOutlineView!

    @IBOutlet var textView: NSTextView!

    var elementStore: ElementModelStore?

    /// The tree of the document in this window; nil for a classic one.
    private var sceneTree: Scene.Tree?

    /// The document this window shows, for the scene canvas to write its edits through.
    weak var document: Document?

    /// The comments of the document this window shows, and the pins of its element on the canvas.
    private var commentsController: CommentsController?
    /// The file the comments controller was made for, so Save As makes a new one.
    private var commentsForURL: URL?

    /// Compare mode of the window, while it is on, and the report rows it puts in the outline.
    private var compareController: CompareController?
    private var compareOutlineSource: CompareOutlineSource?

    /// The sheet of Insert Component…, held while it is open.
    private var componentPicker: ComponentPickerController?
    /// What the slot menu that follows Insert Component… writes and where.
    private var pendingComponent: Scene.Component?
    private var pendingInto: Scene.Node?
    private var pendingSlots: [String] = []
    private weak var pendingCanvas: SceneCanvasView?
    private var pendingCanvasNode: Scene.Node?

    var selectedElement: Element?

    var activeItems: [DiagramItem] = []

    /// The selections Back and Forward walk in this window, and whether one is being shown again.
    private(set) var navigation = NavigateHistory()
    private var restoringNavigation = false

    var updateScheduled: Int = 0
    var updateKindScheduled: ModelEventKind = .Layout

    var updateElements: [Element] = []

    var itemIndex = 0

    var outlineViewDelegate: OutlineViewControllerDelegate?
    var textViewDelegate: TextPropertiesDelegate?

    var updatingProperties: Bool = false

    @IBOutlet weak var toolsSegmentedControl: NSSegmentedControl!

    var searchBox: SearchBoxViewController?

    var operationBox: OperationController?

    var exportMgr = ExportManager()

    @IBOutlet weak var exportSegments: NSSegmentedCell!

    // MARK: - Component Controllers
    var outlineSidebarController: OutlineSidebarController!
    var editorViewController: EditorViewController!
    var propertiesPanelController: PropertiesPanelController!
    var inspectorController: InspectorController!
    @IBAction func clickExtraButton(_ sender: NSSegmentedCell) {
        switch sender.selectedSegment {
        case 0:
            self.scene.addNewItem()
        case 1:
            self.scene.removeItem()
        default: break;
        }
    }

    @IBOutlet weak var windowTitle: NSTextField!

    weak var zoomLabel: NSTextField!

    @IBAction func outlineTextChanged(_ sender: Any) {
        if let newValue = (sender as? NSTextField)?.stringValue, let active = selectedElement {
            let selectedRow = self.worldTree.selectedRow
            if let item = self.worldTree.item(atRow: selectedRow) as? Element {
                if item != active {
                    return;  // Do not rename in this case
                }
                if item.name != newValue {
                    self.elementStore?.updateName(element: item, newValue, undoManager: self.undoManager, refresh: { () -> Void in })
                }
            }

        }
    }

    override func loadView() {
        // Build UI programmatically using component controllers
        let initialFrame = NSRect(x: 0, y: 0, width: 944, height: 764)
        let root = NSView(frame: initialFrame)
        root.wantsLayer = true
        self.view = root

        // Visual effect background
        let vfx = NSVisualEffectView(frame: initialFrame)
        vfx.translatesAutoresizingMaskIntoConstraints = false
        vfx.blendingMode = .behindWindow
        vfx.material = .underWindowBackground
        vfx.state = .followsWindowActiveState
        root.addSubview(vfx)

        NSLayoutConstraint.activate([
            vfx.leadingAnchor.constraint(equalTo: root.leadingAnchor),
            vfx.trailingAnchor.constraint(equalTo: root.trailingAnchor),
            vfx.topAnchor.constraint(equalTo: root.topAnchor),
            vfx.bottomAnchor.constraint(equalTo: root.bottomAnchor),
        ])

        // Main split view (left: outline, right: editor)
        // Use constraints with proper frame to avoid zero-size issues
        let splitView = NSSplitView(frame: initialFrame)
        splitView.translatesAutoresizingMaskIntoConstraints = false
        splitView.isVertical = true
        splitView.dividerStyle = .thin
        vfx.addSubview(splitView)

        NSLayoutConstraint.activate([
            splitView.leadingAnchor.constraint(equalTo: vfx.leadingAnchor),
            splitView.trailingAnchor.constraint(equalTo: vfx.trailingAnchor),
            splitView.topAnchor.constraint(equalTo: vfx.topAnchor),
            splitView.bottomAnchor.constraint(equalTo: vfx.bottomAnchor),
        ])

        // Create component controllers
        outlineSidebarController = OutlineSidebarController()
        outlineSidebarController.viewController = self
        let leftPanel = outlineSidebarController.createView()
        splitView.addArrangedSubview(leftPanel)

        // Right split view (editor + properties)
        let rightSplitView = NSSplitView()
        rightSplitView.translatesAutoresizingMaskIntoConstraints = false
        rightSplitView.isVertical = false
        rightSplitView.dividerStyle = .thin
        splitView.addArrangedSubview(rightSplitView)

        // The inspector of a scene: the third column, next to the editor and the code.
        inspectorController = InspectorController()
        let inspectorPanel = inspectorController.createView()
        splitView.addArrangedSubview(inspectorPanel)

        // Configure split view behavior
        splitView.delegate = self
        mainSplitView = splitView
        self.editorSplitView = rightSplitView

        editorViewController = EditorViewController()
        editorViewController.viewController = self
        let editorPanel = editorViewController.createView()
        rightSplitView.addArrangedSubview(editorPanel)

        propertiesPanelController = PropertiesPanelController()
        propertiesPanelController.viewController = self
        let propertiesPanel = propertiesPanelController.createView()
        rightSplitView.addArrangedSubview(propertiesPanel)

        // Set holding priorities
        rightSplitView.setHoldingPriority(NSLayoutConstraint.Priority(1), forSubviewAt: 0)
        rightSplitView.setHoldingPriority(NSLayoutConstraint.Priority(1), forSubviewAt: 1)

        // Wire up legacy properties for compatibility
        self.scene = editorViewController.sceneView
        self.windowTitle = editorViewController.titleField
        self.toolsSegmentedControl = outlineSidebarController.toolbar
        self.worldTree = outlineSidebarController.outlineView
        self.textView = propertiesPanelController.textView
        self.exportSegments = editorViewController.exportSegments
        self.zoomLabel = editorViewController.zoomLabel

        // Set initial split positions
        DispatchQueue.main.async {
            splitView.setPosition(217, ofDividerAt: 0)
            if splitView.arrangedSubviews.count > 2 {
                splitView.setPosition(
                    splitView.bounds.width - InspectorController.width - splitView.dividerThickness,
                    ofDividerAt: 1)
            }
            let totalHeight = rightSplitView.bounds.height
            if totalHeight > 0 {
                let textScrollHeight: CGFloat = 127
                rightSplitView.setPosition(totalHeight - textScrollHeight - rightSplitView.dividerThickness, ofDividerAt: 0)
            }
        }
    }

    // MARK: - NSSplitViewDelegate Methods

    // Properties to hold references to the split views
    private var mainSplitView: NSSplitView?
    private var editorSplitView: NSSplitView?

    func splitView(_ splitView: NSSplitView, canCollapseSubview subview: NSView) -> Bool {
        return false  // Prevent collapsing of subviews
    }

    func splitView(_ splitView: NSSplitView, constrainMinCoordinate proposedMinimumPosition: CGFloat, ofSubviewAt dividerIndex: Int)
        -> CGFloat
    {
        if splitView == mainSplitView {
            // No collapsing of any pane: the inspector is the last one, so its right edge follows the
            // window. However, keep it from becoming the widest thing in the window.
            if dividerIndex == 0 { return 150 }
            return splitView.bounds.width - InspectorController.maxWidth
        } else if splitView == editorSplitView {
            // Minimum height for editor area (bottom part)
            return 200
        }
        return proposedMinimumPosition
    }

    func splitView(_ splitView: NSSplitView, constrainMaxCoordinate proposedMaximumPosition: CGFloat, ofSubviewAt dividerIndex: Int)
        -> CGFloat
    {
        if splitView == mainSplitView {
            // Maximum left panel width, and the room the inspector keeps at the last divider
            if dividerIndex == 0 { return 400 }
            return splitView.bounds.width - InspectorController.minWidth
        } else if splitView == editorSplitView {
            // Maximum height for editor area, leaving at least 50px for text
            return splitView.frame.height - 50
        }
        return proposedMaximumPosition
    }

    /// The inspector keeps its width when the window resizes; the middle column takes the room.
    func splitView(_ splitView: NSSplitView, shouldAdjustSizeOfSubview view: NSView) -> Bool {
        !(splitView == mainSplitView && view === inspectorController?.containerView)
    }

    override func viewDidLoad() {
        super.viewDidLoad()

        // Setup component controllers
        outlineSidebarController.setupDelegate()
        propertiesPanelController.setupDelegate(self)
        editorViewController.onLoad(self)
        editorViewController.setupExportMenu(viewController: self)
        inspectorController.attach(canvas: editorViewController.sceneCanvas?.canvas, document: document)
        editorViewController.showScene(sceneTree)
        reloadSceneItems()
        installComments()
        installNavigationHistory()

        // Legacy delegate setup for compatibility
        self.outlineViewDelegate = OutlineViewControllerDelegate(self)
        worldTree.delegate = self.outlineViewDelegate
        worldTree.dataSource = self.outlineViewDelegate

        self.textViewDelegate = TextPropertiesDelegate(self, self.textView!)

        // Set up zoom level callback to update the label
        scene.onZoomChanged = { [weak self] percentage in
            self?.updateZoomLabel(percentage)
        }

        if elementStore != nil && self.scene != nil {
            setElementModel(elementStore: elementStore!)
        }

        exportMgr.setViewController(self)

        let exportMenu = exportMgr.createMenu()

        exportSegments.setMenu(exportMenu, forSegment: 0)

        DistributedNotificationCenter.default().addObserver(
            self, selector: #selector(darkModeChanged), name: NSNotification.Name(rawValue: "AppleInterfaceThemeChangedNotification"),
            object: nil)
    }
    deinit {
        // Unregister distributed notifications
        DistributedNotificationCenter.default().removeObserver(self)
    }

    override func viewDidAppear() {
        editorViewController?.onAppear()
    }

    @objc func darkModeChanged(_ notif: NSNotification) {
        DispatchQueue.main.asyncAfter(
            deadline: .now() + 0.2,
            execute: {
                self.scene.scheduleRedraw()
                if let delegate = self.textViewDelegate {
                    delegate.highlight()
                }
            })
    }

    @IBAction func elementToolbarAction(_ sender: NSSegmentedCell) {
        switch sender.selectedSegment {
        case 0:  // This is add of new element.
            handleAddElement()
        case 1:  // This is remove of selected element.
            handleRemoveElement()
        case 2:  // This is options for selected element.
            handleElementOptions()
        default:
            break
        }
    }

    // Wrapper for programmatic segmented controls:
    // Programmatic NSSegmentedControl will send the control as sender, so forward to existing IBAction (which expects NSSegmentedCell).
    @objc func leftToolbarAction(_ sender: NSSegmentedControl) {
        if let cell = sender.cell as? NSSegmentedCell {
            elementToolbarAction(cell)
        }
    }

    // Wrapper for the top-right segmented control that forwards to the existing IBAction signature.
    @objc func extraSegmentAction(_ sender: NSSegmentedControl) {
        if let cell = sender.cell as? NSSegmentedCell {
            clickExtraButton(cell)
        }
    }

    fileprivate func hideSearchBox() {
        hideView(searchBox)
        searchBox = nil
    }

    fileprivate func hideView(_ controller: NSViewController?) {
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

    @IBAction func pasteAsItem(_ sender: NSMenuItem) {
        scene?.pasteAsItem(sender)
    }
    @IBAction func pasteAsItemSet(_ sender: NSMenuItem) {
        scene?.pasteAsItemSet(sender)
    }

    @IBAction func selectAllItemsKind(_ sender: NSMenuItem) {
        scene.selectAllByKind(kind: ItemKind.Item)
    }

    @IBAction func selectAllLinks(_ sender: NSMenuItem) {
        scene.selectAllByKind(kind: ItemKind.Link)
    }

    @IBAction func showSearchBox(_ sender: NSMenuItem) {
        if let active = self.selectedElement {
            hideSearchBox()

            // Instantiate programmatically (no storyboard)
            let sb = SearchBoxViewController()
            sb.setElement(active)

            sb.parentView = self.view

            // Avoid retaining self in closure strongly
            sb.closeAction = { [weak self] in self?.hideSearchBox() }
            sb.setActive = { [weak self] (item) in
                guard let self = self else { return }
                self.scene.setActiveItem(item)
                self.scene.centerItem(item, 120)
            }

            // Reasonable default size for the popover
            sb.preferredContentSize = NSSize(width: 400, height: 180)

            self.searchBox = sb

            self.present(sb, asPopoverRelativeTo: self.view.frame, of: self.view, preferredEdge: .maxX, behavior: .transient)
        }
    }

    @IBAction func showOperationBox(_ sender: NSMenuItem) {
        showOperationBox()
    }
    func showOperationBox() {
        if self.activeItems.count > 0, let element = self.selectedElement, let store = self.elementStore {
            hideOperationBox()

            // Instantiate OperationController programmatically
            let operations = OperationController()
            operations.setController(self)
            operations.setStore(store)
            operations.setElement(element)
            operations.setItems(self.activeItems)
            operations.preferredContentSize = NSSize(width: 300, height: 50)

            self.operationBox = operations

            let bounds = self.scene.getSelectionBounds()
            self.present(
                operations, asPopoverRelativeTo: NSRect(origin: bounds.origin, size: bounds.size),
                of: self.scene, preferredEdge: .minY, behavior: .transient)
        }
    }

    @objc func showHelp(_ sender: Any?) {
        // Show help in a separate window (programmatic)
        let helpController = HelpController()
        let helpWindow = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 800, height: 600),
            styleMask: [.titled, .closable, .resizable],
            backing: .buffered,
            defer: false)
        helpWindow.contentViewController = helpController
        let wc = NSWindowController(window: helpWindow)
        wc.showWindow(self)
    }

    @objc func zoomOutAction(_ sender: NSButton) {
        scene?.zoomOut(nil)
    }

    @objc func zoomInAction(_ sender: NSButton) {
        scene?.zoomIn(nil)
    }

    @objc func resetZoomAction(_ sender: NSButton) {
        scene?.resetZoom(nil)
    }

    func updateZoomLabel(_ percentage: Int) {
        zoomLabel?.stringValue = "\(percentage)%"
    }

    @IBAction func selectAllItems(_ sender: NSMenuItem) {
        guard let responder = self.view.window?.firstResponder else {
            return
        }
        if responder == self.scene {
            self.scene.selectAllItems()
            return
        } else if responder == self.textView {
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
        } else if responder == self.textView {
            self.textView.setSelectedRange(NSRange(location: 0, length: 0))
        }
    }

    @IBAction func editTitle(_ sender: NSMenuItem) {
        if let active = self.scene?.activeItems.first {
            scene.setActiveItem(active)
            scene?.editTitle(active, .Name)
        }
    }
    @IBAction func editBody(_ sender: NSMenuItem) {
        if let active = self.scene?.activeItems.first {
            scene.setActiveItem(active)
            scene?.editTitle(active, .Body)
        }
    }

    @IBAction func editValue(_ sender: NSMenuItem) {
        if let active = self.scene?.activeItems.first {
            scene.setActiveItem(active)
            scene?.editTitle(active, .Value)
        }
    }

    @IBAction func quickEdit(_ sender: NSMenuItem) {
        showOperationBox()
    }

    @IBAction func addLinkedItem(_ sender: NSMenuItem) {
        scene?.addNewItem()
    }

    @IBAction func addLinkedStyledItem(_ sender: NSMenuItem) {
        scene?.addNewItem(copyProps: true)
    }

    @IBAction func addFreeItem(_ sender: NSMenuItem) {
        scene?.addTopItem()
    }

    func validateMenuItem(_ menuItem: NSMenuItem) -> Bool {
        if let action = menuItem.action {

            if action == #selector(moveSceneNodeUp(_:)) || action == #selector(moveSceneNodeDown(_:))
                || action == #selector(moveSceneNodeIntoPrevious(_:)) || action == #selector(moveSceneNodeIntoNext(_:))
            {
                return movableSceneNode != nil
            }
            if action == #selector(renameSelection(_:)) {
                guard sceneTree != nil else { return selectedElement.map { $0.kind != .Root } ?? false }
                guard let canvas = editorViewController?.sceneCanvas?.canvas else { return false }
                return canvas.selectedNode != nil || canvas.screenNode != nil
            }
            if action == #selector(exportSelectionPNG(_:)) || action == #selector(exportSelectionPNGAtScale(_:)) {
                return canExportSelection
            }
            if action == #selector(exportSVG(_:)) {
                return sceneTree != nil
            }
            if action == #selector(exportPagesPNG(_:)) || action == #selector(exportPagesPDF(_:)) {
                return document != nil
            }
            if action == #selector(selectAllItems(_:)) {
                if self.view.window?.firstResponder == self.scene {
                    return true
                }
            }
            if action == #selector(importPenFile(_:)) {
                return true
            }
            if action == #selector(self.showSearchBox(_:)) {
                return true
            }
            if action == #selector(navigateBack(_:)) {
                return navigation.can(-1, valid: navigationValid)
            }
            if action == #selector(navigateForward(_:)) {
                return navigation.can(1, valid: navigationValid)
            }
            if action == #selector(self.showCommentList(_:)) {
                return commentsForDocument() != nil
            }
            if action == #selector(self.toggleShowResolvedComments(_:)) {
                guard let comments = commentsForDocument() else { return false }
                menuItem.state = comments.showResolved ? .on : .off
                return true
            }
            if action == #selector(self.toggleCompare(_:)) {
                menuItem.state = compareController?.isOn == true ? .on : .off
                return sceneTree != nil
            }
            if action == #selector(selectAllLinks) || action == #selector(selectAllItemsKind) {
                return true
            }
            if action == #selector(self.editTitle(_:)) || action == #selector(self.editBody(_:)) || action == #selector(self.editValue(_:))
                || action == #selector(self.showOperationBox(_:))
            {
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
            } else if responder == self.scene {
                target = .SceneView
            } else {
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

    @IBAction public func duplicateItem(_ sender: NSMenuItem) {
        switch findTarget() {
        case .WorldTree:
            if let active = self.selectedElement {
                let elementCopy = active.clone()

                self.elementStore?.add(
                    active.parent!, elementCopy, undoManager: self.undoManager,
                    refresh: { () -> Void in
                        DispatchQueue.main.async(execute: {
                            if active.parent!.kind == .Root {
                                self.worldTree.reloadData()
                            } else {
                                self.worldTree.reloadItem(active.parent!, reloadChildren: true)
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

    @IBAction public func inheritItem(_ sender: NSMenuItem) {
        switch findTarget() {
        case .WorldTree, .SceneView:
            if let active = self.selectedElement {
                let elementCopy = active.clone()

                let refs = Element.prepareItemRefs(elementCopy.items)

                for itm in elementCopy.items {
                    // Make clean propertirs
                    itm.properties = ModelProperties()

                    // Calculate index of item
                    let cmd = TennNode.newCommand(PersistenceStyleKind.Inherit.name, TennNode.newStrNode("../\(itm.name)"))
                    if let ind = refs[itm] {
                        cmd.add(TennNode.newIntNode(ind))
                    }
                    itm.properties.append(cmd)
                }

                self.elementStore?.add(
                    active, elementCopy, undoManager: self.undoManager,
                    refresh: { () -> Void in
                        DispatchQueue.main.async(execute: {
                            if active.parent!.kind == .Root {
                                self.worldTree.reloadData()
                            } else {
                                self.worldTree.reloadItem(active.parent!, reloadChildren: true)
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
        } else {
            // Add root item
            active = self.elementStore?.model
        }
        if let act = active {
            self.elementStore?.add(
                act, newEl, undoManager: self.undoManager,
                refresh: { () -> Void in
                    DispatchQueue.main.async(execute: {
                        if act.kind == .Root {
                            self.worldTree.reloadData()
                        } else {
                            self.worldTree.reloadItem(act, reloadChildren: true)
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
                } else {
                    selectedElement = parent
                }

                self.elementStore?.remove(
                    parent, active, undoManager: self.undoManager,
                    refresh: { () -> Void in
                        DispatchQueue.main.async(execute: {
                            if parent.kind == .Root {
                                self.worldTree.reloadData()
                            } else {
                                self.worldTree.reloadItem(parent, reloadChildren: true)
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

    override func present(
        _ viewController: NSViewController, asPopoverRelativeTo positioningRect: NSRect, of positioningView: NSView,
        preferredEdge: NSRectEdge, behavior: NSPopover.Behavior
    ) {

        if let vc = viewController as? SourcePopoverViewController {
            if let active = self.selectedElement {
                vc.setElement(element: active)

                super.present(
                    viewController, asPopoverRelativeTo: positioningRect, of: positioningView, preferredEdge: preferredEdge,
                    behavior: behavior)
                return
            }
        }

        if let vc = viewController as? SyncViewController {
            if let active = self.selectedElement {
                vc.setElement(element: active)
                vc.setViewController(self)

                super.present(
                    viewController, asPopoverRelativeTo: positioningRect, of: positioningView, preferredEdge: preferredEdge,
                    behavior: behavior)
                return
            }
        }

        super.present(
            viewController, asPopoverRelativeTo: positioningRect, of: positioningView, preferredEdge: preferredEdge, behavior: behavior)
    }

    func onElementSelected(_ element: Element?) {
        if selectedElement != element {
            self.selectedElement = element
            self.activeItems = []
            self.inspectorController?.showClassic(items: [], element: element)

            if let el = element {
                self.scene.setActiveElement(el)
                if sceneTree != nil, let path = document?.sceneElementPath(of: el) {
                    editorViewController?.showSceneElement(path)
                    reloadSceneItems()
                }

                self.updateTextProperties()
                self.navigationSync()
            }
        }
    }

    /// Shows the place a link names in this window, as go/app's gotoLink: the element, its node, and a
    /// line for what the file does not have.
    func showLink(element: [String], node: String) {
        guard isViewLoaded, let document else { return }
        view.window?.makeKeyAndOrderFront(nil)
        guard let target = LinkOpen.target(in: document, element: element, node: node) else {
            linkMessage("No element \(element.joined(separator: ".")) in this file.")
            return
        }
        if let path = target.elementPath {
            editorViewController?.showSceneElement(path)
            reloadSceneItems()
            guard !node.isEmpty else { return }
            if let offset = target.nodeOffset {
                editorViewController?.sceneCanvas?.canvas.setMessage(nil)
                editorViewController?.sceneCanvas?.canvas.selectSource(at: offset)
            } else {
                linkMessage("No node \(node) in the element \(element.joined(separator: ".")).")
            }
            return
        }
        guard let selected = target.element else { return }
        onElementSelected(selected)
        outlineSidebarController?.setSelectedElement(selected)
        guard !node.isEmpty else { return }
        if let name = target.item, let item = selected.items.first(where: { !($0 is LinkItem) && $0.name == name }) {
            scene?.setActiveItem(item)
        } else {
            linkMessage("No node \(node) in the element \(element.joined(separator: ".")).")
        }
    }

    /// Says what a link could not show; a classic window has no banner, so it asks.
    private func linkMessage(_ text: String) {
        if sceneTree != nil {
            editorViewController?.sceneCanvas?.canvas.setMessage(text)
            return
        }
        let alert = NSAlert()
        alert.messageText = "Open Link"
        alert.informativeText = text
        alert.runModal()
    }

    func updateTextProperties() {
        if document?.sceneTree == nil {
            inspectorController?.refresh()
        }
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
        // A Save As names another file: the report of the old one is not this document any more.
        if let compare = compareController, compare.isOn, !compare.isCurrent() {
            closeCompare()
        }
    }

    /// Puts the canvas of a scene document in the window, or the classic one when tree is nil.
    func setSceneTree(_ tree: Scene.Tree?) {
        sceneTree = tree
        if isViewLoaded {
            editorViewController?.showScene(tree)
            reloadSceneItems()
            inspectorController?.refresh()
            refreshCompare()
        }
    }

    // MARK: - Compare with Git

    /// View > Compare with Git: the file against its last commit, over the outline and the canvas.
    @objc func toggleCompare(_ sender: Any?) {
        guard isViewLoaded else { return }
        if compareController?.isOn == true {
            closeCompare()
            return
        }
        guard let document, sceneTree != nil else { return }
        let compare = CompareController(document: document)
        if !compare.start() {
            closeCompare()
            editorViewController?.sceneCanvas?.canvas.setMessage(compare.message)
            return
        }
        compareController = compare
        showCompare()
    }

    /// Swaps the element outline for the rows of the report and frames the changed nodes.
    private func showCompare() {
        guard let compare = compareController, let canvas = editorViewController?.sceneCanvas?.canvas else { return }
        worldTree.menu = nil  // the element actions do not act on report rows
        let source = CompareOutlineSource(compare: compare)
        source.onSelect = { [weak self] row in self?.selectCompare(row) }
        compareOutlineSource = source
        worldTree.dataSource = source
        worldTree.delegate = source
        worldTree.reloadData()
        for row in source.topRows {
            worldTree.expandItem(row, expandChildren: true)
        }
        canvas.setMessage(nil)
        paintCompare()
    }

    /// Compares the text again after an edit, or leaves the mode when the document is no longer the one
    /// the comparison was started on.
    private func refreshCompare() {
        guard let compare = compareController else { return }
        compare.refresh()
        guard compare.isOn else {
            closeCompare()
            return
        }
        compareOutlineSource?.reload()
        worldTree.reloadData()
        if let source = compareOutlineSource {
            for row in source.topRows {
                worldTree.expandItem(row, expandChildren: true)
            }
        }
        paintCompare()
    }

    /// Puts the summary and the frames of the report on the canvas, or the reason it could not be read.
    private func paintCompare() {
        guard let compare = compareController, let canvas = editorViewController?.sceneCanvas?.canvas else { return }
        canvas.setCompareFrames(compare.frames())
        canvas.setCompareNote(compare.summary())
        canvas.setMessage(compare.message)
    }

    /// Leaves compare mode and puts the element outline and the canvas back as they were.
    private func closeCompare() {
        guard compareController != nil else { return }
        compareController?.close()
        compareController = nil
        compareOutlineSource = nil
        if isViewLoaded {
            worldTree.menu = outlineViewDelegate?.createMenu()
            worldTree.dataSource = outlineViewDelegate
            worldTree.delegate = outlineViewDelegate
            worldTree.reloadData()
            editorViewController?.sceneCanvas?.canvas.setCompareFrames([])
            editorViewController?.sceneCanvas?.canvas.setCompareNote(nil)
            editorViewController?.sceneCanvas?.canvas.setMessage(nil)
        }
    }

    /// Shows the element and the node a report row names in the version on the canvas; a node the new
    /// version no longer has leaves its element shown.
    private func selectCompare(_ row: CompareController.Row) {
        guard let node = row.node, !node.element.isEmpty, let path = elementIndexPath(node.element) else { return }
        editorViewController?.showSceneElement(path)
        reloadSceneItems()
        if let current = node.new {
            editorViewController?.sceneCanvas?.canvas.selectSource(at: current.range.start)
        }
    }

    /// The outline path of an element the comparison names by its names from the file root; #n picks
    /// the repeated sibling an element path counts.
    private func elementIndexPath(_ namePath: String) -> [Int]? {
        guard let main = sceneTree?.main else { return nil }
        var nodes = main.elems
        var path: [Int] = []
        for raw in namePath.components(separatedBy: "/") {
            let (name, repeatIndex) = Self.pathSegment(raw)
            var seen = 0
            var found: Int?
            for (i, n) in nodes.enumerated() where n.name == name {
                if seen == repeatIndex {
                    found = i
                    break
                }
                seen += 1
            }
            guard let i = found else { return nil }
            path.append(i)
            nodes = nodes[i].elems
        }
        return path.isEmpty ? nil : path
    }

    /// A path segment with the #n a repeated name among siblings gets, as Diff writes it.
    private static func pathSegment(_ text: String) -> (name: String, repeatIndex: Int) {
        guard let hash = text.lastIndex(of: "#"), let n = Int(text[text.index(after: hash)...]) else {
            return (text, 0)
        }
        return (String(text[..<hash]), n)
    }

    /// Add Library…: a file of components becomes an element of the document that mounts it, and Insert
    /// Component offers what it holds (go/app/outline_scene.go addLibrary).
    @objc func addLibrary(_ sender: Any?) {
        guard let window = view.window, let document, document.sceneTree != nil else { return }
        let panel = NSOpenPanel()
        panel.title = "Add Library"
        panel.allowsMultipleSelection = false
        panel.canChooseDirectories = false
        panel.allowedContentTypes = [UTType(filenameExtension: "tenn") ?? .data]
        panel.directoryURL = document.fileURL?.deletingLastPathComponent()
        panel.beginSheetModal(for: window) { [weak self] response in
            guard response == .OK, let url = panel.url else { return }
            MainActor.assumeIsolated {
                guard let self, let document = self.document else { return }
                do {
                    try document.addLibrary(at: url)
                    self.reloadSceneItems()
                } catch {
                    self.editorViewController?.sceneCanvas?.canvas.setMessage(error.localizedDescription)
                }
            }
        }
    }

    /// File > Import Pen File…: the .pen files become .tenn beside the first of them and that output
    /// opens (go/app/import_pen.go importPenDialog).
    @objc func importPenFile(_ sender: Any?) {
        let panel = NSOpenPanel()
        panel.title = "Import Pen File"
        panel.allowsMultipleSelection = true
        panel.canChooseDirectories = false
        panel.allowedContentTypes = [UTType(filenameExtension: "pen") ?? .data]
        panel.directoryURL = document?.fileURL?.deletingLastPathComponent()
        panel.begin { [weak self] response in
            guard response == .OK, let self else { return }
            let paths = panel.urls.map(\.path)
            // Converting reads whole files; the window stays as it was until the job is done.
            DispatchQueue.global(qos: .userInitiated).async {
                let result = Result { try Self.importPen(paths: paths) }
                DispatchQueue.main.async {
                    switch result {
                    case .success(let report):
                        self.showPenImport(report)
                    case .failure(let error):
                        self.showPenImport("Import failed", "\(error)")
                    }
                }
            }
        }
    }

    /// Converts paths and writes their .tenn beside the first of them; main is the output of that first
    /// file, notes what the import left out (go/app/import_pen.go penImport).
    nonisolated static func importPen(paths: [String]) throws -> (main: String, notes: [String], count: Int) {
        guard let first = paths.first else { throw PenImportFailure(message: "no .pen file chosen") }
        let url = URL(fileURLWithPath: first).standardizedFileURL
        let result = try PenImport.convert(paths: paths)
        try result.write(to: url.deletingLastPathComponent().path)
        guard let file = result.files.first(where: { $0.path == url.path }) else {
            throw PenImportFailure(message: "\(first): no output written")
        }
        let notes = penNotes(file)
        return (url.deletingLastPathComponent().appendingPathComponent(file.name + ".tenn").path, notes.lines, notes.count)
    }

    /// The warnings and unsupported features of an import, warnings first and features by name; past
    /// the limit the last line is how many more there are (go/app/import_pen.go penNotes).
    nonisolated static func penNotes(_ f: PenImport.File, limit: Int = 8) -> (lines: [String], count: Int) {
        var lines = f.warnings
        for key in f.unsupported.keys.sorted() {
            lines.append("\(key) (\(f.unsupported[key] ?? 0))")
        }
        let count = lines.count
        if count > limit {
            lines = Array(lines.prefix(limit)) + ["and \(count - limit) more"]
        }
        return (lines, count)
    }

    /// Says what the import left out, then opens what it wrote.
    private func showPenImport(_ report: (main: String, notes: [String], count: Int)) {
        if report.count > 0 {
            showPenImport(
                "Imported with notes", "\(report.count) notes:\n\n\(report.notes.joined(separator: "\n"))")
        }
        openPenImport(report.main)
    }

    private func showPenImport(_ title: String, _ text: String) {
        let alert = NSAlert()
        alert.messageText = title
        alert.informativeText = text
        alert.runModal()
    }

    /// Opens the .tenn of the import; a window that already shows it comes to the front.
    private func openPenImport(_ path: String) {
        let url = URL(fileURLWithPath: path)
        if let document = NSDocumentController.shared.document(for: url) {
            document.showWindows()
            return
        }
        NSDocumentController.shared.openDocument(withContentsOf: url, display: true) { [weak self] _, _, error in
            guard let error else { return }
            self?.showPenImport("Import failed", "\(error)")
        }
    }

    /// Puts the rows of the scene outline back after an edit that added or removed a mount, with the
    /// element the canvas shows selected again.
    func reloadOutline() {
        guard let model = elementStore?.model else { return }
        worldTree.reloadData()
        if PreferenceConstants.preference.autoExpand {
            expandItems(model.elements, PreferenceConstants.preference.autoExpandLevel)
        }
        if let path = editorViewController?.sceneCanvas?.canvas.elementPath,
            let el = document?.sceneElement(at: path)
        {
            let row = worldTree.row(forItem: el)
            if row >= 0 {
                worldTree.selectRowIndexes(IndexSet(integer: row), byExtendingSelection: false)
            }
        }
    }

    /// Insert Component…: the components of the mounted libraries and of the document, a sheet to pick
    /// one, and a use written where the selection says (go/app/outline_scene.go showComponentPicker).
    @objc func insertComponent(_ sender: Any?) {
        guard let window = view.window, let document, document.sceneTree != nil,
            let canvas = editorViewController?.sceneCanvas?.canvas, let canvasNode = canvas.screenNode
        else { return }
        guard !document.components("", limit: 1).isEmpty else {
            canvas.setMessage("No component to insert: add a library with File > Add Library…")
            return
        }
        let picker = ComponentPickerController(
            provider: { [weak document] q in document?.components(q, limit: 200) ?? [] },
            onPick: { [weak self] component in
                self?.componentPicker = nil
                self?.insert(component, canvas: canvas, canvasNode: canvasNode)
            })
        componentPicker = picker
        picker.present(asSheetFor: window)
    }

    /// Writes the use of component where InsertTarget says, or offers its slots when the selected
    /// instance has named ones.
    private func insert(_ component: Scene.Component, canvas: SceneCanvasView, canvasNode: Scene.Node) {
        guard let tree = document?.sceneTree else { return }
        let (into, slots) = insertTarget(tree, canvas.selectedNode)
        if let into, !slots.isEmpty {
            pendingComponent = component
            pendingInto = into
            pendingSlots = slots
            pendingCanvas = canvas
            pendingCanvasNode = canvasNode
            let menu = NSMenu()
            for (i, slot) in slots.enumerated() {
                let item = NSMenuItem(
                    title: slot.isEmpty ? "Into the default slot" : "Into slot \(slot)",
                    action: #selector(pickSlot(_:)), keyEquivalent: "")
                item.tag = i
                item.target = self
                menu.addItem(item)
            }
            menu.addItem(.separator())
            let free = NSMenuItem(title: "On the canvas", action: #selector(pickSlot(_:)), keyEquivalent: "")
            free.tag = -1
            free.target = self
            menu.addItem(free)
            menu.popUp(positioning: nil, at: NSEvent.mouseLocation, in: nil)
            return
        }
        write(component, into: into, slot: "", canvas: canvas, canvasNode: canvasNode)
    }

    @objc private func pickSlot(_ sender: NSMenuItem) {
        guard let component = pendingComponent, let canvas = pendingCanvas, let canvasNode = pendingCanvasNode
        else { return }
        let onCanvas = sender.tag < 0
        let slot = onCanvas || !pendingSlots.indices.contains(sender.tag) ? "" : pendingSlots[sender.tag]
        write(component, into: onCanvas ? nil : pendingInto, slot: slot, canvas: canvas, canvasNode: canvasNode)
    }

    /// Writes the use through the document and selects what was written.
    private func write(
        _ component: Scene.Component, into: Scene.Node?, slot: String, canvas: SceneCanvasView,
        canvasNode: Scene.Node
    ) {
        guard let document else { return }
        do {
            let offset = try document.insertComponent(
                path: component.path, name: component.name, into: into, slot: slot, canvas: canvasNode,
                spot: canvas.freeSpot())
            canvas.setMessage(nil)
            canvas.selectSource(at: offset)
            reloadSceneItems()
        } catch {
            canvas.setMessage(error.localizedDescription)
        }
    }

    /// Selects a drawn item of the scene, as a row of the Items pane does.
    func selectSceneItem(_ path: [Int]) {
        editorViewController?.sceneCanvas?.canvas.select(path)
    }

    /// The node the Element > Move actions act on: written in the scene under a parent, not a part of an
    /// instance.
    private var movableSceneNode: Scene.Node? {
        guard sceneTree != nil, let canvas = editorViewController?.sceneCanvas?.canvas,
            let node = canvas.selectedNode, node.parent != nil,
            canvas.part(at: canvas.selection) == nil
        else { return nil }
        return node
    }

    /// Element > Move Up/Down: the node changes places with the sibling before or after it, as one undo
    /// step, and the selection follows it (go/app/scenemove.go moveNodeBy).
    @objc func moveSceneNodeUp(_ sender: Any?) { moveSceneNode(-1, into: false) }
    @objc func moveSceneNodeDown(_ sender: Any?) { moveSceneNode(1, into: false) }

    /// Element > Move Left/Right: the node becomes the last kid of the sibling before it or the first
    /// kid of the one after it, as one undo step (go/app/scenemove.go moveNodeInto).
    @objc func moveSceneNodeIntoPrevious(_ sender: Any?) { moveSceneNode(-1, into: true) }
    @objc func moveSceneNodeIntoNext(_ sender: Any?) { moveSceneNode(1, into: true) }

    private func moveSceneNode(_ delta: Int, into: Bool) {
        guard let document, let canvas = editorViewController?.sceneCanvas?.canvas,
            let node = movableSceneNode, let text = sceneTree?.source(node.pos.file)
        else { return }
        let moved = into ? moveNodeIntoEdit(text, node, delta) : moveNodeEdit(text, node, delta)
        guard let moved else { return }
        do {
            try document.applySceneEdit(moved.edit, in: node.pos.file)
        } catch {
            canvas.setMessage(error.localizedDescription)
            return
        }
        canvas.setMessage(nil)
        reloadSceneItems()
        canvas.selectSource(at: moved.at)
    }

    // MARK: - Rename

    /// The answer to the question about the references of a rename, as Go's Yes/No/Cancel.
    enum RenameAnswer {
        case yes
        case no
        case cancel
    }

    /// Asks for the new name; nil when the dialog is cancelled. A test answers here instead of the dialog.
    var renamePrompt: ((_ old: String) -> String?)?
    /// Asks whether the references follow, as Go's confirmRename; nil opens the dialog. Not asked when
    /// there are none.
    var renameConfirm: ((_ plan: Scene.RenamePlan) -> RenameAnswer)?

    /// Element > Rename (Cmd+Shift+R): the node the canvas selected, else the element it shows.
    @objc func renameSelection(_ sender: Any?) {
        guard sceneTree != nil, let canvas = editorViewController?.sceneCanvas?.canvas else {
            if let element = selectedElement {
                renameElement(element)
            }
            return
        }
        if let node = canvas.selectedNode {
            renameSceneNode(node)
        } else if let node = canvas.screenNode {
            renameSceneNode(node, row: shownElementRow(canvas))
        }
    }

    /// Renames an element of the outline: a classic document through the model, a scene through its text.
    func renameElement(_ element: Element) {
        guard sceneTree != nil else {
            renameClassicElement(element)
            return
        }
        if let node = sceneElementNode(element) {
            renameSceneNode(node, row: element)
        }
    }

    /// Renames a node of the scene as go/app/rename_ui.go renameNodeTo: the plan names the references, the
    /// question asks about them, and the name and the references go in as one undo step.
    func renameSceneNode(_ node: Scene.Node, row: Element? = nil) {
        guard let tree = sceneTree, let canvas = editorViewController?.sceneCanvas?.canvas else { return }
        guard node.pos.file == tree.main?.pos.file else {
            canvas.setMessage("\(node.pos.file) is a mounted file: open it to edit it")
            return
        }
        let old = node.name
        let oldPath = CommentPaths.nodePath(of: node, from: canvas.screenNode)
        let oldElement: [String] =
            node.kind == .element
            ? CommentPaths.elementPath(of: node) : canvas.screenNode.map(CommentPaths.elementPath(of:)) ?? []
        guard let name = askName(old)?.trimmingCharacters(in: .whitespacesAndNewlines), name != old else {
            return
        }
        let plan: Scene.RenamePlan
        restoringNavigation = true
        do {
            plan = try tree.planRename(node, name)
            let answer = askReferences(plan)
            guard answer != .cancel else {
                restoringNavigation = false
                return
            }
            try applyRename(plan, update: answer == .yes)
        } catch {
            restoringNavigation = false
            canvas.setMessage((error as? Scene.RenameError)?.message ?? error.localizedDescription)
            return
        }
        restoringNavigation = false
        navigationRenamed(wasElement: node.kind == .element, element: oldElement, node: oldPath, name: plan.name)
        canvas.setMessage(nil)
        if let row, row.name != plan.name {
            row.name = plan.name
            reloadOutline()
        }
        reloadSceneItems()
        selectRenamed(oldPath, plan.name, canvas)
    }

    /// The classic rename: the model names the element, and a diagram has nothing that names it by path.
    private func renameClassicElement(_ element: Element) {
        guard element.parent != nil,
            let name = askName(element.name)?.trimmingCharacters(in: .whitespacesAndNewlines),
            name != element.name
        else { return }
        guard !name.isEmpty else {
            renameMessage("The name cannot be empty.")
            return
        }
        let old = classicElementPath(element)
        elementStore?.updateName(element: element, name, undoManager: undoManager, refresh: {})
        var now = old
        now[now.count - 1] = element.name
        navigation.elementRenamed(from: old, to: now)
        navigationSync()
    }

    /// Says why a rename was refused; a classic window has no banner over a canvas.
    private func renameMessage(_ text: String) {
        let alert = NSAlert()
        alert.messageText = text
        alert.runModal()
    }

    /// The node of the element an outline row names; nil when the tree no longer holds it.
    private func sceneElementNode(_ element: Element) -> Scene.Node? {
        guard let tree = sceneTree, let path = document?.sceneElementPath(of: element) else { return nil }
        var nodes = tree.main?.elems ?? []
        var node: Scene.Node?
        for i in path {
            guard nodes.indices.contains(i) else { return nil }
            node = nodes[i]
            nodes = nodes[i].elems
        }
        return node?.kind == .element ? node : nil
    }

    /// The outline row of the element the canvas shows; the first element of the file when it shows none.
    private func shownElementRow(_ canvas: SceneCanvasView) -> Element? {
        let path =
            canvas.elementPath
            ?? sceneTree?.main?.elems.firstIndex(where: { $0.kind == .element }).map { [$0] }
        guard let path else { return nil }
        return document?.sceneElement(at: path)
    }

    /// Puts the plan in the text: its own name and, when update, the references, as one undo step. A
    /// reference in a mounted file is refused whole, naming the file: this window has no undo for it.
    private func applyRename(_ plan: Scene.RenamePlan, update: Bool) throws {
        guard let document, let main = sceneTree?.main?.pos.file else { return }
        let edits = plan.edits(update: update)
        if let file = edits.first(where: { $0.file != main })?.file {
            throw Scene.RenameError(message: "\(file) is a mounted file: open it to edit it")
        }
        // applySceneEdits edits the text from the end, so the offsets of the earlier edits stay valid.
        try document.applySceneEdits(Array(edits.map(\.edit).reversed()), in: main)
    }

    /// Selects the node again at its new name's place in the text; a renamed element stays shown as it is.
    private func selectRenamed(_ oldPath: String, _ name: String, _ canvas: SceneCanvasView) {
        guard !oldPath.isEmpty, let shown = canvas.screenNode else { return }
        let cut = oldPath.lastIndex(of: ".")
        let parent = cut.map { String(oldPath[...$0]) } ?? ""
        guard let node = CommentPaths.node(at: parent + normalizeName(name), from: shown) else { return }
        canvas.selectSource(at: node.range.start)
    }

    /// The new name from a box, or nil when the dialog is cancelled.
    private func askName(_ old: String) -> String? {
        if let renamePrompt { return renamePrompt(old) }
        let alert = NSAlert()
        alert.messageText = "Rename"
        alert.informativeText = "The new name of \(String(reflecting: old)):"
        alert.addButton(withTitle: "Rename")
        alert.addButton(withTitle: "Cancel")
        let field = NSTextField(frame: NSRect(x: 0, y: 0, width: 260, height: 24))
        field.stringValue = old
        alert.accessoryView = field
        alert.window.initialFirstResponder = field
        guard alert.runModal() == .alertFirstButtonReturn else { return nil }
        return field.stringValue
    }

    /// Whether the references follow, as Go's confirmRename: no references, no question.
    private func askReferences(_ plan: Scene.RenamePlan) -> RenameAnswer {
        guard !plan.refs.isEmpty else { return .yes }
        if let renameConfirm { return renameConfirm(plan) }
        let alert = NSAlert()
        alert.messageText = "Rename to \(String(reflecting: plan.name))"
        alert.informativeText = """
            \(plan.refs.count) references name it:

            \(refLines(plan.refs, 10))

            Yes: rename and update \(plan.refs.count) references.
            No: rename only; the references keep the old name and stop working.
            Cancel: keep the old name.
            """
        alert.alertStyle = .warning
        alert.addButton(withTitle: "Yes")
        alert.addButton(withTitle: "No")
        alert.addButton(withTitle: "Cancel")
        switch alert.runModal() {
        case .alertFirstButtonReturn: return .yes
        case .alertSecondButtonReturn: return .no
        default: return .cancel
        }
    }

    /// The places that name the node as go/app/rename.go refLines lists them: file:line and the line.
    private func refLines(_ refs: [Scene.Ref], _ limit: Int) -> String {
        var out: [String] = []
        for (i, r) in refs.enumerated() {
            if i == limit {
                out.append("and \(refs.count - limit) more")
                break
            }
            let text = sceneTree?.source(r.file) ?? ""
            let bytes = Array(text.utf8)
            let start = lineStart(text, r.arg.start)
            let end = bytes[start...].firstIndex(of: 0x0A) ?? bytes.count
            let line = bytes[..<start].reduce(1) { $1 == 0x0A ? $0 + 1 : $0 }
            var body = String(decoding: bytes[start..<end], as: UTF8.self)
                .trimmingCharacters(in: .whitespaces)
            if body.count > 70 { body = String(body.prefix(70)) + "..." }
            out.append("\(r.file):\(line)  \(body)")
        }
        return out.joined(separator: "\n")
    }

    /// A row of the component browser of the Items pane: the component goes where Insert Component… would
    /// put it, at the canvas selection or a free spot, and is selected.
    func insertComponentRow(path: String, name: String) {
        guard let document,
            let component = document.components("", limit: 200).first(where: { $0.path == path }),
            let canvas = editorViewController?.sceneCanvas?.canvas, let canvasNode = canvas.screenNode
        else { return }
        insert(component, canvas: canvas, canvasNode: canvasNode)
    }

    /// A component dropped on the canvas at a point of the scene: the use takes that spot, and the new
    /// instance is selected. False for a path the document no longer offers.
    @discardableResult
    func dropComponent(path: String, at point: Scene.Point) -> Bool {
        guard let document,
            let component = document.components("", limit: 200).first(where: { $0.path == path }),
            let canvas = editorViewController?.sceneCanvas?.canvas, let canvasNode = canvas.screenNode
        else { return false }
        do {
            let offset = try document.insertComponent(
                path: component.path, name: component.name, into: nil, slot: "", canvas: canvasNode,
                spot: (point.x, point.y))
            canvas.setMessage(nil)
            canvas.selectSource(at: offset)
            reloadSceneItems()
            return true
        } catch {
            canvas.setMessage(error.localizedDescription)
            return false
        }
    }

    /// Points the scene canvas at the comments of the document: pins, new threads and new comments.
    private func installComments() {
        guard let canvas = editorViewController?.sceneCanvas?.canvas else { return }
        canvas.onPinClick = { [weak self] id in self?.showCommentThread(id) }
        canvas.onAddComment = { [weak self] text, anchor in self?.addComment(text: text, anchor: anchor) }
        refreshCommentPins()
    }

    /// The comments of the document: its file, standardized, names them; a document without a file has none.
    private func commentsForDocument() -> CommentsController? {
        guard sceneTree != nil else { return nil }
        let url = document?.fileURL
        if let commentsController, commentsForURL == url { return commentsController }
        let comments = CommentsController(url: url)
        commentsForURL = url
        comments.onChange = { [weak self] in self?.refreshCommentPins() }
        commentsController = comments
        return comments
    }

    /// Places the pins of the comments of the element the canvas shows, as go/app placePins.
    private func refreshCommentPins() {
        guard let canvas = editorViewController?.sceneCanvas?.canvas else { return }
        // A scene without a file cannot write comments; the canvas says so instead of asking for the text.
        canvas.commentUnavailable =
            sceneTree != nil && document?.fileURL == nil ? "Save the file to comment on it." : nil
        guard sceneTree != nil, let comments = commentsForDocument(), let element = canvas.screenNode else {
            canvas.setCommentPins([])
            return
        }
        let boxes = commentBoxes(of: canvas, element: element)
        let names = CommentPaths.elementPath(of: element)
        canvas.setCommentPins(
            comments.pins(element: names, box: { boxes[$0] }, origin: .zero, showResolved: comments.showResolved))
    }

    /// View > Comments…: the comments of the document over the window; a row shows its place and opens
    /// its thread (go/app comments_thread.go showCommentList).
    @objc func showCommentList(_ sender: Any?) {
        guard isViewLoaded, let window = view.window, let comments = commentsForDocument() else { return }
        window.makeKeyAndOrderFront(nil)
        comments.presentList(from: window) { [weak self] id in
            self?.jumpToComment(id)
        }
    }

    /// Shows the place of a comment and opens its thread, as go/app jumpToComment.
    func jumpToComment(_ id: String) {
        guard let comments = commentsForDocument(), let target = comments.jumpTarget(id: id), let document else {
            return
        }
        guard let place = LinkOpen.target(in: document, element: target.element, node: target.node) else {
            linkMessage("No element \(target.element.joined(separator: ".")) in this file.")
            return
        }
        if let path = place.elementPath {
            editorViewController?.showSceneElement(path)
            reloadSceneItems()
        }
        guard !target.node.isEmpty else {
            showCommentThread(id)
            return
        }
        guard let offset = place.nodeOffset else {
            linkMessage("No node \(target.node) in the element \(target.element.joined(separator: ".")).")
            showCommentThread(id)
            return
        }
        editorViewController?.sceneCanvas?.canvas.setMessage(nil)
        editorViewController?.sceneCanvas?.canvas.selectSource(at: offset)
        showCommentThread(id)
    }

    /// View > Show Resolved Comments: resolved pins stay on the canvas and in the list.
    @objc func toggleShowResolvedComments(_ sender: Any?) {
        guard let comments = commentsForDocument() else { return }
        comments.setShowResolved(!comments.showResolved)
    }

    /// The boxes of the drawn nodes written in the element, by the node path a comment anchors with. A part
    /// of an instance is not written there and has no path, so it places no pin.
    private func commentBoxes(of canvas: SceneCanvasView, element: Scene.Node) -> [String: CGRect] {
        var boxes: [String: CGRect] = [:]
        guard let root = canvas.placed else { return boxes }
        func walk(_ pl: Scene.Placed) {
            if let src = pl.src, src !== element {
                let path = CommentPaths.nodePath(of: src, from: element)
                if !path.isEmpty, boxes[path] == nil {
                    boxes[path] = CGRect(x: pl.x, y: pl.y, width: pl.w, height: pl.h)
                }
            }
            for k in pl.kids { walk(k) }
        }
        walk(root)
        return boxes
    }

    private func showCommentThread(_ id: String) {
        guard let canvas = editorViewController?.sceneCanvas?.canvas, let comments = commentsForDocument()
        else { return }
        let at = canvas.commentPins.first { $0.id == id }.map { canvas.viewPoint(canvas.pinCenter($0)) }
        let rect = NSRect(x: at?.x ?? canvas.bounds.midX, y: at?.y ?? canvas.bounds.midY, width: 1, height: 1)
        comments.showThread(id: id, from: rect, of: canvas)
    }

    private func addComment(text: String, anchor: Comments.Anchor) {
        guard let canvas = editorViewController?.sceneCanvas?.canvas, let comments = commentsForDocument()
        else { return }
        do {
            try comments.addComment(text: text, anchor: anchor)
            canvas.setMessage(nil)
        } catch {
            canvas.setMessage(error.localizedDescription)
        }
    }

    /// Lists the items of the element the scene canvas shows.
    func reloadSceneItems() {
        if sceneTree == nil {
            outlineSidebarController?.showItems(nil)
        } else {
            outlineSidebarController?.showItems(editorViewController?.sceneCanvas?.canvas)
        }
        refreshCommentPins()
    }

    // MARK: - Back and Forward

    @objc func navigateBack(_ sender: Any?) { navigate(-1) }

    @objc func navigateForward(_ sender: Any?) { navigate(1) }

    /// Shows the nearest entry in direction that still resolves, without recording it, as go/app's navGo.
    private func navigate(_ direction: Int) {
        guard !restoringNavigation, let entry = navigation.move(direction, valid: navigationValid) else { return }
        restoringNavigation = true
        showNavigationEntry(entry)
        restoringNavigation = false
        navigationSync()
    }

    /// Puts the place an entry names on the canvas: the element and the node, or the classic items.
    private func showNavigationEntry(_ entry: NavigationEntry) {
        if let tree = sceneTree, let place = NavigateResolve.scene(entry, in: tree) {
            editorViewController?.showSceneElement(place.path)
            let canvas = editorViewController?.sceneCanvas?.canvas
            if place.nodes.isEmpty {
                canvas?.select(nil)
            } else if let node = place.nodes.first {
                canvas?.selectSource(at: node.range.start)
            }
            if let element = document?.sceneElement(at: place.path) {
                outlineSidebarController?.setSelectedElement(element)
            }
            reloadSceneItems()
            return
        }
        guard let model = elementStore?.model, let place = NavigateResolve.classic(entry, in: model) else { return }
        onElementSelected(place.element)
        outlineSidebarController?.setSelectedElement(place.element)
        scene?.setActiveItems(place.items)
    }

    /// What is selected now in the form the history keeps; nil for a document that shows no element.
    private func currentNavigationEntry() -> NavigationEntry? {
        if sceneTree != nil {
            guard let canvas = editorViewController?.sceneCanvas?.canvas, let shown = canvas.screenNode,
                shown.kind == .element
            else { return nil }
            var entry = NavigationEntry(element: CommentPaths.elementPath(of: shown))
            guard !entry.element.isEmpty else { return nil }
            if let node = canvas.selectedNode {
                let path = CommentPaths.nodePath(of: node, from: shown)
                if !path.isEmpty { entry.nodes = [path] }
            }
            return entry
        }
        guard let element = selectedElement, element.parent != nil else { return nil }
        return NavigationEntry(
            element: classicElementPath(element), items: activeItems.filter { !($0 is LinkItem) }.map(\.name))
    }

    /// Whether an entry still resolves in the document; one that fails is skipped, not removed.
    private func navigationValid(_ entry: NavigationEntry) -> Bool {
        if let tree = sceneTree {
            return NavigateResolve.scene(entry, in: tree) != nil
        }
        guard let model = elementStore?.model else { return false }
        return NavigateResolve.classic(entry, in: model) != nil
    }

    /// Records the selection as an entry unless one is being restored, and shows which of Back and Forward
    /// can go; go/app's navSync.
    private func navigationSync() {
        if !restoringNavigation, let entry = currentNavigationEntry() {
            navigation.push(entry)
        }
        updateNavigationMenu()
    }

    /// Follows the canvas selection; its one handler is the inspector's, which keeps its half.
    private func installNavigationHistory() {
        guard let canvas = editorViewController?.sceneCanvas?.canvas else { return }
        let inspector = canvas.onSelectionChange
        canvas.onSelectionChange = { [weak self] in
            inspector?()
            self?.navigationSync()
        }
        navigationSync()
    }

    /// Shows which of Back and Forward can go in the Navigate menu, as go/app's navSync sets its items.
    private func updateNavigationMenu() {
        guard let menu = NSApp.mainMenu?.items.compactMap(\.submenu).first(where: { $0.title == "Navigate" })
        else { return }
        let back = navigation.can(-1, valid: navigationValid)
        let forward = navigation.can(1, valid: navigationValid)
        for item in menu.items {
            if item.action == #selector(navigateBack(_:)) {
                item.isEnabled = back
            } else if item.action == #selector(navigateForward(_:)) {
                item.isEnabled = forward
            }
        }
    }

    /// Rewrites the history after a rename, as go/app's afterElementRename and afterNodeRename. An element
    /// rename moves the element names of the entries; a node rename the paths of it and below it.
    private func navigationRenamed(wasElement: Bool, element: [String], node: String, name: String) {
        if wasElement {
            guard !element.isEmpty else { return }
            var now = element
            now[now.count - 1] = name
            navigation.elementRenamed(from: element, to: now)
            navigationSync()
            return
        }
        guard !node.isEmpty else { return }
        guard let cut = node.lastIndex(of: ".") else {
            navigation.nodeRenamed(in: element, from: node, to: normalizeName(name))
            return
        }
        navigation.nodeRenamed(in: element, from: node, to: String(node[..<cut]) + "." + normalizeName(name))
    }

    public func setElementModel(elementStore: ElementModelStore) {

        if let es = self.elementStore, es.model == elementStore.model {
            return
        }
        // Unregister from previous store listeners if needed
        if let old = self.elementStore {
            old.onUpdate.removeAll(where: { ($0 as AnyObject) === (self as AnyObject) })
        }
        self.elementStore = elementStore

        if let um = self.undoManager {
            um.removeAllActions()
        }
        if self.scene == nil {
            return
        }
        // Cleanup Undo stack
        if let um = self.undoManager {
            um.removeAllActions()
        }

        // Register as listener if not already present (avoid duplicates)
        if !(elementStore.onUpdate.contains(where: { ($0 as AnyObject) === (self as AnyObject) })) {
            elementStore.onUpdate.append(self)
        }

        scene.setModel(store: self.elementStore!)
        scene.onSelection.removeAll()
        scene.onSelection.append({ (element) -> Void in
            self.activeItems = element
            self.inspectorController?.showClassic(items: element, element: self.selectedElement)
            self.updateTextProperties()
            self.navigationSync()
        })

        scene.setActiveElement(elementStore.model)
        inspectorController?.showClassic(items: scene.activeItems, element: self.selectedElement)

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
        if document?.sceneTree == nil {
            inspectorController?.refresh()
        }
        if self.updatingProperties {
            self.updateWindowTitle()
            return
        }
        //TODO: Add optimizations based on particular element

        self.updateElements.append(evt.element)
        if self.updateScheduled == 0 || (self.updateKindScheduled == .Layout && evt.kind == .Structure) {
            self.updateKindScheduled = evt.kind
            self.updateScheduled = 1

            DispatchQueue.main.asyncAfter(
                deadline: .now() + 0.2,
                execute: {

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
                        self.worldTree.selectRowIndexes(
                            IndexSet.init(arrayLiteral: childIndex),
                            byExtendingSelection: false)
                    }

                    self.updateScheduled = 0

                    self.updateWindowTitle()
                })
        }
    }

    func mergeProperties(_ node: TennNode) {
        updatingProperties = true
        // setProperties takes an optional manager: there is no undo stack until the window
        // is key, and force-unwrapping here crashed the edit.
        if let active = activeItems.first {
            if let element = self.selectedElement {
                self.elementStore?.setProperties(element, active, node, undoManager: undoManager, refresh: { () -> Void in })
            }
        } else if let element = self.selectedElement {
            self.elementStore?.setProperties(element, node, undoManager: undoManager, refresh: { () -> Void in })
        }
        updatingProperties = false
    }
}
