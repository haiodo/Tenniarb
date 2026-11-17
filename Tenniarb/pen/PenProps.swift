//
//  PenProps.swift
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

/// How props reads a pen map. An override map (explicit) writes only the keys it has, including values
/// equal to the default, because they may restore what the component changed.
struct PenPctx {
    var mode: String
    var explicit = false
    /// The pen map with the overrides applied, for values derived from several keys.
    var eff: PenMap?
}

extension PenImport {
    static func mod360(_ v: Double) -> Double {
        let r = v.truncatingRemainder(dividingBy: 360)
        let s = (r + 360).truncatingRemainder(dividingBy: 360)
        return s
    }

    static func toStr(_ v: Any?) -> String { v as? String ?? "" }

    static func hasProp(_ ps: [PenProp], _ name: String) -> Bool { ps.contains { $0.name == name } }
}

extension PenConvert {
    /// Resolves a hex literal or a $variable to #rrggbb[aa] for the mode. `themed` says the place holds an
    /// expression, so a variable the file writes per theme stays switchable through the context.
    func color(_ f: PenFile, _ s: String, _ mode: String, _ themed: Bool) -> String? {
        if !s.hasPrefix("$") { return PenImport.parseHex(s) }
        var name = String(s.dropFirst())
        if let i = name.firstIndex(of: ":") { name = String(name[name.index(after: i)...]) }
        guard let (owner, v) = variable(f, name) else {
            f.unsup("unknown variable")
            return nil
        }
        f.variables[name, default: 0] += 1
        if v["value"] is [Any], themed {
            if let expr = themedColor(owner, v) { return expr }
        }
        switch v["value"] {
        case let value as String:
            return color(f, value, mode, themed)
        case let list as [Any]:
            var pick = ""
            for it in list {
                let im = it as? PenMap
                let th = im?["theme"] as? PenMap ?? [:]
                let cs = im?["value"] as? String ?? ""
                if pick.isEmpty { pick = cs }
                for tv in th.values where (tv as? String) == mode { pick = cs }
            }
            return color(f, pick, mode, themed)
        default:
            return nil
        }
    }

    /// Looks the name up in the file and in its libraries, and returns the file that declares it.
    func variable(_ f: PenFile, _ name: String) -> (PenFile, PenMap)? {
        if let v = f.vars[name] { return (f, v) }
        for a in PenImport.sortedKeys(f.imports) {
            if let lib = f.imports[a], let v = lib.vars[name] { return (lib, v) }
        }
        return nil
    }

    /// The color of a fill or stroke value; gradients give their first stop. With wantGradient (the fill
    /// of an item) a linear gradient is also stored there as the value of color, in full.
    func paint(
        _ f: PenFile, _ v: Any?, _ mode: String, _ wantGradient: Bool, _ themed: Bool,
        _ gradient: inout [PenArg]?
    ) -> String? {
        switch v {
        case let s as String:
            return color(f, s, mode, themed)
        case let list as [Any]:
            if list.count > 1 { f.unsup("several fills (first used)") }
            if let first = list.first {
                return paint(f, first, mode, wantGradient, themed, &gradient)
            }
        case let m as PenMap:
            if let en = m["enabled"] as? Bool, !en { return nil }
            if (m["type"] as? String) == "shader" {
                // the shader program and its uniforms have no counterpart: the url names what to draw by hand
                f.unsup("fill shader " + PenImport.toStr(m["url"]))
                return nil
            }
            if (m["type"] as? String) != "gradient" {
                f.unsup("fill " + PenImport.toStr(m["type"]))
                return nil
            }
            if wantGradient, let args = linearGradient(f, m, mode) {
                gradient = args
                return args[2].s
            }
            f.unsup("gradient fill (first stop)")
            for ci in m["colors"] as? [Any] ?? [] {
                let cm = ci as? PenMap
                if let col = color(f, cm?["color"] as? String ?? "", mode, false) { return col }
            }
        default:
            break
        }
        return nil
    }

    /// The value of color for a pen linear gradient: "linear angle color pos ...". The pen rotation is
    /// degrees counter-clockwise from up and the angle here is CSS degrees (clockwise from up), the
    /// direction from the first stop to the last. A gradient with a center or a length of its own, another
    /// kind, or a stop that is not a color gives nil.
    func linearGradient(_ f: PenFile, _ m: PenMap, _ mode: String) -> [PenArg]? {
        if let k = m["gradientType"] as? String, k != "linear" { return nil }
        if m["center"] != nil { return nil }
        if let sz = m["size"] as? PenMap {
            if let h = sz["height"] as? Double, h != 1 { return nil }
            if sz["width"] != nil { return nil }
        }
        let rot = m["rotation"] as? Double ?? 0
        let cols = m["colors"] as? [Any] ?? []
        if cols.count < 2 { return nil }
        var args = [PenImport.wordArg("linear"), PenImport.numArg(PenImport.mod360(360 - rot))]
        for ci in cols {
            let cm = ci as? PenMap
            let pos = cm?["position"] as? Double
            let col = color(f, cm?["color"] as? String ?? "", mode, false)
            guard let pos, let col else { return nil }
            args.append(PenImport.wordArg(col))
            args.append(PenImport.numArg(pos))
        }
        return args
    }

    func sizeProp(_ f: PenFile, _ name: String, _ v: Any?, _ explicit: Bool) -> [PenProp] {
        switch v {
        case let x as Double:
            return [PenImport.prop(name, PenImport.numArg(x))]
        case let x as String:
            var word = "hug"
            if x.hasPrefix("fill_container") { word = "fill" }
            if let i = x.firstIndex(of: "("), String(x[i...]) != "(0)" {
                var head = String(x[..<i])
                if head.hasSuffix("(") { head = String(head.dropLast()) }
                f.unsup("size fallback in " + head)
            }
            if word == "hug" && !explicit { return [] }
            return [PenImport.prop(name, PenImport.wordArg(word))]
        default:
            return []
        }
    }

    func padArgs(_ v: Any?) -> [PenArg]? {
        switch v {
        case let p as Double:
            return [PenImport.numArg(p)]
        case let list as [Any]:
            return list.map { PenImport.numArg($0 as? Double ?? 0) }
        default:
            return nil
        }
    }

    /// The width a per-side pen stroke gives each of the four sides, a side the file leaves out counting
    /// as 0: that width when it is the same on every side (0 for none), and false when the sides differ
    /// or one is not a number.
    func perSideWidth(_ m: PenMap) -> (w: Double, same: Bool) {
        var w = 0.0
        for (i, k) in ["top", "right", "bottom", "left"].enumerated() {
            let has = m[k] != nil
            let n = m[k] as? Double
            if has && n == nil { return (0, false) }
            let value = n ?? 0
            if i > 0 && value != w { return (0, false) }
            w = value
        }
        return (w, true)
    }

    func alignWord(_ f: PenFile, _ key: String, _ v: Any?) -> String? {
        guard let s = v as? String else { return nil }
        switch s {
        case "center", "end":
            return s
        case "space_between":
            return "space-between"
        case "space_around":
            f.unsup(key + " space_around")
            return nil
        default:
            return nil
        }
    }

    func layoutProps(_ f: PenFile, _ m: PenMap, _ x: PenPctx) -> [PenProp] {
        var ps: [PenProp] = []
        let lay = m["layout"]
        if lay != nil || !x.explicit {
            var dir = "row"
            switch lay as? String {
            case "vertical": dir = "column"
            case "none": dir = "abs"
            default: break
            }
            ps.append(PenImport.prop("auto-layout", PenImport.wordArg(dir)))
        }
        if let g = m["gap"] as? Double, g != 0 || x.explicit {
            ps.append(PenImport.prop("gap", PenImport.numArg(g)))
        }
        if let a = padArgs(m["padding"]) {
            ps.append(PenProp(name: "padding", args: a))
        }
        for (key, name) in [("alignItems", "align"), ("justifyContent", "justify")] {
            guard let v = m[key] else { continue }
            if let w = alignWord(f, key, v) {
                ps.append(PenImport.prop(name, PenImport.wordArg(w)))
            } else if x.explicit && (v as? String) == "start" {
                ps.append(PenImport.prop(name, PenImport.wordArg("start")))
            }
        }
        return ps
    }

    func radiusProp(_ f: PenFile, _ v: Any?) -> [PenProp] {
        switch v {
        case let r as Double:
            if r > 0 { return [PenImport.prop("corner-radius", PenImport.numArg(r))] }
        case let list as [Any]:
            if list.isEmpty { return [] }
            let nums = list.map { $0 as? Double ?? 0 }
            let same = nums.allSatisfy { $0 == nums[0] }
            let args = same ? [PenImport.numArg(nums[0])] : nums.map { PenImport.numArg($0) }
            if args[0].num > 0 || args.count > 1 {
                return [PenProp(name: "corner-radius", args: args)]
            }
        default:
            break
        }
        return []
    }

    /// Writes a pen opacity; an override writes 1 too, it may restore what the component dimmed.
    func opacityProp(_ f: PenFile, _ v: Any?, _ explicit: Bool) -> [PenProp] {
        guard let n = v as? Double else {
            f.unsup("opacity")
            return []
        }
        if n < 0 || n > 1 {
            f.unsup("opacity")
            return []
        }
        if n < 1 || explicit { return [PenImport.prop("opacity", PenImport.numArg(n))] }
        return []
    }

    func shadowProp(_ f: PenFile, _ v: Any?, _ mode: String) -> [PenProp] {
        var m: PenMap?
        switch v {
        case let x as PenMap:
            m = x
        case let list as [Any]:
            for it in list {
                guard let im = it as? PenMap else { continue }
                if m == nil && (im["type"] as? String) == "shadow" && (im["shadowType"] as? String) != "inner" {
                    m = im
                } else {
                    f.unsup("effect " + PenImport.toStr(im["type"]) + " " + PenImport.toStr(im["shadowType"]))
                }
            }
        default:
            break
        }
        guard let m else { return [] }
        if (m["type"] as? String) != "shadow" || (m["shadowType"] as? String) == "inner" {
            f.unsup("effect " + PenImport.toStr(m["type"]) + " " + PenImport.toStr(m["shadowType"]))
            return []
        }
        var dx = 0.0
        var dy = 0.0
        if let off = m["offset"] as? PenMap {
            dx = off["x"] as? Double ?? 0
            dy = off["y"] as? Double ?? 0
        }
        let blur = m["blur"] as? Double ?? 0
        var col = "#00000040"
        if let cs = m["color"] as? String, let cc = color(f, cs, mode, false) { col = cc }
        // Y is up in .tenn as in pos, pen's offset is down.
        return [
            PenImport.prop(
                "shadow", PenImport.numArg(dx), PenImport.numArg(-dy), PenImport.numArg(blur),
                PenImport.wordArg(col))
        ]
    }

    /// The picture a pen fill names, as the image property of the item: .tenn has a picture on an item,
    /// not among its paints, so a fill image becomes the content of the box and the item keeps no fill
    /// color. A text or a path takes its fill as ink, and a fill mode the format does not know keeps the
    /// warning.
    func imageFill(_ f: PenFile, _ m: PenMap, _ typ: String) -> [PenProp]? {
        if typ == "text" || typ == "path" { return nil }
        guard let fill = m["fill"] as? PenMap, (fill["type"] as? String) == "image" else { return nil }
        if let en = fill["enabled"] as? Bool, !en { return nil }
        guard let url = fill["url"] as? String, !url.isEmpty else { return nil }
        // The url is written as the .pen has it, relative to the design, so a .tenn written beside its
        // design reads the picture; elsewhere the path names no file and the scene warns when it loads.
        let mode = PenImport.toStr(fill["mode"])
        let fits = ["fill": "fill", "cover": "cover", "fit": "contain", "": "fill"]
        guard let fit = fits[mode] else {
            f.unsup("fill image mode " + mode)
            return nil
        }
        return [PenImport.prop("image", PenImport.strArg(url), PenImport.wordArg("fit"), PenImport.wordArg(fit))]
    }

    /// The fill and the border of an item. go/scene reads display only on a link, so none is written.
    func lookProps(_ f: PenFile, _ m: PenMap, _ typ: String, _ x: PenPctx) -> [PenProp] {
        var grad: [PenArg]?
        let wantGradient = typ != "text" && typ != "path"  // a text color and an icon fill are one color
        let img = imageFill(f, m, typ)
        let fillVal: Any? = img != nil ? nil : m["fill"]
        let fill = paint(f, fillVal, x.mode, wantGradient, true, &grad)
        let stroke = paint(f, m["stroke"], x.mode, false, true, &grad)
        let hasFill = fill != nil
        let hasStroke = stroke != nil
        var fillArgs = [PenImport.wordArg(fill ?? "")]
        if let grad { fillArgs = grad }
        var sw = m["strokeWidth"] as? Double
        var hasWidth = sw != nil
        if let sides = m["strokeWidth"] as? PenMap {
            let (w, same) = perSideWidth(sides)
            if same {
                sw = w  // the same width on every side is a plain border
                hasWidth = true
            } else {
                // .tenn draws a border on all four sides of the box or none, so the sides the file leaves
                // out get a line too
                f.unsup("per-side stroke width (a 1 point border on all four sides is drawn)")
            }
        }
        if hasStroke && !hasWidth { sw = 1 }
        let strokeText = stroke ?? ""
        // A colour of no alpha draws nothing, and a path is stroked along its geometry, centered as pen
        // draws it: neither loses the alignment.
        if hasStroke, let width = sw, width > 0, typ != "path",
            !(strokeText.utf8.count == 9 && strokeText.hasSuffix("00"))
        {
            // .tenn draws the stroke inside the box; pen puts it on the edge, so half of a centered stroke
            // and all of an outer one move into the box
            var al = PenImport.toStr(m["strokeAlignment"])
            if al.isEmpty { al = "center" }
            if al != "inner" {
                f.unsup("stroke alignment " + al + " (.tenn draws the stroke inside the box)")
            }
        }
        if x.explicit {
            return explicitLook(typ, m, x.eff, fillArgs, strokeText, sw ?? 0, hasFill, hasStroke) + (img ?? [])
        }
        switch typ {
        case "text":
            if let fill { return [PenImport.prop("text-color", PenImport.wordArg(fill))] }
            return []
        case "path":
            return pathLook(f, m, fill ?? "", strokeText, sw ?? 0, hasFill, hasStroke)
        default:
            break
        }
        var ps: [PenProp] = []
        if typ == "ellipse" {
            // go/scene has no ellipse; a radius of half the width is what gfx/pen does too
            if let w = m["width"] as? Double {
                ps.append(PenImport.prop("corner-radius", PenImport.numArg(w / 2)))
            } else {
                f.unsup("ellipse of flexible width")
            }
        }
        if hasFill {
            ps.append(PenProp(name: "color", args: fillArgs))
        } else if typ == "ellipse" {
            ps.append(PenImport.prop("color", PenImport.wordArg("#00000000")))
        }
        if hasStroke {
            ps.append(PenImport.prop("border-color", PenImport.wordArg(strokeText)))
            ps.append(PenImport.prop("line-width", PenImport.numArg(sw ?? 0)))
        } else if hasFill || typ == "ellipse" {
            ps.append(PenImport.prop("line-width", PenImport.numArg(0)))
        }
        return ps + (img ?? [])
    }

    /// Maps the pen viewBox onto the box. Without one pen draws the path unscaled, which is a view box of
    /// the size of the node.
    func viewBoxProp(_ m: PenMap) -> [PenProp] {
        if let vb = m["viewBox"] as? [Any], vb.count == 4 {
            let args = vb.map { PenImport.numArg($0 as? Double ?? 0) }
            return [PenProp(name: "view-box", args: args)]
        }
        if let w = m["width"] as? Double, let h = m["height"] as? Double {
            return [
                PenImport.prop(
                    "view-box", PenImport.numArg(0), PenImport.numArg(0), PenImport.numArg(w), PenImport.numArg(h))
            ]
        }
        return []
    }

    /// The look of a pen path as a vector icon; nil when pen draws nothing.
    func pathLook(
        _ f: PenFile, _ m: PenMap, _ fill: String, _ stroke: String, _ sw: Double, _ hasFill: Bool,
        _ hasStroke: Bool
    ) -> [PenProp] {
        let g = m["geometry"] as? String ?? ""
        if g.isEmpty || !hasFill && !hasStroke { return [] }
        var ps = [PenImport.prop("path", PenImport.strArg(g))] + viewBoxProp(m)
        if (m["fillRule"] as? String) == "evenodd" {
            ps.append(PenImport.prop("fill-rule", PenImport.wordArg("evenodd")))
        }
        if hasFill {
            ps.append(PenImport.prop("color", PenImport.wordArg(fill)))
        }
        if hasStroke {
            ps.append(PenImport.prop("border-color", PenImport.wordArg(stroke)))
            ps.append(PenImport.prop("line-width", PenImport.numArg(sw)))
        }
        return ps
    }

    func explicitLook(
        _ typ: String, _ m: PenMap, _ eff: PenMap?, _ fill: [PenArg], _ stroke: String, _ sw: Double,
        _ hasFill: Bool, _ hasStroke: Bool
    ) -> [PenProp] {
        var ps: [PenProp] = []
        if let g = m["geometry"] as? String, typ == "path" {
            ps.append(PenImport.prop("path", PenImport.strArg(g)))
            if m["viewBox"] != nil || eff?["viewBox"] == nil {
                ps += viewBoxProp(PenImport.mergeMaps(eff ?? [:], m))
            }
        } else if m["viewBox"] != nil, typ == "path" {
            ps += viewBoxProp(m)
        }
        if typ == "path" && (m["fillRule"] as? String) == "evenodd" {
            ps.append(PenImport.prop("fill-rule", PenImport.wordArg("evenodd")))
        }
        if m["fill"] != nil && hasFill {
            let name = typ == "text" ? "text-color" : "color"
            ps.append(PenProp(name: name, args: fill))
        }
        if m["stroke"] != nil && hasStroke {
            ps.append(PenImport.prop("border-color", PenImport.wordArg(stroke)))
        }
        if m["strokeWidth"] != nil || hasStroke {
            ps.append(PenImport.prop("line-width", PenImport.numArg(sw)))
        }
        return ps
    }

    func countIgnored(_ f: PenFile, _ m: PenMap) {
        for k in ["rotation", "flipX", "flipY", "strokeLinecap", "strokeLinejoin", "metadata", "layoutIncludeStroke"] {
            guard let v = m[k] else { continue }
            if let n = v as? Double, n == 0 { continue }
            if let b = v as? Bool, !b { continue }
            f.unsup(k)
        }
    }

    /// Maps the font keys present in m; overrides carry only the keys they change, so defaults are written
    /// only there, to reset what the component sets.
    func fontProps(_ m: PenMap, _ explicit: Bool) -> [PenProp] {
        var ps: [PenProp] = []
        if let s = m["fontFamily"] as? String, !s.isEmpty {
            ps.append(PenImport.prop("font-family", PenImport.strArg(s)))
        }
        switch m["fontWeight"] {
        case let w as String:
            if explicit || w != "normal" && w != "400" {
                ps.append(PenImport.prop("font-weight", PenImport.wordArg(w)))
            }
        case let w as Double:
            if explicit || w != 400 {
                ps.append(PenImport.prop("font-weight", PenImport.numArg(w)))
            }
        default:
            break
        }
        if let s = m["fontStyle"] as? String, !s.isEmpty, explicit || s != "normal" {
            ps.append(PenImport.prop("font-style", PenImport.wordArg(s)))
        }
        if let n = m["letterSpacing"] as? Double, explicit || n != 0 {
            ps.append(PenImport.prop("letter-spacing", PenImport.numArg(n)))
        }
        return ps
    }

    func textProps(_ f: PenFile, _ m: PenMap, _ x: PenPctx) -> [PenProp] {
        var ps: [PenProp] = []
        // A text grows one way in the file: `fixed-width` wraps inside the width it names, `auto` takes
        // the width of its words. A component of a text is the first kind and carries the width of its
        // sample, so an instance that grows of itself must say so - otherwise it takes the width of the
        // sample and sticks out of the frame it was placed in.
        if let g = m["textGrowth"] as? String, g == "auto" {
            ps.append(PenImport.prop("width", PenImport.wordArg("hug")))
        }
        if let s = m["content"] as? String {
            ps.append(PenImport.prop("title", PenImport.strArg(s)))
        } else if m["content"] != nil {
            f.unsup("text content other than a string")
        }
        let hasSize = m["fontSize"] != nil
        let hasLH = m["lineHeight"] != nil
        if hasSize || hasLH || !x.explicit {
            let eff = x.eff ?? m
            var size = eff["fontSize"] as? Double ?? 0
            if size == 0 { size = 14 }
            if size != 18 || x.explicit {
                ps.append(PenImport.prop("font-size", PenImport.numArg(size)))
            }
            // pen.dev rounds the line box to whole units.
            if let lh = eff["lineHeight"] as? Double, lh > 0 {
                ps.append(PenImport.prop("line-height", PenImport.numArg((lh * size).rounded())))
            }
        }
        ps += fontProps(m, x.explicit)
        // A leaf is centered in go/scene, left in pen.dev.
        switch m["textAlign"] as? String {
        case "center":
            ps.append(PenImport.prop("layout", PenImport.wordArg("middle")))
        case "right":
            ps.append(PenImport.prop("layout", PenImport.wordArg("right")))
        default:
            if m["textAlign"] != nil || !x.explicit {
                ps.append(PenImport.prop("layout", PenImport.wordArg("left")))
            }
        }
        return ps
    }

    /// Converts the keys of a pen map of the given type; position is up to the caller.
    func props(_ f: PenFile, _ m: PenMap, _ typ: String, _ x: PenPctx) -> [PenProp] {
        countIgnored(f, m)
        var ps: [PenProp] = []
        if !x.explicit {
            ps += PenImport.themeProps(m)
        }
        if typ == "frame" {
            ps += layoutProps(f, m, x)
        }
        if typ == "text" {
            ps += textProps(f, m, x)
            if !x.explicit {
                ps.append(PenImport.prop("auto-layout", PenImport.wordArg("row")))
            }
        }
        for k in ["width", "height"] {
            if let v = m[k] {
                ps += sizeProp(f, k, v, x.explicit)
            }
        }
        if let b = m["clip"] as? Bool, b || x.explicit {
            ps.append(PenImport.prop("clip", PenImport.wordArg(b ? "yes" : "no")))
        }
        if typ == "icon" {
            // An icon has no box and no border of its own: its fill is the colour of the glyph, which is
            // the stroke of the path, and the path itself is the drawing.
            ps += iconProps(f, m, x)
        } else {
            ps += lookProps(f, m, typ, x)
        }
        if let o = m["opacity"] {
            ps += opacityProp(f, o, x.explicit)
        }
        if let r = m["cornerRadius"] {
            ps += radiusProp(f, r)
        }
        if let e = m["effect"] {
            ps += shadowProp(f, e, x.mode)
        }
        return ps
    }
}
