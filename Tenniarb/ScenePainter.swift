//
//  ScenePainter.swift
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

import Cocoa
import CoreText
import ImageIO

/// Paints a built scene screen with CoreGraphics: the port of what go/scene/build.go writes into a gfx
/// picture, read from the same Placed tree, styles, texts and bodies the layout produced.
final class ScenePainter {
    private let screen: Scene.Screen
    private let p: Scene.Prep
    private var rasters: [String: CGImage] = [:]
    private var broken: Set<String> = []
    private var rotations: [ObjectIdentifier: Double] = [:]
    private let codeBackground = Scene.parseColor("grey-200") ?? Scene.Color()
    private let quoteBar = Scene.parseColor("grey-400") ?? Scene.Color()

    init(screen: Scene.Screen) {
        self.screen = screen
        self.p = screen.p
    }

    /// Paints the items of the screen in paint order, then the links over them.
    func paint(_ c: CGContext) {
        paint(screen.placed(), c)
    }

    /// Paints a placed tree the caller holds, so the picture and the hit-test share one geometry.
    func paint(_ placed: Scene.Placed, _ c: CGContext) {
        paint(placed, c, ox: 0, oy: 0, isRoot: true)
        paintLinks(placed, c)
    }

    // MARK: - Items

    /// Paints one item and its subtree; ox and oy are the origin of the parent, so the context stays in
    /// the coordinates of the item itself.
    private func paint(_ pl: Scene.Placed, _ c: CGContext, ox: Double, oy: Double, isRoot: Bool) {
        let id = ObjectIdentifier(pl.r)
        guard let ln = p.frames[id] else { return }
        let st = p.styles[id] ?? Scene.Look()
        c.saveGState()
        c.translateBy(x: pl.x - ox, y: pl.y - oy)
        if let deg = rotation(pl), deg != 0 {
            c.translateBy(x: pl.w / 2, y: pl.h / 2)
            c.rotate(by: deg * .pi / 180)
            c.translateBy(x: -pl.w / 2, y: -pl.h / 2)
        }
        if !isRoot {
            if st.opacity < 1 {
                c.setAlpha(st.opacity)
            }
            if st.clip {
                clipToBox(st, pl.w, pl.h, c)
            }
            paintPicture(pl, ln, st, c)
        } else if let im = p.images[id] {
            // The root of a screen is otherwise a bare container; a picture there is the whole screen.
            drawImage(im, pl.prop("corner-radius").map { p.radii($0) } ?? [0, 0, 0, 0], pl.w, pl.h, nil, c)
        }
        if st.display != "none" {
            for k in pl.kids {
                paint(k, c, ox: pl.x, oy: pl.y, isRoot: false)
            }
            if let bn = p.bodyAt[id], let body = p.bodies[id] {
                drawBody(body, x: bn.rx, y: bn.ry, width: bn.rw, c)
            }
        }
        c.restoreGState()
    }

    /// The picture of one item: shadows, fill or gradient, picture, border, title and body.
    private func paintPicture(_ pl: Scene.Placed, _ ln: Scene.LayoutNode, _ st: Scene.Look, _ c: CGContext) {
        let w = pl.w
        let h = pl.h
        guard w > 0, h > 0, st.display != "none" else { return }
        let id = ObjectIdentifier(pl.r)
        let mask = pictureMask(st, w, h)
        if st.display == "stack" {
            // Three sheets behind the box, stepped 2 up and right, as the classic renderer draws a stack.
            for i in 1...3 {
                let d = 2 * Double(3 - i)
                let sheet = roundedPath(CGRect(x: d, y: -d, width: w, height: h), st.radius)
                fillWithShadow(sheet, st.fill, Scene.Shadow(dx: 1, dy: -1), c)
            }
        }
        var shadows = st.shadows
        if st.display != "text" && st.display != "no-fill" && st.fill.a > 0 {
            let shape = boxPath(st, w, h)
            if !shadows.isEmpty {
                // A shadow takes the alpha of the fill; a gradient is drawn over an opaque base, so a
                // gradient with transparent stops shows the base.
                var base = st.fill
                base.a = 1
                for sh in shadows {
                    fillWithShadow(shape, base, sh, c)
                }
            }
            if let g = st.grad {
                fillGradient(shape, g, w, h, c)
            } else if shadows.isEmpty {
                c.addPath(shape)
                c.setFillColor(st.fill.cgColor)
                c.fillPath()
            }
            shadows = []
        }
        if let im = p.images[id], st.display.isEmpty || st.display == "rect" {
            drawImage(im, st.radius, w, h, mask, c)
        }
        if st.display != "text" && st.lineW > 0 && st.border.a > 0 {
            let inW = st.lineW / 2
            let inner = st.radius.map { max($0 - inW, 0) }
            let edge: CGPath
            if st.display == "circle" {
                edge = CGPath(
                    ellipseIn: CGRect(x: inW, y: inW, width: w - st.lineW, height: h - st.lineW), transform: nil)
            } else {
                edge = roundedPath(CGRect(x: inW, y: inW, width: w - st.lineW, height: h - st.lineW), inner)
            }
            c.saveGState()
            c.setStrokeColor(st.border.cgColor)
            c.setLineWidth(st.lineW)
            c.setLineJoin(.miter)
            c.setMiterLimit(10)
            if let dash = st.dash {
                c.setLineDash(phase: st.dashPhase, lengths: dash.map { CGFloat($0) })
            }
            if !shadows.isEmpty {
                for sh in shadows {
                    c.saveGState()
                    setShadow(sh, c)
                    c.addPath(edge)
                    c.strokePath()
                    c.restoreGState()
                }
            } else {
                c.addPath(edge)
                c.strokePath()
            }
            c.restoreGState()
        }
        if let ti = p.texts[id] {
            // With a body the title goes to the top left and the body follows it, as in classic diagrams.
            let leaf = ln.kids.isEmpty && p.bodies[id] == nil
            let sh = st.display == "text" ? st.shadows.first : nil
            drawTitle(ti, pl, st, leaf: leaf, shadow: sh, c)
        }
        if let body = p.bodies[id], p.bodyAt[id] == nil {
            var y = body.padY
            if let ti = p.texts[id] {
                y = ti.padY + ti.height(w - 2 * ti.padX)
            }
            drawBody(body, x: body.padX, y: y, width: w - 2 * body.padX, c)
        }
    }

    /// The static rotate of an item, read once: a bad one warns here, as the build of Go warns.
    private func rotation(_ pl: Scene.Placed) -> Double? {
        let id = ObjectIdentifier(pl.r)
        let deg: Double
        if let known = rotations[id] {
            deg = known
        } else {
            deg = p.rotation(pl.r)
            rotations[id] = deg
        }
        return deg == 0 ? nil : deg
    }

    private func clipToBox(_ st: Scene.Look, _ w: Double, _ h: Double, _ c: CGContext) {
        let rect = CGRect(x: 0, y: 0, width: w, height: h)
        c.addPath(roundedPath(rect, st.radius))
        c.clip()
    }

    // MARK: - Title

    /// Draws the wrapped lines the layout measured: a leaf centers each line unless layout says left or
    /// right, an item with children puts them at its top left.
    private func drawTitle(
        _ ti: Scene.TextInfo, _ pl: Scene.Placed, _ st: Scene.Look, leaf: Bool, shadow: Scene.Shadow?, _ c: CGContext
    ) {
        if ti.parts != nil {
            drawParts(ti, pl, st, leaf: leaf, shadow: shadow, c)
            return
        }
        let lines = ti.wrap(pl.w - 2 * ti.padX)
        var y = ti.padY
        if leaf {
            y = (pl.h - Double(lines.count) * ti.lineH) / 2
        }
        for (i, s) in lines.enumerated() {
            if s.isEmpty { continue }
            var x = ti.padX
            if leaf {
                switch ti.align {
                case .left: break
                case .right: x = pl.w - ti.padX - ti.width(s)
                case .middle: x = (pl.w - ti.width(s)) / 2
                }
            }
            let base = y + Double(i) * ti.lineH + (ti.lineH - (ti.asc + ti.desc)) / 2 + ti.asc
            drawText(s, ti.font, st.text, x: x, base: base, shadow: shadow, c)
        }
    }

    /// Draws a title that holds runs of its own: the emphasis and colour of a classic title, and its
    /// pictures, which this engine keeps by size only.
    private func drawParts(
        _ ti: Scene.TextInfo, _ pl: Scene.Placed, _ st: Scene.Look, leaf: Bool, shadow: Scene.Shadow?, _ c: CGContext
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
            for part in ln.parts {
                if let img = part.img {
                    // shortcut: an inline picture keeps no pixels in the engine, so it stays empty here;
                    // only a classic diagram draws it, through its own canvas.
                    x += img.w
                    continue
                }
                let color = part.hasColor ? part.color : st.text
                drawText(part.text, ti.partFont(part), color, x: x, base: base, shadow: shadow, c)
                x += ti.partWidth(part)
            }
            y += ln.h
        }
    }

    // MARK: - Body

    /// Draws the lines of a body, its code backgrounds and quote bars, as bodyText.draw of body.go.
    private func drawBody(_ body: Scene.BodyText, x: Double, y: Double, width: Double, _ c: CGContext) {
        var yy = y
        for line in body.lines(width) {
            yy += line.gap
            if line.code && width > 0 {
                fillRect(x, yy, width, line.h, codeBackground, c)
            }
            if line.quote {
                fillRect(x, yy, Scene.bodyBarW, line.h, quoteBar, c)
            }
            var xx = x + line.x
            let base = yy + (line.h - (line.asc + line.desc)) / 2 + line.asc
            for run in line.runs {
                if let img = run.img {
                    xx += img.w
                    continue
                }
                let w = body.widthOf(run.text, run.font)
                if run.code && !line.code && w > 0 {
                    fillRect(xx, yy, w, line.h, codeBackground, c)
                }
                if !run.text.isEmpty {
                    drawText(run.text, run.font, run.color, x: xx, base: base, shadow: nil, c)
                }
                xx += w
            }
            yy += line.h
        }
    }

    private func fillRect(_ x: Double, _ y: Double, _ w: Double, _ h: Double, _ color: Scene.Color, _ c: CGContext) {
        c.setFillColor(color.cgColor)
        c.fill(CGRect(x: x, y: y, width: w, height: h))
    }

    // MARK: - Links

    private func paintLinks(_ root: Scene.Placed, _ c: CGContext) {
        var byPath: [String: Scene.Placed] = [:]
        func index(_ pl: Scene.Placed, _ path: String) {
            byPath[path] = pl
            for k in pl.kids {
                index(k, path.isEmpty ? k.r.key : path + "." + k.r.key)
            }
        }
        index(root, "")
        for link in screen.links() {
            guard let parent = byPath[link.parent] else { continue }
            c.saveGState()
            c.translateBy(x: parent.x, y: parent.y)
            if let deg = rotation(parent), deg != 0 {
                c.translateBy(x: parent.w / 2, y: parent.h / 2)
                c.rotate(by: deg * .pi / 180)
                c.translateBy(x: -parent.w / 2, y: -parent.h / 2)
            }
            let path = CGMutablePath()
            for step in link.path {
                switch step {
                case let .move(x, y):
                    path.move(to: CGPoint(x: x, y: y))
                case let .line(x, y):
                    path.addLine(to: CGPoint(x: x, y: y))
                case let .quad(cx, cy, x, y):
                    path.addQuadCurve(to: CGPoint(x: x, y: y), control: CGPoint(x: cx, y: cy))
                }
            }
            c.setStrokeColor(link.color.cgColor)
            c.setLineWidth(link.width)
            if let dash = link.dash {
                c.setLineDash(phase: link.dashPhase, lengths: dash.map { CGFloat($0) })
            }
            c.addPath(path)
            c.strokePath()
            if link.arrow, let head = arrowHead(link) {
                c.addPath(head)
                c.setFillColor(link.color.cgColor)
                c.fillPath()
            }
            c.restoreGState()
        }
    }

    /// The closed head of an arrow link, as arrowHead of build.go: the direction of the last stretch.
    private func arrowHead(_ link: Scene.Link) -> CGPath? {
        guard let last = link.path.last else { return nil }
        var to = Scene.Point()
        var from = Scene.Point()
        func point(_ step: Scene.PathStep) -> Scene.Point {
            switch step {
            case let .move(x, y), let .line(x, y): return Scene.Point(x: x, y: y)
            case let .quad(_, _, x, y): return Scene.Point(x: x, y: y)
            }
        }
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
        let path = CGMutablePath()
        path.move(to: CGPoint(x: to.x, y: to.y))
        path.addLine(to: CGPoint(x: base.x + n.x * size / 2, y: base.y + n.y * size / 2))
        path.addLine(to: CGPoint(x: base.x - n.x * size / 2, y: base.y - n.y * size / 2))
        path.closeSubpath()
        return path
    }

    // MARK: - Pictures

    /// Draws a picture of a file into a box of w by h with its fit, the corners of radius cut off.
    private func drawImage(_ im: Scene.ItemImage, _ radius: [Double], _ w: Double, _ h: Double, _ mask: CGPath?, _ c: CGContext) {
        guard let img = raster(im.path) else { return }
        let iw = Double(img.width)
        let ih = Double(img.height)
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
        c.saveGState()
        if let mask {
            c.addPath(mask)
            c.clip()
        }
        if im.fit == "cover" || radius != [0, 0, 0, 0] {
            c.addPath(
                radius == [0, 0, 0, 0]
                    ? CGPath(rect: CGRect(x: 0, y: 0, width: w, height: h), transform: nil)
                    : roundedPath(CGRect(x: 0, y: 0, width: w, height: h), radius))
            c.clip()
        }
        c.translateBy(x: x, y: y + dh)
        c.scaleBy(x: 1, y: -1)
        c.draw(img, in: CGRect(x: 0, y: 0, width: dw, height: dh))
        c.restoreGState()
    }

    /// The decoded picture of a file, kept between the draws of one screen.
    /// shortcut: a decode per screen, not the byte-budget cache of go/scene/image.go.
    private func raster(_ path: String) -> CGImage? {
        if let img = rasters[path] {
            return img
        }
        if path.isEmpty || broken.contains(path) {
            return nil
        }
        let url = URL(fileURLWithPath: path) as CFURL
        guard
            let src = CGImageSourceCreateWithURL(url, [kCGImageSourceShouldCache: false] as CFDictionary),
            let img = CGImageSourceCreateImageAtIndex(src, 0, nil)
        else {
            broken.insert(path)
            return nil
        }
        rasters[path] = img
        return img
    }

    // MARK: - Shapes

    /// The outline of the box of an item: an ellipse for display circle, else its rounded rectangle.
    private func boxPath(_ st: Scene.Look, _ w: Double, _ h: Double) -> CGPath {
        if st.display == "circle" {
            return CGPath(ellipseIn: CGRect(x: 0, y: 0, width: w, height: h), transform: nil)
        }
        var radius = st.radius
        if st.radiusDefault && (w < 2 || h < 2) {
            // A hairline is a divider, not a pill: the classic renderer keeps it square.
            radius = [0, 0, 0, 0]
        }
        return roundedPath(CGRect(x: 0, y: 0, width: w, height: h), radius)
    }

    /// The shape that cuts the pictures a box draws: the ellipse of display circle, nil for a plain box.
    private func pictureMask(_ st: Scene.Look, _ w: Double, _ h: Double) -> CGPath? {
        guard st.display == "circle", w > 0, h > 0 else { return nil }
        return CGPath(ellipseIn: CGRect(x: 0, y: 0, width: w, height: h), transform: nil)
    }

    // MARK: - Paint primitives

    private func setShadow(_ sh: Scene.Shadow, _ c: CGContext) {
        c.setShadow(offset: CGSize(width: sh.dx, height: sh.dy), blur: sh.blur, color: sh.color.cgColor)
    }

    private func fillWithShadow(_ path: CGPath, _ color: Scene.Color, _ sh: Scene.Shadow, _ c: CGContext) {
        c.saveGState()
        setShadow(sh, c)
        c.addPath(path)
        c.setFillColor(color.cgColor)
        c.fillPath()
        c.restoreGState()
    }

    private func fillGradient(_ path: CGPath, _ g: Scene.Gradient, _ w: Double, _ h: Double, _ c: CGContext) {
        let (from, to) = g.line(w, h)
        guard let space = CGColorSpace(name: CGColorSpace.sRGB),
            let grad = CGGradient(
                colorsSpace: space, colors: g.stops.map { $0.color.cgColor } as CFArray,
                locations: g.stops.map { CGFloat($0.offset) })
        else {
            return
        }
        c.saveGState()
        c.addPath(path)
        c.clip()
        c.drawLinearGradient(
            grad, start: CGPoint(x: from.x, y: from.y), end: CGPoint(x: to.x, y: to.y),
            options: [.drawsBeforeStartLocation, .drawsAfterEndLocation])
        c.restoreGState()
    }

    /// Draws text with its baseline at base, as gfx.Recorder.Text lays it out in a y-down picture.
    private func drawText(
        _ s: String, _ font: Scene.Font, _ color: Scene.Color, x: Double, base: Double, shadow: Scene.Shadow?,
        _ c: CGContext
    ) {
        guard !s.isEmpty else { return }
        var attrs: [NSAttributedString.Key: Any] = [.font: font.nsFont, .foregroundColor: color.nsColor]
        if font.spacing != 0 {
            attrs[.kern] = font.spacing
        }
        let line = CTLineCreateWithAttributedString(NSAttributedString(string: s, attributes: attrs))
        c.saveGState()
        if let shadow {
            c.setShadow(offset: CGSize(width: shadow.dx, height: shadow.dy), blur: shadow.blur, color: shadow.color.cgColor)
        }
        c.translateBy(x: x, y: base)
        c.scaleBy(x: 1, y: -1)
        c.textMatrix = .identity
        c.textPosition = .zero
        CTLineDraw(line, c)
        c.restoreGState()
    }
}

extension Scene.Color {
    /// The colour as CoreGraphics takes it.
    var cgColor: CGColor { CGColor(srgbRed: r, green: g, blue: b, alpha: a) }

    /// The colour as AppKit text takes it.
    var nsColor: NSColor { NSColor(srgbRed: r, green: g, blue: b, alpha: a) }
}

/// A rectangle with a radius per corner: top left, top right, bottom right, bottom left.
private func roundedPath(_ rect: CGRect, _ radii: [Double]) -> CGPath {
    let limit = min(rect.width, rect.height) / 2
    let r = radii.map { min($0, limit) }
    let x = rect.minX
    let y = rect.minY
    let w = rect.width
    let h = rect.height
    // A quarter circle of radius 1 as a cubic, as gfx.Path.RoundRect4 draws a corner.
    let k = 1 - 0.5522847498307936
    let path = CGMutablePath()
    path.move(to: CGPoint(x: x + r[0], y: y))
    path.addLine(to: CGPoint(x: x + w - r[1], y: y))
    path.addCurve(
        to: CGPoint(x: x + w, y: y + r[1]), control1: CGPoint(x: x + w - r[1] * k, y: y),
        control2: CGPoint(x: x + w, y: y + r[1] * k))
    path.addLine(to: CGPoint(x: x + w, y: y + h - r[2]))
    path.addCurve(
        to: CGPoint(x: x + w - r[2], y: y + h), control1: CGPoint(x: x + w, y: y + h - r[2] * k),
        control2: CGPoint(x: x + w - r[2] * k, y: y + h))
    path.addLine(to: CGPoint(x: x + r[3], y: y + h))
    path.addCurve(
        to: CGPoint(x: x, y: y + h - r[3]), control1: CGPoint(x: x + r[3] * k, y: y + h),
        control2: CGPoint(x: x, y: y + h - r[3] * k))
    path.addLine(to: CGPoint(x: x, y: y + r[0]))
    path.addCurve(
        to: CGPoint(x: x + r[0], y: y), control1: CGPoint(x: x, y: y + r[0] * k),
        control2: CGPoint(x: x + r[0] * k, y: y))
    path.closeSubpath()
    return path
}
