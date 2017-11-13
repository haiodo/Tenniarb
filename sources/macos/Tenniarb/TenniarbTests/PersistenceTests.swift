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
    
}

