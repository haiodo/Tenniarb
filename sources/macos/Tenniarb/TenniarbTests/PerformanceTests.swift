//
//  PerformanceTests.swift
//  TenniarbTests
//
//  Created by Andrey Sobolev on 21.08.2019.
//  Copyright © 2019 Andrey Sobolev. All rights reserved.
//

import Foundation
import Cocoa
import XCTest

@testable import Tenniarb

class PerformanceTests: XCTestCase {
    
    override func setUp() {
        super.setUp()
        // Put setup code here. This method is called before the invocation of each test method in the class.
    }
    
    override func tearDown() {
        // Put teardown code here. This method is called after the invocation of each test method in the class.
        super.tearDown()
    }
    
    func randomString(length: Int) -> String {
        let letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
        let randomCharacters = (0..<length).map{_ in letters.randomElement()!}
        return String(randomCharacters)
    }
    
    func testLexerParsingPerformance() {
        let nde = TennNode(kind: .Statements)
        
        var now = Date()
        let value = randomString(length: 1024000)
        for i in 0...10 {
            nde.add(TennNode.newCommand("cmd_\(i)", TennNode.newStrNode("String value: \(i) \(value)")))
            nde.add(TennNode.newCommand("cmd_\(i)_float", TennNode.newFloatNode(Date().timeIntervalSinceNow)))
            nde.add(TennNode.newCommand("cmd_\(i)_int", TennNode.newIntNode(i)))
            nde.add(TennNode.newCommand("cmd_\(i)_int", TennNode.newMarkdownNode(value)))
        }
        Swift.debugPrint("Elapsed generate \(Date().timeIntervalSince(now))")
        
        now = Date()
        let str = nde.toStr()
        Swift.debugPrint("Elapsed toStr \(Date().timeIntervalSince(now))")
        
        Swift.debugPrint("Len of generated string \(str.count)")
        
        now = Date()
        _ = TennParser().parse(str)
        Swift.debugPrint("Elapsed parse \(Date().timeIntervalSince(now))")
        
//        now = Date()
//        let p = TennParser()
//        p.factory = { source in FastTennLexer( source )}
//        _ = p.parse(str)
//        Swift.debugPrint("Elapsed parse \(Date().timeIntervalSince(now))")
        
    }   
}
