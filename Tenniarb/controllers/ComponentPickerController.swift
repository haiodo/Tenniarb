//
//  ComponentPickerController.swift
//  Tenniarb
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
//

import Cocoa

/// The sheet behind Insert Component…: the components of the libraries and of the document, filtered
/// by a search field and grouped by branch (go/app/outline_scene.go showComponentPicker).
@MainActor
final class ComponentPickerController: NSWindowController, NSTableViewDataSource, NSTableViewDelegate,
    NSSearchFieldDelegate
{
    /// One row of the list: a group heading when component is nil, else a component to insert.
    struct Row {
        let label: String
        let component: Scene.Component?
    }

    /// The rows the list shows now, headings included; read by tests.
    private(set) var rows: [Row] = []

    /// Search text of the field; setting it refills the list.
    var query: String {
        get { searchField.stringValue }
        set {
            searchField.stringValue = newValue
            refill()
        }
    }

    /// The component under the selected row, nil on a heading or an empty list.
    var selectedComponent: Scene.Component? {
        rows.indices.contains(tableView.selectedRow) ? rows[tableView.selectedRow].component : nil
    }

    private let provider: @MainActor (String) -> [Scene.Component]
    private let onPick: @MainActor (Scene.Component) -> Void
    private let searchField = NSSearchField()
    private let tableView = NSTableView()
    private weak var parent: NSWindow?

    /// provider is called with the current query each time the list refills.
    init(
        provider: @escaping @MainActor (String) -> [Scene.Component],
        onPick: @escaping @MainActor (Scene.Component) -> Void
    ) {
        self.provider = provider
        self.onPick = onPick
        super.init(window: nil)

        let window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 420, height: 420),
            styleMask: [.titled, .resizable],
            backing: .buffered,
            defer: false
        )
        window.title = "Insert Component"
        window.minSize = NSSize(width: 320, height: 240)
        window.isReleasedWhenClosed = false
        window.contentView = contentView()
        window.initialFirstResponder = searchField
        self.window = window
        refill()
    }

    required init?(coder: NSCoder) {
        fatalError("init(coder:) is not supported")
    }

    /// Shows the sheet over parent.
    func present(asSheetFor parent: NSWindow) {
        guard let window else { return }
        self.parent = parent
        parent.beginSheet(window)
        window.makeFirstResponder(searchField)
    }

    private func contentView() -> NSView {
        searchField.translatesAutoresizingMaskIntoConstraints = false
        searchField.placeholderString = "Component"
        searchField.delegate = self

        let column = NSTableColumn(identifier: NSUserInterfaceItemIdentifier(rawValue: "ComponentColumn"))
        column.resizingMask = .autoresizingMask
        tableView.addTableColumn(column)
        tableView.columnAutoresizingStyle = .firstColumnOnlyAutoresizingStyle
        tableView.headerView = nil
        tableView.rowHeight = 20
        tableView.intercellSpacing = .zero
        tableView.dataSource = self
        tableView.delegate = self
        tableView.target = self
        tableView.doubleAction = #selector(insert(_:))
        tableView.frame = NSRect(x: 0, y: 0, width: 388, height: 400)
        tableView.autoresizingMask = [.width]

        let scroll = NSScrollView()
        scroll.translatesAutoresizingMaskIntoConstraints = false
        scroll.hasVerticalScroller = true
        scroll.hasHorizontalScroller = false
        scroll.autohidesScrollers = true
        scroll.borderType = .bezelBorder
        scroll.documentView = tableView

        let insertButton = NSButton(title: "Insert", target: self, action: #selector(insert(_:)))
        insertButton.keyEquivalent = "\r"
        let cancelButton = NSButton(title: "Cancel", target: self, action: #selector(cancel(_:)))
        cancelButton.keyEquivalent = "\u{1b}"

        let buttons = NSStackView(views: [cancelButton, insertButton])
        buttons.translatesAutoresizingMaskIntoConstraints = false
        buttons.orientation = .horizontal
        buttons.spacing = 8

        let view = NSView(frame: NSRect(x: 0, y: 0, width: 420, height: 420))
        view.addSubview(searchField)
        view.addSubview(scroll)
        view.addSubview(buttons)
        NSLayoutConstraint.activate([
            searchField.topAnchor.constraint(equalTo: view.topAnchor, constant: 16),
            searchField.leadingAnchor.constraint(equalTo: view.leadingAnchor, constant: 16),
            searchField.trailingAnchor.constraint(equalTo: view.trailingAnchor, constant: -16),
            scroll.topAnchor.constraint(equalTo: searchField.bottomAnchor, constant: 12),
            scroll.leadingAnchor.constraint(equalTo: view.leadingAnchor, constant: 16),
            scroll.trailingAnchor.constraint(equalTo: view.trailingAnchor, constant: -16),
            buttons.topAnchor.constraint(equalTo: scroll.bottomAnchor, constant: 12),
            buttons.leadingAnchor.constraint(greaterThanOrEqualTo: view.leadingAnchor, constant: 16),
            buttons.trailingAnchor.constraint(equalTo: view.trailingAnchor, constant: -16),
            buttons.bottomAnchor.constraint(equalTo: view.bottomAnchor, constant: -16),
        ])
        return view
    }

    private func refill() {
        var built: [Row] = []
        var previousGroup = ""
        for component in provider(searchField.stringValue) {
            if !component.group.isEmpty && component.group != previousGroup {
                built.append(Row(label: component.group, component: nil))
            }
            built.append(Row(label: component.name, component: component))
            previousGroup = component.group
        }
        rows = built
        tableView.reloadData()
        guard let first = rows.firstIndex(where: { $0.component != nil }) else {
            tableView.deselectAll(nil)
            return
        }
        tableView.selectRowIndexes(IndexSet(integer: first), byExtendingSelection: false)
        tableView.scrollRowToVisible(first)
        tableView.scroll(NSPoint(x: 0, y: tableView.rect(ofRow: first).minY))
    }

    @objc private func insert(_ sender: Any?) {
        guard let component = selectedComponent else { return }
        finish(component)
    }

    @objc private func cancel(_ sender: Any?) {
        finish(nil)
    }

    private func finish(_ component: Scene.Component?) {
        guard let window else { return }
        parent?.endSheet(window)
        window.close()
        if let component { onPick(component) }
    }

    func numberOfRows(in tableView: NSTableView) -> Int {
        rows.count
    }

    func tableView(_ tableView: NSTableView, viewFor tableColumn: NSTableColumn?, row: Int) -> NSView? {
        let identifier = NSUserInterfaceItemIdentifier(rawValue: "ComponentCell")
        let cell: NSTableCellView
        if let reused = tableView.makeView(withIdentifier: identifier, owner: self) as? NSTableCellView {
            cell = reused
        } else {
            cell = NSTableCellView(frame: .zero)
            cell.identifier = identifier
            let text = NSTextField(labelWithString: "")
            text.translatesAutoresizingMaskIntoConstraints = false
            text.lineBreakMode = .byTruncatingTail
            cell.addSubview(text)
            cell.textField = text
            NSLayoutConstraint.activate([
                text.leadingAnchor.constraint(equalTo: cell.leadingAnchor, constant: 4),
                text.trailingAnchor.constraint(equalTo: cell.trailingAnchor, constant: -4),
                text.centerYAnchor.constraint(equalTo: cell.centerYAnchor),
            ])
        }
        let item = rows[row]
        let heading = item.component == nil
        cell.textField?.stringValue = item.label
        cell.textField?.font = heading ? NSFont.systemFont(ofSize: 11, weight: .semibold) : NSFont.systemFont(ofSize: 13)
        cell.textField?.textColor = heading ? .secondaryLabelColor : .labelColor
        return cell
    }

    func tableView(_ tableView: NSTableView, shouldSelectRow row: Int) -> Bool {
        rows.indices.contains(row) && rows[row].component != nil
    }

    func controlTextDidChange(_ obj: Notification) {
        query = searchField.stringValue
    }

    func control(_ control: NSControl, textView: NSTextView, doCommandBy commandSelector: Selector) -> Bool {
        switch commandSelector {
        case #selector(NSView.insertNewline(_:)):
            insert(nil)
            return true
        case #selector(NSView.cancelOperation(_:)):
            cancel(nil)
            return true
        default:
            return false
        }
    }
}
