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

import Cocoa
import Foundation
import cmkdown

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
    private static let backgroundDefaultHex = "#e7e9ebff"
    private static let backgroundDarkDefaultHex = "#2e2e2eff"

    public var backgroundDefault: CGColor {
        return CGColor(red: 0xe7 / 255, green: 0xe9 / 255, blue: 0xeb / 255, alpha: 1)
    }
    public var backgroundDarkDefault: CGColor {
        return CGColor(red: 0x2e / 255, green: 0x2e / 255, blue: 0x2e / 255, alpha: 1)
    }

    var backgroundColorCache: CGColor? = nil
    var backgroundColorDarkCache: CGColor? = nil

    var initDone = false
    var isInitializing = false
    var defaults: UserDefaults

    nonisolated(unsafe) private static var _isInitializing = false
    public static var isInitializing: Bool {
        return _isInitializing
    }

    var darkMode: Bool = false

    static func isDarkModeRaw() -> Bool {
        let isDarkMode: Bool

        isDarkMode = NSAppearance.currentDrawing().bestMatch(from: [.darkAqua, .aqua]) == .darkAqua

        return isDarkMode
    }

    func isDiagramDarkMode() -> Bool {
        if !darkMode && getTextColorBasedOn(backgroundGet) == styleWhite {
            return true
        }
        if darkMode && getTextColorBasedOn(backgroundDarkGet) == styleBlack {
            return false
        }
        return darkMode
    }

    public var background: CGColor {
        get {
            if backgroundColorCache == nil {
                if let str = defaults.string(forKey: preferenceColorsBackground),
                    let clr = parseColor(str)
                {
                    backgroundColorCache = clr
                } else {
                    backgroundColorCache = backgroundDefault
                }
            }
            return backgroundColorCache!
        }
        set {
            backgroundColorCache = newValue
            defaults.set(colorToHex(newValue), forKey: preferenceColorsBackground)
        }
    }

    public var backgroundGet: CGColor {
        if backgroundColorCache == nil {
            if let str = defaults.string(forKey: preferenceColorsBackground),
                let clr = parseColor(str)
            {
                backgroundColorCache = clr
            } else {
                backgroundColorCache = backgroundDefault
            }
        }
        return backgroundColorCache!
    }

    public var backgroundDark: CGColor {
        get {
            if backgroundColorDarkCache == nil {
                if let str = defaults.string(forKey: preferenceColorsBackgroundDark),
                    let clr = parseColor(str)
                {
                    backgroundColorDarkCache = clr
                } else {
                    backgroundColorDarkCache = backgroundDarkDefault
                }
            }
            return backgroundColorDarkCache!
        }
        set {
            backgroundColorDarkCache = newValue
            defaults.set(colorToHex(newValue), forKey: preferenceColorsBackgroundDark)
        }
    }

    public var backgroundDarkGet: CGColor {
        if backgroundColorDarkCache == nil {
            if let str = defaults.string(forKey: preferenceColorsBackgroundDark),
                let clr = parseColor(str)
            {
                backgroundColorDarkCache = clr
            } else {
                backgroundColorDarkCache = backgroundDarkDefault
            }
        }
        return backgroundColorDarkCache!
    }

    public func getBackground() -> CGColor {
        return darkMode ? backgroundDark : background
    }

    public var autoExpand: Bool {
        return defaults.bool(forKey: preferenceAutoExpand)
    }

    public var autoExpandLevel: Int {
        return defaults.integer(forKey: preferenceExpandLevel)
    }

    public var renderEnableBackground: Bool {
        return defaults.bool(forKey: renderEnableBackgroundKey)
    }

    public var renderUseNativeResolution: Bool {
        return defaults.bool(forKey: renderUseNativeResolutionKey)
    }

    public var uiQuickPanelOnTop: Bool {
        return defaults.bool(forKey: preferenceUIQuickPanelOnTop)
    }

    public var uiTransparentBackground: Bool {
        return defaults.bool(forKey: prererenceUITransparentBackground)
    }

    nonisolated(unsafe) private static var _preference: PreferenceConstants?
    private static let _lock = NSLock()

    init() {
        self.defaults = NSUserDefaultsController.shared.defaults

        self.isInitializing = true
        PreferenceConstants._isInitializing = true

        checkDefaults()

        self.isInitializing = false
        PreferenceConstants._isInitializing = false

        DistributedNotificationCenter.default.addObserver(
            self,
            selector: #selector(interfaceModeChanged(sender:)),
            name: NSNotification.Name(rawValue: "AppleInterfaceThemeChangedNotification"),
            object: nil)

        self.initDone = true
        self.darkMode = PreferenceConstants.isDarkModeRaw()
    }

    @objc func interfaceModeChanged(sender: NSNotification) {
        darkMode = PreferenceConstants.isDarkModeRaw()
    }

    public func checkDefaults() {
        let center = NotificationCenter.default
        let removedObservers: [(NSObjectProtocol, Notification.Name)] = []

        defaults.register(defaults: [
            preferenceAutoExpand: true,
            preferenceExpandLevel: 2,
            preferenceColorsBackground: PreferenceConstants.backgroundDefaultHex,
            preferenceColorsBackgroundDark: PreferenceConstants.backgroundDarkDefaultHex,
            preferenceExportRenderBackground: false,
            preferenceExportUseNativeScale: true,
            renderEnableBackgroundKey: true,
            renderUseNativeResolutionKey: true,
            preferenceUIQuickPanelOnTop: true,
            prererenceUITransparentBackground: false,
        ])

        for (observer, name) in removedObservers {
            center.addObserver(observer, selector: #selector(handleNotification(_:)), name: name, object: nil)
        }
    }

    @objc private func handleNotification(_ notification: Notification) {
    }

    public static var preference: PreferenceConstants {
        _lock.lock()
        defer { _lock.unlock() }

        if _preference == nil {
            _preference = PreferenceConstants()
        }
        return _preference!
    }
}

func colorToHex(_ color: CGColor) -> String {
    if !Thread.isMainThread {
        return colorToHexSafe(color)
    }

    guard let nsColor = NSColor(cgColor: color) else {
        return "#000000ff"
    }

    var r: CGFloat = 0
    var g: CGFloat = 0
    var b: CGFloat = 0
    var a: CGFloat = 1.0

    if let rgbColor = nsColor.usingColorSpace(.deviceRGB) {
        rgbColor.getRed(&r, green: &g, blue: &b, alpha: &a)
    } else {
        r = nsColor.redComponent
        g = r
        b = r
    }

    let rInt = Int(round(r * 255))
    let gInt = Int(round(g * 255))
    let bInt = Int(round(b * 255))
    let aInt = Int(round(a * 255))

    return String(format: "#%02x%02x%02x%02x", rInt, gInt, bInt, aInt)
}

func colorToHexSafe(_ color: CGColor) -> String {
    guard let colorSpace = color.colorSpace,
        colorSpace.model == .rgb,
        let components = color.components,
        components.count >= 3
    else {
        return "#000000ff"
    }

    let r = Int(round(components[0] * 255))
    let g = Int(round(components[1] * 255))
    let b = Int(round(components[2] * 255))
    let a = components.count >= 4 ? Int(round(components[3] * 255)) : 255

    return String(format: "#%02x%02x%02x%02x", r, g, b, a)
}

func parseColor(_ hex: String, alpha: CGFloat = 1.0) -> CGColor? {
    var hexSanitized = hex.trimmingCharacters(in: .whitespacesAndNewlines)
    hexSanitized = hexSanitized.replacingOccurrences(of: "#", with: "")

    if let namedHex = ColorNames[hexSanitized.lowercased()] {
        hexSanitized = namedHex.replacingOccurrences(of: "#", with: "")
    }

    var rgb: UInt64 = 0
    guard Scanner(string: hexSanitized).scanHexInt64(&rgb) else {
        return nil
    }

    let r: CGFloat
    let g: CGFloat
    let b: CGFloat

    if hexSanitized.count == 6 {
        r = CGFloat((rgb & 0xFF0000) >> 16) / 255.0
        g = CGFloat((rgb & 0x00FF00) >> 8) / 255.0
        b = CGFloat(rgb & 0x0000FF) / 255.0
    } else if hexSanitized.count == 8 {
        r = CGFloat((rgb & 0xFF000000) >> 24) / 255.0
        g = CGFloat((rgb & 0x00FF0000) >> 16) / 255.0
        b = CGFloat((rgb & 0x0000FF00) >> 8) / 255.0
        return CGColor(red: r, green: g, blue: b, alpha: CGFloat(rgb & 0x000000FF) / 255.0)
    } else {
        return nil
    }

    return CGColor(red: r, green: g, blue: b, alpha: alpha)
}

// MARK: - PreferencesGeneralController

class PreferencesGeneralController: NSViewController {
    var backgroundField: NSTextField!
    var backgroundDarkField: NSTextField!
    var expandLevelField: NSTextField!
    var autoExpandButton: NSButton!
    var renderEnableButton: NSButton!
    var renderNativeScaleButton: NSButton!
    var uiQuickPanelButton: NSButton!
    var uiTransparentButton: NSButton!

    override func loadView() {
        // Two-column layout matching original storyboard: 580x257
        let root = NSView(frame: NSRect(x: 0, y: 0, width: 500, height: 200))
        self.view = root

        let leftX: CGFloat = 20
        let rightX: CGFloat = 270

        // ===== LEFT COLUMN =====

        // --- Outline Section ---
        let outlineLabel = NSTextField(labelWithString: "Outline")
        outlineLabel.font = NSFont.boldSystemFont(ofSize: 12)
        outlineLabel.frame = NSRect(x: leftX, y: 165, width: 100, height: 16)
        root.addSubview(outlineLabel)

        autoExpandButton = NSButton(checkboxWithTitle: "Structure Auto expand", target: self, action: #selector(autoExpandChanged(_:)))
        autoExpandButton.frame = NSRect(x: leftX + 10, y: 140, width: 200, height: 18)
        root.addSubview(autoExpandButton)

        let expandLabel = NSTextField(labelWithString: "Expand to level:")
        expandLabel.frame = NSRect(x: leftX + 10, y: 115, width: 100, height: 16)
        root.addSubview(expandLabel)

        expandLevelField = NSTextField(string: "2")
        expandLevelField.frame = NSRect(x: leftX + 115, y: 113, width: 40, height: 20)
        let numFormatter = NumberFormatter()
        numFormatter.numberStyle = .decimal
        numFormatter.maximumFractionDigits = 0
        expandLevelField.formatter = numFormatter
        expandLevelField.target = self
        expandLevelField.action = #selector(expandLevelChanged(_:))
        root.addSubview(expandLevelField)

        // --- Background Section ---
        let backgroundLabel = NSTextField(labelWithString: "Background")
        backgroundLabel.font = NSFont.boldSystemFont(ofSize: 12)
        backgroundLabel.frame = NSRect(x: leftX, y: 85, width: 100, height: 16)
        root.addSubview(backgroundLabel)

        let whiteLabel = NSTextField(labelWithString: "White")
        whiteLabel.frame = NSRect(x: leftX + 10, y: 60, width: 40, height: 16)
        root.addSubview(whiteLabel)

        backgroundField = NSTextField(string: "")
        backgroundField.frame = NSRect(x: leftX + 55, y: 58, width: 90, height: 20)
        backgroundField.target = self
        backgroundField.action = #selector(backgroundChanged(_:))
        backgroundField.cell?.sendsActionOnEndEditing = true
        root.addSubview(backgroundField)

        let resetButton = NSButton(title: "Reset", target: self, action: #selector(resetBackground(_:)))
        resetButton.frame = NSRect(x: leftX + 150, y: 56, width: 55, height: 22)
        resetButton.bezelStyle = .rounded
        resetButton.controlSize = .small
        root.addSubview(resetButton)

        let darkLabel = NSTextField(labelWithString: "Dark")
        darkLabel.frame = NSRect(x: leftX + 10, y: 32, width: 40, height: 16)
        root.addSubview(darkLabel)

        backgroundDarkField = NSTextField(string: "")
        backgroundDarkField.frame = NSRect(x: leftX + 55, y: 30, width: 90, height: 20)
        backgroundDarkField.target = self
        backgroundDarkField.action = #selector(backgroundDarkChanged(_:))
        backgroundDarkField.cell?.sendsActionOnEndEditing = true
        root.addSubview(backgroundDarkField)

        let resetDarkButton = NSButton(title: "Reset", target: self, action: #selector(resetDarkBackground(_:)))
        resetDarkButton.frame = NSRect(x: leftX + 150, y: 28, width: 55, height: 22)
        resetDarkButton.bezelStyle = .rounded
        resetDarkButton.controlSize = .small
        root.addSubview(resetDarkButton)

        // ===== RIGHT COLUMN =====

        // --- Export Section ---
        let exportLabel = NSTextField(labelWithString: "Export")
        exportLabel.font = NSFont.boldSystemFont(ofSize: 12)
        exportLabel.frame = NSRect(x: rightX, y: 165, width: 100, height: 16)
        root.addSubview(exportLabel)

        renderEnableButton = NSButton(checkboxWithTitle: "Render background", target: self, action: #selector(renderEnableChanged(_:)))
        renderEnableButton.frame = NSRect(x: rightX + 10, y: 140, width: 180, height: 18)
        root.addSubview(renderEnableButton)

        renderNativeScaleButton = NSButton(
            checkboxWithTitle: "Use Native resolution", target: self, action: #selector(renderNativeScaleChanged(_:)))
        renderNativeScaleButton.frame = NSRect(x: rightX + 10, y: 118, width: 180, height: 18)
        root.addSubview(renderNativeScaleButton)

        // --- UI Section ---
        let uiLabel = NSTextField(labelWithString: "UI")
        uiLabel.font = NSFont.boldSystemFont(ofSize: 12)
        uiLabel.frame = NSRect(x: rightX, y: 85, width: 100, height: 16)
        root.addSubview(uiLabel)

        uiQuickPanelButton = NSButton(checkboxWithTitle: "Show popup quick panel", target: self, action: #selector(uiQuickPanelChanged(_:)))
        uiQuickPanelButton.frame = NSRect(x: rightX + 10, y: 60, width: 180, height: 18)
        root.addSubview(uiQuickPanelButton)

        uiTransparentButton = NSButton(
            checkboxWithTitle: "Transparent background", target: self, action: #selector(uiTransparentChanged(_:)))
        uiTransparentButton.frame = NSRect(x: rightX + 10, y: 38, width: 180, height: 18)
        root.addSubview(uiTransparentButton)
    }

    override func viewDidLoad() {
        super.viewDidLoad()
        loadPreferences()
    }

    func loadPreferences() {
        let defaults = UserDefaults.standard

        autoExpandButton.state = defaults.bool(forKey: preferenceAutoExpand) ? .on : .off
        expandLevelField.integerValue = defaults.integer(forKey: preferenceExpandLevel)

        backgroundField.stringValue = colorToHex(PreferenceConstants.preference.backgroundGet)
        backgroundDarkField.stringValue = colorToHex(PreferenceConstants.preference.backgroundDarkGet)

        renderEnableButton.state = defaults.bool(forKey: renderEnableBackgroundKey) ? .on : .off
        renderNativeScaleButton.state = defaults.bool(forKey: renderUseNativeResolutionKey) ? .on : .off

        uiQuickPanelButton.state = defaults.bool(forKey: preferenceUIQuickPanelOnTop) ? .on : .off
        uiTransparentButton.state = defaults.bool(forKey: prererenceUITransparentBackground) ? .on : .off
    }

    @objc func autoExpandChanged(_ sender: NSButton) {
        UserDefaults.standard.set(sender.state == .on, forKey: preferenceAutoExpand)
    }

    @objc func expandLevelChanged(_ sender: NSTextField) {
        UserDefaults.standard.set(sender.integerValue, forKey: preferenceExpandLevel)
    }

    @objc func backgroundChanged(_ sender: NSTextField) {
        if let color = parseColor(sender.stringValue) {
            PreferenceConstants.preference.background = color
        }
    }

    @objc func backgroundDarkChanged(_ sender: NSTextField) {
        if let color = parseColor(sender.stringValue) {
            PreferenceConstants.preference.backgroundDark = color
        }
    }

    @objc func resetBackground(_ sender: NSButton) {
        PreferenceConstants.preference.defaults.removeObject(forKey: preferenceColorsBackground)
        PreferenceConstants.preference.backgroundColorCache = nil
        backgroundField.stringValue = colorToHex(PreferenceConstants.preference.backgroundGet)
    }

    @objc func resetDarkBackground(_ sender: NSButton) {
        PreferenceConstants.preference.defaults.removeObject(forKey: preferenceColorsBackgroundDark)
        PreferenceConstants.preference.backgroundColorDarkCache = nil
        backgroundDarkField.stringValue = colorToHex(PreferenceConstants.preference.backgroundDarkGet)
    }

    @objc func renderEnableChanged(_ sender: NSButton) {
        UserDefaults.standard.set(sender.state == .on, forKey: renderEnableBackgroundKey)
    }

    @objc func renderNativeScaleChanged(_ sender: NSButton) {
        UserDefaults.standard.set(sender.state == .on, forKey: renderUseNativeResolutionKey)
    }

    @objc func uiQuickPanelChanged(_ sender: NSButton) {
        UserDefaults.standard.set(sender.state == .on, forKey: preferenceUIQuickPanelOnTop)
    }

    @objc func uiTransparentChanged(_ sender: NSButton) {
        UserDefaults.standard.set(sender.state == .on, forKey: prererenceUITransparentBackground)
    }

    override func keyDown(with event: NSEvent) {
        if "\u{1B}" == event.characters {
            self.view.window?.close()
        }
    }
}

// MARK: - PreferencesController

class PreferencesController: NSTabViewController {
    override func viewDidLoad() {
        super.viewDidLoad()
        // Hide the tab bar since we only have one tab
        tabStyle = .unspecified
    }

    override func viewWillAppear() {
        super.viewWillAppear()

        self.view.window?.backingType = .buffered
        let window = view.window!
        let contentSize = CGSize(width: 500, height: 200)
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
        self.view.window?.styleMask = [.titled, .closable]
    }

    private func setWindowFrame(for viewController: NSViewController) {
        let window = view.window!
        let contentSize = CGSize(width: 500, height: 200)
        let newWindowSize = window.frameRect(forContentRect: CGRect(origin: .zero, size: contentSize)).size

        var frame = window.frame
        frame.origin.y += frame.height - newWindowSize.height
        frame.size = newWindowSize
        window.animator().setFrame(frame, display: true)
    }

    override func transition(
        from fromViewController: NSViewController, to toViewController: NSViewController, options: NSViewController.TransitionOptions = [],
        completionHandler completion: (() -> Void)? = nil
    ) {
        // AppKit invokes the handler on the main thread; the wrapper only satisfies @Sendable checking.
        nonisolated(unsafe) let completion = completion
        let sendableCompletion: @Sendable () -> Void = {
            completion?()
        }
        NSAnimationContext.runAnimationGroup(
            { context in
                context.duration = 0.5
                setWindowFrame(for: toViewController)
                super.transition(
                    from: fromViewController, to: toViewController, options: [.crossfade, .allowUserInteraction],
                    completionHandler: sendableCompletion)
            }, completionHandler: nil)
    }
}
