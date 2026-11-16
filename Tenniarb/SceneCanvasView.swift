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
            needsDisplay = true
            return
        }
        self.tree = tree
        let el = tree.main?.elems.first { $0.kind == .element }
        let built: Scene.Screen?
        if let el {
            built = tree.buildElement(el, measurer: Scene.TextMeasurer(), eval: sceneEvalVars)
        } else {
            built = tree.build("", measurer: Scene.TextMeasurer(), eval: sceneEvalVars)
        }
        screen = built
        placed = built?.placed()
        painter = built.map(ScenePainter.init)
        message =
            built == nil
            ? tree.diagnostics.first { !$0.warn }.map { "\($0.pos): \($0.message)" } ?? "the scene does not build"
            : nil
        if placed(at: selection) == nil {
            selection = nil
        }
        needsDisplay = true
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
        // Only an item has a title of its own; a use or a link is edited through at overrides, absent here.
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

    /// Writes the title of the item at path through the document; a refusal stays on the canvas.
    private func writeTitle(_ value: String, _ path: [Int]) {
        // shortcut: a part of a use or of a mounted library is refused rather than edited through
        // atPropEdit (go/app/sceneedit.go) or the library window (go/app/mountdoc.go), neither ported yet.
        guard let document else {
            message = "the canvas is not attached to a document"
            needsDisplay = true
            return
        }
        guard let pl = placed(at: path), let node = pl.src, let tree,
            let source = tree.source(node.pos.file), let edit = setPropEdit(source, node, "title", value)
        else { return }
        do {
            try document.applySceneEdit(edit, in: node.pos.file)
            if let next = document.sceneTree, next !== tree {
                show(next)  // a canvas with no window controller hears nothing of the reload
            }
        } catch {
            message = error.localizedDescription
            needsDisplay = true
        }
    }

    // MARK: - Input

    override func mouseDown(with event: NSEvent) {
        endTitleEdit(commit: true)
        let p = scenePoint(convert(event.locationInWindow, from: nil))
        guard let hit = hitItem(p) else {
            select(nil)
            return
        }
        select(hit)
        if event.clickCount >= 2 {
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
