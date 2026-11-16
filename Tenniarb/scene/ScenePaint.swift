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
}
