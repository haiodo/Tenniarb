//
//  Application.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 07.06.2019.
//  Copyright © 2019 Andrey Sobolev. All rights reserved.
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

import Foundation
import Cocoa

public class TenniarbApplication: NSApplication {

    override init() {
        super.init()
        // Setup the main menu immediately during application initialization
        // This ensures the menu is available before any windows are shown
        setupMainMenu()
    }

    required init?(coder: NSCoder) {
        super.init(coder: coder)
        setupMainMenu()
    }

    /// Show preferences window directly (since delegate may not be set yet)
    @objc func showPreferences(_ sender: Any?) {
        Swift.print("TenniarbApplication.showPreferences called!")
        // Try to forward to AppDelegate first
        if let appDelegate = self.delegate as? AppDelegate {
            Swift.print("  - forwarding to AppDelegate")
            appDelegate.showPreferences(sender)
            return
        }

        // If no delegate, show preferences directly
        Swift.print("  - showing preferences directly")
        showPreferencesWindow()
    }

    private static var preferencesWindowController: NSWindowController?

    private func showPreferencesWindow() {
        // Check if window already exists and is visible
        if let wc = TenniarbApplication.preferencesWindowController, wc.window?.isVisible == true {
            wc.window?.makeKeyAndOrderFront(self)
            return
        }

        // Create the preferences controller and its child panes in code
        let prefs = PreferencesController()
        let general = PreferencesGeneralController()
        general.title = "General"
        let tabItem = NSTabViewItem(viewController: general)
        prefs.addTabViewItem(tabItem)

        // Create window with proper initial size
        let contentRect = NSRect(x: 0, y: 0, width: 500, height: 200)
        let window = NSWindow(contentRect: contentRect,
                              styleMask: [.titled, .closable],
                              backing: .buffered,
                              defer: false)
        window.title = "Preferences"
        window.contentViewController = prefs
        window.center()

        let wc = NSWindowController(window: window)
        TenniarbApplication.preferencesWindowController = wc

        // When the window closes, clear the stored reference
        NotificationCenter.default.addObserver(forName: NSWindow.willCloseNotification, object: window, queue: nil) { _ in
            TenniarbApplication.preferencesWindowController = nil
        }

        wc.showWindow(self)
    }

    /// Build the complete application menu programmatically to match the storyboard.
    private func setupMainMenu() {
        let mainMenu = NSMenu(title: "Main Menu")

        let appName = Bundle.main.object(forInfoDictionaryKey: "CFBundleName") as? String ?? "Tenniarb"

        // MARK: - Application menu (Tenniarb)
        let appMenuItem = NSMenuItem()
        mainMenu.addItem(appMenuItem)
        let appMenu = NSMenu(title: appName)
        appMenuItem.submenu = appMenu

        appMenu.addItem(NSMenuItem(title: "About \(appName)", action: #selector(NSApplication.orderFrontStandardAboutPanel(_:)), keyEquivalent: ""))
        appMenu.addItem(NSMenuItem.separator())

        // Settings menu item - target is self (TenniarbApplication) which forwards to AppDelegate
        let prefsItem = NSMenuItem(title: "Settings…", action: #selector(showPreferences(_:)), keyEquivalent: ",")
        prefsItem.target = self
        appMenu.addItem(prefsItem)

        appMenu.addItem(NSMenuItem.separator())

        // Services submenu
        let servicesItem = NSMenuItem(title: "Services", action: nil, keyEquivalent: "")
        let servicesMenu = NSMenu(title: "Services")
        servicesItem.submenu = servicesMenu
        self.servicesMenu = servicesMenu
        appMenu.addItem(servicesItem)

        appMenu.addItem(NSMenuItem.separator())

        appMenu.addItem(NSMenuItem(title: "Hide \(appName)", action: #selector(NSApplication.hide(_:)), keyEquivalent: "h"))

        let hideOthersItem = NSMenuItem(title: "Hide Others", action: #selector(NSApplication.hideOtherApplications(_:)), keyEquivalent: "h")
        hideOthersItem.keyEquivalentModifierMask = [.option, .command]
        appMenu.addItem(hideOthersItem)

        appMenu.addItem(NSMenuItem(title: "Show All", action: #selector(NSApplication.unhideAllApplications(_:)), keyEquivalent: ""))

        appMenu.addItem(NSMenuItem.separator())

        appMenu.addItem(NSMenuItem(title: "Quit \(appName)", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q"))

        // MARK: - File menu
        let fileMenuItem = NSMenuItem()
        mainMenu.addItem(fileMenuItem)
        let fileMenu = NSMenu(title: "File")
        fileMenuItem.submenu = fileMenu

        fileMenu.addItem(NSMenuItem(title: "New", action: #selector(NSDocumentController.newDocument(_:)), keyEquivalent: "n"))
        fileMenu.addItem(NSMenuItem(title: "Open…", action: #selector(NSDocumentController.openDocument(_:)), keyEquivalent: "o"))

        // Open Recent submenu - NSDocumentController will populate this automatically
        let openRecentItem = NSMenuItem(title: "Open Recent", action: nil, keyEquivalent: "")
        let openRecentMenu = NSMenu(title: "Open Recent")
        // Mark this menu as the recent documents menu so NSDocumentController manages it
        openRecentMenu.perform(NSSelectorFromString("_setMenuName:"), with: "NSRecentDocumentsMenu")
        openRecentItem.submenu = openRecentMenu
        // Add Clear Menu item - NSDocumentController expects this
        let clearItem = NSMenuItem(title: "Clear Menu", action: #selector(NSDocumentController.clearRecentDocuments(_:)), keyEquivalent: "")
        openRecentMenu.addItem(clearItem)
        fileMenu.addItem(openRecentItem)

        fileMenu.addItem(NSMenuItem.separator())
        fileMenu.addItem(NSMenuItem(title: "Close", action: #selector(NSWindow.performClose(_:)), keyEquivalent: "w"))
        fileMenu.addItem(NSMenuItem(title: "Save…", action: #selector(NSDocument.save(_:)), keyEquivalent: "s"))

        let saveAsItem = NSMenuItem(title: "Save As…", action: NSSelectorFromString("saveDocumentAs:"), keyEquivalent: "S")
        saveAsItem.keyEquivalentModifierMask = [.command]
        fileMenu.addItem(saveAsItem)

        fileMenu.addItem(NSMenuItem(title: "Revert to Saved", action: NSSelectorFromString("revertDocumentToSaved:"), keyEquivalent: ""))

        fileMenu.addItem(NSMenuItem.separator())

        let pageSetupItem = NSMenuItem(title: "Page Setup…", action: #selector(NSDocument.runPageLayout(_:)), keyEquivalent: "P")
        pageSetupItem.keyEquivalentModifierMask = [.shift, .command]
        fileMenu.addItem(pageSetupItem)

        fileMenu.addItem(NSMenuItem(title: "Print…", action: #selector(NSDocument.printDocument(_:)), keyEquivalent: "p"))

        // MARK: - Edit menu
        let editMenuItem = NSMenuItem()
        mainMenu.addItem(editMenuItem)
        let editMenu = NSMenu(title: "Edit")
        editMenuItem.submenu = editMenu

        editMenu.addItem(NSMenuItem(title: "Undo", action: Selector(("undo:")), keyEquivalent: "z"))
        editMenu.addItem(NSMenuItem(title: "Redo", action: Selector(("redo:")), keyEquivalent: "Z"))
        editMenu.addItem(NSMenuItem.separator())

        // Add submenu
        let addSubmenuItem = NSMenuItem(title: "Add", action: nil, keyEquivalent: "")
        let addSubmenu = NSMenu(title: "Add")
        addSubmenuItem.submenu = addSubmenu
        addSubmenu.addItem(NSMenuItem(title: "New item", action: #selector(ViewController.addFreeItem(_:)), keyEquivalent: "n"))
        addSubmenu.addItem(NSMenuItem(title: "New Linked item", action: #selector(ViewController.addLinkedItem(_:)), keyEquivalent: ""))
        let linkedStyledItem = NSMenuItem(title: "Linked styled item", action: #selector(ViewController.addLinkedStyledItem(_:)), keyEquivalent: "\t")
        linkedStyledItem.keyEquivalentModifierMask = [.option]
        addSubmenu.addItem(linkedStyledItem)
        editMenu.addItem(addSubmenuItem)

        editMenu.addItem(NSMenuItem(title: "Duplicate", action: #selector(ViewController.duplicateItem(_:)), keyEquivalent: "d"))
        editMenu.addItem(NSMenuItem(title: "Inherit", action: #selector(ViewController.inheritItem(_:)), keyEquivalent: ""))

        let deleteItem = NSMenuItem(title: "Delete", action: NSSelectorFromString("delete:"), keyEquivalent: "\u{08}")
        deleteItem.keyEquivalentModifierMask = []
        editMenu.addItem(deleteItem)

        editMenu.addItem(NSMenuItem.separator())

        let showEditBoxItem = NSMenuItem(title: "Show edit box", action: #selector(ViewController.showOperationBox(_:)), keyEquivalent: " ")
        showEditBoxItem.keyEquivalentModifierMask = []
        editMenu.addItem(showEditBoxItem)

        let editTitleItem = NSMenuItem(title: "Edit title", action: #selector(ViewController.editTitle(_:)), keyEquivalent: "\r")
        editTitleItem.keyEquivalentModifierMask = []
        editMenu.addItem(editTitleItem)

        let editBodyItem = NSMenuItem(title: "Edit body", action: #selector(ViewController.editBody(_:)), keyEquivalent: "\r")
        editBodyItem.keyEquivalentModifierMask = [.shift]
        editMenu.addItem(editBodyItem)

        let editValueItem = NSMenuItem(title: "Edit value", action: #selector(ViewController.editValue(_:)), keyEquivalent: "\r")
        editValueItem.keyEquivalentModifierMask = [.option]
        editMenu.addItem(editValueItem)

        editMenu.addItem(NSMenuItem.separator())
        editMenu.addItem(NSMenuItem(title: "Cut", action: #selector(NSText.cut(_:)), keyEquivalent: "x"))
        editMenu.addItem(NSMenuItem(title: "Copy", action: #selector(NSText.copy(_:)), keyEquivalent: "c"))
        editMenu.addItem(NSMenuItem(title: "Paste", action: #selector(NSText.paste(_:)), keyEquivalent: "v"))
        editMenu.addItem(NSMenuItem(title: "Paste as Item", action: #selector(ViewController.pasteAsItem(_:)), keyEquivalent: ""))
        editMenu.addItem(NSMenuItem(title: "Paste as Item set", action: #selector(ViewController.pasteAsItemSet(_:)), keyEquivalent: ""))
        editMenu.addItem(NSMenuItem.separator())
        editMenu.addItem(NSMenuItem(title: "Select All", action: #selector(ViewController.selectAllItems(_:)), keyEquivalent: "a"))

        let selectAllItemsItem = NSMenuItem(title: "Select All Items", action: #selector(ViewController.selectAllItemsKind(_:)), keyEquivalent: "a")
        selectAllItemsItem.keyEquivalentModifierMask = [.shift, .command]
        editMenu.addItem(selectAllItemsItem)

        editMenu.addItem(NSMenuItem(title: "Select All Links", action: #selector(ViewController.selectAllLinks(_:)), keyEquivalent: ""))
        editMenu.addItem(NSMenuItem(title: "Select None", action: #selector(ViewController.selectNoneItems(_:)), keyEquivalent: ""))

        // MARK: - Navigate menu
        let navigateMenuItem = NSMenuItem()
        mainMenu.addItem(navigateMenuItem)
        let navigateMenu = NSMenu(title: "Navigate")
        navigateMenuItem.submenu = navigateMenu

        let gotoItem = NSMenuItem(title: "Goto Item", action: #selector(ViewController.showSearchBox(_:)), keyEquivalent: "r")
        gotoItem.keyEquivalentModifierMask = [.command]
        navigateMenu.addItem(gotoItem)

        // MARK: - View menu
        let viewMenuItem = NSMenuItem()
        mainMenu.addItem(viewMenuItem)
        let viewMenu = NSMenu(title: "View")
        viewMenuItem.submenu = viewMenu

        let fullScreenItem = NSMenuItem(title: "Enter Full Screen", action: #selector(NSWindow.toggleFullScreen(_:)), keyEquivalent: "f")
        fullScreenItem.keyEquivalentModifierMask = [.control, .command]
        viewMenu.addItem(fullScreenItem)

        viewMenu.addItem(NSMenuItem.separator())
        viewMenu.addItem(NSMenuItem(title: "Zoom In", action: #selector(SceneDrawView.zoomIn(_:)), keyEquivalent: "="))
        viewMenu.addItem(NSMenuItem(title: "Zoom Out", action: #selector(SceneDrawView.zoomOut(_:)), keyEquivalent: "-"))

        // MARK: - Window menu
        let windowMenuItem = NSMenuItem()
        mainMenu.addItem(windowMenuItem)
        let windowMenu = NSMenu(title: "Window")
        windowMenuItem.submenu = windowMenu

        windowMenu.addItem(NSMenuItem(title: "Minimize", action: #selector(NSWindow.performMiniaturize(_:)), keyEquivalent: "m"))
        windowMenu.addItem(NSMenuItem(title: "Zoom", action: #selector(NSWindow.performZoom(_:)), keyEquivalent: ""))
        windowMenu.addItem(NSMenuItem.separator())
        windowMenu.addItem(NSMenuItem(title: "Bring All to Front", action: #selector(NSApplication.arrangeInFront(_:)), keyEquivalent: ""))

        self.windowsMenu = windowMenu

        // MARK: - Help menu
        let helpMenuItem = NSMenuItem()
        mainMenu.addItem(helpMenuItem)
        let helpMenu = NSMenu(title: "Help")
        helpMenuItem.submenu = helpMenu

        let helpItem = NSMenuItem(title: "\(appName) Help", action: #selector(AppDelegate.showHelp(_:)), keyEquivalent: "?")
        helpMenu.addItem(helpItem)

        self.helpMenu = helpMenu

        self.mainMenu = mainMenu
    }

    override public func sendEvent(_ event: NSEvent) {
        if event.type == NSEvent.EventType.keyDown {
            if (event.modifierFlags.contains(NSEvent.ModifierFlags.command)) {
                if let chars = event.charactersIgnoringModifiers?.lowercased() {
                    switch chars {
                    case "x":
                        if NSApp.sendAction(#selector(NSText.cut(_:)), to:nil, from:self) { return }
                    case "c":
                        if NSApp.sendAction(#selector(NSText.copy(_:)), to:nil, from:self) { return }
                    case "v":
                        if NSApp.sendAction(#selector(NSText.paste(_:)), to:nil, from:self) { return }
                    case "a":
                        if NSApp.sendAction(#selector(NSText.selectAll(_:)), to:nil, from:self) { return }
                    default:
                        break
                    }
                }
            }
        }
        return super.sendEvent(event)
    }
}
