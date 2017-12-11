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


class SceneDrawView: NSView {
    let background = CGColor(red: 253/255, green: 246/255, blue: 227/255, alpha:0.3)
    
    var model:ElementModel?
    
    var element: Element?
    
    var activeElement: DiagramItem?
    
    var dragElement: DiagramItem?
    
    var createIndex: Int = 1
    
    var x: CGFloat = 0
    var y: CGFloat = 0
    
    var ox: CGFloat = 0
    var oy: CGFloat = 0
    
    var mouseDownState = false
    
    var onSelection: [( DiagramItem? ) -> Void] = []
    
    var scene: DrawableScene?
    
    var dirty: Bool = false
    
    var drawScheduled: Bool = false
    
    required init?(coder aDecoder: NSCoder) {
        super.init(coder: aDecoder)
        self.acceptsTouchEvents=true
    }
    
    var prevTouch: NSTouch? = nil
    
    @objc override func touchesBegan(with event: NSEvent) {
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
        self.model = model
        
        self.model?.onUpdate.append( {(element, kind) in
                // We should be smart anought to not rebuild all drawable scene every time
                if kind == .Structure  {
                    self.buildScene()
                }
            })
    }
    
    public func setActiveElement(_ elementModel: Element ) {
        self.element = elementModel
        self.activeElement = nil
        
        self.buildScene()
        
        needsDisplay = true
    }
    
    private func buildScene() {
        let scene = DrawableScene(self.element!)
    
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
        scene?.update()
        
        needsDisplay = true
    }
    
    override func keyDown(with event: NSEvent) {
        if event.characters == "\t" {
            if let active = self.activeElement {
                if active.kind == .Item {
                    // Create and add to activeEl
                    let newEl = DiagramItem(kind: .Item, name: "Untitled \(createIndex)")
                    self.createIndex += 1
                    newEl.x = active.x + 100
                    newEl.y = active.y
                    
                    self.element?.add(source: active, target: newEl)
                    
//                    active.add(newEl)
//                    self.element?.add(source: active, target: newEl)
                    
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
        else if event.characters == "\u{7f}" { // Backspace character
            if let active = self.activeElement, active.name.count > 0 {
                active.name.removeLast()
                self.model?.modified(element!, .Structure)
                if let drawable = scene?.drawables[active] {
                    let drBounds = drawable.getBounds()
                    let off = CGPoint(x: self.ox + bounds.midX, y: self.oy + bounds.midY)
                    let rect = CGRect(x: drBounds.minX + off.x-10, y: drBounds.minY + off.y-10, width: drBounds.width + 50, height: drBounds.height+20)
                    sheduleRedraw(invalidRect: rect)
                }
                else {
                    sheduleRedraw()
                }
            }
        }
        else if event.characters == " " {
            if let active = self.activeElement  {
                if let drawable = scene?.drawables[active] {
                    let drBounds = drawable.getBounds()
                    let off = CGPoint(x: self.ox + bounds.midX, y: self.oy + bounds.midY)
                    let rect = CGRect(x: drBounds.minX + off.x, y: drBounds.minY + off.y, width: drBounds.width, height: drBounds.height)
                    showPopover(bounds: rect)
                }
            }
        }
        else {
            if let active = self.activeElement, let chars = event.characters  {
                active.name += chars
                self.model?.modified(element!, .Structure)
  
                //TODO: Need to add associated links to invalid rect bounds
                if let drawable = scene?.drawables[active] {
                    let drBounds = drawable.getBounds()
                    let off = CGPoint(x: self.ox + bounds.midX, y: self.oy + bounds.midY)
                    let rect = CGRect(x: drBounds.minX + off.x-10, y: drBounds.minY + off.y-10, width: drBounds.width + 50, height: drBounds.height+20)
                    sheduleRedraw(invalidRect: rect)
                }
                else {
                    sheduleRedraw()
                }
            }
        }
        Swift.debugPrint("Keycode:", event.keyCode, " characters: ", event.characters)
    }
    
    private func showPopover(bounds: CGRect) {
        let controller = NSViewController()
        controller.view = NSView(frame: CGRect(x: CGFloat(100), y: CGFloat(50), width: CGFloat(100), height: CGFloat(50)))
        
        let popover = NSPopover()
        popover.contentViewController = controller
        popover.contentSize = controller.view.frame.size
        
        popover.behavior = .transient
        popover.animates = true
        
        // let txt = NSTextField(frame: NSMakeRect(100,50,50,22))
        let txt = NSTextField(frame: controller.view.frame)
        txt.stringValue = "Hello world"
        txt.textColor = NSColor.black.withAlphaComponent(0.95)
        controller.view.addSubview(txt)
        txt.sizeToFit()
        popover.show(relativeTo: bounds, of: self as! NSView, preferredEdge: NSRectEdge.maxY)
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
        
        self.mouseDownState = false
        self.dragElement = nil
    }
    
    override func mouseDown(with event: NSEvent) {
        self.updateMousePosition(event)
        
        self.mouseDownState = true
        
        
        if let drawable = findElement(x: self.x, y: self.y) {
            self.setActiveElement(drawable.item)
            
            self.dragElement = drawable.item
        }
        else {
            self.setActiveElement(nil)
        }
    }
    
    override var acceptsFirstResponder: Bool {
        get {
            return true
        }
    }
    
    override func mouseDragged(with event: NSEvent) {
        self.updateMousePosition(event)
        
        if let de = dragElement {
            de.x += event.deltaX
            de.y -= event.deltaY
                        
            if let em = self.element {
                em.model?.modified(em, .Layout)
                self.scene?.updateLayout(de)
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
