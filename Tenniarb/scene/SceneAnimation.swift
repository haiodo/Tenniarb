//
//  SceneAnimation.swift
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

extension Scene {
    /// One keyframe of an animation: where it is on the timeline and what it sets there.
    struct Frame {
        let percent: String  // from, to or a percentage, as written
        let offset: Double  // 0..1; the frames of an animation are sorted by it
        var props: [Prop] = []  // what the frame sets, the still picture when it is the first frame
    }

    /// One animation of an item (spec 13): the frames of a keyframes block and the timing it plays with.
    struct Animation {
        let name: String
        var duration = 0.0  // seconds
        var easing = "ease"  // one of easings; "ease" when not written, as in CSS
        var repeatCount = 0  // 0 or 1: once; -1: forever; n: n times
        var frames: [Frame] = []
    }
}

extension Scene.Animation {
    /// The rotate in degrees, the scale and the opacity at now seconds, from the frames. loop plays a finite
    /// animation again after its end: the editor previews every animation that way.
    func values(at now: Double, loop: Bool) -> (rotate: Double, scale: Double, opacity: Double) {
        guard !frames.isEmpty, duration > 0 else { return (0, 1, 1) }
        var now = now
        var total = duration
        if loop || repeatCount < 0 {  // forever
            total = 0
        } else if repeatCount > 1 {
            total *= Double(repeatCount)
        }
        if total > 0 && now >= total {
            now = total - 1e-9  // the last frame stays
        }
        let at = now.truncatingRemainder(dividingBy: duration) / duration
        var lo = frames[0]
        var hi = frames[frames.count - 1]
        for (i, f) in frames.enumerated() {
            if f.offset > at { break }
            lo = f
            if i + 1 < frames.count {
                hi = frames[i + 1]
            }
        }
        var k = 1.0
        let span = hi.offset - lo.offset
        if span > 0 {
            k = min(1, max(0, (at - lo.offset) / span))
        }
        k = ease(easing, k)
        let (r0, s0, o0) = frameValues(lo)
        let (r1, s1, o1) = frameValues(hi)
        return (r0 + (r1 - r0) * k, s0 + (s1 - s0) * k, o0 + (o1 - o0) * k)
    }
}

extension Scene.Screen {
    /// The animation the drawn item plays (spec 13), nil when it plays none.
    func animation(of pl: Scene.Placed) -> Scene.Animation? {
        p.anim[ObjectIdentifier(pl.r)]
    }
}

/// The rotate, the scale and the opacity a frame sets, and their defaults.
func frameValues(_ f: Scene.Frame) -> (rotate: Double, scale: Double, opacity: Double) {
    var rot = 0.0
    var scale = 1.0
    var opacity = 1.0
    for p in f.props {
        guard let v = p.num(0) else { continue }
        switch p.name {
        case "rotate": rot = v
        case "scale": scale = v
        case "opacity": opacity = v
        default: break
        }
    }
    return (rot, scale, opacity)
}

/// Maps the progress inside a segment by the CSS easing of the animation.
func ease(_ name: String, _ t: Double) -> Double {
    switch name {
    case "linear":
        return t
    case "ease-in":
        return bezier(t, 0.42, 0, 1, 1)
    case "ease-out":
        return bezier(t, 0, 0, 0.58, 1)
    case "ease-in-out":
        return bezier(t, 0.42, 0, 0.58, 1)
    default:  // ease
        return bezier(t, 0.25, 0.1, 0.25, 1)
    }
}

/// Solves a cubic-bezier timing function: the y of the curve at x t.
func bezier(_ t: Double, _ x1: Double, _ y1: Double, _ x2: Double, _ y2: Double) -> Double {
    if t <= 0 || t >= 1 {
        return t
    }
    func curve(_ s: Double, _ p1: Double, _ p2: Double) -> Double {
        3 * (1 - s) * (1 - s) * s * p1 + 3 * (1 - s) * s * s * p2 + s * s * s
    }
    var lo = 0.0
    var hi = 1.0
    for _ in 0..<30 {
        let mid = (lo + hi) / 2
        if curve(mid, x1, x2) < t {
            lo = mid
        } else {
            hi = mid
        }
    }
    return curve((lo + hi) / 2, y1, y2)
}

/// The properties a keyframe may set (spec 13): appearance only, so the layout never has to run per frame.
let animatedProps: Set<String> = [
    "rotate", "scale", "opacity", "color", "border-color", "text-color", "pos",
]

/// The properties a frame may not set: they need a layout of the whole scene per frame.
let layoutProps: Set<String> = [
    "width", "height", "min-width", "max-width", "min-height", "max-height", "gap", "line-gap", "padding",
    "auto-layout", "wrap", "align", "justify", "absolute",
]

/// The easing words of an animation or a transition, as CSS names them.
let easings: Set<String> = ["linear", "ease", "ease-in", "ease-out", "ease-in-out"]

/// What a property takes, for the warning about a frame value that does not read.
let declShape = [
    "rotate": "degrees", "scale": "1 or 2 numbers", "opacity": "0..1", "pos": "two numbers",
    "color": "a color", "border-color": "a color", "text-color": "a color",
]

/// One keyframe property as a CSS declaration, "" when the property is not animated or its value does not read
/// as one. rotate, scale and translate add to the transform of the group, so the item keeps its place.
func decl(_ pr: Scene.Prop) -> String {
    switch pr.name {
    case "rotate":
        if let v = pr.num(0) { return "rotate:\(cssNumber(v))deg" }
    case "scale":
        if let v = pr.num(0) {
            if let w = pr.num(1) { return "scale:\(cssNumber(v)) \(cssNumber(w))" }
            return "scale:\(cssNumber(v))"
        }
    case "opacity":
        if let v = pr.num(0) { return "opacity:\(cssNumber(v))" }
    case "pos":
        if let x = pr.num(0), let y = pr.num(1) {
            return "translate:\(cssNumber(x))px \(cssNumber(-y))px"  // Y is up in .tenn, down in CSS
        }
    case "color", "text-color", "border-color":
        guard let c = Scene.parseColor(pr.text(0) ?? "") else { return "" }
        return (pr.name == "border-color" ? "stroke:" : "fill:") + cssColor(c)
    default:
        break
    }
    return ""
}

/// %g of Go: the shortest digits of a CSS number.
private func cssNumber(_ v: Double) -> String {
    String(format: "%g", v)
}

/// c as CSS rgb(), the three channels gfx also writes into the SVG.
private func cssColor(_ c: Scene.Color) -> String {
    func q(_ v: Double) -> Int { Int((min(1, max(0, v)) * 255).rounded()) }
    return "rgb(\(q(c.r)),\(q(c.g)),\(q(c.b)))"
}

/// Reads a CSS time: 300ms, 1s, 1.5s; nil when it is not one or is below zero.
private func parseDuration(_ s: String) -> Double? {
    var text = s.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
    var unit = 1.0
    if text.hasSuffix("ms") {
        unit = 0.001
        text.removeLast(2)
    } else if text.hasSuffix("s") {
        text.removeLast()
    } else {
        return nil
    }
    guard let v = Double(text), v >= 0 else { return nil }
    return v * unit
}

/// Reads a frame selector: from, to or a percentage 0..100%.
private func frameOffset(_ sel: String) -> Double? {
    switch sel {
    case "from": return 0
    case "to": return 1
    default:
        guard sel.hasSuffix("%"), let v = Double(sel.dropLast()), v >= 0, v <= 100 else { return nil }
        return v / 100
    }
}

/// The name of a command, "" for anything else (cmdName of go/scene/load.go).
private func commandName(_ c: TennNode) -> String {
    guard c.kind == .Command else { return "" }
    return c.getIdent(0) ?? ""
}

/// The arguments of a command and the commands of its block (split of go/scene/load.go).
private func splitCommand(_ c: TennNode) -> (args: [TennNode], block: [TennNode]) {
    var args: [TennNode] = []
    var block: [TennNode] = []
    for a in (c.children ?? []).dropFirst() {
        if a.kind == .BlockExpr {
            block.append(contentsOf: a.children ?? [])
        } else {
            args.append(a)
        }
    }
    return (args, block)
}

/// nodePos of go/scene/load.go: the position of a command in the file that holds the block it is written in.
private func commandPos(_ file: String, _ c: TennNode) -> Scene.Pos {
    guard let token = c.token else { return Scene.Pos(file: file, line: 1, col: 1) }
    return Scene.Pos(file: file, line: token.line + 1, col: token.col - token.size + 1)
}

extension Scene.Prep {
    /// Resolves the animations of rn and of its kids (spec 13) before the layout: the first frame is written into
    /// the node as if the file said it, so the still picture of PNG and SVG is that frame.
    func animations(_ rn: Scene.RNode, _ defs: [String: Scene.Animation]?) {
        var local = defs
        var own = false
        var declared: Set<String> = []
        for pr in allProps(rn, "animation") where pr.block != nil {
            guard let a = keyframes(pr, rn) else { continue }
            if !own {
                local = copyDefs(defs)
                own = true
            }
            if declared.contains(a.name) {
                t.warn(pr.pos, "animation \(a.name): the frames are declared twice, the last one is used")
            }
            declared.insert(a.name)
            local?[a.name] = a
        }
        for pr in allProps(rn, "animation") where pr.block == nil {
            guard let a = animationUse(pr, local) else { continue }
            anim[ObjectIdentifier(rn)] = a
            rn.props.append(contentsOf: a.frames[0].props)
        }
        for pr in allProps(rn, "transition") {
            transition(pr)
        }
        for k in rn.kids {
            animations(k, local)
        }
    }

    private func copyDefs(_ defs: [String: Scene.Animation]?) -> [String: Scene.Animation] {
        var out = defs ?? [:]
        out.reserveCapacity(out.count + 1)
        return out
    }

    /// The properties of that name on rn, evaluated (spec 6): a node may name several animations.
    private func allProps(_ rn: Scene.RNode, _ name: String) -> [Scene.Prop] {
        var out: [Scene.Prop] = []
        for pr in rn.props where pr.name == name {
            if !hasExpr(pr) {
                out.append(pr)
                continue
            }
            if !canEval() { continue }  // a value check cannot know; the build with an evaluator reports it
            if let evaluated = evalProp(rn, pr) {
                out.append(evaluated)
            }
        }
        return out
    }

    /// Reads "animation <name> { <frame> { properties } ... }": the frames every node that names the animation
    /// plays, as a CSS @keyframes block. A frame property may be an expression, evaluated in the scope of rn.
    private func keyframes(_ pr: Scene.Prop, _ rn: Scene.RNode) -> Scene.Animation? {
        let name = pr.text(0) ?? ""
        if name.isEmpty {
            t.warn(pr.pos, "animation: the block needs a name, as in animation spin { from { ... } to { ... } }")
            return nil
        }
        var a = Scene.Animation(name: name)
        for c in pr.block?.children ?? [] {
            guard c.kind == .Command else { continue }
            let sel = commandName(c)
            guard let off = frameOffset(sel) else {
                t.warn(commandPos(pr.pos.file, c), "animation \(name): frame \"\(sel)\" is not from, to or a percentage")
                continue
            }
            var f = Scene.Frame(percent: sel, offset: off)
            for pc in splitCommand(c).block {
                let pn = commandName(pc)
                if pn.isEmpty { continue }
                var fp = Scene.Prop(
                    name: pn, args: splitCommand(pc).args, pos: commandPos(pr.pos.file, pc), range: Scene.Range())
                if hasExpr(fp) {
                    if eval == nil { continue }  // a value check cannot know; the build with an evaluator reports it
                    guard let evaluated = evalProp(rn, fp) else { continue }
                    fp = evaluated
                }
                switch true {
                case decl(fp) != "":
                    f.props.append(fp)
                case layoutProps.contains(pn):
                    t.warn(fp.pos, "animation \(name): \(pn) is a layout property and is not animated, the frame is ignored")
                case animatedProps.contains(pn):
                    t.warn(fp.pos, "animation \(name): \(pn) takes \(declShape[pn] ?? ""), the frame is ignored")
                default:
                    t.warn(
                        fp.pos,
                        "animation \(name): \(pn) is not animated: rotate, scale, opacity, color, border-color, text-color or pos"
                    )
                }
            }
            a.frames.append(f)
        }
        if a.frames.isEmpty {
            t.warn(pr.pos, "animation \(name): the block has no frames (from { ... } to { ... })")
            return nil
        }
        // Frames of one offset keep file order, as SliceStable does in Go.
        a.frames =
            a.frames.enumerated().sorted { ($0.element.offset, $0.offset) < ($1.element.offset, $1.offset) }
            .map(\.element)
        return a
    }

    /// Reads "animation <name> <duration> [easing] [repeat]" and plays the frames of the block of that name.
    private func animationUse(_ pr: Scene.Prop, _ defs: [String: Scene.Animation]?) -> Scene.Animation? {
        let name = pr.text(0) ?? ""
        if name.isEmpty {
            t.warn(pr.pos, "animation takes a name and a duration: animation spin 1s linear infinite")
            return nil
        }
        guard let def = defs?[name] else {
            t.warn(pr.pos, "animation \(name): no keyframes block with that name in the element")
            return nil
        }
        if pr.args.count < 2 {
            t.warn(pr.pos, "animation \(name): the duration is missing, as in animation \(name) 1s linear infinite")
            return nil
        }
        let word = pr.text(1) ?? ""
        guard let dur = parseDuration(word) else {
            t.warn(pr.pos, "animation \(name): \"\(word)\" is not a duration, as in 300ms or 1s")
            return nil
        }
        if dur <= 0 {
            t.warn(pr.pos, "animation \(name): the duration must be above zero")
            return nil
        }
        var a = def
        a.duration = dur
        a.easing = "ease"
        for i in 2..<pr.args.count {
            let w = pr.text(i) ?? ""
            switch true {
            case easings.contains(w):
                a.easing = w
            case w == "infinite":
                a.repeatCount = -1
            default:
                guard let n = Int(w), n >= 0 else {
                    t.warn(
                        pr.pos,
                        "animation \(name): \"\(w)\" is not an easing (linear, ease, ease-in, ease-out, ease-in-out) or a repeat count"
                    )
                    continue
                }
                a.repeatCount = n
            }
        }
        return a
    }

    /// Reads "transition <properties> <duration> [easing]" (spec 13). The format is checked only: a transition
    /// plays when a style changes, and a scene has no such state yet - a rebuild draws the new value at once.
    private func transition(_ pr: Scene.Prop) {
        var words: [String] = []
        for i in 0..<pr.args.count {
            words.append(pr.text(i) ?? "")
        }
        if let last = words.last, easings.contains(last) {
            words.removeLast()
        }
        if words.count < 2 {
            t.warn(pr.pos, "transition takes properties, a duration and an easing: transition color 200ms ease")
            return
        }
        let word = words.removeLast()
        guard let dur = parseDuration(word) else {
            t.warn(
                pr.pos,
                "transition: \"\(word)\" is not a duration (200ms or 1s) or an easing (linear, ease, ease-in, ease-out, ease-in-out)"
            )
            return
        }
        if dur <= 0 {
            t.warn(pr.pos, "transition: the duration must be above zero")
        }
        let names = words.joined(separator: " ").split { $0 == "," || $0 == ";" || $0 == " " }
        for name in names {
            switch true {
            case animatedProps.contains(String(name)):
                break
            case layoutProps.contains(String(name)):
                t.warn(pr.pos, "transition: \(name) is a layout property and is not animated")
            default:
                t.warn(pr.pos, "transition: \(name) is not animated: rotate, scale, opacity, color, border-color, text-color or pos")
            }
        }
    }
}
