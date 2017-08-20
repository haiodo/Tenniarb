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
    
    func isVisible() -> Bool

    /// raw drag
    func drawBox( context: CGContext, at point: CGPoint )
    
    ///
    func draw( context: CGContext, at point: CGPoint)
    
    /// Layout children
    func layout( _ bounds: CGRect )
    
    /// Return bounds of element
    func getBounds() -> CGRect
    
    
    /// Update from state.
    func update()
}

open class ItemDrawable: Drawable {
    public var item: DiagramItem? = nil
    var visible: Bool = true
    
    var bounds: CGRect = CGRect(x: 0, y: 0, width: 0, height: 0)

    open func drawBox(context: CGContext, at point: CGPoint) {
    }
    
    open func draw(context: CGContext, at point: CGPoint) {
    }
    
    open func layout(_ bounds: CGRect) {
    }
    public func getBounds() -> CGRect {
        return bounds
    }
    public func isVisible() -> Bool {
        return visible
    }
    public func update() {
        
    }
}

open class DrawableContainer: ItemDrawable {
    public var children: [Drawable]? = nil
    
    public var x: CGFloat = 0 // A x position of this element
    public var y: CGFloat = 0 // A y position of this element
    
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
    
    public func find( _ point: CGPoint ) -> ItemDrawable? {
        if let childs = children {
            for c in childs {
                if let drEl = c as? DrawableContainer {
                    let res = drEl.find(point)
                    if res != nil && res?.item != nil {
                        return res
                    }
                }
                else if let cc = c as? ItemDrawable {
                    // Just regular drawable check for bounds
                    if cc.getBounds().contains(point) && cc.item != nil {
                        return cc
                    }
                }
            }
        }
        if self.item != nil {
            // Check self coords
            if self.getBounds().contains(point) {
                return self
            }
        }
        return nil
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
    
    open override func drawBox(context: CGContext, at point: CGPoint) {
        if let ch = self.children {
            for c in ch {
                if c.isVisible() {
                    c.drawBox(context: context, at: point)
                }
            }
        }
    }
    open override func layout( _ bounds: CGRect ) {
        let selfBounds = self.getBounds()
        if let ch = self.children {
            for c in ch {
                if c.isVisible() {
                    c.layout( selfBounds  )
                }
            }
        }
    }
    
    open override func draw(context: CGContext, at point: CGPoint) {
        if let ch = self.children {
            for c in ch {
                if c.isVisible() {
                    c.draw(context: context, at: point)
                }
            }
        }
    }
    
    open override func getBounds()-> CGRect {
        var rect = CGRect(x: 0, y:0, width: 0, height: 0)
        
        if let ch = self.children {
            for c in ch {
                if c.isVisible() {
                    let cbounds = c.getBounds()
                    rect = rect.union(cbounds)
                }
            }
        }
        
        return rect
    }
}


open class DrawableScene: DrawableContainer {
    public var offset = CGPoint(x:0, y:0)
    
    var drawables: [DiagramItem: Drawable] = [:]
    
    var itemToLink:[DiagramItem: [DiagramItem]] = [:]
    
    var activeElementValue: DiagramItem?
    var activeElement: DiagramItem? {
        set {
            if let ae =  activeDrawable {
                ae.lineWidth = RoundBox.DEFAULT_LINE_WIDTH
            }
            self.activeElementValue = newValue
            if let ae = newValue {
                if let de = drawables[ae] as? RoundBox {
                    de.lineWidth = 0.7
                    activeDrawable = de
                }
            }
        }
        get {
            return activeElementValue
        }
    }
    var activeDrawable: RoundBox?
    
    init( _ element: Element) {
        super.init([])
        self.bounds = CGRect(x:0, y:0, width: 0, height: 0)
        
        self.append(buildElementScene(element))
    }
    
    func updateLayout(_ item: DiagramItem) {
        if let box = drawables[item] as? RoundBox {
            box.setPath(CGRect(origin:CGPoint(x: item.data.x, y: item.data.y), size: box.bounds.size))
            
            // Update links
            if let links = itemToLink[item] {
                for l in links {
                    if let data = l.data as? LinkElementData {
                        if let lnkDr = drawables[l] as? DrawableLine {
                            let sourceRect = drawables[data.source]?.getBounds()
                            let targetRect = drawables[data.target]?.getBounds()
                            
                            if let sr = sourceRect, let tr = targetRect {
                                
                                let p1 = CGPoint( x: sr.midX, y:sr.midY )
                                let p2 = CGPoint( x: tr.midX, y:tr.midY )
                                
                                
                                if let cp1 = crossBox(p1, p2, sr) {
                                    lnkDr.source = cp1
                                }
                                else {
                                    lnkDr.source = p1
                                }
                                if let cp2 = crossBox(p1, p2, tr) {
                                    lnkDr.target = cp2
                                }
                                else {
                                    lnkDr.target = p2
                                }
                            }
                        }
                    }
                }
            }
        }
    }
    
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
    
    public override func update() {
        
    }
    
    func buildItemDrawable(_ e: DiagramItem, _ elementDrawable: DrawableContainer) {
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
        
        if self.activeElement == e {
            rectBox.lineWidth = 1
        }
        rectBox.append(textBox)
        
        rectBox.item = e
        
        drawables[e] = rectBox
        
        elementDrawable.append(rectBox)
    }
    func buildElementScene( _ element: Element)-> Drawable {
        let elementDrawable = DrawableContainer()
        
        var links: [DiagramItem] = []
        
        buildItems(element.items, elementDrawable, &links)
        for e in links {
            if let data = e.data as? LinkElementData {
                
                self.addLink( data.source, e )
                self.addLink( data.target, e )
                
                let sourceRect = drawables[data.source]?.getBounds()
                let targetRect = drawables[data.target]?.getBounds()
                
                if let sr = sourceRect, let tr = targetRect {
                    
                    let p1 = CGPoint( x: sr.midX, y:sr.midY )
                    let p2 = CGPoint( x: tr.midX, y:tr.midY )
                    
                    
                    let linkDr = DrawableLine(
                        source: p1,
                        target: p2)
                    drawables[e] = linkDr
                    elementDrawable.insert(
                        linkDr, at: 0)
                    
                    if let cp1 = crossBox(p1, p2, sr) {
                        linkDr.source = cp1
                    }
                    else {
                        linkDr.source = p1
                    }
                    if let cp2 = crossBox(p1, p2, tr) {
                        linkDr.target = cp2
                    }
                    else {
                        linkDr.target = p2
                    }
                }
            }
        }
        return elementDrawable
    }
    func addLink( _ itm: DiagramItem, _ link: DiagramItem) {
        var links = itemToLink[itm]
        if links == nil {
            links = []
        }
        links?.append(link)
        itemToLink[itm] = links
    }
    func buildItems(_ items: [DiagramItem], _ elementDrawable: DrawableContainer, _ links: inout [DiagramItem]) {
        for e in items {
            if e.kind == .Element {
                buildItemDrawable(e, elementDrawable)
                
                if let itms = e.items {
                    buildItems(itms, elementDrawable, &links)
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
    
}

public class RoundBox: DrawableContainer {
    public var fillColor: CGColor
    public var borderColor: CGColor
    public var radius: CGFloat = 8
    public static let DEFAULT_LINE_WIDTH: CGFloat = 0.3
    public var lineWidth: CGFloat = DEFAULT_LINE_WIDTH
    var path: CGMutablePath?
    
    init( bounds: CGRect, fillColor: CGColor, borderColor: CGColor ) {
        self.fillColor = fillColor
        self.borderColor = borderColor
        
        super.init([])
        self.setPath(bounds)
    }
    
    func setPath( _ rect: CGRect) {
        self.bounds = rect
        self.path = CGMutablePath()
        self.path?.move( to: CGPoint(x:  rect.midX, y:rect.minY ))
        self.path?.addArc( tangent1End: CGPoint(x: rect.maxX, y: rect.minY ),
                     tangent2End: CGPoint(x: rect.maxX, y: rect.maxY), radius: radius)
        self.path?.addArc( tangent1End: CGPoint(x: rect.maxX, y: rect.maxY ),
                     tangent2End: CGPoint(x: rect.minX, y: rect.maxY), radius: radius)
        self.path?.addArc( tangent1End: CGPoint(x: rect.minX, y: rect.maxY ),
                     tangent2End: CGPoint(x: rect.minX, y: rect.minY), radius: radius)
        self.path?.addArc( tangent1End: CGPoint(x: rect.minX, y: rect.minY ),
                     tangent2End: CGPoint(x: rect.maxX, y: rect.minY), radius: radius)
        self.path?.closeSubpath()
    }
    public override func drawBox(context: CGContext, at point: CGPoint) {
        // We only need to draw rect for shadow
        self.doDraw(context, at: point)
    }
    
    public override func draw(context: CGContext, at point: CGPoint) {
        context.saveGState()
        self.doDraw(context, at: point)
        let clipBounds = CGRect( origin: CGPoint(x: bounds.origin.x + point.x, y: bounds.origin.y + point.y), size: bounds.size)
        context.clip(to: clipBounds )
        super.draw(context: context, at: CGPoint(x: self.bounds.minX + point.x, y: self.bounds.minY + point.y))
        context.restoreGState()
    }
    
    func doDraw(_ context:CGContext, at point: CGPoint) {
        context.saveGState()
        
        context.setLineWidth( self.lineWidth )
        context.setStrokeColor( self.borderColor )
        context.setFillColor( self.fillColor )
        
        context.translateBy(x: point.x, y: point.y)
        
        context.addPath(self.path!)
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
        let frameSize = CTFramesetterSuggestFrameSizeWithConstraints(fs, CFRangeMake(0, attrString.length), nil, CGSize(width: 300, height: 45), nil)
        
        self.size = CGSize(width: frameSize.width + 10, height: frameSize.height + 8 )
    }
    
    public func isVisible() -> Bool {
        return true
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
    public func update() {
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
    
    public func isVisible() -> Bool {
        return true
    }
    public func getBounds() -> CGRect {
        
        let minX = min( source.x, target.x)
        let maxX = max( source.x, target.x)
        let minY = min( source.y, target.y)
        let maxY = max( source.y, target.y)
        
        return CGRect(x:minX, y:minY, width:(maxX-minX), height:(maxY-minY))
    }
    public func update() {
    }
}
