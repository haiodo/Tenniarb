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

    private var outlineViewDelegate: OutlineViewControllerDelegate?
    private var itemsDelegate: SceneItemsDelegate?

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
        let itemsLabel = NSTextField(labelWithString: "Items")
        itemsLabel.translatesAutoresizingMaskIntoConstraints = false
        itemsLabel.font = NSFont.systemFont(ofSize: 11, weight: .semibold)
        itemsLabel.textColor = .secondaryLabelColor
        itemsHeader.contentView!.addSubview(itemsLabel)
        itemsLabel.leadingAnchor.constraint(equalTo: itemsHeader.contentView!.leadingAnchor, constant: 8).isActive = true
        itemsLabel.centerYAnchor.constraint(equalTo: itemsHeader.contentView!.centerYAnchor).isActive = true

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

        let items = NSOutlineView(frame: .zero)
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

        self.itemsDelegate = SceneItemsDelegate(vc)
        itemsOutlineView.delegate = self.itemsDelegate
        itemsOutlineView.dataSource = self.itemsDelegate
    }

    /// Lists the items of the scene on the canvas; nil clears the pane.
    func showItems(_ canvas: SceneCanvasView?) {
        itemsDelegate?.show(canvas)
        itemsOutlineView?.reloadData()
        itemsOutlineView?.expandItem(nil, expandChildren: false)
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
/// of an instance under it. A row selects the item on the canvas.
@MainActor
final class SceneItemsDelegate: NSObject, NSOutlineViewDataSource, NSOutlineViewDelegate {
    /// One row: a drawn item, its index path from the root of the screen, and the rows under it.
    final class Node {
        let label: String
        let path: [Int]
        var kids: [Node] = []

        init(label: String, path: [Int]) {
            self.label = label
            self.path = path
        }
    }

    private var roots: [Node] = []
    private weak var viewController: ViewController?

    init(_ viewController: ViewController) {
        self.viewController = viewController
        super.init()
    }

    /// Lists the items of the scene on the canvas; nil clears the pane.
    func show(_ canvas: SceneCanvasView?) {
        roots = Self.nodes(canvas, nil)
    }

    private static func nodes(_ canvas: SceneCanvasView?, _ path: [Int]?) -> [Node] {
        (canvas?.itemRows(path) ?? []).map { row in
            let node = Node(label: row.label, path: row.path)
            if row.kids { node.kids = nodes(canvas, row.path) }
            return node
        }
    }

    func outlineView(_ outlineView: NSOutlineView, numberOfChildrenOfItem item: Any?) -> Int {
        (item as? Node)?.kids.count ?? roots.count
    }

    func outlineView(_ outlineView: NSOutlineView, child index: Int, ofItem item: Any?) -> Any {
        (item as? Node)?.kids[index] ?? roots[index]
    }

    func outlineView(_ outlineView: NSOutlineView, isItemExpandable item: Any) -> Bool {
        !((item as? Node)?.kids.isEmpty ?? true)
    }

    func outlineView(_ outlineView: NSOutlineView, objectValueFor tableColumn: NSTableColumn?, byItem item: Any?) -> Any? {
        (item as? Node)?.label
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
        cell?.textField?.stringValue = (item as? Node)?.label ?? ""
        return cell
    }

    func outlineView(_ outlineView: NSOutlineView, rowViewForItem item: Any) -> NSTableRowView? {
        OutlineNSTableRowView()
    }

    func outlineViewSelectionDidChange(_ notification: Notification) {
        guard let outline = notification.object as? NSOutlineView, outline.selectedRow >= 0,
            let node = outline.item(atRow: outline.selectedRow) as? Node
        else { return }
        viewController?.selectSceneItem(node.path)
    }
}
