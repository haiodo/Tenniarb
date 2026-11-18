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

extension NSPasteboard.PasteboardType {
    /// The drag of a component row of the Items pane; the path of the use is the string it carries.
    static let tenniarbComponent = NSPasteboard.PasteboardType("tenniarb.component")
    /// The label of that row, for a receiver that shows what is dragged.
    static let tenniarbComponentName = NSPasteboard.PasteboardType("tenniarb.component.name")
}

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
    /// The painter of the shown screen, for the frames the playback sets.
    private(set) var painter: ScenePainter?
    private(set) var message: String?
    /// The scale the content is drawn at; the scroll view fits the screen into itself.
    var zoom = 1.0 {
        didSet { needsDisplay = true }
    }

    /// The document an edit is written through; nil on a canvas nobody can edit.
    weak var document: Document?

    /// The indexes from the root of the drawn tree to the selected item; nil when nothing is selected.
    private(set) var selection: [Int]?

    /// Called after the selection or the tree behind it changed, so the inspector reads the new one.
    var onSelectionChange: (() -> Void)?

    /// A frame compare mode draws over a changed node: the path the comparison gives it, and its colour.
    struct CompareFrame {
        let path: String
        let color: NSColor
    }

    /// The frames of the changed nodes; only those on the shown element are drawn.
    private(set) var compareFrames: [CompareFrame] = []
    /// The line below the message: the summary of the comparison while it is on.
    private(set) var compareNote: String?

    /// The pins of the comments of the shown element, in the scene coordinates of the element.
    private(set) var commentPins: [CommentPin] = []
    /// Why Add Comment… cannot write here; the menu shows it instead of the field.
    var commentUnavailable: String?
    /// The scene point the context menu was opened at, for the comment it adds.
    private var commentPoint: Scene.Point?
    /// Called with the id of the comment whose pin a click lands on.
    var onPinClick: ((String) -> Void)?
    /// Called with the text of a new comment and the anchor of the place it was asked at.
    var onAddComment: ((String, Comments.Anchor) -> Void)?

    /// The banner of the comment mode, as go/app comments_pins.go pinsPicture writes it.
    static let commentHint = "Comment mode: click to place a comment, Esc to leave"
    /// Whether a plain click places a comment instead of selecting; C toggles it and Esc leaves.
    private(set) var commentMode = false
    /// The cursor the canvas showed before the comment mode, put back when it leaves.
    private var cursorBeforeCommentMode: NSCursor?
    /// Asks for the text of a new comment; nil shows the sheet over the window. A test answers instead.
    var commentPrompt: ((@escaping (String) -> Void) -> Void)?

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

    // MARK: - Built screens

    /// How many screens of other elements stay off the canvas for a quick switch back, as sceneCacheSize.
    static let cacheSize = 4

    /// A screen taken off the canvas: built for node (nil, the file root) out of tree, held weakly so a
    /// deallocated tree can never be matched by a new one at the same address.
    private struct Cached {
        weak var tree: Scene.Tree?
        let node: Scene.Node?
        let screen: Scene.Screen
        let placed: Scene.Placed
        let painter: ScenePainter
    }

    /// The screens shown before, newest first. The shown one is not here, as a Go view is detached from the
    /// canvas and put back; the entries die with their tree.
    private var cache: [Cached] = []
    /// How many screens the last show left cached; the tests read it.
    var cachedScreenCount: Int { cache.count }
    /// A drag shifted the drawn tree without writing the document: that screen must not be cached.
    private var movedByDrag = false

    /// Takes the shown screen off the canvas into the cache, as go/app cacheScene; the oldest entry goes
    /// when the fifth one arrives.
    private func cacheShown() {
        let moved = movedByDrag
        movedByDrag = false
        guard !moved, let tree, let screen, let placed, let painter else { return }
        cache.insert(Cached(tree: tree, node: element, screen: screen, placed: placed, painter: painter), at: 0)
        if cache.count > SceneCanvasView.cacheSize {
            cache.removeLast()
        }
    }

    /// The screen built for node out of tree, taken out of the cache; nil builds it anew.
    private func take(_ tree: Scene.Tree, _ node: Scene.Node?) -> Cached? {
        guard let i = cache.firstIndex(where: { $0.tree === tree && $0.node === node }) else { return nil }
        return cache.remove(at: i)
    }

    /// Shows the first element of tree, as the Go window opens a document; nil clears the canvas.
    func show(_ tree: Scene.Tree?) {
        endTitleEdit(commit: false)
        setCommentMode(false)  // another screen starts without the mode
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
            indexAnimations()
            needsDisplay = true
            onSelectionChange?()
            return
        }
        cacheShown()
        self.tree = tree
        cache.removeAll { $0.tree !== tree }  // an edit loads a new tree: the screens of the old one are gone
        let el = shownElement(tree) ?? tree.main?.elems.first { $0.kind == .element }
        if let hit = take(tree, el ?? tree.main) {
            element = hit.node
            screen = hit.screen
            placed = hit.placed
            painter = hit.painter
            message = nil
            drag = nil
        } else {
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
        }
        indexSelection()
        if placed(at: selection) == nil {
            selection = nil
        }
        indexAnimations()
        needsDisplay = true
        onSelectionChange?()
    }

    /// Shows another element of the same tree, as a click in the outline does.
    func showElement(_ path: [Int]?) {
        elementPath = path
        show(tree)
    }

    // MARK: - Animations

    /// The frame the playback aims at, as Go's 16 ms.
    static let animFrame = 0.016

    /// The animations of the drawn items, in paint order, and the clock that plays them.
    private var animated: [(id: ObjectIdentifier, anim: Scene.Animation)] = []
    /// The frame timer of the playback. A selector timer keeps its target, so the view stops it with its window;
    /// deinit invalidates it too, hence a nonisolated read of a non-Sendable class.
    nonisolated(unsafe) private var timer: Timer?
    /// When the playback started, for the moment a frame shows.
    private var animFrom = Date()
    /// Whether the animations play; the Play switch of the inspector pauses them.
    private(set) var animationsPlaying = true
    /// The frame the painter draws every animated item with, kept for the tests as well.
    private(set) var animationFrames: [ObjectIdentifier: (rotate: Double, scale: Double, opacity: Double)] = [:]

    /// Whether the shown screen plays any animation, for the inspector.
    var hasAnimations: Bool { !animated.isEmpty }

    /// Plays one frame at now seconds from the start of the playback; a test calls it with a fixed moment
    /// instead of waiting for the timer.
    func tickAnimations(_ now: Double) {
        guard animationsPlaying, !animated.isEmpty else { return }
        applyAnimations { _ in now }
    }

    /// Plays or pauses the animations; a paused canvas keeps the frame the scrub left.
    func setAnimationsPlaying(_ on: Bool) {
        animationsPlaying = on
        guard on else {
            stopTimer()
            return
        }
        animFrom = Date()  // a resume starts over, as the Play switch of the inspector does
        startTimer()
    }

    /// Stops the playback and shows every animation at frac (0..1) of its duration, as the Frame slider does.
    func scrubAnimations(_ frac: Double) {
        animationsPlaying = false
        stopTimer()
        guard !animated.isEmpty else { return }
        applyAnimations { frac * $0 }
    }

    /// Draws every animated item at a moment of its own animation, given its duration.
    private func applyAnimations(_ at: (Double) -> Double) {
        guard let painter else { return }
        var frames: [ObjectIdentifier: (rotate: Double, scale: Double, opacity: Double)] = [:]
        frames.reserveCapacity(animated.count)
        for a in animated {
            frames[a.id] = a.anim.values(at: at(a.anim.duration), loop: true)
        }
        animationFrames = frames
        painter.frames = frames
        needsDisplay = true
    }

    /// Finds the animations of the drawn items and starts their clock; a screen without them draws no frame.
    private func indexAnimations() {
        stopTimer()
        animated = []
        animationFrames = [:]
        painter?.frames = [:]
        animationsPlaying = true
        guard let root = placed, let screen else { return }
        func walk(_ pl: Scene.Placed) {
            if let a = screen.animation(of: pl) {
                animated.append((ObjectIdentifier(pl.r), a))
            }
            for k in pl.kids {
                walk(k)
            }
        }
        walk(root)
        animFrom = Date()
        startTimer()
    }

    /// Starts the frame timer, unless the playback is paused, the screen has no animation or the canvas is
    /// not in a window (a test).
    private func startTimer() {
        stopTimer()
        guard animationsPlaying, !animated.isEmpty, window != nil else { return }
        let t = Timer(
            timeInterval: SceneCanvasView.animFrame, target: self, selector: #selector(animTick), userInfo: nil,
            repeats: true)
        RunLoop.main.add(t, forMode: .common)
        timer = t
    }

    private func stopTimer() {
        timer?.invalidate()
        timer = nil
    }

    /// Plays one frame of the timer.
    @objc private func animTick() {
        guard window != nil else {  // the window went away without a close notice
            stopTimer()
            return
        }
        tickAnimations(Date().timeIntervalSince(animFrom))
    }

    @objc private func windowWillClose(_ note: Notification) {
        stopTimer()
    }

    override func viewDidMoveToWindow() {
        super.viewDidMoveToWindow()
        registerForDraggedTypes([.tenniarbComponent])
        NotificationCenter.default.removeObserver(self, name: NSWindow.willCloseNotification, object: nil)
        guard let window else {
            stopTimer()
            return
        }
        NotificationCenter.default.addObserver(
            self, selector: #selector(windowWillClose(_:)), name: NSWindow.willCloseNotification, object: window)
        animFrom = Date()
        startTimer()
    }

    deinit {
        timer?.invalidate()
        NotificationCenter.default.removeObserver(self)
    }

    /// The node the canvas draws the screen of: the element, or the file root.
    var screenNode: Scene.Node? { element }

    /// The written node of the selected item, or the use of a selected instance.
    var selectedNode: Scene.Node? { placed(at: selection)?.src }

    /// The tree the canvas draws; nil when it shows nothing.
    var shownTree: Scene.Tree? { tree }

    /// Where the selected item is a part of an instance: the use around it and its path in that use.
    func part(at path: [Int]?) -> Part? {
        guard let pl = placed(at: path) else { return nil }
        return partOf[ObjectIdentifier(pl)]
    }

    /// A place right of the content of the screen for a new top level node, as pos x y.
    func freeSpot() -> (x: Double, y: Double) {
        guard let root = placed, !root.kids.isEmpty else { return (0, 0) }
        var maxX = -Double.infinity
        var minY = Double.infinity
        for k in root.kids {
            maxX = max(maxX, k.x + k.w)
            minY = min(minY, k.y)
        }
        return ((maxX + 100).rounded(), (-minY).rounded())
    }

    /// Selects the drawn node written at a byte offset of the file, after an insert put it there.
    func selectSource(at offset: Int) {
        guard let root = placed else { return }
        var found: [Int]?
        func walk(_ pl: Scene.Placed, _ path: [Int]) {
            if found != nil {
                return
            }
            for (i, k) in pl.kids.enumerated() {
                if k.src?.range.start == offset {
                    found = path + [i]
                    return
                }
                walk(k, path + [i])
            }
        }
        walk(root, [])
        select(found)
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

    /// Shows a line of text over the canvas, as a refused edit does; nil clears it, leaving the banner of
    /// the comment mode when it is on.
    func setMessage(_ text: String?) {
        message = text ?? (commentMode ? SceneCanvasView.commentHint : nil)
        needsDisplay = true
    }

    /// Puts the frames of compare mode over the picture; an empty list takes them away.
    func setCompareFrames(_ frames: [CompareFrame]) {
        compareFrames = frames
        needsDisplay = true
    }

    /// Shows the summary line of compare mode under the message; nil clears it.
    func setCompareNote(_ text: String?) {
        guard text != compareNote else { return }
        compareNote = text
        needsDisplay = true
    }

    /// Enters or leaves the mode where a plain click places a comment, as go/app setCommentMode.
    func setCommentMode(_ on: Bool) {
        guard on != commentMode else { return }
        if on, let reason = commentUnavailable {
            setMessage(reason)
            return
        }
        commentMode = on
        if on {
            cursorBeforeCommentMode = NSCursor.current
            NSCursor.crosshair.set()
        } else {
            (cursorBeforeCommentMode ?? .arrow).set()
            cursorBeforeCommentMode = nil
        }
        setMessage(on ? SceneCanvasView.commentHint : nil)
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
        onSelectionChange?()
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
        field.cell = InlineTitleCell(textCell: "")
        field.stringValue = ti?.text ?? ""  // the drawn text; an edit of a title expression flattens it
        field.usesSingleLineMode = true
        // the title of a leaf is centred unless the file asks for another layout
        switch ti?.align ?? .middle {
        case .left: field.alignment = .left
        case .right: field.alignment = .right
        case .middle: field.alignment = .center
        }
        styleField(field)
        field.isEditable = true
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
        field.font = NSFont.systemFont(ofSize: editFontSize * zoom)
        field.layer?.cornerRadius = inlineRadius(pl)
    }

    /// Gives the field the look of inlineOpts: a rounded fill on the layer, so the title drawn behind it
    /// does not show through, and a thin grey border instead of the native bezel.
    private func styleField(_ field: NSTextField) {
        let fill = inlineFill
        field.isBordered = false
        field.isBezeled = false
        field.drawsBackground = true
        field.backgroundColor = fill
        field.focusRingType = .none
        field.wantsLayer = true
        field.layer?.backgroundColor = fill.cgColor
        field.layer?.borderColor =
            NSColor(
                srgbRed: 128.0 / 255, green: 128.0 / 255, blue: 128.0 / 255, alpha: 64.0 / 255
            ).cgColor
        field.layer?.borderWidth = 1
        field.layer?.masksToBounds = true
    }

    /// The fill of the field as inlineOpts: white in light, #1C1C1E in dark.
    private var inlineFill: NSColor {
        effectiveAppearance.bestMatch(from: [.darkAqua, .aqua]) == .darkAqua
            ? NSColor(srgbRed: 28.0 / 255, green: 28.0 / 255, blue: 30.0 / 255, alpha: 1)
            : .white
    }

    /// The corner radius of inlineOpts: the item's corner-radius at this zoom, no more than half the
    /// field, and two points at least so a short box still reads as rounded.
    private func inlineRadius(_ pl: Scene.Placed) -> CGFloat {
        let written = pl.prop("corner-radius")?.num(0) ?? 0
        let radius = ((written > 0 ? written : 8) * zoom).rounded()
        let half = (pl.h * zoom).rounded() / 2
        return max(2, min(radius, half.rounded(.down)))
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
    /// Links take the box of an end from its frame, so that moves too; the kids keep frames relative to it.
    func moveDrag(_ p: Scene.Point) {
        guard var d = drag else { return }
        let dx = p.x - d.from.x
        let dy = p.y - d.from.y
        d.moved = d.moved || abs(dx) + abs(dy) >= 1  // a click of a point or two is no move
        guard d.moved else { return }
        let stepX = dx - d.dx
        let stepY = dy - d.dy
        shift(d.pl, stepX, stepY)
        screen?.p.frames[ObjectIdentifier(d.pl.r)]?.shift(stepX, stepY)
        d.dx = dx
        d.dy = dy
        drag = d
        movedByDrag = true
        needsDisplay = true
    }

    /// Writes the place the drag reached; a drag that never left its point writes nothing.
    func endDrag() {
        guard let d = drag else { return }
        drag = nil
        guard d.moved else { return }
        writePos(d.pl, dx: d.dx, dy: d.dy)
        onSelectionChange?()
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

    /// The text of the item at path as HTML for the clipboard, as go/scene ItemHTML builds it; nil when the
    /// canvas draws no text for it.
    func itemHTML(at path: [Int]? = nil) -> String? {
        guard let screen, let pl = placed(at: path ?? selection) else { return nil }
        let html = screen.itemHTML(pl)
        return html.isEmpty ? nil : html
    }

    /// "Export text as html": the text of the selected item on the clipboard as HTML.
    @objc func copyTextAsHTML(_ sender: Any?) {
        guard let html = itemHTML(at: nil) else { return }
        ClipboardUtils.copyHtml(html)
    }

    /// The place of the selection a link names: the shown element and the selected node, as go/app's
    /// copyLink; the node is empty when nothing is selected.
    var linkPlace: (element: [String], node: String) {
        (
            screenNode.map { CommentPaths.elementPath(of: $0) } ?? [],
            selectedNode.map { CommentPaths.nodePath(of: $0, from: screenNode) } ?? ""
        )
    }

    /// The remotes of the clones a copy asks for: git runs once per root, as an open asks via LinkOpen.
    private static let linkRepos = Links.Repos()

    /// Puts the link to a place of document on the clipboard, as go/app's copyLink: say gets why it is not
    /// copied, or the note about where the link works.
    static func copyLink(document: Document?, element: [String], node: String, say: (String) -> Void) {
        guard let path = document?.fileURL?.path else {
            say("Save the file to copy a link to it.")
            return
        }
        do {
            let (link, note) = try linkRepos.of(
                file: path, home: FileManager.default.homeDirectoryForCurrentUser.path,
                element: element, node: node)
            NSPasteboard.general.clearContents()
            NSPasteboard.general.setString(link.url(), forType: .string)
            if !note.isEmpty { say(note) }
        } catch {
            say(error.localizedDescription)
        }
    }

    /// Copy Link of the menu: the shown element and the selected node.
    @objc private func copyLinkFromMenu(_ sender: Any?) {
        let place = linkPlace
        Self.copyLink(document: document, element: place.element, node: place.node) { [weak self] in
            self?.setMessage($0)
        }
    }

    /// The canvas menu: the export of the text of the selected item, as go/app gives a classic diagram,
    /// a comment at the place clicked and the link to the selection.
    override func menu(for event: NSEvent) -> NSMenu? {
        commentPoint = scenePoint(convert(event.locationInWindow, from: nil))
        let menu = NSMenu()
        menu.autoenablesItems = false
        let export = NSMenuItem(title: "Export text as html", action: #selector(copyTextAsHTML(_:)), keyEquivalent: "")
        export.target = self
        export.isEnabled = itemHTML(at: nil) != nil
        menu.addItem(export)
        menu.addItem(.separator())
        let comment = NSMenuItem(title: "Add Comment…", action: #selector(commentFromMenu(_:)), keyEquivalent: "")
        comment.target = self
        menu.addItem(comment)
        menu.addItem(.separator())
        let link = NSMenuItem(title: "Copy Link", action: #selector(copyLinkFromMenu(_:)), keyEquivalent: "")
        link.target = self
        menu.addItem(link)
        return menu
    }

    /// The anchor of a click at p, as Go's AnchorAt: the written item under it and the point from its box,
    /// or the element origin when the click lands on nothing written.
    func anchor(at p: Scene.Point) -> Comments.Anchor {
        let element = screenNode.map { CommentPaths.elementPath(of: $0) } ?? []
        guard let hit = hitItem(p), let pl = placed(at: hit), let src = pl.src else {
            return Comments.Anchor(element: element, node: "", point: Comments.Point(x: p.x, y: p.y))
        }
        let node = CommentPaths.nodePath(of: src, from: screenNode)
        guard !node.isEmpty else {
            return Comments.Anchor(element: element, node: "", point: Comments.Point(x: p.x, y: p.y))
        }
        return Comments.Anchor(
            element: element, node: node, point: Comments.Point(x: p.x - pl.x, y: p.y - pl.y))
    }

    /// Writes a comment at a point of the screen through the window controller's callback.
    func addComment(text: String, at p: Scene.Point) {
        let text = text.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty else { return }
        onAddComment?(text, anchor(at: p))
    }

    /// Puts the pins of the comments of the shown element over the picture.
    func setCommentPins(_ pins: [CommentPin]) {
        commentPins = pins
        needsDisplay = true
    }

    /// The centre of a pin marker in the scene: above-right of its anchor, so a pin on a corner does not
    /// hide the node (go/app comments.go pinView).
    func pinCenter(_ pin: CommentPin) -> Scene.Point {
        let r = CommentPin.radius / CGFloat(max(zoom, 0.0001))
        return Scene.Point(x: Double(pin.point.x + r * 0.6), y: Double(pin.point.y - r * 0.6))
    }

    /// The pin a click at p lands on: the marker is a constant size in view points, the last drawn wins.
    func pin(at p: Scene.Point) -> CommentPin? {
        let q = viewPoint(p)
        for pin in commentPins.reversed() {
            let c = viewPoint(pinCenter(pin))
            let dx = q.x - c.x
            let dy = q.y - c.y
            if dx * dx + dy * dy <= CommentPin.radius * CommentPin.radius { return pin }
        }
        return nil
    }

    /// Add Comment… of the menu: explains a document without a file, else asks for the text at the click.
    @objc private func commentFromMenu(_ sender: Any?) {
        if let reason = commentUnavailable {
            setMessage(reason)
            return
        }
        guard let p = commentPoint else { return }
        askComment(at: p)
    }

    /// Asks for the text of a comment at p, then writes it through onAddComment.
    private func askComment(at p: Scene.Point) {
        if let commentPrompt {
            commentPrompt { [weak self] text in self?.addComment(text: text, at: p) }
            return
        }
        guard let window else { return }
        let alert = NSAlert()
        alert.messageText = "Add Comment"
        let field = NSTextField(frame: NSRect(x: 0, y: 0, width: 300, height: 24))
        field.placeholderString = "Comment"
        alert.accessoryView = field
        alert.addButton(withTitle: "Post")
        alert.addButton(withTitle: "Cancel")
        alert.window.initialFirstResponder = field
        alert.beginSheetModal(for: window) { [weak self] response in
            MainActor.assumeIsolated {
                guard response == .alertFirstButtonReturn else { return }
                self?.addComment(text: field.stringValue, at: p)
            }
        }
    }

    override func mouseDown(with event: NSEvent) {
        window?.makeFirstResponder(self)  // the C key reaches the canvas
        let p = scenePoint(convert(event.locationInWindow, from: nil))
        if let pin = pin(at: p) {
            onPinClick?(pin.id)
            return
        }
        if commentMode, event.modifierFlags.intersection(.deviceIndependentFlagsMask).isEmpty {
            askComment(at: p)  // no selection and no drag: the click only places a comment
            return
        }
        if event.clickCount < 2, beginDrag(p) { return }
        click(p, clicks: event.clickCount)
    }

    override func mouseDragged(with event: NSEvent) {
        moveDrag(scenePoint(convert(event.locationInWindow, from: nil)))
    }

    override func mouseUp(with event: NSEvent) {
        endDrag()
    }

    /// C toggles the comment mode and Esc leaves it, as go/app commentKey; the title field keeps its keys.
    override func keyDown(with event: NSEvent) {
        if commentMode, event.keyCode == 53 {
            setCommentMode(false)
            return
        }
        let mods = event.modifierFlags.intersection(.deviceIndependentFlagsMask)
        if editField == nil, mods.isDisjoint(with: [.command, .control, .option]),
            event.charactersIgnoringModifiers?.lowercased() == "c"
        {
            setCommentMode(!commentMode)
            return
        }
        super.keyDown(with: event)
    }

    // MARK: - Dropping a component

    /// The use path a component drag carries, when the document still offers it; nil for any other drag,
    /// which the canvas ignores like a plain text drop.
    private func draggedComponent(_ sender: NSDraggingInfo) -> String? {
        guard let path = sender.draggingPasteboard.string(forType: .tenniarbComponent),
            let document, document.components("", limit: 200).contains(where: { $0.path == path })
        else { return nil }
        return path
    }

    override func draggingEntered(_ sender: NSDraggingInfo) -> NSDragOperation {
        draggedComponent(sender) == nil ? [] : .copy
    }

    override func draggingUpdated(_ sender: NSDraggingInfo) -> NSDragOperation {
        draggingEntered(sender)
    }

    override func prepareForDragOperation(_ sender: NSDraggingInfo) -> Bool {
        draggedComponent(sender) != nil
    }

    override func performDragOperation(_ sender: NSDraggingInfo) -> Bool {
        guard let path = draggedComponent(sender) else { return false }
        let point = scenePoint(convert(sender.draggingLocation, from: nil))
        return document?.vc?.dropComponent(path: path, at: point) ?? false
    }

    /// A click at a point of the screen: selects what it lands on and, on the second click, edits the title.
    func click(_ p: Scene.Point, clicks: Int) {
        endTitleEdit(commit: true)
        guard let hit = hitItem(p) else {
            select(nil)
            onSelectionChange?()
            return
        }
        select(pick(hit, clicks: clicks))
        onSelectionChange?()
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
        drawCompareFrames()
        var textY = SceneCanvasView.padding
        for (text, color) in [(message, NSColor.systemRed), (compareNote, NSColor.secondaryLabelColor)] {
            guard let text else { continue }
            NSAttributedString(
                string: text,
                attributes: [
                    .font: NSFont.systemFont(ofSize: 13),
                    .foregroundColor: color,
                ]
            ).draw(at: NSPoint(x: SceneCanvasView.padding, y: textY))
            textY += 18
        }
        if let pl = placed(at: selection) {
            NSColor.systemBlue.setStroke()
            let path = NSBezierPath(roundedRect: viewRect(pl, grow: 2), xRadius: 3, yRadius: 3)
            path.lineWidth = 1.5
            path.stroke()
        }
        drawPins()
    }

    /// Draws the frames of compare mode over the nodes written in the shown element: the report names a
    /// node from the file root, the placed tree names it from the element down.
    private func drawCompareFrames() {
        guard !compareFrames.isEmpty, let root = placed else { return }
        let prefix = elementNames()
        var boxes: [String: Scene.Placed] = [:]
        func walk(_ pl: Scene.Placed) {
            for k in pl.kids {
                if let src = k.src, let segs = sourcePath(src), !segs.isEmpty {
                    let key = segs.joined(separator: ".")
                    if boxes[key] == nil {
                        boxes[key] = k
                    }
                }
                walk(k)
            }
        }
        walk(root)
        for frame in compareFrames {
            let sel: String
            if prefix.isEmpty {
                sel = frame.path
            } else if frame.path.hasPrefix(prefix + ".") {
                sel = String(frame.path.dropFirst(prefix.count + 1))
            } else {
                continue
            }
            guard let pl = boxes[sel] else { continue }
            frame.color.setStroke()
            let path = NSBezierPath(rect: viewRect(pl))
            path.lineWidth = 2
            path.stroke()
        }
    }

    /// The names from the file root to the shown element, joined by "/", as the comparison writes them:
    /// a repeated name among sibling elements gets #n.
    private func elementNames() -> String {
        guard let element, element.kind != .file else { return "" }
        var segs: [String] = []
        var cur: Scene.Node? = element
        while let c = cur, c.kind != .file {
            segs.append(elementSeg(c))
            cur = c.parent
        }
        return segs.reversed().joined(separator: "/")
    }

    private func elementSeg(_ n: Scene.Node) -> String {
        var k = 0
        if let parent = n.parent {
            for c in parent.elems {
                if c === n {
                    break
                }
                if c.name == n.name {
                    k += 1
                }
            }
        }
        return k > 0 ? "\(n.name)#\(k)" : n.name
    }

    /// Draws the markers of the comments over the picture: an accent disc with the number, a ring when the
    /// anchor node is gone, faded when the thread is resolved.
    private func drawPins() {
        for pin in commentPins {
            let c = viewPoint(pinCenter(pin))
            let r = CommentPin.radius
            let disc = NSBezierPath(ovalIn: NSRect(x: c.x - r, y: c.y - r, width: 2 * r, height: 2 * r))
            let color = pin.resolved ? NSColor.controlAccentColor.withAlphaComponent(0.45) : NSColor.controlAccentColor
            let textColor: NSColor
            if pin.detached {
                color.setStroke()
                disc.lineWidth = 2
                disc.stroke()
                textColor = color
            } else {
                color.setFill()
                disc.fill()
                textColor = .white
            }
            let attributes: [NSAttributedString.Key: Any] = [
                .font: NSFont.boldSystemFont(ofSize: 11), .foregroundColor: textColor,
            ]
            let number = "\(pin.number)" as NSString
            let size = number.size(withAttributes: attributes)
            number.draw(at: NSPoint(x: c.x - size.width / 2, y: c.y - size.height / 2), withAttributes: attributes)
        }
    }
}

/// A field over a box taller than its line: the cell keeps the text on the middle line, its editor
/// with it, where a borderless field puts both at the top.
private final class InlineTitleCell: NSTextFieldCell {
    override func drawingRect(forBounds rect: NSRect) -> NSRect {
        var out = super.drawingRect(forBounds: rect)
        guard let font else { return out }
        let line = (font.ascender - font.descender + font.leading).rounded(.up)
        guard line > 0, line < out.height else { return out }
        out.origin.y += (out.height - line) / 2
        out.size.height = line
        return out
    }

    override func select(
        withFrame rect: NSRect, in controlView: NSView, editor textObj: NSText, delegate: Any?,
        start selStart: Int, length selLength: Int
    ) {
        super.select(
            withFrame: drawingRect(forBounds: rect), in: controlView, editor: textObj, delegate: delegate,
            start: selStart, length: selLength)
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
