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
    private var buffer: [Character]
    private var bufferCount: Int = 0
    private var pos: Int = 0
    
    private var tokenBuffer: [TennToken] = []
    private var blockState:[TennTokenType] = []
    
    public var errorHandler: ((_ error: LexerError, _ startPos:Int, _ pos: Int ) -> Void)?
    
    init( _ code: String) {
        self.buffer = code.map {c in c}
        self.bufferCount = self.buffer.count
    }
    
    public func revert(tok: TennToken) {
        tokenBuffer.insert(tok, at: 0)
    }
    private func add(type: TennTokenType, literal: String) {
        self.tokenBuffer.append(
            TennToken(type: type, literal: literal, line: currentLine, col: currentChar, pos: self.pos-literal.count, size: literal.count)
        )
    }
    private func add(check pattern: inout [Character]) {
        if pattern.count > 0 {
            self.add(type: detectSymbolType(pattern: pattern), literal: String(pattern))
            pattern.removeAll(keepingCapacity: true)
        }
        
    }
    private func add(literal: [Character]) {
        self.add(type: .stringLit, literal: String(literal))
    }
    private func detectSymbolType( pattern: [Character] ) -> TennTokenType {
        var value = pattern
        if value.count > 0 && value[0] == "-" {
            value.remove(at: value.index(value.startIndex, offsetBy: 0))
        }
        var dot = false
        var i = 0
        for c in value {
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
    
    private func inc(_ pos: Int = 1) {
        self.currentChar += pos
        self.pos += pos
    }
    private func next() -> Character {
        return charAt(1)
    }
    
    private func charAt(_ offset:Int = 0)-> Character {
        if self.pos + offset < self.bufferCount {
            return self.buffer[self.pos + offset]
        }
        return Character("\0")
    }
    
    private func readString( r: inout [Character], lit: Character) {
        self.add(check: &r)
        self.inc()
        
        var foundEnd = false
        let stPos = self.pos
        while self.pos < self.bufferCount {
            let curChar = self.charAt()
            if  curChar == "\n" {
                self.currentLine += 1;
                self.currentChar = 0;
                r.append(curChar)
            }
            else if curChar == lit {
                let curPos = self.pos
                
                var foundPlus = false
                let oldCurLines = self.currentLine
                let oldCurChar = self.currentChar
                self.inc()
                while self.pos < self.bufferCount {
                    let chartAt = charAt()
                    if (chartAt == " " || chartAt == "\t" || chartAt == "\r") {
                        // All is ok
                    } else if (chartAt == "\n") {
                        self.currentLine += 1
                        self.currentChar = 0
                    } else if (chartAt == "+") {
                        foundPlus = true
                    } else if (chartAt == lit) {
                        break
                    } else {
                        break
                    }
                    self.inc()
                }
                if (!foundPlus) {
                    currentLine = oldCurLines
                    currentChar = oldCurChar
                    self.pos = curPos
                } else {
                    // Success we found + and spaces between and new string.
                    self.inc()
                    continue
                }
                foundEnd = true
                
                self.add(literal: r);
                r.removeAll(keepingCapacity: true)
                self.inc()
                break
            }
            else if (self.charAt() == "\\" && self.next() == lit) {
                r.append(self.next())
                self.inc(1)
            } else {
                r.append(self.charAt())
            }
            self.inc()
        }
        if (r.count > 0) {
            self.add(literal: r)
            r.removeAll(keepingCapacity: true)
        }
        if !foundEnd {
            if let h = self.errorHandler {
                h(.EndOfLineReadString, stPos, pos)
            }
        }
    }
    private func skipCComment(_ r: inout [Character]) {
        self.add(check: &r)
    
        self.inc(2); // Skip \/*
    
        while self.pos < self.bufferCount {
            switch self.charAt() {
            case "\n":
                self.currentLine += 1
                self.currentChar = 0
                break
            case "*":
                if self.next() == "/" {
                    // end of comment
                    self.inc(2)
                    return
                }
            default:
                // No Actions required
                break
            }
            self.inc()
        }
    }

    
    private func processComment( _ r: inout [Character], _ cc: Character) {
        if self.next() == "*" {
            // C/C++ multi-line comment
            self.skipCComment(&r)
        } else if self.next() == "/" {
            // End of line comment
            while self.pos < self.bufferCount {
                if self.charAt() == "\n" {
                    self.currentLine += 1
                    self.currentChar = 0
                    break
                }
                self.inc()
            }
        } else {
            r.append(cc)
            self.inc()
        }
    }
    
    private func processNewLine(_ r: inout [Character], _ cc: Character) -> Bool {
        self.add(check: &r)
        if cc == "\n" {
            self.currentLine += 1
            self.currentChar = 0
            // Check if we need to send a delimiter semicolon symbol.
            if self.blockState.count == 0 || self.blockState[0] == .curlyLe {
                self.add(type: .semiColon, literal: "\n")
            }
        }
        self.inc()
        if !self.tokenBuffer.isEmpty {
            return true
        }
        return false
    }
    
    public func getToken() -> TennToken? {
        if self.tokenBuffer.count > 0 {
            return self.tokenBuffer.removeFirst()
        }
        var r: [Character] = []
        while self.pos < self.bufferCount {
            let cc = charAt()
            switch (cc) {
            case " ", "\t", "\r","\n":
                if self.processNewLine(&r, cc) {
                    return self.tokenBuffer.removeFirst()
                }
            case "{":
                self.add(check: &r)
                self.processCurlyOpen(cc)
            case "}":
                if self.processCurlyClose(&r, cc) {
                    return self.tokenBuffer.removeFirst()
                }
            case ";":
                self.add(check: &r)
                
                self.add(type: .semiColon, literal: String(cc))
                self.inc()
                if  !self.tokenBuffer.isEmpty {
                    return self.tokenBuffer.removeFirst()
                }
            case "/":
                self.processComment(&r, cc)
            case "%":
                let nc = self.next()
                if nc == "{" {
                    readExpression(r: &r, startLit: "{", endLit: "}", type: .markdownLit)
                }
                else {
                    r.append(cc)
                    self.inc()
                }
                break;
            case "$":
                let nc = self.next()
                if nc == "(" {
                    readExpression(r: &r, startLit: "(", endLit: ")", type: .expression)
                } else if nc == "{" {
                    readExpression(r: &r, startLit: "{", endLit: "}", type: .expressionBlock)
                } else {
                    r.append(cc)
                    self.inc()
                }
                break;
            case "\'", "\"":
                self.readString(r: &r, lit: cc)
                if !self.tokenBuffer.isEmpty {
                    return self.tokenBuffer.removeFirst()
                }
            default:
                r.append(cc)
                self.inc()
            }
        }
        
        self.add(check: &r)
        
        if self.pos == self.bufferCount {
            self.add(type: .eof, literal: "\0")
            self.inc()
        }
        
        if (self.tokenBuffer.count > 0) {
            return self.tokenBuffer.removeFirst()
        } else {
            return nil
        }
    }
    
    private func processCurlyOpen(_ cc: Character) {
        self.add(type: .curlyLe, literal: String(cc))
        self.inc()
        
        self.blockState.insert(.curlyLe, at: 0)
    }
    private func processCurlyClose( _ r: inout [Character], _ cc: Character)-> Bool {
        self.add(check: &r)
        
        self.add(type: .curlyRi, literal: String(cc))
        self.inc()
        
        if self.blockState.count == 0 {
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
    
    private func readExpression( r: inout [Character], startLit: Character, endLit: Character, type: TennTokenType) {
        self.add(check: &r)
        self.inc(2)
        
        let stPos = self.pos
        var foundEnd = false
        var indent = 1
        let startLine = self.currentLine
        while self.pos < self.bufferCount {
            let curChar = self.charAt()
            if  curChar == "\n" {
                self.currentLine += 1;
                self.currentChar = 0;
                r.append(curChar)
            }
            else if curChar == startLit {
                indent += 1
                r.append(self.charAt())
            }
            else if curChar == endLit {
                indent -= 1
                if indent == 0 {
                    foundEnd = true
                    break
                }
                else {
                    r.append(self.charAt())
                }
            }
            else {
                r.append(self.charAt())
            }
            self.inc()
        }
        if !foundEnd {
            if let h = self.errorHandler {
                h(.EndOfExpressionReadError, stPos, pos)
            }
        }
        else {
            if (r.count > 0) {
                self.tokenBuffer.append(
                    TennToken(type: type, literal: String(r), line: startLine, col: currentChar, pos: self.pos-r.count, size: r.count)
                )
                r.removeAll(keepingCapacity: true)
            }
            self.inc()
        }
    }
}
