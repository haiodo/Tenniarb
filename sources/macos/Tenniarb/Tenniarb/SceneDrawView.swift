//
//  SceneDrawView.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 26/05/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//

import Cocoa
import CoreText


class SceneDrawView: NSView {
    let background = CGColor(red: 253/255, green: 246/255, blue: 227/255, alpha:1)
    var elementModel: Element?
    
    var activeElement: Element?
    
    var dragElement: Element?
    
    var x: CGFloat = 0
    var y: CGFloat = 0
    
    var trackingArea: NSTrackingArea? = nil
    
    var mouseDownState = false
    
    override var mouseDownCanMoveWindow: Bool {
        get {
            return false
        }
    }
    
    
    public func setElementModel(_ elementModel: Element ) {
        self.elementModel = elementModel
        needsDisplay = true
    }
    
    public func setActiveElement( _ element: Element? ) {
        activeElement = element
        needsDisplay = true
    }
    
    
    func collectElements(el: Element, elements: inout [Element]) {
        
        if elementModel != nil && !(elementModel is ElementModel) {
            elements.append(el)
        }
        elements.append(contentsOf: el.elements)
        for e in el.elements {
            elements.append(e)
//            collectElements(el: e, elements: &elements)
        }
    }
    
    public func findElement(el: Element, x: CGFloat, y: CGFloat) -> Element? {
        
        if( el.x < x && x < el.x + 150 &&
            el.y < y && y < el.y + 50 ) {
            return el
        }

        for e in el.elements {
            let result = findElement(el: e, x: x, y: y)
            if( result != nil) {
                return result
            }
        }
        return nil
    }
    
    
    override func mouseUp(with event: NSEvent) {
        Swift.debugPrint("mouseUp")
        self.updateMousePosition()
        
        self.mouseDownState = false
        self.dragElement = nil
    }
    
    override func mouseDown(with event: NSEvent) {
        Swift.debugPrint("mouseDown")
        self.updateMousePosition()
        
        self.mouseDownState = true
        
        
        if let em = elementModel {
            let el = findElement(el: em, x: self.x, y: self.y)
            if( el != nil) {
                activeElement = el
                
                self.dragElement = el
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
        self.updateMousePosition()
        
        if let de = dragElement {
            de.x += event.deltaX
            de.y -= event.deltaY
            
            needsDisplay = true
        }
    }
    
    func updateMousePosition() {
        let bs = self.convert(frame, from: self)
        let mloc = self.convert(self.window!.mouseLocationOutsideOfEventStream, to: self)
        self.x = (mloc.x - bs.minX - bounds.midX)
        self.y = (mloc.y - bs.minY - bounds.midY)
    }
    
    
    override func mouseMoved(with event: NSEvent) {
        self.updateMousePosition()

        if !mouseDownState {
            if let em = elementModel {
                let el = findElement(el: em, x: self.x, y: self.y)
                if( el != nil) {
                    activeElement = el
                }
                else {
                    activeElement = nil
                }
            }
            needsDisplay = true
        }
    }
    
    func getElements() -> [Element]  {
        var allElements:[Element] = []
        
        collectElements(el: elementModel!, elements: &allElements)
        return allElements
    }
    override func draw(_ dirtyRect: NSRect) {
        
        if( self.elementModel == nil) {
            return
        }
        
        let context = NSGraphicsContext.current?.cgContext
        
        context?.setFillColor(background)
        context?.fill(bounds)
        
        
        let allElements = getElements()
        
        for e in allElements {
            
            let yy: CGFloat = CGFloat(e.y) + bounds.midY
            let xx: CGFloat = CGFloat(e.x) + bounds.midX
            var active = false
            
            if let ae = activeElement {
                if ae.id == e.id {
                    active = true
                }
            }
            drawRoundedRect(rect: CGRect(x:xx, y:yy, width:175, height:45),
                        inContext: context,
                        radius: CGFloat(9),
                        borderColor: CGColor.black,
                        fillColor: CGColor.white,
                        text: e.name,
                        active: active)
        }
    }
    func drawRoundedRect(rect: CGRect, inContext context: CGContext?,
                         radius: CGFloat,
                         borderColor: CGColor,
                         fillColor: CGColor,
                         text: String,
                         active: Bool = false) {
        
        context?.saveGState()
        // 1
        let path = CGMutablePath()
        
        // 2
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
        
        context?.setShadow(offset: CGSize(width: 2, height:-2), blur: 4, color: CGColor(red:0,green:0,blue:0,alpha: 0.5))
        
        
        // 3
        context?.setLineWidth( 0 )
        context?.setStrokeColor(borderColor)
        context?.setFillColor(fillColor)

        
        // 4
        context?.addPath(path)
        context?.drawPath(using: .fillStroke)
        
        
        context?.setShadow(offset: CGSize(width:0, height:0), blur: CGFloat(0))
        
        
        if ( active ) {
            context?.setLineWidth( 0.75 )
            context?.setStrokeColor(borderColor)
            context?.setFillColor(fillColor)
        
            context?.addPath(path)
            context?.drawPath(using: .stroke)
        }
        
        let q: NSString = text as NSString
        
        
        let font = NSFont.systemFont(ofSize: 24)
        
        let textStyle = NSMutableParagraphStyle.default.mutableCopy() as! NSMutableParagraphStyle
        textStyle.alignment = NSTextAlignment.center
        let textColor = NSColor(calibratedRed: 0.147, green: 0.222, blue: 0.162, alpha: 1.0)
        
        let textFontAttributes: [NSAttributedStringKey:Any] = [
            NSAttributedStringKey.foregroundColor: textColor,
            NSAttributedStringKey.paragraphStyle: textStyle,
            NSAttributedStringKey.font: font
        ]
        
        
        q.draw(in: rect, withAttributes: textFontAttributes)
        
        
        context?.restoreGState()
        
    }

}
