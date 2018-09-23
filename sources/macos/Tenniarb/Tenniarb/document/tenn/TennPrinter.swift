//
//  TennPrinter.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 27/06/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//

import Foundation

// A printing extension
extension TennNode {
    private static let spaces = "    "
    private func makeSeq(_ sb: inout String, pattern: String, count: Int ) {
        if count > 0 {
            for _ in 0..<count {
                sb.append(pattern)
            }
        }
    }
    private func getSeq(pattern: String, count: Int ) -> String {
        var sb = ""
        if count > 0 {
            for _ in 0..<count {
                sb.append(pattern)
            }
        }
        return sb
    }
    private func quote(_ val: String) -> String {
        return val.replacingOccurrences(of: "\"", with: "\\\"", options: .literal, range: nil)
    }
    public func childsToStr(_ result: inout String, _ ind: Int, _ clean: Bool) {
        if let children = self.children {
            var i = 0
            for c in children {
                result.append(c.toStr(ind, clean))
                if i != children.count - 1 {
                    if self.kind == .BlockExpr || self.kind == .Statements {
                        result.append("\n")
                    }
                    else {
                        result.append(" ")
                    }
                }
                i += 1
            }
        }
    }
    
    public func toStr( _ indent: Int = 0, _ clean: Bool = false) -> String {
        var result = ""
        
        if self.kind == .Command {
            makeSeq(&result, pattern: TennNode.spaces, count: indent)
        }
        if let tok = self.token {
            switch self.kind {
            case .CharLit, .IntLit, .Ident, .FloatLit:
                result.append(tok.literal)
            case .StringLit:
                if clean {
                    result.append(tok.literal)
                }
                else {
//                    let parts = tok.literal.components(separatedBy: "\n")
//                    if parts.count > 1 {
//                        var c = 0
//                        for p in parts {
//                            if c == parts.count - 1 {
//                                makeSeq(&result, pattern: TennNode.spaces, count: indent+1)
//                                result.append("\"\(quote(p))\"")
//                            }
//                            else {
//                                if c != 0 {
//                                    makeSeq(&result, pattern: TennNode.spaces, count: indent+1)
//                                }
//                                result.append("\"\(quote(p))\\n\" + \n")
//                            }
//                            c += 1
//                        }
//                    }
//                    else {
                    result.append("\"\(quote(tok.literal))\"")
//                    }
                }
            default:
                break
            }
        }
        var ind = indent
        var postfix: String? = nil
        if self.kind == .BlockExpr {
            result.append("{\n")
            if self.count > 0 {
                postfix = "\n\(getSeq(pattern: TennNode.spaces, count: indent))}"
            }
            else {
                postfix = "\(getSeq(pattern: TennNode.spaces, count: indent))}"
            }
            ind += 1
        }
        childsToStr(&result, ind, clean)
        if let p = postfix {
            result.append(p)
        }
        
        return result
    }
}
