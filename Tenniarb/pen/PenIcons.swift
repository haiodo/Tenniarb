//
//  PenIcons.swift
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

// A pen icon node names an icon of a library and carries no geometry of its own: the drawing is the
// vector of that name. go/scene draws a leaf with a `path` and a `view-box` and knows no icon property,
// so the converter writes the path of the glyph into the item, and the .tenn file stays readable on a
// machine without the icons library.
//
// The glyphs are the items of libraries/icons.tenn, the library of the lucide set the application ships:
// the same file a document mounts with --from-file "tenniarb-library://icons".

/// One shape of a glyph: called means the library fills it instead of stroking it.
struct PenGlyphPath {
    var d: String
    var filled: Bool
}

/// The drawing of one item of the icons library.
struct PenGlyph {
    var paths: [PenGlyphPath] = []
    var viewBox: [Double]
    var stroke: Double
}

/// libraries/icons.tenn read once: its glyphs, the view box and stroke of the set, and the error of
/// reading it.
struct PenIconTable {
    /// The view box and stroke of the lucide set, used when the library cannot be read: the converter
    /// still writes the room of an icon.
    static let defaultViewBox = [0.0, 0.0, 24.0, 24.0]
    static let defaultStroke = 2.0

    var glyphs: [String: PenGlyph] = [:]
    var viewBox = defaultViewBox
    var stroke = defaultStroke
    var error: String?
}

enum PenIcons {
    /// The library is read once per process: the converter and the scene take the icons from one file.
    static let table: PenIconTable = read()

    /// Reads the items of the library: one item per icon, with its path, view box and stroke. The text is
    /// written by cmd/tennicons, and the parser of the format reads it like any other .tenn file.
    static func read() -> PenIconTable {
        var set = PenIconTable()
        let workspace = URL(fileURLWithPath: FileManager.default.currentDirectoryPath)
        let file: URL
        do {
            file = try Library.default(workspace: workspace).resolve(Library.Path(name: "icons"))
        } catch {
            set.error = "the icons library is not readable: \(error)"
            return set
        }
        let src: String
        do {
            src = try String(contentsOf: file, encoding: .utf8)
        } catch {
            set.error = "the icons library is not readable: \(error)"
            return set
        }
        do {
            let glyphs = try parse(src)
            set.glyphs = glyphs
            // every item of the set carries the same box and stroke
            if let g = glyphs.values.first {
                set.viewBox = g.viewBox
                set.stroke = g.stroke
            }
            return set
        } catch {
            set.error = "libraries/icons.tenn: \(error)"
            return set
        }
    }

    static func parse(_ src: String) throws -> [String: PenGlyph] {
        let parser = TennParser()
        let ast = parser.parse(src)
        if let e = parser.errors.errors.first {
            throw PenImportFailure(message: "line \(e.line + 1): \(e.message)")
        }
        var glyphs: [String: PenGlyph] = [:]
        for c in ast.children ?? [] where c.kind == .Command {
            let name = c.getIdent(0) ?? ""
            if name != "item" { continue }
            let (args, block) = splitCommand(c)
            guard let item = args.first?.getIdentText() else { continue }
            var g = PenGlyph(viewBox: PenIconTable.defaultViewBox, stroke: PenIconTable.defaultStroke)
            for p in block where p.kind == .Command {
                let pname = p.getIdent(0) ?? ""
                let pargs = splitCommand(p).args
                switch pname {
                case "path":
                    if let d = argIdentAt(pargs, 0) {
                        // `path "M…" color #000` fills its shape instead of stroking it
                        g.paths.append(PenGlyphPath(d: d, filled: cmdHasWord(pargs, "color")))
                    }
                case "view-box":
                    for i in 0..<4 {
                        if let v = argNumAt(pargs, i) { g.viewBox[i] = v }
                    }
                case "line-width":
                    if let v = argNumAt(pargs, 0) { g.stroke = v }
                default:
                    break
                }
            }
            if !item.isEmpty && !g.paths.isEmpty { glyphs[item] = g }
        }
        if glyphs.isEmpty { throw PenImportFailure(message: "no icons in the library") }
        return glyphs
    }

    /// Separates the arguments of a command from the commands of its block.
    static func splitCommand(_ c: TennNode) -> (args: [TennNode], block: [TennNode]) {
        var args: [TennNode] = []
        var block: [TennNode] = []
        for a in (c.children ?? []).dropFirst() {
            if a.kind == .BlockExpr {
                block += a.children ?? []
            } else {
                args.append(a)
            }
        }
        return (args, block)
    }

    /// Reports whether a command names the word among its arguments.
    static func cmdHasWord(_ args: [TennNode], _ word: String) -> Bool {
        args.contains { $0.getIdentText() == word }
    }

    static func argIdentAt(_ args: [TennNode], _ i: Int) -> String? {
        i < args.count ? args[i].getIdentText() : nil
    }

    static func argNumAt(_ args: [TennNode], _ i: Int) -> Double? {
        guard let s = argIdentAt(args, i) else { return nil }
        return Double(s)
    }
}

extension PenConvert {
    /// The fill colour of an icon: no gradient is stored, the glyph is drawn with its own shapes.
    func iconFill(_ f: PenFile, _ v: Any?, _ mode: String) -> String? {
        var noGradient: [PenArg]?
        return paint(f, v, mode, false, true, &noGradient)
    }

    /// The look of a pen icon: the path of the glyph, its view box, the stroke the pen fill colours and
    /// the width of that stroke in the units of the item.
    ///
    /// A ref to an icon component carries an icon name of its own - the instances of one size preset show
    /// different glyphs - so the same function writes the override of a use: path, view box and stroke.
    func iconProps(_ f: PenFile, _ m: PenMap, _ x: PenPctx) -> [PenProp] {
        if let lib = m["library"] as? String, !lib.isEmpty, lib != "lucide" {
            f.unsup("icon library " + lib)
            return []
        }
        let set = PenIcons.table
        if let err = set.error {
            f.iconLibraryUnreadable(err)
            return []
        }
        let name = m["icon"] as? String ?? ""
        var ps: [PenProp] = []
        var box = set.viewBox
        var stroke = set.stroke
        if !name.isEmpty {
            guard let g = set.glyphs[name] else {
                f.iconNotFound(name)
                return []
            }
            box = g.viewBox
            stroke = g.stroke
            let fill = iconFill(f, m["fill"], x.mode)
            let width = iconStroke(box, stroke, m, x)
            ps.append(
                PenImport.prop(
                    "view-box", PenImport.numArg(box[0]), PenImport.numArg(box[1]), PenImport.numArg(box[2]),
                    PenImport.numArg(box[3])))
            // The contour is stroked in the colour of the pen fill - lucide draws every icon with
            // stroke=currentColor and no fill - and a filled detail takes that colour as a fill.
            for gp in g.paths {
                if gp.filled {
                    if let fill {
                        ps.append(PenImport.prop("path", PenImport.strArg(gp.d)))
                        ps.append(PenImport.prop("color", PenImport.wordArg(fill)))
                    }
                    continue
                }
                ps.append(PenImport.prop("path", PenImport.strArg(gp.d)))
                if let fill {
                    ps.append(PenImport.prop("border-color", PenImport.wordArg(fill)))
                }
                if let width {
                    ps.append(PenImport.prop("line-width", PenImport.numArg(width)))
                }
            }
            return ps
        }
        if let fill = iconFill(f, m["fill"], x.mode) {
            ps.append(PenImport.prop("border-color", PenImport.wordArg(fill)))
        }
        if let w = iconStroke(box, stroke, m, x) {
            ps.append(PenImport.prop("line-width", PenImport.numArg(w)))
        }
        return ps
    }

    /// The width of the stroke of the glyph in the units of the item. go/scene scales the view box to the
    /// box of the item but strokes the scaled path with a width of the item, so the width of the library
    /// has to be scaled by hand, or a small icon draws with a stroke too thick for it.
    ///
    /// An override writes the width only when it changes the size of the instance: otherwise the component
    /// already carries the width of that size.
    func iconStroke(_ box: [Double], _ stroke: Double, _ m: PenMap, _ x: PenPctx) -> Double? {
        var src = m
        let vw = box[2]
        let vh = box[3]
        if x.explicit {
            let hasW = m["width"] != nil
            let hasH = m["height"] != nil
            if !hasW && !hasH { return nil }
            if let eff = x.eff { src = eff }
        }
        var s = 1.0  // the icon takes the size of its view box when the box of the item is not written
        if let iw = src["width"] as? Double, iw > 0, let ih = src["height"] as? Double, ih > 0 {
            s = min(iw / vw, ih / vh)
        }
        // Two decimals are plenty for the width of a stroke and keep the numbers of a .tenn short.
        return (stroke * s * 100).rounded() / 100
    }
}

extension PenFile {
    /// Warns once per file when the library itself is missing: every icon of the file would otherwise be
    /// an empty item with no word about the cause.
    func iconLibraryUnreadable(_ message: String) {
        let key = "icons library"
        if once.contains(key) { return }
        once.insert(key)
        warn("\(message): the icons keep their room and are left empty")
    }

    /// Warns once per name, not once per use: a screen may hold a hundred instances of a component whose
    /// glyph the library does not have, and one line about the name says it.
    func iconNotFound(_ name: String) {
        if missingIcons.contains(name) { return }
        missingIcons.insert(name)
        warn("icon \"\(name)\" is not in the icons library (libraries/icons.tenn): the item keeps its room and is left empty")
    }
}
