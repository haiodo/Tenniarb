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

@NSApplicationMain
class AppDelegate: NSObject, NSApplicationDelegate {
    var terminateOnLastWindowClose = true
    var preferencesWindowController: NSWindowController?

    func applicationWillTerminate(_ aNotification: Notification) {
        // Insert code here to tear down your application
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
        return terminateOnLastWindowClose
    }

    public func setTerminateWindows(_ value: Bool ) {
        self.terminateOnLastWindowClose = value
    }

    func applicationDidFinishLaunching(_ notification: Notification) {
        Swift.print("Launching application")

        // Configure Settings menu item to target this AppDelegate
        // Use delayed execution to ensure menu is fully ready
        DispatchQueue.main.async { [weak self] in
            self?.configureSettingsMenu()
        }
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
        NotificationCenter.default.addObserver(forName: NSWindow.willCloseNotification, object: window, queue: nil) { [weak self] _ in
            self?.preferencesWindowController = nil
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
