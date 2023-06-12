//
//  TennPrinter.swift
//  Tenniarb
//
//  Created by Andrey Sobolev on 27/06/2017.
//  Copyright © 2017 Andrey Sobolev. All rights reserved.
//
//  Licensed under the Eclipse Public License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License. You may
//  obtain a copy of the License at https://www.eclipse.org/legal/epl-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//
//  See the License for the specific language governing permissions and
//  limitations under the License.

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
    private func quote(_ val: String, _ tok: String = "\"") -> String {
        return val.replacingOccurrences(of: tok, with: "\\" + tok, options: .literal, range: nil)
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
                    result.append("\"\(quote(tok.literal))\"")
                }
            case .Expression:
                result.append("$(\(tok.literal))")
            case .Image:
                result.append("@(\(tok.literal))")
            case .ExpressionBlock:
                result.append("${")
                result.append("\(tok.literal)")
                result.append("}")
            case .MarkdownLit:
                result.append("%{\(tok.literal)}")
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
