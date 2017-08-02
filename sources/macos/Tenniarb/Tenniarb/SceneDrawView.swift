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
    
    var dirty: Bool = false
    
    
//    override init(frame:CGRect) {
//        super.init(frame: frame)
//    }
//
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
    
//    override func scrollWheel(with event: NSEvent) {
//        ox += event.deltaX*2
//        oy -= event.deltaY*2
//        sheduleRedraw()
//    }
    
    
    fileprivate func sheduleRedraw() {
        if !self.needsDisplay {
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.01, execute: {
                self.needsDisplay = true
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
    
//    @objc func detectPan(_ recognizer:NSPanGestureRecognizer) {
//        let translation  = recognizer.translation(in: self.superview)
//
//        ox += translation.x / 2
//        oy -= translation.y / 2
//        needsDisplay = true
//    }
    
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
            scene.offset = CGPoint(x: self.ox + bounds.midX, y: self.oy + bounds.midY)
            scene.layout(bounds)
            
            context.saveGState()
            context.saveGState()
//            context.setShadow(offset: CGSize(width: 2, height:-2), blur: 4, color: CGColor(red:0,green:0,blue:0,alpha: 0.5))
//            scene.drawBox(context: context)
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
        
        let bgColor = CGColor(red: 1.0, green:1.0, blue:1.0, alpha: 0.7)
        
        let textBox = TextBox(
            text: name ?? "empty",
            textColor: CGColor(red: 0.147, green: 0.222, blue: 0.162, alpha: 1.0),
            fontSize: 18)
        
        let textBounds = textBox.getBounds()
        
        let rectBox = RoundBox( bounds: CGRect(x: e.x, y:e.y, width: textBounds.width, height: textBounds.height),
                                fillColor: bgColor,
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
    
    /**
         Calc point to cross two lines.
     */
    func crossLine( _ p1: CGPoint, _ p2: CGPoint, _ p3: CGPoint, _ p4: CGPoint) -> CGPoint? {
        let d = (p1.x - p2.x) * (p4.y - p3.y) - (p1.y - p2.y) * (p4.x - p3.x)
        let da = (p1.x - p3.x) * (p4.y - p3.y) - (p1.y - p3.y) * (p4.x - p3.x)
        let db = (p1.x - p2.x) * (p1.y - p3.y) - (p1.y - p2.y) * (p1.x - p3.x)
        
        let ta = da / d;
        let tb = db / d;
        
        if ta >= 0 && ta <= 1 && tb >= 0 && tb <= 1
        {
            let dx = p1.x + ta * (p2.x - p1.x)
            let dy = p1.y + ta * (p2.y - p1.y)
            
            return CGPoint(x: dx, y: dy)
        }
        
        return nil
    }
    func crossBox( _ p1:CGPoint, _ p2: CGPoint, _ rect: CGRect)-> CGPoint? {
        let op = rect.origin
        
        //0,0 -> 1,0
        if let cp = crossLine( p1, p2, CGPoint(x: op.x, y: op.y), CGPoint( x: op.x + rect.width, y: op.y) ) {
            return cp
        }
        
        // 0,0 -> 0, 1
        if let cp = crossLine( p1, p2, CGPoint(x: op.x, y: op.y), CGPoint( x: op.x, y: op.y + rect.height) ) {
            return cp
        }
        
        // 0,1 -> 1, 1
        if let cp = crossLine( p1, p2, CGPoint(x: op.x, y: op.y + rect.height), CGPoint( x: op.x + rect.width, y: op.y + rect.height) ) {
            return cp
        }
        // 1,0 -> 1,1
        if let cp = crossLine( p1, p2, CGPoint(x: op.x + rect.width, y: op.y), CGPoint( x: op.x + rect.width, y: op.y + rect.height) ) {
            return cp
        }
        return nil
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
                    
                    let p1 = CGPoint( x: sr.midX, y:sr.midY )
                    let p2 = CGPoint( x: tr.midX, y:tr.midY )
                    
                    
                    if let cp1 = crossBox(p1, p2, sr), let cp2 = crossBox(p1, p2, tr) {
                        elementDrawable.insert(
                            DrawableLine(
                                source: cp1,
                                target: cp2), at: 0)
                    }
                }
            }
        }
        return elementDrawable
    }
}
