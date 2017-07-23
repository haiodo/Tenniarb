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
    let background = CGColor(red: 253/255, green: 246/255, blue: 227/255, alpha:0.7)
    
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
    
    var onSelection: [( Element? ) -> Void] = []
    
    var scene: DrawableScene?
    
    override var mouseDownCanMoveWindow: Bool {
        get {
            return false
        }
    }
    
    func onLoad() {
    }
    
    public func setModel( model:ElementModel ) {
        self.model = model
        
        self.model?.onUpdate.append( {(element) in
                // We should be smart anought to not rebuild all drawable scene every time
                self.buildScene()
            })
    }
    
    public func setActiveElement(_ elementModel: Element ) {
        self.element = elementModel
        self.activeElement = nil
        
        self.buildScene()
        
        needsDisplay = true
    }
    
    private func buildScene() {
        let scene = DrawableScene()
        
        if let element = self.element {
            scene.append(buildElementScene(element))
        }
        
        
        self.scene = scene
    }
    
    public func setActiveElement( _ element: DiagramItem? ) {
        if activeElement == nil && element == nil {
            return
        }
        activeElement = element
        
        if element == nil {
            for f in onSelection {
                f(nil)
            }
        }
        else if let e = element {
            if e.kind == .Element && e.data.refElement != nil {
                for f in onSelection {
                    f(e.data.refElement)
                }
            }
        }
        
        // We need to rebuild scene as active element is changed
        buildScene()
        
        needsDisplay = true
    }
    
    override func keyDown(with event: NSEvent) {
        if event.characters == "\t" {
            if let active = self.activeElement {
                if active.kind == .Element {
                    // Create and add to activeEl
                    let newEl = DiagramItem(kind: .Element, name: "Untitled \(createIndex)")
                    self.createIndex += 1
                    newEl.x = active.x + 100
                    newEl.y = active.y
                    
                    self.element?.add(newEl)
                    
                    self.element?.add(source: active, target: newEl)
                    
                
                    needsDisplay = true
                }
            }
            else {
                // Add top element
                let newEl = Element(name: "Root")
                if let di = self.element?.add(newEl, createLink: false) {
                    di.x = 0
                    di.y = 0
                }
                
                needsDisplay = true
            }
        }
    }
    
    public func findElement(x: CGFloat, y: CGFloat) -> Drawable? {
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
                em.model?.modified(em)
            }
            
            needsDisplay = true
        }
        else {
            ox += event.deltaX
            oy -= event.deltaY
            needsDisplay = true
        }
    }
    
    func updateMousePosition(_ event: NSEvent) {
        
        //TODO: It is so dirty hack. Also devider positions are missied few pixel
        
        let wloc = event.locationInWindow
        
        let sv = self.superview as? NSSplitView
        
        let sv2 = sv?.superview as? NSSplitView
        
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
            context.fill(bounds)
            
            scene.offset = CGPoint(x: self.ox + bounds.midX, y: self.oy + bounds.midY)
            scene.layout(bounds)
            
            context.saveGState()
            context.saveGState()
            context.setShadow(offset: CGSize(width: 2, height:-2), blur: 4, color: CGColor(red:0,green:0,blue:0,alpha: 0.5))
            scene.drawBox(context: context)
            context.restoreGState()
            scene.draw(context: context)
            context.restoreGState()
        }
    }
    fileprivate func buildItemDrawable(_ e: DiagramItem, _ active: Bool, _ drawables: inout [DiagramItem : Drawable], _ elementDrawable: DrawableContainer) {
        var name = e.name
        
        // Referenced element name should be from reference
        if e.data.refElement != nil {
            name = e.data.refElement!.name
        }
        
        let textBox = TextBox(
            text: name ?? "empty",
            textColor: CGColor(red: 0.147, green: 0.222, blue: 0.162, alpha: 1.0),
            fontSize: 18)
        
        let textBounds = textBox.getBounds()
        
        let rectBox = RoundBox( bounds: CGRect(x: e.x, y:e.y, width: textBounds.width, height: textBounds.height),
                                fillColor: CGColor.white,
                                borderColor: CGColor.black)
        
        if active {
            rectBox.lineWidth = 1
        }
        rectBox.append(textBox)
        
        rectBox.item = e
        
        drawables[e] = rectBox
        
        elementDrawable.append(rectBox)
    }
    
    func isActive(_ item: DiagramItem) -> Bool {
        var active = false
        if let ae = activeElement {
            if ae.id == item.id {
                active = true
            }
        }
        return active
    }
    
    fileprivate func buildItems(_ items: [DiagramItem], _ drawables: inout [DiagramItem : Drawable], _ elementDrawable: DrawableContainer, _ links: inout [DiagramItem]) {
        for e in items {
            if e.kind == .Element {
                buildItemDrawable(e, self.isActive(e), &drawables, elementDrawable)
                
                if let itms = e.items {
                    buildItems(itms, &drawables, elementDrawable, &links)
                    // Also need to add a linkes to any if items from e
                    for itm in itms {
                        let linkItem = DiagramItem(kind: .Link, data: LinkElementData(source: e, target: itm ))
                        links.append(linkItem)
                    }
                }
            }
            else if e.kind == .Link  {
                links.append(e)
            }
        }
    }
    
    func buildElementScene( _ element: Element )-> Drawable {
        let elementDrawable = DrawableContainer()
        
        var links: [DiagramItem] = []
        
        var drawables: [DiagramItem: Drawable] = [:]
        buildItems(element.items, &drawables, elementDrawable, &links)
        for e in links {
            if let data = e.data as? LinkElementData {
                let sourceRect = drawables[data.source]?.getBounds()
                let targetRect = drawables[data.target]?.getBounds()
                
                if let sr = sourceRect, let tr = targetRect {
                    elementDrawable.insert(
                        DrawableLine(
                            source: CGPoint( x: sr.midX, y:sr.midY),
                            target: CGPoint( x: tr.midX, y:tr.midY)), at: 0)
                }
            }
        }
        return elementDrawable
    }
}
