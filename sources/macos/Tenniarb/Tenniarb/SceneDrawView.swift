//
//  SceneDrawView.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 26/05/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//

import Cocoa
import CoreText
import SceneKit

class SceneDrawView: SCNView {
    let background = CGColor(red: 253/255, green: 246/255, blue: 227/255, alpha:1)
    var elementModel: Element?
    
    var activeElement: Element?
    
    var dragElement: Element?
    
    var x: CGFloat = 0
    var y: CGFloat = 0
    
    var trackingArea: NSTrackingArea? = nil
    
    var mouseDownState = false
    
    
    var nodes:[Element:SCNNode] = [:]
    var rnodes:[SCNNode:Element] = [:]
    
    override var mouseDownCanMoveWindow: Bool {
        get {
            return false
        }
    }
    
    
    public func setElementModel(_ elementModel: Element ) {
        self.elementModel = elementModel
        needsDisplay = true
        
        nodes.removeAll()
        
        let scene = SCNScene()
        
        // We need to go over parent diagrams also and draw far away
        
        var el = elementModel
        var zoff: CGFloat = 0.0
        while (true) {
            let allElements = getElements(element: el)
            for e in allElements {
                let text = makeText(value: e.name, dx: 0.0, dy:0.0, dz:0.1)
                
                let textBox = text.boundingBox
                
                let box = SCNBox(width: (textBox.max.x-textBox.min.x)/10 + 1, height: 0.7, length: 0.1, chamferRadius: 1)
                let node = SCNNode(geometry: box)
                node.position = SCNVector3(x: e.x/50, y:e.y/50, z: zoff)
                
                node.addChildNode(text)
                
                scene.rootNode.addChildNode(node)
                
                self.nodes[el] = node
                self.rnodes[node] = el
                
            }
            if( el.parent == nil) {
                break;
            }
            if el.parent is ElementModel {
                break;
            }
            el = el.parent!
            
            let plane = SCNPlane(width: 100.0, height: 100.0)
            plane.firstMaterial?.diffuse.contents = NSColor( red: 1.0, green: 1.0, blue: 1.0, alpha: 0.7)
            
            let planeNode = SCNNode(geometry: plane)
            planeNode.position = SCNVector3(x:0, y:0, z: zoff - 2.5)
            
            scene.rootNode.addChildNode(planeNode)
            
            zoff -= 5
            
            
        }
        
        scene.background.contents = background
        
        self.autoenablesDefaultLighting = true
        self.showsStatistics = true
        self.allowsCameraControl = true
        
        // 1
        let cameraNode = SCNNode()
        // 2
        cameraNode.camera = SCNCamera()
        // 3
        cameraNode.position = SCNVector3(x: 0, y: 0, z: 5)
        
        // 4
        scene.rootNode.addChildNode(cameraNode)
        
        self.scene = scene
        self.scene?.rootNode.camera = cameraNode.camera
        
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
    
    func makeText( value: String, dx: CGFloat, dy: CGFloat, dz: CGFloat ) -> SCNNode {
        let newText = SCNText(string: value, extrusionDepth: 0.1 )
        newText.flatness = 0.01
        newText.alignmentMode = kCAAlignmentCenter
        newText.font = NSFont.systemFont(ofSize: 3)
        newText.firstMaterial!.diffuse.contents = NSColor.black
        newText.firstMaterial!.specular.contents = NSColor.black
        
        let _textNode = SCNNode(geometry: newText)
        
        
        let box = _textNode.boundingBox
        _textNode.position = SCNVector3Make(dx-(box.max.x-box.min.x)/20, dy-(box.max.y - box.min.y)/10, dz)
        
        
        _textNode.scale = SCNVector3(0.10, 0.10, 0.10)
        
        return _textNode
    }
    
    public func findElement(el: Element, x: CGFloat, y: CGFloat) -> Element? {
        let p = CGPoint(x: x, y: y)
        
        let options: [ SCNHitTestOption: Any] = [
            .sortResults : true,
            .boundingBoxOnly : true
        ]
        
        let elements = getElements(element: el)
        for e in elements {
            if let nde = nodes[e] {
                let box = nde.boundingBox
                let min = projectPoint(box.min)
                let max = projectPoint(box.max)
                
                Swift.debugPrint(e.name, " == ", min.x, " ", min.y, " ", max.x, " ", max.y)
            }
        }
        
        let hitResults = self.hitTest(p, options: options)
        
        if (hitResults.count > 0){
            for result in hitResults {
                var nde:SCNNode? = result.node
                Swift.debugPrint("find node")
                while nde != nil {
                    let el = self.rnodes[nde!]
                    if el != nil {
                        nde?.geometry?.firstMaterial?.ambient.contents = NSColor(red:0.5, green:0.90, blue:0.0, alpha: 1.0)
                        Swift.debugPrint(el?.name)
                        return el
                    }
                    if nde?.parent == nil {
                        break
                    }
                    nde = nde?.parent
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
            let el = findElement(el: em, x: self.x, y: self.y)
            if( el != nil) {
                updateActive(el!)
                self.dragElement = el
            }
            else {
                activeElement = nil
            }
        }
        needsDisplay = true
    }
    
    func updateActive(_ el: Element) {
        if let active = activeElement {
            // Deactiavte previous
            
            if let nde = self.nodes[active] {
                nde.geometry?.firstMaterial?.ambient.contents = NSColor(red: 0.0, green: 0.0, blue:0.0, alpha: 1.0)
            }
        }
        
        if let nde = self.nodes[el] {
            nde.geometry?.firstMaterial?.ambient.contents = NSColor(red: 1.0, green: 1.0, blue:1.0, alpha: 1.0)
        }
        self.activeElement = el
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
    }
    
    func updateMousePosition(_ event: NSEvent) {
        
        let p = self.convert(event.locationInWindow, from: nil)
        self.x = p.x
        self.y = p.y
//        let bs = self.convert(frame, from: self)
//        let mloc = self.convert(self.window!.mouseLocationOutsideOfEventStream, to: self)
        self.x = p.x - frame.minX
        self.y = p.y - frame.minY
        Swift.debugPrint("x:", self.x, " y: ", self.y)
    }
    
    
    override func mouseMoved(with event: NSEvent) {
       // self.updateMousePosition(event)

//        if mouseDownState {
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
    
    func getElements(element:Element) -> [Element]  {
        var allElements:[Element] = []
        
        collectElements(el: element, elements: &allElements)
        return allElements
    }
    override func draw(_ dirtyRect: NSRect) {
        
        if( self.elementModel == nil) {
            return
        }
        
        let context = NSGraphicsContext.current?.cgContext
        
        context?.setFillColor(background)
        context?.fill(bounds)
        
        
        let allElements = getElements(element: self.elementModel!)
        
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
