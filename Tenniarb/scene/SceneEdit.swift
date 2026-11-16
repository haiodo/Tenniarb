//
//  SceneEdit.swift
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

/// The edit producers of go/app/sceneedit.go: each one reads the text with the byte ranges of
/// Scene.Node and answers the edit that writes a property, a parameter or a command, or nil when there
/// is nothing to change. Offsets are bytes of the text the tree was loaded from.
extension Scene {
    /// A text edit replaces text[start:end) of a document by new. Offsets are bytes, as Scene.Range.
    struct TextEdit {
        var start: Int
        var end: Int
        var new: String
    }
}

/// Prints one command with its arguments the way the compact printer writes them: a word or a number
/// bare, anything else quoted.
func commandText(_ name: String, _ args: String...) -> String { commandText(name, args) }

func commandText(_ name: String, _ args: [String]) -> String {
    ([name] + args.map { atomText(argNode($0)) })
        .joined(separator: " ")
        .trimmingCharacters(in: .whitespacesAndNewlines)
}

/// Keeps a number, a hex colour, an expression and a plain word as written, and quotes the rest.
func argNode(_ a: String) -> TennNode {
    if a.isEmpty {
        return TennNode.newStrNode(a)
    }
    let parser = TennParser()
    let ast = parser.parse("x " + a)
    let cmds = ast.children ?? []
    if !parser.errors.hasErrors(), cmds.count == 1 {
        let kids = cmds[0].children ?? []
        if kids.count == 2, let token = kids[1].token {
            let n = kids[1]
            if n.kind != .StringLit && n.kind != .BlockExpr && token.literal == a {
                return n
            }
        }
    }
    return TennNode.newStrNode(a)
}

/// One argument as the compact printer writes it: a quoted string that reads back as one bare word
/// loses its quotes.
private func atomText(_ n: TennNode) -> String {
    if n.kind == .StringLit, let literal = n.token?.literal, bareSafe(literal) {
        return literal
    }
    return n.toStr(0, false)
}

/// Whether the word lexes as one symbol token, so the parser reads it back as the same text, never as a
/// number.
private func bareSafe(_ s: String) -> Bool {
    if s.isEmpty {
        return false
    }
    for c in s {
        if c.isLetter || "_-.#".contains(c) {
            continue
        }
        guard let first = c.unicodeScalars.first, CharacterSet.decimalDigits.contains(first) else {
            return false
        }
    }
    return detectSymbolType(s) == .symbol
}

/// DetectSymbolType of go/tenn: a run of digits is a number, anything else a symbol; a leading - is the
/// sign of the number, not part of it.
private func detectSymbolType(_ s: String) -> TennTokenType {
    var skipFirst = s.first == "-"
    var dot = false
    var digits = 0
    for c in s {
        if skipFirst {
            skipFirst = false
            continue
        }
        if c == "." {
            if digits == 0 || dot {
                return .symbol
            }
            dot = true
            continue
        }
        guard let first = c.unicodeScalars.first, CharacterSet.decimalDigits.contains(first) else {
            return .symbol
        }
        digits += 1
    }
    return dot ? .floatLit : .intLit
}

/// Prints a coordinate to 1/100 of a point without a trailing .0: a drag gives float noise that makes
/// the file unreadable and says nothing a pixel can show.
func numText(_ f: Double) -> String { canonicalNumber((f * 100).rounded() / 100) }

/// Reports a value written as a number is read back the same, so "007" stays text.
func isNumber(_ v: String) -> Bool {
    guard let f = Double(v) else { return false }
    return canonicalNumber(f) == v
}

/// Reports a value that a command can hold as one bare word.
func plainWord(_ v: String) -> Bool {
    if v.isEmpty || v.contains(where: { " \t\r\n\"';{}".contains($0) }) {
        return false
    }
    return !isNumber(v)
}

/// The command of name written on n itself: in the block of an item, or among the overrides of a use.
/// The last one wins, as in go/scene.
func ownProp(_ n: Scene.Node, _ name: String) -> Scene.Prop? {
    var found: Scene.Prop?
    switch n.kind {
    case .item, .element, .link:
        for p in n.props where p.name == name {
            found = p
        }
    case .use:
        for ov in n.block where ov.kind == .prop && ov.prop?.name == name {
            found = ov.prop
        }
    default:
        break
    }
    return found
}

/// Writes `name args` on n: in place of its own command, else as a new command in its block. No args
/// removes the command.
func setPropEdit(_ text: String, _ n: Scene.Node, _ name: String, _ args: String...) -> Scene.TextEdit? {
    let own = ownProp(n, name)
    if args.isEmpty {
        guard let own else { return nil }
        return removeEdit(text, own.range)
    }
    let line = commandText(name, args)
    if let own {
        return Scene.TextEdit(start: own.range.start, end: own.range.end, new: line)
    }
    return insertEdit(text, n, line)
}

/// Writes the values of parameter prm into the block of the use that sets it: in place of the command
/// of that name, else as a new command. An empty list writes the bare name, which names no style.
func paramEdit(_ text: String, _ use: Scene.Node, _ prm: Scene.Param, _ values: String...) -> Scene.TextEdit {
    var line = prm.name
    for v in values {
        line += " " + paramValue(prm, v)
    }
    if let own = ownProp(use, prm.name) {
        return Scene.TextEdit(start: own.range.start, end: own.range.end, new: line)
    }
    return insertEdit(text, use, line)
}

/// Deletes the value parameter prm of the use sets, so the default of the declaration comes back.
func removeParamEdit(_ text: String, _ use: Scene.Node, _ prm: Scene.Param) -> Scene.TextEdit? {
    guard let own = ownProp(use, prm.name) else { return nil }
    return removeEdit(text, own.range)
}

/// One value of parameter prm as the file writes it (spec 5c): a string in quotes, a number or a word
/// bare, as the default of the declaration is written.
func paramValue(_ prm: Scene.Param, _ v: String) -> String {
    var kind = TennNodeKind.Ident
    if let d = prm.defaultValue, let first = d.args.first {
        kind = first.kind
    }
    switch kind {
    case .StringLit:
        return tennString(v)
    case .IntLit, .FloatLit:
        if isNumber(v) {
            return v
        }
    case .Ident, .CharLit:
        if plainWord(v) {
            return v
        }
    default:
        break
    }
    // no default of a shape, or a value that is not one of it: a number stays a number, else a string
    if isNumber(v) {
        return v
    }
    return tennString(v)
}

private func tennString(_ v: String) -> String { TennNode.newStrNode(v).toStr(0, false) }

/// Finds the at override of path directly in the block of use; paths are compared by their normalized
/// segments, the last one written wins.
func atOf(_ use: Scene.Node, _ path: String) -> Scene.Override? {
    var found: Scene.Override?
    for ov in use.block where ov.kind == .at && samePath(ov.path, path) {
        found = ov
    }
    return found
}

func samePath(_ a: String, _ b: String) -> Bool {
    let sa = a.components(separatedBy: ".")
    let sb = b.components(separatedBy: ".")
    guard sa.count == sb.count else { return false }
    return zip(sa, sb).allSatisfy { normalizeName($0) == normalizeName($1) }
}

/// The command of name in an at override; the last one wins.
func atProp(_ at: Scene.Override, _ name: String) -> Scene.Prop? {
    var found: Scene.Prop?
    for ov in at.block where ov.kind == .prop {
        if let p = ov.prop, p.name == name {
            found = p
        }
    }
    return found
}

/// Writes `name args` for the part of the instance use that path names (names from the root of the
/// instance, spec 4.7): in place of its command in the at override of that path, else as a new command
/// in that at, else as a new at in the block of use. No args removes the command.
func atPropEdit(_ text: String, _ use: Scene.Node, _ path: String, _ name: String, _ args: String...) -> Scene.TextEdit? {
    let at = atOf(use, path)
    let own = at.flatMap { atProp($0, name) }
    switch (args.isEmpty, own) {
    case (true, nil):
        return nil
    case (true, let own?):
        return removeEdit(text, own.range)
    case (false, let own?):
        return Scene.TextEdit(start: own.range.start, end: own.range.end, new: commandText(name, args))
    case (false, nil):
        break
    }
    if let at {
        let holder = Scene.Node(kind: .item, range: at.range, body: at.body)
        return insertEdit(text, holder, commandText(name, args))
    }
    return insertEdit(text, use, commandText("at", path) + " { " + commandText(name, args) + " }")
}

/// The offset of the start of the line that holds off.
func lineStart(_ text: String, _ off: Int) -> Int {
    let b = Array(text.utf8)
    var i = min(max(off, 0), b.count)
    while i > 0 {
        if b[i - 1] == 0x0A {
            return i
        }
        i -= 1
    }
    return 0
}

/// The white space that starts the line of off.
func indentAt(_ text: String, _ off: Int) -> String {
    let b = Array(text.utf8)
    let start = lineStart(text, off)
    var i = start
    while i < b.count && (b[i] == 0x20 || b[i] == 0x09) {
        i += 1
    }
    return String(decoding: b[start..<i], as: UTF8.self)
}

func blank(_ s: String) -> Bool { s.allSatisfy { $0 == " " || $0 == "\t" } }

/// text[start:end) as the Go code slices its string: offsets are bytes, never characters.
private func textSlice(_ text: String, _ start: Int, _ end: Int) -> String {
    let b = Array(text.utf8)
    guard start >= 0, start <= end, end <= b.count else { return "" }
    return String(decoding: b[start..<end], as: UTF8.self)
}

/// Deletes a command: with its line when it is alone on it, else with the ; before or after it.
func removeEdit(_ text: String, _ r: Scene.Range) -> Scene.TextEdit {
    let b = Array(text.utf8)
    let ls = lineStart(text, r.start)
    var le = b[r.end...].firstIndex(of: 0x0A) ?? b.count
    if blank(textSlice(text, ls, r.start)) && blank(textSlice(text, r.end, le)) {
        if le < b.count {
            le += 1
        }
        return Scene.TextEdit(start: ls, end: le, new: "")
    }
    var before = r.start
    while before > 0 && (b[before - 1] == 0x20 || b[before - 1] == 0x09) {
        before -= 1
    }
    if before > 0 && b[before - 1] == 0x3B {
        return Scene.TextEdit(start: before - 1, end: r.end, new: "")
    }
    var after = r.end
    while after < b.count && (b[after] == 0x20 || b[after] == 0x09) {
        after += 1
    }
    if after < b.count && b[after] == 0x3B {
        after += 1
        while after < b.count && b[after] == 0x20 {
            after += 1
        }
        return Scene.TextEdit(start: r.start, end: after, new: "")
    }
    return Scene.TextEdit(start: r.start, end: r.end, new: "")
}

/// Removes the node n.
func deleteEdit(_ text: String, _ n: Scene.Node) -> Scene.TextEdit { removeEdit(text, n.range) }

/// Guesses the indentation step of the file from its first indented line; two spaces when none.
func indentUnit(_ text: String) -> String {
    for line in text.split(separator: "\n", maxSplits: 199, omittingEmptySubsequences: false) {
        let ind = line.prefix { $0 == " " || $0 == "\t" }
        if !ind.isEmpty && ind.count < line.count {
            return String(ind)
        }
    }
    return "  "
}

func indentLines(_ s: String, _ ind: String) -> String {
    s.split(separator: "\n", omittingEmptySubsequences: false)
        .map { $0.isEmpty ? "" : ind + $0 }
        .joined(separator: "\n")
}

/// The lines of src after the first lose the indentation of the line it starts on.
func dedent(_ src: String, _ ind: String) -> String {
    if ind.isEmpty {
        return src
    }
    var lines = src.split(separator: "\n", omittingEmptySubsequences: false).map(String.init)
    for i in 1..<lines.count where lines[i].hasPrefix(ind) {
        lines[i].removeFirst(ind.count)
    }
    return lines.joined(separator: "\n")
}

/// The source of n as the Code section shows it: the lines after the first lose the indentation of the
/// line n starts on.
func nodeText(_ text: String, _ n: Scene.Node) -> String {
    dedent(textSlice(text, n.range.start, n.range.end), indentAt(text, n.range.start))
}

/// Adds cmd (one or more lines) at the end of the content of n: the block of an element, an item or a
/// use, a new block when n has none, the end of the file for a file root or for an element with
/// --from-file.
func insertEdit(_ text: String, _ node: Scene.Node, _ cmd: String) -> Scene.TextEdit {
    let n = node.mount ?? node  // the content of an element with --from-file is written in that file
    let unit = indentUnit(text)
    let b = Array(text.utf8)
    if n.kind == .file {
        let at = b.count
        let pre = at > 0 && b[at - 1] != 0x0A ? "\n" : ""
        return Scene.TextEdit(start: at, end: at, new: pre + cmd + "\n")
    }
    if n.body == Scene.Range() {
        if cmd.contains("\n") {
            let ind = indentAt(text, n.range.start)
            return Scene.TextEdit(
                start: n.range.end, end: n.range.end,
                new: " {\n" + indentLines(cmd, ind + unit) + "\n" + ind + "}")
        }
        return Scene.TextEdit(start: n.range.end, end: n.range.end, new: " { " + cmd + " }")
    }
    let open = n.body.start
    let closing = n.body.end - 1
    let inner = textSlice(text, open + 1, closing)
    if blank(inner.replacingOccurrences(of: "\n", with: "")) && !cmd.contains("\n") {
        return Scene.TextEdit(start: open + 1, end: closing, new: " " + cmd + " ")
    }
    if !inner.contains("\n") && !cmd.contains("\n") {
        var end = closing
        while end > open + 1 && (b[end - 1] == 0x20 || b[end - 1] == 0x09) {
            end -= 1
        }
        return Scene.TextEdit(start: end, end: end, new: "; " + cmd)
    }
    let ind = indentAt(text, n.range.start) + unit
    let ls = lineStart(text, closing)
    if blank(textSlice(text, ls, closing)) {
        return Scene.TextEdit(start: ls, end: ls, new: indentLines(cmd, ind) + "\n")
    }
    return Scene.TextEdit(
        start: closing, end: closing,
        new: "\n" + indentLines(cmd, ind) + "\n" + indentAt(text, n.range.start))
}

/// Adds cmd as the first command of the content of n: the block of an element, an item or a use, a new
/// block when n has none, the start of the file for a file root.
func prependEdit(_ text: String, _ node: Scene.Node, _ cmd: String) -> Scene.TextEdit {
    let n = node.mount ?? node  // the content of an element with --from-file is written in that file
    let unit = indentUnit(text)
    let b = Array(text.utf8)
    if n.kind == .file {
        return Scene.TextEdit(start: 0, end: 0, new: cmd + "\n")
    }
    if n.body == Scene.Range() {
        if cmd.contains("\n") {
            let ind = indentAt(text, n.range.start)
            return Scene.TextEdit(
                start: n.range.end, end: n.range.end,
                new: " {\n" + indentLines(cmd, ind + unit) + "\n" + ind + "}")
        }
        return Scene.TextEdit(start: n.range.end, end: n.range.end, new: " { " + cmd + " }")
    }
    let open = n.body.start
    let closing = n.body.end - 1
    let inner = textSlice(text, open + 1, closing)
    if blank(inner.replacingOccurrences(of: "\n", with: "")) && !cmd.contains("\n") {
        return Scene.TextEdit(start: open + 1, end: closing, new: " " + cmd + " ")
    }
    if !inner.contains("\n") && !cmd.contains("\n") {
        return Scene.TextEdit(start: open + 1, end: open + 1, new: " " + cmd + ";")
    }
    // the command goes before the first line of the content, with that line's indentation
    var first = open + 1
    while first < closing && (b[first] == 0x20 || b[first] == 0x09 || b[first] == 0x0D || b[first] == 0x0A) {
        first += 1
    }
    if first >= closing {
        return Scene.TextEdit(start: open + 1, end: open + 1, new: " " + cmd + " ")
    }
    let ls = lineStart(text, first)
    if ls <= open {  // the content starts on the line of the opening brace
        let ind = indentAt(text, n.range.start) + unit
        return Scene.TextEdit(
            start: open + 1, end: open + 1,
            new: "\n" + indentLines(cmd, ind) + "\n" + indentAt(text, n.range.start) + " ")
    }
    return Scene.TextEdit(start: ls, end: ls, new: indentLines(cmd, indentAt(text, first)) + "\n")
}
