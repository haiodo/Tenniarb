//
//  CompareController.swift
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

/// Compare mode of a scene window, as go/app/compare.go: the file at HEAD against the text of the
/// document. It holds the report and the rows the outline shows; a window drives the canvas from it.
@MainActor
final class CompareController {
    /// One row of the outline: a node of the report, one of its changed properties, or the note of a
    /// moved one. A row without a node is text and selects nothing.
    final class Row {
        let label: String
        let path: String
        let status: Diff.Status?
        let node: Diff.Node?
        let kids: [Row]

        fileprivate init(label: String, path: String, status: Diff.Status?, node: Diff.Node?, kids: [Row]) {
            self.label = label
            self.path = path
            self.status = status
            self.node = node
            self.kids = kids
        }
    }

    private weak var document: Document?
    private(set) var report: Diff.Report?
    /// Why the comparison could not start, or why the last one failed.
    private(set) var message: String?
    private(set) var repo = ""
    private(set) var rev = Diff.revHead
    private var oldText = ""
    private var rel = ""  // the file as the scene tree names it, for the loader
    private var root: URL?
    private var comparedFile: URL?

    /// Whether a report is shown; false when the mode is off or a refresh had to leave it.
    var isOn: Bool { report != nil }

    init(document: Document?) {
        self.document = document
    }

    /// Reads the file at the last commit and compares it with the text of the document. False leaves
    /// the mode off and message says why.
    @discardableResult
    func start() -> Bool {
        close()
        guard let document, let tree = document.sceneTree, let text = document.sceneText,
            let rel = tree.main?.pos.file, let url = document.fileURL
        else {
            message = "Save the document first: a comparison needs a file in a repository"
            return false
        }
        let file = url.resolvingSymlinksInPath().standardizedFileURL
        guard let repo = Diff.repo(file.deletingLastPathComponent()) else {
            message = "The file is not in a git repository: there is no version to compare with"
            return false
        }
        let repoRoot = URL(fileURLWithPath: repo)
        guard let path = Self.relative(from: repoRoot, to: file) else {
            message = "The file is outside the repository: there is no version to compare with"
            return false
        }
        guard let old = Diff.show(repo: repo, rev: rev, rel: path) else {
            message = "\(path) has no version at \(rev) in \(repo): commit the file first"
            return false
        }
        do {
            let report = try Diff.compare(root: tree.root, rel: rel, old: old, new: text)
            self.report = report
            self.repo = repo
            self.oldText = old
            self.rel = rel
            self.root = tree.root
            self.comparedFile = file
            self.message = nil
            return true
        } catch {
            message = "\(error)"
            return false
        }
    }

    /// Compares the text again after an edit; a document that now names another file leaves the mode.
    func refresh() {
        guard report != nil else { return }
        guard let document, document.sceneTree != nil, let text = document.sceneText,
            let url = document.fileURL?.resolvingSymlinksInPath().standardizedFileURL,
            url == comparedFile, let root
        else {
            close()
            return
        }
        do {
            report = try Diff.compare(root: root, rel: rel, old: oldText, new: text)
            message = nil
        } catch {
            message = "\(error)"
        }
    }

    func close() {
        report = nil
        message = nil
        repo = ""
        oldText = ""
        rel = ""
        root = nil
        comparedFile = nil
    }

    /// Whether the document still names the file the comparison was started on; a Save As makes it false.
    func isCurrent() -> Bool {
        guard let file = document?.fileURL?.resolvingSymlinksInPath().standardizedFileURL else { return false }
        return file == comparedFile
    }

    /// The line of a comparison: how many nodes changed, with the version.
    func summary() -> String {
        guard let report else { return "" }
        guard report.changed else { return "No changes against \(rev)" }
        let parts = Diff.Status.counts.compactMap { status -> String? in
            let n = report.count(status)
            return n > 0 ? "\(n) \(status.rawValue)" : nil
        }
        return "Against \(rev): " + parts.joined(separator: ", ")
    }

    /// The rows of the outline: every element, and the nodes that changed or hold a change.
    func rows() -> [Row] {
        guard let report else { return [] }
        return [row(report.root)]
    }

    /// The paths of the nodes the canvas draws a frame over, in the order the colours stack.
    func changedPaths() -> [String] {
        guard let report else { return [] }
        return [.changed, .added, .moved, .removed].flatMap { report.paths($0) }
    }

    /// The frames of the changed nodes; the canvas keeps the ones it draws on the element it shows.
    func frames() -> [SceneCanvasView.CompareFrame] {
        changedPaths().compactMap { path in
            guard let status = report?.node(path)?.status, let color = Self.color(status) else { return nil }
            return SceneCanvasView.CompareFrame(path: path, color: color)
        }
    }

    /// The colour of a status, as compareColors of go/app.
    static func color(_ status: Diff.Status?) -> NSColor? {
        switch status {
        case .added?: return NSColor(srgbRed: 22 / 255, green: 163 / 255, blue: 74 / 255, alpha: 1)
        case .removed?: return NSColor(srgbRed: 220 / 255, green: 38 / 255, blue: 38 / 255, alpha: 1)
        case .changed?: return NSColor(srgbRed: 217 / 255, green: 119 / 255, blue: 6 / 255, alpha: 1)
        case .moved?: return NSColor(srgbRed: 37 / 255, green: 99 / 255, blue: 235 / 255, alpha: 1)
        case nil: return nil
        }
    }

    // MARK: - Rows

    private func row(_ n: Diff.Node) -> Row {
        var lines = n.props.map { Row(label: propLabel($0), path: n.path, status: nil, node: nil, kids: []) }
        if !n.note.isEmpty {
            lines.append(Row(label: n.note, path: n.path, status: nil, node: nil, kids: []))
        }
        let kids = n.kids.filter { $0.kind == "element" || shows($0) }.map { row($0) }
        return Row(label: label(n), path: n.path, status: n.status, node: n, kids: lines + kids)
    }

    /// Whether a node is listed: it changed, or a node under it did.
    private func shows(_ n: Diff.Node) -> Bool {
        n.status != nil || n.kids.contains { shows($0) }
    }

    /// The text of a row: the name, the badge of its status and, for an element, how many nodes under
    /// it changed.
    private func label(_ n: Diff.Node) -> String {
        var label = n.name
        if let status = n.status {
            label += "  " + status.rawValue
        }
        if n.kind == "element" {
            let count = Self.count(n)
            if count > 0 {
                label += "  \(count)"
            }
        }
        return label
    }

    /// How many nodes under n, itself included, have a status.
    private static func count(_ n: Diff.Node) -> Int {
        (n.status == nil ? 0 : 1) + n.kids.reduce(0) { $0 + count($1) }
    }

    /// One changed property: "width  240 -> 200".
    private func propLabel(_ p: Diff.Prop) -> String {
        let old = Self.oneLine(p.old)
        let new = Self.oneLine(p.new)
        if !old.isEmpty && !new.isEmpty {
            return "\(p.name)  \(old) -> \(new)"
        }
        if !new.isEmpty {
            return "\(p.name)  \(new)"
        }
        if !old.isEmpty {
            return "\(p.name)  was \(old)"
        }
        return p.name
    }

    /// A property value on one short line: a template or a style can be long.
    private static func oneLine(_ text: String) -> String {
        let line = text.split(whereSeparator: \.isWhitespace).joined(separator: " ")
        return line.count > 60 ? String(line.prefix(57)) + "..." : line
    }

    /// The slash path of file from the repository root; nil when the file is outside it.
    private static func relative(from root: URL, to file: URL) -> String? {
        let base = root.resolvingSymlinksInPath().standardizedFileURL.path
        let target = file.resolvingSymlinksInPath().standardizedFileURL.path
        guard target.hasPrefix(base + "/") else { return nil }
        return String(target.dropFirst(base.count + 1))
    }
}

/// The outline rows of compare mode: the report tree with a colour per status. It replaces the element
/// outline only while the mode is on, so the element rows keep their drops and edits.
@MainActor
final class CompareOutlineSource: NSObject, NSOutlineViewDataSource, NSOutlineViewDelegate {
    private let compare: CompareController
    private var rows: [CompareController.Row] = []
    var onSelect: ((CompareController.Row) -> Void)?

    init(compare: CompareController) {
        self.compare = compare
        super.init()
        rows = compare.rows()
    }

    var topRows: [CompareController.Row] { rows }

    func reload() { rows = compare.rows() }

    func outlineView(_ outlineView: NSOutlineView, numberOfChildrenOfItem item: Any?) -> Int {
        guard let row = item as? CompareController.Row else { return rows.count }
        return row.kids.count
    }

    func outlineView(_ outlineView: NSOutlineView, child index: Int, ofItem item: Any?) -> Any {
        guard let row = item as? CompareController.Row else { return rows[index] }
        return row.kids[index]
    }

    func outlineView(_ outlineView: NSOutlineView, isItemExpandable item: Any) -> Bool {
        (item as? CompareController.Row)?.kids.isEmpty == false
    }

    func outlineView(_ outlineView: NSOutlineView, shouldEdit tableColumn: NSTableColumn?, item: Any) -> Bool {
        false
    }

    func outlineView(_ outlineView: NSOutlineView, objectValueFor tableColumn: NSTableColumn?, byItem item: Any?) -> Any? {
        (item as? CompareController.Row)?.label
    }

    func outlineView(_ outlineView: NSOutlineView, viewFor tableColumn: NSTableColumn?, item: Any) -> NSView? {
        guard let row = item as? CompareController.Row else { return nil }
        let identifier = NSUserInterfaceItemIdentifier(rawValue: "CompareCell")
        var cellView = outlineView.makeView(withIdentifier: identifier, owner: self) as? NSTableCellView
        if cellView == nil {
            let view = NSTableCellView(frame: NSRect(x: 0, y: 0, width: 200, height: 22))
            view.identifier = identifier
            let field = NSTextField(frame: .zero)
            field.translatesAutoresizingMaskIntoConstraints = false
            field.isBordered = false
            field.isEditable = false
            field.drawsBackground = false
            field.focusRingType = .none
            field.usesSingleLineMode = true
            field.lineBreakMode = .byTruncatingTail
            view.addSubview(field)
            view.textField = field
            NSLayoutConstraint.activate([
                field.leadingAnchor.constraint(equalTo: view.leadingAnchor, constant: 2),
                field.trailingAnchor.constraint(equalTo: view.trailingAnchor, constant: -4),
                field.centerYAnchor.constraint(equalTo: view.centerYAnchor),
            ])
            cellView = view
        }
        if let field = cellView?.textField {
            field.stringValue = row.label
            field.font = row.node == nil ? NSFont.menuFont(ofSize: 12) : NSFont.boldSystemFont(ofSize: 12)
            field.textColor = CompareController.color(row.status) ?? .labelColor
        }
        return cellView
    }

    func outlineViewSelectionDidChange(_ notification: Notification) {
        guard let outline = notification.object as? NSOutlineView,
            let row = outline.item(atRow: outline.selectedRow) as? CompareController.Row, row.node != nil
        else { return }
        onSelect?(row)
    }
}
