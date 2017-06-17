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
    
    class DisplayItem {
        var rect: CGRect
        var textFontAttributes: [NSAttributedStringKey:Any]
        var x: CGFloat
        var y: CGFloat
        var textColor: NSColor
        var active: Bool
        var text: String
        var drawUsingCoreText = false
        var attrString: NSAttributedString
        
        init( text: String, x: CGFloat, y: CGFloat, active: Bool) {
            self.x = x
            self.y = y
            self.active = active
            self.text = text
            
            let font = NSFont.systemFont(ofSize: 24)
            
            let textStyle = NSMutableParagraphStyle.default.mutableCopy() as! NSMutableParagraphStyle
            textStyle.alignment = NSTextAlignment.center
            
            self.textColor = NSColor(calibratedRed: 0.147, green: 0.222, blue: 0.162, alpha: 1.0)
            
            self.textFontAttributes = [
                NSAttributedStringKey.foregroundColor: textColor,
                NSAttributedStringKey.paragraphStyle: textStyle,
                NSAttributedStringKey.font: font
            ]
            
            self.attrString = NSAttributedString(string: text, attributes: textFontAttributes)
            
            let fs = CTFramesetterCreateWithAttributedString(attrString)
            let frameSize = CTFramesetterSuggestFrameSizeWithConstraints(fs, CFRangeMake(0, attrString.length), nil, CGSize(width: 150, height: 45), nil)
            
            self.rect = CGRect(x: x, y:y, width: frameSize.width + 10, height: frameSize.height + 8 )
        }
        
        func draw( context: CGContext ) {
            drawRoundedRect(rect: rect,
                            inContext: context,
                            radius: CGFloat(9),
                            borderColor: CGColor.black,
                            fillColor: CGColor.white,
                            text: text,
                            active: active)
        }
        func drawRoundedRect(rect: CGRect, inContext context: CGContext,
                             radius: CGFloat,
                             borderColor: CGColor,
                             fillColor: CGColor,
                             text: String,
                             active: Bool = false) {
            
            context.saveGState()
            
            let path = CGMutablePath()
            
            path.move( to: CGPoint(x:  rect.midX, y:rect.minY ))
            path.addArc( tangent1End: CGPoint(x: rect.maxX, y: rect.minY ),
                         tangent2End: CGPoint(x: rect.maxX, y: rect.maxY), radius: radius)
            path.addArc( tangent1End: CGPoint(x: rect.maxX, y: rect.maxY ),
                         tangent2End: CGPoint(x: rect.minX, y: rect.maxY), radius: radius)
            path.addArc( tangent1End: CGPoint(x: rect.minX, y: rect.maxY ),
                         tangent2End: CGPoint(x: rect.minX, y: rect.minY), radius: radius)
            path.addArc( tangent1End: CGPoint(x: rect.minX, y: rect.minY ),
                         tangent2End: CGPoint(x: rect.maxX, y: rect.minY), radius: radius)
            path.closeSubpath()
            
            context.setShadow(offset: CGSize(width: 2, height:-2), blur: 4, color: CGColor(red:0,green:0,blue:0,alpha: 0.5))
            
            context.setLineWidth( 0 )
            context.setStrokeColor(borderColor)
            context.setFillColor(fillColor)
            
            context.addPath(path)
            context.drawPath(using: .fillStroke)
            
            
            context.setShadow(offset: CGSize(width:0, height:0), blur: CGFloat(0))
            
            
            if ( active ) {
                context.setLineWidth( 0.75 )
                context.setStrokeColor(borderColor)
                context.setFillColor(fillColor)
                
                context.addPath(path)
                context.drawPath(using: .stroke)
            }
            
            if !self.drawUsingCoreText {
                let q: NSString = text as NSString
            
                q.draw(at: CGPoint(x: rect.minX+5, y: rect.minY+4), withAttributes: textFontAttributes)
            }
            else {
                let ctLine = CTLineCreateWithAttributedString(self.attrString)
                context.textPosition = CGPoint(x: rect.minX+5, y: rect.minY+8)
                CTLineDraw(ctLine, context)
            }
            
            context.restoreGState()
        }
    }
    override func draw(_ dirtyRect: NSRect) {
        
        if( self.elementModel == nil) {
            return
        }
        
        if let context = NSGraphicsContext.current?.cgContext {
            // Draw background
            context.setFillColor(background)
            context.fill(bounds)
//
//            let blurBackground = CGLayer(context, size: bounds.size, auxiliaryInfo: nil)
//            if let blur = blurBackground {
//                if let blurCtx = blur.context {
//                    if let parent = elementModel!.parent {
//                        drawElements(element: parent, blurCtx, useCoreText: true )
//                    }
//
//                }
//
//                context.saveGState()
//                context.setBlendMode(CGBlendMode.luminosity)
//                context.draw(blur, in: bounds)
//                context.restoreGState()
//
//            }
            
            if let element = elementModel {
                drawElements(element: element, context)
            }
        }
    }
    func drawElements( element: Element, _ context: CGContext, useCoreText:Bool = false ) {
        var els: [DisplayItem] = []
        
        for e in element.items {
            let yy: CGFloat = CGFloat(e.y) + bounds.midY + oy
            let xx: CGFloat = CGFloat(e.x) + bounds.midX + ox
            
            var active = false
            if let ae = activeElement {
                if ae.id == e.id {
                    active = true
                }
            }
            
            if e.kind == .Element && e.element != nil {
                let de = DisplayItem( text: e.element!.name,  x: xx, y: yy, active: active )
                
                els.append(de)
                elementRects[e] = de.rect
            }
        }
        
        // Draw links after all item positions are known.
        for e in element.items {
            if e.kind == .Link {
                if let s = e.source, let t = e.target {
                    let sourceRect = elementRects[s]
                    let targetRect = elementRects[t]
                    
                    if let sr = sourceRect, let tr = targetRect {
                        //
                        context.saveGState()
                        
                        context.setLineWidth( 1 )
                        context.setShadow(offset: CGSize(width: 2, height:-2), blur: 4, color: CGColor(red:0,green:0,blue:0,alpha: 0.5))
                        context.setStrokeColor(CGColor.black)
                        context.setFillColor(CGColor.black)
                        
                        let aPath = CGMutablePath()
                        
                        aPath.move(to: CGPoint(x: sr.midX, y: sr.midY))
                        
                        aPath.addLine(to: CGPoint(x:tr.midX, y: tr.midY))
                        
                        //Keep using the method addLineToPoint until you get to the one where about to close the path
                        aPath.closeSubpath()
                        context.addPath(aPath)
                        context.drawPath(using: .fillStroke)
                        
                        context.restoreGState()
                    }
                }
            }
        }
        
        for de in els {
            de.drawUsingCoreText = useCoreText
            de.draw(context: context)
        }
    }
}
