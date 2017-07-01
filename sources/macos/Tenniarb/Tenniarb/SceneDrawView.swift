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
    var elementModel: Element?
    
    var activeElement: DiagramItem?
    
    var dragElement: DiagramItem?
    
    var x: CGFloat = 0
    var y: CGFloat = 0
    
    var ox: CGFloat = 0
    var oy: CGFloat = 0
    
    var trackingArea: NSTrackingArea? = nil
    
    var mouseDownState = false
    
    var elementRects:[DiagramItem: CGRect] = [:]
    
    override var mouseDownCanMoveWindow: Bool {
        get {
            return false
        }
    }
    
    
    public func setElementModel(_ elementModel: Element ) {
        self.elementModel = elementModel
        self.activeElement = nil
        elementRects.removeAll()
        
        needsDisplay = true
    }
    
    public func setActiveElement( _ element: DiagramItem? ) {
        activeElement = element
        needsDisplay = true
    }
    
    public func findElement(el: Element, x: CGFloat, y: CGFloat) -> DiagramItem? {
        for item in el.items {
            if let r = elementRects[item] {
                if( item.x < x && x < item.x + r.width &&
                    item.y < y && y < item.y + r.height ) {
                    return item
                }
            }
        }
        return nil
    }
    
    
    override func mouseUp(with event: NSEvent) {
        Swift.debugPrint("mouseUp")
        self.updateMousePosition(event)
        
        self.mouseDownState = false
        self.dragElement = nil
    }
    
    override func mouseDown(with event: NSEvent) {
        Swift.debugPrint("mouseDown")
        self.updateMousePosition(event)
        
        self.mouseDownState = true
        
        
        if let em = elementModel {
            let item = findElement(el: em, x: self.x, y: self.y)
            if( item != nil) {
                activeElement = item
                
                self.dragElement = item
            }
            else {
                activeElement = nil
            }
        }
        needsDisplay = true
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

//        if !mouseDownState {
//            if let em = elementModel {
//                let el = findElement(el: em, x: self.x, y: self.y)
//                if( el != nil) {
//                    activeElement = el
//                }
//                else {
//                    activeElement = nil
//                }
//            }
//            needsDisplay = true
//        }
    }
    
    override func draw(_ dirtyRect: NSRect) {
        
        if( self.elementModel == nil) {
            return
        }
        
        if let context = NSGraphicsContext.current?.cgContext {
            // Draw background
            context.setFillColor(background)
            context.fill(bounds)
            
            let scene = DrawableScene()
            
            scene.offset = CGPoint(x: self.ox + bounds.midX, y: self.oy + bounds.midY)
            
            if let element = elementModel {
                scene.append(buildElements(element: element))
            }
            
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
    func buildElements( element: Element )-> Drawable {
        let elementDrawable = DrawableElement()
        
        var links: [DiagramItem] = []
        for e in element.items {
            var active = false
            if let ae = activeElement {
                if ae.id == e.id {
                    active = true
                }
            }
            
            
            if e.kind == .Element && e.element != nil {
                let textBox = TextBox( text: e.element!.name,  textColor: CGColor(red: 0.147, green: 0.222, blue: 0.162, alpha: 1.0)  )
                
                let textBounds = textBox.getBounds()
                
                let rectBox = RoundBox( bounds: CGRect(x: e.x, y:e.y, width: textBounds.width, height: textBounds.height),
                                        fillColor: CGColor.white,
                                        borderColor: CGColor.black)
                
                if active {
                    rectBox.lineWidth = 1
                }
                rectBox.append(textBox)
                
                elementDrawable.append(rectBox)
                
                let rectBounds = rectBox.getBounds()
                
                elementRects[e] = CGRect(origin: CGPoint(x: rectBounds.origin.x + self.ox, y: rectBounds.origin.y + self.oy), size: rectBounds.size)
            }
            else if e.kind == .Link  {
                links.append(e)
            }
        }
        for e in links {
            if let s = e.source, let t = e.target {
                let sourceRect = elementRects[s]
                let targetRect = elementRects[t]
                
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
