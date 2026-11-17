//
//  OutlineSidebarController.swift
//  Tenniarb
//
//  Created by Assistant on 07/03/2026.
//  Copyright © 2026 Andrey Sobolev. All rights reserved.
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

@MainActor
class OutlineSidebarController: NSObject {
    weak var viewController: ViewController?

    private(set) var outlineView: NSOutlineView!
    private(set) var toolbar: NSSegmentedControl!
    private(set) var containerView: NSView!
    private(set) var itemsOutlineView: NSOutlineView!
    /// The switch of the Items pane: the drawn items, or the components to insert.
    private var itemsModeControl: NSSegmentedControl!
    /// The picture of the component the pane has selected; shown in the components mode only.
    private var itemsPreview: NSImageView!
    private var itemsPreviewBox: NSView!

    private var outlineViewDelegate: OutlineViewControllerDelegate?
    private(set) var itemsDelegate: SceneItemsDelegate?

    func createView() -> NSView {
        let stackView = NSStackView()
        stackView.translatesAutoresizingMaskIntoConstraints = false
        stackView.orientation = .vertical
        stackView.alignment = .leading
        stackView.spacing = 0
        stackView.distribution = .fill
        self.containerView = stackView

        // Top toolbar box
        let toolbarBox = NSBox()
        toolbarBox.translatesAutoresizingMaskIntoConstraints = false
        toolbarBox.boxType = .custom
        toolbarBox.borderWidth = 0
        toolbarBox.titlePosition = .noTitle
        toolbarBox.fillColor = .clear
        stackView.addArrangedSubview(toolbarBox)
        toolbarBox.heightAnchor.constraint(equalToConstant: 30).isActive = true
        toolbarBox.widthAnchor.constraint(equalTo: stackView.widthAnchor).isActive = true

        // Toolbar segmented control
        let toolbarControl = NSSegmentedControl(
            labels: ["", ""],
            trackingMode: .momentary,
            target: self,
            action: #selector(toolbarAction(_:))
        )
        toolbarControl.segmentStyle = .texturedRounded
        toolbarControl.translatesAutoresizingMaskIntoConstraints = false
        if let addImg = NSImage(named: NSImage.addTemplateName) {
            toolbarControl.setImage(addImg, forSegment: 0)
        }
        if let removeImg = NSImage(named: NSImage.removeTemplateName) {
            toolbarControl.setImage(removeImg, forSegment: 1)
        }
        toolbarBox.contentView!.addSubview(toolbarControl)
        toolbarControl.trailingAnchor.constraint(equalTo: toolbarBox.contentView!.trailingAnchor).isActive = true
        toolbarControl.topAnchor.constraint(equalTo: toolbarBox.contentView!.topAnchor).isActive = true
        self.toolbar = toolbarControl

        let outlineScroll = NSScrollView()
        outlineScroll.translatesAutoresizingMaskIntoConstraints = false
        outlineScroll.hasVerticalScroller = true
        outlineScroll.hasHorizontalScroller = false
        outlineScroll.autohidesScrollers = true
        outlineScroll.borderType = .noBorder
        outlineScroll.drawsBackground = false
        outlineScroll.contentView.drawsBackground = false
        stackView.addArrangedSubview(outlineScroll)

        let outline = OutlineNSOutlineView(frame: .zero)
        outline.translatesAutoresizingMaskIntoConstraints = false
        let col = NSTableColumn(identifier: NSUserInterfaceItemIdentifier(rawValue: "OutlineColumn"))
        col.title = "Element"
        col.width = 185
        col.minWidth = 140
        col.maxWidth = 300
        outline.addTableColumn(col)
        outline.outlineTableColumn = col
        outline.headerView = nil
        outline.backgroundColor = NSColor.clear
        outline.style = .sourceList
        outline.rowHeight = 22
        outline.intercellSpacing = NSSize(width: 0, height: 5)
        outline.indentationPerLevel = 10
        outlineScroll.documentView = outline

        self.outlineView = outline

        // The items of the scene on the canvas, under the elements: go/app/items.go, second level of
        // navigation.
        let itemsHeader = NSBox()
        itemsHeader.translatesAutoresizingMaskIntoConstraints = false
        itemsHeader.boxType = .custom
        itemsHeader.borderWidth = 0
        itemsHeader.titlePosition = .noTitle
        itemsHeader.fillColor = .clear
        stackView.addArrangedSubview(itemsHeader)
        itemsHeader.heightAnchor.constraint(equalToConstant: 22).isActive = true
        itemsHeader.widthAnchor.constraint(equalTo: stackView.widthAnchor).isActive = true
        let mode = NSSegmentedControl(
            labels: ["Items", "Components"], trackingMode: .selectOne, target: self,
            action: #selector(itemsModeChanged(_:)))
        mode.translatesAutoresizingMaskIntoConstraints = false
        mode.segmentStyle = .texturedRounded
        mode.controlSize = .small
        mode.font = NSFont.systemFont(ofSize: 11)
        mode.selectedSegment = 0
        itemsHeader.contentView!.addSubview(mode)
        mode.leadingAnchor.constraint(equalTo: itemsHeader.contentView!.leadingAnchor, constant: 6).isActive = true
        mode.trailingAnchor.constraint(equalTo: itemsHeader.contentView!.trailingAnchor, constant: -6).isActive = true
        mode.centerYAnchor.constraint(equalTo: itemsHeader.contentView!.centerYAnchor).isActive = true
        self.itemsModeControl = mode

        // The picture of the component picked in the browser, above the rows it was picked from.
        let previewBox = NSView()
        previewBox.translatesAutoresizingMaskIntoConstraints = false
        previewBox.isHidden = true
        stackView.addArrangedSubview(previewBox)
        previewBox.heightAnchor.constraint(equalToConstant: 104).isActive = true
        previewBox.widthAnchor.constraint(equalTo: stackView.widthAnchor).isActive = true
        let preview = NSImageView()
        preview.translatesAutoresizingMaskIntoConstraints = false
        preview.imageScaling = .scaleProportionallyUpOrDown
        previewBox.addSubview(preview)
        preview.centerXAnchor.constraint(equalTo: previewBox.centerXAnchor).isActive = true
        preview.centerYAnchor.constraint(equalTo: previewBox.centerYAnchor).isActive = true
        preview.widthAnchor.constraint(equalToConstant: 160).isActive = true
        preview.heightAnchor.constraint(equalToConstant: 100).isActive = true
        self.itemsPreviewBox = previewBox
        self.itemsPreview = preview

        let itemsScroll = NSScrollView()
        itemsScroll.translatesAutoresizingMaskIntoConstraints = false
        itemsScroll.hasVerticalScroller = true
        itemsScroll.hasHorizontalScroller = false
        itemsScroll.autohidesScrollers = true
        itemsScroll.borderType = .noBorder
        itemsScroll.drawsBackground = false
        itemsScroll.contentView.drawsBackground = false
        stackView.addArrangedSubview(itemsScroll)
        itemsScroll.heightAnchor.constraint(equalToConstant: 180).isActive = true
        itemsScroll.widthAnchor.constraint(equalTo: stackView.widthAnchor).isActive = true

        let items = SceneItemsOutlineView(frame: .zero)
        items.translatesAutoresizingMaskIntoConstraints = false
        let itemsCol = NSTableColumn(identifier: NSUserInterfaceItemIdentifier(rawValue: "ItemsColumn"))
        itemsCol.title = "Item"
        itemsCol.width = 185
        items.addTableColumn(itemsCol)
        items.outlineTableColumn = itemsCol
        items.headerView = nil
        items.backgroundColor = NSColor.clear
        items.style = .sourceList
        items.rowHeight = 20
        items.intercellSpacing = NSSize(width: 0, height: 2)
        items.indentationPerLevel = 10
        itemsScroll.documentView = items
        self.itemsOutlineView = items

        return stackView
    }

    func setupDelegate() {
        guard let vc = viewController else { return }

        self.outlineViewDelegate = OutlineViewControllerDelegate(vc)
        outlineView.delegate = self.outlineViewDelegate
        outlineView.dataSource = self.outlineViewDelegate

        self.itemsDelegate = SceneItemsDelegate(vc, sidebar: self)
        itemsOutlineView.delegate = self.itemsDelegate
        itemsOutlineView.dataSource = self.itemsDelegate
        itemsOutlineView.target = self.itemsDelegate
        itemsOutlineView.doubleAction = #selector(SceneItemsDelegate.activateRow(_:))
        (itemsOutlineView as? SceneItemsOutlineView)?.onReturn = { [weak itemsDelegate] in
            itemsDelegate?.activateSelected()
        }
        itemsOutlineView.setDraggingSourceOperationMask(.copy, forLocal: true)
    }

    /// Lists the items of the scene on the canvas; nil clears the pane.
    func showItems(_ canvas: SceneCanvasView?) {
        itemsDelegate?.show(canvas)
    }

    /// The rows the Items pane shows now; read by tests.
    var itemsRows: [SceneItemsDelegate.Row] { itemsDelegate?.rows ?? [] }

    /// Whether the pane lists the components to insert instead of the drawn items.
    var showsComponents: Bool { itemsDelegate?.components ?? false }

    /// Switches the pane between the drawn items and the components to insert, and reloads it.
    func setItemsMode(_ components: Bool) {
        guard let itemsDelegate, itemsDelegate.components != components else { return }
        itemsDelegate.setComponents(components)
        itemsModeControl.selectedSegment = components ? 1 : 0
        itemsPreviewBox.isHidden = !components
        showPreview(itemsDelegate.selectedComponentUsePath)
    }

    /// The picture of the built component at path, fitted into the pane; read by tests.
    func previewImage(path: String, size: NSSize = NSSize(width: 160, height: 100)) -> NSImage? {
        itemsDelegate?.previewImage(path: path, size: size)
    }

    /// Shows the component at path over the list; nil clears the picture.
    fileprivate func showPreview(_ path: String?) {
        guard showsComponents else {
            itemsPreview.image = nil
            return
        }
        if let path, let image = itemsDelegate?.previewImage(path: path) {
            itemsPreview.contentTintColor = nil
            itemsPreview.image = image
        } else {
            // A component of slots is empty until it is filled; the puzzle says what the row is.
            itemsPreview.contentTintColor = .tertiaryLabelColor
            itemsPreview.image =
                path == nil
                ? nil : NSImage(systemSymbolName: "puzzlepiece", accessibilityDescription: "component")
        }
    }

    @objc private func itemsModeChanged(_ sender: NSSegmentedControl) {
        setItemsMode(sender.selectedSegment == 1)
    }

    @objc private func toolbarAction(_ sender: NSSegmentedControl) {
        switch sender.selectedSegment {
        case 0:
            viewController?.handleAddElement()
        case 1:
            viewController?.handleRemoveElement()
        default:
            break
        }
    }

    // MARK: - Public API

    func reloadData() {
        outlineView?.reloadData()
    }

    func reloadItem(_ item: Any?, reloadChildren: Bool = false) {
        outlineView?.reloadItem(item, reloadChildren: reloadChildren)
    }

    func expandItem(_ item: Any?, expandChildren: Bool = false) {
        outlineView?.expandItem(item, expandChildren: expandChildren)
    }

    func selectRowIndexes(_ indexes: IndexSet, byExtendingSelection extend: Bool) {
        outlineView?.selectRowIndexes(indexes, byExtendingSelection: extend)
    }

    func selectedRow() -> Int {
        return outlineView?.selectedRow ?? -1
    }

    func item(atRow row: Int) -> Any? {
        return outlineView?.item(atRow: row)
    }

    func row(forItem item: Any) -> Int {
        return outlineView?.row(forItem: item) ?? -1
    }

    func beginUpdates() {
        outlineView?.beginUpdates()
    }

    func endUpdates() {
        outlineView?.endUpdates()
    }

    func setSelectedElement(_ element: Element?) {
        if let el = element {
            let row = outlineView.row(forItem: el)
            if row >= 0 {
                outlineView.selectRowIndexes(IndexSet([row]), byExtendingSelection: false)
            }
        }
    }
}

/// The rows of the Items pane: the drawn items of the scene on the canvas, one row per item and the parts
/// of an instance under it, or the components a use can insert, grouped by the branch they are defined in.
@MainActor
final class SceneItemsDelegate: NSObject, NSOutlineViewDataSource, NSOutlineViewDelegate {
    /// What a row is: a drawn item, a heading of the browser, a component to insert, or the hint of an
    /// empty browser.
    enum Kind: String {
        case item, group, component, hint
    }

    /// One row: a drawn item and the rows under it, or a component to insert and the use path it writes.
    final class Row {
        let label: String
        let kind: Kind
        /// The index path from the root of the screen for a drawn item; empty otherwise.
        let path: [Int]
        /// The use path of a component row: what a double click inserts and a drag carries.
        let usePath: String
        var kids: [Row] = []

        var component: Bool { kind == .component }

        init(label: String, kind: Kind, path: [Int] = [], usePath: String = "") {
            self.label = label
            self.kind = kind
            self.path = path
            self.usePath = usePath
        }
    }

    /// The rows the pane shows now; read by tests.
    private(set) var rows: [Row] = []
    /// Whether the pane lists the components to insert instead of the drawn items.
    private(set) var components = false

    private weak var viewController: ViewController?
    private weak var sidebar: OutlineSidebarController?
    private weak var canvas: SceneCanvasView?
    private var previews: [String: NSImage] = [:]
    private var previewMisses = Set<String>()

    init(_ viewController: ViewController, sidebar: OutlineSidebarController) {
        self.viewController = viewController
        self.sidebar = sidebar
        super.init()
    }

    /// Lists the items of the scene on the canvas; nil clears the pane.
    func show(_ canvas: SceneCanvasView?) {
        self.canvas = canvas
        refresh()
    }

    /// Switches between the drawn items and the browser, and rebuilds the rows.
    func setComponents(_ on: Bool) {
        components = on
        // The rows of the other mode are no rows of this one; the index would point at an unrelated item.
        sidebar?.itemsOutlineView?.deselectAll(nil)
        refresh()
    }

    /// Rebuilds the rows and reloads the list, keeping the component the pane had selected when it is
    /// still offered.
    func refresh() {
        let outline = sidebar?.itemsOutlineView
        let keep = selectedComponentUsePath
        previews.removeAll()
        previewMisses.removeAll()
        rows = builtRows()
        outline?.reloadData()
        outline?.expandItem(nil, expandChildren: false)
        guard components, let outline else { return }
        let picked =
            rows.firstIndex { $0.component && $0.usePath == keep }
            ?? rows.firstIndex { $0.component }
        if let picked {
            outline.selectRowIndexes(IndexSet(integer: picked), byExtendingSelection: false)
        } else {
            outline.deselectAll(nil)
            sidebar?.showPreview(nil)
        }
    }

    /// The use path of the selected component, nil for another row or none.
    var selectedComponentUsePath: String? {
        guard let outline = sidebar?.itemsOutlineView, outline.selectedRow >= 0,
            let row = outline.item(atRow: outline.selectedRow) as? Row, row.component
        else { return nil }
        return row.usePath
    }

    private func builtRows() -> [Row] {
        components ? componentRows() : Self.itemRows(canvas, nil)
    }

    private static func itemRows(_ canvas: SceneCanvasView?, _ path: [Int]?) -> [Row] {
        (canvas?.itemRows(path) ?? []).map { row in
            let node = Row(label: row.label, kind: .item, path: row.path)
            if row.kids { node.kids = itemRows(canvas, row.path) }
            return node
        }
    }

    /// The components of the mounted libraries and of the document, under a heading when they are defined
    /// in a branch of one.
    private func componentRows() -> [Row] {
        guard let document = viewController?.document else { return [] }
        var out: [Row] = []
        var last = ""
        for c in document.components("", limit: 200) {
            if !c.group.isEmpty && c.group != last {
                out.append(Row(label: c.group, kind: .group))
            }
            last = c.group
            out.append(Row(label: c.name, kind: .component, usePath: c.path))
        }
        guard out.isEmpty else { return out }
        let hint =
            document.libraries().isEmpty
            ? "No component to insert: add a library with File > Add Library…"
            : "The libraries mount no component"
        return [Row(label: hint, kind: .hint)]
    }

    // MARK: - Preview

    /// The picture of the component at path, fitted into a size box; cached by path and size until the
    /// pane reloads. Nil when the component draws nothing.
    func previewImage(path: String, size: NSSize = NSSize(width: 160, height: 100)) -> NSImage? {
        guard let document = viewController?.document, let tree = document.sceneTree else { return nil }
        let key = "\(path)|\(Int(size.width))x\(Int(size.height))"
        if let hit = previews[key] { return hit }
        if previewMisses.contains(key) { return nil }
        guard let component = document.components("", limit: 200).first(where: { $0.path == path }),
            let image = ComponentPreview.image(tree: tree, node: component.node, size: size)
        else {
            previewMisses.insert(key)
            return nil
        }
        previews[key] = image
        return image
    }

    // MARK: - Activation

    /// A double click on a row, as the list sends it.
    @objc func activateRow(_ sender: Any?) {
        guard let outline = (sender as? NSOutlineView) ?? sidebar?.itemsOutlineView else { return }
        let index = outline.clickedRow >= 0 ? outline.clickedRow : outline.selectedRow
        guard index >= 0, let row = outline.item(atRow: index) as? Row else { return }
        activate(row)
    }

    /// Return on the selected row.
    func activateSelected() {
        guard let outline = sidebar?.itemsOutlineView, outline.selectedRow >= 0,
            let row = outline.item(atRow: outline.selectedRow) as? Row
        else { return }
        activate(row)
    }

    /// A component row inserts its use where Insert Component… would; a heading, a hint and a drawn item
    /// do nothing.
    func activate(_ row: Row) {
        guard row.component else { return }
        viewController?.insertComponentRow(path: row.usePath, name: row.label)
    }

    // MARK: - Data source

    func outlineView(_ outlineView: NSOutlineView, numberOfChildrenOfItem item: Any?) -> Int {
        (item as? Row)?.kids.count ?? rows.count
    }

    func outlineView(_ outlineView: NSOutlineView, child index: Int, ofItem item: Any?) -> Any {
        (item as? Row)?.kids[index] ?? rows[index]
    }

    func outlineView(_ outlineView: NSOutlineView, isItemExpandable item: Any) -> Bool {
        !((item as? Row)?.kids.isEmpty ?? true)
    }

    func outlineView(_ outlineView: NSOutlineView, objectValueFor tableColumn: NSTableColumn?, byItem item: Any?) -> Any? {
        (item as? Row)?.label
    }

    func outlineView(_ outlineView: NSOutlineView, shouldSelectItem item: Any) -> Bool {
        guard let row = item as? Row else { return false }
        return row.kind == .item || row.component
    }

    func outlineView(_ outlineView: NSOutlineView, viewFor tableColumn: NSTableColumn?, item: Any) -> NSView? {
        let identifier = NSUserInterfaceItemIdentifier(rawValue: "SceneItemCell")
        var cell = outlineView.makeView(withIdentifier: identifier, owner: self) as? NSTableCellView
        if cell == nil {
            let view = NSTableCellView(frame: NSRect(x: 0, y: 0, width: 180, height: 20))
            view.identifier = identifier
            let text = NSTextField(labelWithString: "")
            text.translatesAutoresizingMaskIntoConstraints = false
            text.font = NSFont.menuFont(ofSize: 13)
            text.lineBreakMode = .byTruncatingTail
            view.addSubview(text)
            view.textField = text
            NSLayoutConstraint.activate([
                text.leadingAnchor.constraint(equalTo: view.leadingAnchor, constant: 2),
                text.trailingAnchor.constraint(equalTo: view.trailingAnchor, constant: -4),
                text.centerYAnchor.constraint(equalTo: view.centerYAnchor),
            ])
            cell = view
        }
        let row = item as? Row
        cell?.textField?.stringValue = row?.label ?? ""
        cell?.textField?.textColor = row?.kind == .group || row?.kind == .hint ? .secondaryLabelColor : .labelColor
        return cell
    }

    func outlineView(_ outlineView: NSOutlineView, rowViewForItem item: Any) -> NSTableRowView? {
        OutlineNSTableRowView()
    }

    /// Rows of the browser carry their use path, which the canvas reads on a drop.
    func outlineView(_ outlineView: NSOutlineView, pasteboardWriterForItem item: Any) -> NSPasteboardWriting? {
        guard components, let row = item as? Row, row.component else { return nil }
        let pasteboard = NSPasteboardItem()
        pasteboard.setString(row.usePath, forType: .tenniarbComponent)
        pasteboard.setString(row.label, forType: .tenniarbComponentName)
        return pasteboard
    }

    func outlineViewSelectionDidChange(_ notification: Notification) {
        guard let outline = notification.object as? NSOutlineView, outline.selectedRow >= 0,
            let row = outline.item(atRow: outline.selectedRow) as? Row
        else {
            sidebar?.showPreview(nil)
            return
        }
        guard row.component else {
            sidebar?.showPreview(nil)
            if !row.path.isEmpty {
                viewController?.selectSceneItem(row.path)
            }
            return
        }
        sidebar?.showPreview(row.usePath)
    }
}

/// The list of the Items pane: Return acts on a row as a double click does.
final class SceneItemsOutlineView: NSOutlineView {
    var onReturn: (() -> Void)?

    override func keyDown(with event: NSEvent) {
        if event.keyCode == 36 || event.keyCode == 76, let onReturn {
            onReturn()
            return
        }
        super.keyDown(with: event)
    }
}

/// A component as the browser draws it: the piece fitted into the box, centered on a transparent
/// background. An item is its part of the element it sits in, an element whole (go/app/component_preview.go).
enum ComponentPreview {
    /// Where the component is drawn in its own element; nil when everything in it draws nothing.
    @MainActor
    static func image(tree: Scene.Tree, node: Scene.Node, size: NSSize = NSSize(width: 160, height: 100)) -> NSImage? {
        // The canvas builds elements, so an item is drawn as its part of the element it sits in.
        var build = node
        if node.kind != .element && node.kind != .file {
            for p in sequence(first: node.parent, next: { $0?.parent }) {
                if let p, p.kind == .element || p.kind == .file {
                    build = p
                    break
                }
            }
        }
        guard let screen = tree.buildElement(build, measurer: Scene.TextMeasurer(), eval: sceneEvalVars),
            let piece = placed(screen.placed(), of: node)
        else { return nil }
        let box = bounds(piece)
        guard box.width > 0, box.height > 0 else { return nil }

        let scale = min(size.width / box.width, size.height / box.height, 1)
        let density = 2.0  // the image keeps its point size and stays sharp on a Retina screen
        let w = Int(size.width * density)
        let h = Int(size.height * density)
        guard w > 0, h > 0,
            let ctx = CGContext(
                data: nil, width: w, height: h, bitsPerComponent: 8, bytesPerRow: 0,
                space: CGColorSpaceCreateDeviceRGB(),
                bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)
        else { return nil }
        ctx.setShouldAntialias(true)
        ctx.scaleBy(x: density, y: density)
        ctx.translateBy(x: 0, y: size.height)
        ctx.scaleBy(x: 1, y: -1)  // the scene counts y down
        ctx.translateBy(
            x: (size.width - box.width * scale) / 2 - box.minX * scale,
            y: (size.height - box.height * scale) / 2 - box.minY * scale)
        ctx.scaleBy(x: scale, y: scale)
        ScenePainter(screen: screen).paint(under(piece, screen: screen), ctx)
        guard let cg = ctx.makeImage() else { return nil }
        return NSImage(cgImage: cg, size: size)
    }

    /// Where n is drawn in the tree of placed nodes; nil when it is not drawn.
    private static func placed(_ root: Scene.Placed, of n: Scene.Node) -> Scene.Placed? {
        if root.src === n { return root }
        for k in root.kids {
            if let got = placed(k, of: n) { return got }
        }
        return nil
    }

    /// The box of pl and its drawn kids, in the coordinates of the screen.
    private static func bounds(_ pl: Scene.Placed) -> CGRect {
        var box = CGRect(x: pl.x, y: pl.y, width: pl.w, height: pl.h)
        for k in pl.kids {
            box = box.union(bounds(k))
        }
        return box
    }

    /// The piece under a stand-in root that draws nothing: ScenePainter skips the picture of the root it
    /// is given, and the piece has a picture of its own.
    private static func under(_ pl: Scene.Placed, screen: Scene.Screen) -> Scene.Placed {
        let r = Scene.RNode(kind: .item)
        screen.p.frames[ObjectIdentifier(r)] = Scene.LayoutNode()
        let root = Scene.Placed(src: nil, inst: false, free: true, parent: nil, x: 0, y: 0, w: pl.w, h: pl.h, r: r)
        root.kids = [pl]
        return root
    }
}
