//
//  PenImport.swift
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

/// A pen JSON object, as Go reads it into map[string]any: numbers are Double, booleans Bool.
typealias PenMap = [String: Any]

/// An error of reading or converting a .pen file.
struct PenImportFailure: Error, CustomStringConvertible {
    let message: String

    var description: String { message }
}

enum PenKind {
    case file
    case item
    case use
    case slot
}

/// A command argument: a quoted string or a bare word or number.
struct PenArg {
    var s: String
    var quoted = false

    /// The number a bare argument holds, as Go parses it back where a width is computed.
    var num: Double { Double(s) ?? 0 }
}

struct PenProp {
    var name: String
    var args: [PenArg]
}

struct PenOptions {}

/// What a node needs to know about its place.
struct PenBCtx {
    var mode = ""
    var parentAbs = false  // the parent has layout none: x and y matter
    var top = false
    var noIndex = false  // node of a block, not of the file tree
    var layout = false  // the parent is a frame: go/scene draws no name there
    var res: PenFile?
}

/// A node of the produced scene before it is printed.
final class PenNode {
    var kind: PenKind
    var file: PenFile
    var id = ""  // pen id, empty for nodes made up here
    var type = ""  // pen type of the node, of the root of the component for a use
    /// The file the pen refs and ids of the node are written in; it differs from `file` for a copy of a
    /// node of a library that a screen file holds in a block.
    var res: PenFile?
    var pen: PenMap = [:]
    var name = ""
    var parent: PenNode?  // scope: lookups go up this chain
    var props: [PenProp] = []
    var kids: [PenNode] = []  // item and slot children, file root content
    // use
    var target: PenNode?
    var block: [PenOvr] = []
    var content: [PenNode] = []  // nodes written inside the block: they are in scope of paths in it
    // slot
    var slot = ""
    // ref is the pen ref string of a use, kept for messages.
    var ref = ""
    var state: UInt8 = 0
    var needed = false
    var missing = false
    var b = PenBCtx()

    init(kind: PenKind, file: PenFile) {
        self.kind = kind
        self.file = file
    }

    var isNamed: Bool { kind == .item || kind == .use }
}

enum PenOvKind {
    case at
    case replace
    case slot
}

/// One command in the block of a use. `chain` names the target by the nodes on the way down from the
/// instance root; the path is written from their final names.
final class PenOvr {
    var kind: PenOvKind
    var chain: [PenNode]
    var props: [PenProp]
    var nodes: [PenNode]
    var slot: PenNode?

    init(kind: PenOvKind, chain: [PenNode] = [], props: [PenProp] = [], nodes: [PenNode] = [], slot: PenNode? = nil) {
        self.kind = kind
        self.chain = chain
        self.props = props
        self.nodes = nodes
        self.slot = slot
    }
}

/// One .pen file with the libraries it imports.
final class PenFile {
    let path: String
    /// The output file name without the extension: ui-kit.lib for ui-kit.lib.pen.
    let name: String
    var root: PenNode!
    var imports: [String: PenFile] = [:]  // pen alias -> library
    /// The name the other files import this one by.
    var alias: String

    var raw: PenMap
    var rawIdx: [String: PenMap] = [:]
    var vars: [String: PenMap] = [:]
    var index: [String: PenNode] = [:]
    var dropped: Set<String> = []
    /// The type of every node this converter leaves out: a ref to one resolves to nothing, and the
    /// placeholder says the type is not supported instead of guessing that the component is missing.
    var unsupportedById: [String: String] = [:]
    var slots: Set<String> = []
    /// The top level pen nodes that became items: the pen index and the node.
    var tops: [PenTop] = []
    /// What was left out or approximated, by feature.
    var unsupported: [String: Int] = [:]
    /// The variables whose values were resolved to concrete colors.
    var variables: [String: Int] = [:]
    var warnings: [String] = []
    /// The refs that point nowhere.
    var placeholders: [String] = []
    /// The icon names the table does not have, so that the warning is written once per name.
    var missingIcons: Set<String> = []
    var once: Set<String> = []

    init(path: String, name: String, alias: String, raw: PenMap) {
        self.path = path
        self.name = name
        self.alias = alias
        self.raw = raw
    }

    func unsup(_ feature: String) { unsupported[feature, default: 0] += 1 }

    func warn(_ message: String) { warnings.append(message) }

    func mountAlias(_ lib: PenFile) -> String { PenImport.normalize(lib.alias) }

    /// Finds the pen map and the file of a ref string as written in this file.
    func lookupRaw(_ ref: String) -> (PenFile, PenMap)? {
        let (alias, id) = PenImport.splitRef(ref)
        let g: PenFile? = alias.isEmpty ? self : imports[alias]
        guard let g, let m = g.rawIdx[id] else { return nil }
        return (g, m)
    }

    /// Follows refs down to the type of the node an instance is made of.
    func rootType(_ m: PenMap) -> String {
        var m = m
        var f: PenFile = self
        for _ in 0..<40 {
            let t = m["type"] as? String ?? ""
            if t != "ref" { return t }
            let s = m["ref"] as? String ?? ""
            guard let (g, c) = f.lookupRaw(s) else { return "frame" }
            f = g
            m = c
        }
        return "frame"
    }

    /// Finds the node of a ref string as written in this file.
    func resolve(_ ref: String) -> PenNode? {
        let (alias, id) = PenImport.splitRef(ref)
        let g: PenFile? = alias.isEmpty ? self : imports[alias]
        return g?.index[id]
    }
}

struct PenTop {
    var index: Int
    var node: PenNode
}

/// Converts a set of .pen files, libraries first.
final class PenResult {
    let files: [PenFile]

    init(files: [PenFile]) {
        self.files = files
    }
}

/// The state of one conversion run.
final class PenConvert {
    let options: PenOptions
    var mode = "light"
    var files: [String: PenFile] = [:]
    var order: [PenFile] = []  // libraries before the files that import them

    init(options: PenOptions) {
        self.options = options
    }
}

enum PenImport {
    typealias Options = PenOptions
    typealias Kind = PenKind
    typealias Arg = PenArg
    typealias Prop = PenProp
    typealias Node = PenNode
    typealias File = PenFile
    typealias Top = PenTop
    typealias Result = PenResult

    /// Reads the .pen files with the libraries they import and converts all of them.
    static func convert(paths: [String], options: Options = Options()) throws -> Result {
        let c = PenConvert(options: options)
        for p in paths {
            _ = try c.load(p)
        }
        for f in c.order {
            c.buildFile(f)
        }
        for f in c.order {
            c.finishAll(f)
        }
        c.finalizeNames()
        return PenResult(files: c.order)
    }

    // MARK: - Small helpers shared by the conversion

    /// Percent-free number text: zero stays "0", everything else prints as the shortest form that reads back.
    static func fnum(_ v: Double) -> String {
        if v == 0 { return "0" }
        var s = String(v)
        if s.hasSuffix(".0") { s = String(s.dropLast(2)) }
        guard let e = s.firstIndex(where: { $0 == "e" || $0 == "E" }) else { return s }
        // Go formats a fraction with 'f': the exponent form of String(Double) has to be written out.
        var mantissa = String(s[s.startIndex..<e])
        let exp = Int(s[s.index(after: e)...]) ?? 0
        var negative = false
        if mantissa.hasPrefix("-") {
            negative = true
            mantissa = String(mantissa.dropFirst())
        }
        var digits = mantissa.replacingOccurrences(of: ".", with: "")
        var point = (mantissa.firstIndex(of: ".").map { mantissa.distance(from: mantissa.startIndex, to: $0) } ?? mantissa.count) + exp
        if point <= 0 {
            digits = String(repeating: "0", count: 1 - point) + digits
            point = 1
        }
        if digits.count < point {
            digits += String(repeating: "0", count: point - digits.count)
        }
        let cut = digits.index(digits.startIndex, offsetBy: point)
        let whole = String(digits[digits.startIndex..<cut])
        let frac = String(digits[cut...])
        let text = frac.isEmpty ? whole : whole + "." + frac
        return negative ? "-" + text : text
    }

    static func numArg(_ v: Double) -> PenArg { PenArg(s: fnum(v)) }

    static func wordArg(_ s: String) -> PenArg { PenArg(s: s) }

    /// Quotes a string. go/tenn's printer does not escape a backslash and the lexer reads one before a
    /// quote as an escape, so such a backslash is dropped.
    static func strArg(_ s: String) -> PenArg {
        var s = s.replacingOccurrences(of: "\\\"", with: "\"")
        if s.hasSuffix("\\") { s = String(s.dropLast()) }
        return PenArg(s: s, quoted: true)
    }

    static func prop(_ name: String, _ args: PenArg...) -> PenProp { PenProp(name: name, args: args) }

    /// convertNameJS of web/packages/core/src/execution.ts, as in go/scene.
    static func normalize(_ name: String) -> String {
        var out = ""
        for scalar in name.unicodeScalars {
            let cat = scalar.properties.generalCategory
            switch cat {
            case .uppercaseLetter, .lowercaseLetter, .titlecaseLetter, .modifierLetter, .otherLetter,
                .nonspacingMark, .spacingMark, .enclosingMark,
                .decimalNumber, .letterNumber, .otherNumber:
                out.unicodeScalars.append(scalar)
            default:
                out.append("_")
            }
        }
        return out
    }

    static func isLetter(_ c: Character) -> Bool {
        c.unicodeScalars.allSatisfy {
            switch $0.properties.generalCategory {
            case .uppercaseLetter, .lowercaseLetter, .titlecaseLetter, .modifierLetter, .otherLetter:
                return true
            default:
                return false
            }
        }
    }

    static func isDigit(_ c: Character) -> Bool {
        c.unicodeScalars.allSatisfy { $0.properties.generalCategory == .decimalNumber }
    }

    static func simpleIdent(_ s: String) -> Bool {
        for (i, c) in s.enumerated() {
            if !(isLetter(c) || c == "_" || i > 0 && isDigit(c)) {
                return false
            }
        }
        return !s.isEmpty
    }

    static func parseHex(_ hex: String) -> String? {
        var s = hex
        if s.hasPrefix("#") { s = String(s.dropFirst()) }
        if s.count == 3 || s.count == 4 {
            var doubled = ""
            for c in s {
                doubled.append(c)
                doubled.append(c)
            }
            s = doubled
        }
        if s.count != 6 && s.count != 8 { return nil }
        guard UInt32(s, radix: 16) != nil else { return nil }
        return "#" + s.lowercased()
    }

    static func sortedKeys(_ m: PenMap) -> [String] { m.keys.sorted() }

    /// The string of a JSON value, "" for anything else, as Go's v.(string) does.
    static func penString(_ v: Any?) -> String { v as? String ?? "" }

    static func kids(_ m: PenMap) -> [PenMap] {
        let list = m["children"] as? [Any] ?? []
        return list.compactMap { $0 as? PenMap }
    }

    static func splitRef(_ s: String) -> (alias: String, id: String) {
        if let i = s.firstIndex(of: ":") {
            return (String(s[s.startIndex..<i]), String(s[s.index(after: i)...]))
        }
        return ("", s)
    }

    static func penName(_ m: PenMap) -> String {
        let typ = m["type"] as? String ?? ""
        let id = m["id"] as? String ?? ""
        var s = m["name"] as? String ?? ""
        s = s.replacingOccurrences(of: "\n", with: " ").replacingOccurrences(of: "\r", with: " ")
        s = s.trimmingCharacters(in: .whitespacesAndNewlines)
        if s.isEmpty { return typ + " " + id }
        return s
    }

    static func mapMode(_ m: PenMap, _ mode: String) -> String {
        var mode = mode
        if let th = m["theme"] as? PenMap {
            for k in sortedKeys(th) where k.hasSuffix("mode") {
                if let s = th[k] as? String { mode = s }
            }
        }
        return mode
    }

    static func off(_ m: PenMap) -> Bool {
        guard let en = m["enabled"] as? Bool else { return false }
        return !en
    }

    /// The hidden property for a pen enabled key; an override writes both values, a node only yes.
    static func hiddenProp(_ m: PenMap, _ explicit: Bool) -> [PenProp] {
        guard let en = m["enabled"] as? Bool else { return [] }
        if !en { return [prop("hidden", wordArg("yes"))] }
        return explicit ? [prop("hidden", wordArg("no"))] : []
    }

    static func posProps(_ m: PenMap, _ b: PenBCtx) -> [PenProp] {
        let x = m["x"] as? Double ?? 0
        let y = m["y"] as? Double ?? 0
        let abs = !b.top && !b.parentAbs && (m["layoutPosition"] as? String) == "absolute"
        var ps: [PenProp] = []
        if abs {
            ps.append(prop("absolute", wordArg("yes")))
        }
        if b.top || b.parentAbs || abs {
            if x != 0 || y != 0 {
                ps.append(prop("pos", numArg(x), numArg(-y)))
            }
        }
        return ps
    }

    /// The JSON of a .pen file with Go's value types: Bool stays Bool and every number is a Double.
    static func normalizeJSON(_ v: Any) -> Any {
        if let dict = v as? [String: Any] {
            var out = PenMap(minimumCapacity: dict.count)
            for (k, x) in dict where !(x is NSNull) {
                // A null reads as a value that stands for nothing: every type check of the converter
                // falls through it, as a nil map value does in Go.
                out[k] = normalizeJSON(x)
            }
            return out
        }
        if let list = v as? [Any] { return list.map(normalizeJSON) }
        if let n = v as? NSNumber {
            // JSONSerialization hands booleans and numbers alike as NSNumber.
            if CFGetTypeID(n) == CFBooleanGetTypeID() { return n.boolValue }
            return n.doubleValue
        }
        return v
    }

    /// Deep equality of two pen values, as reflect.DeepEqual compares the maps of a .pen.
    static func penEqual(_ a: Any?, _ b: Any?) -> Bool {
        switch (a, b) {
        case (nil, nil): return true
        case let (x as String, y as String): return x == y
        case let (x as Double, y as Double): return x == y
        case let (x as Bool, y as Bool): return x == y
        case let (x as PenMap, y as PenMap):
            if x.count != y.count { return false }
            for (k, v) in x {
                guard let w = y[k] else { return false }
                if !penEqual(v, w) { return false }
            }
            return true
        case let (x as [Any], y as [Any]):
            return x.count == y.count && zip(x, y).allSatisfy { penEqual($0, $1) }
        case let (x as PenFile, y as PenFile): return x === y
        default: return a is NSNull && b is NSNull
        }
    }
}

// MARK: - Reading the files

extension PenConvert {
    func load(_ path: String) throws -> PenFile {
        let abs = URL(fileURLWithPath: path).standardizedFileURL.path
        if let f = files[abs] { return f }
        let data: Data
        do {
            data = try Data(contentsOf: URL(fileURLWithPath: abs))
        } catch {
            throw PenImportFailure(message: "\(abs): \(error.localizedDescription)")
        }
        let parsed: Any
        do {
            parsed = try JSONSerialization.jsonObject(with: data, options: [.fragmentsAllowed])
        } catch {
            throw PenImportFailure(message: "\(abs): \(error.localizedDescription)")
        }
        guard let root = PenImport.normalizeJSON(parsed) as? PenMap else {
            throw PenImportFailure(message: "\(abs): the file is not a JSON object")
        }
        var base = (abs as NSString).lastPathComponent
        if base.hasSuffix(".pen") { base = String(base.dropLast(4)) }
        var aliasName = base
        if aliasName.hasSuffix(".lib") { aliasName = String(aliasName.dropLast(4)) }
        let f = PenFile(path: abs, name: base, alias: PenImport.normalize(aliasName), raw: root)
        f.root = PenNode(kind: .file, file: f)
        files[abs] = f
        if let v = root["variables"] as? PenMap {
            for (k, x) in v {
                if let xm = x as? PenMap { f.vars[k] = xm }
            }
        }
        func walk(_ m: PenMap) {
            if let id = m["id"] as? String { f.rawIdx[id] = m }
            for k in PenImport.kids(m) { walk(k) }
        }
        walk(root)
        if let imp = root["imports"] as? PenMap {
            for alias in PenImport.sortedKeys(imp) {
                guard let rel = imp[alias] as? String else { continue }
                let file = (abs as NSString).deletingLastPathComponent
                let lib: PenFile
                do {
                    lib = try load((file as NSString).appendingPathComponent(rel))
                } catch {
                    throw PenImportFailure(message: "\(abs): import \(alias): \(error)")
                }
                f.imports[alias] = lib
            }
        }
        order.append(f)
        return f
    }
}

// MARK: - Building the item tree

extension PenConvert {
    func slotName(_ f: PenFile, _ name: String) -> String {
        var base = PenImport.normalize(name)
        if base.isEmpty { base = "slot" }
        var s = base
        var i = 2
        while f.slots.contains(s) {
            s = base + "_" + PenImport.fnum(Double(i)).trimmingCharacters(in: .whitespaces)
            i += 1
        }
        f.slots.insert(s)
        return s
    }

    /// Converts one pen node; nil when the node is not part of the scene.
    func buildNode(_ f: PenFile, _ m: PenMap, _ parent: PenNode?, _ bctx: PenBCtx) -> PenNode? {
        var b = bctx
        let id = m["id"] as? String ?? ""
        if PenImport.off(m) && (b.noIndex || b.top) {
            // nothing can switch it on again: a block node has no id to address, a screen is not drawn
            f.dropped.insert(id)
            return nil
        }
        b.mode = PenImport.mapMode(m, b.mode)
        if let r = m["$res"] as? PenFile { b.res = r }
        if b.res == nil { b.res = f }
        let typ = m["type"] as? String ?? ""
        switch typ {
        case "frame", "rectangle", "ellipse", "path", "text", "icon":
            return buildItem(f, m, typ, parent, b)
        case "ref":
            let n = PenNode(kind: .use, file: f)
            n.res = b.res
            n.id = id
            n.type = b.res?.rootType(m) ?? ""
            n.pen = m
            n.parent = parent
            n.name = PenImport.penName(m)
            n.ref = m["ref"] as? String ?? ""
            n.b = b
            if !b.noIndex { f.index[id] = n }
            return n
        default:
            if !id.isEmpty { f.unsupportedById[id] = typ }
            f.unsup("type " + typ)
            return nil
        }
    }

    func buildItem(_ f: PenFile, _ m: PenMap, _ typ: String, _ parent: PenNode?, _ b: PenBCtx) -> PenNode {
        let id = m["id"] as? String ?? ""
        let n = PenNode(kind: .item, file: f)
        n.res = b.res
        n.id = id
        n.type = typ
        n.pen = m
        n.parent = parent
        n.name = PenImport.penName(m)
        n.b = b
        if !b.noIndex { f.index[id] = n }
        n.props = PenImport.hiddenProp(m, false) + props(f, m, typ, PenPctx(mode: b.mode))
        n.props += PenImport.posProps(m, b)
        let cb = PenBCtx(
            mode: b.mode, parentAbs: (m["layout"] as? String) == "none", noIndex: b.noIndex,
            layout: typ == "frame", res: b.res)
        var holder = n
        if m["slot"] is [Any], typ == "frame" {
            let s = PenNode(kind: .slot, file: f)
            s.res = b.res
            s.parent = n
            s.slot = slotName(f, n.name)
            n.kids.append(s)
            holder = s
        }
        for km in PenImport.kids(m) {
            if let k = buildNode(f, km, holder, cb) {
                holder.kids.append(k)
            }
        }
        return n
    }

    /// Makes the item tree of the file; refs are resolved later by finish.
    func buildFile(_ f: PenFile) {
        for (i, m) in PenImport.kids(f.raw).enumerated() {
            if let n = buildNode(f, m, f.root, PenBCtx(mode: mode, top: true)) {
                f.root.kids.append(n)
                f.tops.append(PenTop(index: i, node: n))
            }
        }
    }
}
