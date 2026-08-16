//
//  SearchBoxController.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 17/04/2018.
//  Copyright © 2018 Andrey Sobolev. All rights reserved.
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
import Foundation

class SearchBoxViewController: NSViewController, NSTextFieldDelegate, NSTextViewDelegate, NSPopoverDelegate {

    @IBOutlet weak var searchBox: NSTextField!

    var parentView: NSView?

    var element: Element?

    var changes: Int = 0

    var currentItems: [DiagramItem] = []

    var closeAction: (() -> Void)?
    var setActive: ((_ item: DiagramItem) -> Void)?

    var searchResultDelegate: SearchBoxResultDelegate?

    @IBOutlet weak var resultView: NSOutlineView!

    override func loadView() {
        // Build the search box UI programmatically so the controller can be used without a storyboard
        let root = NSView(frame: NSRect(x: 0, y: 0, width: 400, height: 180))
        root.wantsLayer = true
        self.view = root

        let vfx = NSVisualEffectView(frame: .zero)
        vfx.translatesAutoresizingMaskIntoConstraints = false
        vfx.blendingMode = .behindWindow
        vfx.material = .popover
        vfx.state = .followsWindowActiveState
        root.addSubview(vfx)
        NSLayoutConstraint.activate([
            vfx.leadingAnchor.constraint(equalTo: root.leadingAnchor),
            vfx.trailingAnchor.constraint(equalTo: root.trailingAnchor),
            vfx.topAnchor.constraint(equalTo: root.topAnchor),
            vfx.bottomAnchor.constraint(equalTo: root.bottomAnchor),
        ])

        // Search field
        let sb = NSTextField(string: "")
        sb.translatesAutoresizingMaskIntoConstraints = false
        sb.isBordered = true
        sb.isEditable = true
        sb.focusRingType = .none
        sb.placeholderString = "Search..."
        vfx.addSubview(sb)
        sb.leadingAnchor.constraint(equalTo: vfx.leadingAnchor, constant: 8).isActive = true
        sb.trailingAnchor.constraint(equalTo: vfx.trailingAnchor, constant: -8).isActive = true
        sb.topAnchor.constraint(equalTo: vfx.topAnchor, constant: 8).isActive = true
        sb.heightAnchor.constraint(equalToConstant: 30).isActive = true
        self.searchBox = sb

        // Results outline view in scroll view
        let resultsScroll = NSScrollView()
        resultsScroll.translatesAutoresizingMaskIntoConstraints = false
        resultsScroll.hasVerticalScroller = true
        resultsScroll.autohidesScrollers = true
        vfx.addSubview(resultsScroll)
        resultsScroll.leadingAnchor.constraint(equalTo: vfx.leadingAnchor).isActive = true
        resultsScroll.trailingAnchor.constraint(equalTo: vfx.trailingAnchor).isActive = true
        resultsScroll.topAnchor.constraint(equalTo: sb.bottomAnchor, constant: 8).isActive = true
        resultsScroll.bottomAnchor.constraint(equalTo: vfx.bottomAnchor, constant: -8).isActive = true

        let outline = NSOutlineView(frame: .zero)
        outline.translatesAutoresizingMaskIntoConstraints = false
        let col = NSTableColumn(identifier: NSUserInterfaceItemIdentifier(rawValue: "fdS-FP-WJp"))
        outline.addTableColumn(col)
        outline.outlineTableColumn = col
        outline.headerView = nil
        resultsScroll.documentView = outline
        self.resultView = outline

        // Default preferred size for popover presentation
        self.preferredContentSize = NSSize(width: 400, height: 180)
    }

    override func viewDidLoad() {
        self.searchBox.delegate = self
        let delegate = SearchBoxResultDelegate(self)
        self.searchResultDelegate = delegate
        resultView.delegate = delegate
        resultView.dataSource = delegate
    }

    func setElement(_ element: Element) {
        self.element = element
    }

    //    func windowDidResignMain(_ notification: Notification) {
    //        close()
    //    }

    override func viewWillAppear() {
        self.view.window?.hidesOnDeactivate = true

        //        self.searc
        self.searchBox.becomeFirstResponder()

        self.view.window?.center()
    }

    func control(_ control: NSControl, textView: NSTextView, doCommandBy commandSelector: Selector) -> Bool {
        if commandSelector == #selector(NSView.cancelOperation(_:)) {
            close()
            return true
        }
        if commandSelector == #selector(NSView.insertNewline(_:)) {
            self.close()
            return true
        }

        //        if commandSelector == #selector(NSView.keyDown(with:)) {
        //            return true
        //        }

        if commandSelector == #selector(NSView.moveUp(_:)) {
            let row = self.resultView.selectedRow
            if row > 0 {
                self.resultView.selectRowIndexes(NSIndexSet(index: row - 1) as IndexSet, byExtendingSelection: false)
                self.resultView.scrollRowToVisible(row - 1)
            } else {
                self.resultView.selectRowIndexes(NSIndexSet(index: self.currentItems.count - 1) as IndexSet, byExtendingSelection: false)
                self.resultView.scrollRowToVisible(self.currentItems.count - 1)
            }

            return true
        }
        if commandSelector == #selector(NSView.moveDown(_:)) {
            let row = self.resultView.selectedRow
            if row + 1 < self.currentItems.count {
                self.resultView.selectRowIndexes(NSIndexSet(index: row + 1) as IndexSet, byExtendingSelection: false)
                self.resultView.scrollRowToVisible(row + 1)
            } else {
                self.resultView.selectRowIndexes(NSIndexSet(index: 0) as IndexSet, byExtendingSelection: false)
                self.resultView.scrollRowToVisible(0)
            }
            return true
        }

        // TODO: Resize both text and drawed item to fit value smoothly.

        return false
    }

    override func selectAll(_ sender: Any?) {
        self.searchBox.selectAll(sender)
    }

    func controlTextDidChange(_ notification: Notification) {
        changes += 1
        sheduleUpdate()
    }

    fileprivate func sheduleUpdate() {
        let curChanges = self.changes
        DispatchQueue.main.asyncAfter(
            deadline: .now() + 0.3,
            execute: {
                if curChanges == self.changes {
                    let textContent = self.searchBox.stringValue

                    if let el = self.element {
                        self.currentItems = el.items.filter({ (item) in

                            let nameMatched = item.name.lowercased().contains(textContent.lowercased())

                            var textValue = ""
                            SceneDrawView.getBodyText(item, nil, &textValue)

                            let bodyMatched = textValue.lowercased().contains(textContent.lowercased())

                            return nameMatched || bodyMatched
                        }).sorted(by: { (a, b) in a.name.lexicographicallyPrecedes(b.name) })

                        self.resultView.reloadData()
                        if self.currentItems.count > 0 {
                            self.resultView.selectRowIndexes(NSIndexSet(index: 0) as IndexSet, byExtendingSelection: false)
                        }
                    }

                    self.view.needsDisplay = true
                }
            })
    }

    override func keyDown(with event: NSEvent) {
    }

    @IBAction func applyClose(_ sender: NSButton) {
        close()
    }
    func close() {
        if let cl = self.closeAction {
            cl()
        }
    }
}

class SearchBoxResultDelegate: NSObject, NSOutlineViewDataSource, NSOutlineViewDelegate {
    let controller: SearchBoxViewController
    init(_ controller: SearchBoxViewController) {
        self.controller = controller
    }

    func outlineView(_ outlineView: NSOutlineView, numberOfChildrenOfItem item: Any?) -> Int {
        if item == nil {
            return self.controller.currentItems.count
        }
        return 0
    }
    func outlineView(_ outlineView: NSOutlineView, child index: Int, ofItem item: Any?) -> Any {
        if item == nil {
            return controller.currentItems[index]
        }
        return ""
    }

    func outlineView(_ outlineView: NSOutlineView, isItemExpandable item: Any) -> Bool {
        return false
    }

    func outlineView(_ outlineView: NSOutlineView, objectValueFor tableColumn: NSTableColumn?, byItem item: Any?) -> Any? {
        //1
        if let el = item as? DiagramItem {
            return el.name
        }
        return nil
    }

    func outlineView(_ outlineView: NSOutlineView, viewFor viewForTableColumn: NSTableColumn?, item: Any) -> NSView? {
        if let el = item as? DiagramItem {
            if let view = outlineView.makeView(withIdentifier: NSUserInterfaceItemIdentifier(rawValue: "SearchItemCell"), owner: self)
                as? NSTableCellView
            {
                if let textField = view.textField {
                    var textValue = ""
                    SceneDrawView.getBodyText(el, nil, &textValue)

                    if textValue.count > 0 {
                        textField.stringValue = (el.name + " - " + textValue).replacingOccurrences(of: "\n", with: "\\n")
                    } else {
                        textField.stringValue = el.name.replacingOccurrences(of: "\n", with: "\\n")
                    }
                }
                return view
            } else {
                // Fallback: create a simple NSTableCellView so the outline works without a storyboard prototype
                let cell = NSTableCellView(frame: NSRect(x: 0, y: 0, width: 1000, height: 18))
                cell.identifier = NSUserInterfaceItemIdentifier(rawValue: "SearchItemCell")

                let textField = NSTextField(frame: NSRect(x: 0, y: 0, width: 1000, height: 18))
                textField.isBordered = false
                textField.isEditable = false
                textField.backgroundColor = NSColor.clear
                textField.translatesAutoresizingMaskIntoConstraints = false
                cell.addSubview(textField)
                cell.textField = textField

                NSLayoutConstraint.activate([
                    textField.leadingAnchor.constraint(equalTo: cell.leadingAnchor, constant: 2),
                    textField.trailingAnchor.constraint(equalTo: cell.trailingAnchor, constant: -2),
                    textField.centerYAnchor.constraint(equalTo: cell.centerYAnchor),
                ])

                var textValue = ""
                SceneDrawView.getBodyText(el, nil, &textValue)
                if textValue.count > 0 {
                    textField.stringValue = (el.name + " - " + textValue).replacingOccurrences(of: "\n", with: "\\n")
                } else {
                    textField.stringValue = el.name.replacingOccurrences(of: "\n", with: "\\n")
                }

                return cell
            }
        }
        return nil
    }

    //    func outlineView(_ outlineView: NSOutlineView, rowViewForItem item: Any) -> NSTableRowView? {
    //        return OutlineNSTableRowView()
    //    }

    @objc func outlineViewSelectionDidChange(_ notification: Notification) {

        let selectedIndex = controller.resultView.selectedRow
        if let el = controller.resultView.item(atRow: selectedIndex) as? DiagramItem {
            self.controller.setActive?(el)
        }
    }
}
