//
//  LexerTests.swift
//  TenniarbTests
//
//  Created by Andrey Sobolev on 25/06/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//
//  Licensed under the Eclipse Public License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License. You may
//  obtain a copy of the License at https://www.eclipse.org/legal/epl-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//
//  See the License for the specific language governing permissions and
//  limitations under the License.

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
