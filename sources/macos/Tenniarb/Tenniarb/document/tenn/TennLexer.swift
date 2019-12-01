//
//  TennLexer.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 21/06/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//

import Foundation

public class TennLexer: TennLexerProtocol {
    private var currentLine: Int = 0
    private var currentChar: Int = 0
    private var bufferCount: Int = 0
    private var pos: Int = 0
    
    private var tokenBuffer: [TennToken] = []
    private var blockState:[TennTokenType] = []
    private var code: String
    
    private var it: String.Iterator
    private var nextChar: Character?
    private var currentCharValue: Character
    
    public var errorHandler: ((_ error: LexerError, _ startPos:Int, _ pos: Int ) -> Void)?
    
    init( _ code: String) {
        self.code = code
        self.it = code.makeIterator()
        
        if let cc = self.it.next() {
            self.currentCharValue = cc
        } else {
            self.currentCharValue = "\0"
        }
        self.bufferCount = self.code.count
    }
    
    public func revert(tok: TennToken) {
        tokenBuffer.insert(tok, at: 0)
    }
    private func add(type: TennTokenType, literal: String) {
        let c = literal.count
        self.tokenBuffer.append(
            TennToken(type: type, literal: literal, line: currentLine, col: currentChar, pos: self.pos-c, size: c)
        )
    }
    
    private func add(check pattern: String) {
        if !pattern.isEmpty {
            self.add(type: detectSymbolType(pattern: pattern), literal: pattern)
        }
        
    }
    private func add(literal: String) {
        let c = literal.count
        self.tokenBuffer.append(
            TennToken(type: .stringLit, literal: literal, line: currentLine, col: currentChar, pos: self.pos-c, size: c)
        )
    }
    private func detectSymbolType( pattern value: String ) -> TennTokenType {
        var skipFirst = false
        if !value.isEmpty && value[value.startIndex] == "-" {
            skipFirst = true
        }
        var dot = false
        var i = 0
        for c in value {
            if skipFirst {
                skipFirst = false
                continue
            }
            if c == "." {
                if i == 0  || dot {
                    return .symbol
                }
                dot = true
                continue
            }
            if !CharacterSet.decimalDigits.contains(c.unicodeScalars.first!) {
                return .symbol
            }
            i += 1
        }
        if dot {
            return .floatLit
        }
        return TennTokenType.intLit
    }
    
    private func inc() {
        self.currentChar += 1
        self.pos += 1
        if let nc = self.nextChar {
            self.currentCharValue = nc
            self.nextChar = nil
        } else {
            if let cc = self.it.next() {
                self.currentCharValue = cc
            } else {
                self.currentCharValue = "\0"
            }
        }
    }
    private func next() -> Character {
        if let nc = self.nextChar {
            return nc
        }
        if let ncc = self.it.next() {
            self.nextChar = ncc
        }
        if let nc = self.nextChar {
            return nc
        }
        return "\0"
    }
    
    private func readString( lit: Character) {
        self.inc()
        
        var r = ""
        var foundEnd = false
        let stPos = self.pos
        while self.pos < self.bufferCount {
            if  currentCharValue == "\n" {
                self.currentLine += 1;
                self.currentChar = 0;
                r.append(currentCharValue)
            }
            else if currentCharValue == lit {
                self.add(literal: r);
                r.removeAll()//keepingCapacity: true)
                self.inc()
                foundEnd = true
                break
            }
            else if (currentCharValue == "\\" && self.next() == lit) {
                r.append(self.next())
                self.inc()
            } else {
                r.append(currentCharValue)
            }
            self.inc()
        }
        if !r.isEmpty {
            self.add(literal: r)
            r.removeAll()//keepingCapacity: true)
        }
        if !foundEnd {
            if let h = self.errorHandler {
                h(.EndOfLineReadString, stPos, pos)
            }
        }
    }
    private func skipCComment() {
        self.inc(); // Skip \/*
        self.inc(); // Skip \/*
    
        while self.pos < self.bufferCount {
            switch currentCharValue {
            case "\n":
                self.currentLine += 1
                self.currentChar = 0
                break
            case "*":
                if self.next() == "/" {
                    // end of comment
                    self.inc()
                    self.inc()
                    return
                }
            default:
                // No Actions required
                break
            }
            self.inc()
        }
    }

    
    private func processComment( _ cc: Character) {
        if self.next() == "*" {
            // C/C++ multi-line comment
            self.skipCComment()
        } else if self.next() == "/" {
            // End of line comment
            while self.pos < self.bufferCount {
                if currentCharValue == "\n" {
                    self.currentLine += 1
                    self.currentChar = 0
                    break
                }
                self.inc()
            }
        } else {
            self.inc()
        }
    }
    
    private func processNewLine(_ cc: Character) -> Bool {
        self.inc()
        if cc == "\n" {
            self.currentLine += 1
            self.currentChar = 0
            // Check if we need to send a delimiter semicolon symbol.
            if self.blockState.isEmpty || self.blockState[0] == .curlyLe {
                self.tokenBuffer.append(
                    TennToken(type: .semiColon, literal:"\n", line: currentLine, col: currentChar, pos: self.pos-1, size: 1)
                )
            }
        }
        if !self.tokenBuffer.isEmpty {
            return true
        }
        return false

    }
    
    public func getToken() -> TennToken? {
        if !self.tokenBuffer.isEmpty {
            return self.tokenBuffer.removeFirst()
        }
        
        var r = ""
        while self.pos < self.bufferCount {
            let cc = currentCharValue
            switch (cc) {
            case " ", "\t", "\r","\n":
                if r.count > 0 {
                    self.add(check: r)
                    r.removeAll()
                }
                if self.processNewLine(cc) {
                    return self.tokenBuffer.removeFirst()
                }
            case "{":
                if r.count > 0 {
                    self.add(check: r)
                    r.removeAll()
                }
                self.processCurlyOpen(cc)
            case "}":
                if r.count > 0 {
                    self.add(check: r)
                    r.removeAll()
                }
                if self.processCurlyClose(cc) {
                    return self.tokenBuffer.removeFirst()
                }
            case ";":
                if r.count > 0 {
                    self.add(check: r)
                    r.removeAll()
                }
                self.add(type: .semiColon, literal: String(cc))
                self.inc()
                if  !self.tokenBuffer.isEmpty {
                    return self.tokenBuffer.removeFirst()
                }
            case "/":
                if r.count > 0 {
                    self.add(check: r)
                    r.removeAll()
                }
                self.processComment(cc)
            case "%":
                let nc = self.next()
                if nc == "{" {
                    if r.count > 0 {
                        self.add(check: r)
                        r.removeAll()
                    }
                    readExpression(startLit: "{", endLit: "}", type: .markdownLit)
                }
                else {
                    r.append(cc)
                    self.inc()
                }
                break;
            case "@":
                let nc = self.next()
                if nc == "(" {
                    if r.count > 0 {
                        self.add(check: r)
                        r.removeAll()
                    }
                    readExpression(startLit: "(", endLit: ")", type: .imageData)
                } else {
                    r.append(cc)
                    self.inc()
                }
                break;
            case "$":
                let nc = self.next()
                if nc == "(" {
                    if r.count > 0 {
                        self.add(check: r)
                        r.removeAll()
                    }
                    readExpression(startLit: "(", endLit: ")", type: .expression)
                } else if nc == "{" {
                    if r.count > 0 {
                        self.add(check: r)
                        r.removeAll()
                    }
                    readExpression(startLit: "{", endLit: "}", type: .expressionBlock)
                } else {
                    r.append(cc)
                    self.inc()
                }
                break;
            case "\'", "\"":
                if r.count > 0 {
                    self.add(check: r)
                    r.removeAll()
                }
                self.readString(lit: cc)
                if !self.tokenBuffer.isEmpty {
                    return self.tokenBuffer.removeFirst()
                }
            default:
                r.append(cc)
                self.inc()
            }
        }
        
        if r.count > 0 {
            self.add(check: r)
            r.removeAll()
        }
        
        if self.pos == self.bufferCount {
            self.add(type: .eof, literal: "\0")
            self.inc()
        }
        
        if !self.tokenBuffer.isEmpty {
            return self.tokenBuffer.removeFirst()
        } else {
            return nil
        }
    }
    
    private func processCurlyOpen(_ cc: Character) {
        self.inc()
        self.tokenBuffer.append(
            TennToken(type: .curlyLe, literal: String(cc), line: currentLine, col: currentChar, pos: self.pos-1, size: 1)
        )
        
        self.blockState.insert(.curlyLe, at: 0)
    }
    private func processCurlyClose(_ cc: Character)-> Bool {
        self.inc()
        self.tokenBuffer.append(
            TennToken(type: .curlyRi, literal: String(cc), line: currentLine, col: currentChar, pos: self.pos-1, size: 1)
        )
        
        if self.blockState.isEmpty {
            Swift.debugPrint("Invalid open close tokens expected .curlyLe but found \(cc)")
            return false
        }
        let openToken = self.blockState.removeFirst()
        // Check for token crossing here.
        if openToken != .curlyLe {
            Swift.debugPrint("Invalid open close tokens expected .curlyLe but found \(openToken)")
        }
        
        if !self.tokenBuffer.isEmpty {
            return true
        }
        return false
    }
    
    private func readExpression( startLit: Character, endLit: Character, type: TennTokenType) {
        self.inc()
        self.inc()
        
        var r = ""
        let stPos = self.pos
        var foundEnd = false
        var indent = 1
        let startLine = self.currentLine
        while self.pos < self.bufferCount {
            let curChar = currentCharValue
            if  curChar == "\n" {
                self.currentLine += 1;
                self.currentChar = 0;
                r.append(curChar)
            }
            else if curChar == startLit {
                indent += 1
                r.append(curChar)
            }
            else if curChar == endLit {
                indent -= 1
                if indent == 0 {
                    foundEnd = true
                    break
                }
                else {
                    r.append(curChar)
                }
            }
            else {
                r.append(curChar)
            }
            self.inc()
        }
                
        if !foundEnd {
            if let h = self.errorHandler {
                h(.EndOfExpressionReadError, stPos, pos)
            }
        }
        else {
            if !r.isEmpty {
                let c = r.count
                self.tokenBuffer.append(
                    TennToken(type: type, literal: String(r), line: startLine, col: currentChar, pos: self.pos-c, size: c)
                )
                r.removeAll()//keepingCapacity: true)
            }
            self.inc()
        }
    }
}
