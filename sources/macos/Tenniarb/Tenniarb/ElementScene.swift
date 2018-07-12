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
    func layout( _ bounds: CGRect, _ dirty: CGRect )
    
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
    
    open func layout(_ bounds: CGRect, _ dirty: CGRect) {
        visible = true
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
        var lines:[DrawableLine] = []
        
        // Check activeDrawable first
        if let childs = children {
            for c in childs {
                if let drEl = c as? DrawableContainer {
                    let res = drEl.find(point)
                    if res != nil && res!.item != nil {
                        return res
                    }
                }
                else if let cc = c as? ItemDrawable {
                    if let ccl = c as? DrawableLine {
                       lines.append(ccl)
                    }
                    else {
                        // Just regular drawable check for bounds
                        if cc.getBounds().contains(point) && cc.item != nil {
                            return cc
                        }
                    }
                }
            }
        }
        for ccl in lines {
            if ccl.find(point) {
                return ccl
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
    open override func layout( _ bounds: CGRect, _ dirty: CGRect ) {
        let selfBounds = self.getBounds()

        if let ch = self.children {
            for c in ch {
                c.layout( selfBounds, dirty )
            }
        }
        let newBounds = self.getBounds()
        visible = dirty.intersects(newBounds)
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

class DrawableItemStyle {
    var color: CGColor?
    var borderColor: CGColor?
    var fontSize:CGFloat = 18.0
    var width:CGFloat?
    var height:CGFloat?
    
    var lineDash:String?
    
    
    /**
     One of values:
        * default, not specified - just item box
        * text - as a just text box
        * etc
     
     */
    var display: String?
    
    /*
        A child layout specification
     Values:
        * manual, not specified - just as placed
        * auto - managed layout
     */
    var layout: String?
    
    static func hexStringToUIColor (hexString:String, alpha: CGFloat = 1.0) -> CGColor {
        let hexString: String = hexString.trimmingCharacters(in: CharacterSet.whitespacesAndNewlines)
        let scanner = Scanner(string: hexString)
        if (hexString.hasPrefix("#")) {
            scanner.scanLocation = 1
        }
        var color: UInt32 = 0
        scanner.scanHexInt32(&color)
        let mask = 0x000000FF
        let r = Int(color >> 16) & mask
        let g = Int(color >> 8) & mask
        let b = Int(color) & mask
        let red   = CGFloat(r) / 255.0
        let green = CGFloat(g) / 255.0
        let blue  = CGFloat(b) / 255.0
        return CGColor(red:red, green:green, blue:blue, alpha:alpha)
    }
    
    static func parseColor(_ color: String, alpha: CGFloat = 1.0  ) -> CGColor {
        if color.starts(with: "#") {
            return hexStringToUIColor(hexString: color, alpha: alpha)
        }
        if let cl = ColorNames[color] {
            return hexStringToUIColor(hexString: cl, alpha: alpha)
        }
        
        return CGColor.black.copy(alpha: alpha)!
    }
    
    static func parseStyle( item: DiagramItem, default result: DrawableItemStyle = DrawableItemStyle() ) -> DrawableItemStyle {
        for child in item.properties {
            if child.kind == .Command, child.count > 0, let cmdName = child.getIdent(0) {
                switch cmdName {
                case "color":
                    if let color = child.getIdent(1) {
                        result.color = parseColor(color.lowercased(), alpha: 0.7)
                    }
                case "font-size":
                    if let value = child.getFloat(1) {
                        result.fontSize = CGFloat(value)
                        if result.fontSize > 37 {
                           result.fontSize = 36
                        }
                        else if result.fontSize < 4 {
                            result.fontSize = 4
                        }
                    }
                case "display":
                    if let value = child.getIdent(1) {
                        result.display = value
                    }
                case "layout":
                    if let value = child.getIdent(1) {
                        result.layout = value
                    }
                case "line-dash":
                    if let value = child.getIdent(1) {
                        result.lineDash = value
                    }
                case "width":
                    if let value = child.getFloat(1) {
                        result.width = CGFloat(value)
                    }
                case "height":
                    if let value = child.getFloat(1) {
                        result.height = CGFloat(value)
                    }
                case "borderColor":
                    if let color = child.getIdent(1) {
                        result.borderColor = parseColor(color.lowercased())
                    }
                default:
                    break;
                }
            }
        }
        return result
    }
    
    func fontSize( _ size: CGFloat) -> DrawableItemStyle {
        self.fontSize = size
        return self
    }
}


open class DrawableScene: DrawableContainer {
    public var offset = CGPoint(x:0, y:0)
    
    var drawables: [DiagramItem: Drawable] = [:]
    
    var itemToLink:[DiagramItem: [DiagramItem]] = [:]
    
    var activeDrawable: Drawable?
    
    var lineToDrawable: Drawable?
    
    var editingMode: Bool = false {
        didSet {
            self.updateActiveElement()
        }
    }
    
    var activeElement: DiagramItem? {
        didSet {
            self.updateActiveElement()
        }
    }
    
    init( _ element: Element) {
        super.init([])
        self.bounds = CGRect(x:0, y:0, width: 0, height: 0)
        
        self.append(buildElementScene(element))
    }
    
    public override func find( _ point: CGPoint ) -> ItemDrawable? {
        if let active = activeElement, let activeDr = drawables[active] {
            if activeDr.getBounds().contains(point) {
                if let ln = activeDr as? DrawableLine {
                    if ln.find(point) {
                        return activeDr as? ItemDrawable
                    }
                }
                else {
                    return activeDr as? ItemDrawable
                }
            }
        }
        return super.find(point)
    }
    
    func updateLineTo(_ de: DiagramItem, _ point: CGPoint ) -> DiagramItem? {
        var result: DiagramItem?
        if let de = self.drawables[de] {
            let bounds = de.getBounds()
            
            var mid = CGPoint( x: bounds.midX, y: bounds.midY)
            
            var targetPoint = point
            
            var tBounds: CGRect?
            
            if let targetDe = self.find(point) {
                if targetDe.item?.kind == .Item {
                    tBounds = targetDe.getBounds()
                    targetPoint = CGPoint( x: tBounds!.midX, y: tBounds!.midY)
                    self.activeElement = targetDe.item
                    result = targetDe.item
                }
            }
            
            if let cp1 = crossBox(mid, targetPoint, bounds) {
                mid = cp1
            }
            
            if let tb = tBounds {
                if let cp2 = crossBox(mid, targetPoint, tb) {
                    targetPoint = cp2
                }
            }
            
            self.lineToDrawable = DrawableLine( source: mid, target: targetPoint, style: DrawableItemStyle() )
        }
        return result
    }
    
    func removeLineTo() {
        self.lineToDrawable = nil
    }
    
    func updateActiveElement() {
        if let ae = activeElement {
            if let de = drawables[ae] as? ItemDrawable {
                if let line =  de as? DrawableLine {
                    activeDrawable = SelectorLine(source: line.source, target: line.target, extra: line.extraPoints)
                    
                }
                else {
                    let deBounds = de.getBounds()
                    activeDrawable = SelectorBox(
                        pos: CGPoint(x: deBounds.origin.x - 5, y: deBounds.origin.y - 5 ),
                        size: CGSize(width: deBounds.width + 10, height: deBounds.height + 10 ),
                        color: !self.editingMode ? SelectorBox.normalColor: SelectorBox.editingColor
                    )
                }
            }
        }
        else {
            activeDrawable = nil
        }
    }
    
    func updateLayout(_ item: DiagramItem, _ pos: CGPoint) -> CGRect {
        updateActiveElement()
        
        var result:CGRect = CGRect(origin: pos, size: CGSize(width:1, height:1))
        
        if let ad = activeDrawable {
            result = ad.getBounds()
        }
        
        if let box = drawables[item] as? RoundBox {
            result = result.union(box.getBounds())
            box.setPath(CGRect(origin:CGPoint(x: pos.x, y: pos.y), size: box.bounds.size))
        }
        if let box = drawables[item] as? EmptyBox {
            result = result.union(box.getBounds())
            box.setPath(CGRect(origin:CGPoint(x: pos.x, y: pos.y), size: box.bounds.size))
        }
        if let box = drawables[item] as? DrawableLine {
            result = result.union(box.getBounds())
            box.control = pos
            box.updateLayout(source: box.sourceRect, target: box.targetRect)
        }
        // Update links
        if let links = itemToLink[item] {
            for l in links {
                if let data = l as? LinkItem {
                    if let lnkDr = drawables[l] as? DrawableLine {
                        result = result.union(lnkDr.getBounds())
                        
                        if let src = data.source, let dst = data.target {
                            let sourceRect = drawables[src]?.getBounds()
                            let targetRect = drawables[dst]?.getBounds()
                            
                            if let sr = sourceRect, let tr = targetRect {
                                lnkDr.updateLayout(source: sr, target: tr)
                            }
                            result = result.union(lnkDr.getBounds())
                        }
                    }
                }
            }
        }
        return result
    }
    
    open override func layout( _ bounds: CGRect, _ dirty: CGRect ) {
        self.bounds = bounds
        super.layout(self.bounds, dirty)
    }
    
    open func draw(context: CGContext) {
        draw(context: context, at: offset)
        
        if let selBox = self.activeDrawable {
            selBox.draw(context: context, at: offset)
        }
        
        if let lineTo = self.lineToDrawable {
            lineTo.draw(context: context, at: offset)
        }
    }
    open func drawBox(context: CGContext) {
        drawBox(context: context, at: offset)
        
        if let selBox = self.activeDrawable {
            selBox.drawBox(context: context, at: offset)
        }
        
        if let lineTo = self.lineToDrawable {
            lineTo.draw(context: context, at: offset)
        }
    }
    
    public override func update() {
        
    }
    
    fileprivate func buildRoundRect(_ bounds: CGRect, _ bgColor: CGColor, _ borderColor: CGColor, _ e: DiagramItem, _ textBox: TextBox, _ elementDrawable: DrawableContainer, fill: Bool = true, stack:Int=0) {
        let rectBox = RoundBox( bounds: bounds,
                                fillColor: bgColor,
                                borderColor: borderColor, fill: fill)
        rectBox.stack = stack
        if self.activeElement == e {
            rectBox.lineWidth = 1
        }
        rectBox.append(textBox)
        
        rectBox.item = e
        
        drawables[e] = rectBox
        elementDrawable.append(rectBox)
    }
    fileprivate func buildEmptyRect(_ bounds: CGRect, _ e: DiagramItem, _ textBox: TextBox, _ elementDrawable: DrawableContainer) {
        let rectBox = EmptyBox( bounds: bounds )
        rectBox.append(textBox)
        rectBox.item = e
        
        drawables[e] = rectBox
        elementDrawable.append(rectBox)
    }
    
    func buildItemDrawable(_ e: DiagramItem, _ elementDrawable: DrawableContainer) {
        let name = e.name
        
        let style = DrawableItemStyle.parseStyle(item: e)
        
        let bgColor = style.color ?? CGColor(red: 1.0, green:1.0, blue:1.0, alpha: 0.7)
        let borderColor = style.borderColor ?? CGColor.black
        
        let textBox = TextBox(
            text: (name.count > 0 ? name :  " ").replacingOccurrences(of: "\\n", with: "\n"),
            textColor: CGColor(red: 0.147, green: 0.222, blue: 0.162, alpha: 1.0),
            fontSize: style.fontSize)
        
        let textBounds = textBox.getBounds()
        
        var width = max(20, textBounds.width)
        if let styleWidth = style.width {
            width = styleWidth //max(width, styleWidth)
        }
        
        var height = max(20, textBounds.height)
        if let styleHeight = style.height {
            height = styleHeight//max(height, styleHeight)
        }
        
        let bounds = CGRect(x: e.x, y:e.y, width: width, height: height)
        
        if let display = style.display {
            switch display {
            case "text":
                buildEmptyRect(bounds,  e, textBox, elementDrawable)
            case "no-fill":
                buildRoundRect(bounds, bgColor, borderColor, e, textBox, elementDrawable, fill: false)
            case "stack":
                buildRoundRect(bounds, bgColor, borderColor, e, textBox, elementDrawable, stack: 3)
            default:
                buildRoundRect(bounds, bgColor, borderColor, e, textBox, elementDrawable)
            }
        }
        else {
            buildRoundRect(bounds, bgColor, borderColor, e, textBox, elementDrawable)
        }
    }
    
    func buildElementScene( _ element: Element)-> Drawable {
        let elementDrawable = DrawableContainer()
        
        var links: [DiagramItem] = []
        
        buildItems(element.items, elementDrawable, &links)
        for e in links {
            if let data = e as? LinkItem {
                
                if let src = data.source, let dst = data.target {
                    self.addLink( src, e )
                    self.addLink( dst, e )
                
                    let sourceRect = drawables[src]?.getBounds()
                    let targetRect = drawables[dst]?.getBounds()
                    
                    if let sr = sourceRect, let tr = targetRect {
                        let defaultLinkStyle = DrawableItemStyle().fontSize(12)
                        let linkStyle = DrawableItemStyle.parseStyle(item: e, default: defaultLinkStyle)
                        
                        let linkDr = DrawableLine(
                            source: sr,
                            target: tr,
                            style: linkStyle,
                            control: CGPoint(x: e.x, y: e.y ))
                        
                        if data.name.count > 0 {
                            linkDr.addLabel(data.name)
                        }
                        
                        linkDr.item = e
                        drawables[e] = linkDr
                        elementDrawable.insert(
                            linkDr, at: 0)
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
            if e.kind == .Item {
                buildItemDrawable(e, elementDrawable)
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
    public var fill: Bool = true
    public static let DEFAULT_LINE_WIDTH: CGFloat = 0.3
    public var lineWidth: CGFloat = DEFAULT_LINE_WIDTH
    public var stack: Int = 0
    public var stackStep = CGPoint(x:5, y:5)
    
    var path: CGMutablePath?
    
    init( bounds: CGRect, fillColor: CGColor, borderColor: CGColor, fill: Bool ) {
        self.fillColor = fillColor
        self.borderColor = borderColor
        self.fill = fill
        
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
        
        if self.stack > 0 {
            for i in 1...self.stack {
                context.saveGState()
                
                context.translateBy(x: self.stackStep.x * CGFloat(self.stack-i),
                                    y: self.stackStep.y * CGFloat(self.stack-i))
                context.addPath(self.path!.copy()!)
                context.drawPath(using: .fill)

                context.restoreGState()
            }
        }
        
        context.addPath(self.path!)
        
        if self.fill {
            context.drawPath(using: .fillStroke)
        }
        else {
            context.drawPath(using: .stroke)
        }
        
        context.restoreGState()
    }
    
    public override func layout(_ bounds: CGRect, _ dirty: CGRect) {
        let selfBounds = self.bounds
        
        if let ch = self.children {
            for c in ch {
                c.layout( selfBounds, dirty )
            }
        }
        let newBounds = self.bounds
        visible = dirty.intersects(newBounds)
    }
    
    public override func getBounds() -> CGRect {
//        if self.stack > 0 {
//            return CGRect(origin: bounds.origin,
//                          size: CGSize(
//                            width: bounds.width + CGFloat(self.stack-1)*self.stackStep.x,
//                            height: bounds.height + CGFloat(self.stack-1)*self.stackStep.y
//            ))
//        }
        return bounds
    }
}

public class EmptyBox: DrawableContainer {
    init( bounds: CGRect) {
        super.init([])
        self.bounds = bounds
    }
    
    func setPath( _ rect: CGRect) {
        self.bounds = rect
    }
    public override func drawBox(context: CGContext, at point: CGPoint) {
        // We only need to draw rect for shadow
    }
    
    public override func draw(context: CGContext, at point: CGPoint) {
        context.saveGState()
        let clipBounds = CGRect( origin: CGPoint(x: bounds.origin.x + point.x, y: bounds.origin.y + point.y), size: bounds.size)
        context.clip(to: clipBounds )
        super.draw(context: context, at: CGPoint(x: self.bounds.minX + point.x, y: self.bounds.minY + point.y))
        context.restoreGState()
    }
    
    public override func getBounds() -> CGRect {
        return bounds
    }
}

func arrow(from start: CGPoint, to end: CGPoint, tailWidth: CGFloat, headWidth: CGFloat, headLength: CGFloat) -> CGMutablePath? {
    let length = hypot(end.x - start.x, end.y - start.y)
    if length < 5 {
        return nil
    }
    let tailLength = length - headLength
    
    func p(_ x: CGFloat, _ y: CGFloat) -> CGPoint { return CGPoint(x: x, y: y) }
    let points: [CGPoint] = [
        p(0, tailWidth / 2.0),
        p(tailLength, tailWidth / 2.0),
        p(tailLength, headWidth / 2.0),
        p(length, 0),
        p(tailLength, -headWidth / 2.0),
        p(tailLength, -tailWidth / 2.0),
        p(0, -tailWidth / 2.0)
    ]
    
    let cosine = (end.x - start.x) / length
    let sine = (end.y - start.y) / length
    let transform = CGAffineTransform(a: cosine, b: sine, c: -sine, d: cosine, tx: start.x, ty: start.y)
    
    let path = CGMutablePath()
    path.addLines(between: points, transform: transform )
    path.closeSubpath()
    return path
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
        let frameSize = CTFramesetterSuggestFrameSizeWithConstraints(fs, CFRangeMake(0, attrString.length), nil, CGSize(width: 1000, height: 1000), nil)
        
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
    
    public func layout(_ bounds: CGRect, _ dirty: CGRect) {
        // Put in centre of bounds
        self.point = CGPoint(x: (bounds.width-size.width)/2 , y: (bounds.height-size.height)/2)
    }
    
    public func getBounds() -> CGRect {
        return CGRect(origin: point, size: size )
    }
    public func update() {
    }
}

public class DrawableLine: ItemDrawable {
    var source: CGPoint = CGPoint.zero
    var target: CGPoint = CGPoint.zero
    
    var extraPoints: [CGPoint] = []
    
    var sourceRect: CGRect = CGRect.zero
    var targetRect: CGRect = CGRect.zero
    var lineWidth: CGFloat = 1
    var style: DrawableItemStyle
    var control: CGPoint
    
    var label: TextBox?
    
    init( source: CGRect, target: CGRect, style: DrawableItemStyle, control: CGPoint = CGPoint.zero) {
        self.sourceRect = source
        self.targetRect = target
        self.style = style
        self.control = control
        super.init()
        
        self.updateLayout(source: source, target: target)
    }
    
    init( source: CGPoint, target: CGPoint, style: DrawableItemStyle) {
        self.source = source
        self.target = target
        self.style = style
        self.control = CGPoint.zero
    }
    
    func addLabel(_ label: String) {
        self.label = TextBox(text: label.replacingOccurrences(of: "\\n", with: "\n"),
                             textColor: self.style.color ?? CGColor(red: 0, green: 0, blue: 0, alpha: 1),
                             fontSize: self.style.fontSize)
    }
    
    func find( _ point: CGPoint)-> Bool {
        
        var ln: [CGPoint] = []
        ln.append(self.source)
        ln.append(contentsOf: self.extraPoints)
        ln.append(self.target)
        
        for i in 0...(ln.count-2) {
            if crossPointLine(ln[i], ln[i+1], point) {
                return true
            }
            
        }
        
        return false
    }
    
    func correctMiddle( point: inout CGPoint, rect: CGRect, order: inout Bool ) {
        if point.x == rect.origin.x {
            // Left part
            point.y = rect.origin.y + rect.height / 2
            order = true
        }
        else if point.x == (rect.origin.x + rect.width) {
            // Right part
            point.y = rect.origin.y + rect.height / 2
            order = true
        }
        else if point.y == rect.origin.y {
            // Bottom part
            point.x = rect.origin.x + rect.width / 2
            order = false
        }
        else {
            // Top part
            point.x = rect.origin.x + rect.width / 2
            order = false
        }
    }
    
    public func updateLayout(source sr: CGRect, target tr: CGRect) {
        self.extraPoints.removeAll()

        if let layout = self.style.layout, layout.starts(with: "middle") {
            
            let x1 = sr.origin.x
            let x2 = tr.origin.x
            let w1 = sr.width
            let w2 = tr.width
            
            let y1 = sr.origin.y
            let y2 = tr.origin.y
            let h1 = sr.height
            let h2 = tr.height
            
            
            let cx = !(x1+w1 < x2 || x1 > x2 + w2)
            let cy = !(y1+h1 < y2 || y1 > y2 + h2)

            if cx && !cy {
                // Variant A
                
                if y1 + h1 < y2 {
                    // Below
                    self.source = CGPoint(x: x1 + w1, y: y1 + h1/2.0)
                    self.target = CGPoint(x: x2 + w2, y: y2 + h2/2.0)
                    
                    let offset = max(x1 + w1, x2 + w2) + 20;
                    
                    self.extraPoints.append(CGPoint(x: offset, y: self.source.y))
                    self.extraPoints.append(CGPoint(x: offset, y: self.target.y))
                    
                    return
                }
                else {
                    // Under
                    self.source = CGPoint(x: x1, y: y1 + h1/2.0)
                    self.target = CGPoint(x: x2, y: y2 + h2/2.0)
                    
                    let offset = min(x1, x2) - 20;
                    
                    self.extraPoints.append(CGPoint(x: offset, y: self.source.y))
                    self.extraPoints.append(CGPoint(x: offset, y: self.target.y))
                    return
                }
            }
            else if !cx && cy {
                // Variant B
                if x1 + w1 < x2 {
                    // Left
                    self.source = CGPoint(x: x1 + w1/2, y: y1)
                    self.target = CGPoint(x: x2 + w2/2, y: y2)
                    
                    let offset = min(y1, y2) - 20;
                    
                    self.extraPoints.append(CGPoint(x: self.source.x, y: offset))
                    self.extraPoints.append(CGPoint(x: self.target.x, y: offset))
                    return
                }
                else {
                    // Right
                    self.source = CGPoint(x: x1 + w1/2, y: y1 + h1)
                    self.target = CGPoint(x: x2 + w2/2, y: y2 + h2)
                    
                    let offset = max(y1 + h1, y2 + h2) + 20;
                    
                    self.extraPoints.append(CGPoint(x: self.source.x, y: offset))
                    self.extraPoints.append(CGPoint(x: self.target.x, y: offset))
                    return
                }
            }
            else {
                // variant C
                if x1 + w1 < x2 {
                    // Left
                    
                    
                    if y1 + h1 < y2 {
                        // Up
                        self.source = CGPoint(x: x1 + w1, y: y1 + h1/2)
                        self.target = CGPoint(x: x2 + w2/2, y: y2)
                        self.extraPoints.append(CGPoint(x: self.target.x, y: self.source.y))
                    }
                    else {
                        // Down
                        self.source = CGPoint(x: x1 + w1/2, y: y1)
                        self.target = CGPoint(x: x2, y: y2 + h2/2)
                        self.extraPoints.append(CGPoint(x: self.source.x, y: self.target.y))
                    }
                    return
                }
                else {
                    // Right and Up
                    if y1 + h1 < y2 {
                        self.source = CGPoint(x: x1 + w1/2, y: y1 + h1)
                        self.target = CGPoint(x: x2 + w2, y: y2 + h2/2)
                        self.extraPoints.append(CGPoint(x: self.source.x, y: self.source.y))
                    }
                    else {
                        self.source = CGPoint(x: x1, y: y1 + h2/2)
                        self.target = CGPoint(x: x2 + w2/2, y: y2 + h2)
                        self.extraPoints.append(CGPoint(x: self.target.x, y: self.source.y))
                    }
                    return
                }
                
            }
        }
        else {
            let p1 = CGPoint( x: sr.midX, y:sr.midY )
            let p2 = CGPoint( x: tr.midX, y:tr.midY )
            
            let hasControl = self.control != CGPoint.zero
            
            let fromToLen = sqrt(pow(p1.x-p2.x,2)+pow(p1.y-p2.y,2))
            let fTo = CGPoint(x: (p2.x-p1.x)/fromToLen, y: (p2.y-p1.y)/fromToLen)
            let ctrlPoint = CGPoint(x: p1.x + control.x + fTo.x*fromToLen/2, y: p1.y + fTo.y*fromToLen/2 + control.y)
            
            if hasControl {
                self.extraPoints.append(ctrlPoint)
            }
            if let cp1 = crossBox(p1, hasControl ? ctrlPoint : p2, sr) {
                self.source = cp1
            }
            else {
                self.source = p1
            }
            if let cp2 = crossBox(hasControl ? ctrlPoint : p1, p2, tr) {
                self.target = cp2
            }
            else {
                self.target = p2
            }
        }
    }
    
    public override func drawBox(context: CGContext, at point: CGPoint) {
        self.draw(context: context, at: point)
    }
    
    public override func draw(context: CGContext, at point: CGPoint) {
        //
        context.saveGState()
        
        context.setLineWidth( self.lineWidth )
        context.setStrokeColor(self.style.color ?? CGColor.black)
        context.setFillColor(self.style.color ?? CGColor.black)
        
        let drawArrow = self.style.display == "arrow" ||
                        self.style.display == "arrow-source" ||
                        self.style.display == "arrows"
        
        if let dash = self.style.lineDash {
            switch dash {
            case "dotted":
                context.setLineDash(phase: 1, lengths: [1, 4])
            case "dashed":
                context.setLineDash(phase: 5, lengths: [5])
            case "solid": break;
            default:break;
            }
        }
        
        var fillType: CGPathDrawingMode = .stroke
        
        let fromPt = CGPoint(x: source.x + point.x, y: source.y + point.y)
        let toPt = CGPoint( x: target.x + point.x, y: target.y + point.y)
        
        let aPath = CGMutablePath()
        aPath.move(to: fromPt)
        
        for ep in self.extraPoints {
            aPath.addLine(to: CGPoint(x: ep.x + point.x, y: ep.y + point.y))
            
            if let lbl = self.label {
                lbl.point = CGPoint(x: ep.x, y: ep.y - lbl.getBounds().height)
            }
        }
        
        if self.extraPoints.isEmpty {
            if let lbl = self.label {
                let lblBounds = lbl.getBounds()
                var nx = (source.x + target.x)/2
                
                if abs(source.y-target.y) < 10 {
                    nx -= lblBounds.width/2
                }
                var ny = (source.y + target.y)/2
                if abs(source.x-target.x) < 10 {
                    ny -= lblBounds.height/2
                }
                lbl.point = CGPoint(x: nx, y: ny)
            }
        }
        
        aPath.addLine(to: toPt)
//        aPath.closeSubpath()
        
        context.addPath(aPath)
        
        context.drawPath(using: fillType)
        
        if drawArrow {
            fillType = .fillStroke
            
            let spt = extraPoints.count > 0 ? CGPoint(x: extraPoints[0].x + point.x, y: extraPoints[0].y + point.y) : toPt
            let ept = extraPoints.count > 0 ? CGPoint(x: extraPoints[extraPoints.count-1].x + point.x, y: extraPoints[extraPoints.count-1].y + point.y) : fromPt
            
            if self.style.display == "arrow" || self.style.display == "arrows" {
                if let arr = arrow(from: ept, to: toPt,
                                   tailWidth: 0, headWidth: 10, headLength: 10) {
                    context.addPath(arr)
                }
            }
            if self.style.display == "arrow-source" || self.style.display == "arrows" {
                if let arr = arrow(from: spt, to: fromPt,
                                   tailWidth: 0, headWidth: 10, headLength: 10) {
                    context.addPath(arr)
                }
            }
            context.drawPath(using: fillType)
        }
        
        if let lbl = self.label {
            lbl.draw(context: context, at: point)
        }
        
        
        context.restoreGState()
    }
    
    public override func layout(_ bounds: CGRect, _ dirty: CGRect) {
        
    }
    
    public override func isVisible() -> Bool {
        return true
    }
    public override func getBounds() -> CGRect {
        
        var minX = min(source.x, target.x)
        var maxX = max(source.x, target.x)
        var minY = min(source.y, target.y)
        var maxY = max(source.y, target.y)
        
        for ep in self.extraPoints {
            minX = min(ep.x, minX)
            maxX = max(ep.x, maxX)
            minY = min(ep.y, minY)
            maxY = max(ep.y, maxY)
        }
        
        return CGRect(x:minX, y:minY, width:(maxX-minX), height:max(maxY-minY, 5.0))
    }
    public override func update() {
    }
}

public class SelectorBox: Drawable {
    var pos: CGPoint
    var size: CGSize
    
    var color: CGColor
    
    var lineWidth: CGFloat = 1
    
    static let normalColor = CGColor(red: 0, green: 0, blue: 1, alpha: 1)
    static let editingColor = CGColor(red: 0, green: 1, blue: 0, alpha: 1)
    
    init( pos: CGPoint, size: CGSize, color: CGColor = normalColor) {
        self.pos = pos
        self.size = size
        self.color = color
    }
    
    public func drawBox(context: CGContext, at point: CGPoint) {
        self.draw(context: context, at: point)
    }
    
    public func draw(context: CGContext, at point: CGPoint) {
        //
        context.saveGState()
        
        context.setStrokeColor( self.color )
        context.setFillColor( self.color )
        context.setLineWidth( 1 )
        
        context.setShadow(offset: CGSize(width: 0.0, height: 0.0), blur: 5.0, color: self.color)
        context.setLineDash(phase: 5, lengths: [5])
        
        let rect = CGRect(origin: CGPoint(x: pos.x + point.x, y: pos.y + point.y), size: self.size)
//        context.addRect(rect)
        
        let path = CGMutablePath()
        
        path.move( to: CGPoint(x:  rect.midX, y:rect.minY ))
        
        let radius:CGFloat = 9.0
        path.addArc( tangent1End: CGPoint(x: rect.maxX, y: rect.minY ),
                           tangent2End: CGPoint(x: rect.maxX, y: rect.maxY), radius: radius)
        path.addArc( tangent1End: CGPoint(x: rect.maxX, y: rect.maxY ),
                           tangent2End: CGPoint(x: rect.minX, y: rect.maxY), radius: radius)
        path.addArc( tangent1End: CGPoint(x: rect.minX, y: rect.maxY ),
                           tangent2End: CGPoint(x: rect.minX, y: rect.minY), radius: radius)
        path.addArc( tangent1End: CGPoint(x: rect.minX, y: rect.minY ),
                           tangent2End: CGPoint(x: rect.maxX, y: rect.minY), radius: radius)
        path.closeSubpath()
        
        context.addPath(path)
        
        context.drawPath(using: .stroke)
        
        context.restoreGState()
    }
    
    public func layout(_ bounds: CGRect, _ dirty: CGRect) {
        
    }
    
    public func isVisible() -> Bool {
        return true
    }
    public func getBounds() -> CGRect {
        return CGRect(origin: self.pos, size: self.size)
    }
    public func update() {
    }
}

public class SelectorLine: ItemDrawable {
    var source: CGPoint
    var target: CGPoint
    var color: CGColor
    var extra: [CGPoint]
    var lineWidth: CGFloat = 1.5
    
    static let normalColor = CGColor(red: 0, green: 0, blue: 1, alpha: 1)
    
    init( source: CGPoint, target: CGPoint, extra: [CGPoint], color: CGColor = normalColor) {
        self.source = source
        self.target = target
        self.extra = extra
        self.color = color
    }
    
    public override func drawBox(context: CGContext, at point: CGPoint) {
        self.draw(context: context, at: point)
    }
    
    public override func draw(context: CGContext, at point: CGPoint) {
        //
        context.saveGState()
        
        context.setLineWidth( self.lineWidth )
        context.setStrokeColor(self.color)
        context.setFillColor(self.color)
        
        context.setShadow(offset: CGSize(width: 0.0, height: 0.0), blur: 5.0, color: self.color)
        context.setLineDash(phase: 5, lengths: [5])

        let aPath = CGMutablePath()
        
        aPath.move(to: CGPoint(x: source.x + point.x, y: source.y + point.y))
        for ep in self.extra {
            aPath.addLine(to: CGPoint( x: ep.x + point.x, y: ep.y + point.y))
        }
        aPath.addLine(to: CGPoint( x: target.x + point.x, y: target.y + point.y))
        
        //Keep using the method addLineToPoint until you get to the one where about to close the path
//        aPath.closeSubpath()
        context.addPath(aPath)
        context.drawPath(using: .stroke)
        
        context.restoreGState()
    }
    
    public override func layout(_ bounds: CGRect, _ dirty: CGRect) {
        
    }
    
    public override func isVisible() -> Bool {
        return true
    }
    public override func getBounds() -> CGRect {
        
        var minX = min(source.x, target.x)
        var maxX = max(source.x, target.x)
        var minY = min(source.y, target.y)
        var maxY = max(source.y, target.y)
        
        for ep in self.extra {
            minX = min(ep.x, minX)
            maxX = max(ep.x, maxX)
            minY = min(ep.y, minY)
            maxY = max(ep.y, maxY)
        }
        
        return CGRect(x:minX, y:minY, width:(maxX-minX), height:max(maxY-minY, 5.0))
    }
    public override func update() {
    }
}


