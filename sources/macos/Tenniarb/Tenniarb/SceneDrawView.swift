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
    var elementModel: ElementModel?
    
    var activeElement: Element?
    
    
    public func setElementModel(_ elementModel: ElementModel ) {
        self.elementModel = elementModel
        self.setNeedsDisplay(bounds)
    }
    
    
    func collectElements(el: Element, elements: inout [Element]) {
        elements.append(el)
        elements.append(contentsOf: el.elements)
        for e in el.elements {
            collectElements(el: e, elements: &elements)
        }
    }
    
    override func mouseDown(with event: NSEvent) {
        
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
        
        let context = NSGraphicsContext.current()?.cgContext
        
        context?.setFillColor(background)
        context?.fill(bounds)
        
        
        let allElements = getElements()
        
        var posx = 50
        let posy = 50
        for e in allElements {
            
            let yy: CGFloat = bounds.maxY - CGFloat(150+posy)
            let xx: CGFloat = CGFloat(posx)
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
            posx += 200
//            posy += 50
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
        
        let textStyle = NSMutableParagraphStyle.default().mutableCopy() as! NSMutableParagraphStyle
        textStyle.alignment = NSTextAlignment.center
        let textColor = NSColor(calibratedRed: 0.147, green: 0.222, blue: 0.162, alpha: 1.0)
        
        let textFontAttributes: [String:Any] = [
            NSForegroundColorAttributeName: textColor,
            NSParagraphStyleAttributeName: textStyle,
            NSFontAttributeName: font
        ]
        
        
        q.draw(in: rect, withAttributes: textFontAttributes)
        
        
        context?.restoreGState()
        
    }

}
