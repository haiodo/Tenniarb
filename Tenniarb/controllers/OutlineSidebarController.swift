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

    private var outlineViewDelegate: OutlineViewControllerDelegate?

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

        // Outline scroll view
        let outlineScroll = NSScrollView()
        outlineScroll.translatesAutoresizingMaskIntoConstraints = false
        outlineScroll.hasVerticalScroller = true
        outlineScroll.hasHorizontalScroller = false
        outlineScroll.autohidesScrollers = true
        outlineScroll.borderType = .noBorder
        outlineScroll.drawsBackground = false
        outlineScroll.contentView.drawsBackground = false
        stackView.addArrangedSubview(outlineScroll)

        // Outline view
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

        return stackView
    }

    func setupDelegate() {
        guard let vc = viewController else { return }

        self.outlineViewDelegate = OutlineViewControllerDelegate(vc)
        outlineView.delegate = self.outlineViewDelegate
        outlineView.dataSource = self.outlineViewDelegate
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
