//
//  SceneDrawView.swift
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
import CoreText
import CoreImage


enum SceneMode {
    case Normal // A usual selection mode
    case Editing  // Title editing mode
    case Dragging // Dragging mode
    case DiagramMove
    case LineDrawing // Line dragging mode
    case Selection  // Selection with shift key
}

enum EditingMode {
    case Name // edvarng of name/title
    case Body // editing of body, shift + enter
    case Value // A custom edit field selection
}

public class NSOptionsPopup: NSView {
    public override func draw(_ dirtyRect: NSRect) {
        // Do not Draw background
    }
}

public class PopupEditField: NSTextField {
    override init(frame frameRect: NSRect) {
        super.init(frame: frameRect)
    }
    
    var shiftKeyDown = false
    
    required init?(coder: NSCoder) {
        fatalError("init(coder:) has not been implemented")
    }
    
    public override func selectAll(_ sender: Any?) {
    }
    
    override public func keyDown(with event: NSEvent) {
        if event.modifierFlags.contains(NSEvent.ModifierFlags.shift) {
            shiftKeyDown = true
        }
        super.keyDown(with: event)
    }
    //    override public func keyUp(with event: NSEvent) {
    //        super.keyDown(with: event)
    //    }
    public override func flagsChanged(with event: NSEvent) {
        if event.modifierFlags.contains(NSEvent.ModifierFlags.shift) {
            shiftKeyDown = true
        } else {
            shiftKeyDown = false
        }
        super.flagsChanged(with: event)
    }
}

class EditTitleDelegate: NSObject, NSTextFieldDelegate, NSTextDelegate {
    var view: SceneDrawView
    init( _ view: SceneDrawView) {
        self.view = view
    }
    
    func control(_ control: NSControl, textView: NSTextView, doCommandBy commandSelector: Selector) -> Bool {
        if commandSelector == #selector(NSView.cancelOperation(_:)) {
            self.view.commitTitleEditing(nil)
            return true
        }
        if commandSelector == #selector(NSView.insertNewline(_:)) {
            if self.view.editBox?.shiftKeyDown ?? false {
                // Just shift click
                let loc = textView.selectedRange().location
                let insertPart = "\n"
                
                let str = NSAttributedString(
                    string:insertPart,
                    attributes:[NSAttributedString.Key.font: textView.font ?? NSFont.systemFont(ofSize: 12)]
                )
                textView.textStorage?.insert(str, at: loc)
                return true
            }
            self.view.commitTitleEditing(textView)
            return true
        }
        // TODO: Resize both text and drawed item to fit value smoothly.
        
        return false
    }
}


class SceneDrawView: NSView, IElementModelListener, NSMenuItemValidation {
    //    let background = CGColor(red: 253/255, green: 246/255, blue: 227/255, alpha:1)
    var store: ElementModelStore?
    
    var element: Element?
    
    var activeItems: [DiagramItem] = []
    
    var dragElements: [DiagramItem] = []
    
    var dragMap:[DiagramItem: CGPoint] = [:]
    
    var lineToPoint: CGPoint?
    var lineTarget: DiagramItem?
    
    var createIndex: Int = 1
    
    var x: CGFloat = 0
    var y: CGFloat = 0
    
    var mode: SceneMode = .Normal
    
    var editBox: PopupEditField? = nil
    var editBoxItem: Drawable? = nil
    var editBoxDelegate: EditTitleDelegate?
    
    var pivotPoint: CGPoint = CGPoint(x:0, y:0)
    
    var styleManager: StyleManager?
    
    var selectionStart: CGPoint = CGPoint(x:0, y:0)
    
    var editingMode: EditingMode = .Name
    
    var clickCounter = 0
    
    var viewController: ViewController?
    
    var zoomLevel: CGFloat = 1
    
    var ox: CGFloat {
        set {
            if let active = element {
                active.ox = Double(newValue)
            }
        }
        get {
            if let active = element {
                return CGFloat(active.ox)
            }
            return 0
        }
    }
    var oy: CGFloat {
        set {
            if let active = element {
                active.oy = Double(newValue)
            }
        }
        get {
            if let active = element {
                return CGFloat(active.oy)
            }
            return 0
        }
    }
    
    var mouseDownState = false
    
    var onSelection: [( [DiagramItem] ) -> Void] = []
    
    var scene: DrawableScene?
    
    var drawScheduled: Bool = false
    
    var lastInvalidRect: CGRect? = nil
    
    required init?(coder aDecoder: NSCoder) {
        super.init(coder: aDecoder)
        self.allowedTouchTypes = [.direct, .indirect]
    }
    
    var prevTouch: NSTouch? = nil
    
    var popupView: NSView?
    var popupItem: DiagramItem?
    var scaleFactor: CGFloat = 1
    
    @objc override func touchesBegan(with event: NSEvent) {
        let wloc = event.locationInWindow
        let vp = self.convert(wloc, from: nil)
        
        if self.mode == .Editing || self.mode == .LineDrawing || self.mode == .Dragging || self.mode == .DiagramMove {
            return
        }
        let touches = event.touches(matching: NSTouch.Phase.touching, in: self)
        if touches.count == 2 && self.bounds.contains(vp) {
            prevTouch = touches.first
        }
    }
    
    func scheduleRedraw() {
        scheduleRedraw(invalidRect: nil)
    }
    
    fileprivate func scheduleRedraw( invalidRect: CGRect? ) {
        if !self.drawScheduled {
            drawScheduled = true
            lastInvalidRect = invalidRect
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.01, execute: {
                if let rect = invalidRect {
                    self.setNeedsDisplay(rect)
                    self.lastInvalidRect = nil
                }
                else {
                    self.needsDisplay = true
                }
                self.drawScheduled = false
            })
        }
        else {
            lastInvalidRect = nil
        }
    }
    
    override func scrollWheel(with event: NSEvent) {        
        if self.mode == .Editing || self.mode == .LineDrawing || self.mode == .Dragging {
            return
        }
        
        hidePopup()
                    
        let dx = event.deltaX * 3
        let dy = event.deltaY * 3
        self.ox += dx
        self.oy -= dy
        
        scheduleRedraw()
        
    }
    
    @objc override func touchesMoved(with event: NSEvent) {
        if self.mode == .Editing || self.mode == .LineDrawing || self.mode == .Dragging {
            return
        }
        let wloc = event.locationInWindow
        let vp = self.convert(wloc, from: nil)
        
        let touches = event.touches(matching: NSTouch.Phase.touching, in: self)
        if touches.count == 2 && self.bounds.contains(vp) {
            hidePopup()
            if prevTouch == nil {
                prevTouch = touches.first
                return
            }
            var touch: NSTouch? = nil
            var diffTouch: NSTouch? = nil
            for t in touches {
                if t.identity.isEqual(prevTouch?.identity) {
                    touch = t
                    break
                } else {
                    diffTouch = t
                }
            }
            if touch != nil {
                let np1 = prevTouch!.normalizedPosition
                let np2 = touch!.normalizedPosition
                
                let dx = (np2.x-np1.x)*prevTouch!.deviceSize.width*3
                let dy = (np2.y-np1.y)*prevTouch!.deviceSize.height*3
                self.ox += dx
                self.oy += dy
                
                scheduleRedraw()
            }
            if touch != nil {
                prevTouch = touch
            } else {
                prevTouch = diffTouch
            }
        }
    }
    
    @objc override func touchesEnded(with event: NSEvent) {
        prevTouch = nil
    }
    
    override var mouseDownCanMoveWindow: Bool {
        get {
            return false
        }
    }
    
    func onLoad(_ vc: ViewController ) {
        styleManager = StyleManager(scene: self)
        self.viewController = vc
        
        if let win = vc.view.window {
            self.scaleFactor = win.backingScaleFactor
        }
        
        NotificationCenter.default.addObserver(self, selector: #selector(defaultsChanged), name: UserDefaults.didChangeNotification, object: nil)        
    }
    
    func onAppear( ) {
        if let win = self.window {
            self.scaleFactor = win.backingScaleFactor
        }
    }
    
    @objc func defaultsChanged(_ notif: NSNotification) {
        if self.element != nil {
            buildScene()
            scheduleRedraw()
        }
    }
    
    
    func notifyChanges(_ evt: ModelEvent) {
        
        // We should be smart anought to not rebuild all drawable scene every time
        if evt.items.count > 0 {
            var removedItems: [DiagramItem] = []
            for (k, v) in evt.items {
                if v == .Remove  {
                    if self.activeItems.contains(k) {
                        removedItems.append(k)
                    }
                }
            }
            if removedItems.count > 0 {
                let newActive = self.activeItems.filter({ e in !removedItems.contains(e) })
                setActiveItems(newActive, force: true)
            }
        }
        if evt.kind == .Structure  {
            self.buildScene()
            scheduleRedraw()
        }
        else {
            scheduleRedraw()
        }
    }
    
    public func setModel( store: ElementModelStore ) {
        if let um = self.undoManager {
            um.removeAllActions()
        }
        
        if self.store?.model != store.model {
            self.store = store
            self.store?.onUpdate.append( self )
        }
        zoomLevel = 1
    }
    
    public func setActiveElement(_ elementModel: Element ) {
        
        if self.element == elementModel {
            return
        }
        if self.popupView != nil {
            self.popupView?.removeFromSuperview()
            self.popupView = nil
        }
        // Discard any editing during switch
        self.commitTitleEditing(nil)
        
        self.element = elementModel
        self.activeItems.removeAll()
        self.store?.executionContext.setScaleFactor(self.scaleFactor)
        // Center diagram to fit all items
        self.store?.executionContext.setElement(elementModel)
        
        if self.scene != nil {
            self.scene?.activeElements.removeAll()
            self.scene?.editingMode = false
        }
        self.buildScene()
        
        if let bounds = scene?.getBounds() {
            self.ox = -1 * bounds.midX
            self.oy = -1 * bounds.midY
        }
        self.pivotPoint = CGPoint(x:0, y:0)
        zoomLevel = 1
        needsDisplay = true
    }
    
    func centerItem( _ item: DiagramItem, _ offset: CGFloat ) {
        
        if let dr = self.scene?.drawables[item] {
            let bounds = dr.getBounds()
            self.ox = -1 * bounds.midX
            self.oy = -1 * bounds.midY - offset
        }
    }
    
    var count = 0;
    func animate() {
        if let fps = element?.properties.get("animation"), let delay = fps.getFloat(1) {
            //Swift.debugPrint("Animate: \(delay) count:\(count)")
            count += 1
            if self.mode == .Normal {
                self.store!.executionContext.updateAll({
                    DispatchQueue.main.asyncAfter(deadline: DispatchTime.now() + Double(delay), execute: {
                        self.buildScene()
                        self.needsDisplay = true
                    })
                })
            } else {
                DispatchQueue.main.asyncAfter(deadline: DispatchTime.now() + Double(delay), execute: self.animate)
            }
        }
    }
    private func buildScene() {
        // We need preserve selection of previous scene
        
        if let vc = self.viewController, let win = vc.view.window {
            self.scaleFactor = win.backingScaleFactor
        }
        
        var oldActiveItem: [DiagramItem] = []
        var oldEditMode: Bool = false
        if let oldScene = self.scene {
            oldActiveItem = oldScene.activeElements
            oldEditMode = oldScene.editingMode
        }
        
        let darkMode = PreferenceConstants.preference.isDiagramDarkMode()
        
        let scene = DrawableScene(self.element!, darkMode: darkMode, executionContext: self.store!.executionContext,
                                  scaleFactor: self.scaleFactor)
        
        if oldActiveItem.count > 0 {
            scene.updateActiveElements(oldActiveItem)
            scene.editingMode = oldEditMode
        }
        
        
        if let fps = element?.properties.get("animation"), let delay = fps.getFloat(1) {
            DispatchQueue.main.asyncAfter(deadline: DispatchTime.now() + Double(delay), execute: self.animate)
        }
        
        self.scene = scene
    }
    
    public func setActiveItem( _ element: DiagramItem?, immideateDraw: Bool = false ) {
        var els: [DiagramItem] = []
        if let act = element {
            els.append(act)
        }
        
        self.setActiveItems(els, immideateDraw: immideateDraw)
    }
    
    func changeItemProps( _ property: String, _ values: TennNode...) {
        self.changeItemProps(property, values)
    }
    func changeItemProps( _ property: String, _ values: [TennNode]) {
        guard let itm = self.activeItems.first, activeItems.count == 1 else {
            return
        }
        
        let newItemProps = itm.toTennAsProps(.BlockExpr)
        var changed = false
        if let itmProp = newItemProps.getNamedElement(property) {
            // Property exists, we need to replace value
            itmProp.children?.removeAll()
            itmProp.add(TennNode.newIdent(property))
            itmProp.add(values)
            changed = true
        }
        else {
            // Just add new property
            let cmd = TennNode.newCommand(property)
            cmd.add(values)
            newItemProps.add(cmd)
            changed = true
        }
        
        if changed {
            self.store?.setProperties(self.element!, itm, newItemProps, undoManager: self.undoManager, refresh: self.scheduleRedraw)
        }
    }
    
    let itemDisplayVariants = ["■ rect", "□ no-fill", "● circle", "❐ stack", "≣ text"]
    let linkDisplayVariants = ["– solid","→ arrow", "↔︎ arrows", "← arrow-source"]
    let lineStyleDisplayVariants = ["– solid", "⤍ dashed", "⤑ dotted"]
    
    @objc func displayMenuAction( _ sender: NSMenuItem ) {
        let val = sender.title
        let value = val.suffix(from: val.index(val.startIndex, offsetBy: 2))
        changeItemProps("display", TennNode.newIdent(String(value)))
    }
    
    func createMenu( selector: Selector, items: [String]) -> NSMenu {
        let menu = NSMenu()
        for i in items {
            menu.addItem(NSMenuItem(title: i, action: selector, keyEquivalent: ""))
        }
        return menu
    }
    @objc func fontMenuAction( _ sender: NSMenuItem ) {
        let value = sender.title
        changeItemProps("font-size", TennNode.newIdent(value))
    }
    
    @objc func layoutMenuAction( _ sender: NSMenuItem ) {
        let value = sender.title
        let svalue = value.split(separator: " ")
        let vars = svalue.map( { (e) in return TennNode.newIdent(String(e)) } )
        changeItemProps("layout", vars)
    }
    
    @objc func bodyFontMenuAction( _ sender: NSMenuItem ) {
        let value = sender.title
        changeItemProps("font-size", TennNode.newIdent(value))
    }
    
    @objc func markerMenuAction( _ sender: NSMenuItem ) {
        let value = sender.title
        changeItemProps("marker", TennNode.newStrNode(value))
    }
    @objc func lineWidthAction( _ sender: NSMenuItem ) {
        let value = sender.title
        changeItemProps("line-width", TennNode.newIdent(value))
    }
        
    @objc func heightAction( _ sender: NSMenuItem ) {
        let value = sender.title
        changeItemProps("height", TennNode.newIdent(value))
    }
    
    @objc func commandAction( _ sender: NSMenuItem ) {
        let value = sender.title
        var split = value.split(separator: " ")
        if split.count >= 2 {
            let cmd = split.removeFirst()
            let nodes = split.map({(it) -> TennNode in TennNode.newIdent(String(it))})
            changeItemProps(String(cmd), nodes)
        }
    }
    
    @objc func colorMenuAction( _ sender: NSMenuItem ) {
        let value = String(sender.title.dropFirst(1))
        changeItemProps("color", TennNode.newIdent(value))
    }
    @objc func borderColorMenuAction( _ sender: NSMenuItem ) {
        let value = String(sender.title.dropFirst(1))
        changeItemProps("border-color", TennNode.newIdent(value))
    }
    @objc func textColorMenuAction( _ sender: NSMenuItem ) {
        let value = String(sender.title.dropFirst(1))
        changeItemProps("text-color", TennNode.newIdent(value))
    }
    
    @objc func lineStyleMenuAction( _ sender: NSMenuItem ) {
        let val = sender.title
        let value = val.suffix(from: val.index(val.startIndex, offsetBy: 2))
        changeItemProps("line-style", TennNode.newIdent(String(value)))
    }
    
    @objc func segmentAction(_ sender: NSSegmentedCell) {
        guard let act = self.activeItems.first, activeItems.count == 1, let popup = self.popupView else {
            return
        }
        
        if act.kind == .Item {
            switch sender.selectedSegment {
            case 0:
                self.addNewItem()
                return
            case 3:
                self.removeItem()
            default:
                break
            }
        } else {
            switch sender.selectedSegment {            
            case 1:
                self.removeItem()
            default:
                break
            }
        }
        // If there is menu defined
        var menuOrigin = popup.frame.origin
        for i in 0..<sender.selectedSegment {
            menuOrigin.x += sender.width(forSegment: i)
        }
        menuOrigin.y -= 2
        if let menu = sender.menu(forSegment: sender.selectedSegment) {
            menu.popUp(positioning: nil, at: menuOrigin, in: self)
        }
    }
    
    fileprivate func createMarkerMenus() -> NSMenu {
        let menu = NSMenu()
        let smiles = menu.addItem(withTitle: "😀 Emoji", action: nil, keyEquivalent: "")
        smiles.submenu = createMenu(selector: #selector(markerMenuAction(_:)), items: ["😀","😛","😱","😵","😷","🐶","🐱","🐭","🐰","🦊","🌻","🌧","🌎","🔥","❄️","💦","☂️"])
        
        let numbers = menu.addItem(withTitle: "🔢 Numbers", action: nil, keyEquivalent: "")
        numbers.submenu = createMenu(selector: #selector(markerMenuAction(_:)), items: ["0️⃣","1️⃣","2️⃣","3️⃣","4️⃣","5️⃣","6️⃣","7️⃣","8️⃣","9️⃣","🔟"])
        
        let objects = menu.addItem(withTitle: "🖥 Objects", action: nil, keyEquivalent: "")
        objects.submenu = createMenu(selector: #selector(markerMenuAction(_:)), items: ["⌚️","🖥","🖨","⌛️","⏰","⚒","🧲","💣","🔒","✂️","🧸","🎁"])
        
        let symbols = menu.addItem(withTitle: "🔠 Symbols", action: nil, keyEquivalent: "")
        symbols.submenu = createMenu(selector: #selector(markerMenuAction(_:)), items: ["🆗","🆖","#️⃣","🔤","ℹ️","🚻","🔃","➕","➖","➗","✖️","♾","💲","✔️","♠️","♣️","♥️","♦️"])
        
        return menu
    }
    
    fileprivate func createFontMenu() -> NSMenu {
        return createMenu(selector: #selector(fontMenuAction(_:)),
                          items: ["8", "10", "12", "14", "16", "18", "20", "22", "26", "32", "36"])
    }
    
    fileprivate func createLayoutMenu() -> NSMenu {
        return createMenu(selector: #selector(layoutMenuAction(_:)),
                          items: ["left", "right", "center", "top", "top right", "bottom", "bottom right"])
    }
    
    fileprivate func createDisplayMenu(_ item: Bool) -> NSMenu {
        return createMenu(selector: #selector(displayMenuAction(_:)),
                          items: item ? itemDisplayVariants : linkDisplayVariants )
    }
    
    fileprivate func createColorMenu() -> NSMenu {
        return createMenu(selector: #selector(colorMenuAction(_:)),
                          items: ["🔴red", "🟢green", "🔵blue", "🟡yellow", "🟠orange", "🟤brown", "⚫️black", "🟣purple"])
    }
    
    fileprivate func createBorderColorMenu() -> NSMenu {
        return createMenu(selector: #selector(borderColorMenuAction(_:)),
                          items: ["🔴red", "🟢green", "🔵blue", "🟡yellow", "🟠orange", "🟤brown", "⚫️black", "🟣purple"])
    }
    
    fileprivate func createTextColorMenu() -> NSMenu {
        return createMenu(selector: #selector(textColorMenuAction(_:)),
                          items: ["🔴red", "🟢green", "🔵blue", "🟡yellow", "🟠orange", "🟤brown", "⚫️black", "🟣purple"])
    }
    
    fileprivate func createLineWidthMenu() -> NSMenu {
        return createMenu(selector: #selector(lineWidthAction(_:)),
                          items: ["0.3", "0.5", "1", "1.5", "2", "5"])
    }
    
    fileprivate func createLineStyleMenu() -> NSMenu {
        return createMenu(selector: #selector(lineStyleMenuAction(_:)),
                          items: lineStyleDisplayVariants)
    }
    
    fileprivate func showPopup() {
        if self.popupView != nil {
            self.popupView?.removeFromSuperview()
            self.popupView = nil
        }
        
        if !PreferenceConstants.preference.uiQuickPanelOnTop {
            return
        }
        
        if self.mode == .Editing {
            return
        }
        
        guard let act = self.activeItems.first, activeItems.count == 1, let dr = scene?.drawables[act] else {
            return
        }
        let bounds = dr.getSelectorBounds()
        let origin =  CGPoint(x: scene!.offset.x + bounds.origin.x + 15, y: scene!.offset.y + bounds.origin.y + bounds.height + 10)
        
        let segments = NSSegmentedControl() //frame: CGRect(x: 0, y: 0, width: 300, height: 48))
        
        segments.segmentStyle = .rounded
        segments.segmentCount = 10
        
        //        segments.action = #selector(segmentAction(_:))
        
        var segm = -1
        if act.kind == .Item {
            segm += 1
            segments.setLabel("✑", forSegment: segm)
            
            let menu = createMarkerMenus()
            
            segments.setMenu(
                menu,
                forSegment: segm)
            if #available(OSX 10.13, *) {
                segments.setShowsMenuIndicator(true, forSegment: segm)
                
            }
            segments.setWidth(36, forSegment: segm)
            
            
            segm += 1
            segments.setLabel("Ƭ", forSegment: segm)
            segments.setMenu(
                createFontMenu(),
                forSegment: segm)
            if #available(OSX 10.13, *) {
                segments.setShowsMenuIndicator(true, forSegment: segm)
                
            }
            segments.setWidth(36, forSegment: segm)
            
        }
        
        segm += 1
        //        segments.setLabel("Display", forSegment: segm)
        //        segments.setImage(NSImage(named: NSImage.flowViewTemplateName ), forSegment: segm)
        segments.setLabel("❑", forSegment: segm)
        segments.setImageScaling(.scaleProportionallyUpOrDown, forSegment: segm)
        
        segments.setMenu(
            createDisplayMenu(act.kind == .Item),
            forSegment: segm)
        
        if #available(OSX 10.13, *) {
            segments.setShowsMenuIndicator(true, forSegment: segm)
        }
        segments.setWidth(36, forSegment: segm)
        
        if act.kind == .Item {
            segm += 1
            segments.setLabel("🔴", forSegment: segm)
            segments.setMenu(
                createColorMenu(),
                forSegment: segm)
            if #available(OSX 10.13, *) {
                segments.setShowsMenuIndicator(true, forSegment: segm)
                
            }
            segments.setWidth(36, forSegment: segm)
        }
        
        // Border style
        segm += 1
        segments.setLabel("⊞", forSegment: segm)
        segments.setMenu(
            createLineStyleMenu(),
            forSegment: segm)
        if #available(OSX 10.13, *) {
            segments.setShowsMenuIndicator(true, forSegment: segm)
            
        }
        segments.setWidth(36, forSegment: segm)
        
        segm += 1
        segments.setLabel("〰", forSegment: segm)
        segments.setMenu(
            createLineWidthMenu(),
            forSegment: segm)
        if #available(OSX 10.13, *) {
            segments.setShowsMenuIndicator(true, forSegment: segm)
            
        }
        segments.setWidth(36, forSegment: segm)
        
        segments.segmentCount = segm + 1
        
        segments.trackingMode = .momentary
        var popup: NSView?
        
        segments.frame = NSRect(origin: NSPoint(x: 0, y: 0 ), size: segments.fittingSize)
        
        popup  = NSOptionsPopup(frame: NSRect(origin: origin, size: segments.fittingSize))
        
        self.popupView = popup
        popup!.addSubview(segments)
        
        let shadow = NSShadow()
        shadow.shadowOffset = NSSize(width: -5, height: -5)
        shadow.shadowBlurRadius = 10
        shadow.shadowColor = NSColor(red: 0, green: 0, blue: 0, alpha: 0.7)
        popup!.shadow = shadow
        
        segments.allowedTouchTypes = []        
        popupItem = act
        
        popup!.allowedTouchTypes = []
        popup?.frame = NSRect(origin: origin, size: segments.bounds.size)
        
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.2, execute: {
            if self.popupView != nil && self.popupView == popup {
                self.addSubview(popup!)
            }
        })
    }
    
    fileprivate func hidePopup() {
        if self.popupView != nil {
            self.popupView?.removeFromSuperview()
            self.popupView = nil
            self.popupItem = nil
            self.window?.becomeFirstResponder()
        }
    }
    
    public func setActiveItems( _ items: [DiagramItem], immideateDraw: Bool = false, force: Bool = false ) {
        if items.count == 0 && self.activeItems.count == 0 {
            return
        }
        hidePopup()
        // We need to update pivot point
        if let act = items.first {
            var offset = CGFloat(100.0)
            if let dr = scene?.drawables[act] {
                offset = CGFloat(dr.getBounds().width + 10)
            }
            self.pivotPoint = CGPoint(x: act.x + offset , y: act.y)
        }
        
        if !force && activeItems.elementsEqual(items) {
            // No need to select same list
            return
        }
        activeItems = items
        
        for f in onSelection {
            f(items)
        }
        
        // We need to rebuild scene as active element is changed
        scene?.updateActiveElements(items)
        
        
        showPopup()
        
        if immideateDraw {
            needsDisplay = true
        }
        else {
            scheduleRedraw()
        }
    }
    
    fileprivate func commitTitleEditing(_ textView: NSTextView?) {
        guard let textBox = self.editBox else {
            return
        }
        if let active = self.activeItems.first {
            if let tv = textView {
                let textValue = tv.string
                switch self.editingMode {
                case .Name:
                    self.store?.updateName(item: active, textValue, undoManager: self.undoManager, refresh: scheduleRedraw)
                case .Body:
                    self.setBody(active, textValue)
                    break
                case .Value:
                    self.setValue(active, textValue)
                    break;
                }
            }
        }
        
        self.setNormalMode()
        textBox.removeFromSuperview()
        self.editBox = nil
        self.editBoxItem = nil
        self.scene?.editBoxBounds = nil
        self.scene?.editingMode = false
        self.window?.makeFirstResponder(self)
        scheduleRedraw()
    }
    
    func getSelectionBounds() -> CGRect {
        var finalBounds: CGRect?
        for itm in self.activeItems {
            if let dr = self.scene?.drawables[itm] {
                let deBounds = dr.getBounds()
                let bounds = CGRect(
                    x: deBounds.origin.x + scene!.offset.x,
                    y: deBounds.origin.y + scene!.offset.y,
                    width: deBounds.width,
                    height: deBounds.height
                )
                if finalBounds == nil {
                    finalBounds = bounds
                } else {
                    finalBounds = finalBounds?.union(bounds)
                }
            }
        }
        if let fb = finalBounds{
            return CGRect(x: fb.origin.x * zoomLevel, y: fb.origin.y * zoomLevel, width: fb.width * zoomLevel, height: fb.height * zoomLevel)
        }
        return self.frame
    }
    
    func getEditBoxBounds( item: Drawable, _ zoomLevel: CGFloat ) -> CGRect {
        var deBounds = self.editBoxItem!.getSelectorBounds()
        if let link =  item as? DrawableLine {
            deBounds = link.getLabelBounds()
        }
        let bounds = CGRect(
            x: (deBounds.origin.x + scene!.offset.x) * zoomLevel,
            y: (deBounds.origin.y + scene!.offset.y) * zoomLevel,
            width: max(deBounds.width, 100) * zoomLevel,
            height: max(deBounds.height, 20) * zoomLevel
        )
        
        return bounds
    }
    
    static func getBodyText(_ item: DiagramItem, _ bodyStyle: DrawableStyle?, _ textValue: inout String) {
        if let bodyNode = item.properties.get( "body" ) {
            // Body could have custome properties like width, height, color, font-size, so we will parse it as is.
            if let bodyBlock = bodyNode.getChild(1) {
                if bodyBlock.kind == .BlockExpr {
                    if let style = bodyStyle {
                        style.parseStyle(bodyBlock, [:] )
                    }
                    
                    if let bodyText = bodyBlock.getNamedElement("text"), let txtNode = bodyText.getChild(1) {
                        if let txt = getString(txtNode, [:]) {
                            textValue = txt
                        }
                    }
                }
                else if let txt = getString(bodyBlock, [:]) {
                    textValue = txt
                }
            }
        }
    }
    static func findFieldName(_ item:DiagramItem) -> String {
        var fieldName = "value"
        if let field = item.properties.get(PersistenceStyleKind.FieldName.name), let customFieldName = field.getIdent(1) {
            fieldName = customFieldName
        }
        return fieldName
    }
    static func getCustomText(_ item: DiagramItem, _ bodyStyle: DrawableStyle?, _ textValue: inout String) {
        let fieldName = findFieldName(item)
        
        if let bodyNode = item.properties.get( fieldName ) {
            // Body could have custome properties like width, height, color, font-size, so we will parse it as is.
            if let bodyBlock = bodyNode.getChild(1) {
                if let txt = getString(bodyBlock, [:]) {
                    textValue = txt
                }
            }
        }
    }
    
    func getBody( _ item: DiagramItem, _ style: DrawableStyle ) -> (String, CGFloat) {
        let bodyStyle = style.copy()
        bodyStyle.fontSize -= 2 // Make a bit smaller for body
        var textValue = ""
        
        SceneDrawView.getBodyText(item, bodyStyle, &textValue)
        return (prepareBodyText(textValue), bodyStyle.fontSize)
    }
    func getCustomField( _ item: DiagramItem, _ style: DrawableStyle ) -> (String, CGFloat)? {
        let bodyStyle = style.copy()
        bodyStyle.fontSize -= 2 // Make a bit smaller for body
        var textValue = ""
        
        SceneDrawView.getCustomText(item, bodyStyle, &textValue)
        return (prepareBodyText(textValue), bodyStyle.fontSize)
    }
    fileprivate func setBody( _ item: DiagramItem, _ body: String ) {
        let newProps = item.toTennAsProps(.BlockExpr)
        if let bodyNode = newProps.getNamedElement( "body" ) {
            if let bodyBlock = bodyNode.getChild(1) {
                if bodyBlock.kind == .BlockExpr {
                    if let bodyText = bodyBlock.getNamedElement("text") {
                        bodyText.children?.removeAll()
                        bodyText.add(TennNode.newIdent("text"))
                        bodyText.add(TennNode.newMarkdownNode(body))
                    }
                }
                else {
                    // just replace existing text
                    bodyNode.children?.removeAll()
                    bodyNode.add(TennNode.newIdent("body"))
                    bodyNode.add(TennNode.newMarkdownNode(body))
                }
            }
        }
        else {
            let cmd = TennNode.newCommand("body")
            if body.contains("\n") || body.contains("\\n") {
                cmd.add(TennNode.newMarkdownNode(body))
            } else {
                cmd.add(TennNode.newStrNode(body))
            }
            newProps.add(cmd)
        }
        self.store?.setProperties(self.element!, item, newProps, undoManager: self.undoManager, refresh: self.scheduleRedraw)
    }
    
    private static func detectSymbolType( pattern value: String ) -> (TennNodeKind, TennTokenType) {
        var skipFirst = false
        if !value.isEmpty && value.hasPrefix("-") {
            skipFirst = true
        }
        var dot = false
        var i = 0
        for c in value {
            if skipFirst {
                skipFirst = false
                continue
            }
            if c == "." {
                if i == 0  || dot {
                    return (.Ident,.symbol)
                }
                dot = true
                continue
            }
            if c == " " || c == "\n" || c == "\t" {
                return (.StringLit, .stringLit)
            }
            if !CharacterSet.decimalDigits.contains(c.unicodeScalars.first!) {
                return (.Ident,.symbol)
            }
            i += 1
        }
        if dot {
            return (.FloatLit,.floatLit)
        }
        return (.IntLit,.intLit)
    }
    
    fileprivate func setValue( _ item: DiagramItem, _ value: String ) {
        let fieldName = SceneDrawView.findFieldName(item)
        let newProps = item.toTennAsProps(.BlockExpr)
        let (nType, tType) = SceneDrawView.detectSymbolType(pattern: value)
        
        if let bodyNode = newProps.getNamedElement( fieldName ) {
            // just replace existing text
            bodyNode.children?.removeAll()
            bodyNode.add(TennNode.newIdent(fieldName))
            
            // Try detect field type
            bodyNode.add(TennNode(kind: nType , tok: TennToken(type: tType, literal: value) ))
        }
        else {
            let cmd = TennNode.newCommand(fieldName)
            cmd.add(TennNode(kind: nType , tok: TennToken(type: tType, literal: value) ))
            newProps.add(cmd)
        }
        self.store?.setProperties(self.element!, item, newProps, undoManager: self.undoManager, refresh: self.scheduleRedraw)
    }
    
    public func editTitle(_ active: DiagramItem, _ editMode: EditingMode) {
        self.mode = .Editing
        self.editingMode = editMode
        
        guard let de = scene?.drawables[active] else {
            return
        }
        
        hidePopup()
        
        if editBox != nil {
            editBox!.removeFromSuperview()
        }
        self.editBoxItem = de
        
        let bounds = getEditBoxBounds(item: de, self.zoomLevel)
        editBox = PopupEditField(frame: bounds)
        scene?.editBoxBounds = getEditBoxBounds(item: de, 1)
        scene?.editingMode = true
        
        editBox?.wantsLayer = true
        let textFieldLayer = CALayer()
        editBox?.layer = textFieldLayer
        editBox?.layer?.backgroundColor = scene?.sceneStyle.defaultItemStyle.color
        editBox?.layer?.borderColor = scene?.sceneStyle.defaultItemStyle.borderColor
        editBox?.layer?.borderWidth = 0.5
        editBox?.layer?.cornerRadius = 8
        
        if self.editBoxDelegate == nil {
            self.editBoxDelegate = EditTitleDelegate(self)
        }
        
        let style = active.kind == .Item ? self.scene!.sceneStyle.defaultItemStyle.copy() : self.scene!.sceneStyle.defaultLineStyle.copy()
        style.parseStyle(active.properties, [:])
        
        editBox?.delegate = self.editBoxDelegate
        switch self.editingMode {
        case .Name:
            editBox?.stringValue = active.name
            editBox?.font = NSFont.systemFont(ofSize: style.fontSize * zoomLevel)
        case .Body:
            let (text, fontSize) = self.getBody(active, style)
            editBox?.stringValue = text
            editBox?.font = NSFont.systemFont(ofSize: fontSize * zoomLevel)
        case .Value:
            if let (text, fontSize) = self.getCustomField(active, style) {
                editBox?.stringValue = text
                editBox?.font = NSFont.systemFont(ofSize: fontSize * zoomLevel)
            } else {
                mode = .Normal
                return
            }
        }
        
        editBox?.drawsBackground = true
        editBox?.isBordered = true
        editBox?.focusRingType = .none
        
        self.addSubview(editBox!)
        
        self.window?.makeFirstResponder(editBox!)
        
        scheduleRedraw()
    }
    
    fileprivate func setNormalMode() {
        self.mode = .Normal
        scene?.editingMode = false
        self.editBoxItem = nil
        needsDisplay = true
    }
    
    func addTopItem() {
        // Add top element
        let newEl = DiagramItem(kind: .Item, name: "Untitled \(createIndex)")
        self.createIndex += 1
        
        newEl.x = pivotPoint.x
        newEl.y = pivotPoint.y
        self.store?.add(self.element!, newEl, undoManager: self.undoManager, refresh: self.scheduleRedraw)
        
        self.setActiveItem(newEl)
        scheduleRedraw()
    }
    
    func addNewItem(copyProps:Bool = false) {
        guard let active = self.activeItems.first else {
            self.addTopItem()
            return
        }
        if active.kind == .Item {
            // Create and add to activeEl
            let newEl = DiagramItem(kind: .Item, name: "Untitled \(createIndex)")
            self.createIndex += 1
            
            
            newEl.x = pivotPoint.x
            newEl.y = pivotPoint.y
            
            
            if copyProps {
                // Copy parent properties
                for p in active.properties {
                    newEl.properties.append(p.clone())
                }
            }
            
            self.store?.add(self.element!, source: active, target: newEl, undoManager: self.undoManager, refresh: self.scheduleRedraw)
            self.setActiveItem(newEl)
            scheduleRedraw()
        }
    }
    
    //    override func becomeFirstResponder() -> Bool {
    //        return true
    //    }
    
    @objc func duplicateItem() {
        guard let curElement = self.element else {
            return
        }
        var items: [DiagramItem] = []
        var oldNewItems: [DiagramItem:DiagramItem] = [:]
        var links: [DiagramItem] = []
        
        let offsetx = CGFloat(75)
        let offsety = CGFloat(0)
        var processedLinks: [String] = []
        for active in self.activeItems {
            if active.kind == .Item {
                // Create and add to activeEl
                let newEl = DiagramItem(kind: .Item, name: active.name )
                newEl.description = active.description
                oldNewItems[active] = newEl
                
                newEl.x = active.x + offsetx
                newEl.y = active.y + offsety
                
                // Copy parent properties
                for p in active.properties {
                    newEl.properties.append(p.clone())
                }
                
                items.append(newEl)
            }
            else if active.kind == .Link {
                if processedLinks.contains(active.id.uuidString) {
                    continue
                }
                processedLinks.append(active.id.uuidString)
                let li = active.clone()
                links.append(li)
                items.append(li)
            }
            
            for itm  in curElement.getRelatedItems(active, source: false) {
                if itm.kind == .Link {
                    if processedLinks.contains(itm.id.uuidString) {
                        continue
                    }
                    processedLinks.append(itm.id.uuidString)
                    let li = itm.clone()
                    links.append(li)
                    items.append(li)
                }
            }
            
        }
        for li in links {
            if let lli = li as? LinkItem {
                if let src = lli.source, let newItm = oldNewItems[src] {
                    lli.source = newItm
                }
                if let dst = lli.target, let newItm = oldNewItems[dst] {
                    lli.target = newItm
                }
            }
        }
        
        if items.count > 0 {
            self.store?.add(curElement, items, undoManager: self.undoManager, refresh: self.scheduleRedraw)
            self.setActiveItems(items)
            scheduleRedraw()
        }
    }
    
    func showAlert(question: String, _ button1: String = "Yes", _ button2: String = "Cancel", _ text: String = "") -> Bool {
        let alert = NSAlert()
        alert.messageText = question
        alert.informativeText = text
        alert.alertStyle = .warning
        alert.addButton(withTitle: button1)
        alert.addButton(withTitle: button2)
        return alert.runModal() == .alertFirstButtonReturn
    }
    
    func removeItem() {
        // Backspace character
        if self.activeItems.count > 0  {
            self.store?.remove(self.element!, items: self.activeItems, undoManager: self.undoManager, refresh: self.scheduleRedraw)
            scheduleRedraw()
        }
    }
    
    @IBAction override public func moveUp(_ sender: Any?) {
        ox += 0
        oy -= 200
        scheduleRedraw()
    }
    @IBAction override public func moveDown(_ sender: Any?) {
        ox += 0
        oy -= -200
        scheduleRedraw()
    }
    
    @IBAction override public func moveLeft(_ sender: Any?) {
        ox += 200
        oy -= 0
        scheduleRedraw()
    }
    
    @IBAction override public func moveRight(_ sender: Any?) {
        ox += -200
        oy -= 0
        scheduleRedraw()
    }
    
    @IBAction func selectAllItems(_ sender: NSMenuItem) {
        if self.mode == .Normal {
            selectAllItems()
        } else if self.mode == .Editing, let eb = self.editBox {
            eb.selectAll(sender)
        }
    }
    
    @IBAction func selectNoneItems(_ sender: NSMenuItem) {
        if self.mode == .Normal {
            selectNoneItems()
        }
    }
    
    override func keyDown(with event: NSEvent) {
        if self.mode == .Editing {
            return
        }
        if event.characters == "\t" {
            addNewItem(copyProps: event.modifierFlags.contains(NSEvent.ModifierFlags.option))
            return
        }
        if event.characters == "x" {
            removeItem()
            return
        }               
        
        if let sk = event.specialKey, let sc = self.scene {
            var ops: [ElementOperation] = []
            for active in self.activeItems {
                switch sk {
                case NSEvent.SpecialKey.leftArrow:
                    var x = roundf(Float(active.x - sc.sceneStyle.gridSpan.x))
                    x = x - Float(Int(x) % Int(sc.sceneStyle.gridSpan.x))
                    let y = active.y
                    let newPos = CGPoint( x: CGFloat(x), y: CGFloat(y))
                    ops.append(store!.createUpdatePosition(item: active, newPos: newPos))
                case NSEvent.SpecialKey.rightArrow:
                    var x = roundf(Float(active.x + sc.sceneStyle.gridSpan.x))
                    x = x - Float(Int(x) % Int(sc.sceneStyle.gridSpan.x))
                    let y = active.y
                    let newPos = CGPoint( x: CGFloat(x), y: CGFloat(y))
                    ops.append(store!.createUpdatePosition(item: active, newPos: newPos))
                case NSEvent.SpecialKey.upArrow:
                    let x = active.x
                    var y = roundf(Float(active.y + sc.sceneStyle.gridSpan.y))
                    y = y - Float(Int(y) % Int(sc.sceneStyle.gridSpan.y))
                    let newPos = CGPoint( x: CGFloat(x), y: CGFloat(y))
                    ops.append(store!.createUpdatePosition(item: active, newPos: newPos))
                case NSEvent.SpecialKey.downArrow:
                    let x = active.x
                    var y = roundf(Float(active.y - sc.sceneStyle.gridSpan.y))
                    y = y - Float(Int(y) % Int(sc.sceneStyle.gridSpan.y))
                    let newPos = CGPoint( x: CGFloat(x), y: CGFloat(y))
                    ops.append(store!.createUpdatePosition(item: active, newPos: newPos))
                default:
                    break;
                }
            }
            if ops.count > 0 {
                store?.compositeOperation(notifier: self.element!, undoManaget: self.undoManager, refresh: scheduleRedraw, ops)
                //                self.setActiveItems(self.activeItems)
                scheduleRedraw()
                return 
            }
        }
        super.keyDown(with: event)
    }
    
    public func getActiveItemBounds() -> CGRect? {
        if let active = self.activeItems.first, let drawable = scene?.drawables[active] {
            let drBounds = drawable.getBounds()
            let off = CGPoint(x: self.ox + bounds.midX, y: self.oy + bounds.midY)
            return CGRect(x: drBounds.minX + off.x, y: drBounds.minY + off.y, width: drBounds.width, height: drBounds.height)
        }
        return nil
    }
    
    private func showPopover(bounds: CGRect) {
        let controller = NSViewController()
        controller.view = NSView(frame: CGRect(x: CGFloat(0), y: CGFloat(0), width: CGFloat(100), height: CGFloat(50)))
        controller.view.autoresizesSubviews = true
        
        let popover = NSPopover()
        popover.contentViewController = controller
        popover.contentSize = controller.view.frame.size
        
        popover.behavior = .transient
        popover.animates = false
        
        // let txt = NSTextField(frame: NSMakeRect(100,50,50,22))
        let txt = NSTextField(frame: controller.view.frame)
        txt.stringValue = "Hello world"
        txt.textColor = NSColor.black.withAlphaComponent(0.95)
        controller.view.addSubview(txt)
        txt.sizeToFit()
        popover.show(relativeTo: bounds, of: self, preferredEdge: NSRectEdge.maxY)
    }
    
    public func findElement(x: CGFloat, y: CGFloat, allowAll: Bool = false) -> [ItemDrawable] {
        let point = CGPoint(x: x, y: y)
        return self.scene?.find(point, allowAll: allowAll) ?? []
    }
    
    
    override func mouseUp(with event: NSEvent) {
        self.updateMousePosition(event)
        if self.mode == .Editing {
            // No dragging allowed until editing is not done
            return
        }
        if self.mode == .Selection {
            self.mode = .Normal
            
            if !event.modifierFlags.contains(NSEvent.ModifierFlags.shift) {
                setActiveItems(self.dragElements)
            }
            self.dragElements.removeAll()
            self.scene?.selectionBox = nil
            self.scene?.updateActiveElements(self.activeItems)
            
            for f in onSelection {
                f(self.activeItems)
            }
            
            scheduleRedraw()
            showPopup()
            return
        }
        
        if self.mode == .LineDrawing {
            if let source = self.dragElements.first, let target = self.lineTarget {
                // Create a new line if not yet pressent between elements
                
                //TennNode.newCommand("display", TennNode.newStrNode("arrow"))
                store?.add(element!, source:source, target: target, undoManager: self.undoManager, refresh: self.scheduleRedraw, props: [])
            }
            
            scene?.removeLineTo()
            self.lineToPoint = nil
            self.lineTarget = nil
            scheduleRedraw()
        }
        else {
            // Check if up/down interval is appropriate for dragging.
            
            let now = Date()
            
            if let down = self.downDate {
                if now.timeIntervalSince(down).isLess(than: 0.2) {
                    
                    let drawables = findElement(x: self.x, y: self.y)
                    if drawables.count == 0 {
                        self.setActiveItem(nil)
                        self.scene?.selectionBox = nil
                    }
                    
                    showPopup()
                    self.mode = .Normal
                    return
                }
            }
            
            var ops: [ElementOperation] = []
            
            for de in self.dragElements {
                if let newPos = self.dragMap.removeValue(forKey: de) {
                    let pos = newPos
                    if pos.x != de.x || pos.y != de.y {
                        ops.append(store!.createUpdatePosition(item: de, newPos: newPos))
                    }
                }
            }
            if ops.count > 0 {
                store?.compositeOperation(notifier: self.element!, undoManaget: self.undoManager, refresh: scheduleRedraw, ops)
                self.setActiveItems(self.dragElements)
                scheduleRedraw()
            }
        }
        
        self.mouseDownState = false
        for de in self.dragElements {
            self.dragMap.removeValue(forKey: de)
        }
        self.dragElements.removeAll()
        
        self.mode = .Normal
        showPopup()
    }
    
    var downDate:Date? = nil
    
    override func mouseDown(with event: NSEvent) {
        self.updateMousePosition(event)
        
        self.downDate = Date()
        
        if self.mode == .Editing {
            // No dragging allowed until editing is not done
            self.commitTitleEditing(nil)
        }
        
        
        self.mouseDownState = true
        
        self.dragMap.removeAll()
        self.dragElements.removeAll()
        
        if event.modifierFlags.contains(NSEvent.ModifierFlags.shift) {
            self.selectionStart = CGPoint(x: self.x, y: self.y)
            self.mode = .Selection
            // Copy current selection
            self.dragElements = self.activeItems
            //Deselect all
            self.setActiveItem(nil)
            scene?.updateActiveElements(self.activeItems)
            return
        }
        
        var drawables = findElement(x: self.x, y: self.y)
        
        var result: [ItemDrawable] = []
        let point = CGPoint(x: self.x, y: self.y)
        
        if event.clickCount == 1 {
            for active in self.activeItems {
                if let activeDr = scene?.drawables[active] {
                    if activeDr.getBounds().contains(point) {
                        if let ln = activeDr as? DrawableLine {
                            if ln.find(point) {
                                if let act = activeDr as? ItemDrawable {
                                    result.append(act)
                                }
                            }
                        }
                        else {
                            if let act = activeDr as? ItemDrawable {
                                result.append(act)
                            }
                        }
                    }
                }
            }
        }
        if event.clickCount == 2 && drawables.count == 1 {
            self.editTitle(drawables[0].item!, .Name)
            return
        }
        
        if result.count > 0 {
            drawables = result
        }
        
        if drawables.count == 0 {
            //            if event.clickCount == 2 {
            //                self.setActiveItem(nil)
            //                self.mode = .DiagramMove
            //                self.scene?.selectionBox = nil
            //            }
            
            self.pivotPoint = CGPoint(x: self.x , y: self.y)
            
            return
        }
        
        if event.modifierFlags.contains(NSEvent.ModifierFlags.command) {
            // This is selection operation
            guard let dr = drawables.first else {
                return
            }
            guard let itm = dr.item else {
                return
            }
            if self.activeItems.contains(itm) {
                self.activeItems.remove(at: self.activeItems.firstIndex(of: itm)!)
            }
            else {
                self.activeItems.append(itm)
            }
            setActiveItems(self.activeItems, force: true )
            return
        }
        else {
            var itemsToSelect: [DiagramItem] = []
            for newDrawable in drawables {
                if let newItem = newDrawable.item, !self.activeItems.contains(newItem) {
                    itemsToSelect.append(newItem)
                }
            }
            if itemsToSelect.count > 0 {
                self.clickCounter += 1;
                let pos: Int = clickCounter % itemsToSelect.count
                let itm = itemsToSelect[pos]
                self.setActiveItem(itm)
                scene?.updateActiveElements(self.activeItems)
            }
        }
        
        self.dragElements.append(contentsOf: self.activeItems)
        
        if event.modifierFlags.contains(NSEvent.ModifierFlags.option) && self.dragElements.count == 1 {
            // Add all outgoing items and all outgoing items.
            var itemsToCheck: [DiagramItem] = [self.activeItems[0]]
            while itemsToCheck.count > 0 {
                let item = itemsToCheck.removeFirst()
                for itm in self.element!.items {
                    // Need to check if item is Link and source or target is our client
                    if itm.kind == .Link, let lData = itm as? LinkItem, let source = lData.source, let target = lData.target {
                        if source.id == item.id {
                            // we found target item
                            if !self.dragElements.contains(target) {
                                self.dragElements.append(target)
                                itemsToCheck.append(target)
                            }
                        }
                    }
                }
            }
            self.setActiveItems(dragElements)
            
        }
        
        if event.modifierFlags.contains(NSEvent.ModifierFlags.control) && self.dragElements.count == 1 {
            self.mode = .LineDrawing
            self.lineToPoint = CGPoint(x: self.x, y: self.y )
        }
        else {
            self.mode = .Dragging
            for de in self.dragElements  {
                self.dragMap[de] = CGPoint(x: de.x, y: de.y)
            }
        }
    }
    
    override var acceptsFirstResponder: Bool {
        get {
            return true
        }
    }
    
    
    override func mouseDragged(with event: NSEvent) {
        //        if self.mode != .Dragging && self.mode != .DiagramMove && self.mode != .LineDrawing && self.mode != .Selection  {
        //            return
        //        }
        
        self.hidePopup()
        self.updateMousePosition(event)
                
        if self.mode == .Selection {
            let minX = min(self.selectionStart.x, self.x)
            let minY = min(self.selectionStart.y, self.y)
            let width = abs(self.selectionStart.x - self.x)
            let height = abs(self.selectionStart.y - self.y)
            let selBox = CGRect(x: minX, y: minY, width: width, height: height)
            scene?.selectionBox = selBox
            
            self.activeItems.removeAll()
            if let drv = scene?.drawables {
                for (it, d) in drv {
                    let db = d.getBounds()
                    if selBox.intersects(db) && d.isSelectable() {
                        self.activeItems.append(it)
                    }
                }
            }
            
            scene?.updateActiveElements(self.activeItems)
            
            scheduleRedraw()
            return
        }
        
        if self.dragElements.count > 0 {
            var newPositions: [DiagramItem: CGPoint] = [:]
            
            for de in dragElements {
                if self.mode == .LineDrawing {
                    self.lineToPoint = CGPoint(x: self.x, y: self.y)
                    self.lineTarget = scene?.updateLineTo( de, self.lineToPoint! )
                    
                    scheduleRedraw()
                }
                else {
                    if let pos = self.dragMap[de], (de.kind == .Item || self.dragElements.count == 1), let dde = self.scene?.drawables[de] {
                        let newPos = CGPoint(x: pos.x + event.deltaX / zoomLevel, y:pos.y - event.deltaY / zoomLevel)
                        self.dragMap[de] = newPos
                        
                        var viewPos = CGPoint( x: newPos.x, y: newPos.y)
                        
                        if let box = dde as? RoundBox {
                            viewPos.y -= box.bounds.height
                        }
                        if let box = dde as? CircleBox {
                            viewPos.y -= box.bounds.height
                        }
                        if let box = dde as? EmptyBox {
                            viewPos.y -= box.bounds.height
                        }
                        newPositions[de] = viewPos
                    }
                }
            }
            if newPositions.count > 0 {
                //TODO: Pass a list of drag items and sibling drag items,
                // Also do not add sibling drag items to direct list, and use separate one, to not select after finish of
                // Operation.
                let dirtyRegion = self.scene!.updateLayout(newPositions)
                
                let sceneBounds = zoomBounds()
                
                let p = CGPoint(x: (self.ox +  sceneBounds.midX + dirtyRegion.origin.x ) * zoomLevel, y: ( self.oy + sceneBounds.midY + dirtyRegion.origin.y ) * zoomLevel)
                let newDirty = CGRect(origin: p, size: CGSize(width: dirtyRegion.size.width * zoomLevel, height: dirtyRegion.size.height * zoomLevel ))
                scheduleRedraw(invalidRect: newDirty)
            }
        }
        else {
            ox += event.deltaX / zoomLevel
            oy -= event.deltaY / zoomLevel
            
            self.mode = .DiagramMove
            
            scheduleRedraw()
        }
    }
    
    func zoomBounds()-> CGRect {
        return CGRect(x: bounds.origin.x / zoomLevel,
                                 y: bounds.origin.y / zoomLevel,
                                 width: bounds.width / zoomLevel,
                                 height: bounds.height / zoomLevel)
    }
    
    func updateMousePosition(_ event: NSEvent) {
        let wloc = event.locationInWindow
        
        let vp = self.convert(wloc, from: nil)
        
        let sceneBounds = zoomBounds()
        
        let pos = CGPoint(x: ( vp.x / zoomLevel ) - sceneBounds.midX - ox, y: ( vp.y / zoomLevel )  - sceneBounds.midY - oy )
        
        self.x = pos.x
        self.y = pos.y
    }
    
    override func mouseMoved(with event: NSEvent) {
        if self.mode == .Editing {
            return
        }
        self.updateMousePosition(event)
        
        // Process hide of popup if we go out to much
        
        if let act = self.popupItem, let dr = scene?.drawables[act], let popupView = self.popupView {
            let bounds = dr.getSelectorBounds()
            let popupBounds = popupView.bounds
            var rect = bounds
            rect.size = CGSize(width: max(rect.width, popupBounds.width), height: rect.height + popupBounds.height)
            rect = rect.insetBy(dx: -30, dy: -30)
            let p = CGPoint(x: self.x, y: self.y)
            
            if !rect.contains(p) {
                hidePopup()
            }
            
            return
        }
    }
    
    override func viewWillStartLiveResize() {
        if self.mode == .Editing {
            commitTitleEditing(nil)
        }
    }        
    
    fileprivate func drawRulers(_ scene: DrawableScene, _ context: CGContext) {
        let ycount = 20
        let xcount = 30
        
        let sceneBounds = zoomBounds()
        
        var leftBoxes: [Int] = []
        var rightBoxes: [Int] = []
        
        var topBoxes: [Int] = []
        var bottomBoxes: [Int] = []
        
        for _ in 0...ycount {
            leftBoxes.append(0)
            rightBoxes.append(0)
        }
        
        for _ in 0...xcount {
            topBoxes.append(0)
            bottomBoxes.append(0)
        }
        
        
        let ystep = sceneBounds.height / CGFloat(ycount)
        let xstep = sceneBounds.width / CGFloat(xcount)
        
        
        for d in scene.drawables.values {
            let db = d.getBounds()
            
            let x = db.minX + scene.offset.x
            let y = db.minY + scene.offset.y
            
            if x + db.width < 0 {
                var ypos = Int((y + db.height/2) / ystep)
                if ypos > ycount {
                    ypos = ycount
                }
                if ypos < 0 {
                    ypos = 0
                }
                leftBoxes[ypos] += 1
            }
            if x > sceneBounds.width {
                var ypos = Int((y + db.height/2) / ystep)
                if ypos > ycount {
                    ypos = ycount
                }
                if ypos < 0 {
                    ypos = 0
                }
                rightBoxes[ypos] += 1
            }
            if y + db.height < 0 {
                var xpos = Int((x + db.width/2) / xstep)
                if xpos >= xcount {
                    xpos = xcount
                    rightBoxes[0] += 1
                }
                else if xpos <= 0 {
                    xpos = 0
                    leftBoxes[0] += 1
                }
                else {
                    bottomBoxes[xpos] +=  1
                }
            }
            if y > sceneBounds.height {
                var xpos = Int((x + db.width/2) / xstep)
                if xpos >= xcount {
                    xpos = xcount
                    rightBoxes[ycount] += 1
                }
                else if xpos <= 0 {
                    xpos = 0
                    leftBoxes[ycount] += 1
                }
                else {
                    topBoxes[xpos] += 1
                }
            }
        }
        
        if PreferenceConstants.preference.darkMode {
            context.setStrokeColor(CGColor(red: 227 / 255.0 , green: 157 / 255.0, blue: 68 / 255.0, alpha: 1))
            context.setFillColor(CGColor(red: 227 / 255.0 , green: 157 / 255.0, blue: 68 / 255.0, alpha: 1))
            context.setShadow(offset: CGSize(width:3, height: -3), blur: 5.0)
        }
        else {
            context.setStrokeColor(CGColor(red: 0, green: 0, blue: 0, alpha: 1))
            context.setFillColor(CGColor(red: 0, green: 0, blue: 0, alpha: 1))
            context.setShadow(offset: CGSize(width:3, height: -3), blur: 5.0)
        }
        
        
        
        var needDraw = false
        for i in 0...ycount {
            var c = leftBoxes[i]
            var newy = ystep * CGFloat(i) + 2
            let cx = CGFloat(0.2)
            if c > 0 {
                if c > 50 {
                    c = 50
                }
                if i == ycount {
                    newy = newy - CGFloat(c)*cx - 7
                }
                context.addEllipse(in: CGRect(x: CGFloat(2), y: newy, width: 5 + CGFloat(c)*cx, height: 5 + CGFloat(c)*cx))
                needDraw = true
            }
            
            var cr = rightBoxes[i]
            if cr > 0 {
                if cr > 50 {
                    cr = 50
                }
                if i == ycount {
                    newy = sceneBounds.height - CGFloat(cr)*cx - 7
                }
                context.addEllipse(in: CGRect(x: sceneBounds.width - 7 - CGFloat(cr)*cx, y: newy, width: 5 + CGFloat(cr)*cx, height: 5 + CGFloat(cr)*cx))
                needDraw = true
            }
        }
        
        for i in 0...xcount {
            var c = bottomBoxes[i]
            var newx = xstep * CGFloat(i) + 2
            let cy = CGFloat(0.2)
            if c > 0 {
                if c > 50 {
                    c = 50
                }
                if i == xcount {
                    newx = newx - CGFloat(c)*cy - 7
                }
                context.addEllipse(in: CGRect(x: newx, y: CGFloat(2), width: 5 + CGFloat(c)*cy, height: 5 + CGFloat(c)*cy))
                needDraw = true
            }
            
            var cr = topBoxes[i]
            if cr > 0 {
                if cr > 50 {
                    cr = 50
                }
                if i == xcount {
                    newx = newx - CGFloat(cr)*cy - 7
                }
                context.addEllipse(in: CGRect(x: newx, y: sceneBounds.height - 7 - CGFloat(cr)*cy, width: 5 + CGFloat(cr)*cy, height: 5 + CGFloat(cr)*cy))
                needDraw = true
            }
        }
        if needDraw {
            context.drawPath(using: .fillStroke)
        }
    }
    
    override func draw(_ dirtyRect: NSRect) {
        if( self.element == nil) {
            return
        }
        
        // Check if apperance changed
        
        let nDarkMode = PreferenceConstants.preference.isDiagramDarkMode()
        if self.scene?.darkMode != nDarkMode {
            buildScene()
        }
//        let now = Date()
        if let context = NSGraphicsContext.current?.cgContext, let scene = self.scene  {
            context.saveGState()
            // Draw background
            var color = PreferenceConstants.preference.background
            
            if PreferenceConstants.preference.uiTransparentBackground {
                color = color.copy(alpha: 0.1)!
            }
            
            context.setFillColor( color )
            
            context.setShouldAntialias(true)
            context.fill(self.bounds)
                        
            let sceneBounds = zoomBounds()
            
            let sceneDirtyRect = CGRect(x: dirtyRect.origin.x / zoomLevel,
                                     y: dirtyRect.origin.y  / zoomLevel,
                                     width: dirtyRect.width  / zoomLevel,
                                     height: dirtyRect.height / zoomLevel)
            
            scene.offset = CGPoint(x: self.ox + sceneBounds.midX, y: self.oy + sceneBounds.midY)
            
            let sceneDirty = CGRect(
                origin: CGPoint(x: sceneDirtyRect.origin.x - scene.offset.x, y: sceneDirtyRect.origin.y - scene.offset.y),
                size:sceneDirtyRect.size
            )
                        
                        
            context.saveGState()
            
            context.scaleBy(x: zoomLevel, y: zoomLevel)
            
            scene.layout(sceneBounds, sceneDirty)
            scene.draw(context: context)
            
            drawRulers(scene, context)
            
            context.restoreGState()
            context.restoreGState()
        }
        //        Swift.debugPrint("draw \(Date().timeIntervalSince(now))")
    }
    
    public func selectAllItems() {
        self.setActiveItems(self.element!.items)
        scheduleRedraw()
    }
    
    public func selectAllByKind(kind: ItemKind) {
        self.setActiveItems(self.element!.items.filter({ it in it.kind == kind}))
        scheduleRedraw()
    }
    
    public func selectNoneItems() {
        self.setActiveItems([])
        scheduleRedraw()
    }
    
    
    /// Selectors
    
    @objc public func zoomIn(_ sender: NSMenuItem) {
        zoomLevel /= 0.75
        scheduleRedraw()
    }
    
    @objc public func zoomOut(_ sender: NSMenuItem) {
        zoomLevel *= 0.75
        scheduleRedraw()
    }
    
    @objc public func removeItmAction(_ sender: NSMenuItem) {
        removeItem()
    }
    
    @objc public func addTopItm(_ sender: NSMenuItem) {
        addTopItem()
    }
    
    @objc func addNewItemNoCopy(_ sender: NSMenuItem) {
        addNewItem(copyProps: false)
    }
    // For selector
    @objc func addNewItemCopy(_ sender: NSMenuItem) {
        addNewItem(copyProps: true)
    }
    
    @objc func delete( _ sender: NSObject ) {
        removeItem()
    }
    
    
    @objc func cut( _ sender: NSObject ) {
        copy(sender)
        removeItem()
    }
    
    @objc func copy( _ sender: NSObject ) {
        if self.activeItems.count > 0 {
            let block = self.element!.storeItems(self.activeItems)
            ClipboardUtils.copy(block)
        }
    }
    
    @objc func pasteAsItem( _ sender: NSObject ) {
        if let value = NSPasteboard.general.string(forType: .string) {
            let newEl = DiagramItem(kind: .Item, name: "pasted \(createIndex)")
            self.createIndex += 1
            
            newEl.x = pivotPoint.x
            newEl.y = pivotPoint.y
            newEl.properties = ModelProperties()
            
            // Create text and title
            newEl.properties.append(TennNode.newCommand("text", TennNode.newMarkdownNode(value)))
            newEl.properties.append(TennNode.newCommand("title", TennNode.newMarkdownNode("${text}" )))
            self.store?.add(self.element!, newEl, undoManager: self.undoManager, refresh: self.scheduleRedraw)
            
            self.setActiveItem(newEl)
        }
    }
    @objc func pasteAsItemSet( _ sender: NSObject ) {
        if let value = NSPasteboard.general.string(forType: .string) {
            let lines =  value.split(separator: "\n")
            
            var items:[DiagramItem] = []
            
            var rootItem: DiagramItem! = nil
            var i = 0
            for l in lines {
                let newEl = DiagramItem(kind: .Item, name: String(l))
                self.createIndex += 1
                
                newEl.x = pivotPoint.x
                newEl.y = pivotPoint.y - 35 * CGFloat(i)
                
                if rootItem == nil {
                    rootItem = newEl
                } else {
                    let li = LinkItem(kind: .Link, name: "", source: rootItem, target: newEl)
                    li.properties.append(TennNode.newCommand("display", TennNode.newIdent("arrow")))
                    items.append(li)
                }
                
                i += 1
                items.append(newEl)
            }
            
            self.store?.add(self.element!, items, undoManager: self.undoManager, refresh: self.scheduleRedraw)
            
        }
    }
    
    @objc func paste( _ sender: NSObject ) {
        ClipboardUtils.paste { node in
            if node.kind == .Command {
                if let activeItem = self.activeItems.first {
                    let props = activeItem.toTennAsProps()
                    props.add(node)
                    if let cmdName = node.getIdent(0), let imgName = node.getIdent(1), cmdName == "image" {
                        if let titleItem = props.getNamedElement("title") {
                            // Prepend image to title
                            if let bodyText = titleItem.getIdent(1) {
                                titleItem.children![1] = TennNode.newMarkdownNode("@(\(imgName)|96)\n\(bodyText)" )
                            }
                        } else {
                            // Create title from name and image.
                            props.add(TennNode.newCommand("title", TennNode.newMarkdownNode("@(\(imgName)|96)\n${name}" )))
                        }
                    }
                    self.store?.setProperties(self.element!, activeItem, props, undoManager: self.undoManager, refresh: self.scheduleRedraw)
                    return
                } else {
                    // Add new diagram item.
                    let newEl = DiagramItem(kind: .Item, name: "Untitled \(createIndex)")
                    self.createIndex += 1
                    
                    newEl.x = pivotPoint.x
                    newEl.y = pivotPoint.y
                    newEl.properties = ModelProperties()
                    newEl.properties.append(node)
                    if let cmdName = node.getIdent(0), let imgName = node.getIdent(1), cmdName == "image" {
                        // Create title from name and image.
                        newEl.properties.append(TennNode.newCommand("title", TennNode.newMarkdownNode("@(\(imgName)|96)\n${name}" )))
                    }
                    self.store?.add(self.element!, newEl, undoManager: self.undoManager, refresh: self.scheduleRedraw)
                    
                    self.setActiveItem(newEl)
                }
            } else {
                let items = Element.parseItems(node: node)
                if items.count > 0 {
                    // Move items a bit,
                    //                    for i in items {
                    //                        if i.kind == .Item {
                    //                            i.x += 10
                    //                            i.y -= 10
                    //                        }
                    //                    }
                    self.store?.add(self.element!, items, undoManager: self.undoManager, refresh: self.scheduleRedraw)
                    
                    self.setActiveItems(items)
                    scheduleRedraw()
                }
            }
        }
    }
    
    func validateMenuItem(_ menuItem: NSMenuItem) -> Bool {
        if let action = menuItem.action {
            if action == #selector(cut) {
                return !self.activeItems.isEmpty
            }
            else if action == #selector(copy(_:)) {
                return !self.activeItems.isEmpty
            }
            else if action == #selector(cut(_:)) {
                return !self.activeItems.isEmpty
            }
            else if action == #selector(delete(_:)) {
                return !self.activeItems.isEmpty
            }
            else if action == #selector(duplicateItem) {
                return !self.activeItems.isEmpty
            }
            else if action == #selector(fontMenuAction) {
                return true
            }
            else if action == #selector(copyItemAsHTML) {
                return true
            }
            else if action == #selector(paste(_:)) {
                return ClipboardUtils.canPaste()
            }            
        }
        return true
    }
    
    @objc func attachImage( _ sender: NSObject ) {
        let myOpen = NSOpenPanel()
        myOpen.allowedFileTypes = ["png", "jpg", "jpeg"]
        myOpen.allowsOtherFileTypes = false
        myOpen.isExtensionHidden = true
        myOpen.nameFieldStringValue = self.element!.name
        myOpen.title = "Attach image file..."
        
        myOpen.begin { (result) -> Void in
            if result == NSApplication.ModalResponse.OK {
                if let filename = myOpen.url {
                    let image = NSImage(byReferencing: filename)
                    if let active = self.activeItems.first {
                        let name = filename.lastPathComponent
                        //                        self.store?.setImage(self.element!, active, name: name, image: image,
                        //                                             undoManager: self.undoManager,  refresh: {()->Void in})
                        let newProps = active.toTennAsProps(.BlockExpr)
                        if let tiffData = image.tiffRepresentation {                            
                            let imageRep = NSBitmapImageRep(data: tiffData)
                            if let pngData = imageRep?.representation(using: .png, properties: [:]) {
                                newProps.add(TennNode.newCommand("image", TennNode.newStrNode(name), TennNode.newImageNode(pngData.base64EncodedString())))
                            }
                        }
                        self.store?.setProperties(self.element!, active, newProps,
                                                  undoManager: self.undoManager,  refresh: {()->Void in})
                    }
                }
            }
        }
    }
    
    @objc func applyShadow(_ sender: NSMenuItem ) {
        // Apply shadow to  default or to selected item.
        if self.activeItems.count == 0 {
            // Apply for default style
            if let active = self.element {
                let newProps = active.properties.clone()
                
                var styleNode = newProps.get("styles")
                if styleNode == nil {
                    styleNode = TennNode.newCommand("styles", TennNode.newBlockExpr())
                    newProps.append(styleNode!)
                }
                
                var itemsStyleNode = styleNode?.getChild(1)?.getNamedElement("item")
                if itemsStyleNode == nil {
                    itemsStyleNode = TennNode.newCommand("item", TennNode.newBlockExpr())
                    styleNode?.getChild(1)?.add(itemsStyleNode!)
                }
                
                if let shadowItem = itemsStyleNode?.getChild(1)?.getNamedElement("shadow") {
                    shadowItem.children = TennNode.newCommand("shadow", TennNode.newIntNode(-5), TennNode.newIntNode(-5), TennNode.newIntNode(5)).children
                } else {
                    // Add new sync config
                    itemsStyleNode?.getChild(1)?.add(
                        TennNode.newCommand("shadow", TennNode.newIntNode(-5), TennNode.newIntNode(-5), TennNode.newIntNode(5))
                    )
                }
                //            self.controller.viewController?.mergeProperties(newProps.asNode())
                self.store?.setProperties(active, newProps.asNode(),
                                          undoManager: undoManager,  refresh: {()->Void in})
            }
            
        } else {
            //TODO:
        }
    }
    
    fileprivate func addChildMenu(menu: NSMenu, title: String, submenu: NSMenu) {
        let item = NSMenuItem(title: title, action: nil, keyEquivalent: "")
        menu.addItem(item)
        menu.setSubmenu(submenu, for: item)
    }
    
    fileprivate func createStylesMenu(_ menu: NSMenu) {
        let style = NSMenuItem(
            title: "Style", action: nil, keyEquivalent: "")
        menu.addItem(style)
        menu.setSubmenu(styleManager?.createMenu(), for: style)
        
        if self.activeItems.count == 0 {
            let quick = NSMenu()
            quick.autoenablesItems=true
            
            quick.addItem(NSMenuItem(title: "Enable shadows", action: #selector(applyShadow), keyEquivalent: ""))
            
            let quickMenuItem = NSMenuItem(title: "Global Styles", action: nil, keyEquivalent: "")
            menu.addItem(quickMenuItem)
            menu.setSubmenu(quick, for: quickMenuItem)
        }
    }
    
    fileprivate func createQuickStyleMenu(_ menu: NSMenu) {
        let qstyle = NSMenuItem(
            title: "Quick Style", action: nil, keyEquivalent: "")
                
        let qstyleMenu = NSMenu()
        
        guard let act = self.activeItems.first, activeItems.count == 1 else {
            return
        }
                    
        addChildMenu(menu: qstyleMenu, title: "Color", submenu: createColorMenu())
        if act.kind != .Link {
            addChildMenu(menu: qstyleMenu, title: "Border color", submenu: createBorderColorMenu())
        }
        addChildMenu(menu: qstyleMenu, title: "Text Color", submenu: createTextColorMenu())
        qstyleMenu.addItem(NSMenuItem.separator())
        addChildMenu(menu: qstyleMenu, title: "Font", submenu: createFontMenu())
        if act.kind != .Link {
            addChildMenu(menu: qstyleMenu, title: "Markers", submenu: createMarkerMenus())
        }
        qstyleMenu.addItem(NSMenuItem.separator())
        addChildMenu(menu: qstyleMenu, title: "Display", submenu: createDisplayMenu(act.kind == .Item))
        addChildMenu(menu: qstyleMenu, title: "Layout", submenu: createLayoutMenu())
        qstyleMenu.addItem(NSMenuItem.separator())
        addChildMenu(menu: qstyleMenu, title: "Line style", submenu: createLineStyleMenu())
        addChildMenu(menu: qstyleMenu, title: "Line width", submenu: createLineWidthMenu())
        qstyleMenu.addItem(NSMenuItem.separator())
        
        if act.kind != .Link {
        addChildMenu(menu: qstyleMenu, title: "Width", submenu: createMenu(selector: #selector(commandAction(_:)),
                                                                           items: ["width 10", "width 50", "width 100", "width 300", "width 500"]))
        addChildMenu(menu: qstyleMenu, title: "Height", submenu: createMenu(selector: #selector(commandAction(_:)),
                                                                            items: ["height 10", "height 50", "height 100", "height 300", "height 500"]))
        }
        
        qstyleMenu.addItem(NSMenuItem.separator())
        addChildMenu(menu: qstyleMenu, title: "Layer", submenu: createMenu(selector: #selector(commandAction(_:)),
                                                                            items: ["layer background", "layer hover"]))
        if act.kind != .Link {
            addChildMenu(menu: qstyleMenu, title: "Corner Radius", submenu: createMenu(selector: #selector(commandAction(_:)),
                                                                            items: ["corner-radius 0", "corner-radius 5", "corner-radius 15"]))
        }
        
        addChildMenu(menu: qstyleMenu, title: "Shadow", submenu: createMenu(selector: #selector(commandAction(_:)),
                                                                            items: ["shadow 5 -5", "shadow 5 5"]))
        
        menu.addItem(qstyle)
        menu.setSubmenu(qstyleMenu, for: qstyle)
    }
    
    @objc public func performGridLayout(_ sender: NSMenuItem) {
        let l = GridLayout()
        let ctx = LayoutContext(self.element!, scene: self.scene!, store: self.store!, bounds: self.viewController!.view.bounds)
        let ops = l.apply(context: ctx, clean: true)
        if ops.count > 0 {
            store?.compositeOperation(notifier: self.element!, undoManaget: self.undoManager, refresh: scheduleRedraw, ops)
            scheduleRedraw()
            return
        }
    }
    
    @objc public func performSpringLayout(_ sender: NSMenuItem) {
        let l = SpringLayout()
        let vbounds = self.viewController!.scene.bounds
        let bounds = CGRect(x: -1 * vbounds.width / 2, y: -1 * vbounds.height / 2, width: vbounds.width, height: vbounds.height)
        let ctx = LayoutContext(self.element!, scene: self.scene!, store: self.store!, bounds: bounds )
        let ops = l.apply(context: ctx, clean: true)
        if ops.count > 0 {
            store?.compositeOperation(notifier: self.element!, undoManaget: self.undoManager, refresh: {
                if let bounds = self.scene?.getBounds() {
                    self.ox = -1 * bounds.midX
                    self.oy = -1 * bounds.midY
                }
                self.pivotPoint = CGPoint(x:0, y:0)
                self.scheduleRedraw()
            }, ops)
            return
        }
    }
    
    @objc public func copyItemAsHTML(_ sender: NSMenuItem) {
        if self.activeItems.count == 1 {
            if let actItem = self.activeItems.first, let scene = self.scene, let itm = scene.drawables[actItem] {
                itm.traverse {itm in
                    if let textBox = itm as? TextBox {
                        do {
                            let attrString = textBox.attrStr
                            let dta = try attrString.data(from: NSMakeRange(0, attrString.length), documentAttributes: [ NSAttributedString.DocumentAttributeKey.documentType:NSAttributedString.DocumentType.html ])
                            if let strValue = String(data: dta, encoding: String.Encoding.utf8) {
                                ClipboardUtils.copyHtml(strValue)
                            }
                        }
                        catch _ {
                            // Ignore
                        }
                    }
                    return true
                }
            }
        }
    }
    
    @objc func alignLeadingEdges(_ sender: NSMenuItem) {
        if self.activeItems.count == 0 {
            return
        }
        var ops: [ElementOperation] = []
        var leftPos = CGFloat(roundf(Float(self.activeItems[0].x)))
        for active in self.activeItems {
            if active.kind != .Item {
                           continue
                       }
            if active.x < leftPos  {
                leftPos = CGFloat(roundf(Float(active.x)))
            }
        }
        for active in self.activeItems {
            if active.kind != .Item {
                continue
            }
            let newPos = CGPoint( x: leftPos, y: active.y)
            ops.append(store!.createUpdatePosition(item: active, newPos: newPos))
        }
        if ops.count > 0 {
            store?.compositeOperation(notifier: self.element!, undoManaget: self.undoManager, refresh: scheduleRedraw, ops)
            scheduleRedraw()
            return
        }
    }
    
    @objc func moveItemForward(_ sender: NSMenuItem) {
        if self.activeItems.count != 1 {
            return
        }
        var ops: [ElementOperation] = []
        
        for active in self.activeItems {
            ops.append(contentsOf: store!.createUpdateOrder(item: active, newPos: nil))
        }
        if ops.count > 0 {
            store?.compositeOperation(notifier: self.element!, undoManaget: self.undoManager, refresh: scheduleRedraw, ops)
            scheduleRedraw()
            return
        }
    }
    @objc func moveItemBackward(_ sender: NSMenuItem) {
        if self.activeItems.count != 1 {
            return
        }
        var ops: [ElementOperation] = []
        
        for active in self.activeItems {
            ops.append(contentsOf: store!.createUpdateOrder(item: active, newPos: 0))
        }
        if ops.count > 0 {
            store?.compositeOperation(notifier: self.element!, undoManaget: self.undoManager, refresh: scheduleRedraw, ops)
            scheduleRedraw()
            return
        }
    }
    
    @objc func selectItemAtPoint(_ sender: NSMenuItem) {
        if let itm = sender.representedObject as? DiagramItem {
            setActiveItem(itm)
        }
    }
    
    @objc func alignTrailingEdges(_ sender: NSMenuItem) {
        if self.activeItems.count == 0 {
            return
        }
        var ops: [ElementOperation] = []
        var rightPos = CGFloat(roundf(Float(self.activeItems[0].x)))
        for active in self.activeItems {
            if active.kind != .Item {
                continue
            }
            if let dr = self.scene?.drawables[active] {
                let bnds = dr.getBounds()
                if active.x + bnds.width > rightPos  {
                    rightPos = CGFloat(roundf(Float(active.x + bnds.width)))
                }
            }
        }
        for active in self.activeItems {
            if active.kind != .Item {
                continue
            }
            if let dr = self.scene?.drawables[active] {
                let bnds = dr.getBounds()
                let newPos = CGPoint( x: rightPos - bnds.width, y: active.y)
                ops.append(store!.createUpdatePosition(item: active, newPos: newPos))
            }
        }
        if ops.count > 0 {
            store?.compositeOperation(notifier: self.element!, undoManaget: self.undoManager, refresh: scheduleRedraw, ops)
            scheduleRedraw()
            return
        }
    }
    
    @objc func alignTopEdges(_ sender: NSMenuItem) {
        if self.activeItems.count == 0 {
            return
        }
        var ops: [ElementOperation] = []
        var topPos = CGFloat(roundf(Float(self.activeItems[0].y)))
        for active in self.activeItems {
            if active.kind != .Item {
                continue
            }
            if active.y  > topPos  {
                topPos = CGFloat(roundf(Float(active.y )))
            }
        }
        for active in self.activeItems {
            if active.kind != .Item {
                continue
            }
            let newPos = CGPoint( x: active.x, y: topPos )
            ops.append(store!.createUpdatePosition(item: active, newPos: newPos))
        }
        if ops.count > 0 {
            store?.compositeOperation(notifier: self.element!, undoManaget: self.undoManager, refresh: scheduleRedraw, ops)
            scheduleRedraw()
            return
        }
    }
    @objc func alignBottomEdges(_ sender: NSMenuItem) {
        if self.activeItems.count == 0 {
            return
        }
        var ops: [ElementOperation] = []
        var bottomPos = CGFloat(roundf(Float(self.activeItems[0].y)))
        for active in self.activeItems {
            if active.kind != .Item {
                continue
            }
            if let dr = self.scene?.drawables[active] {
                let bnds = dr.getBounds()
                if active.y - bnds.height < bottomPos  {
                    bottomPos = CGFloat(roundf(Float(active.y - bnds.height)))
                }
            }
        }
        for active in self.activeItems {
            if active.kind != .Item {
                continue
            }
            if let dr = self.scene?.drawables[active] {
                let bnds = dr.getBounds()
                let newPos = CGPoint( x: active.x, y: bottomPos + bnds.height)
                ops.append(store!.createUpdatePosition(item: active, newPos: newPos))
            }
        }
        if ops.count > 0 {
            store?.compositeOperation(notifier: self.element!, undoManaget: self.undoManager, refresh: scheduleRedraw, ops)
            scheduleRedraw()
            return
        }
    }
    
    @objc func alignCenterVertical(_ sender: NSMenuItem) {
        if self.activeItems.count == 0 {
            return
        }
        var ops: [ElementOperation] = []
        var topPos = CGFloat(roundf(Float(self.activeItems[0].y)))
        for active in self.activeItems {
            if active.kind != .Item {
                continue
            }
            if let dr = self.scene?.drawables[active] {
                let bnds = dr.getBounds()
                if active.y + bnds.height > topPos  {
                    topPos = CGFloat(roundf(Float(active.y + bnds.height)))
                }
            }
        }
        for active in self.activeItems {
            if active.kind != .Item {
                continue
            }
            if let dr = self.scene?.drawables[active] {
                let bnds = dr.getBounds()
                let newPos = CGPoint( x: active.x, y: topPos - bnds.height)
                ops.append(store!.createUpdatePosition(item: active, newPos: newPos))
            }
        }
        if ops.count > 0 {
            store?.compositeOperation(notifier: self.element!, undoManaget: self.undoManager, refresh: scheduleRedraw, ops)
            scheduleRedraw()
            return
        }
    }
    
    fileprivate func createAlighMenu(_ menu: NSMenu) {
        let align = NSMenuItem(
            title: "Align", action: nil, keyEquivalent: "")
        menu.addItem(align)
        
        let menu = NSMenu()
        menu.autoenablesItems=true
        
        menu.addItem(NSMenuItem( title: "Leading Edges", action: #selector(alignLeadingEdges(_:)), keyEquivalent: ""))
        menu.addItem(NSMenuItem( title: "Trailing Edges", action: #selector(alignTrailingEdges(_:)), keyEquivalent: ""))
        menu.addItem(NSMenuItem( title: "Top Edges", action: #selector(alignTopEdges(_:)), keyEquivalent: ""))
        menu.addItem(NSMenuItem( title: "Bottom Edges", action: #selector(alignBottomEdges(_:)), keyEquivalent: ""))
        menu.addItem(NSMenuItem.separator())
        //        menu.addItem(
        
        menu.setSubmenu(menu, for: align)
    }
    
    fileprivate func createOrderMenu(_ menu: NSMenu) {
        let order = NSMenuItem(
            title: "Order", action: nil, keyEquivalent: "")
        menu.addItem(order)
        
        let menu = NSMenu()
        menu.autoenablesItems=true
        
        menu.addItem(NSMenuItem( title: "Move Forward", action: #selector(moveItemForward(_:)), keyEquivalent: ""))
        menu.addItem(NSMenuItem( title: "Move Backward", action: #selector(moveItemBackward(_:)), keyEquivalent: ""))
        menu.addItem(NSMenuItem.separator())
        //        menu.addItem(
        
        menu.setSubmenu(menu, for: order)
    }
    
    fileprivate func createSelection(_ menu: NSMenu) {
        
        let drawablesLess = findElement(x: self.x, y: self.y)
        let drawables = findElement(x: self.x, y: self.y, allowAll: true)
        if drawablesLess.count == drawables.count && drawables.count <= 1 {
            return
        }
        
        menu.addItem(NSMenuItem.separator())
        
        let selectItem = NSMenuItem(
            title: "Select", action: nil, keyEquivalent: "")
        menu.addItem(selectItem)
        
        let menu = NSMenu()
        menu.autoenablesItems=true
        
        for dr in drawables {
            if let itm = dr.item {
                let mi = NSMenuItem( title: itm.name, action: #selector(selectItemAtPoint(_:)), keyEquivalent: "")
                mi.representedObject = itm
                menu.addItem(mi)
            }
        }
        menu.setSubmenu(menu, for: selectItem)
    }
    
    override func menu(for event: NSEvent) -> NSMenu? {
        if event.buttonNumber != 1 {
            return nil
        }
        self.updateMousePosition(event)
        
        self.dragMap.removeAll()
        self.dragElements.removeAll()
        
        // Check if some of figures are already
        var checkSelection = true
        let drawables = findElement(x: self.x, y:  self.y)
        for itm in drawables {
            if let itm = itm.item {
                if self.activeItems.contains(itm) {
                    checkSelection = false
                }
            }
        }
        
        if checkSelection {
            if let drawable = findElement(x: self.x, y:  self.y).first, let itm = drawable.item {
                if !self.activeItems.contains(itm) {
                    self.setActiveItem(itm, immideateDraw: true)
                }
            }
            else {
                self.setActiveItem(nil, immideateDraw: true)
            }
        }
        
        let addAction = NSMenuItem(title: "New item", action: #selector(addTopItm), keyEquivalent: "")
        
        if self.activeItems.count > 0 {
            let menu = NSMenu()
            
            let addLinkedAction = NSMenuItem(
                title: "New linked item", action: #selector(addNewItemNoCopy), keyEquivalent: "")
            let addLinkedCopyAction = NSMenuItem(
                title: "Linked styled item", action: #selector(addNewItemCopy), keyEquivalent: "")
            let deleteAction = NSMenuItem(
                title: "Delete", action: #selector(removeItmAction), keyEquivalent: "")
            
            let duplicateAction = NSMenuItem(
                title: "Duplicate", action: #selector(duplicateItem), keyEquivalent: "")
            
            menu.addItem(addAction)
            menu.addItem(NSMenuItem.separator())
            menu.addItem(addLinkedAction)
            menu.addItem(addLinkedCopyAction)
            menu.addItem(NSMenuItem.separator())
            createStylesMenu(menu)
            createQuickStyleMenu(menu)
            if self.activeItems.count > 1 {
                menu.addItem(NSMenuItem.separator())
                createAlighMenu(menu)
            }
            menu.addItem(NSMenuItem.separator())
            menu.addItem(duplicateAction)
            
            if self.activeItems.count == 1 {
                menu.addItem(NSMenuItem.separator())
                menu.addItem(NSMenuItem(
                    title: "Attach image", action: #selector(attachImage), keyEquivalent: ""))
                self.createOrderMenu(menu)
                
                menu.addItem(NSMenuItem.separator())
                let copyHtml = NSMenuItem(title: "Export text as html", action: #selector(copyItemAsHTML(_:)), keyEquivalent: "")
                menu.addItem(copyHtml)
                
            }
            self.createSelection(menu)
            menu.addItem(NSMenuItem.separator())
            menu.addItem(deleteAction)
            
            return menu
        }
        else {
            
            self.pivotPoint = CGPoint(x: self.x, y: self.y)
            // No items selected.
            let menu = NSMenu()
            
            let addAction = NSMenuItem(title: "New item", action: #selector(addTopItm), keyEquivalent: "")
            menu.addItem(addAction)
            
            //            let testLayout = NSMenuItem(title: "Test layout", action: #selector(performSpringLayout), keyEquivalent: "")
            //            menu.addItem(testLayout)
            
            menu.addItem(NSMenuItem.separator())
            self.createStylesMenu(menu)
            self.createSelection(menu)
            
            return menu
        }
    }
}