//
//  PenCompact.swift
//  Tenniarb
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
//

import Foundation
import Tenn

/// The compact canonical printer of go/tenn's compact.go: a block of plain commands on one line, quotes
/// dropped where a bare word reads back as the same text. ToStr stays the byte-exact printer of the Tenn
/// package; this one only chooses the line breaks.
enum PenCompact {
    /// The widest line (bytes) a one-line block may reach, as CompactLimit.
    static let limit = 160

    static func compact(_ node: TennNode, indent: String = "  ", limit: Int = limit, pack: Bool = false) -> String {
        var buf = ""
        if node.kind == .Statements || node.kind == .BlockExpr {
            for (i, c) in (node.children ?? []).enumerated() {
                if i > 0 { buf.append("\n") }
                cmd(&buf, c, 0, indent, limit, pack)
            }
        } else {
            cmd(&buf, node, 0, indent, limit, pack)
        }
        return buf
    }

    private static func cmd(
        _ buf: inout String, _ c: TennNode, _ ind: Int, _ indent: String, _ limit: Int, _ pack: Bool
    ) {
        buf += String(repeating: indent, count: ind)
        guard c.kind == .Command else {
            buf += atom(c)
            return
        }
        for (i, ch) in (c.children ?? []).enumerated() {
            if i > 0 { buf.append(" ") }
            if ch.kind != .BlockExpr {
                buf += atom(ch)
                continue
            }
            if let one = oneLineBlock(ch), lineWidth(buf) + one.utf8.count <= limit {
                buf += one
                continue
            }
            buf += "{\n"
            block(&buf, ch.children ?? [], ind + 1, indent, limit, pack)
            buf += "\n" + String(repeating: indent, count: ind) + "}"
        }
    }

    /// Prints the commands of a multi-line block, one per line, or packed.
    private static func block(
        _ buf: inout String, _ cmds: [TennNode], _ ind: Int, _ indent: String, _ limit: Int, _ pack: Bool
    ) {
        var j = 0
        while j < cmds.count {
            if j > 0 { buf.append("\n") }
            let run = pack ? packRun(Array(cmds[j...]), ind, indent, limit) : []
            if run.count > 1 {
                buf += String(repeating: indent, count: ind) + run.joined(separator: "; ")
                j += run.count
                continue
            }
            cmd(&buf, cmds[j], ind, indent, limit, pack)
            j += 1
        }
    }

    /// The text of the commands from the start of cmds that share a line; none when Pack is off.
    private static func packRun(_ cmds: [TennNode], _ ind: Int, _ indent: String, _ limit: Int) -> [String] {
        var width = ind * indent.utf8.count
        var run: [String] = []
        for c in cmds {
            guard let text = plainCommand(c) else { break }
            if !run.isEmpty {
                if width + 2 + text.utf8.count > limit { break }
                width += 2
            }
            width += text.utf8.count
            run.append(text)
        }
        return run
    }

    /// Renders a command without a block or a line break.
    private static func plainCommand(_ c: TennNode) -> String? {
        guard c.kind == .Command else { return nil }
        var parts: [String] = []
        for a in c.children ?? [] {
            if a.kind == .BlockExpr { return nil }
            parts.append(atom(a))
        }
        let text = parts.joined(separator: " ")
        return text.contains("\n") ? nil : text
    }

    /// Renders a block whose commands hold no blocks and no line breaks.
    private static func oneLineBlock(_ b: TennNode) -> String? {
        let children = b.children ?? []
        if children.isEmpty { return "{}" }
        var parts: [String] = []
        for c in children {
            guard c.kind == .Command else { return nil }
            var sb = ""
            for (i, a) in (c.children ?? []).enumerated() {
                if a.kind == .BlockExpr { return nil }
                if i > 0 { sb.append(" ") }
                sb += atom(a)
            }
            if sb.contains("\n") { return nil }
            parts.append(sb)
        }
        return "{ " + parts.joined(separator: "; ") + " }"
    }

    private static func atom(_ n: TennNode) -> String {
        if n.kind == .StringLit, let tok = n.token, bareSafe(tok.literal) {
            return tok.literal
        }
        return n.toStr(0, false)
    }

    /// The word lexes as one symbol token, never a number, so the parser gives an Ident that reads as the
    /// same text.
    private static func bareSafe(_ s: String) -> Bool {
        if s.isEmpty { return false }
        for c in s {
            if !(PenImport.isLetter(c) || PenImport.isDigit(c) || c == "_" || c == "-" || c == "." || c == "#") {
                return false
            }
        }
        return detectSymbolType(s) == .symbol
    }

    /// Classifies a bare word as intLit, floatLit or symbol.
    private static func detectSymbolType(_ value: String) -> TennTokenType {
        var skipFirst = value.hasPrefix("-")
        var dot = false
        var i = 0
        for c in value {
            if skipFirst {
                skipFirst = false
                continue
            }
            if c == "." {
                if i == 0 || dot { return .symbol }
                dot = true
                continue
            }
            guard let first = c.unicodeScalars.first, first.properties.generalCategory == .decimalNumber else {
                return .symbol
            }
            i += 1
        }
        return dot ? .floatLit : .intLit
    }

    /// The bytes after the last line break of buf.
    private static func lineWidth(_ buf: String) -> Int {
        guard let i = buf.lastIndex(of: "\n") else { return buf.utf8.count }
        return buf[buf.index(after: i)...].utf8.count
    }
}
