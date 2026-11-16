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

    /// The bytes a scene document was read from. A scene has no write side yet, so a save puts them back
    /// instead of printing classic text over the file.
    private var sceneBytes: Data?

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

        do {
            let storedValue = try String(contentsOf: url, encoding: String.Encoding.utf8)

            let now = Date()

            let parser = TennParser()
            let node = parser.parse(storedValue)

            if parser.errors.hasErrors() {
                return
            }

            let elementModel = ElementModel.parseTenn(node: node)

            Swift.debugPrint("Elapsed parse \(Date().timeIntervalSince(now))")

            // The kind reads the parsed model, as go/app does; a scene document then keeps the tree and
            // drops the model, so its file is never treated as a classic diagram.
            if kindOfModel(elementModel) != .diagram {
                let openScene = { MainActor.assumeIsolated { self.readScene(url, text: storedValue) } }
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
        } catch {
            Swift.print("Failed to load file")
        }
    }

    /// Puts a classic document in the window, dropping the scene of a document read before.
    @MainActor private func showClassic(_ elementModel: ElementModel) {
        sceneTree = nil
        sceneBytes = nil
        updateStore(elementModel)
        vc?.setElementModel(elementStore: self.store!)
        vc?.setSceneTree(nil)
    }

    /// Opens a scene document read-only: the tree is kept, the outline stays empty, and a save writes the
    /// file back as it came.
    @MainActor private func readScene(_ url: URL, text: String) {
        sceneBytes = Data(text.utf8)
        do {
            sceneTree = try SceneLoader(root: workspaceRoot(url)).loadAuto(url.path)
        } catch {
            sceneTree = nil
            Swift.print("Failed to load scene \(url.path): \(error)")
        }
        let model = ElementModel()
        model.modelName = url.lastPathComponent
        updateStore(model)
        vc?.setElementModel(elementStore: self.store!)
        vc?.setSceneTree(sceneTree)
        self.fileURL = url
    }

    override var isDocumentEdited: Bool {
        get {
            return store?.modified ?? true
        }
    }

    override func write(to url: URL, ofType typeName: String) throws {
        // AppKit calls write() off the main queue when autosavesInPlace is true.
        let readStore: () -> ElementModelStore? = {
            if Thread.isMainThread {
                return MainActor.assumeIsolated { self.store }
            }
            return DispatchQueue.main.sync { MainActor.assumeIsolated { self.store } }
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
        do {
            if let bytes = sceneBytes {
                try bytes.write(to: url, options: .atomic)
                finish(url.lastPathComponent)
                self.fileURL = url
                return
            }
            if let es = readStore() {
                let value = es.model.toTennStr()
                try value.write(to: url, atomically: true, encoding: String.Encoding.utf8)
                es.modified = false

                es.model.modelName = url.lastPathComponent
                finish(url.lastPathComponent)
                self.fileURL = url
            }
        } catch {
            Swift.print("Some error happen")
        }
    }
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
