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
        }
        startExternalChangePoll()
        watchTerminate()
        if !runningTests {
            DispatchQueue.main.async { [weak self] in
                self?.offerRecovery()
            }
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

/// The answer of openDocument's completion handler, read on the main queue after the run loop spins.
private final class OpenAnswer: @unchecked Sendable {
    var document: Document?
    var error: Error?
}
