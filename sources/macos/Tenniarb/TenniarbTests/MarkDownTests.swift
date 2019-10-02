//
//  MarkDownTests.swift
//  TenniarbTests
//
//  Created by Andrey Sobolev on 20.09.2019.
//  Copyright © 2019 Andrey Sobolev. All rights reserved.
//

import Foundation

import XCTest

@testable import Tenniarb

class MarkdownTests: XCTestCase {
    
    override func setUp() {
        super.setUp()
        // Put setup code here. This method is called before the invocation of each test method in the class.
    }
    
    override func tearDown() {
        // Put teardown code here. This method is called after the invocation of each test method in the class.
        super.tearDown()
    }
    
    func testBoldParsing() {
        let tokens = MarkdownLexer.getTokens(code: "*Display* queries/paths\nin *Responses*\nRM-13104")
        XCTAssert(tokens.count == 3)
    }
    
    func testBoldParsing2() {
           let tokens = MarkdownLexer.getTokens(code: """
            * 1. *Re-connect* local NSM only(if pod are same)
            * 1.1 Modify NSMD(1/2) stored connection info
            * 1.2 do Request() on local Dataplane
            * 1.3 return connection to NSC/NSE.
            * - Dataplane/NSMD1 is potential fail points here.
            * 2. Cleanup and configure new connection.
            * 2.1 NSMD1 do *Close()* on local Dataplane.
            * 2.2 NSMD1 do Close() on remote NSMD2
            * 2.3 NDMS2 do Close() on local DataPlane
            * 2.4 NSMD2 do Close() on local NSE
            * 2.5 do "Connection" with all steps again.
            """)
           XCTAssert(tokens.count == 3)
       }
    
    
    
    func testBasicParsing() {
        let lexer = MarkdownLexer(
            """
        *box* text
        # title A

        Regular text *bold* line _italic_ line.

        @(my_image|640)

        """)
        
        var tokens:[MarkdownToken] = []
                
        while true {
            guard let t = lexer.getToken() else {
                break
            }
            tokens.append(t)
        }
        
        
        Swift.debugPrint("qwe")
        
//        XCTAssert(tokens.count == 6, <#T##message: String##String#>)
        
    }
}

