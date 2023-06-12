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
    private var code: Data
    
    private var it: Data.Iterator
    private var nextChar: Character?
    private var currentCharValue: Character
    
    public var errorHandler: ((_ error: LexerError, _ startPos:Int, _ pos: Int ) -> Void)?
    
    init( _ code: String) {
        self.code = code.data(using: String.Encoding.utf8)!
        self.it = self.code.makeIterator()
        
        if let cc = self.it.next() {
            self.currentCharValue = Character(UnicodeScalar(cc))
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
    
    @inlinable
    func detectSymbolType( pattern value: String ) -> TennTokenType {
        var skipFirst = false
        if !value.isEmpty && value.hasPrefix("-") {
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
                self.currentCharValue = Character(UnicodeScalar(cc))
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
            self.nextChar = Character(UnicodeScalar(ncc))
        }
        if let nc = self.nextChar {
            return nc
        }
        return "\0"
    }
    
    private func readString( lit: Character) {
        self.inc()
        
        var foundEnd = false
        var stPos = self.pos
        var r = ""
        while self.pos < self.bufferCount {
            if  currentCharValue == "\n" {
                self.currentLine += 1;
                self.currentChar = 0;
            }
            else if currentCharValue == lit {
                if stPos < self.pos {
                    guard let ss = String(bytes: self.code[stPos..<self.pos], encoding:String.Encoding.utf8) else {
                        if let h = self.errorHandler {
                            h(.UTF8Error, stPos, pos)
                        }
                        return
                    }
                    r.append(ss)
                }
                self.add(literal: r);
                r.removeAll()
                self.inc()
                stPos = self.pos
                foundEnd = true
                break
            }
            else if (currentCharValue == "\\" && self.next() == lit) {
                if stPos < self.pos {
                    guard let ss = String(bytes: self.code[stPos..<self.pos], encoding:String.Encoding.utf8) else {
                        if let h = self.errorHandler {
                            h(.UTF8Error, stPos, pos)
                        }
                        return
                    }
                    r.append(ss)
                }
                r.append(lit)
                self.inc()
                self.inc()
                stPos = self.pos
                continue
            }
            self.inc()
        }
        if stPos < self.pos {
            guard let ss = String(bytes: self.code[stPos..<self.pos], encoding:String.Encoding.utf8) else {
                if let h = self.errorHandler {
                    h(.UTF8Error, stPos, pos)
                }
                return
            }
            r.append(ss)
        }
        if !r.isEmpty {
            self.add(literal: r)
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
    
    @inline(__always)
    fileprivate func returnToken() -> TennToken? {
        return self.tokenBuffer.removeFirst()
    }
    
    public func getToken() -> TennToken? {
        if !self.tokenBuffer.isEmpty {
            return returnToken()
        }
        
        var r = ""
        var stPos = self.pos
        
        let appendFunc: () -> Bool = {
            if stPos < self.pos {
                guard let ss = String(bytes: self.code[stPos..<self.pos], encoding:String.Encoding.utf8) else {
                    if let h = self.errorHandler {
                        h(.UTF8Error, stPos, self.pos)
                    }
                    return false
                }
                stPos = self.pos
                r.append(ss)
            }
            return true
        }
        
        while self.pos < self.bufferCount {
            let cc = currentCharValue
            switch (cc) {
            case " ", "\t", "\r","\n":
                if stPos < self.pos {
                    if !appendFunc() {
                        return nil
                    }
                    self.add(check: r)
                    r.removeAll()
                }
                if self.processNewLine(cc) {
                    return returnToken()
                }
                stPos = self.pos
            case "{":
                if !appendFunc() {
                    return nil
                }
                if r.count > 0 {
                    self.add(check: r)
                    r.removeAll()
                }
                self.processCurlyOpen(cc)
                stPos = self.pos
            case "}":
                if !appendFunc() {
                    return nil
                }
                if r.count > 0 {
                    self.add(check: r)
                    r.removeAll()
                }
                if self.processCurlyClose(cc) {
                    return returnToken()
                }
                stPos = self.pos
            case ";":
                if !appendFunc() {
                    return nil
                }
                if r.count > 0 {
                    self.add(check: r)
                    r.removeAll()
                }
                self.add(type: .semiColon, literal: String(cc))
                self.inc()
                if  !self.tokenBuffer.isEmpty {
                    return returnToken()
                }
                stPos = self.pos
            case "/":
                if !appendFunc() {
                    return nil
                }
                if r.count > 0 {
                    self.add(check: r)
                    r.removeAll()
                }
                self.processComment(cc)
                stPos = self.pos
            case "%":
                let nc = self.next()
                if nc == "{" {
                    if !appendFunc() {
                        return nil
                    }
                    if r.count > 0 {
                        self.add(check: r)
                        r.removeAll()
                    }
                    readExpression(startLit: "{", endLit: "}", type: .markdownLit)
                    stPos = self.pos
                }
                else {
                    self.inc()
                }
                break;
            case "@":
                let nc = self.next()
                if nc == "(" {
                    if !appendFunc() {
                        return nil
                    }
                    if r.count > 0 {
                        self.add(check: r)
                        r.removeAll()
                    }
                    readImage(startLit: "(", endLit: ")", type: .imageData)
                    stPos = self.pos
                } else {
                    self.inc()
                }
                break;
            case "$":
                let nc = self.next()
                if nc == "(" {
                    if !appendFunc() {
                        return nil
                    }
                    if r.count > 0 {
                        self.add(check: r)
                        r.removeAll()
                    }
                    readExpression(startLit: "(", endLit: ")", type: .expression)
                    stPos = self.pos
                } else if nc == "{" {
                    if !appendFunc() {
                        return nil
                    }
                    if r.count > 0 {
                        self.add(check: r)
                        r.removeAll()
                    }
                    readExpression(startLit: "{", endLit: "}", type: .expressionBlock)
                    stPos = self.pos
                } else {
                    self.inc()
                }
                break;
            case "\'", "\"":
                if !appendFunc() {
                    return nil
                }
                if r.count > 0 {
                    self.add(check: r)
                    r.removeAll()
                }
                self.readString(lit: cc)
                if !self.tokenBuffer.isEmpty {
                    return returnToken()
                }
                stPos = self.pos
            default:
                self.inc()
            }
        }
        
        if !appendFunc() {
            return nil
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
            return returnToken()
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
        
        let stPos = self.pos
        var foundEnd = false
        var indent = 1
        let startLine = self.currentLine
        while self.pos < self.bufferCount {
            let curChar = currentCharValue
            if  curChar == "\n" {
                self.currentLine += 1;
                self.currentChar = 0;
            }
            else if curChar == startLit {
                indent += 1
            }
            else if curChar == endLit {
                indent -= 1
                if indent == 0 {
                    foundEnd = true
                    break
                }
            }
            self.inc()
        }
        
        guard let r = String(bytes: self.code[stPos..<self.pos], encoding:String.Encoding.utf8) else {
            if let h = self.errorHandler {
                h(.UTF8Error, stPos, pos)
            }
            return
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
            }
            self.inc()
        }
    }
    private func readImage( startLit: Character, endLit: Character, type: TennTokenType) {
        self.inc()
        self.inc()
        
        let stPos = self.pos
        var foundEnd = false
        let startLine = self.currentLine
        while self.pos < self.bufferCount {
            let curChar = currentCharValue
            if  curChar == "\n" {
                self.currentLine += 1;
                self.currentChar = 0;
            }
            else if curChar == endLit {
                foundEnd = true
                break
            }
            self.inc()
        }
        
        guard let r = String(bytes: self.code[stPos..<self.pos], encoding:String.Encoding.utf8) else {
            if let h = self.errorHandler {
                h(.UTF8Error, stPos, pos)
            }
            return
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
            }
            self.inc()
        }
    }
}
