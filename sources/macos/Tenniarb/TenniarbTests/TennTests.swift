//
//  LexerTests.swift
//  TenniarbTests
//
//  Created by Andrey Sobolev on 25/06/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//

import XCTest

@testable import Tenniarb

class LexerTests: XCTestCase {

    override func setUp() {
        super.setUp()
        // Put setup code here. This method is called before the invocation of each test method in the class.
    }
    
    override func tearDown() {
        // Put teardown code here. This method is called after the invocation of each test method in the class.
        super.tearDown()
    }

    func testBasicParsing() {
        // This is an example of a functional test case.
        // Use XCTAssert and related functions to verify your tests produce the correct results.
        
        let lexer = TennLexer("qwe asd")
        let t1 = lexer.getToken()
        
        XCTAssertEqual(t1?.literal, "qwe")
        XCTAssertEqual(t1?.type, TennTokenType.symbol)
        
        let t2 = lexer.getToken()
        
        XCTAssertEqual(t2?.literal, "asd")
        XCTAssertEqual(t2?.type, TennTokenType.symbol)
        
        let t3 = lexer.getToken()
        XCTAssertEqual(t3?.type, TennTokenType.eof)
        
        let t4 = lexer.getToken()
        XCTAssertNil(t4)
    }
    func testEmojiiLexing() {
        // This is an example of a functional test case.
        // Use XCTAssert and related functions to verify your tests produce the correct results.
        
        let lexer = TennLexer("qwe 😈")
        let t1 = lexer.getToken()
        
        XCTAssertEqual(t1?.literal, "qwe")
        XCTAssertEqual(t1?.type, TennTokenType.symbol)
        
        let t2 = lexer.getToken()
        
        XCTAssertEqual(t2?.literal, "😈")
        XCTAssertEqual(t2?.type, TennTokenType.symbol)
        
        let t3 = lexer.getToken()
        XCTAssertEqual(t3?.type, TennTokenType.eof)
        
        let t4 = lexer.getToken()
        XCTAssertNil(t4)
    }
}
