//
//  ElementScene.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 01/07/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//

import Foundation

import Cocoa

/// A basic drawable element
public protocol Drawable {
    /// raw drag
    func drawBox( context: CGContext, at point: CGPoint )
    
    ///
    func draw( context: CGContext, at point: CGPoint)
    
    /// Layout children
    func layout( _ bounds: CGRect )
    
    /// Return bounds of element
    func getBounds()-> CGRect
    
    var visible: Bool {
        get
    }
    
}

open class DrawableElement: Drawable {
    public var children: [Drawable]? = nil
    
    public var x: CGFloat = 0 // A x position of this element
    public var y: CGFloat = 0 // A y position of this element
    
    public var visible: Bool = true
    
    public init( _ childs: [Drawable]) {
        if childs.count > 0 {
            children = []
            for c in childs {
                children!.append(c)
            }
        }
    }
    
    convenience init( _ childs: Drawable...) {
        self.init(childs)
    }
    
    public func append( _ child: Drawable ) {
        if self.children == nil {
            self.children = []
        }
        self.children?.append(child)
    }
    public func insert( _ child: Drawable, at index: Int ) {
        if self.children == nil {
            self.children = []
        }
        self.children?.insert(child, at: index)
    }
    
    open func drawBox(context: CGContext, at point: CGPoint) {
        if let ch = self.children {
            for c in ch {
                if c.visible {
                    c.drawBox(context: context, at: point)
                }
            }
        }
    }
    open func layout( _ bounds: CGRect ) {
        let selfBounds = getBounds()
        if let ch = self.children {
            for c in ch {
                if c.visible {
                    c.layout( selfBounds  )
                }
            }
        }
    }
    
    open func draw(context: CGContext, at point: CGPoint) {
        if let ch = self.children {
            for c in ch {
                if c.visible {
                    c.draw(context: context, at: point)
                }
            }
        }
    }
    
    open func getBounds() -> CGRect {
        var rect = CGRect(x: 0, y:0, width: 0, height: 0)
        
        if let ch = self.children {
            for c in ch {
                if c.visible {
                    let cbounds = c.getBounds()
                    rect = rect.union(cbounds)
                }
            }
        }
        
        return rect
    }
}


open class DrawableScene: DrawableElement {
    public var bounds: CGRect = CGRect(x:0, y:0, width: 0, height: 0)
    
    public var offset = CGPoint(x:0, y:0)
    
    open override func layout( _ bounds: CGRect ) {
        self.bounds = bounds
        super.layout(self.bounds)
    }
    
    open func draw(context: CGContext) {
        draw(context: context, at: offset)
    }
    open func drawBox(context: CGContext) {
        drawBox(context: context, at: offset)
    }
}

public class RoundBox: DrawableElement {
    public var bounds: CGRect
    public var fillColor: CGColor
    public var borderColor: CGColor
    public var radius: CGFloat = 8
    public var lineWidth: CGFloat = 0.3
    
    init( bounds: CGRect, fillColor: CGColor, borderColor: CGColor ) {
        self.bounds = bounds
        self.fillColor = fillColor
        self.borderColor = borderColor
        
        super.init([])
    }
    public override func drawBox(context: CGContext, at point: CGPoint) {
        // We only need to draw rect for shadow
        self.doDraw(context, at: point)
    }
    
    public override func draw(context: CGContext, at point: CGPoint) {
        self.doDraw(context, at: point)
        super.draw(context: context, at: CGPoint(x: self.bounds.minX + point.x, y: self.bounds.minY + point.y))
    }
    
    func doDraw(_ context:CGContext, at point: CGPoint) {
        context.saveGState()
        
        let path = CGMutablePath()
        
        var rect = bounds
        rect.origin.x += point.x
        rect.origin.y += point.y
        
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
        
        context.setLineWidth( self.lineWidth )
        context.setStrokeColor( self.borderColor )
        context.setFillColor( self.fillColor )
        
        context.addPath(path)
        context.drawPath(using: .fillStroke)
        
        context.restoreGState()
    }
    
    public override func getBounds() -> CGRect {
        return bounds
    }
}


public class TextBox: Drawable {
    var size: CGSize = CGSize(width: 0, height:0)
    var point: CGPoint = CGPoint(x:0, y:0)
    var textColor: NSColor
    var textFontAttributes: [NSAttributedStringKey:Any]
    let text:String
    var font: NSFont
    var textStyle: NSMutableParagraphStyle
    
    public var visible: Bool = true
    
    public init( text: String, textColor: CGColor, fontSize:CGFloat = 24) {
        self.font = NSFont.systemFont(ofSize: fontSize)
        self.text = text
        
        self.textColor = NSColor(cgColor: textColor)!
        
        self.textStyle = NSMutableParagraphStyle.default.mutableCopy() as! NSMutableParagraphStyle
        textStyle.alignment = NSTextAlignment.center
        
        self.textFontAttributes = [
            NSAttributedStringKey.foregroundColor: self.textColor,
            NSAttributedStringKey.paragraphStyle: self.textStyle,
            NSAttributedStringKey.font: self.font
        ]
        let attrString = NSAttributedString(string: text, attributes: textFontAttributes)
        
        let fs = CTFramesetterCreateWithAttributedString(attrString)
        let frameSize = CTFramesetterSuggestFrameSizeWithConstraints(fs, CFRangeMake(0, attrString.length), nil, CGSize(width: 150, height: 45), nil)
        
        self.size = CGSize(width: frameSize.width + 10, height: frameSize.height + 8 )
    }
    
    public func drawBox(context: CGContext, at point: CGPoint) {
        
    }
    
    public func draw(context: CGContext, at point: CGPoint) {
        let q: NSString = self.text as NSString
        
        q.draw(at: CGPoint(x: point.x + self.point.x + 5, y: point.y + self.point.y+4), withAttributes: textFontAttributes)
    }
    
    public func layout(_ bounds: CGRect) {
        // Put in centre of bounds
        self.point = CGPoint(x: (bounds.width-size.width)/2 , y: (bounds.height-size.height)/2)
    }
    
    public func getBounds() -> CGRect {
        return CGRect(origin: point, size: size )
    }
}

public class DrawableLine: Drawable {
    var source: CGPoint
    var target: CGPoint
    var color: CGColor
    var lineWidth: CGFloat = 1
    
    init( source: CGPoint, target: CGPoint, color: CGColor = CGColor.black) {
        self.source = source
        self.target = target
        self.color = color
    }
    
    public func drawBox(context: CGContext, at point: CGPoint) {
        self.draw(context: context, at: point)
    }
    
    public func draw(context: CGContext, at point: CGPoint) {
        //
        context.saveGState()
        
        context.setLineWidth( self.lineWidth )
        context.setStrokeColor(self.color)
        context.setFillColor(self.color)
        
        let aPath = CGMutablePath()
        
        aPath.move(to: CGPoint(x: source.x + point.x, y: source.y + point.y))
        aPath.addLine(to: CGPoint( x: target.x + point.x, y: target.y + point.y))
        
        //Keep using the method addLineToPoint until you get to the one where about to close the path
        aPath.closeSubpath()
        context.addPath(aPath)
        context.drawPath(using: .fillStroke)
        
        context.restoreGState()
    }
    
    public func layout(_ bounds: CGRect) {
        
    }
    
    public func getBounds() -> CGRect {
        
        let minX = min( source.x, target.x)
        let maxX = max( source.x, target.x)
        let minY = min( source.y, target.y)
        let maxY = max( source.y, target.y)
        
        return CGRect(x:minX, y:minY, width:(maxX-minX), height:(maxY-minY))
    }
    
    public var visible: Bool = true
    
    
}
