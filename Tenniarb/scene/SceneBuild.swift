//
//  SceneBuild.swift
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

extension Scene {
    /// Defaults of a text leaf, as in the Tenniarb app (font-size 18) plus a padding around the text.
    static let defaultFontSize = 18.0
    static let textPadX = 10.0
    static let textPadY = 6.0
    /// The inset go/render gives a text leaf of a classic diagram when `padding` is not written: the picture of
    /// an old file must not move when the canvas draws it through the scene engine.
    static let classicTextPad = 4.0
    /// The corner radius go/render gives a box of a classic diagram.
    static let classicRadius = 8.0
    /// The ascent and the descent of a line as go/render takes them, so a classic diagram keeps its old line spacing.
    /// shortcut: the ratios approximate the real metrics of SF; a font with other metrics draws with these, as old files expect.
    static let classicAscentRatio = 1980.0 / 2048
    static let classicDescentRatio = 432.0 / 2048

    /// Turns resolved nodes into a layout tree and keeps what the drawing needs (prep of layout.go).
    final class Prep {
        let t: Tree
        /// CoreText as the window draws unless a stand asks for gfx.EstimateText.
        let measurer: TextMeasurer
        /// Options.EvalVars: nil means no expression runs and a ${...} is a warning.
        let eval: EvalFunc?
        var frames: [ObjectIdentifier: LayoutNode] = [:]
        var texts: [ObjectIdentifier: TextInfo] = [:]
        var bodies: [ObjectIdentifier: BodyText] = [:]
        /// A body laid out as a child of its item, when the item is a flow container.
        var bodyAt: [ObjectIdentifier: LayoutNode] = [:]
        var styles: [ObjectIdentifier: Look] = [:]
        var ends: [ObjectIdentifier: (Scene.RNode, Scene.RNode)] = [:]  // link -> its two items
        /// classic: the element being laid out is a classic diagram (sceneRoot is false).
        var classic = false
        /// The colours of the markdown markup already reported.
        var warned: Set<String> = []
        /// A property with an expression -> its evaluated copy, nil if it failed.
        private var evals: [EvalKey: Prop?] = [:]

        /// One property of one node: the Prop of a scene is shared between instances, so its evaluation is not.
        private struct EvalKey: Hashable {
            let node: ObjectIdentifier
            let prop: ObjectIdentifier
        }

        init(_ t: Tree, measurer: TextMeasurer = TextMeasurer(), eval: EvalFunc? = nil) {
            self.t = t
            self.measurer = measurer
            self.eval = eval
        }

        /// The layout of an element: its content at their own positions.
        func root(_ rn: RNode) -> LayoutNode {
            classic = !sceneRoot(rn)
            let n = LayoutNode()
            n.dir = .abs
            n.node = rn
            frames[id(rn)] = n
            // shortcut: a root that is one picture takes its pixel size in image.go; until then only width and height size it.
            if let pr = prop(rn, "width"), let v = pr.num(0) {
                n.width = .fixed(v)
            }
            if let pr = prop(rn, "height"), let v = pr.num(0) {
                n.height = .fixed(v)
            }
            for k in rn.kids where k.kind == .item && !hidden(k) {
                let kid = item(k, parentAuto: false, up: Scene.Color.black)
                kid.parent = n
                n.kids.append(kid)
            }
            links(rn)
            return n
        }

        /// Lays out one node; parentAuto tells that an auto-layout parent decides its hug and fill sizes,
        /// up is the text colour of the parent.
        func item(_ rn: RNode, parentAuto: Bool, up: Scene.Color) -> LayoutNode {
            let n = LayoutNode()
            n.node = rn
            frames[id(rn)] = n
            let auto = prop(rn, "auto-layout") != nil
            let inCtx = auto || parentAuto
            if let pr = prop(rn, "pos") {
                // Y is up everywhere in .tenn, pos is the top left corner, so the screen y is -pos.y.
                n.x = pr.num(0) ?? 0
                n.y = 0 - (pr.num(1) ?? 0)
            }
            n.width = size(rn, "width", inCtx, parentAuto)
            n.height = size(rn, "height", inCtx, parentAuto)
            if inCtx {
                limits(rn, n)
                n.absolute = flag(rn, "absolute")
            }
            n.padding = padding(rn)  // the text inset of a leaf, the children inset of a container
            if auto {
                container(rn, n)
            } else {
                n.dir = .abs
            }
            let st = style(rn, inCtx || (rn.inst && holdsItems(rn)), up)
            styles[id(rn)] = st
            // A body is the content of a leaf; an item that holds items shows them instead and is a backdrop.
            if !holdsItems(rn) {
                bodies[id(rn)] = bodyInfo(rn, st, inCtx)
            }
            for k in rn.kids where k.kind == .item && !hidden(k) {
                let kid = item(k, parentAuto: auto, up: st.text)
                kid.parent = n
                n.kids.append(kid)
            }
            // The body of an auto-layout item is a text block of its own after the children, so the flow puts it
            // below them and the item grows with it; a leaf paints it under its title instead.
            if let body = bodies[id(rn)], auto, !n.kids.isEmpty {
                let bn = LayoutNode()
                bn.width = .fill
                bn.leafW = { body.natural() }
                bn.leafH = { w in body.height(w) }
                body.padX = 0
                body.padY = 0
                bn.parent = n
                n.kids.append(bn)
                bodyAt[id(rn)] = bn
            }
            if n.kids.isEmpty {
                leaf(rn, n, inCtx)
            } else {
                if !rn.inst {  // the name of an instance is an alias, not its content
                    n.text = textOf(rn, inCtx)
                }
                links(rn)
            }
            texts[id(rn)] = n.text
            return n
        }

        /// Reads the properties that only an item with auto-layout has.
        private func container(_ rn: RNode, _ n: LayoutNode) {
            if let word = word(rn, "auto-layout", ["row", "column", "abs"]) {
                switch word {
                case "row": n.dir = .row
                case "column": n.dir = .column
                default: n.dir = .abs
                }
            }
            n.gap = num(rn, "gap") ?? 0
            // shortcut: a wrapping flow is one line until gowt layoutWrap and line-gap are ported.
            if let word = word(rn, "align", ["start", "center", "end"]) {
                n.align = align(word)
            }
            if let word = word(rn, "justify", ["start", "center", "end", "space-between"]) {
                n.justify = align(word)
            }
        }

        private func limits(_ rn: RNode, _ n: LayoutNode) {
            n.minWidth = num(rn, "min-width") ?? 0
            n.maxWidth = num(rn, "max-width") ?? 0
            n.minHeight = num(rn, "min-height") ?? 0
            n.maxHeight = num(rn, "max-height") ?? 0
        }

        private func padding(_ rn: RNode) -> Insets {
            guard let pr = prop(rn, "padding") else { return Insets() }
            var values: [Double] = []
            for i in 0..<pr.args.count {
                guard let value = pr.num(i) else {
                    invalid(pr.pos, "padding takes numbers")
                    return Insets()
                }
                values.append(value)
            }
            switch values.count {
            case 1:
                return Insets(top: values[0], right: values[0], bottom: values[0], left: values[0])
            case 2:  // vertical, horizontal
                return Insets(top: values[0], right: values[1], bottom: values[0], left: values[1])
            case 4:
                return Insets(top: values[0], right: values[1], bottom: values[2], left: values[3])
            default:
                invalid(pr.pos, "padding takes 1, 2 or 4 numbers, got \(values.count)")
                return Insets()
            }
        }

        /// Reads width or height: a number, or hug and fill where an auto-layout is at work.
        private func size(_ rn: RNode, _ name: String, _ inCtx: Bool, _ parentAuto: Bool) -> Size {
            guard let pr = prop(rn, name), !pr.args.isEmpty else { return .hug }
            if let v = pr.num(0) {
                // go/render takes a size below one as not written, so an item with a zero measure falls back
                // to its text: an old file that computes 0 draws a dot, not an invisible box.
                if classic && v < 1 {
                    return .hug
                }
                return .fixed(v)
            }
            // text, not the token: an expression may give the word as well as a number.
            let word = pr.text(0) ?? ""
            switch true {
            case word == "hug" || word == "fill":
                return sizeWord(pr, word, inCtx, parentAuto)
            case word.hasSuffix("%"):
                t.warn(pr.pos, "\(name): percentages are not supported, the size is hug")
            default:
                invalid(pr.pos, "\(name) takes a number, hug or fill, got \"\(word)\"")
            }
            return .hug
        }

        private func sizeWord(_ pr: Prop, _ word: String, _ inCtx: Bool, _ parentAuto: Bool) -> Size {
            guard inCtx else {
                invalid(pr.pos, "\(pr.name) \(word) needs auto-layout on the item or on its parent")
                return .hug
            }
            if pr.args.count > 1 {
                t.warn(pr.pos, "\(pr.name) fill with a fallback value is not supported")
            }
            if word == "fill" {
                // There is nothing to fill in a plain parent: the item takes its own size.
                guard parentAuto else {
                    t.warn(pr.pos, "\(pr.name) fill needs auto-layout on the parent, the size is hug")
                    return .hug
                }
                return .fill
            }
            return .hug
        }

        /// Gives an item without children the size of its title and body.
        private func leaf(_ rn: RNode, _ n: LayoutNode, _ inCtx: Bool) {
            // An icon draws no name as text, as in a layout context; icon.go is not ported.
            let ti = textOf(rn, inCtx)
            let body = bodies[id(rn)]
            var padX = classic ? Scene.classicTextPad : Scene.textPadX
            var padY = classic ? Scene.classicTextPad : Scene.textPadY
            if inCtx {
                padX = 0
                padY = 0
            }
            // padding insets the text of an item whether or not it has auto-layout: a leaf with a written size
            // reads it as the space between its border and the text.
            if prop(rn, "padding") != nil {
                padX = n.padding.left
                padY = n.padding.top
            }
            guard ti != nil || body != nil else {
                // go/render keeps the inset around an item with no text: a dot of eight points, which old
                // diagrams use as markers and anchors.
                if classic && !inCtx {
                    n.leafW = { 2 * padX }
                    n.leafH = { _ in 2 * padY }
                }
                return
            }
            if let ti {
                ti.padX = padX
                ti.padY = padY
                n.text = ti
            }
            if let body {
                body.padX = padX
                body.padY = padY
            }
            let classic = self.classic
            n.leafW = {
                var w = 0.0
                if let ti {
                    w = max(w, ti.natural())
                }
                if let body {
                    w = max(w, body.natural())
                }
                // go/render kept the fraction of a measured line; only a scene rounds its leaf to whole points.
                return classic ? w + 2 * padX : ceil(w) + 2 * padX
            }
            n.leafH = { w in
                let inner = w - 2 * padX
                var h = 0.0
                if let ti {
                    h += ti.height(inner)
                }
                if let body {
                    h += body.height(inner)
                }
                return h + 2 * padY
            }
        }

        /// Reports an absolutely placed item that lies wholly outside its clipping parent: it is laid out but
        /// never drawn. Items of a flow that overflow are cut by design and not reported.
        func warnOutside(_ rn: RNode) {
            guard let ln = frames[id(rn)] else { return }
            let clip = styles[id(rn)]?.clip ?? false
            for k in rn.kids {
                guard k.kind == .item, let kn = frames[id(k)] else { continue }
                if clip && (ln.dir == .abs || kn.absolute) && kn.rw > 0 && kn.rh > 0
                    && (kn.rx >= ln.rw || kn.ry >= ln.rh || kn.rx + kn.rw <= 0 || kn.ry + kn.rh <= 0)
                {
                    t.warn(
                        k.pos,
                        String(
                            format: "item \"%@\" is at %g, %g (size %g x %g) outside its clipping parent \"%@\" (%g x %g) and is not drawn",
                            k.name, kn.rx, kn.ry, kn.rw, kn.rh, rn.name, ln.rw, ln.rh))
                }
                warnOutside(k)
            }
        }

        /// Whether the item is not drawn, and so not laid out.
        func hidden(_ rn: RNode) -> Bool { flag(rn, "hidden") }

        /// A value the scene cannot use: an error in a scene, and only a warning in a classic diagram, where
        /// go/render drew what it understood and ignored the rest of the file.
        func invalid(_ pos: Pos, _ message: String) {
            if classic {
                t.warn(pos, message)
            } else {
                t.fail(pos, message)
            }
        }

        /// prop is rn.prop with the expressions of the arguments evaluated (spec 6), in the scope of the instance
        /// of the node (spec 5c). Without an evaluator an expression is a warning and the property is ignored.
        func prop(_ rn: RNode, _ name: String) -> Prop? {
            let pr = rn.prop(name)
            guard let pr, hasExpr(pr) else { return pr }
            if eval == nil {
                if !classic {
                    t.warn(pr.pos, "\(name): expressions are not evaluated")
                }
                return nil
            }
            return evalProp(rn, pr)
        }

        /// pr evaluated in the scope of rn, once per build; nil when an expression failed, which leaves the
        /// property unset.
        func evalProp(_ rn: RNode, _ pr: Prop) -> Prop? {
            let key = EvalKey(node: ObjectIdentifier(rn), prop: ObjectIdentifier(pr))
            if let cached = evals[key] {
                return cached
            }
            let out = evaluated(rn, pr)
            evals.updateValue(out, forKey: key)
            return out
        }

        /// Evaluates every expression of the tree before the styles: use-style ${kind} has to be a value by then
        /// (spec 5c). Only the nodes change, not the shared Prop: a node of an instance is its own, its Prop not.
        func evalTree(_ rn: RNode) {
            for (i, pr) in rn.props.enumerated() where hasExpr(pr) {
                if let out = evalProp(rn, pr) {
                    rn.props[i] = out
                }
            }
            for k in rn.kids {
                evalTree(k)
            }
        }

        /// A copy of pr with every argument evaluated, the plain ones copied as they are; nil when an expression
        /// failed. The source property is shared between instances, so it is never changed.
        private func evaluated(_ rn: RNode, _ pr: Prop) -> Prop? {
            var vals: [String] = []
            for a in pr.args {
                let (v, ok) = evalArg(eval, a, rn.vars)
                if !ok {
                    t.warn(pr.pos, "\(pr.name): the expression \"\(a.getIdentText() ?? "")\" is not evaluated")
                    return nil
                }
                vals.append(v)
            }
            return Prop(name: pr.name, args: pr.args, val: vals, block: pr.block, pos: pr.pos, range: pr.range)
        }

        func num(_ rn: RNode, _ name: String) -> Double? {
            guard let pr = prop(rn, name) else { return nil }
            guard let value = pr.num(0) else {
                invalid(pr.pos, "\(name) takes a number")
                return nil
            }
            return value
        }

        func flag(_ rn: RNode, _ name: String) -> Bool {
            guard let pr = prop(rn, name) else { return false }
            switch pr.text(0) ?? "" {
            case "yes", "true": return true
            case "no", "false": return false
            default:
                invalid(pr.pos, "\(name) takes yes or no")
                return false
            }
        }

        func word(_ rn: RNode, _ name: String, _ allowed: [String]) -> String? {
            guard let pr = prop(rn, name) else { return nil }
            let s = pr.text(0) ?? ""
            if allowed.contains(s) {
                return s
            }
            if s == "space-around" {
                t.warn(pr.pos, "\(name) space-around is not supported by the layout")
            } else if name != "auto-layout" || !s.isEmpty {
                invalid(pr.pos, "\(name) takes one of \(allowed.joined(separator: ", ")), got \"\(s)\"")
            }
            return nil
        }

        private func align(_ word: String) -> Align {
            switch word {
            case "center": return .center
            case "end": return .end
            case "space-between": return .spaceBetween
            default: return .start
            }
        }

        private func id(_ rn: RNode) -> ObjectIdentifier { ObjectIdentifier(rn) }
    }
}

/// Whether the node has an item kid, so it draws no box of its own unless it asks.
func holdsItems(_ rn: Scene.RNode) -> Bool {
    rn.kids.contains { $0.kind == .item }
}

/// Model.SceneElement on the expanded tree: an instance, an item with child nodes or auto-layout. Other elements
/// are classic diagrams, which go/render draws with their expressions.
func sceneRoot(_ rn: Scene.RNode) -> Bool {
    rn.kids.contains { $0.kind == .item && ($0.inst || !$0.kids.isEmpty || $0.prop("auto-layout") != nil) }
}

extension Scene.Tree {
    /// The placed tree of the element at path ("" is the file itself): expanded, styled and laid out; nil when
    /// the element has an error and there is nothing to draw.
    func layout(
        _ path: String = "", measurer: Scene.TextMeasurer = Scene.TextMeasurer(), eval: Scene.EvalFunc? = nil
    ) -> Scene.LayoutNode? {
        build(path, measurer: measurer, eval: eval)?.root
    }
}
