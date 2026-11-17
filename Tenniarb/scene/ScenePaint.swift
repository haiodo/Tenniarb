//
//  ScenePaint.swift
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
import cmkdown

extension Scene {
    /// A linear gradient of an item fill, as in CSS: the line runs at angle degrees (0 up, 90 right,
    /// 180 down) through the center of the box and reaches the corner colors.
    struct Gradient {
        var angle = 0.0
        var stops: [Stop] = []

        struct Stop {
            var offset = 0.0
            var color = Scene.Color()
        }

        /// The color that stands for the gradient where one color is needed: the first stop that shows.
        func base() -> Scene.Color {
            stops.first { $0.color.a > 0 }?.color ?? stops[0].color
        }

        /// The line of the gradient through a box of w by h, as gradient.at of paint.go: the angle in
        /// degrees runs 0 up, 90 right, 180 down, and the line reaches the corners of the box.
        func line(_ w: Double, _ h: Double) -> (from: Scene.Point, to: Scene.Point) {
            let a = angle * Double.pi / 180
            let dx = sin(a)
            let dy = -cos(a)
            let half = (abs(w * dx) + abs(h * dy)) / 2
            let cx = w / 2
            let cy = h / 2
            return (Scene.Point(x: cx - dx * half, y: cy - dy * half), Scene.Point(x: cx + dx * half, y: cy + dy * half))
        }
    }

    /// A drop shadow: dx and dy in screen points, y down, as gfx.Shadow.
    struct Shadow: Equatable {
        var dx = 0.0
        var dy = 0.0
        var blur = defaultShadowBlur
        var color = defaultShadowColor
    }

    /// The look of an item, as go/scene/build.go style.
    struct Look {
        var fill = Scene.Color()
        var border = Scene.Color()
        var text = Scene.Color()
        var grad: Gradient?  // set by color linear ...; fill is then its first visible stop
        var shadows: [Shadow] = []  // in file order, the last one on top
        var lineW = 1.0
        var lineCap = ""  // line-cap: butt when empty, else round or square
        var lineJoin = ""  // line-join: miter when empty, else round or bevel
        var dash: [Double]?  // line-style dashed or dotted; nil is a solid line
        var dashPhase = 0.0
        var radius = [0.0, 0.0, 0.0, 0.0]  // top left, top right, bottom right, bottom left
        var radiusDefault = false  // the radius of a classic diagram, which a hairline does not take
        var marker = ""  // the text an old file draws in front of the title, an emoji of a mind map
        var display = ""  // rect when unset; no-fill, circle, stack, text or none
        var layer = 0  // draw order among the siblings
        var clip = false
        var opacity = 1.0
    }
}

/// Defaults of a shadow as in go/render: blur 5, black at 1/3 alpha.
let defaultShadowBlur = 5.0
let defaultShadowColor = Scene.Color(a: 1.0 / 3)
/// The layer of an item among its siblings: background then hover, the zero layer between them.
let layerBackground = 1
let layerHover = 2
/// The border colour of a box of a classic diagram: the generic grey of go/render, #424242.
let classicBorder = Scene.Color(r: 66.0 / 255, g: 66.0 / 255, b: 66.0 / 255, a: 1)

extension Scene.Prep {
    /// Reads color: one color, or `linear <angle> <color> [pos] <color> [pos] ...` with positions 0..1 for
    /// all stops or none (even spread). A bad gradient warns and gives def.
    func fill(_ rn: Scene.RNode, _ def: Scene.Color) -> (Scene.Color, Scene.Gradient?) {
        guard let pr = prop(rn, "color") else { return (def, nil) }
        if pr.text(0) != "linear" {
            return (color(rn, "color", def), nil)
        }
        guard let g = gradient(pr) else { return (def, nil) }
        return (g.base(), g)
    }

    func gradient(_ pr: Scene.Prop) -> Scene.Gradient? {
        func bad(_ message: String) -> Scene.Gradient? {
            t.warn(pr.pos, "color linear: \(message), the default is used")
            return nil
        }
        guard let angle = pr.num(1) else { return bad("the angle is not a number") }
        var g = Scene.Gradient(angle: (angle.truncatingRemainder(dividingBy: 360) + 360).truncatingRemainder(dividingBy: 360))
        var placed = 0
        var i = 2
        while i < pr.args.count {
            let s = pr.text(i) ?? ""
            guard let c = Scene.parseColor(s) else { return bad("\"\(s)\" is not a color") }
            i += 1
            var off = 0.0
            if let v = pr.num(i) {
                off = v
                i += 1
                placed += 1
            }
            g.stops.append(Scene.Gradient.Stop(offset: off, color: c))
        }
        let n = g.stops.count
        switch true {
        case n < 2:
            return bad("needs at least 2 stops")
        case placed == 0:
            for i in 0..<n {
                g.stops[i].offset = Double(i) / Double(n - 1)
            }
        case placed != n:
            return bad("give a position to every stop or to none")
        default:
            break
        }
        for (i, s) in g.stops.enumerated() {
            if s.offset < 0 || s.offset > 1 || i > 0 && s.offset < g.stops[i - 1].offset {
                return bad("positions must grow within 0..1")
            }
        }
        return g
    }

    /// Reads every `shadow dx dy [blur [color]]` of the box, in file order: two of them make a soft one
    /// (a wide faint pass) and a crisp one under it. Y is up as in pos, the screen offset is -dy.
    func shadows(_ rn: Scene.RNode) -> [Scene.Shadow] {
        var out: [Scene.Shadow] = []
        for pr in rn.props where pr.name == "shadow" {
            var pr = pr
            if hasExpr(pr) {
                if !canEval() {
                    if !classic {
                        t.warn(pr.pos, "shadow: expressions are not evaluated")
                    }
                    continue
                }
                guard let evaluated = evalProp(rn, pr) else { continue }
                pr = evaluated
            }
            if let sh = shadowOf(pr) {
                out.append(sh)
            }
        }
        return out
    }

    /// Parses one shadow command: a bad offset drops it, a bad blur or color takes the default.
    func shadowOf(_ pr: Scene.Prop) -> Scene.Shadow? {
        guard let dx = pr.num(0), let dy = pr.num(1) else {
            t.warn(pr.pos, "shadow takes dx dy [blur [color]], numbers for the offset; ignored")
            return nil
        }
        var sh = Scene.Shadow(dx: dx, dy: -dy)
        if pr.args.count > 2 {
            if let v = pr.num(2), v >= 0 {
                sh.blur = v
            } else {
                t.warn(pr.pos, "shadow: the blur is not a number from 0, \(defaultShadowBlur) is used")
            }
        }
        if pr.args.count > 3 {
            let s = pr.text(3) ?? ""
            if let c = Scene.parseColor(s) {
                sh.color = c
            } else {
                t.warn(pr.pos, "shadow: \"\(s)\" is not a color, the default is used")
            }
        }
        if pr.args.count > 4 {
            t.warn(pr.pos, "shadow takes dx dy [blur [color]], the extra values are ignored")
        }
        return sh
    }

    func color(_ rn: Scene.RNode, _ name: String, _ def: Scene.Color) -> Scene.Color {
        guard let pr = prop(rn, name) else { return def }
        let s = pr.text(0) ?? ""
        guard let c = Scene.parseColor(s) else {
            t.warn(pr.pos, "\(name): \"\(s)\" is not a color")
            return def
        }
        return c
    }

    /// Reads line-style: the dashes the renderer of a classic diagram uses, so a border and a link look
    /// the same in both.
    func dash(_ rn: Scene.RNode) -> ([Double]?, Double) {
        guard let pr = prop(rn, "line-style") else { return (nil, 0) }
        switch pr.text(0) ?? "" {
        case "dashed": return ([5, 5], 5)
        case "dotted": return ([1, 4], 1)
        case "solid": return (nil, 0)
        default:
            t.warn(pr.pos, "line-style takes solid, dashed or dotted, got \"\(pr.text(0) ?? "")\"")
            return (nil, 0)
        }
    }

    /// Reads corner-radius: one value for all corners or four in CSS order; anything else uses the first.
    func radii(_ pr: Scene.Prop) -> [Double] {
        let first = pr.num(0) ?? 0
        var out = [first, first, first, first]
        switch pr.args.count {
        case 0, 1:
            break
        case 4:
            for i in 0..<4 {
                guard let v = pr.num(i) else {
                    t.warn(pr.pos, "corner-radius: radius \(i + 1) is not a number, the first value is used for all corners")
                    return [first, first, first, first]
                }
                out[i] = v
            }
        default:
            t.warn(pr.pos, "corner-radius: takes 1 or 4 radii (top left, top right, bottom right, bottom left), the first value is used")
        }
        return out
    }

    /// Reads the look of an item: Tenniarb's defaults are a white fill and a black 1 point border; in a
    /// layout context there is no fill and no border until color or border-color is given.
    func style(_ rn: Scene.RNode, _ inCtx: Bool, _ up: Scene.Color) -> Scene.Look {
        var st = Scene.Look()
        var fill = Scene.Color.white
        var border = Scene.Color.black
        // A picture is the content of the box: the white default would show through a transparent PNG and
        // the black frame around every photo is not what a box of content draws.
        if inCtx || fileImage(rn.prop("image")) {
            fill = Scene.Color()
            border = Scene.Color()
        }
        if classic && !inCtx && border.a > 0 {
            // The border of a classic box is the grey go/render gives it, not the black of a scene.
            border = classicBorder
        }
        (st.fill, st.grad) = self.fill(rn, fill)
        st.shadows = shadows(rn)
        st.border = color(rn, "border-color", border)
        var text = up  // inherited from the parent, black at the root
        if st.fill.a > 0 {
            text = Scene.contrast(st.fill)
        }
        st.text = color(rn, "text-color", text)
        if let pr = prop(rn, "line-cap") {
            let w = pr.text(0) ?? ""
            switch w {
            case "round", "square": st.lineCap = w
            case "butt": break
            default: t.warn(pr.pos, "line-cap takes butt, round or square, got \"\(w)\"")
            }
        }
        if let pr = prop(rn, "line-join") {
            let w = pr.text(0) ?? ""
            switch w {
            case "round", "bevel": st.lineJoin = w
            case "miter": break
            default: t.warn(pr.pos, "line-join takes miter, round or bevel, got \"\(w)\"")
            }
        }
        if let v = num(rn, "line-width") {
            st.lineW = v
        }
        (st.dash, st.dashPhase) = dash(rn)
        if let pr = prop(rn, "corner-radius") {
            st.radius = radii(pr)
        } else if classic {
            // The classic renderer rounds every box by 8 points unless the file says otherwise.
            st.radius = [Scene.classicRadius, Scene.classicRadius, Scene.classicRadius, Scene.classicRadius]
            st.radiusDefault = true
        }
        if let pr = prop(rn, "marker") {
            // go/render draws the marker in front of the title: an emoji of a mind map is the text of its box.
            st.marker = pr.text(0) ?? ""
        }
        if let pr = prop(rn, "display") {
            st.display = pr.text(0) ?? ""
        }
        if let pr = prop(rn, "layer") {
            switch pr.text(0) ?? "" {
            case "background": st.layer = layerBackground
            case "hover": st.layer = layerHover
            default: break
            }
        }
        if let v = num(rn, "opacity") {
            if v < 0 || v > 1 {
                t.warn(prop(rn, "opacity")?.pos ?? rn.pos, "opacity takes 0..1, got \(v)")
            } else {
                st.opacity = v
            }
        }
        if inCtx {
            st.clip = flag(rn, "clip")
        }
        return st
    }

    /// Reads every path of a leaf, in file order, and the view-box of the item: a shape may name its own
    /// ink - color fills it, border-color and line-width stroke it - and takes the one of the item otherwise.
    /// The first path is drawn first, so it lies under the rest. Without view-box the bounds of the first
    /// path stand in.
    func icon(_ rn: Scene.RNode, _ n: Scene.LayoutNode) {
        let list = namedProps(rn, "path")
        if list.isEmpty { return }
        var vx = 0.0
        var vy = 0.0
        var vw = 0.0
        var vh = 0.0
        var box = false
        if let vb = prop(rn, "view-box") {
            var v = [0.0, 0.0, 0.0, 0.0]
            var ok = vb.args.count == 4
            var i = 0
            while ok && i < 4 {
                if let x = vb.num(i) {
                    v[i] = x
                } else {
                    ok = false
                }
                i += 1
            }
            if ok && v[2] > 0 && v[3] > 0 {
                vx = v[0]
                vy = v[1]
                vw = v[2]
                vh = v[3]
                box = true
            } else {
                t.warn(vb.pos, "view-box takes 4 numbers with width and height above 0, the path bounds are used")
            }
        }
        let hasFill = prop(rn, "color") != nil
        let hasStroke = prop(rn, "border-color") != nil
        let (itemFill, itemGrad) = fill(rn, Scene.Color())
        let itemStroke = color(rn, "border-color", Scene.Color())
        var out: [Scene.Icon] = []
        for pr in list {
            let d = pr.text(0) ?? ""
            guard let g = Scene.SVGPath.parse(d), !g.isEmpty else {
                t.warn(pr.pos, "path: not an SVG path, the item is drawn as a box")
                continue
            }
            var ic = Scene.Icon(path: g.path, viewBox: CGRect(x: vx, y: vy, width: vw, height: vh))
            if !box {
                ic.viewBox = g.bounds
            }
            if ic.viewBox.width <= 0 || ic.viewBox.height <= 0 {
                t.warn(pr.pos, "path: empty bounds, give a view-box")
                continue
            }
            // The ink of the command is the whole ink of its shape: a shape that names a fill is filled and not
            // stroked, one that names a stroke is stroked and not filled. Only a shape that names no ink at all
            // takes the one of the item, as a shadow without a colour takes the alpha of the fill.
            let (fillColor, hasOwnFill) = pr.colorOf("color")
            let (strokeColor, hasOwnStroke) = pr.colorOf("border-color")
            if hasOwnFill || hasOwnStroke {
                ic.fill = fillColor
                ic.stroke = strokeColor
            } else if !hasFill && !hasStroke && out.isEmpty {
                ic.fill = styles[ObjectIdentifier(rn)]?.text ?? Scene.Color()
            } else {
                ic.fill = itemFill
                ic.grad = itemGrad
                ic.stroke = itemStroke
            }
            if let v = pr.numOf("line-width"), v >= 0 {
                ic.lineWidth = v
            } else {
                ic.lineWidth = styles[ObjectIdentifier(rn)]?.lineW ?? 1
            }
            if let v = pr.wordOf("fill-rule", ["evenodd", "nonzero"]) {
                ic.evenOdd = v == "evenodd"
            } else if let v = word(rn, "fill-rule", ["evenodd", "nonzero"]) {
                ic.evenOdd = v == "evenodd"
            }
            out.append(ic)
        }
        if out.isEmpty { return }
        icons[ObjectIdentifier(rn)] = out
        n.leafW = { out[0].viewBox.width }
        n.leafH = { _ in out[0].viewBox.height }
    }
}

extension Scene.Prop {
    /// The text of the argument that follows the word name in a command: `path "M…" color #e11` names its ink
    /// that way.
    private func argText(_ name: String) -> String? {
        for (i, a) in args.enumerated() where i + 1 < args.count && a.getIdentText() == name {
            return args[i + 1].getIdentText()
        }
        return nil
    }

    /// The colour written as an argument of the command, found false when it writes none.
    func colorOf(_ name: String) -> (Scene.Color, Bool) {
        guard let text = argText(name), let c = Scene.parseColor(text) else { return (Scene.Color(), false) }
        return (c, true)
    }

    /// The number written as an argument of the command.
    func numOf(_ name: String) -> Double? {
        guard let text = argText(name) else { return nil }
        return Double(text.trimmingCharacters(in: .whitespaces))
    }

    /// The word written as an argument of the command, one of the given.
    func wordOf(_ name: String, _ words: [String]) -> String? {
        guard let text = argText(name), words.contains(text) else { return nil }
        return text
    }
}

extension Scene.Screen {
    /// The text of an item as HTML, for "Export text as html": the title and the body as the canvas lays
    /// them out, so a copy of the text matches the picture on the screen.
    func itemHTML(_ pl: Scene.Placed) -> String {
        let rn = pl.r
        var out = ""
        if let ti = p.texts[ObjectIdentifier(rn)] {
            out += "<div>" + runsHTML(titleRuns(ti)) + "</div>"
        }
        if let body = p.bodies[ObjectIdentifier(rn)] {
            out += blocksHTML(body)
        }
        return out
    }
}

/// The title as one run list: the whole title is one line here, and its parts carry the fonts.
private func titleRuns(_ ti: Scene.TextInfo) -> [Scene.BodyRun] {
    guard let parts = ti.parts else {
        return [Scene.BodyRun(text: ti.text, font: ti.font)]
    }
    return parts.map { part in
        if part.img != nil {
            return Scene.BodyRun(text: "[image]", font: part.font)
        }
        return Scene.BodyRun(text: part.text, font: ti.partFont(part), color: part.color)
    }
}

/// The body of an item as HTML, a block at a time.
private func blocksHTML(_ body: Scene.BodyText) -> String {
    var out = ""
    for blk in body.blocks {
        switch blk.kind {
        case .heading:
            let level = blk.level >= 1 && blk.level <= 6 ? blk.level : 1
            out += "<h\(level)>" + runsHTML(blk.runs) + "</h\(level)>"
        case .paragraph:
            out += "<div>" + runsHTML(blk.runs) + "</div>"
        case .quote:
            out += "<blockquote>" + runsHTML(blk.runs) + "</blockquote>"
        case .list:
            out += "<ul>"
            for item in blk.items {
                out += "<li>" + runsHTML(item) + "</li>"
            }
            out += "</ul>"
        case .code:
            out += "<pre><code>" + escapeHTML(blk.code.joined(separator: "\n")) + "</code></pre>"
        case .toc:
            break  // the table of contents repeats the headings of the body
        }
    }
    return out
}

/// A run of text as HTML: the style of the run wraps its words, and the links nest inside.
private func runsHTML(_ runs: [Scene.BodyRun]) -> String {
    var out = ""
    for r in runs where !r.text.isEmpty {
        var text = escapeHTML(r.text)
        var style = ""
        if r.color.a > 0 {
            style += String(format: "color:#%02x%02x%02x;", to8(r.color.r), to8(r.color.g), to8(r.color.b))
        }
        if r.font.size > 0 {
            style += String(format: "font-size:%gpx;", r.font.size)
        }
        if !style.isEmpty {
            text = "<span style=\"" + style + "\">" + text + "</span>"
        }
        if r.code {
            text = "<code>" + text + "</code>"
        }
        if r.font.italic {
            text = "<i>" + text + "</i>"
        }
        if r.font.weight >= 600 {
            text = "<b>" + text + "</b>"
        }
        if !r.link.isEmpty {
            text = "<a href=\"" + escapeHTML(r.link) + "\">" + text + "</a>"
        }
        out += text
    }
    return out
}

/// The five characters html.EscapeString of Go turns into entities, spelled as it spells them.
private func escapeHTML(_ s: String) -> String {
    s.replacingOccurrences(of: "&", with: "&amp;")
        .replacingOccurrences(of: "'", with: "&#39;")
        .replacingOccurrences(of: "<", with: "&lt;")
        .replacingOccurrences(of: ">", with: "&gt;")
        .replacingOccurrences(of: "\"", with: "&#34;")
}

/// A colour channel of 0...1 as the two digits of a hex colour.
private func to8(_ v: Double) -> Int32 {
    if v <= 0 {
        return 0
    }
    if v >= 1 {
        return 255
    }
    return Int32(v * 255 + 0.5)
}
