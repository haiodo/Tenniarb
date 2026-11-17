//
//  InspectorController.swift
//  Tenniarb
//
//  Licensed under the Eclipse Public License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License. You may
//  obtain a copy of the License at https://www.eclipse.org/legal/epl-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.
//

import Cocoa

/// The right column of a scene document: the properties of the selected node, as go/app/inspector_scene.go
/// builds them. A field holds the node's own command; the value in effect from the component, a style or a
/// default is the hint.
@MainActor
final class InspectorController: NSObject, NSTextFieldDelegate {
    /// The width Go gives the inspector; wide enough for a pair of fields without cutting them.
    static let width: CGFloat = 272
    /// How far the divider may move: the column is fixed in spirit, not in the way of a wider path.
    static let minWidth: CGFloat = 240
    static let maxWidth: CGFloat = 320

    /// One row as the tests and the column see it: the caption, the value the node writes, the hint of an
    /// unset field, the kind of control, and what a popup offers.
    struct InspectorRow {
        enum Kind: String {
            case text
            case popup
            case toggle
            case info
        }

        let section: String
        let label: String
        var text: String
        var placeholder = ""
        let kind: Kind
        var choices: [String] = []
    }

    private(set) var rows: [InspectorRow] = []
    /// The header the section title shows: Item, Frame, Instance, Slot, or Screen with nothing selected.
    private(set) var title = ""

    private(set) var containerView: NSView!

    private weak var canvas: SceneCanvasView?
    private weak var document: Document?

    private var stack: NSStackView?
    private var rowControls: [RowControl] = []
    /// The selection path the controls were built for; a different one rebuilds them.
    private var shownSelection: [Int]?

    // MARK: - Wiring

    /// Points the column at a canvas and the document its edits go through.
    func attach(canvas: SceneCanvasView?, document: Document?) {
        self.canvas = canvas
        self.document = document
        canvas?.onSelectionChange = { [weak self] in self?.refresh() }
        refresh()
    }

    /// Builds the column: a scroll view around the sections, no horizontal bar, so a row that cannot wrap is
    /// cut at the fixed width rather than widening the pane.
    func createView() -> NSView {
        let root = NSView()
        root.translatesAutoresizingMaskIntoConstraints = false

        let scroll = NSScrollView()
        scroll.translatesAutoresizingMaskIntoConstraints = false
        scroll.hasVerticalScroller = true
        scroll.hasHorizontalScroller = false
        scroll.autohidesScrollers = true
        scroll.borderType = .noBorder
        scroll.drawsBackground = false
        scroll.contentView.drawsBackground = false
        root.addSubview(scroll)
        NSLayoutConstraint.activate([
            scroll.leadingAnchor.constraint(equalTo: root.leadingAnchor),
            scroll.trailingAnchor.constraint(equalTo: root.trailingAnchor),
            scroll.topAnchor.constraint(equalTo: root.topAnchor),
            scroll.bottomAnchor.constraint(equalTo: root.bottomAnchor),
        ])

        let content = NSView()
        content.translatesAutoresizingMaskIntoConstraints = false
        let stack = NSStackView()
        stack.translatesAutoresizingMaskIntoConstraints = false
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 10
        stack.edgeInsets = NSEdgeInsets(top: 10, left: 8, bottom: 10, right: 8)
        content.addSubview(stack)
        scroll.documentView = content
        NSLayoutConstraint.activate([
            stack.leadingAnchor.constraint(equalTo: content.leadingAnchor),
            stack.trailingAnchor.constraint(equalTo: content.trailingAnchor),
            stack.topAnchor.constraint(equalTo: content.topAnchor),
            stack.bottomAnchor.constraint(equalTo: content.bottomAnchor),
            content.widthAnchor.constraint(equalTo: scroll.contentView.widthAnchor),
        ])
        self.containerView = root
        self.stack = stack
        reloadUI()
        return root
    }

    // MARK: - Reading the selection

    /// The drawn node the rows describe: the selected item or instance, or the part of an instance the use
    /// around it edits through an at override.
    private struct Selection {
        let pl: Scene.Placed
        let node: Scene.Node?
        let part: SceneCanvasView.Part?
    }

    private func selection() -> Selection? {
        guard let canvas, let path = canvas.selection, let pl = canvas.placed(at: path) else { return nil }
        return Selection(pl: pl, node: pl.src, part: canvas.part(at: path))
    }

    /// Where an edit of the selection is written: the whole text of the file the command is in.
    private struct Context {
        let file: String
        let text: String
    }

    private func context(_ sel: Selection) -> Context? {
        guard let document, let tree = document.sceneTree else { return nil }
        let file: String
        if let part = sel.part {
            file = part.use.pos.file
        } else if let node = sel.node {
            file = node.pos.file
        } else {
            return nil
        }
        guard let text = tree.source(file) else { return nil }
        return Context(file: file, text: text)
    }

    /// The command the selected node writes itself; a part holds its own in the at override of the use.
    private func ownField(_ sel: Selection, _ name: String) -> Scene.Prop? {
        if let part = sel.part {
            guard !part.at.isEmpty, let at = atOf(part.use, part.at) else { return nil }
            return atProp(at, name)
        }
        guard let node = sel.node else { return nil }
        return ownProp(node, name)
    }

    private static func texts(_ p: Scene.Prop) -> [String] {
        (0..<p.args.count).map { p.text($0) ?? "" }
    }

    private func args(_ sel: Selection, _ name: String) -> [String] {
        if let own = ownField(sel, name) {
            return InspectorController.texts(own)
        }
        guard let eff = sel.pl.prop(name) else { return [] }
        return InspectorController.texts(eff)
    }

    // MARK: - Rows

    /// One editable field of the column: the command it writes, which argument, and what a popup offers.
    private struct Field {
        let section: String
        let label: String
        let kind: InspectorRow.Kind
        var property = ""
        var index = 0
        var choices: [String] = []
        var unset = ""
        var numeric = false
        var list = false
    }

    private static let fields: [Field] = [
        Field(
            section: "Layout", label: "Auto layout", kind: .popup, property: "auto-layout",
            choices: ["none", "row", "column", "abs"], unset: "none"),
        Field(section: "Layout", label: "Gap", kind: .text, property: "gap", numeric: true),
        Field(
            section: "Layout", label: "Align", kind: .popup, property: "align",
            choices: ["none", "start", "center", "end"], unset: "none"),
        Field(
            section: "Layout", label: "Justify", kind: .popup, property: "justify",
            choices: ["none", "start", "center", "end", "space-between"], unset: "none"),
        Field(section: "Layout", label: "Clip", kind: .toggle, property: "clip"),
        Field(section: "Layout", label: "Position X", kind: .text, property: "pos", index: 0, numeric: true),
        Field(section: "Layout", label: "Position Y", kind: .text, property: "pos", index: 1, numeric: true),
        Field(section: "Layout", label: "Size W", kind: .text, property: "width"),
        Field(section: "Layout", label: "Size H", kind: .text, property: "height"),
        Field(section: "Layout", label: "Padding", kind: .text, property: "padding", list: true),

        Field(section: "Fill", label: "Color", kind: .text, property: "color"),

        Field(section: "Text", label: "Title", kind: .text, property: "title"),
        Field(section: "Text", label: "Color", kind: .text, property: "text-color"),
        Field(section: "Text", label: "Size", kind: .text, property: "font-size", numeric: true),
        Field(section: "Text", label: "Font", kind: .text, property: "font-family"),
        Field(section: "Text", label: "Weight", kind: .text, property: "font-weight"),

        Field(section: "Border", label: "Color", kind: .text, property: "border-color"),
        Field(section: "Border", label: "Width", kind: .text, property: "line-width", numeric: true),
        Field(
            section: "Border", label: "Style", kind: .popup, property: "line-style",
            choices: ["none", "solid", "dashed", "dotted"], unset: "none"),
        Field(section: "Border", label: "Radius", kind: .text, property: "corner-radius"),

        Field(
            section: "Shape", label: "Display", kind: .popup, property: "display",
            choices: ["rect", "no-fill", "circle", "stack", "text"], unset: "rect"),
        Field(
            section: "Shape", label: "Layer", kind: .popup, property: "layer",
            choices: ["default", "background", "hover"], unset: "default"),

        Field(section: "Effects", label: "Opacity", kind: .text, property: "opacity", numeric: true),
        Field(section: "Effects", label: "Hidden", kind: .toggle, property: "hidden"),
    ]

    private func field(for row: InspectorRow) -> Field? {
        InspectorController.fields.first { $0.section == row.section && $0.label == row.label }
    }

    /// One row as the current tree writes it.
    private func fieldRow(_ f: Field, _ sel: Selection) -> InspectorRow {
        let own = ownField(sel, f.property)
        let eff = sel.pl.prop(f.property)
        var text = ""
        var placeholder = ""
        switch f.kind {
        case .text:
            if f.list {
                text = own.map(InspectorController.texts)?.joined(separator: " ") ?? ""
            } else if let own, f.index < own.args.count {
                text = own.text(f.index) ?? ""
            }
            if own == nil, f.index == 0, let eff {
                placeholder = InspectorController.texts(eff).joined(separator: " ")
            }
        case .popup:
            if let own {
                text = own.text(0) ?? ""
            } else if let eff {
                let value = InspectorController.texts(eff).joined(separator: " ")
                text = value.isEmpty ? f.unset : value
            } else {
                text = f.unset
            }
        case .toggle:
            text = own == nil ? "off" : "yes"
        case .info:
            break
        }
        return InspectorRow(
            section: f.section, label: f.label, text: text, placeholder: placeholder, kind: f.kind,
            choices: f.choices)
    }

    /// Item, Frame, Instance, Slot: what the header calls the shown node, as go/app sceneSections.
    private func kindText(_ n: Scene.Node?) -> String {
        guard let n else { return "Screen" }
        switch n.kind {
        case .use:
            return "Instance"
        case .slot:
            return "Slot"
        case .item:
            return sceneKids(n).isEmpty ? "Item" : "Frame"
        default:
            return "Item"
        }
    }

    /// What go/editor SceneKids lists under a node: items and uses, and the slots of an item.
    private func sceneKids(_ n: Scene.Node) -> [Scene.Node] {
        n.content.filter { c in
            c.kind == .item || c.kind == .use || (n.kind == .item && c.kind == .slot)
        }
    }

    private func sizeText(_ w: Double, _ h: Double) -> String {
        func f(_ v: Double) -> String {
            if v.rounded() == v, abs(v) < 1e15 { return String(Int(v)) }
            return String(v)
        }
        return "\(f(w)) x \(f(h))"
    }

    private func build() -> (title: String, rows: [InspectorRow]) {
        guard let canvas, canvas.screen != nil else { return ("", []) }
        var head = "Screen"
        var name = canvas.screenNode?.name ?? ""
        var size = ""
        var component: String?
        let sel = selection()
        if let sel, let node = sel.node {
            head = kindText(node)
            name = node.name
            size = sizeText(sel.pl.w, sel.pl.h)
            if sel.part == nil, node.kind == .use {
                component = node.path
            }
        } else if let root = canvas.placed {
            size = sizeText(root.w, root.h)
        }
        var rows: [InspectorRow] = [
            InspectorRow(section: "Header", label: "Name", text: name, kind: .info)
        ]
        if let component {
            rows.append(InspectorRow(section: "Header", label: "Component", text: component, kind: .info))
        }
        rows.append(InspectorRow(section: "Header", label: "Size", text: size, kind: .info))
        guard let sel, sel.node != nil else { return (head, rows) }
        rows += InspectorController.fields.map { fieldRow($0, sel) }
        return (head, rows)
    }

    /// Rebuilds the values from the tree behind the canvas, and the controls when the rows themselves
    /// changed; a value the user is typing into is not overwritten.
    func refresh() {
        let built = build()
        let path = canvas?.selection
        let selectionChanged = path != shownSelection
        shownSelection = path
        let keysChanged = built.rows.map(InspectorController.rowKey) != rows.map(InspectorController.rowKey)
        let titleChanged = built.title != title
        title = built.title
        rows = built.rows
        if selectionChanged || keysChanged || titleChanged {
            reloadUI()
        }
        for (i, row) in rows.enumerated() where rowControls.indices.contains(i) {
            rowControls[i].apply(row)
        }
    }

    private static func rowKey(_ r: InspectorRow) -> String {
        "\(r.section)|\(r.label)|\(r.kind.rawValue)|\(r.choices.joined(separator: ","))"
    }

    /// The row with this caption: "Text.Color" and "Text Color" pick the Color of the Text section, a plain
    /// caption the first row that holds it.
    func row(_ label: String) -> InspectorRow? {
        let key = label.lowercased()
        if let exact = rows.first(where: { $0.label.lowercased() == key }) {
            return exact
        }
        let parts = key.replacingOccurrences(of: ".", with: " ").split(separator: " ", maxSplits: 1)
        guard parts.count == 2 else { return nil }
        let section = String(parts[0])
        let caption = String(parts[1])
        return rows.first { $0.section.lowercased() == section && $0.label.lowercased() == caption }
    }

    // MARK: - Writing

    /// Sets a text row, as typing into it and pressing Return does.
    func setText(_ value: String, for label: String) {
        guard let row = row(label) else { return }
        commitText(row, value)
    }

    /// Picks a choice of a popup row; the choice that stands for unset removes the command.
    func setChoice(_ choice: String, for label: String) {
        guard let row = row(label) else { return }
        applyChoice(row, choice)
    }

    /// Flips a toggle row: on writes the command with its words, off removes it.
    func toggle(_ label: String) {
        guard let row = row(label), row.kind == .toggle else { return }
        applyToggle(row, on: row.text != "yes")
    }

    private func commitText(_ row: InspectorRow, _ raw: String) {
        guard row.kind == .text, let f = field(for: row) else { return }
        let value = raw.trimmingCharacters(in: .whitespacesAndNewlines)
        guard value != row.text else {
            refresh()
            return
        }
        guard !f.numeric || value.isEmpty || isNumberOrExpr(value) else {
            refresh()  // a number field takes a number or the source of an expression, as go/app
            return
        }
        guard let sel = selection(), context(sel) != nil else {
            refresh()
            return
        }
        if f.list {
            let words = value.split(whereSeparator: { $0 == " " || $0 == "\t" }).map(String.init)
            write(f.property, words)
        } else if value.isEmpty && f.index == 0 {
            write(f.property, [])  // an empty field removes the command
        } else {
            var values = args(sel, f.property)
            while values.count <= f.index {
                values.append("0")
            }
            values[f.index] = value
            write(f.property, values)
        }
        refresh()
    }

    private func applyChoice(_ row: InspectorRow, _ choice: String) {
        guard row.kind == .popup, let f = field(for: row), f.choices.contains(choice) else { return }
        if choice == f.unset {
            write(f.property, [])
        } else {
            write(f.property, [choice])
        }
        refresh()
    }

    private func applyToggle(_ row: InspectorRow, on: Bool) {
        guard row.kind == .toggle, let f = field(for: row) else { return }
        write(f.property, on ? ["yes"] : [])
        refresh()
    }

    @discardableResult
    private func write(_ name: String, _ args: [String]) -> Bool {
        guard let sel = selection(), let ctx = context(sel) else {
            refresh()
            return false
        }
        if let part = sel.part {
            guard !part.at.isEmpty else {
                canvas?.setMessage(
                    "Not applied: the item has no unique name inside the instance, edit it in the use")
                return false
            }
            guard let edit = atEdit(ctx.text, part.use, part.at, name, args) else { return false }
            return apply(edit, in: ctx.file)
        }
        guard let node = sel.node, let edit = propEdit(ctx.text, node, name, args) else { return false }
        return apply(edit, in: ctx.file)
    }

    /// setPropEdit for the array a field holds: the printer takes the values as one command, as the variadic
    /// producer does with its arguments.
    private func propEdit(_ text: String, _ node: Scene.Node, _ name: String, _ args: [String]) -> Scene.TextEdit? {
        guard !args.isEmpty else { return setPropEdit(text, node, name) }
        let line = commandText(name, args)
        if let own = ownProp(node, name) {
            return Scene.TextEdit(start: own.range.start, end: own.range.end, new: line)
        }
        return insertEdit(text, node, line)
    }

    /// atPropEdit for the array a field holds: the override goes into the at of the part, or a new at is
    /// written in the block of the use.
    private func atEdit(
        _ text: String, _ use: Scene.Node, _ path: String, _ name: String, _ args: [String]
    ) -> Scene.TextEdit? {
        guard !args.isEmpty else { return atPropEdit(text, use, path, name) }
        let line = commandText(name, args)
        let at = atOf(use, path)
        if let own = at.flatMap({ atProp($0, name) }) {
            return Scene.TextEdit(start: own.range.start, end: own.range.end, new: line)
        }
        if let at {
            let holder = Scene.Node(kind: .item, range: at.range, body: at.body)
            return insertEdit(text, holder, line)
        }
        return insertEdit(text, use, commandText("at", path) + " { " + line + " }")
    }

    /// Applies one text edit through the document; a refusal stays over the canvas and the old text stays.
    private func apply(_ edit: Scene.TextEdit, in file: String) -> Bool {
        guard let document else { return false }
        do {
            try document.applySceneEdit(edit, in: file)
            canvas?.setMessage(nil)
            if let next = document.sceneTree, let canvas, next !== canvas.shownTree {
                canvas.show(next)
            }
            return true
        } catch {
            canvas?.setMessage(error.localizedDescription)
            refresh()
            return false
        }
    }

    private func isNumberOrExpr(_ s: String) -> Bool {
        if Double(s) != nil { return true }
        return s.hasPrefix("${") || s.hasPrefix("$(")
    }

    // MARK: - The column

    private func reloadUI() {
        guard let stack else { return }
        for control in rowControls {
            control.detach()  // a field of the old rows must not write into the new selection
        }
        for view in stack.arrangedSubviews {
            stack.removeArrangedSubview(view)
            view.removeFromSuperview()
        }
        rowControls = []
        var section = ""
        var body: NSStackView?
        for (i, row) in rows.enumerated() {
            if row.section != section {
                section = row.section
                let header = NSTextField(labelWithString: row.section == "Header" ? title : row.section)
                header.font = NSFont.systemFont(ofSize: 11, weight: .semibold)
                header.textColor = .secondaryLabelColor
                header.lineBreakMode = .byTruncatingTail
                stack.addArrangedSubview(header)
                header.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -16).isActive = true
                let next = NSStackView()
                next.orientation = .vertical
                next.alignment = .leading
                next.spacing = 6
                stack.addArrangedSubview(next)
                next.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -16).isActive = true
                body = next
            }
            let control = RowControl(row: row, tag: i, target: self)
            body?.addArrangedSubview(control.view)
            control.view.widthAnchor.constraint(equalTo: body!.widthAnchor).isActive = true
            rowControls.append(control)
        }
    }

    // MARK: - Controls

    @objc fileprivate func textCommitted(_ field: NSTextField) {
        guard rows.indices.contains(field.tag) else { return }
        commitText(rows[field.tag], field.stringValue)
    }

    @objc fileprivate func popupChanged(_ popup: NSPopUpButton) {
        guard rows.indices.contains(popup.tag), let choice = popup.titleOfSelectedItem else { return }
        applyChoice(rows[popup.tag], choice)
    }

    @objc fileprivate func toggleChanged(_ button: NSButton) {
        guard rows.indices.contains(button.tag) else { return }
        applyToggle(rows[button.tag], on: button.state == .on)
    }

    func controlTextDidEndEditing(_ obj: Notification) {
        guard let field = obj.object as? NSTextField else { return }
        textCommitted(field)
    }
}

/// One row of the column: the caption on the left and the control on the right, built once and refilled
/// when the tree changes.
@MainActor
private final class RowControl {
    let view: NSView

    private let kind: InspectorController.InspectorRow.Kind
    private let info: NSTextField?
    private let text: NSTextField?
    private let popup: NSPopUpButton?
    private let toggleButton: NSButton?

    init(row: InspectorController.InspectorRow, tag: Int, target: InspectorController) {
        kind = row.kind
        let caption = NSTextField(labelWithString: row.label)
        caption.font = NSFont.systemFont(ofSize: 11)
        caption.textColor = .secondaryLabelColor
        caption.lineBreakMode = .byTruncatingTail
        caption.widthAnchor.constraint(equalToConstant: 68).isActive = true

        var ctl: NSView
        switch row.kind {
        case .info:
            let label = NSTextField(labelWithString: row.text)
            label.font = NSFont.systemFont(ofSize: 11)
            label.textColor = .secondaryLabelColor
            label.lineBreakMode = .byTruncatingMiddle
            label.setContentCompressionResistancePriority(.defaultLow, for: .horizontal)
            info = label
            text = nil
            popup = nil
            toggleButton = nil
            ctl = label
        case .text:
            let field = NSTextField(frame: .zero)
            field.font = NSFont.systemFont(ofSize: 11)
            field.controlSize = .small
            field.isEditable = true
            field.isBordered = true
            field.bezelStyle = .roundedBezel
            field.lineBreakMode = .byTruncatingTail
            field.usesSingleLineMode = true
            field.placeholderString = row.placeholder
            field.tag = tag
            field.target = target
            field.action = #selector(InspectorController.textCommitted(_:))
            field.delegate = target
            field.setContentHuggingPriority(.defaultLow, for: .horizontal)
            field.setContentCompressionResistancePriority(.defaultLow, for: .horizontal)
            field.widthAnchor.constraint(greaterThanOrEqualToConstant: 40).isActive = true
            info = nil
            text = field
            popup = nil
            toggleButton = nil
            ctl = field
        case .popup:
            let button = NSPopUpButton(frame: .zero, pullsDown: false)
            button.font = NSFont.systemFont(ofSize: 11)
            button.controlSize = .small
            button.addItems(withTitles: row.choices)
            button.tag = tag
            button.target = target
            button.action = #selector(InspectorController.popupChanged(_:))
            button.setContentHuggingPriority(.defaultLow, for: .horizontal)
            info = nil
            text = nil
            popup = button
            toggleButton = nil
            ctl = button
        case .toggle:
            let button = NSButton(checkboxWithTitle: "", target: target, action: #selector(InspectorController.toggleChanged(_:)))
            button.controlSize = .small
            button.tag = tag
            info = nil
            text = nil
            popup = nil
            toggleButton = button
            ctl = button
        }

        let line = NSStackView(views: [caption, ctl])
        line.orientation = .horizontal
        line.alignment = .centerY
        line.spacing = 8
        line.translatesAutoresizingMaskIntoConstraints = false
        view = line
        apply(row)
    }

    func apply(_ row: InspectorController.InspectorRow) {
        switch kind {
        case .info:
            info?.stringValue = row.text
            info?.toolTip = row.text
        case .text:
            text?.placeholderString = row.placeholder
            if text?.currentEditor() == nil {
                text?.stringValue = row.text
            }
        case .popup:
            if let popup, popup.titleOfSelectedItem != row.text {
                if let i = row.choices.firstIndex(of: row.text) {
                    popup.selectItem(at: i)
                } else {
                    popup.select(nil)
                }
            }
        case .toggle:
            toggleButton?.state = row.text == "yes" ? .on : .off
        }
    }

    /// Takes the row out of the responder chain before its view is dropped with a rebuilt column.
    func detach() {
        text?.target = nil
        text?.delegate = nil
        popup?.target = nil
        toggleButton?.target = nil
    }
}
