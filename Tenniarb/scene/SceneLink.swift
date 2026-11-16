//
//  SceneLink.swift
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
    /// A point in the coordinates of the parent of a link.
    struct Point: Equatable {
        var x = 0.0
        var y = 0.0

        func dist(_ q: Point) -> Double { hypot(x - q.x, y - q.y) }
        func sub(_ q: Point) -> Point { Point(x: x - q.x, y: y - q.y) }
        func add(_ q: Point) -> Point { Point(x: x + q.x, y: y + q.y) }
        func mul(_ f: Double) -> Point { Point(x: x * f, y: y * f) }

        /// The direction of the vector, or the zero point for a zero one.
        func unit() -> Point {
            let len = hypot(x, y)
            return len == 0 ? Point() : Point(x: x / len, y: y / len)
        }
    }

    /// A side of a box in layout coordinates: x grows to the right, y down.
    enum Side: Int {
        case right
        case left
        case top
        case bottom
    }

    /// The bend of an elbow link; corner-radius on the link overrides it.
    static let elbowRadius = 12.0

    /// One step of the line a link paints, as the verbs of gfx.Path it uses.
    enum PathStep {
        case move(Double, Double)
        case line(Double, Double)
        case quad(Double, Double, Double, Double)
    }

    /// A line drawn for a link, in the coordinates of the item that owns it.
    struct Link {
        /// The path of the item whose coordinates the line is drawn in; "" for the element itself.
        var parent = ""
        var path: [PathStep] = []
        var color = Scene.Color.black
        var width = 1.0
        var dash: [Double]?
        var dashPhase = 0.0
        var arrow = false
    }

    /// The stroke of a link, as the part of gfx.StrokeStyle a line reads.
    struct LinkStyle {
        var width = 1.0
        var dash: [Double]?
        var dashPhase = 0.0
    }

    /// Where one link leaves or enters a box: the side and the place along it, 0..1.
    struct LinkCorner {
        var side = Side.right
        var f = 0.0
    }
}

/// One group of elbow links on a side of a box: the box of the end and the side.
private struct ElbowKey: Hashable {
    let rn: ObjectIdentifier
    let side: Scene.Side
}

private struct ElbowMember {
    let lk: Scene.RNode
    let order: Double
}

extension Scene.Prep {
    // MARK: - Routing (linkroute.go)

    /// route as written on a link, without a warning: the elbow layout reads it many times.
    func routeWord(_ lk: Scene.RNode) -> String {
        if let pr = prop(lk, "route") {
            return pr.text(0) ?? ""
        }
        return ""
    }

    /// Whether the link asks for the elbow route.
    func isElbow(_ lk: Scene.RNode) -> Bool { routeWord(lk) == "elbow" }

    /// A side as the route names it.
    private func sideWord(_ s: String) -> Scene.Side? {
        switch s {
        case "right": return .right
        case "left": return .left
        case "top": return .top
        case "bottom": return .bottom
        default: return nil
        }
    }

    /// The sides the elbow leaves and enters: the ones the route names, else the sides of the boxes that
    /// look at each other.
    func routeSides(_ lk: Scene.RNode, _ a: Scene.LayoutNode, _ c: Scene.LayoutNode) -> (Scene.Side, Scene.Side) {
        var sides = facingSides(a, c)
        guard let pr = prop(lk, "route") else { return sides }
        for i in 0..<2 {
            if i + 1 >= pr.args.count {
                break
            }
            let word = pr.text(i + 1) ?? ""
            guard let side = sideWord(word) else {
                t.warn(pr.pos, "a route side is left, right, top or bottom, got \"\(word)\"")
                continue
            }
            if i == 0 {
                sides.0 = side
            } else {
                sides.1 = side
            }
        }
        return sides
    }

    /// Warns about a route the format does not know; a link without one is straight.
    func checkRoute(_ lk: Scene.RNode) {
        switch routeWord(lk) {
        case "", "straight", "elbow":
            break
        case let word:
            t.warn(prop(lk, "route")?.pos ?? lk.pos, "route takes straight or elbow, got \"\(word)\"")
        }
    }

    /// The corners of an elbow link: it leaves and enters the sides of its boxes that face each other, and the
    /// links that share a side spread along it, in the order of the boxes they lead to.
    func elbow(_ parent: Scene.RNode, _ lk: Scene.RNode) -> [Scene.Point] {
        guard let pair = ends[ObjectIdentifier(lk)],
            let a = frames[ObjectIdentifier(pair.0)],
            let c = frames[ObjectIdentifier(pair.1)]
        else {
            return []
        }
        let (starts, finishes) = elbowSides(parent)
        var sa = starts[ObjectIdentifier(lk)]
        var sc = finishes[ObjectIdentifier(lk)]
        if sa == nil {
            // The link was not in the groups: sides still have to be known for the drawing.
            let sides = routeSides(lk, a, c)
            sa = Scene.LinkCorner(side: sides.0, f: 0.5)
            sc = Scene.LinkCorner(side: sides.1, f: 0.5)
        }
        guard let sa, let sc else { return [] }
        let pa = sideAnchor(a, sa.side, sa.f)
        let pc = sideAnchor(c, sc.side, sc.f)
        // A side that is a horizontal edge makes the line leave upwards or downwards, and the other way round.
        let leaves = !sideVertical(sa.side)
        let enters = !sideVertical(sc.side)
        switch true {
        case leaves && !enters:
            return [pa, Scene.Point(x: pa.x, y: pc.y), pc]
        case !leaves && enters:
            return [pa, Scene.Point(x: pc.x, y: pa.y), pc]
        case leaves:
            let my = (pa.y + pc.y) / 2
            return [pa, Scene.Point(x: pa.x, y: my), Scene.Point(x: pc.x, y: my), pc]
        default:
            let mx = (pa.x + pc.x) / 2
            return [pa, Scene.Point(x: mx, y: pa.y), Scene.Point(x: mx, y: pc.y), pc]
        }
    }

    /// Places every elbow link of a parent on the sides of both its ends.
    func elbowSides(_ parent: Scene.RNode) -> ([ObjectIdentifier: Scene.LinkCorner], [ObjectIdentifier: Scene.LinkCorner]) {
        var startGroups: [ElbowKey: [ElbowMember]] = [:]
        var endGroups: [ElbowKey: [ElbowMember]] = [:]
        var sides: [ObjectIdentifier: (Scene.Side, Scene.Side)] = [:]
        for k in parent.kids where k.kind == .link && isElbow(k) {
            guard let pair = ends[ObjectIdentifier(k)],
                let a = frames[ObjectIdentifier(pair.0)],
                let c = frames[ObjectIdentifier(pair.1)]
            else {
                continue
            }
            let (from, to) = routeSides(k, a, c)
            sides[ObjectIdentifier(k)] = (from, to)
            startGroups[ElbowKey(rn: ObjectIdentifier(pair.0), side: from), default: []].append(
                ElbowMember(lk: k, order: along(c, to)))
            endGroups[ElbowKey(rn: ObjectIdentifier(pair.1), side: to), default: []].append(
                ElbowMember(lk: k, order: along(a, from)))
        }
        func place(_ groups: [ElbowKey: [ElbowMember]], _ which: Int) -> [ObjectIdentifier: Scene.LinkCorner] {
            var out: [ObjectIdentifier: Scene.LinkCorner] = [:]
            for list in groups.values {
                let sorted = list.enumerated()
                    .sorted { ($0.element.order, $0.offset) < ($1.element.order, $1.offset) }
                    .map(\.element)
                for (i, m) in sorted.enumerated() {
                    let side = which == 0 ? sides[ObjectIdentifier(m.lk)]?.0 : sides[ObjectIdentifier(m.lk)]?.1
                    out[ObjectIdentifier(m.lk)] = Scene.LinkCorner(
                        side: side ?? .right, f: Double(i + 1) / Double(sorted.count + 1))
                }
            }
            return out
        }
        return (place(startGroups, 0), place(endGroups, 1))
    }

    /// The sides of a and of c that look at each other.
    private func facingSides(_ a: Scene.LayoutNode, _ c: Scene.LayoutNode) -> (Scene.Side, Scene.Side) {
        let dx = (c.rx + c.rw / 2) - (a.rx + a.rw / 2)
        let dy = (c.ry + c.rh / 2) - (a.ry + a.rh / 2)
        if abs(dx) >= abs(dy) {
            return dx >= 0 ? (.right, .left) : (.left, .right)
        }
        return dy >= 0 ? (.bottom, .top) : (.top, .bottom)
    }

    /// Whether the side is a vertical edge, where anchors vary in y.
    private func sideVertical(_ side: Scene.Side) -> Bool { side == .left || side == .right }

    /// The point at f of a side of n.
    private func sideAnchor(_ n: Scene.LayoutNode, _ side: Scene.Side, _ f: Double) -> Scene.Point {
        switch side {
        case .right: return Scene.Point(x: n.rx + n.rw, y: n.ry + n.rh * f)
        case .left: return Scene.Point(x: n.rx, y: n.ry + n.rh * f)
        case .top: return Scene.Point(x: n.rx + n.rw * f, y: n.ry)
        case .bottom: return Scene.Point(x: n.rx + n.rw * f, y: n.ry + n.rh)
        }
    }

    /// The centre of n on the axis a side of the other box runs along.
    private func along(_ n: Scene.LayoutNode, _ side: Scene.Side) -> Double {
        sideVertical(side) ? n.ry + n.rh / 2 : n.rx + n.rw / 2
    }

    /// The polyline with the interior corners rounded by r.
    private func elbowPath(_ points: [Scene.Point], _ r: Double) -> [Scene.PathStep] {
        guard points.count >= 2 else { return [] }
        var out: [Scene.PathStep] = [.move(points[0].x, points[0].y)]
        for i in 1..<points.count - 1 {
            let prev = points[i - 1]
            let cur = points[i]
            let next = points[i + 1]
            let rr = min(r, min(cur.dist(prev) / 2, cur.dist(next) / 2))
            if rr <= 0 {
                out.append(.line(cur.x, cur.y))
                continue
            }
            let before = cur.sub(cur.sub(prev).unit().mul(rr))
            let after = cur.add(next.sub(cur).unit().mul(rr))
            out.append(.line(before.x, before.y))
            out.append(.quad(cur.x, cur.y, after.x, after.y))
        }
        let last = points[points.count - 1]
        out.append(.line(last.x, last.y))
        return out
    }

    // MARK: - The link around it (layout.go, build.go)

    /// Collects the ends of the links of rn for the drawing: both ends must be items of the same parent, and a
    /// link with a hidden end goes with it.
    func links(_ rn: Scene.RNode) {
        var byName: [String: [Scene.RNode]] = [:]
        for k in rn.kids where k.kind == .item {
            byName[k.name.precomposedStringWithCanonicalMapping, default: []].append(k)
        }
        func end(_ lk: Scene.RNode, _ name: String, _ idxProp: String) -> Scene.RNode? {
            let list = byName[name.precomposedStringWithCanonicalMapping] ?? []
            var idx = 0
            if let pr = prop(lk, idxProp), let v = pr.num(0) {
                guard v.isFinite, v >= 0, v < Double(list.count) else { return nil }
                idx = Int(v)
            }
            guard idx < list.count else { return nil }
            return list[idx]
        }
        for k in rn.kids where k.kind == .link {
            guard let a = end(k, k.from, "source-index"), let b = end(k, k.to, "target-index") else {
                invalid(k.pos, "link \"\(k.from)\" \"\(k.to)\": both ends must be items of the same parent")
                continue
            }
            if frames[ObjectIdentifier(a)] == nil || frames[ObjectIdentifier(b)] == nil {
                continue
            }
            ends[ObjectIdentifier(k)] = (a, b)
            _ = linkStyle(k)  // a bad line-style or colour is reported here, not while drawing
        }
    }

    /// The colour and the stroke a link draws its line with.
    func linkStyle(_ lk: Scene.RNode) -> (Scene.Color, Scene.LinkStyle) {
        var st = Scene.LinkStyle()
        if let v = num(lk, "line-width") {
            st.width = v
        }
        (st.dash, st.dashPhase) = dash(lk)
        return (color(lk, "color", .black), st)
    }

    /// The line of lk where the boxes of its ends are now: nil when it paints nothing.
    func linkPicture(_ parent: Scene.RNode, _ lk: Scene.RNode) -> Scene.Link? {
        guard let pair = ends[ObjectIdentifier(lk)],
            let a = frames[ObjectIdentifier(pair.0)],
            let c = frames[ObjectIdentifier(pair.1)]
        else {
            return nil
        }
        let pa = Scene.Point(x: a.rx + a.rw / 2, y: a.ry + a.rh / 2)
        let pc = Scene.Point(x: c.rx + c.rw / 2, y: c.ry + c.rh / 2)
        if pa == pc {
            return nil
        }
        let ctrl = linkControl(prop(lk, "pos"), pa, pc)
        var path: [Scene.PathStep] = []
        switch true {
        case isElbow(lk):
            let points = elbow(parent, lk)
            if points.count < 2 {
                return nil
            }
            var radius = Scene.elbowRadius
            if let pr = prop(lk, "corner-radius"), let v = pr.num(0) {
                radius = v
            }
            path = elbowPath(points, radius)
        case ctrl != nil:
            // The ends leave the boxes toward the control point, not toward each other.
            let from = edgePoint(pa, a, ctrl!)
            let to = edgePoint(pc, c, ctrl!)
            path = [.move(from.x, from.y)]
            if isQuad(lk) {
                path.append(.quad(ctrl!.x, ctrl!.y, to.x, to.y))
            } else {
                path.append(.line(ctrl!.x, ctrl!.y))
                path.append(.line(to.x, to.y))
            }
        default:
            let from = edgePoint(pa, a, pc)
            let to = edgePoint(pc, c, pa)
            path = [.move(from.x, from.y), .line(to.x, to.y)]
        }
        let (color, stroke) = linkStyle(lk)
        if hasLinkLabel(lk) {
            return nil  // a label paints text, and a link that paints text is no line in the dump
        }
        var out = Scene.Link()
        out.path = path
        out.color = color
        out.width = stroke.width
        out.dash = stroke.dash
        out.dashPhase = stroke.dashPhase
        // The flag stands for the closed head the canvas paints; the dump records it, not the triangle.
        out.arrow = prop(lk, "display")?.text(0) == "arrow"
        return out
    }

    /// Whether the link writes a label: linkLabel of layout.go is nil only for a link without text.
    private func hasLinkLabel(_ lk: Scene.RNode) -> Bool {
        guard let text = prop(lk, "label")?.text(0) else { return false }
        return !text.isEmpty
    }

    /// Whether the link asks for the quad curve with layout quad.
    func isQuad(_ lk: Scene.RNode) -> Bool {
        prop(lk, "layout")?.text(0) == "quad"
    }
}

/// The control point a link's pos gives: the middle of its two boxes plus the offset, which is y up where
/// the scene y is down.
private func linkControl(_ pr: Scene.Prop?, _ pa: Scene.Point, _ pc: Scene.Point) -> Scene.Point? {
    guard let pr, let x = pr.num(0), let y = pr.num(1), !(x == 0 && y == 0) else { return nil }
    return Scene.Point(x: (pa.x + pc.x) / 2 + x, y: (pa.y + pc.y) / 2 - y)
}

/// Where the line from the centre of box n toward other leaves the box.
private func edgePoint(_ center: Scene.Point, _ n: Scene.LayoutNode, _ other: Scene.Point) -> Scene.Point {
    let dx = other.x - center.x
    let dy = other.y - center.y
    var t = Double.infinity
    if dx != 0 {
        t = min(t, n.rw / 2 / abs(dx))
    }
    if dy != 0 {
        t = min(t, n.rh / 2 / abs(dy))
    }
    t = min(t, 1)
    return Scene.Point(x: center.x + dx * t, y: center.y + dy * t)
}
