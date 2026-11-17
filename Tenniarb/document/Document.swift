//
//  Document.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 26/05/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
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
import Tenn

class Document: NSDocument, IElementModelListener, NSWindowDelegate, @unchecked Sendable {
    @MainActor var store: ElementModelStore?

    @MainActor var vc: ViewController?

    /// The tree of a scene document, read in on open; nil for a classic one.
    @MainActor private(set) var sceneTree: Scene.Tree?

    /// The text a scene document holds now: every edit goes through it, and a save writes it.
    @MainActor private(set) var sceneText: String?

    /// The text the last read or save put on disk, so an undo back to it clears the edited mark.
    @MainActor private var savedSceneText: String?

    /// The canonical text of a classic document as the last read or save put it on disk.
    @MainActor private var savedText: String?

    /// The version of the file the last read or write left on disk; a different one there is another
    /// program's.
    @MainActor private var diskStamp: FileStamp?

    /// The versions of the files of the scene tree, the mounted ones included, when the tree was last
    /// read, kept or saved; a different one there is a library another program changed.
    @MainActor private var sceneStamps: [String: FileStamp?] = [:]

    /// What the last conversion from or to JSON Canvas left out; the window shows it.
    @MainActor private(set) var sceneCanvasNotes: [String] = []

    /// The workspace root the scene mounts resolve in, kept so an edit loads the same tree.
    @MainActor private var sceneRoot: URL?

    /// Where each row of the scene outline is: the index path of its element among the elements of the
    /// file the tree was loaded for.
    @MainActor private(set) var sceneElementPaths: [ObjectIdentifier: [Int]] = [:]
    /// The rows of the outline by path, for the canvas to find the row of the element it shows.
    @MainActor private var sceneElementsByPath: [[Int]: Element] = [:]

    /// The path of an outline row of a scene; nil for a classic document.
    @MainActor
    func sceneElementPath(of element: Element) -> [Int]? {
        sceneElementPaths[ObjectIdentifier(element)]
    }

    /// The row of the element at a path, as the canvas keeps it; nil when the path is gone.
    @MainActor
    func sceneElement(at path: [Int]) -> Element? {
        sceneElementsByPath[path]
    }

    /// The rows of a scene: one element per row, nested as the file writes them, each with the index path
    /// that tells the canvas which element to show.
    @MainActor
    private func sceneOutline(_ root: Scene.Node?) -> (rows: [Element], paths: [ObjectIdentifier: [Int]], byPath: [[Int]: Element]) {
        var paths: [ObjectIdentifier: [Int]] = [:]
        var byPath: [[Int]: Element] = [:]
        func rows(_ nodes: [Scene.Node], _ path: [Int]) -> [Element] {
            var out: [Element] = []
            for (i, n) in nodes.enumerated() where n.kind == .element {
                let row = Element(name: n.name)
                paths[ObjectIdentifier(row)] = path + [i]
                byPath[path + [i]] = row
                // The content of --from-file is written in that file: the row is the mount, not its screen.
                row.elements = n.mount == nil ? rows(n.elems, path + [i]) : []
                out.append(row)
            }
            return out
        }
        let rows = rows(root?.elems ?? [], [])
        return (rows, paths, byPath)
    }

    /// Keeps the outline rows in step with the tree after an edit: a property edit leaves the elements as
    /// they are, so their rows and the selection stay; a mount added or undone makes new rows.
    @MainActor
    private func refreshOutlineRows() {
        let (rows, paths, byPath) = sceneOutline(sceneTree?.main)
        if Set(paths.values) == Set(sceneElementPaths.values) {
            return
        }
        sceneElementPaths = paths
        sceneElementsByPath = byPath
        store?.model.elements = rows
        vc?.reloadOutline()
    }

    override class var autosavesInPlace: Bool {
        return true
    }

    override init() {
        super.init()

        // By default create with sample scene.
        let elementModel = ElementModelFactory().elementModel
        elementModel.modelName = "Unnamed"

        updateStore(elementModel)
    }

    fileprivate func updateStore(_ elementModel: ElementModel) {
        // If we had a previous store, unregister from its listeners first.
        if let old = self.store {
            old.onUpdate.removeAll(where: { ($0 as AnyObject) === (self as AnyObject) })
        }
        // Create a new store and register this Document as its listener.
        self.store = ElementModelStore(elementModel)
        if !(self.store!.onUpdate.contains(where: { ($0 as AnyObject) === (self as AnyObject) })) {
            self.store!.onUpdate.append(self)
        }
    }

    func notifyChanges(_ evt: ModelEvent) {
        updateChangeCount(.changeDone)
        vc?.updateWindowTitle()
    }

    override func makeWindowControllers() {
        // Create window programmatically instead of loading it from the storyboard.
        // The layout and controllers are still the same classes (WindowController / ViewController)
        // but instantiated in code so the UI can be managed and changed more easily.
        let contentRect = NSMakeRect(196, 240, 944, 764)
        var style: NSWindow.StyleMask = [.titled, .closable, .miniaturizable, .resizable]
        style.insert(.fullSizeContentView)
        let window = NSWindow(
            contentRect: contentRect,
            styleMask: style,
            backing: .buffered,
            defer: false)
        window.titleVisibility = .hidden
        window.titlebarAppearsTransparent = true
        window.acceptsMouseMovedEvents = true
        window.isReleasedWhenClosed = false
        window.minSize = NSSize(width: 600, height: 400)

        let windowController = WindowController(window: window)

        // Register the window controller with the document first.
        self.addWindowController(windowController)

        // Instantiate the main content view controller programmatically.
        let contentVC = ViewController()

        // Keep a reference to the view controller and wire things up.
        self.vc = contentVC
        contentVC.document = self
        windowController.window?.delegate = self

        // Set contentViewController after window is registered to avoid zero-size constraint conflicts
        // The view will be properly sized by the window's content rect
        windowController.contentViewController = contentVC

        self.vc?.setElementModel(elementStore: self.store!)
        self.vc?.setSceneTree(sceneTree)

        // Restore saved window position (if any).
        if let uri = self.fileURL?.absoluteString, let window = windowController.window,
            let data = PreferenceConstants.preference.defaults.string(forKey: windowPositionOption + uri)
        {
            let p = TennParser()
            let node = p.parse(data)
            if !p.errors.hasErrors() {
                if let pos = node.getChild(0) {
                    let frame = CGRect(
                        x: CGFloat(pos.getFloat(1) ?? 0), y: CGFloat(pos.getFloat(2) ?? 0), width: CGFloat(pos.getFloat(3) ?? 0),
                        height: CGFloat(pos.getFloat(4) ?? 0))
                    window.setFrame(frame, display: true)
                }
            }
        }

    }

    func saveWindowPosition() {
        if let frame = vc?.view.window?.frame, let uri = self.fileURL?.absoluteString {
            let nde = TennNode.newCommand(
                "pos",
                TennNode.newFloatNode(Double(frame.origin.x)),
                TennNode.newFloatNode(Double(frame.origin.y)),
                TennNode.newFloatNode(Double(frame.size.width)),
                TennNode.newFloatNode(Double(frame.size.height))
            )
            PreferenceConstants.preference.defaults.set(nde.toStr(), forKey: windowPositionOption + uri)
        }
    }
    func windowDidMove(_ notification: Notification) {
        saveWindowPosition()
    }
    func windowDidResize(_ notification: Notification) {
        saveWindowPosition()
    }
    func windowDidExpose(_ notification: Notification) {
        saveWindowPosition()
    }

    override func read(from url: URL, ofType typeName: String) throws {
        // Stamped before the read, as go/app's Load: a write that lands during it is the next poll's.
        let stamp = fileStamp(of: url)

        do {
            var storedValue: String
            var notes: [String] = []
            if JSONCanvas.isCanvas(url.path) {
                let converted = try JSONCanvas.canvasText(path: url.path, data: try Data(contentsOf: url))
                storedValue = converted.text
                notes = converted.notes
            } else {
                storedValue = try String(contentsOf: url, encoding: String.Encoding.utf8)
            }
            setCanvasNotes(notes)

            let now = Date()

            let parser = TennParser()
            let node = parser.parse(storedValue)

            if parser.errors.hasErrors() {
                // The file is read, its text kept out of the document: stamping stops the poll retrying.
                setDiskStamp(stamp)
                return
            }

            let elementModel = ElementModel.parseTenn(node: node)

            Swift.debugPrint("Elapsed parse \(Date().timeIntervalSince(now))")

            // The kind reads the parsed model, as go/app does; a scene document then keeps the tree and
            // drops the model, so its file is never treated as a classic diagram.
            if kindOfModel(elementModel) != .diagram {
                let openScene = {
                    MainActor.assumeIsolated {
                        self.readScene(url, text: storedValue)
                        self.setDiskStamp(stamp)
                    }
                }
                if Thread.isMainThread {
                    openScene()
                } else {
                    DispatchQueue.main.sync(execute: openScene)
                }
                return
            }

            elementModel.modelName = url.lastPathComponent

            // AppKit may call read() off the main queue (autosave, open-in-place, iCloud).
            if Thread.isMainThread {
                MainActor.assumeIsolated {
                    self.showClassic(elementModel)
                }
            } else {
                DispatchQueue.main.sync {
                    MainActor.assumeIsolated {
                        self.showClassic(elementModel)
                    }
                }
            }

            self.fileURL = url
            setDiskStamp(stamp)
            setSavedText(elementModel.toTennStr())
        } catch {
            Swift.print("Failed to load file")
        }
    }

    /// Puts a classic document in the window, dropping the scene of a document read before.
    @MainActor private func showClassic(_ elementModel: ElementModel) {
        sceneTree = nil
        sceneText = nil
        savedSceneText = nil
        sceneRoot = nil
        sceneStamps = [:]
        updateStore(elementModel)
        vc?.setElementModel(elementStore: self.store!)
        vc?.setSceneTree(nil)
    }

    /// Opens a scene document: the tree is kept, the outline stays empty, and the text is what an edit
    /// changes and a save writes.
    @MainActor private func readScene(_ url: URL, text: String) {
        let root = workspaceRoot(url)
        sceneRoot = root
        sceneText = text
        savedSceneText = text
        savedText = nil
        let loader = SceneLoader(root: root)
        let source = url.resolvingSymlinksInPath().standardizedFileURL
        let rel = relativePath(from: root.resolvingSymlinksInPath().standardizedFileURL, to: source) ?? url.lastPathComponent
        loader.overlay[rel] = text
        do {
            sceneTree = try loader.loadAuto(rel)
        } catch {
            sceneTree = nil
            Swift.print("Failed to load scene \(url.path): \(error)")
        }
        let (rows, paths, byPath) = sceneOutline(sceneTree?.main)
        sceneElementPaths = paths
        sceneElementsByPath = byPath
        let model = ElementModel()
        model.modelName = url.lastPathComponent
        model.elements = rows
        updateStore(model)
        vc?.setElementModel(elementStore: self.store!)
        vc?.setSceneTree(sceneTree)
        self.fileURL = url
        stampScene()
    }

    /// Mounts a library file as an element at the end of the document, as Go's Element > Add Library
    /// does: the path is relative to the document, the name is the stem or the next free one, one undo step.
    @MainActor
    func addLibrary(at url: URL) throws {
        guard let text = sceneText, let tree = sceneTree, let root = sceneRoot, let main = tree.main else {
            throw SceneEditError(message: "the document is not a scene")
        }
        let target = url.resolvingSymlinksInPath().standardizedFileURL
        let rootURL = root.resolvingSymlinksInPath().standardizedFileURL
        guard isInside(target, rootURL) else {
            throw SceneEditError(
                message:
                    "\(url.lastPathComponent) is outside the workspace root \(rootURL.path): move it into the folder of the document, then add it again"
            )
        }
        let base = (fileURL?.deletingLastPathComponent() ?? rootURL).resolvingSymlinksInPath().standardizedFileURL
        guard let rel = relativePath(from: base, to: target) else {
            throw SceneEditError(message: "cannot mount \(url.lastPathComponent) from this document")
        }
        guard rel != "." else {
            throw SceneEditError(message: "the document cannot mount itself as a library")
        }
        var name = target.deletingPathExtension().lastPathComponent
        if name.isEmpty {
            name = "library"
        }
        let taken = Set(tree.main?.elems.map(\.name) ?? [])
        let first = name
        var i = 2
        while taken.contains(name) {
            name = "\(first) \(i)"
            i += 1
        }
        let cmd =
            "element " + TennNode.newStrNode(name).toStr(0, false) + " --from-file "
            + TennNode.newStrNode(rel).toStr(0, false)
        let bytes = Array(text.utf8)
        let prefix = bytes.last == 0x0A ? "" : "\n"
        try applySceneEdit(
            Scene.TextEdit(start: bytes.count, end: bytes.count, new: prefix + cmd + "\n"), in: main.pos.file)
    }

    /// The files the document mounts with --from-file, each once: the alias a use names and the file.
    @MainActor
    func libraries() -> [(lib: String, file: String)] {
        guard let f = sceneTree?.main else { return [] }
        var out: [(lib: String, file: String)] = []
        var seen = Set<ObjectIdentifier>()
        func walk(_ n: Scene.Node) {
            for c in n.elems {
                if let m = c.mount, seen.insert(ObjectIdentifier(m)).inserted {
                    out.append((c.key, m.pos.file))
                }
                walk(c)
            }
        }
        walk(f)
        return out
    }

    /// The files the tree mounts, nested mounts included, as slash paths from the workspace root; the
    /// tree holds every file it loaded, so a library mounted through another file is here too.
    @MainActor
    func mountedFiles() -> [String] {
        guard let tree = sceneTree else { return [] }
        let main = tree.main?.pos.file
        return tree.order.map(\.pos.file).filter { $0 != main }
    }

    /// What File > Open Library offers: every mounted file with the alias of its first mount and its
    /// absolute URL.
    @MainActor
    func mountedLibraries() -> [(lib: String, file: String, url: URL)] {
        guard let root = sceneRoot else { return [] }
        var aliases: [String: String] = [:]
        for l in libraries() where aliases[l.file] == nil {
            aliases[l.file] = l.lib
        }
        return mountedFiles().map { file in
            (aliases[file] ?? (file as NSString).deletingPathExtension, file, root.appendingPathComponent(file))
        }
    }

    /// What the mounted libraries offer for the query, then what the document itself reuses, grouped
    /// by the branch they are defined in (go/app/outline_scene.go components).
    @MainActor
    func components(_ q: String, limit: Int) -> [Scene.Component] {
        guard let tree = sceneTree else { return [] }
        var out: [Scene.Component] = []
        for r in libraries() {
            guard let n = tree.file(r.file) else { continue }
            out += sceneComponents(alias: r.lib, root: n, q: q, limit: limit - out.count)
        }
        return byGroup(out + ownComponents(q, limit: limit - out.count))
    }

    /// The nodes the document's own use statements target: a path from the root names them wherever
    /// the insertion goes. A node the file never uses is not offered.
    @MainActor
    func ownComponents(_ q: String, limit: Int) -> [Scene.Component] {
        guard let tree = sceneTree, let f = tree.main, limit > 0 else { return [] }
        var seen = Set<ObjectIdentifier>()
        var nodes: [Scene.Node] = []
        func walk(_ n: Scene.Node) {
            for k in sceneNodes(n) {
                if k.mount != nil {
                    continue
                }
                guard k.kind == .use else {
                    walk(k)
                    continue
                }
                guard let parent = k.parent, let target = try? tree.resolve(parent, k.path) else { continue }
                guard !seen.contains(ObjectIdentifier(target)), target.pos.file == f.pos.file else { continue }
                guard usePath("", target) != nil, !target.name.isEmpty else { continue }
                seen.insert(ObjectIdentifier(target))
                nodes.append(target)
            }
        }
        walk(f)
        let root = commonAncestor(nodes)
        let query = q.lowercased()
        var out: [Scene.Component] = []
        for n in nodes {
            if !query.isEmpty && !n.name.lowercased().contains(query) && !n.key.lowercased().contains(query) {
                continue
            }
            guard let p = usePath("", n) else { continue }
            out.append(Scene.Component(path: p, name: n.name, group: branchOf(root, n), node: n))
            if out.count >= limit {
                break
            }
        }
        return out
    }

    /// Writes `use <path> as <name>` into node, in slot, or on the canvas at spot when node is nil.
    /// Answers the byte offset of the use in the edited text, for the canvas to select it.
    @MainActor
    func insertComponent(
        path: String, name: String, into node: Scene.Node?, slot: String, canvas: Scene.Node,
        spot: (x: Double, y: Double)
    ) throws -> Int {
        guard let text = sceneText else {
            throw SceneEditError(message: "the document is not a scene")
        }
        let target = node ?? canvas
        let useName = freeName(target, name)
        var use = commandText("use", path, "as", useName)
        var cmd = slotCommand(slot, use)
        if node == nil {
            use += " { " + commandText("pos", numText(spot.x), numText(spot.y)) + " }"
            cmd = use
        }
        let edit = insertEdit(text, target, cmd)
        guard let at = byteOffset(of: use, in: edit.new) else {
            throw SceneEditError(message: "cannot find the use in \(cmd)")
        }
        try applySceneEdit(edit, in: (target.mount ?? target).pos.file)
        return edit.start + at
    }

    /// The byte offset of needle in text; nil when text does not hold it.
    private func byteOffset(of needle: String, in text: String) -> Int? {
        let hay = Array(text.utf8)
        let pin = Array(needle.utf8)
        guard !pin.isEmpty, hay.count >= pin.count else { return nil }
        for i in 0...(hay.count - pin.count) where Array(hay[i..<i + pin.count]) == pin {
            return i
        }
        return nil
    }

    /// Whether target is root or under it, both resolved.
    private func isInside(_ target: URL, _ root: URL) -> Bool {
        target.path == root.path || target.path.hasPrefix(root.path + "/")
    }

    /// The path of target from base as a slash path, the way filepath.Rel writes it; nil when the two are
    /// on different volumes.
    private func relativePath(from base: URL, to target: URL) -> String? {
        let a = base.pathComponents
        let b = target.pathComponents
        var i = 0
        while i < a.count, i < b.count, a[i] == b[i] { i += 1 }
        var parts = Array(repeating: "..", count: a.count - i)
        parts += b[i...]
        return parts.isEmpty ? "." : parts.joined(separator: "/")
    }

    /// Applies one text edit of a scene document: the edited text is loaded again before it is kept, and
    /// an edit that does not parse or that adds an error is dropped whole, so a save never writes a
    /// broken file. Only the main file can be edited: a mounted one has no window of its own yet.
    ///
    /// file is the file the offsets are in, as go/app's fileOf/ownFileOf choose it.
    @MainActor
    func applySceneEdit(_ edit: Scene.TextEdit, in file: String) throws {
        try applySceneEdits([edit], in: file)
    }

    /// Applies edits of one file as one undo step. A list from setShadowsEdit addresses one text, so its
    /// edits come last-to-first and each one leaves the range of the next valid.
    @MainActor
    func applySceneEdits(_ edits: [Scene.TextEdit], in file: String) throws {
        guard let text = sceneText, let tree = sceneTree, let root = sceneRoot else {
            throw SceneEditError(message: "the document is not a scene")
        }
        guard let main = tree.main?.pos.file, file == main else {
            throw SceneEditError(message: "\(file) is a mounted file: open it to edit it")
        }
        var bytes = Array(text.utf8)
        for edit in edits {
            guard edit.start >= 0, edit.start <= edit.end, edit.end <= bytes.count else {
                throw SceneEditError(message: "range \(edit.start)..\(edit.end) is outside the text of \(bytes.count) bytes")
            }
            bytes.replaceSubrange(edit.start..<edit.end, with: edit.new.utf8)
        }
        let after = String(decoding: bytes, as: UTF8.self)
        let loader = SceneLoader(root: root)
        loader.overlay = [main: after]
        let next: Scene.Tree
        do {
            next = try loader.loadAuto(main)
        } catch {
            throw SceneEditError(message: "not applied: \(error)")
        }
        if next.fatalCount > tree.fatalCount {
            let first = next.diagnostics.first { !$0.warn }?.message ?? "the scene does not load"
            throw SceneEditError(message: "not applied: \(first)")
        }
        keepScene(after, next)
        // The undo keeps the text, not the tree: a scene tree is tens of megabytes, and the old text
        // loads again as it did before the edit.
        undoManager?.registerUndo(withTarget: self) { doc in
            MainActor.assumeIsolated { doc.restoreScene(text) }
        }
        undoManager?.setActionName("Edit text")
        updateChangeCount(.changeDone)
    }

    /// Puts the text and the tree of a scene in the document and in its window.
    @MainActor
    private func keepScene(_ text: String, _ tree: Scene.Tree) {
        sceneText = text
        sceneTree = tree
        stampScene()
        refreshOutlineRows()
        vc?.setSceneTree(tree)
        syncSceneEdited()
    }

    /// Puts back the text an undo returns to, and makes the undo that comes next its redo.
    @MainActor
    private func restoreScene(_ text: String) {
        guard let tree = sceneTree, let root = sceneRoot, let main = tree.main?.pos.file, let current = sceneText else {
            return
        }
        let loader = SceneLoader(root: root)
        loader.overlay = [main: text]
        guard let next = try? loader.loadAuto(main) else { return }
        keepScene(text, next)
        undoManager?.registerUndo(withTarget: self) { doc in
            MainActor.assumeIsolated { doc.restoreScene(current) }
        }
        undoManager?.setActionName("Edit text")
    }

    /// Keeps the edited mark and the title in step with the text, so an undo back to the saved text
    /// clears them without a second count of the changes.
    @MainActor
    private func syncSceneEdited() {
        store?.modified = sceneText != savedSceneText
        vc?.updateWindowTitle()
    }

    /// Keeps the notes of a JSON Canvas conversion for the window; read and write may run off the
    /// main queue.
    private func setCanvasNotes(_ notes: [String]) {
        let work = { MainActor.assumeIsolated { self.sceneCanvasNotes = notes } }
        if Thread.isMainThread { work() } else { DispatchQueue.main.sync(execute: work) }
    }

    /// Keeps the stamp of the file version just read or written; read and write may run off the main queue.
    nonisolated private func setDiskStamp(_ stamp: FileStamp?) {
        let work = { MainActor.assumeIsolated { self.diskStamp = stamp } }
        if Thread.isMainThread { work() } else { DispatchQueue.main.sync(execute: work) }
    }

    /// Stamps every file of the tree: a load, an edit or a save makes the text of the mounted files
    /// what the document shows now.
    @MainActor
    /// Takes the stamps of the files the tree was loaded from. A save that did not relayout the mounts
    /// leaves theirs alone, so a library changed before the save is still read by the poll.
    private func stampScene(includeMounts: Bool = true) {
        guard let tree = sceneTree, let root = sceneRoot else {
            sceneStamps = [:]
            return
        }
        var next: [String: FileStamp?] = includeMounts ? [:] : sceneStamps
        for n in tree.order where includeMounts || n.pos.file == tree.main?.pos.file {
            next[n.pos.file] = fileStamp(of: root.appendingPathComponent(n.pos.file))
        }
        sceneStamps = next
    }

    /// The main-queue hop for a save, which AppKit may run off the main queue.
    nonisolated private func refreshSceneStamps() {
        let work = { MainActor.assumeIsolated { self.stampScene(includeMounts: false) } }
        if Thread.isMainThread { work() } else { DispatchQueue.main.sync(execute: work) }
    }

    /// Keeps the canonical classic text of the version just read or written.
    nonisolated private func setSavedText(_ text: String?) {
        let work = { MainActor.assumeIsolated { self.savedText = text } }
        if Thread.isMainThread { work() } else { DispatchQueue.main.sync(execute: work) }
    }

    override var isDocumentEdited: Bool {
        get {
            return store?.modified ?? true
        }
    }

    /// Whether the file on disk is not the version this document last read or wrote. A file that cannot
    /// be stat-ed (an editor replacing it) is not a change, as go/app's Watcher.Poll.
    @MainActor
    func diskChanged() -> Bool {
        guard let url = fileURL, let known = diskStamp, let now = fileStamp(of: url) else { return false }
        return now != known
    }

    /// Whether a file the tree mounts changed on disk since the last read, edit or save. A stat error is
    /// not a change, as for the main file.
    @MainActor
    func mountedFileChanged() -> Bool {
        guard let root = sceneRoot else { return false }
        let main = sceneTree?.main?.pos.file
        for (file, known) in sceneStamps where file != main {
            guard let now = fileStamp(of: root.appendingPathComponent(file)) else { continue }
            if now != known { return true }
        }
        return false
    }

    /// Reads the whole scene again after one of its files changed, as go/app's sceneReload: the text of
    /// the file on disk is loaded again and the document stops being edited.
    @MainActor
    private func reloadScene(from url: URL) {
        let stamp = fileStamp(of: url)
        do {
            var text: String
            var notes: [String] = []
            if JSONCanvas.isCanvas(url.path) {
                let converted = try JSONCanvas.canvasText(path: url.path, data: try Data(contentsOf: url))
                text = converted.text
                notes = converted.notes
            } else {
                text = try String(contentsOf: url, encoding: .utf8)
            }
            readScene(url, text: text)
            sceneCanvasNotes = notes
            diskStamp = stamp
            stampScene()
            updateChangeCount(.changeCleared)
            syncSceneEdited()
        } catch {
            // The tree and the text already open stay; the next poll tries again.
        }
    }

    /// Follows another program's changes: an unedited document reads the file again, a changed library
    /// reloads the whole scene, an edited document keeps its text.
    @MainActor
    func pollExternalChanges() {
        guard let url = fileURL, !isDocumentEdited else { return }
        if sceneTree != nil, mountedFileChanged() {
            reloadScene(from: url)
            return
        }
        guard diskChanged() else { return }
        do {
            try revert(toContentsOf: url, ofType: fileType ?? "")
        } catch {
            // The text of the version already open stays; the next poll asks again.
        }
    }

    /// The text a stopped run keeps: the scene text, or the classic model written as .tenn.
    @MainActor
    func snapshotText() -> String? {
        sceneText ?? store?.model.toTennStr()
    }

    /// Puts a kept text back as an unsaved draft, one undo step back to what the document held.
    /// Nothing changes when the text does not load.
    @MainActor
    func restoreDraft(_ text: String) throws {
        if let tree = sceneTree {
            guard let root = sceneRoot, let main = tree.main?.pos.file else {
                throw SceneEditError(message: "the document is not a scene")
            }
            let loader = SceneLoader(root: root)
            loader.overlay = [main: text]
            let next: Scene.Tree
            do {
                next = try loader.loadAuto(main)
            } catch {
                throw SceneEditError(message: "not restored: \(error)")
            }
            if next.fatalCount > tree.fatalCount {
                let first = next.diagnostics.first { !$0.warn }?.message ?? "the scene does not load"
                throw SceneEditError(message: "not restored: \(first)")
            }
            let before = sceneText
            keepScene(text, next)
            undoManager?.registerUndo(withTarget: self) { doc in
                MainActor.assumeIsolated {
                    if let before { doc.restoreScene(before) }
                }
            }
            undoManager?.setActionName("Restore unsaved changes")
            updateChangeCount(.changeDone)
            return
        }
        guard let model = classicModel(text) else {
            throw SceneEditError(message: "not restored: the text does not parse")
        }
        let before = store?.model.toTennStr()
        showClassic(model)
        store?.modified = text != savedText
        undoManager?.registerUndo(withTarget: self) { doc in
            MainActor.assumeIsolated {
                if let before { doc.restoreClassicDraft(before) }
            }
        }
        undoManager?.setActionName("Restore unsaved changes")
        updateChangeCount(.changeDone)
        vc?.updateWindowTitle()
    }

    /// Parses a classic document's text; nil when it does not parse.
    @MainActor
    private func classicModel(_ text: String) -> ElementModel? {
        let parser = TennParser()
        let node = parser.parse(text)
        guard !parser.errors.hasErrors() else { return nil }
        let model = ElementModel.parseTenn(node: node)
        model.modelName = store?.model.modelName ?? ""
        return model
    }

    /// Puts a classic text back in the document, as the undo of restoreDraft.
    @MainActor
    private func restoreClassicDraft(_ text: String) {
        guard let model = classicModel(text) else { return }
        let current = store?.model.toTennStr()
        showClassic(model)
        store?.modified = text != savedText
        undoManager?.registerUndo(withTarget: self) { doc in
            MainActor.assumeIsolated {
                if let current { doc.restoreClassicDraft(current) }
            }
        }
        undoManager?.setActionName("Restore unsaved changes")
        vc?.updateWindowTitle()
    }

    override func write(to url: URL, ofType typeName: String) throws {
        // AppKit calls write() off the main queue when autosavesInPlace is true.
        let readStore: () -> ElementModelStore? = {
            if Thread.isMainThread {
                return MainActor.assumeIsolated { self.store }
            }
            return DispatchQueue.main.sync { MainActor.assumeIsolated { self.store } }
        }
        let sceneTextNow: () -> String? = {
            if Thread.isMainThread {
                return MainActor.assumeIsolated { self.sceneText }
            }
            return DispatchQueue.main.sync { MainActor.assumeIsolated { self.sceneText } }
        }
        let markSaved: (String) -> Void = { text in
            let work = {
                MainActor.assumeIsolated {
                    self.savedSceneText = text
                    self.syncSceneEdited()
                }
            }
            if Thread.isMainThread { work() } else { DispatchQueue.main.sync(execute: work) }
        }
        let finish: (String) -> Void = { name in
            let work = {
                MainActor.assumeIsolated {
                    self.updateChangeCount(.changeCleared)
                    self.vc?.updateWindowTitle()
                }
            }
            if Thread.isMainThread { work() } else { DispatchQueue.main.sync(execute: work) }
            _ = name
        }
        if JSONCanvas.isCanvas(url.path) {
            let convert: () throws -> (data: Data, notes: [String], text: String?) = {
                let work = { try MainActor.assumeIsolated { try self.canvasData(to: url) } }
                if Thread.isMainThread { return try work() }
                return try DispatchQueue.main.sync(execute: work)
            }
            let converted = try convert()
            try converted.data.write(to: url, options: .atomic)
            setDiskStamp(fileStamp(of: url))
            refreshSceneStamps()
            if let text = converted.text {
                markSaved(text)
            } else if let es = readStore() {
                es.modified = false
                es.model.modelName = url.lastPathComponent
                setSavedText(es.model.toTennStr())
            }
            setCanvasNotes(converted.notes)
            finish(url.lastPathComponent)
            self.fileURL = url
            return
        }
        do {
            if let text = sceneTextNow() {
                // shortcut: a save under another name keeps the tree of the old path; an edit after it
                // still loads from the folder the document was opened in.
                try Data(text.utf8).write(to: url, options: .atomic)
                setDiskStamp(fileStamp(of: url))
                refreshSceneStamps()
                markSaved(text)
                finish(url.lastPathComponent)
                self.fileURL = url
                return
            }
            if let es = readStore() {
                let value = es.model.toTennStr()
                try value.write(to: url, atomically: true, encoding: String.Encoding.utf8)
                setDiskStamp(fileStamp(of: url))
                refreshSceneStamps()
                es.modified = false
                setSavedText(value)

                es.model.modelName = url.lastPathComponent
                finish(url.lastPathComponent)
                self.fileURL = url
            }
        } catch {
            Swift.print("Some error happen")
        }
    }

    /// The document as JSON Canvas: the current scene text, or the classic model written as .tenn, loaded
    /// again through the workspace root so mounts resolve.
    @MainActor
    private func canvasData(to url: URL) throws -> (data: Data, notes: [String], text: String?) {
        let text: String
        let savedText: String?
        if let sceneText {
            text = sceneText
            savedText = sceneText
        } else if let modelText = store?.model.toTennStr() {
            text = modelText
            savedText = nil
        } else {
            throw SceneEditError(message: "the document has no text to write as JSON Canvas")
        }
        let source = (fileURL ?? url).resolvingSymlinksInPath().standardizedFileURL
        let storedRoot = sceneRoot?.resolvingSymlinksInPath().standardizedFileURL
        let root =
            storedRoot.flatMap { isInside(source, $0) ? $0 : nil } ?? workspaceRoot(source).resolvingSymlinksInPath().standardizedFileURL
        guard let rel = relativePath(from: root, to: source), rel != "..", !rel.hasPrefix("../") else {
            throw SceneEditError(message: "\(source.path) is outside the workspace root \(root.path)")
        }
        let loader = SceneLoader(root: root)
        loader.overlay[rel] = text
        let tree = try loader.loadAuto(rel)
        let converted = try JSONCanvas.fromTree(tree)
        if let main = tree.main, !main.elems.isEmpty, converted.canvas.nodes.isEmpty {
            let why =
                converted.notes.isEmpty
                ? "no element of the document could be written as JSON Canvas" : converted.notes.joined(separator: "\n")
            throw SceneEditError(message: why)
        }
        return (try JSONCanvas.marshal(converted.canvas), converted.notes, savedText)
    }
}

/// A version of a file on disk: modification time and size, as go/app's Stamp.
private struct FileStamp: Equatable {
    let modified: Date
    let size: Int64
}

/// The stamp of the file at url, or nil when it cannot be stat-ed.
private func fileStamp(of url: URL) -> FileStamp? {
    guard let attrs = try? FileManager.default.attributesOfItem(atPath: url.path),
        let size = (attrs[.size] as? NSNumber)?.int64Value,
        let modified = attrs[.modificationDate] as? Date
    else {
        return nil
    }
    return FileStamp(modified: modified, size: size)
}

/// Why a scene edit was dropped: the message the window shows.
struct SceneEditError: LocalizedError {
    let message: String

    var errorDescription: String? { message }
}

/// The folder mounts resolve in, as app.WorkspaceRoot: the nearest folder up with .git or
/// tenniarb.tenn, else the folder of the file.
private func workspaceRoot(_ path: URL) -> URL {
    let dir = path.deletingLastPathComponent().path
    var d = dir
    while true {
        for mark in [".git", "tenniarb.tenn"] {
            if FileManager.default.fileExists(atPath: (d as NSString).appendingPathComponent(mark)) {
                return URL(fileURLWithPath: d)
            }
        }
        // URL.deletingLastPathComponent of "/" gives "/..", so the string path is what terminates.
        let up = (d as NSString).deletingLastPathComponent
        if up == d { return URL(fileURLWithPath: dir) }
        d = up
    }
}
