//
//  SceneDrawView.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 26/05/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//


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
    override public func keyUp(with event: NSEvent) {
        super.keyDown(with: event)
    }
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
            if lastInvalidRect != nil && invalidRect != nil {
                lastInvalidRect = lastInvalidRect!.union(invalidRect!)
            }
        }
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
        
        NotificationCenter.default.addObserver(self, selector: #selector(defaultsChanged), name: UserDefaults.didChangeNotification, object: nil)        
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
            Swift.debugPrint("Animate: \(delay) count:\(count)")
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
        
        var oldActiveItem: [DiagramItem] = []
        var oldEditMode: Bool = false
        if let oldScene = self.scene {
            oldActiveItem = oldScene.activeElements
            oldEditMode = oldScene.editingMode
        }
        
        let darkMode = PreferenceConstants.preference.isDiagramDarkMode()
        
        let scene = DrawableScene(self.element!, darkMode: darkMode, executionContext: self.store!.executionContext)
        
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
    
    func changeItemProps( _ property: String, _ value: TennNode) {
        guard let itm = self.activeItems.first, activeItems.count == 1 else {
            return
        }
        
        let newItemProps = itm.toTennAsProps(.BlockExpr)
        var changed = false
        if let itmProp = newItemProps.getNamedElement(property) {
            // Property exists, we need to replace value
            if itmProp.getIdent(1) != value.getIdentText() {
                itmProp.children?.removeAll()
                itmProp.add(TennNode.newIdent(property), value)
                changed = true
            }
        }
        else {
            // Just add new property
            newItemProps.add(TennNode.newCommand(property, value))
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
    @objc func markerMenuAction( _ sender: NSMenuItem ) {
        let value = sender.title
        changeItemProps("marker", TennNode.newStrNode(value))
    }
    @objc func lineWidthAction( _ sender: NSMenuItem ) {
        let value = sender.title
        changeItemProps("line-width", TennNode.newIdent(value))
    }
    
    @objc func colorMenuAction( _ sender: NSMenuItem ) {
        let value = sender.title
        changeItemProps("color", TennNode.newIdent(value))
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
    
    fileprivate func showPopup() {
        if self.popupView != nil {
            self.popupView?.removeFromSuperview()
            self.popupView = nil
        }
        
        if self.mode == .Editing {
            return
        }
        
        guard let act = self.activeItems.first, activeItems.count == 1, let dr = scene?.drawables[act] else {
            return
        }
        let bounds = dr.getSelectorBounds()
        let origin =  CGPoint(x: scene!.offset.x + bounds.origin.x, y: scene!.offset.y + bounds.origin.y + bounds.height)
        
        let segments = NSSegmentedControl(frame: CGRect(x: 0, y: 0, width: 300, height: 48))
        segments.segmentStyle = .texturedRounded
        segments.segmentCount = 10
        
//        segments.action = #selector(segmentAction(_:))

        var segm = -1
        if act.kind == .Item {
            segm += 1
            segments.setLabel("✑", forSegment: segm)
            
            let menu = NSMenu()
            
            let smiles = menu.addItem(withTitle: "😀 Emoji", action: nil, keyEquivalent: "")
            smiles.submenu = createMenu(selector: #selector(markerMenuAction(_:)), items: ["😀","😛","😱","😵","😷","🐶","🐱","🐭","🐰","🦊","🌻","🌧","🌎","🔥","❄️","💦","☂️"])
            
            let numbers = menu.addItem(withTitle: "🔢 Numbers", action: nil, keyEquivalent: "")
            numbers.submenu = createMenu(selector: #selector(markerMenuAction(_:)), items: ["0️⃣","1️⃣","2️⃣","3️⃣","4️⃣","5️⃣","6️⃣","7️⃣","8️⃣","9️⃣","🔟"])
            
            let objects = menu.addItem(withTitle: "🖥 Objects", action: nil, keyEquivalent: "")
            objects.submenu = createMenu(selector: #selector(markerMenuAction(_:)), items: ["⌚️","🖥","🖨","⌛️","⏰","⚒","🧲","💣","🔒","✂️","🧸","🎁"])
            
            let symbols = menu.addItem(withTitle: "🔠 Symbols", action: nil, keyEquivalent: "")
            symbols.submenu = createMenu(selector: #selector(markerMenuAction(_:)), items: ["🆗","🆖","#️⃣","🔤","ℹ️","🚻","🔃","➕","➖","➗","✖️","♾","💲","✔️","♠️","♣️","♥️","♦️"])
            
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
                createMenu(selector: #selector(fontMenuAction(_:)),
                           items: ["8", "10", "12", "14", "16", "18", "20", "22", "26", "32", "36"]),
                forSegment: segm)
            if #available(OSX 10.13, *) {
                segments.setShowsMenuIndicator(true, forSegment: segm)
                
            }
            segments.setWidth(36, forSegment: segm)
            
        }
        
        segm += 1
//        segments.setLabel("Display", forSegment: segm)
        segments.setImage(NSImage(named: NSImage.flowViewTemplateName), forSegment: segm)
        segments.setImageScaling(.scaleProportionallyUpOrDown, forSegment: segm)
        if act.kind == .Item {
            segments.setMenu(
                createMenu(selector: #selector(displayMenuAction(_:)),
                           items: itemDisplayVariants ),
                forSegment: segm)
        } else {
            segments.setMenu(
                createMenu(selector: #selector(displayMenuAction(_:)),
                           items: linkDisplayVariants),
                forSegment: segm)
        }
        if #available(OSX 10.13, *) {
            segments.setShowsMenuIndicator(true, forSegment: segm)
        }
        segments.setWidth(36, forSegment: segm)
        
        if act.kind == .Item {
            segm += 1
            segments.setLabel("✎", forSegment: segm)
            segments.setMenu(
                createMenu(selector: #selector(colorMenuAction(_:)),
                           items: ["red", "green", "blue", "yellow", "orange", "brown", "blue", "lightblue", "purple"]),
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
            createMenu(selector: #selector(lineStyleMenuAction(_:)),
                       items: lineStyleDisplayVariants),
            forSegment: segm)
        if #available(OSX 10.13, *) {
            segments.setShowsMenuIndicator(true, forSegment: segm)
            
        }
        segments.setWidth(36, forSegment: segm)
        
        segm += 1
        segments.setLabel("〰", forSegment: segm)
        segments.setMenu(
            createMenu(selector: #selector(lineWidthAction(_:)),
                       items: ["0.3", "0.5", "1", "1.5", "2", "5"]),
            forSegment: segm)
        if #available(OSX 10.13, *) {
            segments.setShowsMenuIndicator(true, forSegment: segm)
            
        }
        segments.setWidth(36, forSegment: segm)
        
        segments.segmentCount = segm + 1
        
        segments.trackingMode = .momentary
        
//        segments.sizeToFit()
        let popup  = NSView(frame: NSRect(origin: origin, size: segments.bounds.size))
        self.popupView = popup
        popup.addSubview(segments)
        
        let shadow = NSShadow()
        shadow.shadowOffset = NSSize(width: -5, height: -5)
        shadow.shadowBlurRadius = 7
        shadow.shadowColor = NSColor(red: 0, green: 0, blue: 0, alpha: 0.7)
        popup.shadow = shadow
        
        segments.allowedTouchTypes = []
        
        popup.allowedTouchTypes = []
        popupItem = act
        
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.2, execute: {
            if self.popupView != nil && self.popupView == popup {
                self.addSubview(popup)
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
        if finalBounds != nil {
            return finalBounds!
        }
        return self.frame
    }
    
    func getEditBoxBounds( item: Drawable ) -> CGRect {
        var deBounds = self.editBoxItem!.getBounds()
        if let link =  item as? DrawableLine {
            deBounds = link.getLabelBounds()
        }
        let bounds = CGRect(
            x: deBounds.origin.x + scene!.offset.x,
            y: deBounds.origin.y + scene!.offset.y,
            width: max(deBounds.width, 100),
            height: max(deBounds.height, 20)
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
    
    func getBody( _ item: DiagramItem, _ style: DrawableStyle ) -> (String, CGFloat) {
        let bodyStyle = style.copy()
        bodyStyle.fontSize -= 2 // Make a bit smaller for body
        var textValue = ""
        
        SceneDrawView.getBodyText(item, bodyStyle, &textValue)
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
    
    fileprivate func editTitle(_ active: DiagramItem, _ editMode: EditingMode) {
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
        
        let bounds = getEditBoxBounds(item: de)
        editBox = PopupEditField(frame: bounds)
        scene?.editBoxBounds = bounds
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
            editBox?.font = NSFont.systemFont(ofSize: style.fontSize)
        case .Body:
            let (text, fontSize) = self.getBody(active, style)
            editBox?.stringValue = text
            editBox?.font = NSFont.systemFont(ofSize: fontSize)
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
    
    @objc func duplicateItem() {
        guard let curElement = self.element else {
            return
        }
        var items: [DiagramItem] = []
        var oldNewItems: [DiagramItem:DiagramItem] = [:]
        var links: [DiagramItem] = []
        
        let offsetx = CGFloat(75)
        let offsety = CGFloat(-25)
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
                    if processedLinks.contains(active.id.uuidString) {
                        continue
                    }
                    processedLinks.append(active.id.uuidString)
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
        }
    
        if event.characters == "\u{0D}" {
            if let active = self.activeItems.first  {
                setActiveItem(active)
                if event.modifierFlags.contains(NSEvent.ModifierFlags.shift) {
                    editTitle(active, .Body)
                }
                else {
                    editTitle(active, .Name)
                }
            }
        }
        else if event.characters == "\u{7f}" {
            removeItem()
        } else if event.characters == " " {
            self.viewController?.showOperationBox()
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
            }
        }
//        Swift.debugPrint("Keycode:", event.keyCode, " characters: ", event.characters)
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
    
    public func findElement(x: CGFloat, y: CGFloat) -> [ItemDrawable] {
        let point = CGPoint(x: x, y: y)
        
        return self.scene?.find(point) ?? []
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
                    if self.mode != .DiagramMove && drawables.count == 0 {
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
        
        if event.clickCount == 2 && drawables.count == 1 && result.count == 1 {
            editTitle(drawables[0].item!, .Name)
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
                    if selBox.intersects(db) {
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
                    self.lineToPoint = CGPoint(x: self.x, y: self.y )
                    self.lineTarget = scene?.updateLineTo( de, self.lineToPoint! )
                    
                    scheduleRedraw()
                }
                else {
                    if let pos = self.dragMap[de], (de.kind == .Item || self.dragElements.count == 1) {
                        let newPos = CGPoint(x: pos.x + event.deltaX, y:pos.y - event.deltaY)
                        self.dragMap[de] = newPos
                        newPositions[de] = newPos
                    }
                }
            }
            if newPositions.count > 0 {
                let dirtyRegion = self.scene!.updateLayout(newPositions)
                
                let p = CGPoint(x: self.ox + bounds.midX + dirtyRegion.origin.x, y: self.oy + bounds.midY + dirtyRegion.origin.y)
                scheduleRedraw(invalidRect: CGRect(origin: p, size: CGSize(width: dirtyRegion.size.width, height: dirtyRegion.size.height)))
            }
        }
        else {
            ox += event.deltaX
            oy -= event.deltaY
            
            self.mode = .DiagramMove
            
            scheduleRedraw()
        }
    }
    
    func updateMousePosition(_ event: NSEvent) {
        let wloc = event.locationInWindow
        
        let vp = self.convert(wloc, from: nil)
        let pos = CGPoint(x: vp.x - bounds.midX - ox, y: vp.y - bounds.midY - oy )
        
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
            var rect = bounds.insetBy(dx: -30, dy: -50)
            
            rect.size = CGSize(width: max(rect.width, popupBounds.width), height: rect.height)
            
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
        
        
        let ystep = bounds.height / CGFloat(ycount)
        let xstep = bounds.width / CGFloat(xcount)
        
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
            if x > bounds.width {
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
            if y > bounds.height {
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
                    newy = bounds.height - CGFloat(cr)*cx - 7
                }
                context.addEllipse(in: CGRect(x: bounds.width - 7 - CGFloat(cr)*cx, y: newy, width: 5 + CGFloat(cr)*cx, height: 5 + CGFloat(cr)*cx))
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
                context.addEllipse(in: CGRect(x: newx, y: bounds.height - 7 - CGFloat(cr)*cy, width: 5 + CGFloat(cr)*cy, height: 5 + CGFloat(cr)*cy))
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
        
        if let context = NSGraphicsContext.current?.cgContext, let scene = self.scene  {
            context.saveGState()
            // Draw background
            context.setFillColor( PreferenceConstants.preference.background)
            
            context.setShouldAntialias(true)
            context.fill(self.bounds)
//            context.stroke(dirtyRect, width: 1)
            
            scene.offset = CGPoint(x: self.ox + bounds.midX, y: self.oy + bounds.midY)
            
            let sceneDirty = CGRect(
                origin: CGPoint(x: dirtyRect.origin.x - scene.offset.x, y: dirtyRect.origin.y-scene.offset.y),
                size:dirtyRect.size
            )
            
            context.saveGState()
            
            scene.layout(bounds, sceneDirty)
            scene.draw(context: context)
            context.restoreGState()
            
            drawRulers(scene, context)
            
            context.restoreGState()
        }
    }
    
    public func selectAllItems() {
        self.setActiveItems(self.element!.items)
        scheduleRedraw()
    }
    public func selectNoneItems() {
        self.setActiveItems([])
        scheduleRedraw()
    }
    
    
    /// Selectors
    
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
            
            NSPasteboard.general.clearContents()
            let value = block.toStr()
            NSPasteboard.general.setString(value, forType: .string)
        }
    }
    
    @objc func paste( _ sender: NSObject ) {      
        if let value = NSPasteboard.general.string(forType: .string) {
            let p = TennParser()
            let node = p.parse(value)
            if p.errors.hasErrors() {
                return // If there is errors, we could not paste.
            }
            let items = Element.parseItems(node: node)
            if items.count > 0 {
                // Move items a bit,
                for i in items {
                    if i.kind == .Item {
                        i.x += 10
                        i.y -= 10
                    }
                }
                self.store?.add(self.element!, items, undoManager: self.undoManager, refresh: self.scheduleRedraw)
                
                self.setActiveItems(items)
                scheduleRedraw()
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
                    do {
                        let data:NSData = try NSData(contentsOf: filename)
                        let encoded = data.base64EncodedString()
                        
                        if let active = self.activeItems.first {
                            let newProps = active.properties.clone()
                            
                            var imgNode = newProps.get("image")
                            if imgNode == nil {
                                imgNode = TennNode.newCommand("image", TennNode.newStrNode(filename.lastPathComponent), TennNode.newStrNode(encoded))
                                newProps.append(imgNode!)
                            }
                            else {
                                imgNode?.children = [TennNode.newIdent("image"), TennNode.newStrNode(filename.lastPathComponent), TennNode.newStrNode(encoded)]
                            }
                            self.store?.setProperties(self.element!, active, newProps.asNode(),
                                                        undoManager: self.undoManager,  refresh: {()->Void in})
                        }
                    }
                    catch {
                        Swift.debugPrint("Error saving file")
                    }
                }
            }
        }
    }
    
    
    fileprivate func createStylesMenu(_ menu: NSMenu) {
        let style = NSMenuItem(
            title: "Style", action: nil, keyEquivalent: "")
        menu.addItem(style)
        menu.setSubmenu(styleManager?.createMenu(), for: style)
    }
    
    override func menu(for event: NSEvent) -> NSMenu? {
        if event.buttonNumber != 1 {
            return nil
        }
        self.updateMousePosition(event)
        
        self.dragMap.removeAll()
        self.dragElements.removeAll()
        
        if let drawable = findElement(x: self.x, y:  self.y).first, let itm = drawable.item {
            if !self.activeItems.contains(itm) {
                self.setActiveItem(itm, immideateDraw: true)
            }
        }
        else {
            self.setActiveItem(nil, immideateDraw: true)
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
            menu.addItem(NSMenuItem.separator())
            menu.addItem(duplicateAction)
            
//            if self.activeItems.count == 1 {
//                menu.addItem(NSMenuItem.separator())
//                menu.addItem(NSMenuItem(
//                    title: "Attach image", action: #selector(attachImage), keyEquivalent: ""))
//            }
 
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
            
            menu.addItem(NSMenuItem.separator())
            createStylesMenu(menu)
            
            return menu
        }
    }
}
