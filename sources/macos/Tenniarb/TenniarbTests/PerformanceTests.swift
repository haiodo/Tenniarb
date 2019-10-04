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
    
    func generateString() -> String {
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
        return str
    }
    
    func _testIterateOverString() {
        let str: String = generateString()
        let now = Date()
        var count: Int = 0
        for c in str {
            if c == "A" {
                count += 1
            }
        }
        Swift.debugPrint("---------- \(#function) ------- Elapsed: \(Date().timeIntervalSince(now))")
    }
    
    func _testIterateOverStringIndex() {
        let str: String = generateString()
        let now = Date()
        var count: Int = 0
        let strLen = str.count
        let st = str.startIndex
        
        var idx = str.index(st, offsetBy: 0)
        for _ in 0..<strLen {
            let c = str[idx]
            if c == "A" {
                count += 1
            }
            idx = str.index(after: idx)
        }
        Swift.debugPrint("-------- \(#function) ------ Elapsed: \(Date().timeIntervalSince(now))")
    }
    
    func _testIterateOverStringArray() {
        let str: String = generateString()
        let now = Date()
        var count = 0
        let arr = Array(str)
        Swift.debugPrint("Build array Elapsed: \(Date().timeIntervalSince(now))")
        for c in arr {
            if c == "A" {
                count += 1
            }
        }
        Swift.debugPrint("--------- \(#function) ---------- Elapsed: \(Date().timeIntervalSince(now))")
    }
    
    func _testIterateOverStringPrimitiveArray() {
        let str: String = generateString()
        let now = Date()
        var count = 0
        var arr: [Character] = []
        arr.reserveCapacity(str.count)
        for c in str {
            arr.append(c)
        }
        
        Swift.debugPrint("Build array Elapsed: \(Date().timeIntervalSince(now))")
        for c in arr {
            if c == "A" {
                count += 1
            }
        }
        Swift.debugPrint("--------- \(#function) ---------- Elapsed: \(Date().timeIntervalSince(now))")
    }
    func _testIterateOverStringPrimitiveArrayIndex() {
        let str: String = generateString()
        let now = Date()
        var count = 0
        var arr: [Character] = []
        arr.reserveCapacity(str.count)
        for c in str {
            arr.append(c)
        }
        let strLen = str.count
        Swift.debugPrint("Build array Elapsed: \(Date().timeIntervalSince(now))")
        for i in 0..<strLen {
            let c = arr[i]
            if c == "A" {
                count += 1
            }
        }
        Swift.debugPrint("--------- \(#function) ---------- Elapsed: \(Date().timeIntervalSince(now))")
    }
    func _testIterateOverStringPrimitiveScalarArrayIndex() {
        let str: String = generateString()
        let now = Date()
        var count = 0
        var arr: [UnicodeScalar] = []
        arr.reserveCapacity(str.count)
        for c in str.unicodeScalars {
            arr.append(c)
        }
        let strLen = str.count
        Swift.debugPrint("Build array Elapsed: \(Date().timeIntervalSince(now))")
        for i in 0..<strLen {
            let c = Character(arr[i])
            if c == "A" {
                count += 1
            }
        }
        Swift.debugPrint("--------- \(#function) ---------- Elapsed: \(Date().timeIntervalSince(now))")
    }
    func _testIterateOverStringArrayIndex() {
        let str: String = generateString()
        let now = Date()
        var count = 0
        let arr = Array(str)
        let strLen = str.count
        Swift.debugPrint("Build array Elapsed: \(Date().timeIntervalSince(now))")
        for i in 0..<strLen {
            let c = arr[i]
            if c == "A" {
                count += 1
            }
        }
        Swift.debugPrint("--------- \(#function) ---------- Elapsed: \(Date().timeIntervalSince(now))")
    }
    
    func _testIterateOverUnicodeScalarsArray() {
        let str: String = generateString()
        let now = Date()
        var count = 0
        let arr = Array(str.unicodeScalars)
        Swift.debugPrint("Build scalars array Elapsed: \(Date().timeIntervalSince(now))")
        for c in arr {
            if c == "A" {
                count += 1
            }
        }
        Swift.debugPrint("--------- \(#function) -------- Elapsed: \(Date().timeIntervalSince(now))")
    }
    func _testIterateOverUnicodeScalarsArrayIndex() {
        let str: String = generateString()
        let now = Date()
        var count = 0
        let arr = Array(str.unicodeScalars)
        let strLen = str.count
        Swift.debugPrint("Build scalars array Elapsed: \(Date().timeIntervalSince(now))")
        for i in 0..<strLen {
            let c = Character(arr[i])
            if c == "A" {
                count += 1
            }
        }
        Swift.debugPrint("--------- \(#function) ------- Elapsed: \(Date().timeIntervalSince(now))")
    }
    
    func _testIterateOverUnicodeScalars() {
        let str: String = generateString()
        let now = Date()
        var count = 0
        for cc in str.unicodeScalars {
            let c = Character(cc)
            if c == "A" {
                count += 1
            }
        }
        Swift.debugPrint("--------- \(#function) ------- Elapsed: \(Date().timeIntervalSince(now))")
    }
    func _testIterateOverUnicodeScalarsIndex() {
        let str: String = generateString()
        let now = Date()
        var count = 0
        let sc = str.unicodeScalars
        var idx = sc.index(sc.startIndex, offsetBy: 0)
        let strLen = sc.count
        for i in 0..<strLen {
            let c = Character(sc[idx])
            if c == "A" {
                count += 1
            }
            idx = sc.index(after: idx)
        }
        Swift.debugPrint("--------- \(#function) ------- Elapsed: \(Date().timeIntervalSince(now))")
    }
    
    func _testGenerateTokenArray() {
        let str: String = generateString()
        let now = Date()
        var count = 0
        var result:[Character] = []
        result.reserveCapacity(1024)
        for c in str {
            result.append(c)
        }
        let ss = String(result)
        if str.count != ss.count {
            Swift.print("Error")
        }
        Swift.debugPrint("--------- \(#function) ------- Elapsed: \(Date().timeIntervalSince(now))")
    }
    func _testGenerateTokenString() {
        let str: String = generateString()
        let now = Date()
        var result = ""
        result.reserveCapacity(1024)
        for c in str {
            result.append(c)
        }
        if str.count != result.count {
            Swift.print("Error")
        }
        Swift.debugPrint("--------- \(#function) ------- Elapsed: \(Date().timeIntervalSince(now))")
    }
    
    func testLexerParsingPerformance() {
        for _ in 0..<1 {
            let str: String = generateString()
            let now = Date()
            _ = TennParser().parse(str)
            Swift.debugPrint("Elapsed: \(Date().timeIntervalSince(now))")
        }
    }   
}


/*
 string:                    1.0s
 stringIndex:               1.0s
 stringArray:               7.7s
 stringArrayIndex:          1.8s
 unicodeScallarsArray:      6.3s
 unicodeScallarsArrayIndex: 0.9s
 unicodeScallars:           0.5s
 unicodeScallarsIndex:      0.7s
 */
