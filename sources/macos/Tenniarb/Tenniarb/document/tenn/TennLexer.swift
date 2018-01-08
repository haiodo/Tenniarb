//
//  TennLexer.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 21/06/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//

import Foundation

public enum TennTokenType {
    case invalid
    case eof
    case symbol
    case floatLit
    case intLit
    case stringLit
    case charLit
    case curlyLe
    case curlyRi
    case comma
    case colon
    case semiColon
    case hash
}

public class TennToken {
    public let type: TennTokenType
    public let literal: String
    public let line: Int
    public let col: Int
    
    init( type: TennTokenType, literal: String, line: Int = 0, col:Int = 0) {
        self.type = type
        self.literal = literal
        self.line = line
        self.col = col
    }
}

public class TennLexer {
    var currentLine: Int = 0
    var currentChar: Int = 0
    var buffer: Array<Character>
    var pos: Int = 0
    
    var tokenBuffer: [TennToken] = []
    var blockState:[TennTokenType] = []
    
    init( _ code: String) {
        self.buffer = Array(code)
    }
    
    public func revert(tok: TennToken) {
        tokenBuffer.insert(tok, at: 0)
    }
    private func add(type: TennTokenType, literal: String) {
        self.tokenBuffer.append(
            TennToken(type: type, literal: literal, line: currentLine, col: currentChar)
        )
    }
    private func add(check pattern: inout String) {
        if pattern.count > 0 {
            self.add(type: detectSymbolType(pattern: pattern), literal: pattern)
            pattern.removeAll(keepingCapacity: true)
        }
        
    }
    private func add(literal: String) {
        self.add(type: .stringLit, literal: literal)
    }
    private func detectSymbolType( pattern: String ) -> TennTokenType {
        var value = pattern
        if value.count > 0 && Character("-") == value.characters.first {
            value.remove(at: value.index(value.startIndex, offsetBy: 0))
        }
        var dot = false
        var i = 0
        for c in pattern {
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
        return TennTokenType.stringLit
    }
    
    private func inc(_ pos: Int = 1) {
        self.currentChar += pos
        self.pos += pos
    }
    private func next() -> Character {
        return charAt(1)
    }
    
    private func charAt(_ offset:Int = 0)-> Character {
        if self.pos + offset < self.buffer.count {
            return self.buffer[self.pos + offset]
        }
        return Character("\0")
    }
    
    private func readString( r: inout String, lit: Character) {
        self.add(check: &r)
        self.inc()
        
        while self.pos < self.buffer.count {
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
                while self.pos < self.buffer.count {
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
                
                self.add(literal: r);
                r.removeAll(keepingCapacity: true)
                self.inc()
                break
            }
            else if (self.charAt() == "\\" && self.next() == lit) {
                // Pass escaped
                r.append(self.charAt())
                r.append(self.next())
                self.inc(1)
            } else {
                r.append(self.charAt())
            }
            self.inc()
        }
        if (r.count > 0) {
            // Add last value in case of end of file happen.
            self.add(literal: r)
            r.removeAll(keepingCapacity: true)
        }
    }
    private func skipCComment(_ r: inout String) {
        self.add(check: &r)
    
        self.inc(2); // Skip \/*
    
        while self.pos < self.buffer.count {
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
    
    private func processHash(_ r: inout String) {
        self.add(check: &r)
        
        while self.pos < self.buffer.count {
            if (charAt() == "\n") {
                self.currentLine += 1
                self.currentChar = 0
            }
            r.append(charAt())
            self.inc()
        }
        self.add(type: .hash, literal: r)
        r.removeAll(keepingCapacity: true)
    }
    
    private func processComment( _ r: inout String, _ cc: Character) {
        if self.next() == "*" {
            // C/C++ multi-line comment
            self.skipCComment(&r)
        } else if self.next() == "/" {
            // End of line comment
            while self.pos < self.buffer.count {
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
    
    private func processNewLine(_ r: inout String, _ cc: Character) -> Bool {
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
        var r = String()
        while self.pos < self.buffer.count {
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
            case ",", ";":
                self.add(check: &r)
                
                self.add(type: .semiColon, literal: String(cc))
                self.inc()
                if  !self.tokenBuffer.isEmpty {
                    return self.tokenBuffer.removeFirst()
                }
            case "/":
                self.processComment(&r, cc)
            case "\'", "\"":
                self.readString(r: &r, lit: cc)
                if !self.tokenBuffer.isEmpty {
                    return self.tokenBuffer.removeFirst()
                }
            case "#":
                self.processHash(&r)
            default:
                r.append(cc)
                self.inc()
            }
        }
        
        self.add(check: &r)
        
        if self.pos == self.buffer.count {
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
    private func processCurlyClose( _ r: inout String, _ cc: Character)-> Bool {
        self.add(check: &r)
        
        self.add(type: .curlyRi, literal: String(cc))
        self.inc()
        
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
}

public class TennPrinter {
    
}
