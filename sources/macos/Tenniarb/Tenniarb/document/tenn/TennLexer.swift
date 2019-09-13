//
//  TennLexer.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 21/06/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//

import Foundation

typealias BufferType = [Character]
public class TennLexer: TennLexerProtocol {
    private var currentLine: Int = 0
    private var currentChar: Int = 0
    private var buffer: ContiguousArray<Unicode.Scalar>
    private var bufferCount: Int = 0
    private var pos: Int = 0
    
    private var tokenBuffer: [TennToken] = []
    private var blockState:[TennTokenType] = []
    private var code: String
    
    public var errorHandler: ((_ error: LexerError, _ startPos:Int, _ pos: Int ) -> Void)?
    
    init( _ code: String) {
        self.code = code
        self.buffer = ContiguousArray(code.unicodeScalars)
        self.bufferCount = self.buffer.count
    }
    
    public func revert(tok: TennToken) {
        tokenBuffer.insert(tok, at: 0)
    }
    private func add(type: TennTokenType, literal: BufferType) {
        let c = literal.count
        self.tokenBuffer.append(
            TennToken(type: type, literal: String(literal), line: currentLine, col: currentChar, pos: self.pos-c, size: c)
        )
    }
    
    private func add(check pattern: inout BufferType) {
        if !pattern.isEmpty {
            self.add(type: detectSymbolType(pattern: pattern), literal: pattern)
            pattern.removeAll()//keepingCapacity: true)
        }
        
    }
    private func add(literal: BufferType) {
        let c = literal.count
        self.tokenBuffer.append(
            TennToken(type: .stringLit, literal: String(literal), line: currentLine, col: currentChar, pos: self.pos-c, size: c)
        )
    }
    private func detectSymbolType( pattern value: BufferType ) -> TennTokenType {
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
    
    private func inc(_ pos: Int = 1) {
        self.currentChar += pos
        self.pos += pos
    }
    private func next() -> Character {
        let ppos = self.pos + 1
        if ppos < self.bufferCount {
            return Character(self.buffer[ppos])
        }
        return "\0"
    }
    
    private func readString( r: inout BufferType, lit: Character) {
        self.add(check: &r)
        self.currentChar += 1;  self.pos += 1
        
        var foundEnd = false
        let stPos = self.pos
        while self.pos < self.bufferCount {
            let curChar = Character(self.buffer[self.pos])
            if  curChar == "\n" {
                self.currentLine += 1;
                self.currentChar = 0;
                r.append(curChar)
            }
            else if curChar == lit {
                self.add(literal: r);
                r.removeAll()//keepingCapacity: true)
                self.currentChar += 1;  self.pos += 1
                foundEnd = true
                break
            }
            else if (curChar == "\\" && self.next() == lit) {
                r.append(self.next())
                self.inc(1)
            } else {
                r.append(curChar)
            }
            self.currentChar += 1;  self.pos += 1
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
    private func skipCComment(_ r: inout BufferType) {
        self.add(check: &r)
    
        self.inc(2); // Skip \/*
    
        while self.pos < self.bufferCount {
            switch self.buffer[self.pos] {
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
            self.currentChar += 1;  self.pos += 1
        }
    }

    
    private func processComment( _ r: inout BufferType, _ cc: Character) {
        if self.next() == "*" {
            // C/C++ multi-line comment
            self.skipCComment(&r)
        } else if self.next() == "/" {
            // End of line comment
            while self.pos < self.bufferCount {
                if self.buffer[self.pos] == "\n" {
                    self.currentLine += 1
                    self.currentChar = 0
                    break
                }
                self.currentChar += 1;  self.pos += 1
            }
        } else {
            r.append(cc)
            self.currentChar += 1;  self.pos += 1
        }
    }
    
    private func processNewLine(_ r: inout BufferType, _ cc: Character) -> Bool {
        self.add(check: &r)
        self.currentChar += 1;  self.pos += 1
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
        
        var r: BufferType = BufferType()
        while self.pos < self.bufferCount {
            let cc = Character(self.buffer[self.pos])
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
                
                self.add(type: .semiColon, literal: [cc])
                self.currentChar += 1;  self.pos += 1
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
                    self.currentChar += 1;  self.pos += 1
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
                    self.currentChar += 1;  self.pos += 1
                }
                break;
            case "\'", "\"":
                self.readString(r: &r, lit: cc)
                if !self.tokenBuffer.isEmpty {
                    return self.tokenBuffer.removeFirst()
                }
            default:
                r.append(cc)
                self.currentChar += 1;  self.pos += 1
            }
        }
        
        self.add(check: &r)
        
        if self.pos == self.bufferCount {
            self.add(type: .eof, literal: ["\0"])
            self.currentChar += 1;  self.pos += 1
        }
        
        if !self.tokenBuffer.isEmpty {
            return self.tokenBuffer.removeFirst()
        } else {
            return nil
        }
    }
    
    private func processCurlyOpen(_ cc: Character) {
        self.currentChar += 1;  self.pos += 1
        self.tokenBuffer.append(
            TennToken(type: .curlyLe, literal: String(cc), line: currentLine, col: currentChar, pos: self.pos-1, size: 1)
        )
        
        self.blockState.insert(.curlyLe, at: 0)
    }
    private func processCurlyClose( _ r: inout BufferType, _ cc: Character)-> Bool {
        self.add(check: &r)
        
        self.currentChar += 1;  self.pos += 1
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
    
    private func readExpression( r: inout BufferType, startLit: Character, endLit: Character, type: TennTokenType) {
        self.add(check: &r)
        self.inc(2)
        
        let stPos = self.pos
        var foundEnd = false
        var indent = 1
        let startLine = self.currentLine
        while self.pos < self.bufferCount {
            let curChar = Character(self.buffer[self.pos])
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
            self.currentChar += 1;  self.pos += 1
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
            self.currentChar += 1;  self.pos += 1
        }
    }
}
