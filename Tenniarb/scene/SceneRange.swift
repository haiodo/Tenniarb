//
//  SceneRange.swift
//  Tenniarb
//
//  Licensed under the Eclipse Public License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License. You may
//  obtain a copy of the License at https://www.eclipse.org/legal/epl-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.
//

import Foundation
import Tenn

/// The closing brace of every block of text, by the byte offset of its "{": the Tenn tree keeps no end
/// of a command, and a brace inside a template, a string or a comment is not the brace to stop at.
func tennBlockEnds(_ text: String) -> [Int: Int] {
    let lexer = TennLexer(text)
    var ends: [Int: Int] = [:]
    var open: [Int] = []
    while let token = lexer.getToken() {
        switch token.type {
        case .curlyLe:
            open.append(token.pos)
        case .curlyRi:
            if let start = open.popLast() {
                ends[start] = token.pos + token.size
            }
        default:
            break
        }
    }
    return ends
}
