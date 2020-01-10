//
//  ElementScene.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 01/07/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//

import Foundation

import Cocoa
import JavaScriptCore

// Some debug variabes

let OPTION_perform_clip = true
let OPTION_perform_text_stroke = false


/// A basic drawable element
public protocol Drawable {
    
    func isVisible() -> Bool
    
    /// raw drag
    func drawBox( context: CGContext, at point: CGPoint )
    
    ///
    func draw( context: CGContext, at point: CGPoint)
    
    /// Layout children
    func layout( _ bounds: CGRect, _ dirty: CGRect )
    
    func getSelectorBounds() -> CGRect
    
    /// Return bounds of element
    func getBounds() -> CGRect
    
    
    /// Update from state.
    func update()
    
    func traverse(_ op: (_ itm: Drawable )->Bool)
}

open class ItemDrawable: Drawable {
    public var item: DiagramItem? = nil
    var visible: Bool = true
    
    open var bounds: CGRect = CGRect(x: 0, y: 0, width: 0, height: 0)
    
    open func drawBox(context: CGContext, at point: CGPoint) {
    }
    
    open func draw(context: CGContext, at point: CGPoint) {
    }
    
    open func layout(_ bounds: CGRect, _ dirty: CGRect) {
        visible = true
    }
    
    public func getSelectorBounds() -> CGRect {
        return self.getBounds()
    }
    
    public func getBounds() -> CGRect {
        return bounds
    }
    public func isVisible() -> Bool {
        return visible
    }
    public func update() {
    }
    public func traverse(_ op: (_ itm: Drawable )->Bool) {
        _ = op(self)
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
    
    public override func traverse(_ op: (_ itm: Drawable )->Bool) {
        if op(self), let cc = self.children {
            for c in cc {
                if !op(c) {
                    return
                }
            }
        }
    }
    
    public func find( _ point: CGPoint ) -> [ItemDrawable] {
        var lines:[DrawableLine] = []
        var result: [ItemDrawable] = []
        // Check activeDrawable first
        if let childs = children {
            for c in childs {
                if let drEl = c as? DrawableContainer {
                    result.append(contentsOf: drEl.find(point))
                }
                else if let cc = c as? ItemDrawable {
                    if let ccl = c as? DrawableLine {
                        lines.append(ccl)
                    }
                    else {
                        // Just regular drawable check for bounds
                        if cc.getSelectorBounds().contains(point) && cc.item != nil {
                            result.append(cc)
                        }
                    }
                }
            }
        }
        if result.count > 0 {
            return result
        }
        for ccl in lines {
            if ccl.find(point) {
                result.append(ccl)
            }
        }
        if self.item != nil {
            // Check self coords
            if self.getSelectorBounds().contains(point) {
                result.append(self)
            }
        }
        return result
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
        let selfBounds = self.getSelectorBounds()
        
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
        var first = true
        if let ch = self.children {
            for c in ch {
                let cbounds = c.getBounds()
                if first {
                    rect = cbounds
                    first = false
                }
                else {
                    rect = rect.union(cbounds)
                }
            }
        }
        
        return rect
    }
}

func getString(_ child: TennNode?, _ evaluations: [TennToken: JSValue ]) -> String? {
    
    guard let ch = child else {
        return nil
    }
    // Check if we have override for value
    if let t = ch.token, let ev = evaluations[t] {
        return ev.toString()
    }
    return ch.getIdentText()
}
func updateLineStyle(_ context: CGContext, _ style: DrawableStyle ) {
    if let dash = style.lineStyle {
        switch dash {
        case "dotted":
            context.setLineDash(phase: 1, lengths: [1, 4])
        case "dashed":
            context.setLineDash(phase: 5, lengths: [5])
        case "solid": break;
        default:break;
        }
    }
    context.setLineWidth(style.lineWidth)
}

let styleBlack = CGColor.black // CGColor(red: 0.147, green: 0.222, blue: 0.162, alpha: 1.0)
let styleWhite = CGColor.white //CGColor(red: 0.847, green: 0.822, blue: 0.862, alpha: 1.0)

func getTextColorBasedOn(_ color: CGColor ) -> CGColor {
    if let components = color.components, color.numberOfComponents >= 3 {
        let sum = round( ( (components[0] * 255 * 299) + (components[1] * 255 * 587) + (components[2] * 255 * 114)) / 1000);
        return (sum > 128) ? styleBlack : styleWhite;
    }
    return styleBlack
}

class DrawableStyle {
    var color: CGColor = CGColor(red: 1.0, green:1.0, blue:1.0, alpha: 1)
    var textColorValue: CGColor?
    
    var textColor: CGColor {
        get {
            if textColorValue != nil {
                return textColorValue!
            }
            return getTextColorBasedOn(self.color)
        }
    }
    
    var borderColor: CGColor = CGColor.black
    var fontSize:CGFloat = 18.0
    var width:CGFloat?
    var height:CGFloat?
    var darkMode = false
    
    var lineStyle:String?
    
    var shadow: CGSize? = nil
    var shadowBlur: CGFloat = CGFloat(4)
    var shadowColor: CGColor? = nil
    
    var lineWidth = CGFloat(0)
    
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
    
    /*
     A layer define how element are displayed or selected.
     
     Supported values:
     - default - how it works right now.
     - background - item are drawn as first item before any other, item are not selected by default.
     - hover - item are drawn as last element
     */
    var layer: String?
    
    init( _ darkMode: Bool ) {
        self.darkMode = darkMode
        reset()
    }
    init( item: DiagramItem, _ evaluations: [TennToken: JSValue ] ) {
        reset()
        self.parseStyle(item.properties, evaluations)
    }
    
    func newCopy() -> DrawableStyle {
        return DrawableStyle(darkMode)
    }
    func copy() -> DrawableStyle {
        let result: DrawableStyle = newCopy()
        result.color = self.color
        result.textColorValue = self.textColorValue
        result.borderColor = self.borderColor
        result.fontSize = self.fontSize
        result.width = self.width
        result.height = self.height
        result.display = self.display
        result.layout = self.layout
        result.lineStyle = self.lineStyle
        
        result.shadow = self.shadow
        result.shadowColor = self.shadowColor
        result.shadowBlur = self.shadowBlur
        result.lineWidth = self.lineWidth
        result.layer = self.layer
        
        return result
    }
    
    
    func reset() {
        // Reset to default values
        
        if self.darkMode {
            self.color = CGColor(red: 0.2, green:0.2, blue:0.2, alpha: 1)
            self.textColorValue = nil
            self.borderColor = CGColor.white
        }
        else {
            self.color = CGColor(red: 1.0, green:1.0, blue:1.0, alpha: 1)
            self.textColorValue = nil
            self.borderColor = CGColor.black
        }
        
        self.fontSize = 18
        self.width = nil
        self.height = nil
        self.display = nil
        self.layout = nil
        self.lineStyle = nil
        
        self.shadow = nil
        self.shadowColor = nil
        self.shadowBlur = 5
        self.lineWidth = CGFloat(0.3)
    }
    
    
    func getFloat(_ child: TennNode?, _ evaluations: [TennToken: JSValue ]) -> CGFloat? {
        
        guard let ch =  child else {
            return nil
        }
        // Check if we have override for value
        if let t = ch.token, let ev = evaluations[t] {
            return CGFloat(ev.toDouble())
        }
        if let val = ch.getIdentText(), let floatVal = Float(val) {
            return CGFloat(floatVal)
        }
        return nil
    }
    
    func getComponentValue( _ value: Any ) -> CGFloat {
        if let rr = value as? Int {
            return CGFloat(rr)
        }
        if let rr = value as? Double {
            return CGFloat(rr)
        }
        if let rr = value as? Float {
            return CGFloat(rr)
        }
        return CGFloat(255.0)
    }
    func getColor(_ child: TennNode?, _ evaluations: [TennToken: JSValue ], alpha: CGFloat = 1.0) -> CGColor? {
        
        guard let ch = child else {
            return nil
        }
        // Check if we have override for value
        if let t = ch.token, let ev = evaluations[t] {
            if ev.isArray, let arr = ev.toArray() {
                var r: CGFloat = CGFloat(0)
                var g: CGFloat = CGFloat(0)
                var b: CGFloat = CGFloat(0)
                var al: CGFloat = alpha
                if arr.count >= 3 {
                    r = getComponentValue(arr[0])
                    g = getComponentValue(arr[1])
                    b = getComponentValue(arr[2])
                }
                if arr.count == 4 {
                    al = getComponentValue(arr[3]) / 255.0
                }
                return CGColor(red:  r / 255.0, green: g / 255.0, blue: b / 255.0, alpha: al)
            }
            if let value = ev.toString() {
                return parseColor(value.lowercased(), alpha: alpha)
            }
        }
        if let text = ch.getIdentText() {
            return parseColor(text.lowercased(), alpha: alpha)
        }
        return nil
    }
    
    func parseStyleLine(_ cmdName: String, _ child: TennNode, _ evaluations: [TennToken: JSValue ]) {
        switch cmdName {
        case PersistenceStyleKind.Color.name:
            if let color = getColor(child.getChild(1), evaluations, alpha: 1) {
                self.color = color
            }
        case PersistenceStyleKind.TextColor.name:
            if let color = getColor(child.getChild(1), evaluations, alpha: 1) {
                self.textColorValue = color
            }
        case PersistenceStyleKind.FontSize.name:
            if let value = getFloat(child.getChild(1), evaluations) {
                self.fontSize = value
                if self.fontSize > 37 {
                    self.fontSize = 36
                }
                else if self.fontSize < 4 {
                    self.fontSize = 4
                }
            }
        case PersistenceStyleKind.Display.name:
            if let value = child.getIdent(1) {
                self.display = value
            }
        case PersistenceStyleKind.Layer.name:
            if let value = child.getIdent(1) {
                self.layer = value
            }
        case PersistenceStyleKind.Layout.name:
            var layout = ""
            for i in 1..<child.count {
                if let value = child.getIdent(i) {
                    if layout.count > 0 {
                        layout += ", "
                    }
                    layout += value
                }
            }
            self.layout = layout
        case PersistenceStyleKind.Width.name:
            if let value = getFloat(child.getChild(1), evaluations) {
                if value > 10000 {
                    self.width = 10000
                } else {
                    self.width = value
                }
                
            }
        case PersistenceStyleKind.Height.name:
            if let value = getFloat(child.getChild(1), evaluations) {
                if value > 10000 {
                    self.height = 10000
                } else {
                    self.height = value
                }
            }
        case PersistenceStyleKind.BorderColor.name:
            if let color = getColor(child.getChild(1), evaluations, alpha: 1) {
                self.borderColor = color
            }
        case PersistenceStyleKind.LineStyle.name:
            if let value = child.getIdent(1) {
                self.lineStyle = value
            }
        case PersistenceStyleKind.LineWidth.name:
            if let value = getFloat(child.getChild(1), evaluations) {
                self.lineWidth = CGFloat(value)
            }
        case PersistenceStyleKind.Shadow.name:
            if let xOffset = child.getFloat(1), let yOffset = child.getFloat(2)  {
                self.shadow = CGSize(width: CGFloat(xOffset), height: CGFloat(yOffset))
            }
            if let blur = child.getFloat(3) {
                self.shadowBlur = CGFloat(blur)
            }
            if let clr = getColor(child.getChild(4), evaluations) {
                self.shadowColor = clr
            }
        default:
            break;
        }
    }
    
    func parseStyle( _ properties: ModelProperties, _ evaluations: [TennToken: JSValue ] ) {
        parseStyle( properties.node, evaluations )
    }
    func parseStyle( _ node: TennNode, _ evaluations: [TennToken: JSValue ] ) {
        for child in node.children ?? [] {
            if child.kind == .Command, child.count > 0, let cmdName = child.getIdent(0) {
                self.parseStyleLine(cmdName, child, evaluations)
            }
        }
    }
}


class DrawableLineStyle: DrawableStyle {
    override func parseStyleLine(_ cmdName: String, _ child: TennNode, _ evaluations: [TennToken: JSValue ]) {
        switch cmdName {
        default:
            super.parseStyleLine(cmdName, child, evaluations)
        }
    }
    override func newCopy() -> DrawableStyle {
        return DrawableLineStyle(darkMode)
    }
    override func copy() -> DrawableLineStyle {
        let result = super.copy() as! DrawableLineStyle
        return result
    }
    override func reset() {
        super.reset()
        
        
        if self.darkMode {
            self.color = CGColor(red: 1, green:1, blue:1, alpha: 1)
            self.textColorValue = nil
            self.borderColor = CGColor.white
        }
        else {
            self.color = CGColor(red: 0.2, green:0.2, blue:0.2, alpha: 1)
            self.textColorValue = nil
            self.borderColor = CGColor.black
        }
        
        self.lineStyle = nil
        
        self.lineWidth = CGFloat(1)
    }
}

class DrawableItemStyle: DrawableStyle {
    var title: String? = nil
    var marker: String? = nil
    
    override func newCopy() -> DrawableStyle {
        return DrawableItemStyle(darkMode)
    }
    override func copy() -> DrawableItemStyle {
        let result = super.copy() as! DrawableItemStyle
        return result
    }
    
    override func parseStyleLine(_ cmdName: String, _ child: TennNode, _ evaluations: [TennToken : JSValue]) {
        switch cmdName {
        case PersistenceStyleKind.Title.name:
            self.title = child.getIdent(1)
            if let ch =  child.getChild(1), let t = ch.token, let ev = evaluations[t] {
                self.title = ev.toString()
            }
        case PersistenceStyleKind.Marker.name:
            self.marker = child.getIdent(1)
            if let ch =  child.getChild(1), let t = ch.token, let ev = evaluations[t] {
                self.marker = ev.toString()
            }
        default:
            super.parseStyleLine(cmdName, child, evaluations)
        }
    }
}


class SceneStyle: DrawableStyle {
    var zoomLevel: CGFloat = 1
    var gridSpan = CGPoint( x: 5, y: 5)
    
    var defaultItemStyle: DrawableItemStyle
    var defaultLineStyle: DrawableLineStyle
    
    override init(_ darkMode: Bool) {
        self.defaultItemStyle =  DrawableItemStyle( darkMode )
        self.defaultLineStyle = DrawableLineStyle( darkMode )
        
        super.init( darkMode )
        
        self.defaultLineStyle.fontSize = 12
    }
    
    override func parseStyleLine(_ cmdName: String, _ child: TennNode, _ evaluations: [TennToken: JSValue ]) {
        switch cmdName {
        case PersistenceStyleKind.ZoomLevel.name:
            if let value = child.getFloat(1) {
                self.zoomLevel = CGFloat(value)
            }
        case PersistenceStyleKind.Styles.name:
            // Default styles for entire diagram
            if let childBlock = child.getChild(1), childBlock.kind == .BlockExpr, let children = childBlock.children {
                for styleChild in children {
                    if styleChild.kind == .Command, styleChild.count > 0, let styleName = styleChild.getIdent(0) {
                        switch styleName {
                        case "item":
                            if let styleProps = styleChild.getChild(1), styleProps.kind == .BlockExpr, let styles = styleProps.children {
                                defaultItemStyle.reset()
                                defaultItemStyle.parseStyle(ModelProperties(styles), evaluations)
                            }
                        case "line":
                            if let styleProps = styleChild.getChild(1), styleProps.kind == .BlockExpr, let styles = styleProps.children {
                                defaultLineStyle.reset()
                                defaultLineStyle.parseStyle(ModelProperties(styles), evaluations)
                            }
                        default:
                            break;
                        }
                    }
                }
            }
            break;
        case PersistenceStyleKind.Grid.name:
            if let x = child.getFloat(1), let y = child.getFloat(2) {
                self.gridSpan = CGPoint(x: CGFloat(x), y: CGFloat(y))
            }
            break;
        default:
            super.parseStyleLine(cmdName, child, evaluations)
        }
    }
}
private func spaceCount(_ value: String) -> Int {
    let chars = Array(value)
    var count = 0
    for c in chars {
        if c == " " {
            count += 1
        }
        else {
            break
        }
    }
    return count
}
public func prepareBodyText(_ textValue: String) -> String {
    
    let content = textValue.replacingOccurrences(of: "\\n", with: "\n")
        .replacingOccurrences(of: "\t", with: "    ")
    
    var lines = content.components(separatedBy: "\n")
    var minCount = Int.max
    
    // Remove first empty line
    if lines.count > 0 && lines[lines.startIndex].trimmingCharacters(in: .whitespacesAndNewlines).count == 0 {
        lines.removeFirst()
    }
    
    // remove last empty line.
    if lines.count > 0 && lines[lines.endIndex-1].trimmingCharacters(in: .whitespacesAndNewlines).count == 0 {
        lines.removeLast()
    }
    
    for l in lines {
        if l.trimmingCharacters(in: .whitespacesAndNewlines).count != 0 {
            let ll = spaceCount(l)
            if ll < minCount {
                minCount = ll
            }
        }
    }
    
    return lines.map({body in
        if body.trimmingCharacters(in: .whitespacesAndNewlines).count == 0 {
            return String(body + " ")
        }
        else {
            return String(body[body.index(body.startIndex, offsetBy: minCount)...])
        }
        
    }).joined(separator: "\n")
}

/**
 Cache images inside items with required processing.
 */
open class ImageProvider {
    // Image cache
    var item: DiagramItem
    let scaleFactor: CGFloat
    
    init(_ item: DiagramItem, _ scaleFactor: CGFloat ) {
        self.item = item
        self.scaleFactor = scaleFactor
    }
    
    // path is named and style combimned parsed from @(name|style)
    public func resolveImage(path: String ) -> (NSImage?, CGRect?) {
        // If image already cached.
        var image: CachedImage? = self.item.images[path]
        if image != nil {
            return (image!.image, CGRect(origin: CGPoint(x:0, y:0), size: image!.size))
        }
        var name = path
        var style = ""
        if let pos = path.firstIndex(of: "|") {
            name = String(path.prefix(upTo: pos))
            style = String(path.suffix(from: path.index(after: pos)))
        }
        
        if image == nil {
            // Retrieve image data from properties, if not cached
            self.item.properties.node.traverse {child in
                if let cmdName = child.getIdent(0), let imgName = child.getIdent(1), let imgData = child.getIdent(2) {
                    if cmdName == "image" && imgName == name {
                        if let dta = Data(base64Encoded: imgData, options: .ignoreUnknownCharacters) {
                            if let img = NSImage(data: dta) {
                                image = CachedImage( image: img, size: img.size)
                            }
                        }
                    }
                }
            }
        }
        
        if let img = image {
            var width = img.size.width
            var height = img.size.height
            if style != "" {
                var widthStr = style
                var heightStr = ""
                // We need to apply style is applicable
                if let xPos = style.firstIndex(of: "x") {
                    widthStr = String(style.prefix(upTo: xPos)).trimmingCharacters(in: .whitespacesAndNewlines)
                    heightStr = String(style.suffix(from: style.index(after: xPos))).trimmingCharacters(in: .whitespacesAndNewlines)
                }
                
                // This is aspect scale.
                if !widthStr.isEmpty, let newWidth = Int(widthStr, radix: 10) {
                    width = CGFloat(newWidth)
                }
                if !heightStr.isEmpty, let newHeight = Int(heightStr, radix: 10) {
                    height = CGFloat(newHeight)
                }
                
                if widthStr.isEmpty || heightStr.isEmpty {
                    let r = getMaxRect(maxWidth: width, maxHeight: height, imageWidth: img.size.width, imageHeight: img.size.height)
                    width = r.width
                    height = r.height
                }
            }
            img.image = rescaleImage(img.image, width * self.scaleFactor, height * self.scaleFactor)
            img.size = CGSize(width: width, height: height)
            
            //            Swift.debugPrint("image size:\(image?.size) and \(width):\(height)")
            self.item.images[path] = image
            return (img.image, CGRect(x: 0, y: 0, width: width, height: height))
        }
        
        return (nil, nil)
    }
}

func rescaleImage(_ image: NSImage, _ width: CGFloat, _ height: CGFloat) -> NSImage {
    let newSize = CGSize(width: width, height: height)
    if let bitmapRep = NSBitmapImageRep(
        bitmapDataPlanes: nil, pixelsWide: Int(newSize.width), pixelsHigh: Int(newSize.height),
        bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false,
        colorSpaceName: .calibratedRGB, bytesPerRow: 0, bitsPerPixel: 0
        ) {
        bitmapRep.size = newSize
        NSGraphicsContext.saveGraphicsState()
        NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: bitmapRep)
        image.draw(in: NSRect(x: 0, y: 0, width: newSize.width, height: newSize.height), from: .zero, operation: .copy, fraction: 1.0)
        NSGraphicsContext.restoreGraphicsState()
        
        let resizedImage = NSImage(size: newSize)
        resizedImage.addRepresentation(bitmapRep)
        return resizedImage
    }
    return image
}

open class DrawableScene: DrawableContainer {
    public var offset = CGPoint(x:0, y:0)
    
    var drawables: [DiagramItem: Drawable] = [:]
    
    var itemToLink:[DiagramItem: [DiagramItem]] = [:]
    
    var activeDrawables: [Drawable] = []
    
    var lineToDrawable: Drawable?
    
    var sceneStyle: SceneStyle
    
    var darkMode: Bool
    
    var selectionBox: CGRect?
    
    var editingMode: Bool = false {
        didSet {
            self.updateActiveElements(self.activeElements)
        }
    }
    var editBoxBounds: CGRect?
    
    var activeElements: [DiagramItem] = []
    
    var executionContext: ExecutionContextEvaluator?
    
    let scaleFactor: CGFloat
    
    init( _ element: Element, darkMode: Bool, executionContext: ExecutionContextEvaluator?, scaleFactor: CGFloat = 1, buildChildren: Bool = true, items: [DiagramItem]? = nil) {
        self.sceneStyle = SceneStyle(darkMode)
        self.darkMode = darkMode
        self.executionContext = executionContext
        self.scaleFactor = scaleFactor
        
        super.init([])
        
        self.bounds = CGRect(x:0, y:0, width: 0, height: 0)
        let buildItems = items ?? element.items
        self.append(buildElementScene(element, self.darkMode, buildChildren: buildChildren, items: buildItems))
    }
    
    public override func find( _ point: CGPoint ) -> [ItemDrawable] {
        var result: [ItemDrawable] = []      
        // Add all other items
        let findResult = super.find(point)
        for r in findResult  {
            if !result.contains(where: {e in e.item == r.item}) {
                result.append(r)
            }
        }
        return result
    }
    
    func updateLineTo(_ de: DiagramItem, _ point: CGPoint ) -> DiagramItem? {
        var result: DiagramItem?
        if let de = self.drawables[de] {
            let bounds = de.getBounds()
            
            var mid = CGPoint( x: bounds.midX, y: bounds.midY)
            
            var targetPoint = point
            
            var tBounds: CGRect?
            
            if let targetDe = self.find(point).first {
                if let ti = targetDe.item, ti.kind == .Item {
                    tBounds = targetDe.getBounds()
                    targetPoint = CGPoint( x: tBounds!.midX, y: tBounds!.midY)
                    self.updateActiveElements([ti])
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
            
            self.lineToDrawable = DrawableLine( source: mid, target: targetPoint, style: DrawableLineStyle(self.darkMode) )
        }
        return result
    }
    
    func removeLineTo() {
        self.lineToDrawable = nil
    }
    
    func updateActiveElements( _ actives: [DiagramItem], _ positions: [DiagramItem: CGPoint] = [:]) {
        activeDrawables.removeAll()
        self.activeElements = actives
        if actives.count > 0 {
            for ae in actives {
                if let de = drawables[ae] as? ItemDrawable {
                    if let line =  de as? DrawableLine {
                        self.activeDrawables.append(
                            SelectorLine(source: line.source, target: line.target, extra: line.extraPoints)
                        )
                    }
                    else {
                        var deBounds = de.getSelectorBounds()
                        if let newPos = positions[ae] {
                            deBounds.origin = newPos
                        }
                        self.activeDrawables.append(
                            SelectorBox(
                                pos: CGPoint(x: deBounds.origin.x - 5, y: deBounds.origin.y - 5 ),
                                size: CGSize(width: (editBoxBounds?.width ?? deBounds.width ) + 10, height: (editBoxBounds?.height ?? deBounds.height ) + 10 ),
                                color: !self.editingMode ? SelectorBox.normalColor: SelectorBox.editingColor
                            )
                        )
                    }
                }
            }
        }
        if let sb = self.selectionBox {
            let box = SelectorBox(
                pos: sb.origin,
                size: sb.size,
                color: SelectorBox.normalColor
            )
            box.radius = 0
            self.activeDrawables.append( box )
        }
    }
    
    func updateLayout(_ newPositions: [DiagramItem: CGPoint]) -> CGRect {
        if newPositions.count == 0 {
            return getBounds()
        }
        
        let pos = newPositions.first!.value
        var result:CGRect = CGRect(origin: pos, size: CGSize(width:1, height:1))
        for ad in activeDrawables {
            result = result.union(ad.getSelectorBounds().insetBy(dx: -40, dy: -40))
        }
        
        updateActiveElements(self.activeElements, newPositions)
        
        for ad in activeDrawables {
            result = result.union(ad.getSelectorBounds().insetBy(dx: -40, dy: -40))
        }
        
        
        for (item, pos)  in newPositions {
            result = result.union(CGRect(origin: pos, size: CGSize(width:1, height:1)))
            
            if let box = drawables[item] as? RoundBox {
                result = result.union(box.getSelectorBounds())
                box.setPath(CGRect(origin:CGPoint(x: pos.x, y: pos.y), size: box.getSelectorBounds().size))
                // Since bounds could be different
                result = result.union(box.getSelectorBounds())
            }
            if let box = drawables[item] as? CircleBox {
                result = result.union(box.getSelectorBounds())
                box.setPath(CGRect(origin:CGPoint(x: pos.x, y: pos.y), size: box.getSelectorBounds().size))
                // Since bounds could be different
                result = result.union(box.getSelectorBounds())
            }
            if let box = drawables[item] as? EmptyBox {
                result = result.union(box.getSelectorBounds())
                box.setPath(CGRect(origin:CGPoint(x: pos.x, y: pos.y), size: box.getSelectorBounds().size))
                // Since bounds could be different
                result = result.union(box.getSelectorBounds())
            }
            if let box = drawables[item] as? DrawableLine {
                result = result.union(box.getSelectorBounds())
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
                                let sourceRect = drawables[src]?.getSelectorBounds()
                                let targetRect = drawables[dst]?.getSelectorBounds()
                                
                                if let sr = sourceRect, let tr = targetRect {
                                    lnkDr.updateLayout(source: sr, target: tr)
                                }
                                result = result.union(lnkDr.getSelectorBounds())
                            }
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
        
        context.scaleBy(x: self.sceneStyle.zoomLevel, y: self.sceneStyle.zoomLevel)
        draw(context: context, at: offset)
        
        for selBox in self.activeDrawables {
            selBox.draw(context: context, at: offset)
        }
        
        if let lineTo = self.lineToDrawable {
            lineTo.draw(context: context, at: offset)
        }
    }
    open func drawBox(context: CGContext) {
        drawBox(context: context, at: offset)
        
        for selBox in self.activeDrawables {
            selBox.drawBox(context: context, at: offset)
        }
        
        if let lineTo = self.lineToDrawable {
            lineTo.draw(context: context, at: offset)
        }
    }
    
    public override func update() {
        
    }
    
    fileprivate func buildRoundRect(_ bounds: CGRect, _ style: DrawableItemStyle, _ e: DiagramItem, _ textBox: TextBox, _ elementDrawable: DrawableContainer, fill: Bool = true, stack:Int=0) {
        let rectBox = RoundBox( bounds: bounds,
                                style, fill: fill)
        rectBox.stack = stack
        rectBox.append(textBox)
        
        rectBox.item = e
        
        drawables[e] = rectBox
        elementDrawable.append(rectBox)
    }
    
    fileprivate func buildCircle(_ bounds: CGRect, _ style: DrawableStyle, _ e: DiagramItem, _ textBox: TextBox, _ elementDrawable: DrawableContainer, fill: Bool = true){
        let rectBox = CircleBox( bounds: bounds,
                                 style, fill: fill)
        rectBox.append(textBox)
        
        rectBox.item = e
        
        drawables[e] = rectBox
        elementDrawable.append(rectBox)
    }
    
    fileprivate func buildEmptyRect(_ bounds: CGRect, _ style: DrawableStyle, _ e: DiagramItem, _ textBox: TextBox, _ elementDrawable: DrawableContainer) {
        let rectBox = EmptyBox( bounds: bounds, style )
        rectBox.append(textBox)
        rectBox.item = e
        
        drawables[e] = rectBox
        elementDrawable.append(rectBox)
    }
    
    static func calculateSize(attrStr: NSAttributedString) -> CGSize {
        let fs = CTFramesetterCreateWithAttributedString(attrStr)
        
        // Need to have a attributed string without pictures, to have a proper sizes.
        let frameSize = CTFramesetterSuggestFrameSizeWithConstraints(fs, CFRangeMake(0, attrStr.length), nil, CGSize(width: 30000, height: 30000), nil)
        
        //        var size = CGSize(width: frameSize.width, height: frameSize.height )
        // Correct size
        let path = CGMutablePath()
        path.addRect(CGRect(x: 0, y: 0, width: 30000, height: 30000))
        
        let frame = CTFramesetterCreateFrame(fs, CFRangeMake(0, attrStr.length), path, nil)
        
        var size = CGSize(width:0, height:0)
        
        if let lines = CTFrameGetLines(frame) as? [CTLine] {
            var maxWidth = size.width
            for l in lines {
                var maxHeight = CGFloat(0)
                var imagesWidth = CGFloat(0)
                let range = CTLineGetStringRange(l)
                
                var ascent: CGFloat = 0
                var descent: CGFloat = 0
                var leading: CGFloat = 0
                
                let lineWidth = CGFloat(CTLineGetTypographicBounds(l, &ascent, &descent, &leading))
                
                var maxFontSize = ascent + descent
                
                maxHeight = ascent + descent + leading
                
                for i in 0..<range.length {
                    if let attr = attrStr.attribute(NSAttributedString.Key.font, at: range.location+i, effectiveRange: nil), let font = attr as? NSFont {
                        if font.pointSize > maxFontSize {
                            maxFontSize = font.pointSize
                        }
                    }
                    if let attr = attrStr.attribute(NSAttributedString.Key.attachment, at: range.location+i, effectiveRange: nil),
                        let attachment = attr as? NSTextAttachment, attachment.image != nil {
                        imagesWidth += attachment.bounds.width
                        if attachment.bounds.height > maxHeight {
                            maxHeight = attachment.bounds.height
                        }
                    }
                }
                if imagesWidth + lineWidth > maxWidth {
                    maxWidth = imagesWidth + lineWidth
                }
                size.height += maxHeight
            }
            size.width = maxWidth
        }
        Swift.debugPrint(size)
        return size
    }
    
    static func toAttributedString(tokens: [MarkdownToken], font: NSFont, color: CGColor, shift: inout CGPoint, imageProvider: ImageProvider, layout: [TextPosition]) -> NSMutableAttributedString {
        let textStyle = NSMutableParagraphStyle.default.mutableCopy() as! NSMutableParagraphStyle
        for pos in layout {
            switch pos {
            case .Center:
                textStyle.alignment = NSTextAlignment.center
            case .Left:
                textStyle.alignment = NSTextAlignment.left
                
            case .Right:
                textStyle.alignment = NSTextAlignment.right
            default:
                break;
            }
        }
        return MarkDownAttributedPrinter.toAttributedStr(tokens, font: font, paragraphStyle: textStyle, foregroundColor: NSColor(cgColor: color)!, shift: &shift, imageProvider: imageProvider)
    }
    
    fileprivate func parseLayout(_ style: DrawableItemStyle, _ horizontal: inout TextPosition, _ vertical: inout TextPosition) {
        if let l = style.layout {
            let ls = Set<String>(l.split(separator: ",").map({t in t.trimmingCharacters(in: .whitespacesAndNewlines)}))
            if ls.contains("center") {
                horizontal = .Center
            }
            if ls.contains("left") {
                horizontal = .Left
            }
            if ls.contains("right") {
                horizontal = .Right
            }
            if ls.contains("middle") {
                vertical = .Middle
            }
            if ls.contains("top") {
                vertical = .Top
            }
            if ls.contains("fill") {
                vertical = .Fill
            }
            if ls.contains("bottom") {
                vertical = .Bottom
            }
        }
    }
    
    func buildItemDrawable(_ e: DiagramItem, _ elementDrawable: DrawableContainer) {
        let name = e.name
        
        let style = self.sceneStyle.defaultItemStyle.copy()
        let evaluatedValues = self.executionContext?.getEvaluated(e) ?? [:]
        
        let imageProvider = ImageProvider(e, self.scaleFactor)
        
        // parse uses with list of styles.
        
        if let styleNode = e.properties.get( "use-style" ),
            styleNode.count > 1,
            let childs = styleNode.children,
            let parent = e.parent,
            let styles = parent.properties.get("styles"),
            let stylesBlock = styles.getChild(1) {
            let elementEvaluated = self.executionContext?.getEvaluated(parent) ?? [:]
            for c in childs[1..<childs.count] {
                if let ident = c.getIdentText(),
                    let parentStyle = stylesBlock.getNamedElement(ident),
                    let styleData = parentStyle.getChild(1) {
                    
                    style.parseStyle(styleData, elementEvaluated )
                }
            }
        }
        
        style.parseStyle(e.properties, evaluatedValues )            
        
        var titleValue = (name.count > 0 ? name :  " ")
        if let title = style.title {
            titleValue = title
        }
        titleValue = prepareBodyText(titleValue)
        // Marker value
        if let marker = style.marker {
            titleValue  = marker + " " + titleValue
        }
        var shift = CGPoint(x:0, y:0)
        
        var bodyAttrString: NSAttributedString?
        if let bodyNode = e.properties.get( "body" ) {
            // Body could have custome properties like width, height, color, font-size, so we will parse it as is.
            let bodyStyle = style.copy()
            bodyStyle.fontSize -= 2 // Make a bit smaller for body
            var textValue = ""
            if let bodyBlock = bodyNode.getChild(1) {
                if bodyBlock.kind == .BlockExpr {
                    bodyStyle.parseStyle(bodyBlock, evaluatedValues)
                    
                    if let bodyText = bodyBlock.getNamedElement("text"), let txtNode = bodyText.getChild(1) {
                        if let txt = getString(txtNode, evaluatedValues) {
                            textValue = txt
                        }
                    }
                }
                else if let txt = getString(bodyBlock, evaluatedValues) {
                    textValue = txt
                }
            }
            var vertical: TextPosition = .Middle
            var horizontal: TextPosition = .Left
            parseLayout(bodyStyle, &horizontal, &vertical)
            bodyAttrString =
                DrawableScene.toAttributedString(tokens: MarkdownLexer.getTokens(code: (titleValue.count > 0 ? "\n": "") + prepareBodyText(textValue)), font: NSFont.systemFont(ofSize: bodyStyle.fontSize), color: bodyStyle.textColor, shift: &shift, imageProvider: imageProvider, layout: [vertical, horizontal] )
        }
        
        var vertical: TextPosition = .Middle
        var horizontal: TextPosition = .Center
        
        let titleTokens = MarkdownLexer.getTokens(code: titleValue)
        
        if bodyAttrString != nil {
            vertical = .Fill
            horizontal = .Left
        }
        
        // If markdown has titles, bullets, we need to change horizontal layout to left one.
        if titleTokens.contains(where: {e in [.bullet, .title, .code].contains(e.type)}) {
            horizontal = .Left
        }
        
        
        parseLayout(style, &horizontal, &vertical)
        
        let attrString = DrawableScene.toAttributedString(tokens: titleTokens, font: NSFont.systemFont(ofSize: style.fontSize), color: style.textColor, shift: &shift, imageProvider: imageProvider, layout: [horizontal, vertical] )
        
        //        do {
        //            let dta = try attrString.data(from: NSMakeRange(0, attrString.length), documentAttributes: [ NSAttributedString.DocumentAttributeKey.documentType:NSAttributedString.DocumentType.html ])
        //            Swift.debugPrint(String(data: dta, encoding: String.Encoding.utf8))
        //        }
        //        catch let error {
        //            // Ignore
        //        }
        
        
        if bodyAttrString != nil {
            attrString.append(bodyAttrString!)
        }
        let textBounds = DrawableScene.calculateSize(attrStr: attrString)
        
        var offx = CGFloat(4)
        var offy = CGFloat(4)
        
        var wx=offx*2
        var wy=offy*2
        
        
        var width = max(20, textBounds.width)
        if let styleWidth = style.width, styleWidth >= 1 {
            width = styleWidth //max(width, styleWidth)
            wx = 0
        }
        
        var height = max(20, textBounds.height)
        if let styleHeight = style.height, styleHeight >= 1 {
            height = styleHeight//max(height, styleHeight)
            wy = 0
        }
        
        if width - offx > textBounds.width {
            wx = 0
            offx = (width - textBounds.width) / 2
        }
        if height - offy > textBounds.height {
            wy = 0
            offy = (height - textBounds.height) / 2
        }
        
        let sz = CGSize(width: width + shift.x+wx, height: height + shift.y + wy )
        bounds = CGRect(origin: CGPoint(x:e.x, y:e.y - sz.height), size: sz)
        
        var finalTextBounds = CGRect( origin: CGPoint(x:offx, y:offy), size: CGSize(width: textBounds.width + shift.x, height: textBounds.height + shift.y))
        
        //        var resultHtmlText = ""
        //               do {
        //
        //                   let r = NSRange(location: 0, length: attrString.length)
        //                   let att = [NSAttributedString.DocumentAttributeKey.documentType: NSAttributedString.DocumentType.html]
        //
        //                   let d = try attrString.data(from: r, documentAttributes: att)
        //
        //                   if let h = String(data: d, encoding: .utf8) {
        //                       resultHtmlText = h
        //                   }
        //               }
        //               catch {
        //                   print("utterly failed to convert to html!!! \n>\(x)<\n")
        //               }
        //               print(resultHtmlText)
        
        if finalTextBounds.size.width + offx*2 < width {
            finalTextBounds.size.width = width - offx*2
        }
        if finalTextBounds.size.height + offy*2 < height {
            finalTextBounds.size.height = height - offy*2
        }
        
        switch vertical {
        case .Middle:
            if finalTextBounds.height > textBounds.height {
                let yshift = (finalTextBounds.height - textBounds.height )
                finalTextBounds.origin.y = offy + yshift / 2
                finalTextBounds.size.height -= yshift
            } else {
                if bounds.height < finalTextBounds.height {
                    let yshift = ( finalTextBounds.height - bounds.height + offy )
                    finalTextBounds.origin.y = offy - ( yshift / 2 )
                }
            }
        case .Top:
            if finalTextBounds.height > textBounds.height {
                let yshift = (finalTextBounds.height - textBounds.height )
                finalTextBounds.origin.y = offy + yshift
                finalTextBounds.size.height -= yshift
            } else {
                if bounds.height < finalTextBounds.height {
                    let yshift = ( finalTextBounds.height - bounds.height + offy )
                    finalTextBounds.origin.y = offy - ( yshift / 2 )
                }
            }
            break
        case .Fill:
            if bounds.height < finalTextBounds.height {
                let yshift = ( finalTextBounds.height - bounds.height )
                finalTextBounds.origin.y = -1 * ( offy + yshift)
            }
            break
        case .Bottom:
            if finalTextBounds.height > textBounds.height {
                let yshift = (finalTextBounds.height - textBounds.height )
                finalTextBounds.origin.y = offy
                finalTextBounds.size.height -= yshift
            } else {
                if bounds.height < finalTextBounds.height {
                    let yshift = ( finalTextBounds.height - bounds.height + offy )
                    finalTextBounds.origin.y = offy - ( yshift / 2 )
                }
            }
        default:
            break;
        }
        
        // If we do not fit for text bounds into finalTextBounds, we need to move down.
        
        let textBox = TextBox(
            text: attrString,
            bounds: finalTextBounds)
        
        if let display = style.display {
            switch display {
            case "text":
                buildEmptyRect(bounds, style, e, textBox, elementDrawable)
            case "no-fill":
                var tc = style.textColorValue
                if tc == nil {
                    tc = getTextColorBasedOn(PreferenceConstants.preference.background)
                }
                buildRoundRect(bounds, style, e, textBox, elementDrawable, fill: false)
            case "stack":
                buildRoundRect(bounds, style, e, textBox, elementDrawable, stack: 3)
            case "circle":
                buildCircle(bounds, style, e, textBox, elementDrawable)
                break;
            default:
                buildRoundRect(bounds, style, e, textBox, elementDrawable, fill: true)
            }
        }
        else {
            buildRoundRect(bounds, style, e, textBox, elementDrawable)
        }
    }
    
    func buildElementScene( _ element: Element, _ darkMode: Bool, buildChildren: Bool, items: [DiagramItem] )-> Drawable {
        let elementDrawable = DrawableContainer()
        
        self.sceneStyle = SceneStyle(darkMode)
        let evaluated = self.executionContext?.getEvaluated(element) ?? [:]
        self.sceneStyle.parseStyle(element.properties, evaluated)
        
        var links: [DiagramItem] = []
        
        if buildChildren {
            buildItems(items, elementDrawable, &links)
            for e in links {
                if let data = e as? LinkItem {
                    
                    let linkStyle = self.sceneStyle.defaultLineStyle.copy()
                    linkStyle.parseStyle(e.properties, evaluated)
                    var sr: CGRect = CGRect(x: 0, y: 0, width: 5, height: 5)
                    var tr: CGRect = CGRect(x: 0, y: 5, width: 5, height: 5)
                    
                    if let src = data.source, let srr = (drawables[src]?.getSelectorBounds()) {
                        self.addLink( src, e )
                        sr = srr
                    }
                    if let dst = data.target, let trr = drawables[dst]?.getSelectorBounds() {
                        self.addLink( dst, e )
                        tr = trr
                    }
                    let linkDr = DrawableLine(
                        source: sr,
                        target: tr,
                        style: linkStyle,
                        control: CGPoint(x: e.x, y: e.y ))
                    
                    if data.name.count > 0 {
                        linkDr.addLabel(data.name, imageProvider: ImageProvider(e, self.scaleFactor))
                    }
                    
                    linkDr.item = e
                    drawables[e] = linkDr
                    elementDrawable.insert(
                        linkDr, at: 0)
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

func getShadowRect(_ bounds: CGRect, _ style:DrawableStyle) -> CGRect {
    var result = CGRect(origin: CGPoint(x: bounds.origin.x, y: bounds.origin.y), size: CGSize(width: bounds.width, height: bounds.height))
    // In case we had shadow, we need to extend bounds a bit.
    if let shadow = style.shadow {
        if shadow.width < 0 {
            result.origin.x += shadow.width - style.shadowBlur * 2
            result.size.width += -1 * shadow.width + style.shadowBlur * 4
        }
        if shadow.width > 0 {
            result.size.width += shadow.width + style.shadowBlur * 2
        }
        
        if shadow.height < 0 {
            result.origin.y += shadow.height - style.shadowBlur * 2
            result.size.height += -1 * shadow.height + style.shadowBlur * 4
        }
        if shadow.height > 0 {
            result.size.height += shadow.height + style.shadowBlur * 2
        }
    }
    return result
}

public class RoundBox: DrawableContainer {
    var style: DrawableItemStyle
    
    var radius: CGFloat = 8
    var fill: Bool = true
    var stack: Int = 0
    var stackStep = CGPoint(x:5, y:5)
    
    var path: CGMutablePath?
    
    init( bounds: CGRect, _ style: DrawableItemStyle, fill: Bool ) {
        self.style = style
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
        
        if let pos = self.style.shadow {
            context.saveGState()
            if let col = self.style.shadowColor {
                context.setShadow(offset: pos, blur: self.style.shadowBlur, color: col)
            } else {
                context.setShadow(offset: pos, blur: self.style.shadowBlur)
            }
            self.doDraw(context, at: point)            
            context.restoreGState()
        } else {
            self.doDraw(context, at: point)
        }
        
        
        var clipBounds = CGRect( origin: CGPoint(x: bounds.origin.x + point.x, y: bounds.origin.y + point.y), size: bounds.size)
        clipBounds.size.width -= 1
        if OPTION_perform_clip {
            context.clip(to: clipBounds )
        }
        super.draw(context: context, at: CGPoint(x: self.bounds.minX + point.x, y: self.bounds.minY + point.y))
        context.restoreGState()
    }
    
    func doDraw(_ context:CGContext, at point: CGPoint) {
        context.saveGState()
        
        context.setLineWidth( self.style.lineWidth )
        context.setStrokeColor( self.style.borderColor )
        context.setFillColor( self.style.color )
        
        updateLineStyle(context, self.style)
        
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
        
        //        context.stroke(getBounds(), width: 1)
        
        context.restoreGState()
    }
    
    public override func layout(_ bounds: CGRect, _ dirty: CGRect) {
        let selfBounds = self.bounds
        
        if let ch = self.children {
            for c in ch {
                c.layout( selfBounds, dirty )
            }
        }
        let newBounds = self.getBounds()
        visible = dirty.intersects(newBounds)
    }
    
    public override func getSelectorBounds() -> CGRect {
        return bounds
    }
    
    public override func getBounds() -> CGRect {
        return getShadowRect(self.bounds, self.style).insetBy(dx: -1*style.lineWidth, dy: -1*style.lineWidth)
    }
}

public class EmptyBox: DrawableContainer {
    var style: DrawableStyle
    
    init( bounds: CGRect, _ style: DrawableStyle ) {
        self.style = style
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
        
        if let pos = self.style.shadow {
            if let col = self.style.shadowColor {
                context.setShadow(offset: pos, blur: self.style.shadowBlur, color: col)
            } else {
                context.setShadow(offset: pos, blur: self.style.shadowBlur)
            }
        }
        let clipBounds = CGRect( origin: CGPoint(x: bounds.origin.x + point.x, y: bounds.origin.y + point.y), size: bounds.size)
        if OPTION_perform_clip {
            context.clip(to: clipBounds )
        }
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

public enum TextPosition {
    case Left
    case Right
    case Center  // Horizomtal
    
    case Top
    case Fill
    case Bottom
    case Middle // Vertical
}
public class TextBox: Drawable {
    var frame: CGRect
    var attrStr: NSAttributedString
    
    public init( text: NSAttributedString, bounds: CGRect) {
        self.frame = bounds
        self.attrStr = text
    }
    
    public func setFrame(_ bounds: CGRect ) {
        self.frame = bounds
    }
    
    public func isVisible() -> Bool {
        return true
    }
    
    public func drawBox(context: CGContext, at point: CGPoint) {
        
    }
    
    public func traverse(_ op: (Drawable) -> Bool) {
        _ = op(self)
    }
    
    public func draw(context: CGContext, at point: CGPoint) {
        let atp = CGPoint(x: point.x + self.frame.origin.x, y: point.y + self.frame.origin.y )
        let atr = CGRect(origin: atp, size: self.frame.size)
        self.attrStr.draw(in: atr)
        
        if OPTION_perform_text_stroke {
            context.stroke(atr)
        }
    }
    
    public func layout(_ parentBounds: CGRect, _ dirty: CGRect) {
    }
    
    public func getSelectorBounds() -> CGRect {
        return frame
    }
    public func getBounds() -> CGRect {
        return frame
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
    var style: DrawableLineStyle
    var control: CGPoint
    
    var label: TextBox?
    
    init( source: CGRect, target: CGRect, style: DrawableLineStyle, control: CGPoint = CGPoint.zero) {
        self.sourceRect = source
        self.targetRect = target
        self.style = style
        self.control = control
        super.init()
        
        self.updateLayout(source: source, target: target)
    }
    
    init( source: CGPoint, target: CGPoint, style: DrawableLineStyle) {
        self.source = source
        self.target = target
        self.style = style
        self.control = CGPoint.zero
    }
    
    func addLabel(_ label: String, imageProvider: ImageProvider) {
        var shift = CGPoint(x:0, y:0)
        let attrStr = DrawableScene.toAttributedString(tokens: MarkdownLexer.getTokens(code: label.replacingOccurrences(of: "\\n", with: "\n")), font: NSFont.systemFont(ofSize: self.style.fontSize), color: self.style.borderColor, shift: &shift, imageProvider: imageProvider, layout: [.Center, .Middle])
        let size = DrawableScene.calculateSize(attrStr: attrStr)
        self.label = TextBox(text: attrStr,
                             bounds: CGRect(origin: CGPoint.zero, size: size))
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
        
        if self.style.layout == "quad" {
            let (aPath, _, _, _, _) = self.buildPath(CGPoint(x:0, y:0))
            for i in -3..<3 {
                for j in -3..<3 {
                    if aPath.contains(CGPoint(x: point.x + CGFloat(i), y: point.y + CGFloat(j)), using: .evenOdd, transform: .identity) {
                        return true
                    }
                }
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
            
            var fromToLen = sqrt(pow(p1.x-p2.x,2)+pow(p1.y-p2.y,2))
            if fromToLen == 0 {
                fromToLen = 1
            }
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
    
    fileprivate func getLabelPosition(_ lbl: TextBox) -> CGPoint {
        let lblBounds = lbl.getBounds()
        var nx = (source.x + target.x)/2
        
        if abs(source.y-target.y) < 10 {
            nx -= lblBounds.width/2
        }
        var ny = (source.y + target.y)/2
        if abs(source.x-target.x) < 10 {
            ny -= lblBounds.height/2
        }
        return CGPoint(x: nx+5, y: ny)
    }
    private func buildPath(_ point: CGPoint) -> (CGMutablePath, CGPoint?, Bool, CGPoint, CGPoint) {
        let aPath =  CGMutablePath()
        var labelPoint: CGPoint?
        let drawArrowTarget = self.style.display == "arrow" || self.style.display == "arrows"
        let drawArrowSoure = self.style.display == "arrow-source" || self.style.display == "arrows"
        let drawArrow = drawArrowTarget || drawArrowSoure
        
        var fromPt = CGPoint(x: source.x + point.x, y: source.y + point.y)
        var fromPtLast = fromPt
        var toPt = CGPoint( x: target.x + point.x, y: target.y + point.y)
        
        aPath.move(to: fromPt)
        
        let quad = self.style.layout == "quad"
        
        var quadCp: CGPoint = fromPt
        for ep in self.extraPoints {
            
            // Move from pt to new location
            fromPtLast = CGPoint(x: ep.x + point.x, y: ep.y + point.y)
            
            if drawArrowSoure {
                // We need to move source point a bit less
                let px = fromPtLast.x - fromPt.x
                let py = fromPtLast.y - fromPt.y
                
                let plen = sqrt( px * px + py * py )
                if plen > 10 {
                    let ltoPt = CGPoint( x: fromPt.x + ( (px / plen) * 5 ), y: fromPt.y + ( (py / plen) * 5 ) )
                    aPath.move(to: ltoPt)
                    fromPt = ltoPt
                }
            }
            if quad {
                quadCp = fromPtLast
            }
            else {
                aPath.addLine(to: fromPtLast)
            }
            
            if let lbl = self.label {
                labelPoint = CGPoint(x: ep.x + 5, y: ep.y - lbl.getBounds().height)
            }
        }
        
        if self.extraPoints.isEmpty {
            if let lbl = self.label {
                labelPoint = getLabelPosition(lbl)
            }
        }
        
        if drawArrowTarget {
            // We need to draw a bit less
            let px = toPt.x - fromPtLast.x
            let py = toPt.y - fromPtLast.y
            
            let plen = sqrt( px * px + py * py )
            
            if plen > 10 {
                let ltoPt = CGPoint( x: toPt.x - ( px / plen * 5 ), y: toPt.y - ( py / plen * 5 ) )
                if quad {
                    aPath.addQuadCurve(to: ltoPt, control: quadCp)
                }
                else {
                    aPath.addLine(to: ltoPt)
                }
                toPt = ltoPt
            } else {
                if quad {
                    aPath.addQuadCurve(to: toPt, control: quadCp)
                }else {
                    aPath.addLine(to: toPt)
                }
            }
        } else {
            if quad {
                aPath.addQuadCurve(to: toPt, control: quadCp)
            }
            else {
                aPath.addLine(to: toPt)
            }
        }
        return (aPath, labelPoint, drawArrow, fromPt, toPt)
    }
    public override func draw(context: CGContext, at point: CGPoint) {
        //
        context.saveGState()
        
        context.setLineWidth( self.lineWidth )
        context.setStrokeColor(self.style.color)
        context.setFillColor(self.style.color)
        
        updateLineStyle(context, style)
        
        let (aPath, labelPoint, drawArrow, fromPt, toPt) = self.buildPath(point)
        //        aPath.closeSubpath()
        var fillType: CGPathDrawingMode = .stroke
        
        context.addPath(aPath)
        context.drawPath(using: fillType)
        
        if drawArrow {
            fillType = .fillStroke
            
            var spt = extraPoints.count > 0 ? CGPoint(x: extraPoints[0].x + point.x, y: extraPoints[0].y + point.y) : toPt
            var ept = extraPoints.count > 0 ? CGPoint(x: extraPoints[extraPoints.count-1].x + point.x, y: extraPoints[extraPoints.count-1].y + point.y) : fromPt
            
            if self.style.display == "arrow" || self.style.display == "arrows" {
                let px = ept.x - toPt.x
                let py = ept.y - toPt.y
                
                let plen = sqrt( px * px + py * py )
                if plen > 10 {
                    ept = CGPoint( x: toPt.x + ( (px / plen) * 10 ), y: toPt.y + ( (py / plen) * 10 ) )
                }
                if let arr = arrow(from: ept, to: toPt,
                                   tailWidth: 0, headWidth: 10, headLength: 10) {
                    context.addPath(arr)
                }
            }
            if self.style.display == "arrow-source" || self.style.display == "arrows" {
                let px = spt.x - fromPt.x
                let py = spt.y - fromPt.y
                
                let plen = sqrt( px * px + py * py )
                if plen > 10 {
                    spt = CGPoint( x: fromPt.x + ( (px / plen) * 10 ), y: fromPt.y + ( (py / plen) * 10 ) )
                }
                if let arr = arrow(from: spt, to: fromPt,
                                   tailWidth: 0, headWidth: 10, headLength: 10) {
                    context.addPath(arr)
                }
            }
            context.drawPath(using: fillType)
        }
        
        if let lbl = self.label, let pp = labelPoint {
            lbl.draw(context: context, at: CGPoint(x: point.x + pp.x, y: point.y + pp.y))
        }
        
        
        //        let b = getSelectorBounds()
        //        context.stroke(CGRect(origin: CGPoint(x: point.x + b.origin.x, y: point.y + b.origin.y), size: b.size))
        
        
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
        
        let (aPath, _, drawArrow, _, _) = self.buildPath(CGPoint(x:0, y:0))
        
        let box = aPath.boundingBox
        
        minX = min(box.minX, minX)
        maxX = max(box.maxX, maxX)
        minY = min(box.minY, minY)
        maxY = max(box.maxY, maxY)
        
        if drawArrow {
            minX -= 10
            minY -= 10
            maxX += 10
            maxY += 10
        }
        
        
        
        return CGRect(x:minX, y:minY, width:abs(maxX-minX), height:max(abs(maxY-minY), 5.0)).insetBy(dx: -1*style.lineWidth, dy: -1*style.lineWidth)
    }
    public override func getSelectorBounds() -> CGRect {
        var minX = min(source.x, target.x)
        var maxX = max(source.x, target.x)
        var minY = min(source.y, target.y)
        var maxY = max(source.y, target.y)
        
        for ep in self.extraPoints {
            minX = min(ep.x, minX)
            maxX = max(ep.x, maxX)
            minY = min(ep.y, minY)
            maxY = max(ep.y, maxY)
            
            if let lbl = self.label {
                let point = CGPoint(x: ep.x + 5, y: ep.y - lbl.getBounds().height)
                let lblb = lbl.getSelectorBounds()
                maxX = max( maxX, point.x + lblb.width + 5)
                minY = min( minY, point.y - 5)
            }
        }
        if self.extraPoints.count == 0 {
            if let lbl = self.label {
                let point = getLabelPosition(lbl)
                let lblb = lbl.getSelectorBounds()
                
                maxX = max( maxX, point.x + lblb.width + 5)
                
                minY = min( minY, point.y - 5)
            }
        }
        return CGRect(x:minX, y:minY, width:abs(maxX-minX), height:max(abs(maxY-minY), 5.0)).insetBy(dx: -1*style.lineWidth, dy: -1*style.lineWidth)
    }
    public func getLabelBounds() -> CGRect {
        for ep in self.extraPoints {
            if let lbl = self.label {
                let point = CGPoint(x: ep.x + 5, y: ep.y - lbl.getBounds().height)
                let lblb = lbl.getSelectorBounds()
                return CGRect(origin: point, size: lblb.size)
            }
        }
        if self.extraPoints.count == 0 {
            if let lbl = self.label {
                let point = getLabelPosition(lbl)
                let lblb = lbl.getSelectorBounds()
                return CGRect(origin: point, size: lblb.size)
            }
        }
        let b = getBounds()
        return CGRect(x: b.midX - 50, y: b.midY - 15, width: 100, height: 30)
    }
    public override func update() {
    }
}

public class SelectorBox: Drawable {
    var pos: CGPoint
    var size: CGSize
    
    var color: CGColor
    
    var lineWidth: CGFloat = 1
    
    var radius:CGFloat = 9.0
    
    static let normalColor = CGColor(red: 0, green: 0, blue: 1, alpha: 1)
    static let editingColor = CGColor(red: 0, green: 1, blue: 0, alpha: 1)
    
    init( pos: CGPoint, size: CGSize, color: CGColor = normalColor) {
        self.pos = pos
        self.size = size
        self.color = color
    }
    
    public func traverse(_ op: (Drawable) -> Bool) {
        _ = op(self)
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
    public func getSelectorBounds() -> CGRect {
        return getBounds()
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



public class ImageBox: Drawable {
    var pos: CGPoint
    var size: CGSize
    var img: CGImage
    
    init( pos: CGPoint, size: CGSize, img: CGImage) {
        self.pos = pos
        self.size = size
        self.img = img
    }
    
    public func traverse(_ op: (Drawable) -> Bool) {
        _ = op(self)
    }
    
    public func drawBox(context: CGContext, at point: CGPoint) {
        self.draw(context: context, at: point)
    }
    
    public func draw(context: CGContext, at point: CGPoint) {
        context.draw(self.img, in: CGRect(origin: CGPoint(x: pos.x + point.x, y: pos.y + point.y), size: size))
    }
    
    public func layout(_ bounds: CGRect, _ dirty: CGRect) {
        
    }
    
    public func isVisible() -> Bool {
        return true
    }
    public func getSelectorBounds() -> CGRect {
        return self.getBounds()
    }
    public func getBounds() -> CGRect {
        return CGRect(origin: self.pos, size: self.size)
    }
    public func update() {
    }
}


public class CircleBox: DrawableContainer {
    var style: DrawableStyle
    var fill: Bool = true
    
    var path: CGMutablePath?
    
    init( bounds: CGRect, _ style: DrawableStyle, fill: Bool ) {
        self.style = style
        self.fill = fill
        
        super.init([])
        self.setPath(bounds)
    }
    
    func setPath( _ rect: CGRect) {
        self.bounds = rect
    }
    public override func drawBox(context: CGContext, at point: CGPoint) {
        // We only need to draw rect for shadow
        self.doDraw(context, at: point)
    }
    
    public override func draw(context: CGContext, at point: CGPoint) {
        context.saveGState()
        
        context.saveGState()
        if let pos = self.style.shadow {
            if let col = self.style.shadowColor {
                context.setShadow(offset: pos, blur: self.style.shadowBlur, color: col)
            } else {
                context.setShadow(offset: pos, blur: self.style.shadowBlur)
            }
        }
        context.beginPath()
        self.doDraw(context, at: point)
        context.restoreGState()
        let clipBounds = CGRect( origin: CGPoint(x: bounds.origin.x + point.x + 2, y: bounds.origin.y + point.y + 2), size: CGSize(width: bounds.size.width-4, height: bounds.size.height-4))
        context.addEllipse(in: clipBounds )
        if OPTION_perform_clip {
            context.clip(using: .winding)
        }
        
        super.draw(context: context, at: CGPoint(x: self.bounds.minX + point.x, y: self.bounds.minY + point.y))
        context.restoreGState()
    }
    
    func doDraw(_ context:CGContext, at point: CGPoint) {
        context.saveGState()
        
        context.setLineWidth(style.lineWidth)
        updateLineStyle(context, style)
        context.setStrokeColor( self.style.borderColor )
        context.setFillColor( self.style.color )
        
        context.translateBy(x: point.x, y: point.y)
        
        context.addEllipse(in: self.bounds )
        
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
        let newBounds = self.getBounds()
        visible = dirty.intersects(newBounds)
    }
    
    public override func getSelectorBounds() -> CGRect {
        return self.bounds
    }
    
    public override func getBounds() -> CGRect {
        return getShadowRect(self.bounds, self.style)
    }
}
