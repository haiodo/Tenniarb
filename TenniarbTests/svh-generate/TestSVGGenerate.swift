//
//  TestSVGGenerate.swift
//  TenniarbTests
//
//  Created by Andrey Sobolev on 21.07.2020.
//  Copyright © 2020 Andrey Sobolev. All rights reserved.
//

import XCTest

@testable import Tenniarb

class SVGExport: XCTestCase {
    var element: Element!
    func loadTestDocument() -> Element {
        do {
            let testBundle = Bundle(for: type(of: self ))
            Swift.debugPrint(testBundle.bundleURL.path )
            let url = URL(fileURLWithPath: testBundle.bundleURL.path + "/Contents/Resources/Example.tenn")
            Swift.debugPrint(url)
            let storedValue = try String(contentsOf: url, encoding: String.Encoding.utf8)
            
            let now = Date()
            
            let parser = TennParser()
            let node = parser.parse(storedValue)
            
            if parser.errors.hasErrors() {
                return Element(name: "Failed element")
            }
            
            let elementModel = ElementModel.parseTenn(node: node)
            
            Swift.debugPrint("Elapsed parse \(Date().timeIntervalSince(now))")
            return elementModel
        }
        catch {
            return Element(name: "Failed element")
        }
    }
    
    override func setUp() {
        super.setUp()
        self.element = loadTestDocument()
    }
    
    func testBasicSVGExport() {
        let elem = element.elements[0].elements[0]
        Swift.debugPrint("Rendering: \(elem.name)")
        
        let exec = ExecutionContext()
        exec.setElement(elem)
        
        let scene = DrawableScene(elem, darkMode: false, executionContext: exec)
        let bounds = scene.getBounds()
        let ox = CGFloat(15)
        let oy = CGFloat(15)
        
        let scaleFactor: CGFloat = 1
            
        let imgBounds = bounds.insetBy(dx: CGFloat((-1 * ox) * 2), dy: CGFloat((-1 * oy) * 2))
        
        scene.offset = CGPoint(x: ox*scaleFactor + CGFloat(-1 * bounds.origin.x), y: oy*scaleFactor + CGFloat(-1 * bounds.origin.y))
        scene.layout(bounds, bounds)
        
        var svgData = """
        <svg style="width:\(imgBounds.width)px; height:\(imgBounds.height)px; border: solid 1px" viewBox="\(imgBounds.minX) \(imgBounds.minY) \(imgBounds.maxX) \(imgBounds.maxY)" version="1.1" preserveAspectRatio="xMidYMid meet" xmlns="http://www.w3.org/2000/svg">\n
        """
        
        svgData += scene.render(type: .svg)
        
        svgData += "\n</svg>"
        Swift.print(svgData)
    }
}
