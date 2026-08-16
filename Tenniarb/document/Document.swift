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

class Document: NSDocument, IElementModelListener, NSWindowDelegate, @unchecked Sendable {
    @MainActor var store: ElementModelStore?

    @MainActor var vc: ViewController?

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

            elementModel.modelName = url.lastPathComponent

            // AppKit may call read() off the main queue (autosave, open-in-place, iCloud).
            if Thread.isMainThread {
                MainActor.assumeIsolated {
                    self.updateStore(elementModel)
                    vc?.setElementModel(elementStore: self.store!)
                }
            } else {
                DispatchQueue.main.sync {
                    MainActor.assumeIsolated {
                        self.updateStore(elementModel)
                        self.vc?.setElementModel(elementStore: self.store!)
                    }
                }
            }

            self.fileURL = url
        } catch {
            Swift.print("Failed to load file")
        }
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
