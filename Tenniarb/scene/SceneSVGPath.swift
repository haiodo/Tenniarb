//
//  SceneSVGPath.swift
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

extension Scene {
    /// An icon: one SVG path of a leaf item, the view box it is fitted into, and the ink it draws with.
    struct Icon {
        var path: CGPath
        /// x, y, width and height in path coordinates; a path without view-box gives its bounds.
        var viewBox: CGRect
        var fill = Scene.Color()
        var stroke = Scene.Color()
        var lineWidth = 1.0
        var evenOdd = false
        /// Set by color linear ...; fill is then its first visible stop.
        var grad: Scene.Gradient?
    }

    /// An SVG path and the box of its points, as gfx.ParseSVGPath and pointBounds of icon.go.
    struct SVGPath {
        var path: CGPath
        /// The points as written, control points included: the box Go takes for a path without view-box.
        var bounds: CGRect
        /// The text held no command at all.
        var isEmpty: Bool

        /// Reads the d attribute of a path; nil when a command or a number does not parse.
        static func parse(_ d: String) -> SVGPath? {
            var sc = SVGScanner(d)
            let path = CGMutablePath()
            var box = PointBox()
            var cur = CGPoint.zero
            var start = CGPoint.zero
            var lastC = CGPoint.zero
            var cmd: Character = " "
            var prev: Character = " "
            var empty = true
            while true {
                sc.skip()
                if sc.atEnd { break }
                let c = sc.s[sc.i]
                if isSVGCommand(c) {
                    cmd = c
                    sc.i += 1
                } else if cmd == " " || cmd == "z" || cmd == "Z" {
                    return nil
                }
                let rel = cmd.isLowercase
                let up = Character(cmd.uppercased())
                let pt = { (x: Double, y: Double) in
                    rel ? CGPoint(x: cur.x + x, y: cur.y + y) : CGPoint(x: x, y: y)
                }
                switch up {
                case "M":
                    guard let v = sc.nums(2) else { return nil }
                    cur = pt(v[0], v[1])
                    start = cur
                    path.move(to: cur)
                    box.add(cur)
                    cmd = rel ? "l" : "L"
                case "L":
                    guard let v = sc.nums(2) else { return nil }
                    cur = pt(v[0], v[1])
                    path.addLine(to: cur)
                    box.add(cur)
                case "H":
                    guard let v = sc.nums(1) else { return nil }
                    cur.x = rel ? cur.x + v[0] : v[0]
                    path.addLine(to: cur)
                    box.add(cur)
                case "V":
                    guard let v = sc.nums(1) else { return nil }
                    cur.y = rel ? cur.y + v[0] : v[0]
                    path.addLine(to: cur)
                    box.add(cur)
                case "C":
                    guard let v = sc.nums(6) else { return nil }
                    let c1 = pt(v[0], v[1])
                    let c2 = pt(v[2], v[3])
                    let e = pt(v[4], v[5])
                    path.addCurve(to: e, control1: c1, control2: c2)
                    box.add(c1)
                    box.add(c2)
                    box.add(e)
                    lastC = c2
                    cur = e
                case "S":
                    guard let v = sc.nums(4) else { return nil }
                    var c1 = cur
                    if prev == "C" || prev == "S" {
                        c1 = CGPoint(x: 2 * cur.x - lastC.x, y: 2 * cur.y - lastC.y)
                    }
                    let c2 = pt(v[0], v[1])
                    let e = pt(v[2], v[3])
                    path.addCurve(to: e, control1: c1, control2: c2)
                    box.add(c1)
                    box.add(c2)
                    box.add(e)
                    lastC = c2
                    cur = e
                case "Q":
                    guard let v = sc.nums(4) else { return nil }
                    let c = pt(v[0], v[1])
                    let e = pt(v[2], v[3])
                    path.addQuadCurve(to: e, control: c)
                    box.add(c)
                    box.add(e)
                    lastC = c
                    cur = e
                case "T":
                    guard let v = sc.nums(2) else { return nil }
                    var c = cur
                    if prev == "Q" || prev == "T" {
                        c = CGPoint(x: 2 * cur.x - lastC.x, y: 2 * cur.y - lastC.y)
                    }
                    let e = pt(v[0], v[1])
                    path.addQuadCurve(to: e, control: c)
                    box.add(c)
                    box.add(e)
                    lastC = c
                    cur = e
                case "A":
                    guard let v = sc.arc() else { return nil }
                    let e = pt(v[5], v[6])
                    svgArc(path, from: cur, rx: v[0], ry: v[1], rotation: v[2], large: v[3] != 0, sweep: v[4] != 0, to: e, box: &box)
                    cur = e
                case "Z":
                    path.closeSubpath()
                    cur = start
                default:
                    return nil
                }
                empty = false
                prev = up
            }
            return SVGPath(path: path, bounds: box.rect, isEmpty: empty)
        }
    }
}

/// The command letters of an SVG path.
private func isSVGCommand(_ c: Character) -> Bool {
    switch c {
    case "M", "m", "L", "l", "H", "h", "V", "v", "C", "c", "S", "s", "Q", "q", "T", "t", "A", "a", "Z", "z":
        return true
    default:
        return false
    }
}

/// Reads the d attribute as ParseSVGPath does: numbers need no separator, a command repeats its numbers.
private struct SVGScanner {
    let s: [Character]
    var i = 0

    init(_ text: String) {
        s = Array(text)
    }

    var atEnd: Bool { i >= s.count }

    mutating func skip() {
        while i < s.count, s[i] == " " || s[i] == "," || s[i] == "\n" || s[i] == "\t" || s[i] == "\r" {
            i += 1
        }
    }

    mutating func num() -> Double? {
        skip()
        let start = i
        var j = i
        if j < s.count, s[j] == "-" || s[j] == "+" { j += 1 }
        var dot = false
        while j < s.count {
            let c = s[j]
            if c >= "0" && c <= "9" {
                j += 1
            } else if c == "." && !dot {
                dot = true
                j += 1
            } else {
                break
            }
        }
        if j < s.count, s[j] == "e" || s[j] == "E" {
            var k = j + 1
            if k < s.count, s[k] == "-" || s[k] == "+" { k += 1 }
            if k < s.count, s[k] >= "0", s[k] <= "9" {
                while k < s.count, s[k] >= "0", s[k] <= "9" { k += 1 }
                j = k
            }
        }
        guard let v = Double(String(s[start..<j])) else { return nil }
        i = j
        return v
    }

    mutating func nums(_ n: Int) -> [Double]? {
        var out: [Double] = []
        for _ in 0..<n {
            guard let v = num() else { return nil }
            out.append(v)
        }
        return out
    }

    /// rx ry rotation large sweep x y: the two flags are single characters, as Go reads them.
    mutating func arc() -> [Double]? {
        var out: [Double] = []
        var i = 0
        while i < 7 {
            if i == 3 || i == 4 {
                skip()
                guard self.i < s.count, s[self.i] == "0" || s[self.i] == "1" else { return nil }
                out.append(s[self.i] == "1" ? 1 : 0)
                self.i += 1
                i += 1
                continue
            }
            guard let v = num() else { return nil }
            out.append(v)
            i += 1
        }
        return out
    }
}

/// The box of the points of a path, as icon.go takes it.
private struct PointBox {
    var x0 = Double.infinity
    var y0 = Double.infinity
    var x1 = -Double.infinity
    var y1 = -Double.infinity

    mutating func add(_ p: CGPoint) {
        x0 = min(x0, p.x)
        y0 = min(y0, p.y)
        x1 = max(x1, p.x)
        y1 = max(y1, p.y)
    }

    var rect: CGRect {
        x0.isFinite ? CGRect(x: x0, y: y0, width: x1 - x0, height: y1 - y0) : .zero
    }
}

/// Appends the elliptical arc from p0 to p1 as cubics (SVG endpoint parameterization), as arcTo of svgpath.go.
private func svgArc(
    _ path: CGMutablePath, from p0: CGPoint, rx rx0: Double, ry ry0: Double, rotation rotDeg: Double, large: Bool,
    sweep: Bool, to p1: CGPoint, box: inout PointBox
) {
    var rx = abs(rx0)
    var ry = abs(ry0)
    if rx == 0 || ry == 0 || p0 == p1 {
        path.addLine(to: p1)
        box.add(p1)
        return
    }
    let phi = rotDeg * Double.pi / 180
    let sphi = sin(phi)
    let cphi = cos(phi)
    let dx = (p0.x - p1.x) / 2
    let dy = (p0.y - p1.y) / 2
    let x1 = cphi * dx + sphi * dy
    let y1 = -sphi * dx + cphi * dy
    let l = x1 * x1 / (rx * rx) + y1 * y1 / (ry * ry)
    if l > 1 {
        let s = sqrt(l)
        rx *= s
        ry *= s
    }
    let num = rx * rx * ry * ry - rx * rx * y1 * y1 - ry * ry * x1 * x1
    let den = rx * rx * y1 * y1 + ry * ry * x1 * x1
    var co = 0.0
    if den != 0 && num > 0 {
        co = sqrt(num / den)
    }
    if large == sweep {
        co = -co
    }
    let cxp = co * rx * y1 / ry
    let cyp = -co * ry * x1 / rx
    let cx = cphi * cxp - sphi * cyp + (p0.x + p1.x) / 2
    let cy = sphi * cxp + cphi * cyp + (p0.y + p1.y) / 2
    func ang(_ ux: Double, _ uy: Double, _ vx: Double, _ vy: Double) -> Double {
        atan2(ux * vy - uy * vx, ux * vx + uy * vy)
    }
    let th1 = ang(1, 0, (x1 - cxp) / rx, (y1 - cyp) / ry)
    var dth = ang((x1 - cxp) / rx, (y1 - cyp) / ry, (-x1 - cxp) / rx, (-y1 - cyp) / ry)
    if !sweep && dth > 0 {
        dth -= 2 * Double.pi
    } else if sweep && dth < 0 {
        dth += 2 * Double.pi
    }
    let n = dth.isFinite ? Int(ceil(abs(dth) / (Double.pi / 2))) : 0
    guard n > 0 else { return }
    let step = dth / Double(n)
    let k = 4.0 / 3 * tan(step / 4)
    func at(_ t: Double) -> (CGPoint, CGPoint) {
        let s = sin(t)
        let c = cos(t)
        let pt = CGPoint(x: cx + cphi * rx * c - sphi * ry * s, y: cy + sphi * rx * c + cphi * ry * s)
        let d = CGPoint(x: -cphi * rx * s - sphi * ry * c, y: -sphi * rx * s + cphi * ry * c)
        return (pt, d)
    }
    for i in 0..<n {
        let a0 = th1 + Double(i) * step
        let a1 = th1 + Double(i + 1) * step
        let (q0, d0) = at(a0)
        var (q1, d1) = at(a1)
        if i == n - 1 {
            q1 = p1
        }
        let c1p = CGPoint(x: q0.x + k * d0.x, y: q0.y + k * d0.y)
        let c2p = CGPoint(x: q1.x - k * d1.x, y: q1.y - k * d1.y)
        path.addCurve(to: q1, control1: c1p, control2: c2p)
        box.add(c1p)
        box.add(c2p)
        box.add(q1)
    }
}
