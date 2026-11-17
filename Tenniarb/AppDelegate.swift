//
//  AppDelegate.swift
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
//

import Cocoa
import StoreKit

@main
@MainActor
class AppDelegate: NSObject, NSApplicationDelegate {
    var terminateOnLastWindowClose = true
    var preferencesWindowController: NSWindowController?

    /// Follows a file another program changes, as go/app's PollInterval.
    private var externalChangeTimer: Timer?

    /// File > Open Library and the File menu that holds it, kept for the menu delegate.
    static let openLibraryTitle = "Open Library"
    private weak var fileMenu: NSMenu?
    private weak var openLibraryItem: NSMenuItem?

    /// Kept alive while the app runs: a released DispatchSourceSignal never fires.
    private var terminationSources: [DispatchSourceSignal] = []

    /// The stopped run is offered once per launch.
    private var offeredRecovery = false

    /// Tests run inside the app and must not install signal handlers or open alerts.
    private var runningTests: Bool {
        ProcessInfo.processInfo.environment["XCTestConfigurationFilePath"] != nil
            || NSClassFromString("XCTestCase") != nil
    }

    func applicationWillTerminate(_ aNotification: Notification) {
        externalChangeTimer?.invalidate()
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
        return terminateOnLastWindowClose
    }

    public func setTerminateWindows(_ value: Bool) {
        self.terminateOnLastWindowClose = value
    }

    func applicationDidFinishLaunching(_ notification: Notification) {
        Swift.print("Launching application")

        // Configure Settings menu item to target this AppDelegate
        // Use delayed execution to ensure menu is fully ready
        DispatchQueue.main.async { [weak self] in
            self?.configureSettingsMenu()
            self?.installOpenLibraryMenu()
        }
        startExternalChangePoll()
        watchTerminate()
        if !runningTests {
            DispatchQueue.main.async { [weak self] in
                self?.offerRecovery()
            }
        }
    }

    /// The system hands a tenniarb:// link over, also before any window exists; each one opens its file
    /// and shows the place in it.
    func application(_ application: NSApplication, open urls: [URL]) {
        for url in urls where Links.isURL(url.absoluteString) {
            openLink(url.absoluteString, from: currentDocument())
        }
    }

    /// Reads an open document again when another program changed its file: go/app asks every 500 ms.
    private func startExternalChangePoll() {
        guard externalChangeTimer == nil else { return }
        let timer = Timer.scheduledTimer(withTimeInterval: 0.5, repeats: true) { _ in
            MainActor.assumeIsolated {
                for document in NSDocumentController.shared.documents {
                    (document as? Document)?.pollExternalChanges()
                }
            }
        }
        timer.tolerance = 0.1
        externalChangeTimer = timer
    }

    /// Keeps unsaved work when the process is told to stop, as go/app's watchTerminate: make install or a
    /// kill cannot ask the windows to save.
    private func watchTerminate() {
        guard terminationSources.isEmpty, !runningTests else { return }
        for sig in [SIGTERM, SIGINT] {
            // The source reports the signal; the default handler would end the process first.
            signal(sig, SIG_IGN)
            let source = DispatchSource.makeSignalSource(signal: sig, queue: .main)
            source.setEventHandler { [weak self] in
                MainActor.assumeIsolated {
                    self?.keepUnsavedWork()
                    exit(0)
                }
            }
            source.resume()
            terminationSources.append(source)
        }
    }

    /// Writes the unsaved text of every open file, as go/app's saveRecovery without the mounted libraries.
    private func keepUnsavedWork() {
        var texts: [String: String] = [:]
        for case let document as Document in NSDocumentController.shared.documents {
            guard let url = document.fileURL, document.isDocumentEdited, let text = document.snapshotText() else {
                continue
            }
            texts[url.resolvingSymlinksInPath().standardizedFileURL.path] = text
        }
        do {
            _ = try Recovery.write(texts, to: Recovery.configDirectory(), now: Date())
        } catch {
            Swift.print("Could not keep unsaved work: \(error)")
        }
    }

    /// Offers the work of a stopped run once per launch; a No leaves the folder for the next launch.
    private func offerRecovery() {
        guard !offeredRecovery else { return }
        offeredRecovery = true
        guard let (folder, manifest) = try? Recovery.load(from: Recovery.configDirectory()) else { return }
        let clock = DateFormatter()
        clock.locale = Locale(identifier: "en_US_POSIX")
        clock.dateFormat = "HH:mm"
        let when = clock.string(from: manifest.at)
        let alert = NSAlert()
        alert.messageText = "Unsaved changes from a stopped run"
        alert.informativeText =
            "Tenniarb was stopped at \(when) with \(manifest.entries.count) unsaved file(s). Open them as unsaved changes?"
        alert.alertStyle = .warning
        alert.addButton(withTitle: "Yes")
        alert.addButton(withTitle: "No")
        guard alert.runModal() == .alertFirstButtonReturn else { return }
        do {
            _ = try restoreRecovery(folder: folder, manifest: manifest)
        } catch {
            let failure = NSAlert()
            failure.messageText = "Could not restore unsaved changes: \(error.localizedDescription)"
            failure.runModal()
        }
    }

    /// Opens every kept text as an unsaved draft of its file, without the question. The folder goes only
    /// when every entry comes back.
    @MainActor
    func restoreRecovery(folder: URL, manifest: RecoveryManifest) throws -> Int {
        var failures: [String] = []
        var restored = 0
        for entry in manifest.entries {
            do {
                let text = try Recovery.text(folder: folder, entryFile: entry.file)
                let document = try openDocument(at: URL(fileURLWithPath: entry.path))
                try document.restoreDraft(text)
                restored += 1
            } catch {
                failures.append("\(entry.path): \(error.localizedDescription)")
            }
        }
        guard failures.isEmpty else { throw RecoveryError(failures: failures) }
        try Recovery.drop(folder: folder)
        return restored
    }

    /// The open document at url, or the file opened as one.
    @MainActor
    private func openDocument(at url: URL) throws -> Document {
        if let document = NSDocumentController.shared.documents.compactMap({ $0 as? Document })
            .first(where: { samePath($0.fileURL, url) })
        {
            return document
        }
        let answer = OpenAnswer()
        NSDocumentController.shared.openDocument(withContentsOf: url, display: true) { document, _, error in
            answer.document = document as? Document
            answer.error = error
        }
        // AppKit answers on the main queue: the run loop lets that answer arrive before the restore goes on.
        let deadline = Date(timeIntervalSinceNow: 30)
        while answer.document == nil, answer.error == nil, Date() < deadline {
            RunLoop.current.run(mode: .default, before: Date(timeIntervalSinceNow: 0.01))
        }
        if let document = answer.document {
            return document
        }
        throw answer.error ?? RecoveryError(failures: ["\(url.path) did not open"])
    }

    /// Whether two URLs name the same file, resolved as go/app's realPath.
    private func samePath(_ a: URL?, _ b: URL) -> Bool {
        guard let a else { return false }
        return a.resolvingSymlinksInPath().standardizedFileURL.path == b.resolvingSymlinksInPath().standardizedFileURL.path
    }

    /// The files the active document mounts, so File > Open Library opens one in its own window. Both the
    /// File menu and the submenu fill their items when shown: a library added since the last look appears.
    private func installOpenLibraryMenu() {
        guard let file = NSApp.mainMenu?.items.first(where: { $0.submenu?.title == "File" })?.submenu,
            let item = file.items.first(where: { $0.title == Self.openLibraryTitle })
        else { return }
        fileMenu = file
        openLibraryItem = item
        file.delegate = self
        item.submenu?.delegate = self
        item.isEnabled = false
    }

    /// Fills Open Library with the mounts of the active document by alias and file.
    private func fillOpenLibrary(_ menu: NSMenu) {
        menu.removeAllItems()
        let libraries = activeSceneDocument()?.mountedLibraries() ?? []
        guard !libraries.isEmpty else {
            let none = NSMenuItem(title: "No mounted libraries", action: nil, keyEquivalent: "")
            none.isEnabled = false
            menu.addItem(none)
            return
        }
        let many = libraries.count > 1
        for library in libraries {
            let item = NSMenuItem(
                title: many ? "\(library.lib) — \(library.file)" : "Open \(library.lib)",
                action: #selector(openLibrary(_:)), keyEquivalent: "")
            item.target = self
            item.representedObject = library.url
            menu.addItem(item)
        }
    }

    /// Opens the chosen library in a window of its own; a window that already shows it comes to the front.
    @objc func openLibrary(_ sender: NSMenuItem) {
        guard let url = sender.representedObject as? URL else { return }
        NSDocumentController.shared.openDocument(withContentsOf: url, display: true) { document, _, error in
            if let error {
                Swift.print("Could not open \(url.path): \(error)")
                return
            }
            (document as? Document)?.showWindows()
        }
    }

    /// File > Open Link…: a field for a link of either form, filled in when the clipboard holds one.
    @objc func openLink(_ sender: Any?) {
        let alert = NSAlert()
        alert.messageText = "Open Link"
        alert.informativeText =
            "A tenniarb://open link, or a file with its element and node: File.tenn#Element/node."
        alert.addButton(withTitle: "Open")
        alert.addButton(withTitle: "Cancel")
        let field = NSTextField(frame: NSRect(x: 0, y: 0, width: 460, height: 24))
        field.placeholderString = "tenniarb://open?… or File.tenn#Element/node"
        if let text = NSPasteboard.general.string(forType: .string), Links.isLink(text) {
            field.stringValue = text.trimmingCharacters(in: .whitespacesAndNewlines)
        }
        alert.accessoryView = field
        guard alert.runModal() == .alertFirstButtonReturn else { return }
        openLink(field.stringValue, from: currentDocument())
    }

    /// The document a link is opened from: the current one, else the one whose window is key.
    private func currentDocument() -> Document? {
        (NSDocumentController.shared.currentDocument ?? NSApp.keyWindow?.windowController?.document) as? Document
    }

    /// Opens the file a link names and shows its place, as go/app's openLink: every failure says why.
    private func openLink(_ text: String, from: Document?) {
        let link: Links.Link
        do {
            link = try Links.parse(text)
        } catch {
            say("Not a link: \(error.localizedDescription).")
            return
        }
        let documents = NSDocumentController.shared.documents.compactMap { ($0 as? Document)?.fileURL }
        guard
            let path = LinkOpen.resolve(
                link, active: from?.fileURL, openDocuments: documents,
                home: FileManager.default.homeDirectoryForCurrentUser, exists: LinkOpen.isFile)
        else {
            say("Cannot find \(link.path)\(link.repo.isEmpty ? "" : " in \(link.repo)") on this machine.")
            return
        }
        let url = URL(fileURLWithPath: path)
        NSDocumentController.shared.openDocument(withContentsOf: url, display: true) { document, _, error in
            MainActor.assumeIsolated {
                guard let document = document as? Document else {
                    self.say("Could not open \(path): \(error?.localizedDescription ?? "not a Tenniarb document").")
                    return
                }
                self.show(link, in: document)
            }
        }
    }

    /// Brings the window of the document forward and shows the place the link names.
    private func show(_ link: Links.Link, in document: Document) {
        document.showWindows()
        guard let vc = document.vc else {
            say("Opened \(document.fileURL?.lastPathComponent ?? "the file") but found no window to show the place in.")
            return
        }
        vc.showLink(element: link.element, node: link.node)
    }

    /// Says why a link did not open.
    private func say(_ text: String) {
        let alert = NSAlert()
        alert.messageText = "Open Link"
        alert.informativeText = text
        alert.runModal()
    }

    /// The document the menus act on: the current one, else the one whose window is key.
    private func activeSceneDocument() -> Document? {
        let current = NSDocumentController.shared.currentDocument ?? NSApp.keyWindow?.windowController?.document
        guard let document = current as? Document, document.sceneTree != nil else { return nil }
        return document
    }

    private func configureSettingsMenu() {
        Swift.print("configureSettingsMenu called, mainMenu: \(String(describing: NSApp.mainMenu))")
        if let appMenu = NSApp.mainMenu?.item(at: 0)?.submenu {
            Swift.print("Found app menu with \(appMenu.items.count) items")
            for item in appMenu.items {
                Swift.print("Menu item: '\(item.title)' keyEquivalent: '\(item.keyEquivalent)'")
                if item.keyEquivalent == "," {
                    item.target = self
                    item.action = #selector(showPreferences(_:))
                    Swift.print("Settings menu configured successfully!")
                    break
                }
            }
        } else {
            Swift.print("Could not find app menu!")
        }
    }

    func applicationDidBecomeActive(_ notification: Notification) {
        Swift.print("Application did become active")
    }

    @objc func showPreferences(_ sender: Any?) {
        Swift.print("AppDelegate.showPreferences called")
        // Show preferences window built from `PreferencesController` programmatically.
        if let wc = preferencesWindowController, wc.window?.isVisible == true {
            Swift.print("  - existing window visible, bringing to front")
            wc.window?.makeKeyAndOrderFront(self)
            return
        }
        Swift.print("  - creating new preferences window")

        // Create the preferences controller and its child panes in code.
        let prefs = PreferencesController()
        let general = PreferencesGeneralController()
        general.title = "General"
        let tabItem = NSTabViewItem(viewController: general)
        prefs.addTabViewItem(tabItem)

        let window = NSWindow(contentViewController: prefs)
        window.styleMask = [.titled, .closable]
        window.title = "Preferences"

        let wc = NSWindowController(window: window)
        self.preferencesWindowController = wc

        // When the window closes, clear the stored reference so it can be recreated.
        NotificationCenter.default.addObserver(forName: NSWindow.willCloseNotification, object: window, queue: .main) { [weak self] _ in
            MainActor.assumeIsolated {
                self?.preferencesWindowController = nil
            }
        }

        wc.showWindow(self)
    }

    @objc func showHelp(_ sender: Any?) {
        let helpController = HelpController()
        let helpWindow = NSWindow(contentViewController: helpController)
        helpWindow.styleMask = [.titled, .closable, .resizable]
        helpWindow.title = "Tenniarb Help"
        helpWindow.setContentSize(NSSize(width: 800, height: 600))
        let wc = NSWindowController(window: helpWindow)
        wc.showWindow(self)
    }
}

extension AppDelegate: NSMenuDelegate {
    func menuNeedsUpdate(_ menu: NSMenu) {
        if menu === openLibraryItem?.submenu {
            fillOpenLibrary(menu)
        } else if menu === fileMenu {
            openLibraryItem?.isEnabled = !(activeSceneDocument()?.mountedLibraries().isEmpty ?? true)
            // Filled here too: AppKit may grey a submenu that is empty when the File menu opens.
            if let submenu = openLibraryItem?.submenu {
                fillOpenLibrary(submenu)
            }
        }
    }
}

/// The answer of openDocument's completion handler, read on the main queue after the run loop spins.
private final class OpenAnswer: @unchecked Sendable {
    var document: Document?
    var error: Error?
}
