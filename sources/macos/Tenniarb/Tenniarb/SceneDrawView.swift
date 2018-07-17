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
            self.view.commitTitleEditing(textView)
            return true
        }
        // TODO: Resize both text and drawed item to fit value smoothly.
        
        return false
    }
}

class SceneDrawView: NSView {
    let background = CGColor(red: 253/255, green: 246/255, blue: 227/255, alpha:0.3)
    
    var store: ElementModelStore?
    
    var element: Element?
    
    var activeElement: DiagramItem?
    
    var dragElement: DiagramItem?
    
    var dragMap:[DiagramItem: CGPoint] = [:]
    
    var lineToPoint: CGPoint?
    var lineTarget: DiagramItem?
    
    var createIndex: Int = 1
    
    var x: CGFloat = 0
    var y: CGFloat = 0
    
    var mode: SceneMode = .Normal
    
    var editBox: NSTextField? = nil
    var editBoxItem: Drawable? = nil
    var editBoxDelegate: EditTitleDelegate?
    
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
    
    var onSelection: [( DiagramItem? ) -> Void] = []
    
    var scene: DrawableScene?
    
    var drawScheduled: Bool = false
    
    required init?(coder aDecoder: NSCoder) {
        super.init(coder: aDecoder)
        self.acceptsTouchEvents=true
    }
    
    var prevTouch: NSTouch? = nil
    
    @objc override func touchesBegan(with event: NSEvent) {
        if self.mode == .Editing || self.mode == .LineDrawing || self.mode == .Dragging {
            return
        }
        let touches = event.touches(matching: NSTouch.Phase.touching, in: self)
        if touches.count == 2 {
            prevTouch = touches.first
        }
    }
    
//    override func scrollWheel(with event: NSEvent) {
//        Swift.debugPrint("Scroll", event)
//    }
    
    func sheduleRedraw() {
        sheduleRedraw(invalidRect: nil)
    }
    var lastInvalidRect: CGRect? = nil
    fileprivate func sheduleRedraw( invalidRect: CGRect? ) {
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
        let touches = event.touches(matching: NSTouch.Phase.touching, in: self)
        if touches.count == 2 {
            if prevTouch == nil {
                prevTouch = touches.first
                return
            }
            var touch: NSTouch? = nil
            for t in touches {
                if t.identity.isEqual(prevTouch?.identity) {
                    touch = t
                    break
                }
            }
            if touch != nil {
                let np1 = prevTouch!.normalizedPosition
                let np2 = touch!.normalizedPosition
                
                self.ox += (np2.x-np1.x)*prevTouch!.deviceSize.width*2.5
                self.oy += (np2.y-np1.y)*prevTouch!.deviceSize.height*2.5
                
                sheduleRedraw()
            }
            prevTouch = touch
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
    
    func onLoad() {
    }
    
    func onUpdate(_ evt: ModelEvent) {
        // We should be smart anought to not rebuild all drawable scene every time
        
        if evt.items.count > 0 {
            if let firstItem = evt.items.first(where: { (itm) in itm.kind == .Item } ) {
                if evt.element.items.contains(firstItem) {
                    setActiveElement(firstItem)
                }
                else {
                    setActiveElement(nil)
                }
            }
        }
        if evt.kind == .Structure  {
            self.buildScene()
            sheduleRedraw()
        }
        else {
            sheduleRedraw()
        }
    }
    
    public func setModel( store: ElementModelStore ) {
        if let oldStore = self.store {
            oldStore.onUpdate.removeAll()
        }
        
        if let um = self.undoManager {
            um.removeAllActions()
        }
        
        self.store = store
        
        self.store?.onUpdate.append( self.onUpdate )
    }
    
    public func setActiveElement(_ elementModel: Element ) {
        
        if self.element == elementModel {
            return
        }
        // Discard any editing during switch
        self.commitTitleEditing(nil)
        
        self.element = elementModel
        self.activeElement = nil
        
        
        // Center diagram to fit all items
        
        self.buildScene()
        
        if let bounds = scene?.getBounds() {
//            var freeX = self.frame.width - bounds.width
//            var freeY = self.frame.height - bounds.height
//            if freeX < 0 {
//                freeX = 0
//            }
//            if freeY < 0 {
//                freeY = 0
//            }
            self.ox = -1 * bounds.midX
            self.oy = -1 * bounds.midY
        }
        
        needsDisplay = true
    }
    
    private func buildScene() {
        // We need preserve selection of previous scene
        
        var oldActiveItem: DiagramItem? = nil
        var oldEditMode: Bool = false
        if let oldScene = self.scene {
            oldActiveItem = oldScene.activeElement
            oldEditMode = oldScene.editingMode
        }
        
        let scene = DrawableScene(self.element!)
        
        if let active = oldActiveItem {
            scene.activeElement = active
            scene.editingMode = oldEditMode
        }
    
        self.scene = scene
    }
    
    public func setActiveElement( _ element: DiagramItem? ) {
        if activeElement == nil && element == nil {
            return
        }
        activeElement = element
        
        for f in onSelection {
            f(element)
        }
        
        // We need to rebuild scene as active element is changed
        scene?.activeElement = element
        
        needsDisplay = true
    }
    
    fileprivate func commitTitleEditing(_ textView: NSTextView?) {
        if let textBox = self.editBox {
            if let active = self.activeElement {
                if let tv = textView {
                    let textValue = tv.string
                    if textValue.count > 0 {
                        self.store?.updateName(item: active, textValue, undoManager: self.undoManager, refresh: sheduleRedraw)
                    }
                }
            }
            
            self.setNormalMode()
            textBox.removeFromSuperview()
            self.editBox = nil
            self.editBoxItem = nil
            self.window?.makeFirstResponder(self)
            needsDisplay = true
        }
    }
    
    fileprivate func getEditBoxBounds( item: Drawable ) -> CGRect {
        let deBounds = self.editBoxItem!.getBounds()
        let bounds = CGRect(
            x: deBounds.origin.x + scene!.offset.x,
            y: deBounds.origin.y + scene!.offset.y,
            width: max(deBounds.width, 100),
            height: deBounds.height
        )
        return bounds
    }
    fileprivate func editTitle(_ active: DiagramItem) {
        self.mode = .Editing
        scene?.editingMode = true
        if let de = scene?.drawables[active] {
            if editBox != nil {
                editBox!.removeFromSuperview()
            }
            self.editBoxItem = de
            
            let bounds = getEditBoxBounds(item: de)
            editBox = NSTextField(frame: bounds)
            if self.editBoxDelegate == nil {
                self.editBoxDelegate = EditTitleDelegate(self)
            }
            
            let style = DrawableItemStyle.parseStyle(item: active)
            
            
            editBox?.delegate = self.editBoxDelegate
            editBox?.stringValue = active.name
            editBox?.drawsBackground = true
            editBox?.isBordered = true
            editBox?.focusRingType = .none
            editBox?.font = NSFont.systemFont(ofSize: style.fontSize)
            
            self.addSubview(editBox!)
            
            self.window?.makeFirstResponder(editBox!)
        }
        
        sheduleRedraw()
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
        
        newEl.x = 0
        newEl.y = 0
        self.store?.add(self.element!, newEl, undoManager: self.undoManager, refresh: self.sheduleRedraw)
    }
    func addNewItem(copyProps:Bool = false) {
        if let active = self.activeElement {
            if active.kind == .Item {
                // Create and add to activeEl
                let newEl = DiagramItem(kind: .Item, name: "Untitled \(createIndex)")
                self.createIndex += 1
                
                var offset = CGFloat(100.0)
                if let dr = scene?.drawables[active] {
                    offset = CGFloat(dr.getBounds().width + 10)
                }
                
                newEl.x = active.x + offset
                newEl.y = active.y
                
                
                if copyProps {
                    // Copy parent properties
                    for p in active.properties {
                        newEl.properties.append(p.clone())
                    }
                }
                
                self.store?.add(self.element!, source: active, target: newEl, undoManager: self.undoManager, refresh: self.sheduleRedraw)
                self.setActiveElement(newEl)
                sheduleRedraw()
            }
        }
        else {
           self.addTopItem()
        }
    }
    
    func duplicateItem() {
        if let active = self.activeElement {
            if active.kind == .Item {
                // Create and add to activeEl
                let newEl = DiagramItem(kind: .Item, name: active.name )
                newEl.description = active.description
                
                var offset = CGFloat(100.0)
                if let dr = scene?.drawables[active] {
                    offset = CGFloat(dr.getBounds().width + 10)
                }
                
                newEl.x = active.x + offset
                newEl.y = active.y
                
                
                // Copy parent properties
                for p in active.properties {
                    newEl.properties.append(p.clone())
                }
                
                self.store?.add(self.element!, newEl, undoManager: self.undoManager, refresh: self.sheduleRedraw)
                self.setActiveElement(newEl)
                sheduleRedraw()
            }
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
        if let active = self.activeElement  {
            self.store?.remove(active.parent!, item: active, undoManager: self.undoManager, refresh: self.sheduleRedraw)
            sheduleRedraw()
        }
    }
    
    
    
    override func keyDown(with event: NSEvent) {
        if event.characters == "\t" {
            addNewItem(copyProps: event.modifierFlags.contains(NSEvent.ModifierFlags.option))
        }
    
        if event.characters == "\u{0D}" {
            if let active = self.activeElement, active.kind == .Item  {
                editTitle(active)
            }
        }
        else if event.characters == "\u{7f}" {
            removeItem()
        }
//        else if event.characters == " " {
//            if let active = self.activeElement  {
//                if let drawable = scene?.drawables[active] {
//                    let drBounds = drawable.getBounds()
//                    let off = CGPoint(x: self.ox + bounds.midX, y: self.oy + bounds.midY)
//                    let rect = CGRect(x: drBounds.minX + off.x, y: drBounds.minY + off.y, width: drBounds.width, height: drBounds.height)
//                    showPopover(bounds: rect)
//                }
//            }
//        }
//        Swift.debugPrint("Keycode:", event.keyCode, " characters: ", event.characters)
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
    
    public func findElement(x: CGFloat, y: CGFloat) -> ItemDrawable? {
        let point = CGPoint(x: x, y: y)
        
        if let drawable = self.scene?.find(point) {
            return drawable
        }
        
        return nil
    }
    
    
    override func mouseUp(with event: NSEvent) {
        self.updateMousePosition(event)
        
        if self.mode == .Editing {
            // No dragging allowed until editing is not done
            return
        }
        
        if let de = self.dragElement {
            if self.mode == .LineDrawing {
                
                if let source = self.dragElement, let target = self.lineTarget {
                    // Create a new line if not yet pressent between elements
                    
                    // Check if line already exists
//                    if element?.items.first(where: { (item) -> Bool in
//                        if item.kind == .Link, let data = item as? LinkItem {
//                            if (data.source == source && data.target == target) {
//                                // Existing item found
//                                return true
//                            }
//                        }
//                        return false
//                    }) == nil {
                        // Add item since not pressent
                        store?.add(element!, source:source, target: target, undoManager: self.undoManager, refresh: self.sheduleRedraw, props: [TennNode.newCommand("display", TennNode.newStrNode("arrow"))])
//                    }
                }
                
                scene?.removeLineTo()
                self.lineToPoint = nil
                self.lineTarget = nil
            }
            else {
                if let newPos = self.dragMap.removeValue(forKey: de) {
                    var pos = newPos
//                    if event.modifierFlags.contains(NSEvent.ModifierFlags.shift) {
//                        // This is snap to grid of 5/5
//                        pos = CGPoint(x: Int(pos.x) - Int(pos.x) % 5,
//                                         y: Int(pos.y) - Int(pos.y) % 5)
//                    }
                    if pos.x != de.x || pos.y != de.y {
                        self.store?.updatePosition(item: de, newPos: pos, undoManager: self.undoManager, refresh: sheduleRedraw)
                    }
                }
                self.setActiveElement(de)
            }
            needsDisplay = true
        }
        
        self.mouseDownState = false
        if let de = self.dragElement {
            self.dragMap.removeValue(forKey: de)
        }
        self.dragElement = nil
        
        self.mode = .Normal
    }
    
    override func mouseDown(with event: NSEvent) {
        self.updateMousePosition(event)
        
        if self.mode == .Editing {
            // No dragging allowed until editing is not done
            self.commitTitleEditing(nil)
        }
        
        self.mouseDownState = true
        self.dragMap.removeAll()
        self.dragElement = nil
        
        if let drawable = findElement(x: self.x, y: self.y) {
            self.setActiveElement(drawable.item)
            scene?.activeElement = nil
            
            self.dragElement = drawable.item
                        
            if event.modifierFlags.contains(NSEvent.ModifierFlags.command) {
                self.mode = .LineDrawing
                self.lineToPoint = CGPoint(x: self.x, y: self.y )
            }
            else {
                self.mode = .Dragging
                self.dragMap[self.dragElement!] = CGPoint(x: self.dragElement!.x, y: self.dragElement!.y)
            }
        }
        else {
            self.setActiveElement(nil)
            self.mode = .DiagramMove
        }
    }
    
    override var acceptsFirstResponder: Bool {
        get {
            return true
        }
    }
    
    override func mouseDragged(with event: NSEvent) {
        if self.mode != .Dragging && self.mode != .DiagramMove && self.mode != .LineDrawing  {
            return
        }
        
        self.updateMousePosition(event)
        
        if let de = dragElement {
            
            if self.mode == .LineDrawing {
                self.lineToPoint = CGPoint(x: self.x, y: self.y )
                self.lineTarget = scene?.updateLineTo( de, self.lineToPoint! )
                
                sheduleRedraw()
            }
            else {
                if let pos = self.dragMap[de] {
                    let newPos = CGPoint(x: pos.x + event.deltaX, y:pos.y - event.deltaY)
                    self.dragMap[de] = newPos
                
                    if let em = self.element {
                        self.store?.modified(ModelEvent(kind: .Layout, element: em))
                        let dirtyRegion = self.scene!.updateLayout(de, newPos)
                        
                        let p = CGPoint(x: self.ox + bounds.midX + dirtyRegion.origin.x-20, y: self.oy + bounds.midY + dirtyRegion.origin.y - 20)
                        sheduleRedraw(invalidRect: CGRect(origin: p, size: CGSize(width: dirtyRegion.size.width + 40, height: dirtyRegion.size.height + 40)))
                    }
                }
            }
        }
        else {
            ox += event.deltaX
            oy -= event.deltaY
            sheduleRedraw()
        }
    }
    
    func updateMousePosition(_ event: NSEvent) {
        //TODO: It is so dirty hack. Also devider positions are missied few pixel
        
        let wloc = event.locationInWindow
        
        let sv = self.superview?.superview
        
        let sv2 = sv?.superview
        
        let treeBounds = sv2?.subviews[0].bounds
        
        let textarea = sv?.subviews[1].bounds
        
        self.x = (wloc.x - treeBounds!.width - bounds.midX) - ox
        self.y = (wloc.y - bounds.midY - textarea!.height) - oy
    }
    
    override func mouseMoved(with event: NSEvent) {
        if self.mode == .Editing {
            return
        }
        self.updateMousePosition(event)
    }
    
    override func viewWillStartLiveResize() {
        if self.mode == .Editing {
            commitTitleEditing(nil)
        }
    }
    
    override func draw(_ dirtyRect: NSRect) {
        if( self.element == nil) {
            return
        }
        
        if let context = NSGraphicsContext.current?.cgContext, let scene = self.scene  {
            // Draw background
            
            context.setFillColor(background)
            context.fill(dirtyRect)
            
            scene.offset = CGPoint(x: self.ox + bounds.midX, y: self.oy + bounds.midY)
            
            let sceneDirty = CGRect(
                origin: CGPoint(x: dirtyRect.origin.x - scene.offset.x, y: dirtyRect.origin.y-scene.offset.y),
                size:dirtyRect.size
            )
            
//            context.saveGState()
            context.saveGState()
//            context.setShadow(offset: CGSize(width: 2, height:-2), blur: 4, color: CGColor(red:0,green:0,blue:0,alpha: 0.5))
//            scene.drawBox(context: context)
//            context.restoreGState()
            scene.layout(bounds, sceneDirty)
            
            // TODO: Add dirty rect filteting
            scene.draw(context: context)
            context.restoreGState()
        }
    }
}
