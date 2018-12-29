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

class DrawableStyle {
    var color: CGColor = CGColor(red: 1.0, green:1.0, blue:1.0, alpha: 0.7)
    var textColor: CGColor = CGColor.black
    
    var borderColor: CGColor = CGColor.black
    var fontSize:CGFloat = 18.0
    var width:CGFloat?
    var height:CGFloat?
    var darkMode = false
    
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
    
    init( _ darkMode: Bool ) {
        self.darkMode = darkMode
        reset()
    }
    init( item: DiagramItem ) {
        reset()
        self.parseStyle(item.properties)
    }
    
    func newCopy() -> DrawableStyle {
        return DrawableStyle(darkMode)
    }
    func copy() -> DrawableStyle {
        let result: DrawableStyle = newCopy()
        result.color = self.color
        result.textColor = self.textColor
        result.borderColor = self.borderColor
        result.fontSize = self.fontSize
        result.width = self.width
        result.height = self.height
        result.display = self.display
        result.layout = self.layout
        return result
    }
    
    
    func reset() {
        // Reset to default values
        
        if self.darkMode {
            self.color = CGColor(red: 0.2, green:0.2, blue:0.2, alpha: 0.7)
            self.textColor = CGColor(red: 0.847, green: 0.822, blue: 0.862, alpha: 1.0)
            self.borderColor = CGColor.white
        }
        else {
            self.color = CGColor(red: 1.0, green:1.0, blue:1.0, alpha: 0.7)
            self.textColor = CGColor(red: 0.147, green: 0.222, blue: 0.162, alpha: 1.0)
            self.borderColor = CGColor.black
        }
        
        self.fontSize = 18
        self.width = nil
        self.height = nil
        self.display = nil
        self.layout = nil
    }

    
    func hexStringToUIColor (hexString:String, alpha: CGFloat = 1.0) -> CGColor {
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
    
    func parseColor(_ color: String, alpha: CGFloat = 1.0  ) -> CGColor {
        if color.starts(with: "#") {
            return hexStringToUIColor(hexString: color, alpha: alpha)
        }
        if let cl = ColorNames[color] {
            return hexStringToUIColor(hexString: cl, alpha: alpha)
        }
        
        return CGColor.black.copy(alpha: alpha)!
    }
    
    func parseStyleLine(_ cmdName: String, _ child: TennNode) {
        switch cmdName {
        case PersistenceStyleKind.Color.name:
            if let color = child.getIdent(1) {
                self.color = self.parseColor(color.lowercased(), alpha: 0.7)
            }
        case PersistenceStyleKind.TextColor.name:
            if let color = child.getIdent(1) {
                self.textColor = self.parseColor(color.lowercased(), alpha: 1)
            }
        case PersistenceStyleKind.FontSize.name:
            if let value = child.getFloat(1) {
                self.fontSize = CGFloat(value)
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
        case PersistenceStyleKind.Layout.name:
            if let value = child.getIdent(1) {
                self.layout = value
            }
        case PersistenceStyleKind.Width.name:
            if let value = child.getFloat(1) {
                self.width = CGFloat(value)
            }
        case PersistenceStyleKind.Height.name:
            if let value = child.getFloat(1) {
                self.height = CGFloat(value)
            }
        case PersistenceStyleKind.BorderColor.name:
            if let color = child.getIdent(1) {
                self.borderColor = self.parseColor(color.lowercased())
            }
        default:
            break;
        }
    }
    
    func parseStyle( _ properties: ModelProperties ) {
        parseStyle( properties.node )
    }
    func parseStyle( _ node: TennNode ) {
        for child in node.children ?? [] {
            if child.kind == .Command, child.count > 0, let cmdName = child.getIdent(0) {
                self.parseStyleLine(cmdName, child)
            }
        }
    }
}


class DrawableLineStyle: DrawableStyle {
    var lineDash:String?
    
    override func parseStyleLine(_ cmdName: String, _ child: TennNode) {
        switch cmdName {
        case PersistenceStyleKind.LineStyle.name:
            if let value = child.getIdent(1) {
                self.lineDash = value
            }
        default:
            super.parseStyleLine(cmdName, child)
        }
    }
    override func newCopy() -> DrawableStyle {
        return DrawableLineStyle(darkMode)
    }
    override func copy() -> DrawableLineStyle {
        let result = super.copy() as! DrawableLineStyle
        result.lineDash = self.lineDash
        return result
    }
    override func reset() {
        super.reset()
        
        
        if self.darkMode {
            self.color = CGColor(red: 1, green:1, blue:1, alpha: 0.7)
            self.textColor = CGColor(red: 0.847, green: 0.822, blue: 0.862, alpha: 1.0)
            self.borderColor = CGColor.white
        }
        else {
            self.color = CGColor(red: 0.2, green:0.2, blue:0.2, alpha: 0.7)
            self.textColor = CGColor(red: 0.147, green: 0.222, blue: 0.162, alpha: 1.0)
            self.borderColor = CGColor.black
        }
        
        self.lineDash = nil
    }
}

class DrawableItemStyle: DrawableStyle {
    override func newCopy() -> DrawableStyle {
        return DrawableItemStyle(darkMode)
    }
    override func copy() -> DrawableItemStyle {
        let result = super.copy() as! DrawableItemStyle
        return result
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
    
    override func parseStyleLine(_ cmdName: String, _ child: TennNode) {
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
                                defaultItemStyle.parseStyle(ModelProperties(styles))
                            }
                        case "line":
                            if let styleProps = styleChild.getChild(1), styleProps.kind == .BlockExpr, let styles = styleProps.children {
                                defaultLineStyle.reset()
                                defaultLineStyle.parseStyle(ModelProperties(styles))
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
            super.parseStyleLine(cmdName, child)
        }
    }
}


open class DrawableScene: DrawableContainer {
    public var offset = CGPoint(x:0, y:0)
    
    var drawables: [DiagramItem: Drawable] = [:]
    
    var itemToLink:[DiagramItem: [DiagramItem]] = [:]
    
    var activeDrawables: [Drawable] = []
    
    var lineToDrawable: Drawable?
    
    var sceneStyle: SceneStyle
    
    var darkMode: Bool
    
    var editingMode: Bool = false {
        didSet {
            self.updateActiveElements(self.activeElements)
        }
    }
    
    var activeElements: [DiagramItem] = []
    
    init( _ element: Element, darkMode: Bool) {
        self.sceneStyle = SceneStyle(darkMode)
        self.darkMode = darkMode
        
        super.init([])
        
        self.bounds = CGRect(x:0, y:0, width: 0, height: 0)
        
        self.append(buildElementScene(element, self.darkMode))
    }
    
    public override func find( _ point: CGPoint ) -> ItemDrawable? {
        for active in self.activeElements {
            if let activeDr = drawables[active] {
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
                        var deBounds = de.getBounds()
                        if let newPos = positions[ae] {
                            deBounds.origin = newPos
                        }
                        self.activeDrawables.append(
                            SelectorBox(
                                pos: CGPoint(x: deBounds.origin.x - 5, y: deBounds.origin.y - 5 ),
                                size: CGSize(width: deBounds.width + 10, height: deBounds.height + 10 ),
                                color: !self.editingMode ? SelectorBox.normalColor: SelectorBox.editingColor
                            )
                        )
                    }
                }
            }
        }
    }
    
    func updateLayout(_ newPositions: [DiagramItem: CGPoint]) -> CGRect {
        if newPositions.count == 0 {
            return getBounds()
        }
        updateActiveElements(self.activeElements, newPositions)
        
        var pos = newPositions.first!.value
        
        var result:CGRect = CGRect(origin: pos, size: CGSize(width:1, height:1))
        
        for ad in activeDrawables {
            result = result.union(ad.getBounds())
        }
        
        for (item, pos)  in newPositions {
            result = result.union(CGRect(origin: pos, size: CGSize(width:1, height:1)))
            
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
    
    fileprivate func buildRoundRect(_ bounds: CGRect, _ bgColor: CGColor, _ borderColor: CGColor, _ e: DiagramItem, _ textBox: TextBox, _ elementDrawable: DrawableContainer, fill: Bool = true, stack:Int=0) -> RoundBox {
        let rectBox = RoundBox( bounds: bounds,
                                fillColor: bgColor,
                                borderColor: borderColor, fill: fill)
        rectBox.stack = stack
        if self.activeElements.contains(e) {
            rectBox.lineWidth = 1
        }
        rectBox.append(textBox)
        
        rectBox.item = e
        
        drawables[e] = rectBox
        elementDrawable.append(rectBox)
        return rectBox
    }
    fileprivate func buildEmptyRect(_ bounds: CGRect, _ e: DiagramItem, _ textBox: TextBox, _ elementDrawable: DrawableContainer) -> EmptyBox {
        let rectBox = EmptyBox( bounds: bounds )
        rectBox.append(textBox)
        rectBox.item = e
        
        drawables[e] = rectBox
        elementDrawable.append(rectBox)
        return rectBox
    }
    
    fileprivate func prepareBodyText(_ textValue: String) -> String {
        let content = textValue.replacingOccurrences(of: "\\n", with: "\n").trimmingCharacters(in: NSCharacterSet.whitespaces)
        return content.split(separator: "\n").map({body in body.trimmingCharacters(in: NSCharacterSet.whitespaces)}).joined(separator: "\n")
    }
    
    func buildItemDrawable(_ e: DiagramItem, _ elementDrawable: DrawableContainer) {
        let name = e.name
        
        let style = self.sceneStyle.defaultItemStyle.copy()
        style.parseStyle(e.properties)
        
        let bgColor = style.color
        let borderColor = style.borderColor
        
        var bodyTextBox: TextBox? = nil
        
        if let bodyNode = e.properties.get( "body" ) {
            // Body could have custome properties like width, height, color, font-size, so we will parse it as is.
            let bodyStyle = style.copy()
            bodyStyle.fontSize -= 2 // Make a bit smaller for body
            var textValue = ""
            if let bodyBlock = bodyNode.getChild(1) {
                if bodyBlock.kind == .BlockExpr {
                    bodyStyle.parseStyle(bodyBlock)
                    
                    if let bodyText = bodyBlock.getNamedElement("text") {
                        if let txtValue = bodyText.getIdent(1) {
                            textValue = txtValue
                        }
                        else if let block = bodyText.getChild(1), block.kind == .BlockExpr {
                            block.childsToStr(&textValue, 0, true)
                        }
                    }
                }
                else if bodyBlock.kind == .Ident || bodyBlock.kind == .StringLit, let strVal = bodyBlock.getIdentText() {
                    textValue = strVal
                }
            }
            bodyTextBox = TextBox(
                text: prepareBodyText(textValue),
                textColor: bodyStyle.textColor,
                fontSize: bodyStyle.fontSize,
                layout: [.Left, .Bottom],
                bounds: CGRect( origin: CGPoint(x:0, y:0), size: CGSize(width: 0, height: 0)),
                padding: CGPoint(x:8, y:0)
            )
        }
        
        let textBox = TextBox(
            text: (name.count > 0 ? name :  " ").replacingOccurrences(of: "\\n", with: "\n").trimmingCharacters(in: NSCharacterSet.whitespaces),
            textColor: style.textColor,
            fontSize: style.fontSize,
            layout: ( bodyTextBox == nil ) ? [.Center, .Middle] : [.Left, .Top],
            bounds: CGRect( origin: CGPoint(x:0, y:0), size: CGSize(width: 0, height: 0)),
            padding: CGPoint(x:8, y:8))

        
        let textBounds = textBox.getBounds()
        let bodyBounds = bodyTextBox?.getBounds()
        
        var width = max(20, textBounds.width, bodyBounds != nil ? bodyBounds!.width : 0)
        if let styleWidth = style.width, styleWidth != -1 {
            width = styleWidth //max(width, styleWidth)
        }
        
        var height = max(20, textBounds.height)
        if let styleHeight = style.height, styleHeight != -1 {
            height = styleHeight//max(height, styleHeight)
        }
        if bodyBounds != nil {
            height += bodyBounds!.height // TODO: Put spacing into some configurable area
        }
        
        let bounds = CGRect(x: e.x, y:e.y, width: width, height: height)
        textBox.setFrame(CGRect(x: 0, y:0, width: width, height: height))
        
        if let tb = bodyTextBox {
            let tbb = tb.getBounds()
            
            tb.setFrame(CGRect(x: 0, y:4, width: width, height: tbb.size.height))
            
            textBox.setFrame(CGRect(x: 0, y:tbb.height + 4, width: width, height: textBox.size.height ))
        }
        
        var box: DrawableContainer? = nil
        if let display = style.display {
            switch display {
            case "text":
                box = buildEmptyRect(bounds,  e, textBox, elementDrawable)
            case "no-fill":
                box = buildRoundRect(bounds, bgColor, borderColor, e, textBox, elementDrawable, fill: false)
            case "stack":
                box = buildRoundRect(bounds, bgColor, borderColor, e, textBox, elementDrawable, stack: 3)
            default:
                box = buildRoundRect(bounds, bgColor, borderColor, e, textBox, elementDrawable)
            }
        }
        else {
            box = buildRoundRect(bounds, bgColor, borderColor, e, textBox, elementDrawable)
        }
        if let parentBox = box, let bbox = bodyTextBox {
            parentBox.append(bbox)
        }
    }
    
    func buildElementScene( _ element: Element, _ darkMode: Bool)-> Drawable {
        let elementDrawable = DrawableContainer()
        
        self.sceneStyle = SceneStyle(darkMode)
        self.sceneStyle.parseStyle(element.properties)
        
        var links: [DiagramItem] = []
        
        buildItems(element.items, elementDrawable, &links)
        for e in links {
            if let data = e as? LinkItem {
                
                let linkStyle = self.sceneStyle.defaultLineStyle.copy()
                linkStyle.parseStyle(e.properties)
                var sr: CGRect = CGRect(x: 0, y: 0, width: 5, height: 5)
                var tr: CGRect = CGRect(x: 0, y: 5, width: 5, height: 5)
                
                if let src = data.source, let srr = (drawables[src]?.getBounds()) {
                    self.addLink( src, e )
                    sr = srr
                }
                if let dst = data.target, let trr = drawables[dst]?.getBounds() {
                    self.addLink( dst, e )
                    tr = trr
                }
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

public enum TextPosition {
    case Left
    case Right
    case Center  // Horizomtal
    case Top
    case Bottom
    case Middle // Vertical
}
public class TextBox: Drawable {
    var size: CGSize = CGSize(width: 0, height:0)
    var point: CGPoint = CGPoint(x:0, y:0)
    var textColor: NSColor
    var textFontAttributes: [NSAttributedString.Key:Any]
    let text:String
    var font: NSFont
    var textStyle: NSMutableParagraphStyle
    var layout: Set<TextPosition>
    var frame: CGRect
    var padding: CGPoint
    
    public init( text: String, textColor: CGColor, fontSize:CGFloat = 24, layout: Set<TextPosition>, bounds: CGRect, padding: CGPoint = CGPoint( x:4, y:4 ) ) {
        self.font = NSFont.systemFont(ofSize: fontSize)
        self.text = text
        self.layout = layout
        self.frame = bounds
        self.padding = padding

        self.textColor = NSColor(cgColor: textColor)!
        
        self.textStyle = NSMutableParagraphStyle.default.mutableCopy() as! NSMutableParagraphStyle
        textStyle.alignment = NSTextAlignment.center
        
        self.textFontAttributes = [
            NSAttributedString.Key.foregroundColor: self.textColor,
            NSAttributedString.Key.paragraphStyle: self.textStyle,
            NSAttributedString.Key.font: self.font
        ]
        let attrString = NSAttributedString(string: text, attributes: textFontAttributes)
        
        let fs = CTFramesetterCreateWithAttributedString(attrString)
        let frameSize = CTFramesetterSuggestFrameSizeWithConstraints(fs, CFRangeMake(0, attrString.length), nil, CGSize(width: 1000, height: 1000), nil)
        
        self.size = CGSize(width: frameSize.width + padding.x, height: frameSize.height + padding.y )
        if( self.size.width > self.frame.width || self.size.height > self.frame.height) {
            self.frame = CGRect(origin: self.frame.origin, size: self.size)
        }
    }
    
    public func setFrame(_ bounds: CGRect ) {
        self.frame = bounds
    }
    
    public func isVisible() -> Bool {
        return true
    }
    
    public func drawBox(context: CGContext, at point: CGPoint) {
        
    }
    
    public func draw(context: CGContext, at point: CGPoint) {
        let q: NSString = self.text as NSString
        
        let atp = CGPoint(x: point.x + self.point.x, y: point.y + self.point.y )
        q.draw(at: atp , withAttributes: textFontAttributes)
        
//        context.stroke(CGRect(origin: CGPoint(x: point.x + self.frame.origin.x, y: point.y + self.frame.origin.y), size: self.frame.size))
    }
    
    public func layout(_ parentBounds: CGRect, _ dirty: CGRect) {
        // Put in centre of bounds
        var px = self.layout.contains(.Center) ? (frame.width-size.width)/2 : 0
        var py = self.layout.contains(.Middle) ? (frame.height-size.height)/2 : 0
        
        if self.layout.contains(.Left) {
            px = 0;
        }
        if self.layout.contains(.Right) {
            px = frame.width - size.width;
        }
        if self.layout.contains(.Bottom) {
            py = 0;
        }
        if self.layout.contains(.Top) {
            py = frame.height - size.height;
        }
        
        //TODO: If we doesn't fit into parent bounds we need to do something with it.
        
        self.point = CGPoint(x: frame.origin.x + px + padding.x / 2 , y: frame.origin.y + py + padding.y / 2)
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
    
    func addLabel(_ label: String) {
        self.label = TextBox(text: label.replacingOccurrences(of: "\\n", with: "\n"),
                             textColor: self.style.borderColor,
                             fontSize: self.style.fontSize, layout: [.Middle, .Center],
                             bounds: CGRect(origin: control, size: CGSize(width:0, height:0)))
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
        context.setStrokeColor(self.style.color)
        context.setFillColor(self.style.color)
        
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
        
        if let lbl = self.label {
            let lblBounds = lbl.getBounds()
            minX = min( minX, lblBounds.minX)
            minY = min( minY, lblBounds.minY)
            maxX = max( maxX, lblBounds.maxX)
            maxY = max( maxY, lblBounds.maxY)
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



public class ImageBox: Drawable {
    var pos: CGPoint
    var size: CGSize
    var img: CGImage
    
    init( pos: CGPoint, size: CGSize, img: CGImage) {
        self.pos = pos
        self.size = size
        self.img = img
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
    public func getBounds() -> CGRect {
        return CGRect(origin: self.pos, size: self.size)
    }
    public func update() {
    }
}
