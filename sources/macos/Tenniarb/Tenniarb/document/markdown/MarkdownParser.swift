//
//  MarkdownParser.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 18.09.2019.
//  Copyright © 2019 Andrey Sobolev. All rights reserved.
//

import Foundation

public enum MarkdownTokenType {
    case text
    case bold   // * bold \* text * - all until next *
    case italic // _ italic \_ text * - all until next _
    case image // @(image name|640x480), @(image name|640), @(image name|x480)
    case color // !(red), !(#ffeeff), !(red|word) !()- default color => global text color
    case expression // ${expression}
    case title // ## Title value
    case bullet // * some value
    case code   // `some code`
    case underline // <underscore>
    case scratch // ~text~
    case eof
}

public class MarkdownToken {
    public let type: MarkdownTokenType
    public let literal: String
    public let line: Int
    public let col: Int
    public let pos: Int
    public let size: Int
    
    init( type: MarkdownTokenType, literal: String, line: Int = 0, col:Int = 0, pos:Int = 0, size:Int = 0) {
        self.type = type
        self.literal = literal
        self.line = line
        self.col = col
        self.pos = pos
        self.size = size
    }
}

extension MarkdownToken: Hashable {
    public static func == (lhs: MarkdownToken, rhs: MarkdownToken) -> Bool {
        return lhs.type == rhs.type && lhs.literal == rhs.literal && lhs.line == rhs.line && lhs.col == lhs.col && lhs.pos == rhs.pos && lhs.size == rhs.size;
    }
    public func hash(into hasher: inout Hasher) {
        hasher.combine(self.type)
        hasher.combine(self.literal)
        hasher.combine(self.col)
        hasher.combine(self.pos)
        hasher.combine(self.size)
    }
}

public class MarkdownLexer {
    private var buffer: ContiguousArray<Unicode.Scalar>
    private var bufferCount: Int = 0
    private var pos: Int = 0
    private var currentLine: Int = 0
    private var currentChar: Int = 0
    
    private var lineState = true
    
    private var tokenBuffer: [MarkdownToken] = []
    private var code: String
    
    public var errorHandler: ((_ error: LexerError, _ startPos:Int, _ pos: Int ) -> Void)?
    
    init( _ code: String) {
        self.code = code
        self.buffer = ContiguousArray(code.unicodeScalars)
        self.bufferCount = self.buffer.count
    }
    
    public func revert(tok: MarkdownToken) {
        tokenBuffer.insert(tok, at: 0)
    }
    private func add(type: MarkdownTokenType, literal: BufferType) {
        let c = literal.count
        self.tokenBuffer.append(
            MarkdownToken(type: type, literal: String(literal), line: currentLine, col: currentChar, pos: self.pos-c, size: c)
        )
    }
    
    private func add(check pattern: inout BufferType) {
        if !pattern.isEmpty {
            self.add(type: .text, literal: pattern)
            pattern.removeAll()//keepingCapacity: true)
        }
        
    }
    private func add(literal: BufferType) {
        let c = literal.count
        self.tokenBuffer.append(
            MarkdownToken(type: .text, literal: String(literal), line: currentLine, col: currentChar, pos: self.pos-c, size: c)
        )
    }
    
    private func next() -> Character {
        let ppos = self.pos + 1
        if ppos < self.bufferCount {
            return Character(self.buffer[ppos])
        }
        return "\0"
    }
    
    public func getToken() -> MarkdownToken? {
        if !self.tokenBuffer.isEmpty {
            return self.tokenBuffer.removeFirst()
        }
        
        var r: BufferType = BufferType()
        
        var wasWhiteSpace = self.pos == 0
        if self.pos > 0 && self.pos < self.bufferCount {
            let prev = self.buffer[self.pos-1]
            switch prev {
            case " ", "\t", "\r","\n":
                wasWhiteSpace = true
            default:
                break
            }
        }
        while self.pos < self.bufferCount {
            let cc = Character(self.buffer[self.pos])
            switch (cc) {
            case " ", "\t", "\r","\n":
                r.append(cc)
                self.currentChar += 1;  self.pos += 1
                wasWhiteSpace = true
                if cc == "\n" {
                    self.currentLine += 1
                    self.currentChar = 0
                    lineState = true // Mark as new line is started and we need to capture prefixes.
                    add(check: &r)
                    r.removeAll()
                }
            case "\\":
//                r.append(cc)
                // Skip next if required to skip
                let nc = self.next()
                self.currentChar += 1;  self.pos += 1
                switch nc {
                case "@", "$", "*", "_", "#", "<", "~", "!":
                    r.append(nc)
                    self.currentChar += 1;  self.pos += 1
                default:
                    break;
                }
                wasWhiteSpace = false
            case "@":
                self.add(check: &r)
                let nc = self.next()
                if nc == "(" {
                    readUntil(r: &r, startLit: "(", endLit: ")", type: .image)
                } else {
                    r.append(cc)
                    self.currentChar += 1;  self.pos += 1
                }
                wasWhiteSpace = false
                break;
            case "!":
                self.add(check: &r)
                let nc = self.next()
                if nc == "(" {
                    readUntil(r: &r, startLit: "(", endLit: ")", type: .color, addEmpty: true)
                }
                else {
                    r.append(cc)
                    self.currentChar += 1;  self.pos += 1
                }
                wasWhiteSpace = false
                break;
            case "*":
                // Check if this is bullets list, if we have at least 1 space and all spaces before it will be bullet.
                if lineState && next() == " " {
                    // Only whitespaces before, and at least one space
                    r.append(cc)
                    self.currentChar += 1;  self.pos += 1
                    self.add(type: .bullet, literal: r)
                    r.removeAll()//keepingCapacity: true)
                } else if( wasWhiteSpace && next() != " " ) {
                    self.add(check: &r) // Add previous line
                    self.currentChar += 1;  self.pos += 1
                    // This is potentially ** ** - strong or * * emphasize
                    if processUntilCharExceptNewLine(&r, "*") {
                        self.add(type: .bold, literal: r)
                    } else {
                        self.add(type: .text, literal: r)
                    }
                    r.removeAll()//keepingCapacity: true)
                } else {
                    // Just *
                    r.append(cc)
                    self.currentChar += 1;  self.pos += 1
                }
                wasWhiteSpace = false
                
                break;
            case "_":
                self.add(check: &r)
                if wasWhiteSpace && next() != " " {
                    self.currentChar += 1;  self.pos += 1
                    if processUntilCharExceptNewLine(&r, "_") {
                        self.add(type: .italic, literal: r)
                    } else {
                        self.add(type: .text, literal: r)
                    }
                    r.removeAll()//keepingCapacity: true)
                } else {
                    r.append(cc)
                    self.currentChar += 1;  self.pos += 1
                }
                wasWhiteSpace = false
                
                break;
            case "<":
                self.add(check: &r)
                if wasWhiteSpace && next() != " " {
                    self.currentChar += 1;  self.pos += 1
                    if processUntilCharExceptNewLine(&r, ">") {
                        self.add(type: .underline, literal: r)
                    } else {
                        self.add(type: .text, literal: r)
                    }
                    r.removeAll()//keepingCapacity: true)
                } else {
                    r.append(cc)
                    self.currentChar += 1;  self.pos += 1
                }
                wasWhiteSpace = false
                
                break;
            case "~":
                self.add(check: &r)
                if wasWhiteSpace && next() != " " {
                    self.currentChar += 1;  self.pos += 1
                    if processUntilCharExceptNewLine(&r, "~") {
                        self.add(type: .scratch, literal: r)
                    } else {
                        self.add(type: .text, literal: r)
                    }
                    r.removeAll()//keepingCapacity: true)
                } else {
                    r.append(cc)
                    self.currentChar += 1;  self.pos += 1
                }
                wasWhiteSpace = false
                
                break;
            case "#":
                self.add(check: &r)
                r.append(cc)
                self.currentChar += 1;  self.pos += 1
                self.processUntilNewLine(&r)
                wasWhiteSpace = true
                self.add(type: .title, literal: r)
                r.removeAll()
                break;
            case "$":
                self.add(check: &r)
                let nc = self.next()
                if nc == "(" {
                    readUntil(r: &r, startLit: "(", endLit: ")", type: .expression)
                } else if nc == "{" {
                    readUntil(r: &r, startLit: "{", endLit: "}", type: .expression)
                } else {
                    r.append(cc)
                    self.currentChar += 1;  self.pos += 1
                }
                wasWhiteSpace = false
                break;
            case "`":
                self.add(check: &r)
                self.readUntilWithEscaping(r: &r, lit: "`", type: .code )
                wasWhiteSpace = false
                break;
            default:
                wasWhiteSpace = false
                r.append(cc)
                self.currentChar += 1;  self.pos += 1
            }
            // Do not collect more whitespace characters.
            if !wasWhiteSpace {
                lineState = false
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
    
    private func processUntilNewLine( _ r: inout BufferType) {
        // End of line comment
        while self.pos < self.bufferCount {
            let cc = Character(self.buffer[self.pos])
            if cc == "\n" {
                self.currentLine += 1
                self.currentChar = 0
                lineState = true // Mark as new line is started and we need to capture prefixes.
                r.append(cc)
                self.currentChar += 1;  self.pos += 1
                break
            }
            r.append(cc)
            self.currentChar += 1;  self.pos += 1
        }
    }
    
    private func processUntilCharExceptNewLine( _ r: inout BufferType, _ c: Character) -> Bool {
        // End of line comment
        while self.pos < self.bufferCount {
            let cc = Character(self.buffer[self.pos])
            if cc == "\n" {
                self.currentLine += 1
                self.currentChar = 0
                r.append(cc)
                self.pos += 1
                lineState = true // Mark as new line is started and we need to capture prefixes.
                return false
            }
            if cc == c {
                // We found out character, return
                self.currentChar += 1;  self.pos += 1
                return true
            }
            r.append(cc)
            self.currentChar += 1;  self.pos += 1
        }
        return false
    }
    
    private func readUntilWithEscaping( r: inout BufferType, lit: Character, type: MarkdownTokenType) {
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
                self.add(type: type, literal: r)
                r.removeAll()//keepingCapacity: true)
                self.currentChar += 1;  self.pos += 1
                foundEnd = true
                break
            }
            else if (curChar == "\\" && self.next() == lit) {
                r.append(self.next())
                self.currentChar += 1;  self.pos += 1
            } else {
                r.append(curChar)
            }
            self.currentChar += 1;  self.pos += 1
        }
        if !r.isEmpty {
            self.add(type: type, literal: r)
            r.removeAll()//keepingCapacity: true)
        }
        if !foundEnd {
            if let h = self.errorHandler {
                h(.EndOfLineReadString, stPos, pos)
            }
        }
    }
    
    private func readUntil( r: inout BufferType, startLit: Character, endLit: Character, type: MarkdownTokenType, addEmpty: Bool = false) {
        self.add(check: &r)
        self.currentChar += 2;  self.pos += 2;
        
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
            if !r.isEmpty || addEmpty {
                let c = r.count
                self.tokenBuffer.append(
                    MarkdownToken(type: type, literal: String(r), line: startLine, col: currentChar, pos: self.pos-c, size: c)
                )
                r.removeAll()//keepingCapacity: true)
            }
            self.currentChar += 1;  self.pos += 1
        }
    }
    public static func getTokens(code: String ) -> [MarkdownToken] {
        let lexer = MarkdownLexer(code)
        var tokens:[MarkdownToken] = []
        
        while true {
            guard let t = lexer.getToken() else {
                break
            }
            tokens.append(t)
        }
        return tokens
    }
}
