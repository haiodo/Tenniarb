//
//  SceneLayout.swift
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
    /// How a node places its children: left to right, top to bottom, or each at its own x, y.
    enum Dir {
        case row
        case column
        case abs
    }

    /// How a size is decided.
    enum SizeKind {
        case hug
        case fixed
        case fill
    }

    /// A width or a height: fixed(v), hug or fill. The zero value is hug.
    struct Size {
        var kind: SizeKind = .hug
        var value = 0.0  // the number of a fixed size

        static let hug = Size()
        static let fill = Size(kind: .fill)

        static func fixed(_ value: Double) -> Size { Size(kind: .fixed, value: value) }
    }

    /// The four paddings of a node: top, right, bottom, left.
    struct Insets {
        var top = 0.0
        var right = 0.0
        var bottom = 0.0
        var left = 0.0
    }

    /// Places children on an axis. spaceBetween is for justifiy only.
    enum Align {
        case start
        case center
        case end
        case spaceBetween
    }

    /// One box of an auto-layout, the port of gowt gfx/layout.Node: set the inputs, call layout(), read rx/ry/rw/rh.
    final class LayoutNode {
        var width: Size = .hug
        var height: Size = .hug
        var minWidth = 0.0
        var maxWidth = 0.0  // zero: no limit
        var minHeight = 0.0
        var maxHeight = 0.0  // zero: no limit

        var x = 0.0  // position in an abs parent, or of an absolute child
        var y = 0.0
        var absolute = false  // leaves the flow of a row or a column parent

        var dir: Dir = .row
        var gap = 0.0
        var padding = Insets()
        var align: Align = .start
        var justify: Align = .start

        var kids: [LayoutNode] = []
        var node: Node?  // the scene node the box was made for

        // Results, relative to the parent's top left corner.
        private(set) var rx = 0.0
        private(set) var ry = 0.0
        private(set) var rw = 0.0
        private(set) var rh = 0.0

        /// Sizes the node from its own width and height, then places everything below.
        func layout() {
            let w = width.kind == .fixed ? clampW(width.value) : intrinsicWidth()
            var h = -1.0
            if height.kind == .fixed {
                h = height.value
            }
            layout(w: w, h: h)
            rx = 0
            ry = 0
        }

        private func clampW(_ value: Double) -> Double { clamp(value, minWidth, maxWidth) }

        private func clampH(_ value: Double) -> Double { clamp(value, minHeight, maxHeight) }

        private func clamp(_ value: Double, _ lo: Double, _ hi: Double) -> Double {
            var value = value
            if hi > 0 && value > hi {
                value = hi
            }
            if value < lo {
                value = lo
            }
            return value
        }

        /// The children that take part in the flow of a row or a column.
        private func flow() -> [LayoutNode] {
            kids.contains { $0.absolute } ? kids.filter { !$0.absolute } : kids
        }

        private func gapsOf(_ gap: Double, _ count: Int) -> Double { gap * Double(max(count - 1, 0)) }

        private func intrinsicWidth() -> Double {
            if width.kind == .fixed {
                return clampW(width.value)
            }
            var w = 0.0
            let kids = dir == .abs ? self.kids : flow()
            switch true {
            case kids.isEmpty:
                w = 0  // text and body sizes arrive with the painter
            case dir == .abs:
                for k in kids {
                    w = max(w, k.x + k.intrinsicWidth())
                }
            case dir == .row:
                w = padding.left + padding.right + gapsOf(gap, kids.count)
                for k in kids {
                    w += k.intrinsicWidth()
                }
            default:
                for k in kids {
                    w = max(w, k.intrinsicWidth())
                }
                w += padding.left + padding.right
            }
            return clampW(w)
        }

        private func kidW(_ avail: Double) -> Double {
            switch width.kind {
            case .fixed:
                return clampW(width.value)
            case .fill:
                return clampW(avail)
            case .hug:
                return intrinsicWidth()
            }
        }

        /// Sizes the node to width w and height h, h < 0 meaning as high as the content.
        private func layout(w: Double, h: Double) {
            let w = clampW(w)
            var h = h
            if h >= 0 {
                h = clampH(h)
            }
            rw = w
            if kids.isEmpty {
                rh = max(h, 0)
            } else if dir == .abs {
                layoutAbs(w: w, h: h)
            } else {
                layoutFlow(w: w, h: h)
                layoutAbsolute()
            }
        }

        private func layoutAbs(w: Double, h: Double) {
            var maxBottom = 0.0
            for k in kids {
                var kh = -1.0
                switch k.height.kind {
                case .fixed: kh = k.height.value
                case .fill: kh = h
                case .hug: break
                }
                k.layout(w: k.kidW(w), h: kh)
                k.rx = k.x
                k.ry = k.y
                maxBottom = max(maxBottom, k.ry + k.rh)
            }
            rh = h
            if h < 0 {
                rh = clampH(maxBottom)
            }
        }

        /// Places the absolute children of a row or a column, once the node has its size.
        private func layoutAbsolute() {
            for k in kids where k.absolute {
                var kh = -1.0
                switch k.height.kind {
                case .fixed: kh = k.height.value
                case .fill: kh = rh
                case .hug: break
                }
                k.layout(w: k.kidW(rw), h: kh)
                k.rx = k.x
                k.ry = k.y
            }
        }

        // No wrap and no line-gap yet: go/scene reads both, gowt layoutWrap has no Swift port.
        private func layoutFlow(w: Double, h: Double) {
            let kids = flow()
            let innerW = w - padding.left - padding.right
            let gaps = gapsOf(gap, kids.count)
            var innerH = h - padding.top - padding.bottom
            if h < 0 {
                innerH = -1
            }
            if dir == .row {
                layoutRow(kids, h: h, innerW: innerW, innerH: innerH, gaps: gaps)
            } else {
                layoutColumn(kids, h: h, innerW: innerW, innerH: innerH, gaps: gaps)
            }
        }

        private func layoutRow(_ kids: [LayoutNode], h: Double, innerW: Double, innerH: Double, gaps: Double) {
            var fixed = 0.0
            var nfill = 0
            for k in kids {
                if k.width.kind == .fill {
                    nfill += 1
                } else {
                    fixed += k.kidW(0)
                }
            }
            var share = 0.0
            if nfill > 0 {
                share = max((innerW - fixed - gaps) / Double(nfill), 0)
            }
            var contentH = 0.0
            for k in kids {
                var kh = -1.0
                switch k.height.kind {
                case .fixed: kh = k.height.value
                case .fill: kh = innerH
                case .hug: break
                }
                k.layout(w: k.kidW(share), h: kh)
                contentH = max(contentH, k.rh)
            }
            var innerH = innerH
            var height = h
            if h < 0 {
                let hug = hugH(contentH)
                height = hug.height
                innerH = hug.inner
                for k in kids where k.height.kind == .fill {
                    k.layout(w: k.rw, h: innerH)
                }
            }
            rh = height
            place(kids, horiz: true, innerW: innerW, innerH: innerH, gaps: gaps)
        }

        private func layoutColumn(_ kids: [LayoutNode], h: Double, innerW: Double, innerH: Double, gaps: Double) {
            var sum = 0.0
            var nfill = 0
            for k in kids {
                if k.height.kind == .fill && innerH >= 0 {
                    nfill += 1
                    continue
                }
                var kh = -1.0
                if k.height.kind == .fixed {
                    kh = k.height.value
                }
                k.layout(w: k.kidW(innerW), h: kh)
                sum += k.rh
            }
            if nfill > 0 {
                let share = max((innerH - sum - gaps) / Double(nfill), 0)
                for k in kids where k.height.kind == .fill {
                    k.layout(w: k.kidW(innerW), h: share)
                    sum += k.rh
                }
            }
            var innerH = innerH
            var height = h
            if h < 0 {
                let hug = hugH(sum + gaps)
                height = hug.height
                innerH = hug.inner
            }
            rh = height
            place(kids, horiz: false, innerW: innerW, innerH: innerH, gaps: gaps)
        }

        /// The height of a node that hugs content, and the room inside its padding.
        private func hugH(_ content: Double) -> (height: Double, inner: Double) {
            let height = clampH(content + padding.top + padding.bottom)
            var inner = content
            if height != content + padding.top + padding.bottom {
                inner = height - padding.top - padding.bottom
            }
            return (height, inner)
        }

        private func place(_ kids: [LayoutNode], horiz: Bool, innerW: Double, innerH: Double, gaps: Double) {
            var used = 0.0
            for k in kids {
                used += horiz ? k.rw : k.rh
            }
            let mainSize = horiz ? innerW : innerH
            let crossSize = horiz ? innerH : innerW
            var gap = self.gap
            var off = 0.0
            switch justify {
            case .center:
                off = (mainSize - used - gaps) / 2
            case .end:
                off = mainSize - used - gaps
            case .spaceBetween:
                if kids.count > 1 {
                    gap = (mainSize - used) / Double(kids.count - 1)
                }
            case .start:
                break
            }
            for k in kids {
                let main = horiz ? k.rw : k.rh
                let cross = horiz ? k.rh : k.rw
                var crossOff = 0.0
                switch align {
                case .center:
                    crossOff = (crossSize - cross) / 2
                case .end:
                    crossOff = crossSize - cross
                case .start, .spaceBetween:
                    break
                }
                if horiz {
                    k.rx = padding.left + off
                    k.ry = padding.top + crossOff
                } else {
                    k.rx = padding.left + crossOff
                    k.ry = padding.top + off
                }
                off += main + gap
            }
        }
    }

    /// Turns a loaded element into layout boxes: reads the same properties go/scene/layout.go does.
    final class LayoutEngine {
        private let tree: Tree

        init(tree: Tree) {
            self.tree = tree
        }

        /// The layout of an element: its content at their own positions.
        func layout(_ element: Node) -> LayoutNode {
            let n = LayoutNode()
            n.node = element
            n.dir = .abs
            if let pr = element.prop("width"), let v = pr.num(0) {
                n.width = .fixed(v)
            }
            if let pr = element.prop("height"), let v = pr.num(0) {
                n.height = .fixed(v)
            }
            for k in element.content where k.kind == .item {
                n.kids.append(item(k, parentAuto: false))
            }
            return n
        }

        private func item(_ node: Node, parentAuto: Bool) -> LayoutNode {
            let n = LayoutNode()
            n.node = node
            let auto = prop(node, "auto-layout") != nil
            let inCtx = auto || parentAuto
            if let pr = prop(node, "pos") {
                n.x = pr.num(0) ?? 0
                n.y = 0 - (pr.num(1) ?? 0)  // .tenn has y up, pos is the top left corner
            }
            n.width = size(node, "width", inCtx: inCtx, parentAuto: parentAuto)
            n.height = size(node, "height", inCtx: inCtx, parentAuto: parentAuto)
            if inCtx {
                limits(node, n)
                n.absolute = flag(node, "absolute")
            }
            n.padding = padding(node)
            if auto {
                container(node, n)
            } else {
                n.dir = .abs
            }
            for k in node.content where k.kind == .item {
                n.kids.append(item(k, parentAuto: auto))
            }
            return n
        }

        /// Reads the properties that only an item with auto-layout has.
        private func container(_ node: Node, _ n: LayoutNode) {
            if let word = word(node, "auto-layout", ["row", "column", "abs"]) {
                switch word {
                case "row": n.dir = .row
                case "column": n.dir = .column
                default: n.dir = .abs
                }
            }
            n.gap = num(node, "gap") ?? 0
            if let word = word(node, "align", ["start", "center", "end"]) {
                n.align = align(word)
            }
            if let word = word(node, "justify", ["start", "center", "end", "space-between"]) {
                n.justify = align(word)
            }
        }

        private func limits(_ node: Node, _ n: LayoutNode) {
            n.minWidth = num(node, "min-width") ?? 0
            n.maxWidth = num(node, "max-width") ?? 0
            n.minHeight = num(node, "min-height") ?? 0
            n.maxHeight = num(node, "max-height") ?? 0
        }

        private func padding(_ node: Node) -> Insets {
            guard let pr = prop(node, "padding") else { return Insets() }
            var values: [Double] = []
            for i in 0..<pr.args.count {
                guard let value = pr.num(i) else {
                    tree.fail(pr.pos, "padding takes numbers")
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
                tree.fail(pr.pos, "padding takes 1, 2 or 4 numbers, got \(values.count)")
                return Insets()
            }
        }

        /// Reads width or height: a number, or hug and fill where an auto-layout is at work.
        private func size(_ node: Node, _ name: String, inCtx: Bool, parentAuto: Bool) -> Size {
            guard let pr = prop(node, name), !pr.args.isEmpty else { return .hug }
            if let value = pr.num(0) {
                return .fixed(value)
            }
            let word = pr.text(0) ?? ""
            switch true {
            case word == "hug" || word == "fill":
                return sizeWord(pr, word, inCtx: inCtx, parentAuto: parentAuto)
            case word.hasSuffix("%"):
                tree.warn(pr.pos, "\(name): percentages are not supported, the size is hug")
            default:
                tree.fail(pr.pos, "\(name) takes a number, hug or fill, got \"\(word)\"")
            }
            return .hug
        }

        private func sizeWord(_ pr: Prop, _ word: String, inCtx: Bool, parentAuto: Bool) -> Size {
            guard inCtx else {
                tree.fail(pr.pos, "\(pr.name) \(word) needs auto-layout on the item or on its parent")
                return .hug
            }
            if pr.args.count > 1 {
                tree.warn(pr.pos, "\(pr.name) fill with a fallback value is not supported")
            }
            if word == "fill" {
                guard parentAuto else {
                    tree.warn(pr.pos, "\(pr.name) fill needs auto-layout on the parent, the size is hug")
                    return .hug
                }
                return .fill
            }
            return .hug
        }

        private func num(_ node: Node, _ name: String) -> Double? {
            guard let pr = prop(node, name) else { return nil }
            guard let value = pr.num(0) else {
                tree.fail(pr.pos, "\(name) takes a number")
                return nil
            }
            return value
        }

        private func flag(_ node: Node, _ name: String) -> Bool {
            guard let pr = prop(node, name) else { return false }
            switch pr.text(0) ?? "" {
            case "yes", "true": return true
            case "no", "false": return false
            default:
                tree.fail(pr.pos, "\(name) takes yes or no")
                return false
            }
        }

        private func word(_ node: Node, _ name: String, _ allowed: [String]) -> String? {
            guard let pr = prop(node, name) else { return nil }
            let s = pr.text(0) ?? ""
            if allowed.contains(s) {
                return s
            }
            if s == "space-around" {
                tree.warn(pr.pos, "\(name) space-around is not supported by the layout")
            } else if name != "auto-layout" || !s.isEmpty {
                tree.fail(pr.pos, "\(name) takes one of \(allowed.joined(separator: ", ")), got \"\(s)\"")
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

        private func prop(_ node: Node, _ name: String) -> Prop? { node.prop(name) }
    }
}
