//
//  CommentsController.swift
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

/// The node paths of an anchor, as go/app/doccore_paths.go: the names from the element down, joined by
/// dots, with "#n" for a repeated name. The Go service reads the same spelling, so it is the format.
enum CommentPaths {

    /// "name#n" as NodePath writes it: the key and the place among the siblings that share it.
    static func splitIndex(_ seg: String) -> (key: String, index: Int) {
        if let hash = seg.lastIndex(of: "#"), let k = Int(seg[seg.index(after: hash)...]), k > 0 {
            return (String(seg[..<hash]), k)
        }
        return (seg, 0)
    }

    /// What go/editor SceneKids lists under n: its items and uses, and the slots of an item.
    static func sceneKids(_ n: Scene.Node) -> [Scene.Node] {
        var out: [Scene.Node] = []
        for c in n.content {
            switch c.kind {
            case .item, .use: out.append(c)
            case .slot where n.kind == .item: out.append(c)
            default: break
            }
        }
        return out
    }

    /// The segment of n among its siblings: its key, and "#n" for the n-th earlier sibling of that key.
    static func keySegment(_ n: Scene.Node, among siblings: [Scene.Node]) -> String {
        var k = 0
        for c in siblings {
            if c === n { break }
            if c.key == n.key { k += 1 }
        }
        return k > 0 ? "\(n.key)#\(k)" : n.key
    }

    /// The segment of an element among its siblings, by name as go/model PathOf writes it.
    static func nameSegment(_ n: Scene.Node, among siblings: [Scene.Node]) -> String {
        var k = 0
        for c in siblings {
            if c === n { break }
            if c.name == n.name { k += 1 }
        }
        return k > 0 ? "\(n.name)#\(k)" : n.name
    }

    /// The path from the element to n as NodePath: "" when n is not written inside it.
    static func nodePath(of n: Scene.Node, from element: Scene.Node?) -> String {
        guard let element else { return "" }
        let mount = element.mount
        var segs: [String] = []
        var cur: Scene.Node? = n
        while let c = cur, c !== element, c !== mount {
            guard let parent = c.parent else { return "" }
            segs.append(keySegment(c, among: sceneKids(parent)))
            cur = parent
        }
        return cur == nil ? "" : segs.reversed().joined(separator: ".")
    }

    /// The node of a path NodePath wrote under element; nil when the node is gone.
    static func node(at path: String, from element: Scene.Node?) -> Scene.Node? {
        guard let element, !path.isEmpty else { return nil }
        var cur = element
        for seg in path.split(separator: ".", omittingEmptySubsequences: false) {
            let (key, index) = splitIndex(String(seg))
            var left = index
            var next: Scene.Node?
            for c in sceneKids(cur) where c.key == key {
                if left == 0 {
                    next = c
                    break
                }
                left -= 1
            }
            guard let next else { return nil }
            cur = next
        }
        return cur
    }

    /// The names from the file root down to an element, as go/model PathOf, with "#n" for a repeat.
    static func elementPath(of node: Scene.Node) -> [String] {
        var segs: [String] = []
        var cur = node
        while cur.kind != .file, let parent = cur.parent {
            segs.append(nameSegment(cur, among: parent.elems))
            cur = parent
        }
        return segs.reversed()
    }
}

/// One comment on the canvas: where its marker sits, in the scene coordinates of the element.
struct CommentPin: Equatable {
    /// The size of a marker in view points; a click within it opens the thread.
    static let radius: CGFloat = 11

    let id: String
    let number: Int
    let point: CGPoint
    /// The node of the anchor is not on the canvas any more: the pin sits at the element origin.
    let detached: Bool
    let resolved: Bool

    /// The pin under point, the one drawn last wins, as go/app PinAtView.
    static func at(_ pins: [CommentPin], point: CGPoint, radius: CGFloat) -> CommentPin? {
        for pin in pins.reversed() {
            let dx = point.x - pin.point.x
            let dy = point.y - pin.point.y
            if dx * dx + dy * dy <= radius * radius { return pin }
        }
        return nil
    }
}

/// The comments of the document in one window: the store of the config folder, the pins on the canvas of
/// the shown element, and the thread of the comment a pin opens.
@MainActor
final class CommentsController {

    enum Error: LocalizedError, Equatable {
        /// A document without a file has no comments file to write.
        case untitled

        var errorDescription: String? {
            switch self {
            case .untitled: return "Save the file to comment on it."
            }
        }
    }

    /// The identity of the document: its repository and path inside it, or the absolute path when no
    /// repository with a remote holds it; the name of its comments file.
    let key: String?

    /// One lookup for every controller: git runs once per repository root.
    private static let repos = Links.Repos()

    private let store: Comments.Store
    private var doc: Comments.Doc
    /// The pins of the last pins(...) call, for pin(at:).
    private(set) var lastPins: [CommentPin] = []
    /// The failure of the last thread action, shown in its popover.
    private(set) var threadError: String?
    /// Whether resolved comments keep their pins and rows; View > Show Resolved Comments asks for it.
    private(set) var showResolved = false
    /// Called after a write and its reload, so the canvas places the pins again.
    var onChange: (() -> Void)?

    private var threadPopover: NSPopover?
    private var threadID: String?
    private var listPopover: NSPopover?

    init(key: String?, configDir: URL = Comments.configDir()) {
        self.key = key
        store = Comments.Store(dir: Comments.dirIn(configDir))
        doc = Comments.Doc(key: key ?? "")
        reload()
    }

    /// The comments of the document at url; nil is an untitled document, which has no comments.
    convenience init(url: URL?, configDir: URL = Comments.configDir()) {
        self.init(key: url.map { CommentsController.repos.docKey($0.path) }, configDir: configDir)
    }

    // MARK: - Reading

    func comments() -> [Comments.Comment] { doc.comments }

    func comment(_ id: String) -> Comments.Comment? { doc.find(id) }

    func reload() {
        guard let key else {
            doc = Comments.Doc(key: "")
            return
        }
        do {
            doc = try store.load(key)
            threadError = nil
        } catch {
            doc = Comments.Doc(key: key)
            threadError = error.localizedDescription
        }
    }

    // MARK: - Writing

    @discardableResult
    func addComment(text: String, anchor: Comments.Anchor, now: Date = Date()) throws -> Comments.Comment {
        var added: Comments.Comment?
        try change { doc in
            added = try doc.add(author: Comments.user, text: text, anchor: anchor, now: now)
        }
        guard let added else { throw Comments.Error.notFound }
        return added
    }

    func reply(id: String, text: String, now: Date = Date()) throws {
        try change { doc in
            try doc.reply(id: id, author: Comments.user, text: text, now: now)
        }
    }

    func resolve(id: String, text: String = "", now: Date = Date()) throws {
        try change { doc in
            try doc.setState(id: id, state: Comments.resolved, author: Comments.user, text: text, now: now)
        }
    }

    func reopen(id: String, now: Date = Date()) throws {
        try change { doc in
            try doc.setState(id: id, state: Comments.open, author: Comments.user, text: "", now: now)
        }
    }

    func delete(id: String) throws {
        try change { doc in
            try doc.delete(id: id, author: Comments.user)
        }
    }

    private func change(_ body: (inout Comments.Doc) throws -> Void) throws {
        guard let key else { throw Error.untitled }
        threadError = nil
        do {
            _ = try store.update(key, body)
            reload()
            onChange?()
        } catch {
            threadError = error.localizedDescription
            throw error
        }
    }

    // MARK: - Pins

    /// The pins of the comments on element, as go/app PlacePins: a node box's top-right corner when the
    /// comment has no point, the box top-left plus the point when it has one, and origin plus the point
    /// for a comment without a node. A node that is not drawn puts its pin at the origin, detached.
    func pins(
        element: [String], box: (String) -> CGRect?, origin: CGPoint = .zero, showResolved: Bool = false
    ) -> [CommentPin] {
        var out: [CommentPin] = []
        for (i, c) in doc.comments.enumerated() {
            if (c.anchor.element ?? []) != element { continue }
            if c.state == Comments.resolved && !showResolved { continue }
            let (point, detached) = pinPlace(c.anchor, box: box, origin: origin)
            out.append(
                CommentPin(
                    id: c.id, number: i + 1, point: point, detached: detached, resolved: c.state == Comments.resolved))
        }
        lastPins = out
        return out
    }

    private func pinPlace(
        _ anchor: Comments.Anchor, box: (String) -> CGRect?, origin: CGPoint
    ) -> (CGPoint, Bool) {
        let offset = anchor.point.map { CGPoint(x: $0.x, y: $0.y) } ?? .zero
        guard let node = anchor.node, !node.isEmpty else {
            return (CGPoint(x: origin.x + offset.x, y: origin.y + offset.y), false)
        }
        guard let b = box(node) else {
            return (origin, true)
        }
        if anchor.point == nil {
            return (CGPoint(x: b.maxX, y: b.minY), false)
        }
        return (CGPoint(x: b.minX + offset.x, y: b.minY + offset.y), false)
    }

    /// The pin of the last pins(...) call under point; the one drawn last wins.
    func pin(at point: CGPoint, radius: CGFloat = CommentPin.radius) -> CommentPin? {
        CommentPin.at(lastPins, point: point, radius: radius)
    }

    /// Puts resolved comments back in or out of the canvas; the window redraws its pins through onChange.
    func setShowResolved(_ on: Bool) {
        guard on != showResolved else { return }
        showResolved = on
        onChange?()
    }

    // MARK: - The list

    /// The rows of View > Comments…, in the order of the document, as go/app commentLabel: the number,
    /// the time, the author and the start of the text, then the place.
    func listRows(now: Date = Date()) -> [(id: String, number: Int, label: String, resolved: Bool)] {
        doc.comments.enumerated().map { i, c in
            let resolved = c.state == Comments.resolved
            var label = "\(i + 1)  \(CommentsController.whenText(c.created, now: now))  \(c.author): \(Self.firstLine(c.text))"
            let place = anchorText(c.anchor)
            if !place.isEmpty { label += "  " + place }
            if resolved { label += "  (resolved)" }
            return (c.id, i + 1, label, resolved)
        }
    }

    /// The first line of a text, its whitespace folded and its end capped as go/app commentLabel.
    private static func firstLine(_ text: String) -> String {
        let line = text.split(whereSeparator: \.isNewline).first.map(String.init) ?? ""
        let folded = line.split(whereSeparator: \.isWhitespace).joined(separator: " ")
        return folded.count > 48 ? String(folded.prefix(47)) + "..." : folded
    }

    /// The popover of View > Comments…: a row per comment, the resolved ones hidden until asked for; a
    /// double click or Return goes to the place of the row (go/app comments_thread.go showCommentList).
    func presentList(from window: NSWindow?, onJump: @escaping @MainActor (String) -> Void) {
        guard let window, let anchor = window.contentView else { return }
        closeThread()
        listPopover?.close()
        let rows = listRows().filter { showResolved || !$0.resolved }
        let list = CommentListController(rows: rows) { [weak self] id in
            self?.listPopover?.close()
            self?.listPopover = nil
            onJump(id)
        }
        let popover = NSPopover()
        popover.behavior = .transient
        popover.contentViewController = list
        listPopover = popover
        popover.show(
            relativeTo: NSRect(x: anchor.bounds.midX, y: anchor.bounds.maxY, width: 1, height: 1), of: anchor,
            preferredEdge: .minY)
    }

    /// Where a comment sits, for the window of View > Comments… to show: the names of its element, the
    /// node path inside it, and the point of the anchor when it has one.
    func jumpTarget(id: String) -> (element: [String], node: String, point: Comments.Point?)? {
        guard let c = doc.find(id) else { return nil }
        return (c.anchor.element ?? [], c.anchor.node ?? "", c.anchor.point)
    }

    // MARK: - Thread

    /// The lines the popover of a thread shows, without a window, as go/app comments_thread.go showThread.
    func threadText(id: String, now: Date = Date()) -> [String] {
        guard let c = doc.find(id) else { return [] }
        var lines: [String] = []
        var head = "#\(numberOf(id))  \(c.author)  \(CommentsController.whenText(c.created, now: now))"
        if c.state == Comments.resolved { head += "  (resolved)" }
        lines.append(head)
        let place = anchorText(c.anchor)
        if !place.isEmpty { lines.append(place) }
        if lastPins.first(where: { $0.id == id })?.detached == true {
            lines.append("The node is gone: the pin sits at the element origin.")
        }
        lines.append(c.text)
        for r in c.replies ?? [] {
            lines.append("\(r.author)  \(CommentsController.whenText(r.created, now: now))")
            lines.append(r.text)
        }
        if let threadError { lines.append(threadError) }
        return lines
    }

    /// Opens the thread of a comment at a place of view; its buttons write through this controller.
    func showThread(id: String, from rect: NSRect, of view: NSView) {
        guard doc.find(id) != nil else { return }
        closeThread()
        let popover = NSPopover()
        popover.behavior = .transient
        let controller = NSViewController()
        threadPopover = popover
        threadID = id
        threadError = nil
        rebuildThread(controller)
        popover.contentViewController = controller
        popover.show(relativeTo: rect, of: view, preferredEdge: .maxY)
    }

    func closeThread() {
        threadPopover?.close()
        threadPopover = nil
        threadID = nil
    }

    private func numberOf(_ id: String) -> Int {
        doc.comments.firstIndex { $0.id == id }.map { $0 + 1 } ?? 0
    }

    private func anchorText(_ a: Comments.Anchor) -> String {
        var place = (a.element ?? []).joined(separator: ".")
        if let node = a.node, !node.isEmpty { place += " / " + node }
        return place
    }

    /// The time of a thread line, as Go's commentWhen: the time today, the date and time before.
    static func whenText(_ date: Date, now: Date = Date()) -> String {
        let formatter = DateFormatter()
        formatter.locale = Locale(identifier: "en_US_POSIX")
        formatter.timeZone = .current
        formatter.dateFormat = Calendar.current.isDate(date, inSameDayAs: now) ? "HH:mm" : "d MMM HH:mm"
        return formatter.string(from: date)
    }

    /// Builds the thread in controller: the lines, a field for the reply and the buttons of the state.
    private func rebuildThread(_ controller: NSViewController) {
        guard let popover = threadPopover, let id = threadID, doc.find(id) != nil else {
            closeThread()
            return
        }
        let width: CGFloat = 320
        let stack = NSStackView()
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 6
        stack.edgeInsets = NSEdgeInsets(top: 10, left: 10, bottom: 10, right: 10)
        stack.translatesAutoresizingMaskIntoConstraints = false
        for (i, line) in threadText(id: id).enumerated() {
            stack.addArrangedSubview(label(line, faint: i == 0, width: width))
        }

        let (field, scroll) = replyField(width: width)
        stack.addArrangedSubview(scroll)
        let buttons = NSStackView()
        buttons.orientation = .horizontal
        buttons.spacing = 6
        buttons.widthAnchor.constraint(equalToConstant: width).isActive = true
        let spacer = NSView()
        spacer.setContentHuggingPriority(.defaultLow, for: .horizontal)
        buttons.addArrangedSubview(spacer)
        buttons.addArrangedSubview(
            ActionButton("Reply") { [weak self] in
                guard let self else { return }
                let text = field.string.trimmingCharacters(in: .whitespacesAndNewlines)
                guard !text.isEmpty else { return }
                do {
                    try self.reply(id: id, text: text)
                    field.string = ""
                } catch {
                    self.threadError = error.localizedDescription
                }
                self.rebuildThread(controller)
            })
        let resolved = doc.find(id)?.state == Comments.resolved
        buttons.addArrangedSubview(
            ActionButton(resolved ? "Reopen" : "Resolve") { [weak self] in
                guard let self else { return }
                do {
                    if resolved {
                        try self.reopen(id: id)
                    } else {
                        try self.resolve(id: id, text: field.string)
                    }
                } catch {
                    self.threadError = error.localizedDescription
                }
                self.rebuildThread(controller)
            })
        if doc.find(id)?.author == Comments.user {
            buttons.addArrangedSubview(
                ActionButton("Delete") { [weak self] in
                    guard let self else { return }
                    do {
                        try self.delete(id: id)
                        self.closeThread()
                    } catch {
                        self.threadError = error.localizedDescription
                        self.rebuildThread(controller)
                    }
                })
        }
        stack.addArrangedSubview(buttons)

        let root = NSView()
        root.addSubview(stack)
        NSLayoutConstraint.activate([
            stack.leadingAnchor.constraint(equalTo: root.leadingAnchor),
            stack.trailingAnchor.constraint(equalTo: root.trailingAnchor),
            stack.topAnchor.constraint(equalTo: root.topAnchor),
            stack.bottomAnchor.constraint(equalTo: root.bottomAnchor),
        ])
        let size = stack.fittingSize
        root.frame = NSRect(origin: .zero, size: size)
        controller.view = root
        controller.preferredContentSize = size
        popover.contentSize = size
    }

    private func label(_ text: String, faint: Bool, width: CGFloat) -> NSTextField {
        let label = NSTextField(labelWithString: text)
        label.font = NSFont.systemFont(ofSize: faint ? 11 : 13)
        label.textColor = faint ? .secondaryLabelColor : .labelColor
        label.lineBreakMode = .byWordWrapping
        label.maximumNumberOfLines = 0
        label.preferredMaxLayoutWidth = width
        label.widthAnchor.constraint(equalToConstant: width).isActive = true
        return label
    }

    /// The reply field and its scroll view, sized as the width of the thread.
    private func replyField(width: CGFloat) -> (NSTextView, NSScrollView) {
        let field = NSTextView(frame: NSRect(x: 0, y: 0, width: width, height: 54))
        field.font = NSFont.systemFont(ofSize: 13)
        field.isRichText = false
        field.isVerticallyResizable = true
        field.isHorizontallyResizable = false
        field.textContainer?.widthTracksTextView = true
        field.autoresizingMask = [.width]
        let scroll = NSScrollView(frame: NSRect(x: 0, y: 0, width: width, height: 54))
        scroll.borderType = .bezelBorder
        scroll.hasVerticalScroller = true
        scroll.documentView = field
        scroll.widthAnchor.constraint(equalToConstant: width).isActive = true
        scroll.heightAnchor.constraint(equalToConstant: 54).isActive = true
        return (field, scroll)
    }
}

/// A button that runs a closure, for the popover of a thread, whose content is rebuilt after every action.
private final class ActionButton: NSButton {
    private let run: () -> Void

    init(_ title: String, _ run: @escaping () -> Void) {
        self.run = run
        super.init(frame: .zero)
        self.title = title
        bezelStyle = .rounded
        target = self
        action = #selector(clicked)
    }

    required init?(coder: NSCoder) {
        fatalError("init(coder:) has not been implemented")
    }

    @objc private func clicked() {
        run()
    }
}

/// The table of the comment list: a row goes to its place on a double click or Return.
@MainActor
private final class CommentListController: NSViewController, NSTableViewDataSource, NSTableViewDelegate {

    private let rows: [(id: String, number: Int, label: String, resolved: Bool)]
    private let onJump: @MainActor (String) -> Void
    private let table = CommentListTable()

    init(
        rows: [(id: String, number: Int, label: String, resolved: Bool)],
        onJump: @escaping @MainActor (String) -> Void
    ) {
        self.rows = rows
        self.onJump = onJump
        super.init(nibName: nil, bundle: nil)
    }

    required init?(coder: NSCoder) {
        fatalError("init(coder:) has not been implemented")
    }

    override func loadView() {
        let width: CGFloat = 460
        let rowsHeight = min(CGFloat(rows.count) * 22 + 8, 320)
        let root = NSView(frame: NSRect(x: 0, y: 0, width: width, height: rows.isEmpty ? 30 : rowsHeight))
        if rows.isEmpty {
            let label = NSTextField(labelWithString: "No comments.")
            label.font = NSFont.systemFont(ofSize: 12)
            label.textColor = .secondaryLabelColor
            label.frame = NSRect(x: 10, y: 6, width: width - 20, height: 18)
            root.addSubview(label)
        } else {
            let column = NSTableColumn(identifier: NSUserInterfaceItemIdentifier(rawValue: "CommentColumn"))
            column.resizingMask = .autoresizingMask
            table.addTableColumn(column)
            table.columnAutoresizingStyle = .firstColumnOnlyAutoresizingStyle
            table.headerView = nil
            table.rowHeight = 22
            table.intercellSpacing = .zero
            table.allowsMultipleSelection = false
            table.dataSource = self
            table.delegate = self
            table.target = self
            table.doubleAction = #selector(doubleClicked)
            table.onReturn = { [weak self] in
                guard let self else { return }
                self.jump(self.table.selectedRow)
            }
            table.frame = NSRect(x: 0, y: 0, width: width - 8, height: root.frame.height - 8)
            table.autoresizingMask = [.width]
            let scroll = NSScrollView(frame: root.bounds.insetBy(dx: 4, dy: 4))
            scroll.autoresizingMask = [.width, .height]
            scroll.hasVerticalScroller = true
            scroll.autohidesScrollers = true
            scroll.borderType = .noBorder
            scroll.documentView = table
            root.addSubview(scroll)
            table.selectRowIndexes(IndexSet(integer: 0), byExtendingSelection: false)
        }
        view = root
        preferredContentSize = root.frame.size
    }

    override func viewDidAppear() {
        super.viewDidAppear()
        view.window?.makeFirstResponder(table)
    }

    private func jump(_ row: Int) {
        guard rows.indices.contains(row) else { return }
        onJump(rows[row].id)
    }

    @objc private func doubleClicked() {
        jump(table.clickedRow)
    }

    func numberOfRows(in tableView: NSTableView) -> Int { rows.count }

    func tableView(_ tableView: NSTableView, viewFor tableColumn: NSTableColumn?, row: Int) -> NSView? {
        let identifier = NSUserInterfaceItemIdentifier(rawValue: "CommentCell")
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
        guard rows.indices.contains(row) else { return cell }
        cell.textField?.stringValue = rows[row].label
        cell.textField?.font = NSFont.systemFont(ofSize: 12)
        cell.alphaValue = rows[row].resolved ? 0.5 : 1
        return cell
    }
}

/// The table of the comment list, which takes Return as the jump of the selected row.
@MainActor
private final class CommentListTable: NSTableView {
    var onReturn: (() -> Void)?

    override func keyDown(with event: NSEvent) {
        if event.keyCode == 36 || event.keyCode == 76 {
            onReturn?()
            return
        }
        super.keyDown(with: event)
    }
}
