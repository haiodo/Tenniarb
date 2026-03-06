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

class ViewController: NSViewController, IElementModelListener, NSMenuItemValidation, NSSplitViewDelegate {

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

    weak var zoomLabel: NSTextField!

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

    override func loadView() {
        // Build UI programmatically to match storyboard structure
        let root = NSView(frame: NSRect(x: 0, y: 0, width: 944, height: 764))
        root.wantsLayer = true
        self.view = root

        // Visual effect background (matches storyboard: blendingMode="behindWindow" material="underWindowBackground")
        let vfx = NSVisualEffectView(frame: .zero)
        vfx.translatesAutoresizingMaskIntoConstraints = false
        vfx.blendingMode = .behindWindow
        vfx.material = .underWindowBackground
        vfx.state = .followsWindowActiveState
        root.addSubview(vfx)

        NSLayoutConstraint.activate([
            vfx.leadingAnchor.constraint(equalTo: root.leadingAnchor),
            vfx.trailingAnchor.constraint(equalTo: root.trailingAnchor),
            vfx.topAnchor.constraint(equalTo: root.topAnchor),
            vfx.bottomAnchor.constraint(equalTo: root.bottomAnchor)
        ])

        // Main split view (left: outline, right: editor) - vertical=YES means left/right split
        let splitView = NSSplitView(frame: .zero)
        splitView.translatesAutoresizingMaskIntoConstraints = false
        splitView.isVertical = true
        splitView.dividerStyle = .thin
        vfx.addSubview(splitView)

        NSLayoutConstraint.activate([
            splitView.leadingAnchor.constraint(equalTo: vfx.leadingAnchor),
            splitView.trailingAnchor.constraint(equalTo: vfx.trailingAnchor),
            splitView.topAnchor.constraint(equalTo: vfx.topAnchor),
            splitView.bottomAnchor.constraint(equalTo: vfx.bottomAnchor)
        ])

        // MARK: - Left panel (outline tree)
        let leftStackView = NSStackView()
        leftStackView.translatesAutoresizingMaskIntoConstraints = false
        leftStackView.orientation = .vertical
        leftStackView.alignment = .leading
        leftStackView.spacing = 0
        leftStackView.distribution = .fill
        splitView.addArrangedSubview(leftStackView)

        // Top toolbar box in left panel (30px height as in storyboard)
        let leftToolbarBox = NSBox()
        leftToolbarBox.translatesAutoresizingMaskIntoConstraints = false
        leftToolbarBox.boxType = .custom
        leftToolbarBox.borderType = .noBorder
        leftToolbarBox.titlePosition = .noTitle
        leftToolbarBox.fillColor = .clear
        leftStackView.addArrangedSubview(leftToolbarBox)
        leftToolbarBox.heightAnchor.constraint(equalToConstant: 30).isActive = true
        leftToolbarBox.widthAnchor.constraint(equalTo: leftStackView.widthAnchor).isActive = true

        // Left toolbar segmented control (add/remove only, 2 segments as in storyboard)
        let leftToolbar = NSSegmentedControl(labels: ["", ""], trackingMode: .momentary, target: self, action: #selector(leftToolbarAction(_:)))
        leftToolbar.segmentStyle = .texturedRounded
        leftToolbar.translatesAutoresizingMaskIntoConstraints = false
        if let addImg = NSImage(named: NSImage.addTemplateName) {
            leftToolbar.setImage(addImg, forSegment: 0)
        }
        if let removeImg = NSImage(named: NSImage.removeTemplateName) {
            leftToolbar.setImage(removeImg, forSegment: 1)
        }
        leftToolbarBox.contentView!.addSubview(leftToolbar)
        leftToolbar.trailingAnchor.constraint(equalTo: leftToolbarBox.contentView!.trailingAnchor, constant: 0).isActive = true
        leftToolbar.topAnchor.constraint(equalTo: leftToolbarBox.contentView!.topAnchor, constant: 0).isActive = true

        // Outline scroll view with transparent background
        let outlineScroll = NSScrollView()
        outlineScroll.translatesAutoresizingMaskIntoConstraints = false
        outlineScroll.hasVerticalScroller = true
        outlineScroll.hasHorizontalScroller = false
        outlineScroll.autohidesScrollers = true
        outlineScroll.borderType = .noBorder
        outlineScroll.drawsBackground = false  // Transparent background
        outlineScroll.contentView.drawsBackground = false
        leftStackView.addArrangedSubview(outlineScroll)

        // Outline view with transparent background (matches storyboard backgroundColor alpha=0)
        let outline = OutlineNSOutlineView(frame: .zero)
        outline.translatesAutoresizingMaskIntoConstraints = false
        let col = NSTableColumn(identifier: NSUserInterfaceItemIdentifier(rawValue: "YFI-la-VX8"))
        col.title = "Element"
        col.width = 185
        col.minWidth = 140
        col.maxWidth = 300
        outline.addTableColumn(col)
        outline.outlineTableColumn = col
        outline.headerView = nil
        outline.backgroundColor = NSColor.clear  // Transparent to show vibrancy
        outline.selectionHighlightStyle = .sourceList
        outline.rowHeight = 22
        outline.intercellSpacing = NSSize(width: 0, height: 5)
        outline.indentationPerLevel = 10
        outlineScroll.documentView = outline

        // MARK: - Right panel (editor area) - uses NSSplitView like storyboard
        // The right side has a horizontal split: bottom = editor stack, top = text properties
        // In storyboard: splitView with fixedFrame, stackView at y=0 height=628, scrollView at y=637 height=127
        let rightSplitView = NSSplitView()
        rightSplitView.isVertical = false  // false = top/bottom split (horizontal divider)
        rightSplitView.dividerStyle = .thin
        splitView.addArrangedSubview(rightSplitView)

        // Configure split view behavior
        splitView.delegate = self
        mainSplitView = splitView
        self.editorSplitView = rightSplitView

        // Calculate initial sizes based on expected right panel size (~718x764)
        let rightWidth: CGFloat = 718
        let rightHeight: CGFloat = 764
        let textScrollHeight: CGFloat = 127
        let dividerThickness: CGFloat = 1
        let editorHeight = rightHeight - textScrollHeight - dividerThickness

        // Bottom part: Editor stack view (contains title bar and scene view)
        // First subview in NSSplitView with isVertical=false is at bottom (y=0)
        let editorStackView = NSStackView(frame: NSRect(x: 0, y: 0, width: rightWidth, height: editorHeight))
        editorStackView.orientation = .vertical
        editorStackView.alignment = .centerX  // Center alignment to avoid leading edge conflicts
        editorStackView.spacing = 0
        editorStackView.distribution = .fill
        editorStackView.autoresizingMask = [.width, .height]
        rightSplitView.addSubview(editorStackView)

        // Top part: Properties panel with text editor and zoom toolbar
        // Container stack view for text scroll and zoom toolbar
        let propertiesContainer = NSStackView(frame: NSRect(x: 0, y: editorHeight + dividerThickness, width: rightWidth, height: textScrollHeight))
        propertiesContainer.orientation = .vertical
        propertiesContainer.alignment = .centerX
        propertiesContainer.spacing = 0
        propertiesContainer.distribution = .fill
        propertiesContainer.autoresizingMask = [.width, .height]
        rightSplitView.addSubview(propertiesContainer)

        // Text properties scroll view (fills most of the space)
        let textScroll = NSScrollView()
        textScroll.translatesAutoresizingMaskIntoConstraints = false
        textScroll.hasVerticalScroller = true
        textScroll.hasHorizontalScroller = false
        textScroll.autohidesScrollers = true
        textScroll.borderType = .noBorder
        textScroll.drawsBackground = false
        // Semi-transparent background for clip view (matches storyboard alpha=0.80171767979452058)
        textScroll.contentView.wantsLayer = true
        textScroll.contentView.layer?.backgroundColor = NSColor(red: 1.0, green: 1.0, blue: 1.0, alpha: 0.8).cgColor
        textScroll.contentView.drawsBackground = false
        propertiesContainer.addArrangedSubview(textScroll)

        // Text view for properties (drawsBackground=NO as in storyboard)
        let textView = TennTextView(frame: NSRect(x: 0, y: 0, width: rightWidth, height: textScrollHeight))
        textView.drawsBackground = false
        textView.isVerticallyResizable = true
        textView.isHorizontallyResizable = true
        textView.autoresizingMask = [.width, .height]
        textView.textContainer?.widthTracksTextView = true
        textView.textContainer?.heightTracksTextView = false
        textView.maxSize = NSSize(width: CGFloat.greatestFiniteMagnitude, height: CGFloat.greatestFiniteMagnitude)
        textView.minSize = NSSize(width: rightWidth, height: textScrollHeight)
        textScroll.documentView = textView

        // Set holding priorities: both to 1 as in storyboard
        rightSplitView.setHoldingPriority(NSLayoutConstraint.Priority(1), forSubviewAt: 0)
        rightSplitView.setHoldingPriority(NSLayoutConstraint.Priority(1), forSubviewAt: 1)

        // Title bar box (34px height as in storyboard, transparent, contains title and buttons)
        let titleBar = NSBox()
        titleBar.translatesAutoresizingMaskIntoConstraints = false
        titleBar.boxType = .custom
        titleBar.borderType = .noBorder
        titleBar.titlePosition = .noTitle
        titleBar.fillColor = .clear
        editorStackView.addArrangedSubview(titleBar)
        let titleBarHeightConstraint = titleBar.heightAnchor.constraint(equalToConstant: 34)
        titleBarHeightConstraint.priority = NSLayoutConstraint.Priority(999)  // High but not required, to allow recovery during init
        titleBarHeightConstraint.isActive = true
        // Width constraint with lower priority to avoid conflicts during initial layout
        let titleBarWidthConstraint = titleBar.widthAnchor.constraint(equalTo: editorStackView.widthAnchor)
        titleBarWidthConstraint.priority = .defaultHigh
        titleBarWidthConstraint.isActive = true

        // Window title text field (font size 15, labelColor as in storyboard)
        let titleField = NSTextField(string: "Window title")
        titleField.translatesAutoresizingMaskIntoConstraints = false
        titleField.isBordered = false
        titleField.isEditable = false
        titleField.isSelectable = false
        titleField.drawsBackground = false
        titleField.focusRingType = .none
        titleField.font = NSFont.systemFont(ofSize: 15)
        titleField.textColor = NSColor.labelColor
        titleField.cell?.lineBreakMode = .byClipping
        titleField.cell?.isScrollable = true
        titleBar.contentView!.addSubview(titleField)

        NSLayoutConstraint.activate([
            titleField.leadingAnchor.constraint(equalTo: titleBar.contentView!.leadingAnchor, constant: 15),
            titleField.centerYAnchor.constraint(equalTo: titleBar.contentView!.centerYAnchor)
        ])

        // Zoom controls group in title bar (as a single unit)
        let zoomStack = NSStackView()
        zoomStack.translatesAutoresizingMaskIntoConstraints = false
        zoomStack.orientation = .horizontal
        zoomStack.spacing = 2
        zoomStack.alignment = .centerY
        zoomStack.distribution = .fill
        titleBar.contentView!.addSubview(zoomStack)

        let zoomOutButton = NSButton()
        zoomOutButton.bezelStyle = .texturedRounded
        zoomOutButton.image = NSImage(named: NSImage.removeTemplateName)
        zoomOutButton.imagePosition = .imageOnly
        zoomOutButton.target = self
        zoomOutButton.action = #selector(zoomOutAction(_:))
        zoomStack.addArrangedSubview(zoomOutButton)

        let zoomLabel = NSTextField()
        zoomLabel.translatesAutoresizingMaskIntoConstraints = false
        zoomLabel.isEditable = false
        zoomLabel.isSelectable = false
        zoomLabel.isBordered = false
        zoomLabel.drawsBackground = false
        zoomLabel.alignment = .center
        zoomLabel.font = NSFont.systemFont(ofSize: 11)
        zoomLabel.textColor = NSColor.labelColor
        zoomLabel.stringValue = "100%"
        zoomLabel.widthAnchor.constraint(equalToConstant: 40).isActive = true
        zoomStack.addArrangedSubview(zoomLabel)

        let zoomInButton = NSButton()
        zoomInButton.bezelStyle = .texturedRounded
        zoomInButton.image = NSImage(named: NSImage.addTemplateName)
        zoomInButton.imagePosition = .imageOnly
        zoomInButton.target = self
        zoomInButton.action = #selector(zoomInAction(_:))
        zoomStack.addArrangedSubview(zoomInButton)

        let resetZoomButton = NSButton()
        resetZoomButton.bezelStyle = .texturedRounded
        resetZoomButton.title = "100%"
        resetZoomButton.font = NSFont.systemFont(ofSize: 11)
        resetZoomButton.target = self
        resetZoomButton.action = #selector(resetZoomAction(_:))
        zoomStack.addArrangedSubview(resetZoomButton)

        // Extra buttons (add/remove) on right side of title bar
        let extraSeg = NSSegmentedControl(labels: ["", ""], trackingMode: .momentary, target: self, action: #selector(extraSegmentAction(_:)))
        extraSeg.segmentStyle = .texturedRounded
        extraSeg.controlSize = .regular
        extraSeg.translatesAutoresizingMaskIntoConstraints = false
        // Set segment widths to match storyboard (20px each)
        extraSeg.setWidth(20, forSegment: 0)
        extraSeg.setWidth(20, forSegment: 1)
        if let addImg = NSImage(named: NSImage.addTemplateName) { extraSeg.setImage(addImg, forSegment: 0) }
        if let remImg = NSImage(named: NSImage.removeTemplateName) { extraSeg.setImage(remImg, forSegment: 1) }
        titleBar.contentView!.addSubview(extraSeg)

        // Help button (bookmarks icon as in storyboard)
        let helpSeg = NSSegmentedControl(labels: [""], trackingMode: .momentary, target: self, action: #selector(showHelp(_:)))
        helpSeg.segmentStyle = .texturedRounded
        helpSeg.controlSize = .regular
        helpSeg.translatesAutoresizingMaskIntoConstraints = false
        // Set segment width to match storyboard (20px)
        helpSeg.setWidth(20, forSegment: 0)
        if let bookmarkImg = NSImage(named: NSImage.bookmarksTemplateName) {
            helpSeg.setImage(bookmarkImg, forSegment: 0)
        }
        titleBar.contentView!.addSubview(helpSeg)

        // Export segmented control (Share)
        let exportControl = NSSegmentedControl(labels: [""], trackingMode: .momentary, target: nil, action: nil)
        exportControl.segmentStyle = .texturedRounded
        exportControl.controlSize = .regular
        exportControl.translatesAutoresizingMaskIntoConstraints = false
        // Set segment width to match storyboard (20px)
        exportControl.setWidth(20, forSegment: 0)
        if let shareImg = NSImage(named: NSImage.shareTemplateName) {
            exportControl.setImage(shareImg, forSegment: 0)
        }
        titleBar.contentView!.addSubview(exportControl)

        // Layout: [Title] ... [ZoomGroup] [Help] [Share] [+/-]
        NSLayoutConstraint.activate([
            zoomStack.leadingAnchor.constraint(greaterThanOrEqualTo: titleField.trailingAnchor, constant: 20),
            zoomStack.centerYAnchor.constraint(equalTo: titleBar.contentView!.centerYAnchor),
            
            helpSeg.leadingAnchor.constraint(greaterThanOrEqualTo: zoomStack.trailingAnchor, constant: 16),
            helpSeg.centerYAnchor.constraint(equalTo: titleBar.contentView!.centerYAnchor),
            
            exportControl.leadingAnchor.constraint(greaterThanOrEqualTo: helpSeg.trailingAnchor, constant: 16),
            exportControl.centerYAnchor.constraint(equalTo: titleBar.contentView!.centerYAnchor),
            
            extraSeg.leadingAnchor.constraint(greaterThanOrEqualTo: exportControl.trailingAnchor, constant: 16),
            extraSeg.trailingAnchor.constraint(equalTo: titleBar.contentView!.trailingAnchor, constant: -10),
            extraSeg.centerYAnchor.constraint(equalTo: titleBar.contentView!.centerYAnchor)
        ])

        // Scene drawing view (fills remaining space in editor stack)
        let sceneView = SceneDrawView(frame: .zero)
        sceneView.translatesAutoresizingMaskIntoConstraints = false
        editorStackView.addArrangedSubview(sceneView)

        // Scene view should expand to fill available space
        sceneView.setContentHuggingPriority(.defaultLow, for: .vertical)
        sceneView.setContentCompressionResistancePriority(.defaultLow, for: .vertical)

        // Width constraint for scene view with lower priority to avoid conflicts
        let sceneWidthConstraint = sceneView.widthAnchor.constraint(equalTo: editorStackView.widthAnchor, constant: -5)
        sceneWidthConstraint.priority = .defaultHigh
        sceneWidthConstraint.isActive = true

        // Wire up properties used elsewhere in the controller
        self.scene = sceneView
        self.windowTitle = titleField
        self.toolsSegmentedControl = leftToolbar
        self.worldTree = outline
        self.textView = textView
        self.exportSegments = exportControl.cell as? NSSegmentedCell
        self.zoomLabel = zoomLabel

        // Set initial split positions after layout
        DispatchQueue.main.async {
            // Left panel width: 217px as in storyboard
            splitView.setPosition(217, ofDividerAt: 0)

            // Right split: position divider so editor gets most space, text scroll gets 127px at top
            // setPosition for horizontal split (isVertical=false) sets position from bottom
            let totalHeight = rightSplitView.bounds.height
            if totalHeight > 0 {
                let textScrollHeight: CGFloat = 127
                // Position = height of bottom pane (editor)
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

    func splitView(_ splitView: NSSplitView, constrainMinCoordinate proposedMinimumPosition: CGFloat, ofSubviewAt dividerIndex: Int) -> CGFloat {
        if splitView == mainSplitView {
            // Minimum left panel width
            return 150
        } else if splitView == editorSplitView {
            // Minimum height for editor area (bottom part)
            return 200
        }
        return proposedMinimumPosition
    }

    func splitView(_ splitView: NSSplitView, constrainMaxCoordinate proposedMaximumPosition: CGFloat, ofSubviewAt dividerIndex: Int) -> CGFloat {
        if splitView == mainSplitView {
            // Maximum left panel width
            return 400
        } else if splitView == editorSplitView {
            // Maximum height for editor area, leaving at least 50px for text
            return splitView.frame.height - 50
        }
        return proposedMaximumPosition
    }

    override func viewDidLoad() {
        super.viewDidLoad()

        self.outlineViewDelegate = OutlineViewControllerDelegate(self)
        worldTree.delegate = self.outlineViewDelegate
        worldTree.dataSource = self.outlineViewDelegate

        self.textViewDelegate = TextPropertiesDelegate(self, self.textView!)

        scene.onLoad(self)

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

        DistributedNotificationCenter.default().addObserver(self, selector: #selector(darkModeChanged), name: NSNotification.Name(rawValue: "AppleInterfaceThemeChangedNotification"), object: nil)
    }
    deinit {
        // Unregister distributed notifications
        DistributedNotificationCenter.default().removeObserver(self)
        // Unregister from element store listeners if present
        if let es = self.elementStore {
            es.onUpdate.removeAll(where: { ($0 as AnyObject) === (self as AnyObject) })
        }
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

    @IBAction func pasteAsItem(_ sender: NSMenuItem ) {
        scene?.pasteAsItem(sender)
    }
    @IBAction func pasteAsItemSet(_ sender: NSMenuItem ) {
        scene?.pasteAsItemSet(sender)
    }

    @IBAction func selectAllItemsKind(_ sender: NSMenuItem ) {
        scene.selectAllByKind(kind: ItemKind.Item)
    }

    @IBAction func selectAllLinks(_ sender: NSMenuItem ) {
        scene.selectAllByKind(kind: ItemKind.Link)
    }

    @IBAction func showSearchBox(_ sender: NSMenuItem ) {
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

    @IBAction func showOperationBox(_ sender: NSMenuItem ) {
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
            self.present(operations, asPopoverRelativeTo: NSRect(origin: bounds.origin, size: bounds.size),
                         of: self.scene, preferredEdge: .minY, behavior: .transient)
        }
    }

    @objc func showHelp(_ sender: Any?) {
        // Show help in a separate window (programmatic)
        let helpController = HelpController()
        let helpWindow = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 800, height: 600),
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
            if action == #selector(selectAllLinks) || action == #selector(selectAllItemsKind) {
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
        // Unregister from previous store listeners if needed
        if let old = self.elementStore {
            old.onUpdate.removeAll(where: { ($0 as AnyObject) === (self as AnyObject) })
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

        // Register as listener if not already present (avoid duplicates)
        if !(elementStore.onUpdate.contains(where: { ($0 as AnyObject) === (self as AnyObject) })) {
            elementStore.onUpdate.append(self)
        }

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
