//
//  SceneSVG.swift
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

import CoreGraphics
import Foundation

/// Writes a built screen as an SVG document of the still picture ScenePainter draws: the same items in
/// z order, the same fills, borders, shadows, text and links, with the y going down as on the canvas.
enum SceneSVG {
    /// The screen as a self-contained SVG document. Scale multiplies the width and height, the view box
    /// stays in screen units.
    static func document(_ screen: Scene.Screen, scale: Double = 1) -> String {
        var writer = SVGWriter(screen)
        return writer.document(scale: scale)
    }
}

private struct SVGWriter {
    private let screen: Scene.Screen
    private let p: Scene.Prep
    private let codeBackground = Scene.parseColor("grey-200") ?? Scene.Color()
    private let quoteBar = Scene.parseColor("grey-400") ?? Scene.Color()
    private var defs: [String] = []
    private var body: [String] = []
    private var nextID = 0
    private var hrefs: [String: String] = [:]
    private var rotations: [ObjectIdentifier: Double] = [:]

    init(_ screen: Scene.Screen) {
        self.screen = screen
        self.p = screen.p
    }

    mutating func document(scale: Double) -> String {
        item(screen.placed(), ox: 0, oy: 0, isRoot: true)
        links()
        let w = num(screen.w)
        let h = num(screen.h)
        let width = num(ceil(screen.w * scale))
        let height = num(ceil(screen.h * scale))
        let css = defs.joined()
        return """
            <svg xmlns="http://www.w3.org/2000/svg" xml:space="preserve" width="\(width)" height="\(height)" \
            viewBox="0 0 \(w) \(h)"><defs>\(css)</defs><g transform="scale(\(num(scale)))">\
            \(body.joined())</g></svg>
            """
    }

    // MARK: - Items

    mutating func item(_ pl: Scene.Placed, ox: Double, oy: Double, isRoot: Bool) {
        let id = ObjectIdentifier(pl.r)
        guard let ln = p.frames[id] else { return }
        let st = p.styles[id] ?? Scene.Look()
        var transform = "translate(\(num(pl.x - ox)) \(num(pl.y - oy)))"
        let deg = rotation(pl)
        if deg != 0 {
            let cx = num(pl.w / 2)
            let cy = num(pl.h / 2)
            transform += " translate(\(cx) \(cy)) rotate(\(num(deg))) translate(-\(cx) -\(cy))"
        }
        var attrs = " transform=\"\(transform)\""
        if !isRoot && st.opacity < 1 {
            attrs += " opacity=\"\(num(st.opacity))\""
        }
        if !isRoot && st.clip {
            let clip = clipPath(circle: false, w: pl.w, h: pl.h, radii: st.radius)
            attrs += " clip-path=\"url(#\(clip))\""
        }
        body.append("<g\(attrs)>")
        if !isRoot {
            picture(pl, ln, st)
        } else if let im = p.images[id] {
            // The root of a screen is a bare container; a picture there is the whole screen.
            let radii = pl.prop("corner-radius").map { p.radii($0) } ?? [0, 0, 0, 0]
            image(im, radii, pl.w, pl.h, mask: nil)
        }
        if st.display != "none" {
            for k in pl.kids {
                item(k, ox: pl.x, oy: pl.y, isRoot: false)
            }
            if let bn = p.bodyAt[id], let text = p.bodies[id] {
                bodyText(text, x: bn.rx, y: bn.ry, width: bn.rw)
            }
        }
        body.append("</g>")
    }

    mutating func picture(_ pl: Scene.Placed, _ ln: Scene.LayoutNode, _ st: Scene.Look) {
        let w = pl.w
        let h = pl.h
        guard w > 0, h > 0, st.display != "none" else { return }
        let id = ObjectIdentifier(pl.r)
        if let icons = p.icons[id], !icons.isEmpty {
            let ds = iconPaths(pl.r, count: icons.count)
            for (i, ic) in icons.enumerated() {
                icon(ic, i < ds.count ? ds[i] : nil, w, h, st)
            }
            if let ti = p.texts[id] {
                let clipped = st.display == "circle"
                if clipped {
                    openClip(w: w, h: h)
                }
                title(ti, pl, st, leaf: true, shadow: nil)
                close(clipped ? 1 : 0)
            }
            return
        }
        if st.display == "stack" {
            // Three sheets behind the box, stepped 2 up and right, as the classic renderer draws a stack.
            for i in 1...3 {
                let d = 2 * Double(3 - i)
                let shadow = Scene.Shadow(dx: 1, dy: -1)
                let radii = st.radius
                let fid = boxFilter(shadow.blur, w + abs(d) + 1, h + abs(d) + 1)
                let shadowAttrs =
                    paint("fill", shadow.color) + " filter=\"url(#\(fid))\""
                body.append(shape(circle: false, d + 1, -d - 1, w, h, radii, shadowAttrs))
                body.append(shape(circle: false, d, -d, w, h, radii, paint("fill", st.fill)))
            }
        }
        var shadows = st.shadows
        if st.display != "text" && st.display != "no-fill" && st.fill.a > 0 {
            let radii = boxRadii(st, w, h)
            let circle = st.display == "circle"
            if !shadows.isEmpty {
                // A shadow takes the alpha of the fill; a gradient goes over an opaque base.
                var base = st.fill
                base.a = 1
                for sh in shadows {
                    let fid = boxFilter(sh.blur, w, h)
                    let attrs = paint("fill", sh.color) + " filter=\"url(#\(fid))\""
                    body.append(shape(circle: circle, sh.dx, sh.dy, w, h, radii, attrs))
                    body.append(shape(circle: circle, 0, 0, w, h, radii, paint("fill", base)))
                }
                if let g = st.grad {
                    let fill = gradient(g, w, h)
                    body.append(shape(circle: circle, 0, 0, w, h, radii, " fill=\"url(#\(fill))\""))
                }
            } else if let g = st.grad {
                let fill = gradient(g, w, h)
                body.append(shape(circle: circle, 0, 0, w, h, radii, " fill=\"url(#\(fill))\""))
            } else {
                body.append(shape(circle: circle, 0, 0, w, h, radii, paint("fill", st.fill)))
            }
            shadows = []
        }
        if let im = p.images[id], st.display.isEmpty || st.display == "rect" {
            let mask = st.display == "circle" ? ellipseMask(w, h) : nil
            image(im, st.radius, w, h, mask: mask)
        }
        if st.display != "text" && st.lineW > 0 && st.border.a > 0 {
            let inW = st.lineW / 2
            let inner = st.radius.map { max($0 - inW, 0) }
            let circle = st.display == "circle"
            let edgeW = w - st.lineW
            let edgeH = h - st.lineW
            var attrs = " fill=\"none\"" + paint("stroke", st.border) + " stroke-width=\"\(num(st.lineW))\""
            attrs += capJoin(st)
            if let dash = st.dash {
                attrs += " stroke-dasharray=\"" + dash.map(num).joined(separator: " ") + "\""
                if st.dashPhase != 0 {
                    attrs += " stroke-dashoffset=\"\(num(st.dashPhase))\""
                }
            }
            for sh in shadows {
                let fid = boxFilter(sh.blur, w, h)
                let shadowAttrs =
                    " fill=\"none\"" + paint("stroke", sh.color) + " stroke-width=\"\(num(st.lineW))\""
                    + capJoin(st) + " filter=\"url(#\(fid))\""
                body.append(shape(circle: circle, inW + sh.dx, inW + sh.dy, edgeW, edgeH, inner, shadowAttrs))
            }
            body.append(shape(circle: circle, inW, inW, edgeW, edgeH, inner, attrs))
        }
        if let ti = p.texts[id] {
            let leaf = ln.kids.isEmpty && p.bodies[id] == nil
            let sh = st.display == "text" ? st.shadows.first : nil
            title(ti, pl, st, leaf: leaf, shadow: sh)
        }
        if let text = p.bodies[id], p.bodyAt[id] == nil {
            var y = text.padY
            if let ti = p.texts[id] {
                y = ti.padY + ti.height(w - 2 * ti.padX)
            }
            bodyText(text, x: text.padX, y: y, width: w - 2 * text.padX)
        }
    }

    // MARK: - Title

    mutating func title(
        _ ti: Scene.TextInfo, _ pl: Scene.Placed, _ st: Scene.Look, leaf: Bool, shadow: Scene.Shadow?
    ) {
        if ti.parts != nil {
            titleParts(ti, pl, st, leaf: leaf, shadow: shadow)
            return
        }
        let lines = ti.wrap(pl.w - 2 * ti.padX)
        var y = ti.padY
        if leaf {
            y = (pl.h - Double(lines.count) * ti.lineH) / 2
        }
        for (i, s) in lines.enumerated() where !s.isEmpty {
            var x = ti.padX
            if leaf {
                switch ti.align {
                case .left: break
                case .right: x = pl.w - ti.padX - ti.width(s)
                case .middle: x = (pl.w - ti.width(s)) / 2
                }
            }
            let base = y + Double(i) * ti.lineH + (ti.lineH - (ti.asc + ti.desc)) / 2 + ti.asc
            text(s, ti.font, st.text, x: x, base: base, shadow: shadow)
        }
    }

    /// A title of runs: each line is one text, every run a tspan at the x the layout measured.
    mutating func titleParts(
        _ ti: Scene.TextInfo, _ pl: Scene.Placed, _ st: Scene.Look, leaf: Bool, shadow: Scene.Shadow?
    ) {
        let lines = ti.layout(pl.w - 2 * ti.padX)
        let total = lines.reduce(0.0) { $0 + $1.h }
        var y = ti.padY
        if leaf {
            y = (pl.h - total) / 2
        }
        for ln in lines {
            var x = ti.padX
            if leaf {
                switch ti.align {
                case .left: break
                case .right: x = pl.w - ti.padX - ln.w
                case .middle: x = (pl.w - ln.w) / 2
                }
            }
            let base = y + (ln.h - (ln.asc + ln.desc)) / 2 + ln.asc
            var spans: [(x: Double, font: Scene.Font, text: String)] = []
            for part in ln.parts {
                if let img = part.img {
                    // shortcut: an inline picture keeps no pixels in the engine, as in ScenePainter.
                    x += img.w
                    continue
                }
                spans.append((x, ti.partFont(part), part.text))
                x += ti.partWidth(part)
            }
            if !spans.isEmpty {
                if let sh = shadow, sh.color.a > 0 {
                    let fid = textFilter(sh.blur)
                    let fill = paint("fill", sh.color) + " filter=\"url(#\(fid))\""
                    body.append(textLine(spans, base: base + sh.dy, fill: fill))
                }
                body.append(textLine(spans, base: base, fill: paint("fill", st.text)))
            }
            y += ln.h
        }
    }

    private func textLine(
        _ spans: [(x: Double, font: Scene.Font, text: String)], base: Double, fill: String
    ) -> String {
        var out = "<text x=\"0\" y=\"\(num(base))\"\(fontAttrs(spans[0].font))\(fill)>"
        for span in spans {
            out += "<tspan x=\"\(num(span.x))\"\(fontAttrs(span.font))>\(esc(span.text))</tspan>"
        }
        return out + "</text>"
    }

    mutating func text(
        _ s: String, _ font: Scene.Font, _ color: Scene.Color, x: Double, base: Double, shadow: Scene.Shadow?
    ) {
        guard !s.isEmpty else { return }
        let attrs = fontAttrs(font)
        if let sh = shadow, sh.color.a > 0 {
            let fid = textFilter(sh.blur)
            body.append(
                "<text x=\"\(num(x))\" y=\"\(num(base + sh.dy))\"\(attrs)\(paint("fill", sh.color))"
                    + " filter=\"url(#\(fid))\">\(esc(s))</text>")
        }
        body.append("<text x=\"\(num(x))\" y=\"\(num(base))\"\(attrs)\(paint("fill", color))>\(esc(s))</text>")
    }

    // MARK: - Body

    mutating func bodyText(_ block: Scene.BodyText, x: Double, y: Double, width: Double) {
        var yy = y
        for line in block.lines(width) {
            yy += line.gap
            if line.code && width > 0 {
                body.append(rect(x, yy, width, line.h, codeBackground))
            }
            if line.quote {
                body.append(rect(x, yy, Scene.bodyBarW, line.h, quoteBar))
            }
            var xx = x + line.x
            let base = yy + (line.h - (line.asc + line.desc)) / 2 + line.asc
            for run in line.runs {
                if let img = run.img {
                    xx += img.w
                    continue
                }
                let w = block.widthOf(run.text, run.font)
                if run.code && !line.code && w > 0 {
                    body.append(rect(xx, yy, w, line.h, codeBackground))
                }
                if !run.text.isEmpty {
                    text(run.text, run.font, run.color, x: xx, base: base, shadow: nil)
                }
                xx += w
            }
            yy += line.h
        }
    }

    private func rect(_ x: Double, _ y: Double, _ w: Double, _ h: Double, _ c: Scene.Color) -> String {
        "<rect x=\"\(num(x))\" y=\"\(num(y))\" width=\"\(num(w))\" height=\"\(num(h))\"\(paint("fill", c))/>"
    }

    // MARK: - Links

    mutating func links() {
        var byPath: [String: Scene.Placed] = [:]
        func index(_ pl: Scene.Placed, _ path: String) {
            byPath[path] = pl
            for k in pl.kids {
                index(k, path.isEmpty ? k.r.key : path + "." + k.r.key)
            }
        }
        let root = screen.placed()
        index(root, "")
        for link in screen.links() {
            guard let parent = byPath[link.parent] else { continue }
            var transform = "translate(\(num(parent.x)) \(num(parent.y)))"
            let deg = rotation(parent)
            if deg != 0 {
                let cx = num(parent.w / 2)
                let cy = num(parent.h / 2)
                transform += " translate(\(cx) \(cy)) rotate(\(num(deg))) translate(-\(cx) -\(cy))"
            }
            var attrs = paint("stroke", link.color) + " stroke-width=\"\(num(link.width))\""
            if let dash = link.dash {
                attrs += " stroke-dasharray=\"" + dash.map(num).joined(separator: " ") + "\""
                if link.dashPhase != 0 {
                    attrs += " stroke-dashoffset=\"\(num(link.dashPhase))\""
                }
            }
            var out = "<g transform=\"\(transform)\"><path d=\"\(linkPath(link.path))\" fill=\"none\"\(attrs)/>"
            if link.arrow, let head = arrowHead(link) {
                out += "<path d=\"\(head)\"\(paint("fill", link.color))/>"
            }
            body.append(out + "</g>")
        }
    }

    private func linkPath(_ steps: [Scene.PathStep]) -> String {
        var d = ""
        for step in steps {
            switch step {
            case let .move(x, y):
                d += "M\(num(x)) \(num(y))"
            case let .line(x, y):
                d += "L\(num(x)) \(num(y))"
            case let .quad(cx, cy, x, y):
                d += "Q\(num(cx)) \(num(cy)) \(num(x)) \(num(y))"
            }
        }
        return d
    }

    /// The closed head of an arrow link, as ScenePainter.arrowHead: the direction of the last stretch.
    private func arrowHead(_ link: Scene.Link) -> String? {
        guard let last = link.path.last else { return nil }
        func point(_ step: Scene.PathStep) -> Scene.Point {
            switch step {
            case let .move(x, y), let .line(x, y): return Scene.Point(x: x, y: y)
            case let .quad(_, _, x, y): return Scene.Point(x: x, y: y)
            }
        }
        let to: Scene.Point
        let from: Scene.Point
        switch last {
        case let .quad(cx, cy, x, y):
            from = Scene.Point(x: cx, y: cy)
            to = Scene.Point(x: x, y: y)
        case .line:
            guard link.path.count >= 2 else { return nil }
            to = point(last)
            from = point(link.path[link.path.count - 2])
        default:
            return nil
        }
        let d = to.sub(from).unit()
        let n = Scene.Point(x: -d.y, y: d.x)
        let size = 6 + link.width
        let base = to.sub(d.mul(size))
        let p1 = Scene.Point(x: base.x + n.x * size / 2, y: base.y + n.y * size / 2)
        let p2 = Scene.Point(x: base.x - n.x * size / 2, y: base.y - n.y * size / 2)
        return "M\(num(to.x)) \(num(to.y))L\(num(p1.x)) \(num(p1.y))L\(num(p2.x)) \(num(p2.y))Z"
    }

    // MARK: - Pictures

    mutating func image(
        _ im: Scene.ItemImage, _ radii: [Double], _ w: Double, _ h: Double, mask: String?
    ) {
        guard let href = href(im.path) else { return }
        let iw = Double(im.w)
        let ih = Double(im.h)
        guard iw > 0, ih > 0 else { return }
        var x = 0.0
        var y = 0.0
        var dw = w
        var dh = h
        switch im.fit {
        case "cover":
            let s = max(w / iw, h / ih)
            dw = iw * s
            dh = ih * s
            x = (w - dw) / 2
            y = (h - dh) / 2
        case "contain":
            let s = min(w / iw, h / ih)
            dw = iw * s
            dh = ih * s
            x = (w - dw) / 2
            y = (h - dh) / 2
        default:
            break
        }
        var closing = 0
        if let mask {
            body.append("<g clip-path=\"url(#\(mask))\">")
            closing += 1
        }
        if im.fit == "cover" || radii != [0, 0, 0, 0] {
            let clip = clipPath(circle: false, w: w, h: h, radii: radii)
            body.append("<g clip-path=\"url(#\(clip))\">")
            closing += 1
        }
        body.append(
            "<image x=\"\(num(x))\" y=\"\(num(y))\" width=\"\(num(dw))\" height=\"\(num(dh))\""
                + " preserveAspectRatio=\"none\" href=\"\(href)\"/>")
        close(closing)
    }

    /// The data URL of a picture file, kept for the draws of one document.
    mutating func href(_ path: String) -> String? {
        if let cached = hrefs[path] {
            return cached
        }
        guard !path.isEmpty, let data = try? Data(contentsOf: URL(fileURLWithPath: path)) else {
            return nil
        }
        let url = "data:\(mime(of: path));base64,\(data.base64EncodedString())"
        hrefs[path] = url
        return url
    }

    private func mime(of path: String) -> String {
        switch (path as NSString).pathExtension.lowercased() {
        case "png": return "image/png"
        case "jpg", "jpeg": return "image/jpeg"
        case "gif": return "image/gif"
        case "webp": return "image/webp"
        case "svg": return "image/svg+xml"
        case "bmp": return "image/bmp"
        case "tif", "tiff": return "image/tiff"
        default: return "image/png"
        }
    }

    // MARK: - Icons

    /// The d text of every path of an item, in file order. The paths that do not parse are absent from
    /// Scene.Prep.icons too, so a count that does not match falls back to the geometry of the built icons.
    mutating func iconPaths(_ rn: Scene.RNode, count: Int) -> [String] {
        let raw = p.namedProps(rn, "path").compactMap { $0.text(0) }
        let valid = raw.filter { d in
            guard let parsed = Scene.SVGPath.parse(d) else { return false }
            return !parsed.isEmpty
        }
        return valid.count == count ? valid : []
    }

    mutating func icon(_ ic: Scene.Icon, _ written: String?, _ w: Double, _ h: Double, _ st: Scene.Look) {
        let v = ic.viewBox
        guard v.width > 0, v.height > 0 else { return }
        let s = min(w / v.width, h / v.height)
        let tx = (w - v.width * s) / 2 - v.minX * s
        let ty = (h - v.height * s) / 2 - v.minY * s
        var attrs = " fill=\"none\""
        if ic.fill.a > 0 {
            if let g = ic.grad {
                // The path is drawn in icon coordinates, so the item-space gradient line maps back through the fit.
                let (from, to) = g.line(w, h)
                let start = Scene.Point(x: (from.x - tx) / s, y: (from.y - ty) / s)
                let end = Scene.Point(x: (to.x - tx) / s, y: (to.y - ty) / s)
                let fill = gradient(g, start, end)
                attrs = " fill=\"url(#\(fill))\""
            } else {
                attrs = paint("fill", ic.fill)
            }
        }
        if ic.lineWidth > 0 && ic.stroke.a > 0 {
            attrs += paint("stroke", ic.stroke) + " stroke-width=\"\(num(ic.lineWidth))\""
            attrs += capJoin(st)
            attrs += " vector-effect=\"non-scaling-stroke\""
        }
        if ic.evenOdd {
            attrs += " fill-rule=\"evenodd\""
        }
        let d = written ?? svgPath(ic.path)
        body.append(
            "<g transform=\"translate(\(num(tx)) \(num(ty))) scale(\(num(s)))\">"
                + "<path d=\"\(esc(d))\"\(attrs)/></g>")
    }

    /// The d of a CGPath, for an icon whose written path cannot be paired with it.
    private func svgPath(_ path: CGPath) -> String {
        var out = ""
        path.applyWithBlock { element in
            let e = element.pointee
            switch e.type {
            case .moveToPoint:
                out += "M\(num(e.points[0].x)) \(num(e.points[0].y))"
            case .addLineToPoint:
                out += "L\(num(e.points[0].x)) \(num(e.points[0].y))"
            case .addQuadCurveToPoint:
                out += "Q\(num(e.points[0].x)) \(num(e.points[0].y)) \(num(e.points[1].x)) \(num(e.points[1].y))"
            case .addCurveToPoint:
                out +=
                    "C\(num(e.points[0].x)) \(num(e.points[0].y)) \(num(e.points[1].x)) \(num(e.points[1].y))"
                    + " \(num(e.points[2].x)) \(num(e.points[2].y))"
            case .closeSubpath:
                out += "Z"
            @unknown default:
                break
            }
        }
        return out
    }

    // MARK: - Shapes and paints

    mutating func clipPath(circle: Bool, w: Double, h: Double, radii: [Double]) -> String {
        let id = newID("c")
        defs.append("<clipPath id=\"\(id)\">\(shape(circle: circle, 0, 0, w, h, radii, ""))</clipPath>")
        return id
    }

    mutating func ellipseMask(_ w: Double, _ h: Double) -> String {
        clipPath(circle: true, w: w, h: h, radii: [0, 0, 0, 0])
    }

    /// Opens a group that clips to the ellipse of the box, for a title over an icon.
    mutating func openClip(w: Double, h: Double) {
        let id = ellipseMask(w, h)
        body.append("<g clip-path=\"url(#\(id))\">")
    }

    mutating func close(_ count: Int) {
        for _ in 0..<count {
            body.append("</g>")
        }
    }

    private func shape(
        circle: Bool, _ x: Double, _ y: Double, _ w: Double, _ h: Double, _ radii: [Double], _ attrs: String
    ) -> String {
        if circle {
            return "<ellipse cx=\"\(num(x + w / 2))\" cy=\"\(num(y + h / 2))\" rx=\"\(num(w / 2))\""
                + " ry=\"\(num(h / 2))\"\(attrs)/>"
        }
        return "<path d=\"\(roundRect(x, y, w, h, radii))\"\(attrs)/>"
    }

    /// A rectangle with a radius per corner, as the private roundedPath of ScenePainter.
    private func roundRect(
        _ x0: Double, _ y0: Double, _ w: Double, _ h: Double, _ radii: [Double]
    ) -> String {
        let limit = min(w, h) / 2
        let r = radii.map { min($0, limit) }
        let k = 1 - 0.5522847498307936
        let x = x0
        let y = y0
        var d = "M\(num(x + r[0])) \(num(y))"
        d += "L\(num(x + w - r[1])) \(num(y))"
        d +=
            "C\(num(x + w - r[1] * k)) \(num(y)) \(num(x + w)) \(num(y + r[1] * k))"
            + " \(num(x + w)) \(num(y + r[1]))"
        d += "L\(num(x + w)) \(num(y + h - r[2]))"
        d +=
            "C\(num(x + w)) \(num(y + h - r[2] * k)) \(num(x + w - r[2] * k)) \(num(y + h))"
            + " \(num(x + w - r[2])) \(num(y + h))"
        d += "L\(num(x + r[3])) \(num(y + h))"
        d +=
            "C\(num(x + r[3] * k)) \(num(y + h)) \(num(x)) \(num(y + h - r[3] * k))"
            + " \(num(x)) \(num(y + h - r[3]))"
        d += "L\(num(x)) \(num(y + r[0]))"
        d +=
            "C\(num(x)) \(num(y + r[0] * k)) \(num(x + r[0] * k)) \(num(y))"
            + " \(num(x + r[0])) \(num(y))Z"
        return d
    }

    /// The radii of the box of an item; a hairline of a classic diagram stays square.
    private func boxRadii(_ st: Scene.Look, _ w: Double, _ h: Double) -> [Double] {
        if st.radiusDefault && (w < 2 || h < 2) {
            return [0, 0, 0, 0]
        }
        return st.radius
    }

    mutating func gradient(_ g: Scene.Gradient, _ w: Double, _ h: Double) -> String {
        let (from, to) = g.line(w, h)
        return gradient(g, from, to)
    }

    mutating func gradient(_ g: Scene.Gradient, _ from: Scene.Point, _ to: Scene.Point) -> String {
        let id = newID("g")
        var out = "<linearGradient id=\"\(id)\" gradientUnits=\"userSpaceOnUse\""
        out += " x1=\"\(num(from.x))\" y1=\"\(num(from.y))\" x2=\"\(num(to.x))\" y2=\"\(num(to.y))\">"
        for stop in g.stops {
            out += "<stop offset=\"\(num(stop.offset))\" stop-color=\"\(rgb(stop.color))\""
            out += " stop-opacity=\"\(num(stop.color.a))\"/>"
        }
        defs.append(out + "</linearGradient>")
        return id
    }

    mutating func boxFilter(_ blur: Double, _ w: Double, _ h: Double) -> String {
        let id = newID("f")
        let pad = 1.5 * max(0, blur) + 2
        defs.append(
            "<filter id=\"\(id)\" filterUnits=\"userSpaceOnUse\" x=\"\(num(-pad))\" y=\"\(num(-pad))\""
                + " width=\"\(num(w + 2 * pad))\" height=\"\(num(h + 2 * pad))\">"
                + "<feGaussianBlur stdDeviation=\"\(num(blur / 2))\"/></filter>")
        return id
    }

    mutating func textFilter(_ blur: Double) -> String {
        let id = newID("f")
        defs.append(
            "<filter id=\"\(id)\" x=\"-100%\" y=\"-100%\" width=\"300%\" height=\"300%\">"
                + "<feGaussianBlur stdDeviation=\"\(num(blur / 2))\"/></filter>")
        return id
    }

    private func capJoin(_ st: Scene.Look) -> String {
        var out = ""
        switch st.lineCap {
        case "round": out += " stroke-linecap=\"round\""
        case "square": out += " stroke-linecap=\"square\""
        default: break
        }
        switch st.lineJoin {
        case "round": out += " stroke-linejoin=\"round\""
        case "bevel": out += " stroke-linejoin=\"bevel\""
        default: out += " stroke-miterlimit=\"10\""
        }
        return out
    }

    private func fontAttrs(_ f: Scene.Font) -> String {
        let family = !f.name.isEmpty ? f.name : (!f.family.isEmpty ? f.family : "Helvetica")
        var out = " font-family=\"\(esc(family))\" font-size=\"\(num(f.size))\""
        if f.weight != 0 {
            out += " font-weight=\"\(f.weight)\""
        }
        if f.italic {
            out += " font-style=\"italic\""
        }
        if f.spacing != 0 {
            out += " letter-spacing=\"\(num(f.spacing))\""
        }
        return out
    }

    private func paint(_ attr: String, _ c: Scene.Color) -> String {
        var out = " \(attr)=\"\(rgb(c))\""
        if c.a < 1 {
            out += " \(attr)-opacity=\"\(num(c.a))\""
        }
        return out
    }

    private func rgb(_ c: Scene.Color) -> String {
        func q(_ v: Double) -> Int { Int((min(1, max(0, v)) * 255).rounded()) }
        return "rgb(\(q(c.r)),\(q(c.g)),\(q(c.b)))"
    }

    private func num(_ v: Double) -> String {
        if !v.isFinite || abs(v) < 5e-5 {
            return "0"
        }
        var s = String(format: "%.3f", v)
        while s.hasSuffix("0") {
            s.removeLast()
        }
        if s.hasSuffix(".") {
            s.removeLast()
        }
        return s == "-0" ? "0" : s
    }

    private func esc(_ s: String) -> String {
        var out = ""
        out.reserveCapacity(s.count)
        for ch in s {
            switch ch {
            case "&": out += "&amp;"
            case "<": out += "&lt;"
            case ">": out += "&gt;"
            case "\"": out += "&quot;"
            case "'": out += "&#39;"
            default: out.append(ch)
            }
        }
        return out
    }

    mutating func newID(_ prefix: String) -> String {
        nextID += 1
        return prefix + String(nextID)
    }

    mutating func rotation(_ pl: Scene.Placed) -> Double {
        let id = ObjectIdentifier(pl.r)
        if let known = rotations[id] {
            return known
        }
        let deg = p.rotation(pl.r)
        rotations[id] = deg
        return deg
    }
}
