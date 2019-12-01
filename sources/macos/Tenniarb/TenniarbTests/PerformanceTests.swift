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
    
    func doTestIterateOverString() -> String {
        let str: String = generateString()
        let now = Date()
        var count: Int = 0
        for c in str {
            if c == "A" {
                count += 1
            }
        }
        return "---------- \(#function) ------- Elapsed: \(Date().timeIntervalSince(now))"
    }
    
    func doTestIterateOverStringIndex() -> String {
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
        return "-------- \(#function) ------ Elapsed: \(Date().timeIntervalSince(now))"
    }
    
    func doTestIterateOverStringArray() -> String {
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
        return "--------- \(#function) ---------- Elapsed: \(Date().timeIntervalSince(now))"
    }
    
    func doTestIterateOverStringPrimitiveArray() -> String {
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
        return "--------- \(#function) ---------- Elapsed: \(Date().timeIntervalSince(now))"
    }
    func doTestIterateOverStringPrimitiveArrayIndex() -> String {
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
        return "--------- \(#function) ---------- Elapsed: \(Date().timeIntervalSince(now))"
    }
    func doTestIterateOverStringPrimitiveScalarArrayIndex() -> String {
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
        return "--------- \(#function) ---------- Elapsed: \(Date().timeIntervalSince(now))"
    }
    func doTestIterateOverStringArrayIndex() -> String {
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
        return "--------- \(#function) ---------- Elapsed: \(Date().timeIntervalSince(now))"
    }
    
    func doTestIterateOverUnicodeScalarsArray() -> String {
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
        return "--------- \(#function) -------- Elapsed: \(Date().timeIntervalSince(now))"
    }
    func doTestIterateOverUnicodeScalarsArrayIndex() -> String {
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
        return "--------- \(#function) ------- Elapsed: \(Date().timeIntervalSince(now))"
    }
    
    func doTestIterateOverUnicodeScalars() -> String {
        let str: String = generateString()
        let now = Date()
        var count = 0
        
        let i = str.unicodeScalars.makeIterator()
        
        for cc in str.unicodeScalars {
            let c = Character(cc)
            if c == "A" {
                count += 1
            }
        }
        return "--------- \(#function) ------- Elapsed: \(Date().timeIntervalSince(now))"
    }
    
    func testIterateOverUnicodeScalarsIterator(){
        let str: String = generateString()
        let now = Date()
        var count = 0
        
        var i = str.unicodeScalars.makeIterator()
        
        while true {
            guard let sc = i.next() else {
                break
            }
            let c = Character(sc)
            if c == "A" {
                count += 1
            }
        }
        Swift.print("--------- \(#function) ------- Elapsed: \(Date().timeIntervalSince(now))")
    }
    
    func doTestIterateOverUnicodeScalarsIndex() -> String {
        let str: String = generateString()
        let now = Date()
        var count = 0
        let sc = str.unicodeScalars
        var idx = sc.index(sc.startIndex, offsetBy: 0)
        let strLen = sc.count
        for _ in 0..<strLen {
            let c = Character(sc[idx])
            if c == "A" {
                count += 1
            }
            idx = sc.index(after: idx)
        }
        return "--------- \(#function) ------- Elapsed: \(Date().timeIntervalSince(now))"
    }
    
    func doTestGenerateTokenArray() -> String {
        let str: String = generateString()
        let now = Date()
        var result:[Character] = []
        result.reserveCapacity(1024)
        for c in str {
            result.append(c)
        }
        let ss = String(result)
        if str.count != ss.count {
            Swift.print("Error")
        }
        return "--------- \(#function) ------- Elapsed: \(Date().timeIntervalSince(now))"
    }
    func doTestGenerateTokenString() -> String {
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
        return "--------- \(#function) ------- Elapsed: \(Date().timeIntervalSince(now))"
    }
    
    func testLexerParsingPerformance() {
        let str: String = generateString()
        measure {
            let now = Date()
            _ = TennParser().parse(str)
            Swift.print("\(#function) Elapsed: \(Date().timeIntervalSince(now))")
        }
    }
    
    func _testPerformance() {
        var values: [String] = []
        values.append(doTestIterateOverString())
        values.append(doTestGenerateTokenArray())
        values.append(doTestGenerateTokenString())
        values.append(doTestIterateOverStringArray())
        values.append(doTestIterateOverStringIndex())
        values.append(doTestIterateOverUnicodeScalars())
        values.append(doTestIterateOverStringArrayIndex())
        values.append(doTestIterateOverUnicodeScalarsArray())
        values.append(doTestIterateOverUnicodeScalarsIndex())
        values.append(doTestIterateOverStringPrimitiveArray())
        values.append(doTestIterateOverUnicodeScalarsArrayIndex())
        values.append(doTestIterateOverStringPrimitiveArrayIndex())
        values.append(doTestIterateOverStringPrimitiveScalarArrayIndex())
        Swift.print("Results: \n \(values.joined(separator: "\n"))" )
    }
    
    let letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
    func testStringBuilding() {
        measure {
            var s = ""
            s.makeContiguousUTF8()
                        
            let randomChar = letters.randomElement()!
            
            for _ in 0..<1000000 {
                s.append(randomChar)
            }
            
            let ss = s as String
            var cc = 0
            for _ in ss {
                cc += 1
            }
        }
    }
    func testStringBuildViaArray() {
        measure {
            var s = Array<Character>()
            
            let randomChar = letters.randomElement()!
            
            for _ in 0..<1000000 {
                s.append(randomChar)
            }
            let ss = String(s)
            
            var cc = 0
            for _ in ss {
                cc += 1
            }
            
        }
    }
    func testStringBuildViaContiguousArray() {
        measure {
            var s = ContiguousArray<Character>()
            
            let randomChar = letters.randomElement()!
            
            for _ in 0..<1000000 {
                s.append(randomChar)
            }
            let ss = String(s)
            
            var cc = 0
            for _ in ss {
                cc += 1
            }
            
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
