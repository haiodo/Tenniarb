//
//  TennPrinter.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 27/06/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//

import Foundation

// A printing extension
extension TennASTNode {
    private func makeSpaces(_ sb: inout String, pattern: String, count: Int ) {
        for _ in 0..<count {
            sb.append(pattern)
        }
    }
    private func getSpaces(pattern: String, count: Int ) -> String {
        var sb = ""
        for _ in 0..<count {
            sb.append(pattern)
        }
        return sb
    }
    public func toStr( _ indent: Int, _ clean: Bool = false) -> String {
        var result = ""
        
        if self.kind == .Command {
            makeSpaces(&result, pattern: "  ", count: indent)
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
                    result.append("\"\(tok.literal)\"")
                }
            default:
                break
            }
        }
        if let children = self.children {
            if children.count > 0 {
                var ind = indent
                var postfix: String? = nil
                if self.kind == .Statements {
                    if indent != 0 {
                        result.append("{\n")
                        postfix = "\n\(getSpaces(pattern: "  ", count: indent))}"
                    }
                    ind += 1
                }
                var i = 0
                for c in children {
                    result.append(c.toStr(ind, clean))
                    if i  != children.count - 1 {
                        if self.kind == .Statements {
                            result.append("\n")
                        }
                        else {
                            result.append(" ")
                        }
                    }
                    i += 1
                }
                if let p = postfix {
                    result.append(p)
                }
            }
        }
        
        return result
    }
}
