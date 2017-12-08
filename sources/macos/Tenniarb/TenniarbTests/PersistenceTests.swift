//
//  TenniarbTests.swift
//  TenniarbTests
//
//  Created by Andrey Sobolev on 25/06/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//

import XCTest

@testable import Tenniarb

class PersistenceTests: XCTestCase {
    
    override func setUp() {
        super.setUp()
        // Put setup code here. This method is called before the invocation of each test method in the class.
    }
    
    override func tearDown() {
        // Put teardown code here. This method is called after the invocation of each test method in the class.
        super.tearDown()
    }
    
    func saveModelTest() {
        let ef = TestElementFactory.createModel()
        
        let storedValue = ef.toTennStr()
        Swift.print(storedValue)
        
        let parser = TennParser()
        let node = parser.parse(storedValue)
        
        XCTAssertTrue(!parser.errors.hasErrors())
        
        let model = ElementModel.parseTenn(node: node)
    }
    
    func testSaveEmptyModel() {
        let model = ElementModel()
        
        let storedValue = model.toTennStr()
        Swift.print(storedValue)
        XCTAssertEqual("", storedValue)
        
        let parser = TennParser()
        let node = parser.parse(storedValue)
        
        XCTAssertTrue(!parser.errors.hasErrors())
        
        let model2 = ElementModel.parseTenn(node: node)
        XCTAssertNotNil(model2)
        XCTAssertEqual(model.name, model2?.name)
    }
    func testSaveOneElement() {
        let model = ElementModel()
        
        _ = model.add(Element( name: "Item1"))
        
        let storedValue = model.toTennStr()
        XCTAssertEqual("element \"Item1\" {\n}", storedValue)
        Swift.print(storedValue)
        
        let parser = TennParser()
        let node = parser.parse(storedValue)
        
        XCTAssertTrue(!parser.errors.hasErrors())
        
        let model2 = ElementModel.parseTenn(node: node)
        XCTAssertNotNil(model2)
        XCTAssertEqual(model.name, model2?.name)
    }
    func testSaveOneElementOneItem() {
        let model = ElementModel()
        
        
        let diagram = Element(name: "Diagram 1")
        _ = model.add(makeItem: diagram )
        
        _ = diagram.add(DiagramItem(kind: .Item, name: "Demo element 1" ))
        _ = diagram.add(DiagramItem(kind: .Item, name: "Demo element 2" ))
        
        let storedValue = model.toTennStr()
        XCTAssertEqual("element \"Diagram 1\" {\n  item \"Demo element 1\"\n  item \"Demo element 2\"\n}", storedValue)
        Swift.print(storedValue)
        
        let parser = TennParser()
        let node = parser.parse(storedValue)
        
        XCTAssertTrue(!parser.errors.hasErrors())
        
        let model2 = ElementModel.parseTenn(node: node)
        XCTAssertNotNil(model2)
        XCTAssertEqual(model.name, model2?.name)
        
        XCTAssertEqual(model2!.count, 1)
        
        let diagramL = model2!.elements[0]
        XCTAssertEqual(diagramL.itemCount, 2)
        let item0 = diagramL.items[0]
        let item1 = diagramL.items[0]
        XCTAssertEqual("Demo element 1", item0.name )
        
    }
    
}

