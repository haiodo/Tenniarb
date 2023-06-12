//
//  PreferencesController.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 21/03/2019.
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

let preferenceAutoExpand = "preferences.structure.auto_expand"
let preferenceExpandLevel = "preferences.structure.expand_level"

let preferenceColorsBackground = "preferences.colors.background"
let preferenceColorsBackgroundDark = "preferences.colors.background_dark"


let preferenceExportRenderBackground = "preferences.export.export_render_background"
let preferenceExportUseNativeScale = "preferences.structure.export_use_native_scale"

let renderEnableBackgroundKey = "preferences.render.enable_background"
let renderUseNativeResolutionKey = "preferences.render.enable_hidpi"

let windowPositionOption = "window.pos."

let preferenceUIQuickPanelOnTop = "preferences.ui.quick_panel_top"
let prererenceUITransparentBackground = "preferences.ui.transparent_background"

public class PreferenceConstants {
    public let backgroundDefault = CGColor(red: 0xe7/255, green: 0xe9/255, blue: 0xeb/255, alpha:1)
    public let backgroundDarkDefault = CGColor(red: 0x2e/255, green: 0x2e/255, blue: 0x2e/255, alpha:1)
    
    var backgroundColorCache: CGColor? = nil
    var backgroundColorDarkCache: CGColor? = nil
    
    var initDone = false
    var defaults: UserDefaults
    
    var darkMode = isDarkModeRaw()
    
    static func isDarkModeRaw() -> Bool {
        let isDarkMode: Bool
        
        if #available(macOS 10.14, *) {
            if NSAppearance.current.bestMatch(from: [.darkAqua, .aqua]) == .darkAqua {
                isDarkMode = true
            } else {
                isDarkMode = false
            }
        } else {
            isDarkMode = false
        }
        
        return isDarkMode
    }
    func isDiagramDarkMode() -> Bool {
        if !darkMode && getTextColorBasedOn(backgroundGet) == styleWhite {
            // Pseudo darm mode
            return true
        }
        if darkMode && getTextColorBasedOn(backgroundDarkGet) == styleBlack {
            // Pseudo white mode
            return false
        }
        
        // Check if background is dark enought we need to switch to dark mode.
        return darkMode
    }
    
    public var background: CGColor {
        get {
            if darkMode {
                return backgroundDarkGet
            }
            return backgroundGet
        }
    }
    
    public var backgroundGet: CGColor {
        get {
            if backgroundColorCache != nil {
                return backgroundColorCache!
            }
            guard let obj = defaults.string(forKey: preferenceColorsBackground) else {
                return backgroundDefault
            }
            self.backgroundColorCache =  parseColor(obj)
            return self.backgroundColorCache!
        }
    }
    
    public var backgroundDarkGet: CGColor {
        get {
            if backgroundColorDarkCache != nil {
                return backgroundColorDarkCache!
            }
            guard let obj = defaults.string(forKey: preferenceColorsBackgroundDark) else {
                return backgroundDarkDefault
            }
            self.backgroundColorDarkCache =  parseColor(obj)
            return self.backgroundColorDarkCache!
        }
    }
    
    public var autoExpand: Bool {
        get {
            return self.defaults.bool(forKey: preferenceAutoExpand)
        }
    }
    
    public var autoExpandLevel: Int {
        get {
            return self.defaults.integer(forKey: preferenceExpandLevel)
        }
    }
    public var renderEnableBackground: Bool {
        get {
            return self.defaults.bool(forKey: renderEnableBackgroundKey)
        }
    }
    
    public var renderUseNativeResolution: Bool {
        get {
            return self.defaults.bool(forKey: renderUseNativeResolutionKey)
        }
    }
    
    public var uiQuickPanelOnTop: Bool {
        get {
            return self.defaults.bool(forKey: preferenceUIQuickPanelOnTop)
        }
    }
    public var uiTransparentBackground: Bool {
        get {
            return self.defaults.bool(forKey: prererenceUITransparentBackground)
        }
    }
    
    init() {
        self.defaults = NSUserDefaultsController.shared.defaults
        
        checkDefaults()
        
        NotificationCenter.default.addObserver(self, selector: #selector(defaultsChanged), name: UserDefaults.didChangeNotification, object: nil)
        
        DistributedNotificationCenter.default().addObserver(self, selector: #selector(darkModeChanged), name: NSNotification.Name(rawValue: "AppleInterfaceThemeChangedNotification"), object: nil)    
    }
    
    @objc func defaultsChanged(_ notif: NSNotification) {
        self.backgroundColorCache = nil
        self.backgroundColorDarkCache = nil
    }
    @objc func darkModeChanged(_ notif: NSNotification) {
        self.darkMode = !self.darkMode
    }
        
    func checkDefaults() {
        if self.initDone {
            return
        }
        
        defaults.register(defaults: [
            preferenceAutoExpand: true,
            preferenceExpandLevel: 2,
            preferenceColorsBackground: colorToHex(backgroundDefault),
            preferenceColorsBackgroundDark: colorToHex( backgroundDarkDefault),
            
            preferenceExportRenderBackground: true,
            preferenceExportUseNativeScale: true,
            renderEnableBackgroundKey: false,
            renderUseNativeResolutionKey: true,
            
            preferenceUIQuickPanelOnTop: true,
            prererenceUITransparentBackground: false,
            ])
        defaults.synchronize()
        initDone = true
    }
    
    public static var preference = PreferenceConstants()
}


class PreferencesGeneralController: NSViewController {
    @IBOutlet weak var background: NSTextField!
    @IBOutlet weak var backgroundDark: NSTextField!
    @IBAction func resetBackground(_ sender: Any) {
        PreferenceConstants.preference.defaults.removeObject(forKey: preferenceColorsBackground)
        background.stringValue = colorToHex(PreferenceConstants.preference.backgroundGet)
    }
    
    @IBAction func resetDarkBackground(_ sender: Any) {
        PreferenceConstants.preference.defaults.removeObject(forKey: preferenceColorsBackgroundDark)
        backgroundDark.stringValue = colorToHex(PreferenceConstants.preference.backgroundDarkGet)
    }
    override func viewDidAppear() {
        super.viewDidAppear()
    }
    override func keyDown(with event: NSEvent) {
        if "\u{1B}" == event.characters {
            self.view.window?.close()
        }
    }
}

class PreferencesController: NSTabViewController {
    override func viewWillAppear() {
        super.viewWillAppear()
        
        self.view.window?.backingType = .buffered
        let window = view.window!
        let contentSize = CGSize(width: 350, height: 170)
        let newWindowSize = window.frameRect(forContentRect: CGRect(origin: .zero, size: contentSize)).size
        
        var frame = window.frame
        frame.origin.y += frame.height - newWindowSize.height
        frame.size = newWindowSize
        
        window.setFrame(frame, display: true)
    }
    
    override func keyDown(with event: NSEvent) {
        if "\u{1B}" == event.characters {
            self.view.window?.close()
        }
    }
    override func viewDidAppear() {
        self.view.window?.styleMask = [.docModalWindow, .closable, .titled]
    }
    
    private func setWindowFrame(for viewController: NSViewController) {
        let window = view.window!
        let key = viewController.title!
        let contentSize = key == "General" ? CGSize(width: 350, height: 170): CGSize(width: 260, height: 110)
        let newWindowSize = window.frameRect(forContentRect: CGRect(origin: .zero, size: contentSize)).size
        
        var frame = window.frame
        frame.origin.y += frame.height - newWindowSize.height
        frame.size = newWindowSize
        window.animator().setFrame(frame, display: true)
    }
    
    override func transition(from fromViewController: NSViewController, to toViewController: NSViewController, options: NSViewController.TransitionOptions = [], completionHandler completion: (() -> Void)? = nil) {
        
        NSAnimationContext.runAnimationGroup({ context in
            context.duration = 0.5
            setWindowFrame(for: toViewController)
            super.transition(from: fromViewController, to: toViewController, options: [.crossfade, .allowUserInteraction], completionHandler: completion)
        }, completionHandler: nil)
    }
}
