//
//  SceneCanvasView.swift
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

/// A scene document on the canvas: one built screen drawn by the scene engine, with a click selecting a
/// drawn item and a double click editing its title in a native field.
final class SceneCanvasView: NSView, NSTextFieldDelegate {
    /// Room around the content, as geometry.Padding of the Go canvas.
    static let padding = 15.0

    private(set) var screen: Scene.Screen?
    /// The drawn items of the screen; the picture and the hit-test share this tree.
    private(set) var placed: Scene.Placed?
    /// The tree the drawn item nodes belong to, for the text their edits are cut from.
    private var tree: Scene.Tree?
    private var painter: ScenePainter?
    private(set) var message: String?
    /// The scale the content is drawn at; the scroll view fits the screen into itself.
    var zoom = 1.0 {
        didSet { needsDisplay = true }
    }

    /// The document an edit is written through; nil on a canvas nobody can edit.
    weak var document: Document?

    /// The indexes from the root of the drawn tree to the selected item; nil when nothing is selected.
    private(set) var selection: [Int]?

    /// A part of a component drawn in an instance: the use around it, and its path from the root of the
    /// instance. at is empty when the part cannot be named inside the instance.
    struct Part {
        let use: Scene.Node
        let at: String
    }

    /// The node whose content the screen draws: the element, or the file root when it has no element.
    private var element: Scene.Node?
    /// The element the canvas shows among the elements of the file; nil shows the first one.
    private(set) var elementPath: [Int]?
    /// The drawn items a click can select: written in the element, or a part of a component under a use.
    private var selectable = Set<ObjectIdentifier>()
    /// Where the parts among them are edited; a written item is not here.
    private var partOf: [ObjectIdentifier: Part] = [:]
    /// The index path of every selectable item from the root's children.
    private var pathOf: [ObjectIdentifier: [Int]] = [:]

    /// The indexes of the item whose title the field edits, and the text it opened with.
    private var editing: [Int]?
    private var editTitle = ""
    private(set) var editField: NSTextField?
    private var editFontSize = Scene.defaultFontSize

    override var isFlipped: Bool { true }
    override var acceptsFirstResponder: Bool { true }

    /// Shows the first element of tree, as the Go window opens a document; nil clears the canvas.
    func show(_ tree: Scene.Tree?) {
        endTitleEdit(commit: false)
        guard let tree else {
            screen = nil
            placed = nil
            painter = nil
            message = nil
            selection = nil
            self.tree = nil
            element = nil
            elementPath = nil
            forgetSelection()
            needsDisplay = true
            return
        }
        self.tree = tree
        let el = shownElement(tree) ?? tree.main?.elems.first { $0.kind == .element }
        let built: Scene.Screen?
        if let el {
            built = tree.buildElement(el, measurer: Scene.TextMeasurer(), eval: sceneEvalVars)
        } else {
            built = tree.build("", measurer: Scene.TextMeasurer(), eval: sceneEvalVars)
        }
        element = el ?? tree.main
        screen = built
        placed = built?.placed()
        painter = built.map(ScenePainter.init)
        message =
            built == nil
            ? tree.diagnostics.first { !$0.warn }.map { "\($0.pos): \($0.message)" } ?? "the scene does not build"
            : nil
        indexSelection()
        if placed(at: selection) == nil {
            selection = nil
        }
        needsDisplay = true
    }

    /// Shows another element of the same tree, as a click in the outline does.
    func showElement(_ path: [Int]?) {
        elementPath = path
        show(tree)
    }

    /// The element at the path the outline chose; nil when it is gone, and the first element is shown.
    private func shownElement(_ tree: Scene.Tree) -> Scene.Node? {
        guard let path = elementPath, !path.isEmpty else { return nil }
        var nodes = tree.main?.elems ?? []
        var node: Scene.Node?
        for i in path {
            guard nodes.indices.contains(i) else { return nil }
            node = nodes[i]
            nodes = nodes[i].elems
        }
        return node?.kind == .element ? node : nil
    }

    /// The zoom that shows all of a cw by ch content in a vw by vh view, but never above 1, as FitCamera.
    static func fitZoom(_ cw: Double, _ ch: Double, _ vw: Double, _ vh: Double) -> Double {
        let zoom = cw > 0 && ch > 0 && vw > 0 && vh > 0 ? min(vw / cw, vh / ch) : 1
        return min(1, max(0.02, zoom))
    }

    /// The size of the content with the padding around it.
    static func contentSize(_ screen: Scene.Screen) -> NSSize {
        NSSize(width: screen.w + 2 * padding, height: screen.h + 2 * padding)
    }

    /// One row of the Items pane: a drawn item one can select and whether it holds items of its own.
    struct ItemRow {
        let label: String
        let path: [Int]
        let kids: Bool
    }

    /// The rows of the Items pane under an item: the drawn kids of the element, or of the item at path.
    func itemRows(_ path: [Int]? = nil) -> [ItemRow] {
        guard let pl = path == nil ? placed : placed(at: path) else { return [] }
        return pl.kids.enumerated().map { i, k in
            ItemRow(
                label: k.r.name.isEmpty ? (k.src?.kind.description ?? "item") : k.r.name,
                path: (path ?? []) + [i], kids: !k.kids.isEmpty)
        }
    }

    /// The drawn item at an index path from the screen root's children; nil when the path is gone.
    func placed(at path: [Int]?) -> Scene.Placed? {
        guard let root = placed, let path else { return nil }
        var pl = root
        for i in path {
            guard pl.kids.indices.contains(i) else { return nil }
            pl = pl.kids[i]
        }
        return pl
    }

    // MARK: - What a click can select

    private func forgetSelection() {
        selectable = []
        partOf = [:]
        pathOf = [:]
    }

    /// Finds the items one can select, as go/app/sceneview.go index: what is written in the element, and
    /// inside an instance the parts of the component the use around it names.
    private func indexSelection() {
        forgetSelection()
        guard let root = placed else { return }
        var seen = Set<ObjectIdentifier>()
        func walk(_ pl: Scene.Placed, _ path: [Int]) {
            for (i, k) in pl.kids.enumerated() {
                add(k, path + [i], &seen)
                walk(k, path + [i])
            }
        }
        walk(root, [])
    }

    private func add(_ k: Scene.Placed, _ path: [Int], _ seen: inout Set<ObjectIdentifier>) {
        let owner = nearestInstance(k.parent)
        if let src = k.src, let segs = sourcePath(src), !segs.isEmpty,
            owner == nil || written(owner) && under(src, owner?.src)
        {
            if seen.insert(ObjectIdentifier(src)).inserted {
                selectable.insert(ObjectIdentifier(k))
                pathOf[ObjectIdentifier(k)] = path
            }
            return
        }
        guard var anchor = k.parent else { return }
        while !(anchor.inst && written(anchor)) {
            guard let parent = anchor.parent else { return }
            anchor = parent
        }
        var segs: [String] = []
        var plain: [String] = []
        var cur: Scene.Placed? = k
        while let c = cur, c !== anchor {
            plain.append(c.src?.key ?? "")
            if let sl = c.src?.parent, sl.kind == .slot {  // the fallback of a slot is reached through the slot
                plain.append(slotSeg(sl))
            }
            segs.append(drawnSeg(c))
            cur = c.parent
        }
        guard let use = anchor.src else { return }
        let base = sourcePath(use) ?? []
        let addressable = plain.allSatisfy { !$0.isEmpty } && !(base + segs).contains { $0.contains("#") }
        selectable.insert(ObjectIdentifier(k))
        partOf[ObjectIdentifier(k)] = Part(
            use: use, at: addressable ? plain.reversed().joined(separator: ".") : "")
        pathOf[ObjectIdentifier(k)] = path
    }

    /// Whether pl is an item written in the element, not a part of a component: its path has no "/".
    private func written(_ pl: Scene.Placed?) -> Bool {
        guard let pl, selectable.contains(ObjectIdentifier(pl)) else { return false }
        return partOf[ObjectIdentifier(pl)] == nil
    }

    private func nearestInstance(_ pl: Scene.Placed?) -> Scene.Placed? {
        var p = pl
        while let cur = p {
            if cur.inst { return cur }
            p = cur.parent
        }
        return nil
    }

    /// The names from the element down to n, with #n for a repeated name, as go/app NodePath; nil when n
    /// is not written inside the element.
    private func sourcePath(_ n: Scene.Node) -> [String]? {
        guard let el = element else { return nil }
        let mount = el.mount
        var segs: [String] = []
        var cur: Scene.Node? = n
        while let c = cur, c !== el, c !== mount {
            guard let parent = c.parent else { return nil }
            segs.append(sourceSeg(parent, c))
            cur = parent
        }
        return cur == nil ? nil : segs.reversed()
    }

    /// What go/editor SceneKids lists under n: the items and uses, and the slots of an item.
    private func sceneKids(_ n: Scene.Node) -> [Scene.Node] {
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

    private func sourceSeg(_ parent: Scene.Node, _ n: Scene.Node) -> String {
        var k = 0
        for c in sceneKids(parent) {
            if c === n { break }
            if c.key == n.key { k += 1 }
        }
        return k > 0 ? "\(n.key)#\(k)" : n.key
    }

    /// The name of p among the drawn siblings, with #n for the n-th repeat.
    private func drawnSeg(_ p: Scene.Placed) -> String {
        let key = p.src?.key ?? ""
        guard let parent = p.parent else { return key }
        var k = 0
        for c in parent.kids {
            if c === p { break }
            if c.src?.key == p.src?.key { k += 1 }
        }
        return k > 0 ? "\(key)#\(k)" : key
    }

    private func slotSeg(_ sl: Scene.Node) -> String { sl.key.isEmpty ? "default" : sl.key }

    private func under(_ n: Scene.Node, _ owner: Scene.Node?) -> Bool {
        guard let owner else { return false }
        var c = n.parent
        while let cur = c {
            if cur === owner { return true }
            c = cur.parent
        }
        return false
    }

    /// What a click on hit selects: the outermost instance above it, and a double click one level deeper
    /// toward hit, as go/app/sceneview_hit.go pick.
    private func pick(_ hit: [Int], clicks: Int) -> [Int] {
        guard let hitPl = placed(at: hit) else { return hit }
        var chain: [Scene.Placed] = []
        var cur: Scene.Placed? = hitPl
        while let c = cur {
            if selectable.contains(ObjectIdentifier(c)) { chain.append(c) }
            cur = c.parent
        }
        chain.reverse()
        guard let top = chain.firstIndex(where: { $0.inst }) else { return hit }
        let at = placed(at: selection).flatMap { sel in chain.firstIndex { $0 === sel } } ?? -1
        if clicks >= 2 {
            if at >= 0, at + 1 < chain.count, let next = pathOf[ObjectIdentifier(chain[at + 1])] {
                return next
            }
            if selectable.contains(ObjectIdentifier(hitPl)), let p = pathOf[ObjectIdentifier(hitPl)] {
                return p
            }
        }
        return pathOf[ObjectIdentifier(chain[top])] ?? hit
    }

    // MARK: - Selection

    /// Selects the drawn item at path; nil clears the selection.
    func select(_ path: [Int]?) {
        let next = placed(at: path) == nil ? nil : path
        guard next != selection else { return }
        selection = next
        needsDisplay = true
    }

    /// The item a press selects: the deepest item there that draws something, else the deepest box that
    /// holds it, as go/app/sceneview_hit.go hitScene. The element of the screen is not selectable.
    func hitItem(_ p: Scene.Point) -> [Int]? {
        guard let root = placed else { return nil }
        return paintedItem(p, root, []) ?? boxedItem(p, root, [])
    }

    private func paintedItem(_ p: Scene.Point, _ pl: Scene.Placed, _ path: [Int]) -> [Int]? {
        for i in pl.kids.indices.reversed() {
            let k = pl.kids[i]
            guard k.x <= p.x, p.x <= k.x + k.w, k.y <= p.y, p.y <= k.y + k.h else { continue }
            let kp = path + [i]
            if let hit = paintedItem(p, k, kp) { return hit }
            if paintsItself(k) { return kp }
        }
        return nil
    }

    private func boxedItem(_ p: Scene.Point, _ pl: Scene.Placed, _ path: [Int]) -> [Int]? {
        for i in pl.kids.indices.reversed() {
            let k = pl.kids[i]
            guard k.x <= p.x, p.x <= k.x + k.w, k.y <= p.y, p.y <= k.y + k.h else { continue }
            return boxedItem(p, k, path + [i]) ?? (path + [i])
        }
        return nil
    }

    /// Whether an item draws something of its own: a fill, a border, an icon, an image, a title or a shadow.
    private func paintsItself(_ pl: Scene.Placed) -> Bool {
        for name in ["color", "border-color", "path", "image", "title", "shadow"] where pl.prop(name) != nil {
            return true
        }
        return false
    }

    // MARK: - Inline title

    /// Opens the field over the selected item, where go/app/editing.go onEdit puts it: over the box the
    /// canvas draws for the item, not a text rect of its own.
    func beginTitleEdit() {
        // Only an item has a title of its own; a selected use is the whole instance, edited in its file.
        guard editField == nil, let path = selection, let pl = placed(at: path), let src = pl.src,
            src.kind == .item, pl.w > 0, pl.h > 0
        else { return }
        let ti = screen?.p.texts[ObjectIdentifier(pl.r)]
        editFontSize = ti?.font.size ?? Scene.defaultFontSize
        let field = NSTextField(frame: .zero)
        field.stringValue = ti?.text ?? ""  // the drawn text; an edit of a title expression flattens it
        field.isBordered = true
        field.drawsBackground = true
        field.backgroundColor = .textBackgroundColor
        field.focusRingType = .none
        field.delegate = self
        addSubview(field)
        editing = path
        editTitle = field.stringValue
        editField = field
        placeEditor()
        window?.makeFirstResponder(field)
        field.currentEditor()?.selectAll(nil)
    }

    /// Closes the field: the typed title goes through the document, or is dropped.
    func endTitleEdit(commit: Bool) {
        guard let field = editField, let path = editing else { return }
        let value = field.stringValue
        let opened = editTitle
        editField = nil
        editing = nil
        editTitle = ""
        if window?.firstResponder is NSTextView {
            window?.makeFirstResponder(self)  // the field editor goes with its field
        }
        field.delegate = nil
        field.removeFromSuperview()
        guard commit, value != opened else { return }
        writeTitle(value, path)
    }

    /// Moves the open field over its item after the canvas or the zoom changed.
    private func placeEditor() {
        guard let field = editField, let pl = placed(at: editing), pl.w > 0, pl.h > 0 else { return }
        field.frame = viewRect(pl)
        // the drawn size, floored so the text stays readable while typing
        field.font = NSFont.systemFont(ofSize: min(40, max(9, editFontSize * zoom)))
    }

    /// Writes the title of the item at path through the document; a refusal stays on the canvas. A part of
    /// a component is written as an at override in the use around it, so its copies stay as they are.
    private func writeTitle(_ value: String, _ path: [Int]) {
        guard let pl = placed(at: path), let node = pl.src, let tree else { return }
        if let part = partOf[ObjectIdentifier(pl)] {
            guard !part.at.isEmpty else {
                // the same banner go/app/sceneview_pointer.go puts on a part that no name addresses
                message = "Not applied: the item has no unique name inside the instance, edit it in the use"
                needsDisplay = true
                return
            }
            guard let source = tree.source(part.use.pos.file),
                let edit = atPropEdit(source, part.use, part.at, "title", value),
                apply(edit, in: part.use.pos.file)
            else { return }
            return
        }
        guard let source = tree.source(node.pos.file), let edit = setPropEdit(source, node, "title", value)
        else { return }
        apply(edit, in: node.pos.file)
    }

    /// Applies one text edit of the scene through the document. A refusal is shown over the rebuilt scene:
    /// a drag may have moved the drawn items, and the file did not take it.
    @discardableResult
    private func apply(_ edit: Scene.TextEdit, in file: String) -> Bool {
        guard let document else {
            message = "the canvas is not attached to a document"
            needsDisplay = true
            return false
        }
        guard let tree else { return false }
        do {
            try document.applySceneEdit(edit, in: file)
            if let next = document.sceneTree, next !== tree {
                show(next)  // a canvas with no window controller hears nothing of the reload
            }
            return true
        } catch {
            show(tree)
            message = error.localizedDescription
            needsDisplay = true
            return false
        }
    }

    // MARK: - Drag

    /// The item a press picked up, and the point of the screen it was picked at.
    private struct Drag {
        let pl: Scene.Placed
        let from: Scene.Point
        var dx = 0.0  // how far the pointer went, in the coordinates of the screen (y down)
        var dy = 0.0
        var moved = false
    }

    private var drag: Drag?

    /// The item a drag of pl moves: pl when its place comes from pos, else the nearest ancestor that has
    /// one. A click on a node inside an auto-layout moves the container it sits in, as go/app movable.
    private func movable(_ pl: Scene.Placed) -> Scene.Placed? {
        var n: Scene.Placed? = pl
        while let cur = n {
            if cur.free && selectable.contains(ObjectIdentifier(cur)) { return cur }
            n = cur.parent
        }
        return nil
    }

    private func holds(_ pl: Scene.Placed, _ p: Scene.Point) -> Bool {
        pl.x <= p.x && p.x <= pl.x + pl.w && pl.y <= p.y && p.y <= pl.y + pl.h
    }

    /// A press on the selected item picks it up, wherever on it the press lands; false when the press is
    /// no drag and the caller makes a click of it.
    @discardableResult
    func beginDrag(_ p: Scene.Point) -> Bool {
        endTitleEdit(commit: true)
        guard document != nil, let cur = placed(at: selection), holds(cur, p), let move = movable(cur) else {
            return false
        }
        drag = Drag(pl: move, from: p)
        return true
    }

    /// Moves the picked item with the pointer: the drawn tree follows, the document is written on release.
    func moveDrag(_ p: Scene.Point) {
        guard var d = drag else { return }
        let dx = p.x - d.from.x
        let dy = p.y - d.from.y
        d.moved = d.moved || abs(dx) + abs(dy) >= 1  // a click of a point or two is no move
        guard d.moved else { return }
        shift(d.pl, dx - d.dx, dy - d.dy)
        d.dx = dx
        d.dy = dy
        drag = d
        needsDisplay = true
    }

    /// Writes the place the drag reached; a drag that never left its point writes nothing.
    func endDrag() {
        guard let d = drag else { return }
        drag = nil
        guard d.moved else { return }
        writePos(d.pl, dx: d.dx, dy: d.dy)
    }

    /// Moves an item and its children in the drawn tree before the edit exists: Screen.MoveItem moves
    /// the same gfx nodes of go/scene.
    private func shift(_ pl: Scene.Placed, _ dx: Double, _ dy: Double) {
        pl.x += dx
        pl.y += dy
        for k in pl.kids {
            shift(k, dx, dy)
        }
    }

    /// Writes the pos of the item a drag moved by dx, dy of the screen: pos is y up, the canvas y down,
    /// as moveSceneNode of go/app. A part is an at override in its use.
    private func writePos(_ pl: Scene.Placed, dx: Double, dy: Double) {
        var x = 0.0
        var y = 0.0
        if let pos = pl.prop("pos") {
            let values = pos.args.map { $0.getIdentText() ?? "" }
            if values.count == 2 {
                x = Double(values[0]) ?? 0
                y = Double(values[1]) ?? 0
            }
        }
        let nx = (x + dx).rounded()
        let ny = (y - dy).rounded()
        guard let tree else { return }
        if let part = partOf[ObjectIdentifier(pl)] {
            guard !part.at.isEmpty else {
                message = "Not applied: the item has no unique name inside the instance, edit it in the use"
                needsDisplay = true
                return
            }
            guard let source = tree.source(part.use.pos.file),
                let edit = atPropEdit(source, part.use, part.at, "pos", numText(nx), numText(ny))
            else { return }
            apply(edit, in: part.use.pos.file)
            return
        }
        guard let node = pl.src, let source = tree.source(node.pos.file),
            let edit = setPropEdit(source, node, "pos", numText(nx), numText(ny))
        else { return }
        apply(edit, in: node.pos.file)
    }

    // MARK: - Input

    override func mouseDown(with event: NSEvent) {
        let p = scenePoint(convert(event.locationInWindow, from: nil))
        if event.clickCount < 2, beginDrag(p) { return }
        click(p, clicks: event.clickCount)
    }

    override func mouseDragged(with event: NSEvent) {
        moveDrag(scenePoint(convert(event.locationInWindow, from: nil)))
    }

    override func mouseUp(with event: NSEvent) {
        endDrag()
    }

    /// A click at a point of the screen: selects what it lands on and, on the second click, edits the title.
    func click(_ p: Scene.Point, clicks: Int) {
        endTitleEdit(commit: true)
        guard let hit = hitItem(p) else {
            select(nil)
            return
        }
        select(pick(hit, clicks: clicks))
        if clicks >= 2 {
            beginTitleEdit()
        }
    }

    func control(_ control: NSControl, textView: NSTextView, doCommandBy commandSelector: Selector) -> Bool {
        switch commandSelector {
        case #selector(NSResponder.insertNewline(_:)):
            endTitleEdit(commit: true)
            return true
        case #selector(NSResponder.cancelOperation(_:)):
            endTitleEdit(commit: false)
            return true
        default:
            return false
        }
    }

    func controlTextDidEndEditing(_ obj: Notification) {
        endTitleEdit(commit: true)  // clicked away: keep what was typed
    }

    // MARK: - Geometry

    /// The origin of the scene content in the view: the padding and the centering at the zoom.
    private var contentOrigin: NSPoint {
        guard let screen else { return .zero }
        let content = SceneCanvasView.contentSize(screen)
        return NSPoint(
            x: (bounds.width - content.width * zoom) / 2 + SceneCanvasView.padding * zoom,
            y: (bounds.height - content.height * zoom) / 2 + SceneCanvasView.padding * zoom)
    }

    /// Where a point of the screen lands in the view.
    func viewPoint(_ p: Scene.Point) -> NSPoint {
        let o = contentOrigin
        return NSPoint(x: o.x + p.x * zoom, y: o.y + p.y * zoom)
    }

    /// The point of the screen under a point of the view.
    func scenePoint(_ v: NSPoint) -> Scene.Point {
        let o = contentOrigin
        guard zoom > 0 else { return Scene.Point() }
        return Scene.Point(x: (v.x - o.x) / zoom, y: (v.y - o.y) / zoom)
    }

    /// The box of an item in the view, grown by grow points for the selection stroke.
    private func viewRect(_ pl: Scene.Placed, grow: Double = 0) -> NSRect {
        let o = viewPoint(Scene.Point(x: pl.x, y: pl.y))
        return NSRect(x: o.x - grow, y: o.y - grow, width: pl.w * zoom + 2 * grow, height: pl.h * zoom + 2 * grow)
    }

    override func setFrameSize(_ newSize: NSSize) {
        super.setFrameSize(newSize)
        placeEditor()
    }

    // MARK: - Drawing

    override func draw(_ dirtyRect: NSRect) {
        guard let c = NSGraphicsContext.current?.cgContext else { return }
        c.setFillColor(PreferenceConstants.preference.background)
        c.fill(bounds)
        if let painter, let placed {
            let o = contentOrigin
            c.saveGState()
            c.translateBy(x: o.x, y: o.y)
            c.scaleBy(x: zoom, y: zoom)
            painter.paint(placed, c)
            c.restoreGState()
        }
        if let message {
            NSAttributedString(
                string: message,
                attributes: [
                    .font: NSFont.systemFont(ofSize: 13),
                    .foregroundColor: NSColor.systemRed,
                ]
            ).draw(at: NSPoint(x: SceneCanvasView.padding, y: SceneCanvasView.padding))
        }
        if let pl = placed(at: selection) {
            NSColor.systemBlue.setStroke()
            let path = NSBezierPath(roundedRect: viewRect(pl, grow: 2), xRadius: 3, yRadius: 3)
            path.lineWidth = 1.5
            path.stroke()
        }
    }
}

/// Holds the canvas in a scroll view: the screen is fitted into it, and a screen too large for the
/// zoom floor scrolls.
final class SceneCanvasScrollView: NSScrollView {
    let canvas = SceneCanvasView(frame: .zero)

    init() {
        super.init(frame: .zero)
        drawsBackground = false
        borderType = .noBorder
        hasVerticalScroller = true
        hasHorizontalScroller = true
        documentView = canvas
    }

    required init?(coder: NSCoder) {
        fatalError("init(coder:) has not been implemented")
    }

    func show(_ tree: Scene.Tree?) {
        canvas.show(tree)
        fit()
    }

    override func layout() {
        super.layout()
        fit()
    }

    /// Sizes the document view so the whole screen fits, and scrolls when it cannot.
    private func fit() {
        let clip = contentView.bounds.size
        guard let screen = canvas.screen else {
            canvas.zoom = 1
            canvas.frame = NSRect(origin: .zero, size: clip)
            return
        }
        let content = SceneCanvasView.contentSize(screen)
        let zoom = SceneCanvasView.fitZoom(content.width, content.height, clip.width, clip.height)
        let size = NSSize(
            width: max(content.width * zoom, clip.width), height: max(content.height * zoom, clip.height))
        if canvas.zoom != zoom {
            canvas.zoom = zoom
        }
        if canvas.frame.size != size {
            canvas.frame = NSRect(origin: .zero, size: size)
        }
    }
}
