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
    
    var model:ElementModel?
    
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
    
    var actionExecutor: UndoActionExecutor?
    
    @objc override func touchesBegan(with event: NSEvent) {
        if self.mode == .Editing || self.mode == .LineDrawing {
            return
        }
        let touches = event.touches(matching: NSTouch.Phase.touching, in: self)
        if touches.count == 2 {
            prevTouch = touches.first
        }
    }
    
    fileprivate func sheduleRedraw( invalidRect: CGRect? = nil ) {
        if !self.drawScheduled {
            drawScheduled = true
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.01, execute: {
                if let rect = invalidRect {
                    self.setNeedsDisplay(rect)
                }
                else {
                    self.needsDisplay = true
                }
                self.drawScheduled = false
            })
        }
    }
    
    @objc override func touchesMoved(with event: NSEvent) {
        if self.mode == .Editing || self.mode == .LineDrawing {
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
    
    public func setModel( model:ElementModel ) {
        if let oldModel = self.model {
            oldModel.onUpdate.removeAll()
        }
        
        if let um = self.undoManager, self.actionExecutor == nil {
            um.removeAllActions()
            actionExecutor = UndoActionExecutor(um, self)
        }
        
        self.model = model
        
        self.model?.onUpdate.append( {(element, kind) in
            // We should be smart anought to not rebuild all drawable scene every time
            if kind == .Structure  {
                self.buildScene()
                self.needsDisplay = true
            }
        })
        
    }
    
    public func setActiveElement(_ elementModel: Element ) {
        
        // Discard any editing during switch
        self.commitTitleEditing(nil)
        
        self.element = elementModel
        self.activeElement = nil
        
        
        // Center diagram to fit all items
        
        self.buildScene()
        
        if let bounds = scene?.getBounds() {
            var freeX = self.frame.width - bounds.width
            var freeY = self.frame.height - bounds.height
            if freeX < 0 {
                freeX = 0
            }
            if freeY < 0 {
                freeY = 0
            }
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
                        active.name = textValue
                        self.model?.modified(element!, .Structure)
                    }
                }
            }
            
            self.setNormalMode()
            textBox.removeFromSuperview()
            self.editBox = nil
            self.window?.makeFirstResponder(self)
            needsDisplay = true
        }
    }
    
    fileprivate func editTitle(_ active: DiagramItem) {
        self.mode = .Editing
        scene?.editingMode = true
        if let de = scene?.drawables[active] {
            if editBox != nil {
                editBox!.removeFromSuperview()
            }
            let deBounds = de.getBounds()
            let bounds = CGRect(
                x: deBounds.origin.x + scene!.offset.x,
                y: deBounds.origin.y + scene!.offset.y,
                width: max(deBounds.width, 100),
                height: deBounds.height
            )
            Swift.debugPrint(bounds)
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
        needsDisplay = true
    }
    
    fileprivate func addNewItem() {
        if let active = self.activeElement {
            if active.kind == .Item {
                // Create and add to activeEl
                let newEl = DiagramItem(kind: .Item, name: "Untitled \(createIndex)")
                self.createIndex += 1
                newEl.x = active.x + 100
                newEl.y = active.y
                
                self.element?.add(source: active, target: newEl)
                
                self.setActiveElement(newEl)
                
                sheduleRedraw()
            }
        }
        else {
            // Add top element
            let newEl = DiagramItem(kind: .Item, name: "Untitled \(createIndex)")
            self.createIndex += 1
            
            newEl.x = 0
            newEl.y = 0
            self.element?.add(newEl)
            
            sheduleRedraw()
        }
    }
    
    override func keyDown(with event: NSEvent) {
        if event.characters == "\t" {
            addNewItem()
        }
    
        if event.characters == "\u{0D}" {
            if let active = self.activeElement, active.kind == .Item  {
                editTitle(active)
            }
        }
        else if event.characters == "\u{7f}" { // Backspace character
            if let active = self.activeElement  {
                active.parent?.remove(active)
                sheduleRedraw()
            }
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
                    if element?.items.first(where: { (item) -> Bool in
                        if item.kind == .Link, let data: LinkElementData = item.getData(.LinkData) {
                            if (data.source == source && data.target == target) ||
                                (data.source == target && data.target == source) {
                                // Existing item found
                                return true
                            }
                        }
                        return false
                    }) == nil {
                        // Add item since not pressent
                        element?.add(source: source, target: target)
                    }
                }
                
                scene?.removeLineTo()
                self.lineToPoint = nil
                self.lineTarget = nil
            }
            else {
                if let newPos = self.dragMap.removeValue(forKey: de), let parent = de.parent {
                    if let ae = actionExecutor {
                        ae.execute(UpdatePosition(self.model!, parent, de, old: CGPoint(x: de.x, y: de.y), new: newPos))
                    }
                    else {
                        de.x = newPos.x
                        de.y = newPos.y
                        parent.model?.modified(parent, .Layout)
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
            return
        }
        
        self.mouseDownState = true
        
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
                
                needsDisplay = true
            }
            else {
                if let pos = self.dragMap[de] {
                    let newPos = CGPoint(x: pos.x + event.deltaX, y:pos.y - event.deltaY)
                    self.dragMap[de] = newPos
                
                    if let em = self.element {
                        em.model?.modified(em, .Layout)
                        self.scene?.updateLayout(de, newPos)
                    }
                }
            }
        }
        else {
            ox += event.deltaX
            oy -= event.deltaY
        }
        
        sheduleRedraw()
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
    
    override func draw(_ dirtyRect: NSRect) {
        
        if( self.element == nil) {
            return
        }
        
        if let context = NSGraphicsContext.current?.cgContext, let scene = self.scene  {
            // Draw background
            
            context.setFillColor(background)
            context.fill(dirtyRect)
            
            scene.offset = CGPoint(x: self.ox + bounds.midX, y: self.oy + bounds.midY)
            
            context.saveGState()
            context.saveGState()
//            context.setShadow(offset: CGSize(width: 2, height:-2), blur: 4, color: CGColor(red:0,green:0,blue:0,alpha: 0.5))
//            scene.drawBox(context: context)
            context.restoreGState()
            scene.layout(bounds)
            
            // TODO: Add dirty rect filteting
            scene.draw(context: context)
            context.restoreGState()
        }
    }
}
