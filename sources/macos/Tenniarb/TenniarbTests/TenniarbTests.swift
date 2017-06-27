//
//  TenniarbTests.swift
//  TenniarbTests
//
//  Created by Andrey Sobolev on 25/06/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//

import XCTest

@testable import Tenniarb

class TenniarbTests: XCTestCase {
    
    override func setUp() {
        super.setUp()
        // Put setup code here. This method is called before the invocation of each test method in the class.
    }
    
    override func tearDown() {
        // Put teardown code here. This method is called after the invocation of each test method in the class.
        super.tearDown()
    }
    
    func testParsing() {
        let parser = TennParser()
        let nde = parser.parse("""
            map qwe {
                elements {
                    node "Platform" {
                    }
                    node ""
                }
            }
            """)
        let asText:String = nde.toStr()
        let expected = """
            map qwe {
              elements {
                node "Platform" {
                }
                node ""
              }
            }
            """
        do {
            try asText.write(toFile: "/tmp/f1.txt", atomically: true, encoding: String.Encoding.utf8)
            try expected.write(toFile: "/tmp/f2.txt", atomically: true, encoding: String.Encoding.utf8)
        }
        catch {
            
        }
        XCTAssertEqual(asText, expected )
    }
    
    
}
