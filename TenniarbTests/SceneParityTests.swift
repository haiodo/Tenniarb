//
//  SceneParityTests.swift
//  TenniarbTests
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
import XCTest

@testable import Tenniarb

/// The Swift side of go/cmd/tennlayout: the same JSON as `web/fixtures/scene/*.layout.json`, made by
/// go/cmd/tennlayout with gfx.EstimateText, so the scene engine is compared against Go itself.
///
/// button-styles and fusio-1620 build without go/scene/icon.go: the dump reads boxes, fills and text, not the
/// paths an icon draws inside its leaf.
final class SceneParityTests: XCTestCase {

    private let fixtures = URL(fileURLWithPath: #filePath)
        .deletingLastPathComponent().deletingLastPathComponent()
        .appendingPathComponent("web/fixtures/scene")

    /// A .tenn with no golden is not a stand fixture; it must be named here so a new one cannot slip in.
    private let noGolden = ["ui-kit.lib.tenn": "a library, go/cmd/tennlayout makes no golden for it"]

    // MARK: - Stand

    func testEveryFixtureHasAGolden() throws {
        let files = try FileManager.default.contentsOfDirectory(atPath: fixtures.path)
        let goldens = Set(files.filter { $0.hasSuffix(".layout.json") }.map { String($0.dropLast(".layout.json".count)) })
        let tenns = Set(files.filter { $0.hasSuffix(".tenn") }.map { String($0.dropLast(".tenn".count)) })
        XCTAssertEqual(
            tenns.subtracting(goldens).subtracting(noGolden.keys.map { $0.replacingOccurrences(of: ".tenn", with: "") }), [],
            "tenn files with no golden and no reason")
        XCTAssertEqual(Set(noGolden.keys).filter { !files.contains($0) }, [], "noGolden names a missing file")
    }

    func testSceneParityWithTennlayout() throws {
        let files = try FileManager.default.contentsOfDirectory(atPath: fixtures.path)
        let names = files.filter { $0.hasSuffix(".layout.json") }
            .map { String($0.dropLast(".layout.json".count)) }
            .sorted()
        for name in names {
            try compare(name)
        }
    }

    private func compare(_ name: String) throws {
        let goldenData = try Data(contentsOf: fixtures.appendingPathComponent(name + ".layout.json"))
        let golden = try XCTUnwrap(JSONSerialization.jsonObject(with: goldenData) as? [String: Any])
        let element = golden["element"] as? String ?? ""

        let tree = try SceneLoader(root: fixtures).load(name + ".tenn")
        // Options.Classic of go/cmd/tennlayout: the values of the element model, nil for a scene.
        let classic = Scene.classicHook(tree, file: name + ".tenn", path: element, context: Scene.classicContext)
        guard
            let screen = tree.build(
                element, measurer: Scene.TextMeasurer(estimated: true), eval: sceneEvalVars, classic: classic)
        else {
            return XCTFail("\(name): the scene does not build: \(tree.diagnostics)")
        }
        var got = dump(screen)
        got["element"] = element
        // Compare the JSON both sides write, not the objects: formatting is not the point, values are.
        let gotJSON = try XCTUnwrap(
            JSONSerialization.jsonObject(with: JSONSerialization.data(withJSONObject: got)) as? [String: Any])
        if let difference = difference(golden, gotJSON, at: name) {
            XCTFail(difference)
        }
    }
}

// MARK: - tennlayout dump

/// The output of go/cmd/tennlayout `dump`, read from the built screen instead of decoded gfx pictures.
private func dump(_ screen: Scene.Screen) -> [String: Any] {
    let boxes = screen.boxes()
    var placed: [Scene.Placed] = []
    func walk(_ p: Scene.Placed) {
        placed.append(p)
        for k in p.kids {
            walk(k)
        }
    }
    walk(screen.placed())
    let nodes: [Any] = placed.enumerated().map { i, pl in
        node(pl, i < boxes.count ? boxes[i].path : "", screen, isRoot: i == 0)
    }
    return [
        "width": round3(screen.w),
        "height": round3(screen.h),
        "nodes": nodes,
        "links": screen.links().map(linkJSON),
    ]
}

/// One link as go/cmd/tennlayout writes it: the stroke, the arrow flag and the path in the item's coordinates.
private func linkJSON(_ link: Scene.Link) -> [String: Any] {
    let dash: Any = link.dash.map { $0.map(round3) } ?? NSNull()
    return [
        "parent": link.parent,
        "path": link.path.map(pathStep),
        "color": hex(link.color),
        "width": round3(link.width),
        "dash": dash,
        "phase": round3(link.dashPhase),
        "arrow": link.arrow,
    ]
}

/// A path as verbs and points: ["m", x, y], ["l", x, y], ["q", cx, cy, x, y].
private func pathStep(_ step: Scene.PathStep) -> [Any] {
    switch step {
    case let .move(x, y): return ["m", round3(x), round3(y)]
    case let .line(x, y): return ["l", round3(x), round3(y)]
    case let .quad(cx, cy, x, y): return ["q", round3(cx), round3(cy), round3(x), round3(y)]
    }
}

private func node(_ pl: Scene.Placed, _ path: String, _ screen: Scene.Screen, isRoot: Bool) -> [String: Any] {
    let p = screen.p
    let rn = pl.r
    let id = ObjectIdentifier(rn)
    let ln = p.frames[id]!
    let st = p.styles[id] ?? Scene.Look()
    let w = ln.rw
    let h = ln.rh
    let clip = flagProp(pl, "clip")
    let painted = w > 0 && h > 0 && st.display != "none"

    var fill: Scene.Color?
    var border: Scene.Color?
    var borderW = 0.0
    var grad: Scene.Gradient?
    var shadows: [Scene.Shadow] = []
    var textColor: Scene.Color?

    if painted {
        if st.display == "stack" {
            // Three sheets behind the box, all with the same shadow, so the dump keeps one.
            fill = st.fill
            addShadow(&shadows, Scene.Shadow(dx: 1, dy: -1, blur: defaultShadowBlur, color: defaultShadowColor))
        }
        let hasFill = st.display != "text" && st.display != "no-fill" && st.fill.a > 0
        if hasFill {
            if let g = st.grad {
                fill = g.base()
                grad = g
                for sh in st.shadows {
                    addShadow(&shadows, sh)
                }
            } else if st.shadows.isEmpty {
                fill = st.fill
            } else {
                // A shadow takes the alpha of the fill: the picture paints an opaque base, then no plain fill.
                var base = st.fill
                base.a = 1
                fill = base
                for sh in st.shadows {
                    addShadow(&shadows, sh)
                }
            }
        }
        if st.display != "text" && st.lineW > 0 && st.border.a > 0 {
            border = st.border
            borderW = st.lineW
            if !hasFill {
                for sh in st.shadows {
                    addShadow(&shadows, sh)
                }
            }
        }
    }

    var lines: [String] = []
    if painted {
        if let ti = p.texts[id] {
            if ti.parts != nil {
                // A marked-up title draws one text op per part of the line, as drawParts of go/scene does.
                for line in ti.layout(w - 2 * ti.padX) {
                    for part in line.parts where !part.text.isEmpty {
                        lines.append(part.text)
                    }
                }
            } else {
                lines = ti.wrap(w - 2 * ti.padX).filter { !$0.isEmpty }
            }
        }
        // The body is drawn under the title, one text op per run: the picture of an item carries both.
        if let body = p.bodies[id], p.bodyAt[id] == nil {
            for line in body.lines(w - 2 * body.padX) {
                for run in line.runs where !run.text.isEmpty {
                    lines.append(run.text)
                }
            }
        }
    }
    if !lines.isEmpty {
        textColor = st.text
        if st.display == "text", let sh = st.shadows.first {
            addShadow(&shadows, sh)  // display text has no box, so the shadow falls on the glyphs
        }
    }

    var radius = [0.0, 0.0, 0.0, 0.0]
    if fill == nil && painted && border != nil {
        // A border-only item draws its path inside by half the line width.
        radius = borderRadius(st, w, h)
    }
    if fill != nil || clip {
        radius = propRadius(pl, w, h, screen.classic)
    }

    return [
        "path": path,
        "name": pl.src?.name ?? "",
        "rect": ["x": round3(pl.x), "y": round3(pl.y), "w": round3(pl.w), "h": round3(pl.h)],
        "radius": radius.map(round3),
        "rotate": round3(rotation(pl)),
        "clip": clip,
        "opacity": round3(isRoot ? 1 : st.opacity),
        "text": lines.isEmpty ? NSNull() : lines,
        "fill": colorOf(fill),
        "border": colorOf(border),
        "borderWidth": border == nil ? NSNull() : round3(borderW),
        "textColor": colorOf(textColor),
        "shadows": shadows.map { ["dx": round3($0.dx), "dy": round3($0.dy), "blur": round3($0.blur), "color": hex($0.color)] },
        "gradient": grad.map { gradientOf($0) } ?? NSNull(),
    ]
}

/// The corner radius of a rounded rectangle path inside the border by half the line width.
private func borderRadius(_ st: Scene.Look, _ w: Double, _ h: Double) -> [Double] {
    if st.display == "circle" {
        return [0.0, 0.0, 0.0, 0.0]  // an ellipse has no corners
    }
    let lim = min(w - st.lineW, h - st.lineW) / 2
    return st.radius.map { r in
        let v = min(max(r - st.lineW / 2, 0), lim)
        return v < 0.001 ? 0 : v
    }
}

/// propRadius: the corner-radius of the item, clamped as gfx.RoundRect4 clamps it.
private func propRadius(_ pl: Scene.Placed, _ w: Double, _ h: Double, _ classic: Bool) -> [Double] {
    var out = [0.0, 0.0, 0.0, 0.0]
    let pr = pl.prop("corner-radius")
    switch true {
    case pr == nil && (!classic || displayOf(pl) == "circle" || w < 2 || h < 2):
        return out
    case pr == nil:
        out = [Scene.classicRadius, Scene.classicRadius, Scene.classicRadius, Scene.classicRadius]
    default:
        let first = pr!.num(0) ?? 0
        out = [first, first, first, first]
        if pr!.args.count == 4 {
            for i in 0..<4 {
                if let v = pr!.num(i) {
                    out[i] = v
                }
            }
        }
    }
    let lim = min(w, h) / 2
    for i in 0..<4 {
        out[i] = min(out[i], lim)
    }
    return out
}

private func displayOf(_ pl: Scene.Placed) -> String {
    pl.prop("display")?.text(0) ?? "rect"
}

/// The rotate of the item as the angle of its transform: the dump reads atan2(m[1], m[0]).
private func rotation(_ pl: Scene.Placed) -> Double {
    let deg = pl.prop("rotate")?.num(0) ?? 0
    let a = deg * Double.pi / 180
    return atan2(sin(a), cos(a)) * 180 / Double.pi
}

private func flagProp(_ pl: Scene.Placed, _ name: String) -> Bool {
    guard let v = pl.prop(name)?.text(0) else { return false }
    return v == "yes" || v == "true"
}

private func gradientOf(_ g: Scene.Gradient) -> [String: Any] {
    let a = g.angle * Double.pi / 180
    let deg = (atan2(sin(a), cos(a)) * 180 / Double.pi).truncatingRemainder(dividingBy: 360)
    let stops: [Any] = g.stops.map { ["offset": round3($0.offset), "color": hex($0.color)] }
    return ["angle": round3((deg + 360).truncatingRemainder(dividingBy: 360)), "stops": stops]
}

private func addShadow(_ list: inout [Scene.Shadow], _ shadow: Scene.Shadow) {
    if let last = list.last, last == shadow {
        return
    }
    list.append(shadow)
}

private func colorOf(_ c: Scene.Color?) -> Any {
    c.map(hex) ?? NSNull()
}

private func hex(_ c: Scene.Color) -> String {
    func q(_ v: Double) -> Int { Int((min(1, max(0, v)) * 255).rounded()) }
    if q(c.a) >= 255 {
        return String(format: "#%02x%02x%02x", q(c.r), q(c.g), q(c.b))
    }
    return String(format: "#%02x%02x%02x%02x", q(c.r), q(c.g), q(c.b), q(c.a))
}

private func round3(_ v: Double) -> Double {
    (v * 1000).rounded() / 1000
}

// MARK: - Parsed JSON comparison

/// The first difference between two parsed JSON values, with its path; nil when they are equal.
private func difference(_ want: Any, _ got: Any, at path: String) -> String? {
    if let w = want as? [String: Any], let g = got as? [String: Any] {
        if Set(w.keys) != Set(g.keys) {
            return "\(path): keys \(w.keys.sorted()) != \(g.keys.sorted())"
        }
        for k in w.keys.sorted() {
            if let d = difference(w[k]!, g[k]!, at: "\(path).\(k)") {
                return d
            }
        }
        return nil
    }
    if let w = want as? [Any], let g = got as? [Any] {
        if w.count != g.count {
            return "\(path): \(w.count) elements != \(g.count)"
        }
        for i in 0..<w.count {
            if let d = difference(w[i], g[i], at: "\(path)[\(i)]") {
                return d
            }
        }
        return nil
    }
    if want is NSNull || got is NSNull {
        return want is NSNull && got is NSNull ? nil : "\(path): \(want) != \(got)"
    }
    if let w = want as? String, let g = got as? String {
        return w == g ? nil : "\(path): \"\(w)\" != \"\(g)\""
    }
    if let w = want as? NSNumber, let g = got as? NSNumber {
        let wBool = CFGetTypeID(w) == CFBooleanGetTypeID()
        let gBool = CFGetTypeID(g) == CFBooleanGetTypeID()
        if wBool != gBool {
            return "\(path): \(wBool ? "bool" : "number") != \(gBool ? "bool" : "number")"
        }
        let equal = wBool ? w.boolValue == g.boolValue : w.doubleValue == g.doubleValue
        return equal ? nil : "\(path): \(w) != \(g)"
    }
    return "\(path): \(want) != \(got)"
}
